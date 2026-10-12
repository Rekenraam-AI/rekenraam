package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeOnlineBackupContentionCancelsAndCleansUp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	source := openTestDatabase(t)
	_, err := source.ExecContext(ctx, "CREATE TABLE busy_backup(id INTEGER PRIMARY KEY); INSERT INTO busy_backup VALUES(1); BEGIN IMMEDIATE; INSERT INTO busy_backup VALUES(2)")
	require.NoError(t, err)
	// An active write on the source connection makes sqlite3_backup_step return
	// BUSY. Using that deliberately exercises the real native false/nil result.
	path := filepath.Join(t.TempDir(), "failed.sqlite")
	copyCtx, cancel := context.WithTimeout(ctx, 40*time.Millisecond)
	defer cancel()
	_, err = OnlineBackupSQLiteDatabase(copyCtx, source, path, OnlineBackupOptions{PagesPerStep: 1, Deadline: time.Second})
	require.ErrorIs(t, err, ErrOnlineBackupDeadline)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		_, err := os.Stat(path + suffix)
		assert.True(t, os.IsNotExist(err), suffix)
	}
	_, err = source.ExecContext(ctx, "ROLLBACK; INSERT INTO busy_backup VALUES(3)")
	require.NoError(t, err, "the failed copy must leave its source usable")
	result, err := OnlineBackupSQLiteDatabase(ctx, source, path, OnlineBackupOptions{PagesPerStep: 1})
	require.NoError(t, err)
	assert.Positive(t, result.PageCount)
	require.NoError(t, VerifySQLiteBackup(ctx, path))
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		_, err := os.Stat(path + suffix)
		assert.True(t, os.IsNotExist(err), suffix)
	}
}

type scriptedBackup struct {
	steps, finishes    int
	remaining          int
	doneAfter          int
	stepErr, finishErr error
}

func (b *scriptedBackup) Step(int) (bool, error) {
	b.steps++
	return b.doneAfter > 0 && b.steps >= b.doneAfter, b.stepErr
}
func (b *scriptedBackup) Remaining() int { return b.remaining }
func (b *scriptedBackup) PageCount() int { return 7 }
func (b *scriptedBackup) Finish() error  { b.finishes++; return b.finishErr }

func TestNativeBackupStepCompletionAndFinishErrors(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"done", "step-error", "finish-error", "canceled", "deadline"} {
		t.Run(name, func(t *testing.T) {
			backup := &scriptedBackup{doneAfter: 1}
			ctx := context.Background()
			var wanted error
			switch name {
			case "step-error":
				wanted = errors.New("step failed")
				backup.stepErr = wanted
			case "finish-error":
				wanted = errors.New("finish failed")
				backup.finishErr = wanted
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				wanted = context.Canceled
			case "deadline":
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 35*time.Millisecond)
				defer cancel()
				backup.doneAfter = 0
				wanted = context.DeadlineExceeded
			}
			var result OnlineBackupResult
			err := copyBackupPages(ctx, backup, 1, &result)
			if wanted == nil {
				require.NoError(t, err)
				assert.Equal(t, 7, result.PageCount)
				assert.Equal(t, 1, backup.steps, "DONE is true, so no second step is allowed")
			} else {
				require.ErrorIs(t, err, wanted)
			}
			assert.Equal(t, 1, backup.finishes, "Finish releases the native handle exactly once")
			if name == "deadline" {
				assert.LessOrEqual(t, backup.steps, 5, "BUSY/LOCKED must wait between attempts")
			}
		})
	}
}
