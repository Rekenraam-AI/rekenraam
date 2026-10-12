package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite "github.com/mattn/go-sqlite3"
)

// OnlineBackupOptions bounds a copy. The online backup API restarts its work
// when the source is written mid-flight, so on a busy database an unbounded
// copy can starve; a deadline turns that into a retryable failure with a
// legible reason instead of a worker that never returns.
type OnlineBackupOptions struct {
	// PagesPerStep is how much is copied between yields. Smaller steps hold
	// the source lock for shorter periods.
	PagesPerStep int32
	// Deadline bounds the whole copy.
	Deadline time.Duration
}

// OnlineBackupResult describes what was copied.
type OnlineBackupResult struct {
	PageCount int
	ByteSize  int64
}

// ErrOnlineBackupDeadline means the copy did not finish inside its deadline —
// retryable, not terminal: the same work can succeed once writes quieten.
var ErrOnlineBackupDeadline = errors.New("online backup did not complete within its deadline")

// OnlineBackupSQLiteDatabase copies a live database using SQLite's online
// backup API, which ADR 0004 names as the preferred in-app mechanism.
// VACUUM INTO stays the operator-triggered path (BackupSQLiteDatabase).
//
// The whole copy — Backup, every Step, and Finish — runs inside nested
// sql.Conn.Raw callbacks, because the raw driver connection is only valid for
// that callback's lifetime; keeping it beyond that is a use-after-free waiting
// for an unlucky schedule.
//
// The source should be the read-only pool: a nightly copy of a large book must
// not hold the single write connection.
func OnlineBackupSQLiteDatabase(ctx context.Context, source *sql.DB, backupPath string, options OnlineBackupOptions) (OnlineBackupResult, error) {
	backupPath = filepath.Clean(strings.TrimSpace(backupPath))
	if backupPath == "" {
		return OnlineBackupResult{}, fmt.Errorf("backup path is required")
	}
	if options.PagesPerStep <= 0 {
		options.PagesPerStep = 256
	}
	if options.Deadline <= 0 {
		options.Deadline = 10 * time.Minute
	}

	// 0700, not 0755: the backup is a full copy of the ledger, and the file
	// itself is pinned to 0600 below.
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o700); err != nil {
		return OnlineBackupResult{}, fmt.Errorf("create backup directory: %w", err)
	}
	if _, err := os.Stat(backupPath); err == nil {
		return OnlineBackupResult{}, fmt.Errorf("backup path already exists")
	} else if !os.IsNotExist(err) {
		return OnlineBackupResult{}, fmt.Errorf("stat backup path: %w", err)
	}

	copyCtx, cancel := context.WithTimeout(ctx, options.Deadline)
	defer cancel()

	connection, err := source.Conn(copyCtx)
	if err != nil {
		return OnlineBackupResult{}, fmt.Errorf("acquire backup source connection: %w", err)
	}
	defer connection.Close()

	// Reserve the artifact privately before the driver writes financial data.
	// O_EXCL also closes the gap between the existence check and creation.
	file, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return OnlineBackupResult{}, fmt.Errorf("create backup destination: %w", err)
	}
	if err := file.Close(); err != nil {
		removeBackupFiles(backupPath)
		return OnlineBackupResult{}, fmt.Errorf("close new backup destination: %w", err)
	}

	// The destination uses DELETE journalling, not the live writer's WAL policy.
	// Both sql.Conn.Raw callbacks stay active until Finish has released SQLite's
	// backup handle. No raw connection escapes its callback.
	destination, err := sql.Open(driverName, sqliteAuxiliaryDSN(backupPath, false))
	if err != nil {
		removeBackupFiles(backupPath)
		return OnlineBackupResult{}, fmt.Errorf("open backup destination: %w", err)
	}
	destination.SetMaxOpenConns(1)
	defer destination.Close()
	destinationConn, err := destination.Conn(copyCtx)
	if err != nil {
		_ = destination.Close()
		removeBackupFiles(backupPath)
		return OnlineBackupResult{}, fmt.Errorf("acquire backup destination: %w", err)
	}
	var result OnlineBackupResult
	rawErr := connection.Raw(func(sourceRaw any) error {
		sourceNative, ok := sourceRaw.(*sqlite.SQLiteConn)
		if !ok {
			return fmt.Errorf("sqlite driver does not support the online backup API")
		}
		return destinationConn.Raw(func(destinationRaw any) error {
			destinationNative, ok := destinationRaw.(*sqlite.SQLiteConn)
			if !ok {
				return fmt.Errorf("sqlite driver does not support the online backup API")
			}
			backup, err := destinationNative.Backup("main", sourceNative, "main")
			if err != nil {
				return fmt.Errorf("start online backup: %w", err)
			}
			return copyBackupPages(copyCtx, backup, int(options.PagesPerStep), &result)
		})
	})
	_ = destinationConn.Close()
	closeErr := destination.Close()
	if rawErr == nil {
		rawErr = closeErr
	}
	if rawErr != nil {
		removeBackupFiles(backupPath)
		if errors.Is(rawErr, driver.ErrBadConn) {
			return OnlineBackupResult{}, fmt.Errorf("online backup connection failed: %w", rawErr)
		}
		return OnlineBackupResult{}, rawErr
	}

	// The copy inherits the source's journal mode, so a WAL database produces a
	// backup plus -wal and -shm sidecars. A backup that is three files is a
	// backup someone will restore two-thirds of, so it is consolidated into one
	// self-contained file here: checkpoint, then switch the *copy* to DELETE
	// journalling, which removes the sidecars. The live database is untouched,
	// and a restored copy returns to WAL when the app opens it.
	if err := consolidateBackupFile(ctx, backupPath); err != nil {
		removeBackupFiles(backupPath)
		return OnlineBackupResult{}, err
	}

	if err := EnforceSQLiteFilePermissions(sqliteFileURL(backupPath)); err != nil {
		removeBackupFiles(backupPath)
		return OnlineBackupResult{}, fmt.Errorf("set sqlite backup permissions: %w", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return OnlineBackupResult{}, fmt.Errorf("stat completed backup: %w", err)
	}
	result.ByteSize = info.Size()

	return result, nil
}

// backupPageCopier keeps the step loop independently testable, including the
// native API's BUSY/LOCKED result (false, nil) and DONE result (true, nil).
type backupPageCopier interface {
	Step(int) (bool, error)
	Remaining() int
	PageCount() int
	Finish() error
}

func copyBackupPages(ctx context.Context, backup backupPageCopier, pages int, result *OnlineBackupResult) (err error) {
	defer func() {
		if finishErr := backup.Finish(); finishErr != nil {
			err = errors.Join(err, fmt.Errorf("finish online backup: %w", finishErr))
		}
	}()
	for {
		if ctx.Err() != nil {
			return errors.Join(ErrOnlineBackupDeadline, ctx.Err())
		}
		before := backup.Remaining()
		done, stepErr := backup.Step(pages)
		if stepErr != nil {
			return fmt.Errorf("copy pages: %w", stepErr)
		}
		if done {
			result.PageCount = backup.PageCount()
			return nil
		}
		// A contended step makes no progress. Pause rather than spin while
		// retaining a bounded response to cancellation and the copy deadline.
		if backup.Remaining() >= before {
			timer := time.NewTimer(10 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.Join(ErrOnlineBackupDeadline, ctx.Err())
			case <-timer.C:
			}
		}
	}
}

func removeBackupFiles(path string) {
	for _, file := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		_ = os.Remove(file)
	}
}

// SyncFileAndParent flushes a file and the directory entry that names it.
// Without the second sync a crash can leave a rename that never happened, which
// is the difference between "the backup is there" and "the backup is there
// after a power cut".
func SyncFileAndParent(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open backup for sync: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync backup: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close backup after sync: %w", err)
	}

	return SyncDirectory(filepath.Dir(path))
}

// SyncDirectory flushes a directory's own entries.
func SyncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open backup directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync backup directory: %w", err)
	}
	return nil
}

// consolidateBackupFile leaves the backup as a single file with no sidecars.
func consolidateBackupFile(ctx context.Context, backupPath string) error {
	backupDatabase, err := sql.Open(driverName, sqliteAuxiliaryDSN(backupPath, false))
	if err != nil {
		return fmt.Errorf("open backup for consolidation: %w", err)
	}
	defer backupDatabase.Close()
	backupDatabase.SetMaxOpenConns(1)

	var busy, logPages, checkpointed int
	if err := backupDatabase.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logPages, &checkpointed); err != nil {
		return fmt.Errorf("checkpoint backup: %w", err)
	}

	var journalMode string
	if err := backupDatabase.QueryRowContext(ctx, "PRAGMA journal_mode=DELETE").Scan(&journalMode); err != nil {
		return fmt.Errorf("set backup journal mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "delete") {
		return fmt.Errorf("backup journal mode is %q, want delete", journalMode)
	}

	if err := backupDatabase.Close(); err != nil {
		return fmt.Errorf("close consolidated backup: %w", err)
	}

	// Whatever the close left behind is not part of the backup.
	for _, sidecar := range []string{backupPath + "-wal", backupPath + "-shm"} {
		if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove backup sidecar: %w", err)
		}
	}

	return nil
}
