package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/config"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/lockfile"
	"rekenraam/backend/internal/secretbox"
)

// backupOf makes a verified backup of a fresh migrated database and returns
// both paths.
func backupOf(t *testing.T) (databaseURL string, backupPath string) {
	t.Helper()

	ctx := context.Background()
	root := t.TempDir()
	databaseURL = db.SQLiteFileURL(filepath.Join(root, "rekenraam.sqlite"))

	database, err := db.Open(ctx, databaseURL)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx, database))

	readOnly, err := db.OpenReadOnly(ctx, databaseURL)
	require.NoError(t, err)
	backupPath = filepath.Join(root, "backups", "rekenraam-2026-08-24.sqlite")
	_, err = db.OnlineBackupSQLiteDatabase(ctx, readOnly, backupPath, db.OnlineBackupOptions{})
	require.NoError(t, err)
	require.NoError(t, readOnly.Close())
	require.NoError(t, database.Close())

	return databaseURL, backupPath
}

// verify-backup answers what a person asks before trusting a file, including
// the question a restore only raises once it is too late.
func TestVerifyBackupCommandReportsIntegritySchemaAndSealedData(t *testing.T) {
	databaseURL, backupPath := backupOf(t)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"verify-backup", "--from", backupPath}, strings.NewReader(""), stdout, stderr)

	require.Equalf(t, 0, exitCode, "stderr: %s", stderr.String())
	output := stdout.String()
	assert.Contains(t, output, "integrity:       ok")
	assert.Contains(t, output, "schema version:")
	assert.Contains(t, output, "rows transactions:")
	assert.Contains(t, output, "sealed data:     none",
		"a book with no sealed rows should say so rather than warn about a key it does not need")
}

func TestVerifyBackupCommandRejectsAnUnusableFile(t *testing.T) {
	root := t.TempDir()
	notABackup := filepath.Join(root, "notes.txt")
	require.NoError(t, os.WriteFile(notABackup, []byte("this is not a database"), 0o600))
	t.Setenv("DATABASE_URL", db.SQLiteFileURL(filepath.Join(root, "rekenraam.sqlite")))
	t.Setenv("APP_ENV", "development")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"verify-backup", "--from", notABackup}, strings.NewReader(""), stdout, stderr)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "backup is not usable")
}

// The restore refuses while a server holds the lock, and says who holds it.
func TestRestoreCommandRefusesWhileTheServerIsRunning(t *testing.T) {
	databaseURL, backupPath := backupOf(t)
	databasePath, err := db.ResolveSQLiteFilePath(databaseURL)
	require.NoError(t, err)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")

	held, err := lockfile.Acquire(databasePath)
	require.NoError(t, err)
	defer held.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"restore", "--from", backupPath}, strings.NewReader(""), stdout, stderr)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "the database is in use")
	assert.Contains(t, stderr.String(), "Stop the server or wait for the other restore to finish")
}

// With the server stopped, the restore installs the backup, keeps the previous
// database whole, and says both things plainly.
func TestRestoreCommandInstallsAndPreservesThePreviousDatabase(t *testing.T) {
	databaseURL, backupPath := backupOf(t)
	databasePath, err := db.ResolveSQLiteFilePath(databaseURL)
	require.NoError(t, err)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"restore", "--from", backupPath}, strings.NewReader(""), stdout, stderr)

	require.Equalf(t, 0, exitCode, "stderr: %s", stderr.String())
	output := stdout.String()
	assert.Contains(t, output, "restored:")
	assert.Contains(t, output, "previous data:")
	assert.Contains(t, output, "attachments:     nothing to restore")
	assert.Contains(t, output, "secret key:")

	assert.FileExists(t, databasePath)
	entries, err := os.ReadDir(filepath.Dir(databasePath))
	require.NoError(t, err)
	var preserved int
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".before-restore-") {
			preserved++
		}
	}
	assert.Equal(t, 1, preserved, "the previous database is kept until a human deletes it")

	// The lock is free afterwards, so the server can start again.
	assert.NoError(t, lockfile.CheckAvailable(databasePath))
}

func TestRestoreCommandRequiresAFromFlag(t *testing.T) {
	databaseURL, _ := backupOf(t)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"restore"}, strings.NewReader(""), stdout, stderr)

	assert.Equal(t, 2, exitCode)
	assert.Contains(t, stderr.String(), "restore requires --from")
}

// --- T-68: the sealed-data report, through the command an operator runs ---
//
// The branch that matters is the one where a key is configured and a sample
// either opens or does not — the question a restore otherwise raises far too
// late. It was covered at service level and never through verify-backup itself.

// sealedBackupOf returns a backup containing one sealed row, plus the key it
// was sealed with.
func sealedBackupOf(t *testing.T) (databaseURL string, backupPath string, key string) {
	t.Helper()

	ctx := context.Background()
	root := t.TempDir()
	databaseURL = db.SQLiteFileURL(filepath.Join(root, "rekenraam.sqlite"))

	database, err := db.Open(ctx, databaseURL)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx, database))

	key = "0123456789abcdef0123456789abcdef"
	box, err := secretbox.New([]byte(key))
	require.NoError(t, err)
	sealed, err := box.Seal([]byte("JBSWY3DPEHPK3PXP"))
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, is_owner, created_at, updated_at)
		VALUES (1, 'owner', 'x', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
		INSERT INTO user_mfa_totp (user_id, secret_ciphertext, status, created_at)
		VALUES (1, ?, 'active', '2026-01-01T00:00:00Z');
	`, sealed)
	require.NoError(t, err)

	readOnly, err := db.OpenReadOnly(ctx, databaseURL)
	require.NoError(t, err)
	backupPath = filepath.Join(root, "backups", "rekenraam-2026-08-24.sqlite")
	_, err = db.OnlineBackupSQLiteDatabase(ctx, readOnly, backupPath, db.OnlineBackupOptions{})
	require.NoError(t, err)
	require.NoError(t, readOnly.Close())
	require.NoError(t, database.Close())

	return databaseURL, backupPath, key
}

func TestVerifyBackupReportsSealedDataOpeningWithTheRetainedKey(t *testing.T) {
	databaseURL, backupPath, key := sealedBackupOf(t)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")
	t.Setenv("REKENRAAM_SECRET_KEY", base64.StdEncoding.EncodeToString([]byte(key)))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"verify-backup", "--from", backupPath}, strings.NewReader(""), stdout, stderr)

	require.Equalf(t, 0, exitCode, "stderr: %s", stderr.String())
	output := stdout.String()
	assert.Contains(t, output, "sealed rows user_mfa_totp:")
	assert.Contains(t, output, "decrypts with the configured REKENRAAM_SECRET_KEY")
}

// The case that matters most: the operator kept a key, but not *the* key.
// Silence here would be the worst possible answer.
func TestVerifyBackupSaysSoWhenTheConfiguredKeyIsTheWrongOne(t *testing.T) {
	databaseURL, backupPath, _ := sealedBackupOf(t)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")
	t.Setenv("REKENRAAM_SECRET_KEY",
		base64.StdEncoding.EncodeToString([]byte("fedcba9876543210fedcba9876543210")))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"verify-backup", "--from", backupPath}, strings.NewReader(""), stdout, stderr)

	require.Equalf(t, 0, exitCode, "a wrong sealed-data key must warn without failing a restorable backup: %s", stderr.String())
	assert.Contains(t, stdout.String(), "does NOT decrypt with the configured key",
		"a key that cannot open the data must be reported, not passed over")
}

// No key configured: the report says what a restore would cost rather than
// staying quiet about it.
func TestVerifyBackupExplainsTheCostWhenNoKeyIsConfigured(t *testing.T) {
	databaseURL, backupPath, _ := sealedBackupOf(t)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")
	t.Setenv("REKENRAAM_SECRET_KEY", "")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"verify-backup", "--from", backupPath}, strings.NewReader(""), stdout, stderr)

	require.Equalf(t, 0, exitCode, "a missing sealed-data key must warn without failing a restorable backup: %s", stderr.String())
	output := stdout.String()
	assert.Contains(t, output, "is not set here")
	assert.Contains(t, output, "would have to be set up again")
}

// A restore that fails after moving the previous database aside leaves nothing
// at the database path. What the operator is told at that moment decides
// whether their data is recoverable or merely present somewhere they will not
// look, so the message has to state which of the three states this is.
func TestRestoreCommandSaysWhereTheDataIsWhenItFailsPartWay(t *testing.T) {
	databaseURL, backupPath := backupOf(t)
	databasePath, err := db.ResolveSQLiteFilePath(databaseURL)
	require.NoError(t, err)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")

	// A directory where the install wants to write its temporary file: the copy
	// fails, and it fails *after* the preserve step, which is the state worth
	// describing.
	require.NoError(t, os.MkdirAll(databasePath+".restoring", 0o700))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"restore", "--from", backupPath}, strings.NewReader(""), stdout, stderr)

	require.Equal(t, 1, exitCode)
	output := stderr.String()
	assert.Contains(t, output, "restore failed:")
	assert.Contains(t, output, "nothing is at "+databasePath)
	assert.Contains(t, output, "move those files beside the database path")
	assert.NotContains(t, output, "nothing was replaced")

	// And the claim is true: the files really are in the preserved directory.
	entries, err := os.ReadDir(filepath.Dir(databasePath))
	require.NoError(t, err)
	var preservedDir string
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".before-restore-") {
			preservedDir = filepath.Join(filepath.Dir(databasePath), entry.Name())
		}
	}
	require.NotEmpty(t, preservedDir, "the previous database must be somewhere")
	preserved, err := os.ReadDir(preservedDir)
	require.NoError(t, err)
	assert.NotEmpty(t, preserved)
	assert.NoFileExists(t, databasePath)
}

// The other side of the same decision: a failure before anything moved must not
// send the operator looking for a preserved copy that does not exist.
func TestRestoreCommandSaysNothingWasReplacedWhenItRefusesEarly(t *testing.T) {
	databaseURL, _ := backupOf(t)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")

	notABackup := filepath.Join(t.TempDir(), "not-a-database.sqlite")
	require.NoError(t, os.WriteFile(notABackup, []byte("this is not a database"), 0o600))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"restore", "--from", notABackup}, strings.NewReader(""), stdout, stderr)

	require.Equal(t, 1, exitCode)
	assert.Contains(t, stderr.String(), "nothing was replaced; the database is untouched.")
}

func TestRestoreCommandReleasesLockOnFailure(t *testing.T) {
	for _, name := range []string{"unusable_backup", "source_equals_destination", "newer_schema", "incompatible_baseline", "cancelled", "after_preserving", "after_installing"} {
		t.Run(name, func(t *testing.T) {
			databaseURL, backupPath := backupOf(t)
			path, err := db.ResolveSQLiteFilePath(databaseURL)
			require.NoError(t, err)
			before, err := os.ReadFile(path)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch name {
			case "unusable_backup":
				backupPath = filepath.Join(t.TempDir(), "missing.sqlite")
			case "source_equals_destination":
				backupPath = path
			case "newer_schema":
				backup, err := db.Open(ctx, db.SQLiteFileURL(backupPath))
				require.NoError(t, err)
				_, err = backup.ExecContext(ctx, `UPDATE goose_db_version SET version_id = 9999 WHERE version_id = (SELECT MAX(version_id) FROM goose_db_version)`)
				require.NoError(t, err)
				require.NoError(t, backup.Close())
			case "incompatible_baseline":
				backup, err := db.Open(ctx, db.SQLiteFileURL(backupPath))
				require.NoError(t, err)
				_, err = backup.ExecContext(ctx, `PRAGMA user_version=0`)
				require.NoError(t, err)
				require.NoError(t, backup.Close())
			case "cancelled":
				cancel()
			case "after_preserving":
				require.NoError(t, os.Mkdir(path+".restoring", 0o700))
			}
			var stdout, stderr bytes.Buffer
			code := runRestoreWith(ctx, config.Config{DatabaseURL: databaseURL}, []string{"--from", backupPath}, &stdout, &stderr,
				func(ctx context.Context, source, destination string) (db.RestoreResult, error) {
					unexpected, err := lockfile.Acquire(path)
					var locked lockfile.ErrLocked
					assert.ErrorAs(t, err, &locked, "restore must own the lock even on its failure path")
					if unexpected != nil {
						defer unexpected.Close()
					}
					result, err := db.RestoreSQLiteDatabase(ctx, source, destination)
					if name == "after_installing" {
						require.NoError(t, err)
						require.True(t, result.Installed)
						return result, errors.New("injected post-install failure")
					}
					return result, err
				})
			require.Equalf(t, 1, code, "stderr: %s", stderr.String())
			if name == "incompatible_baseline" {
				assert.Contains(t, stderr.String(), "incompatible pre-release baseline")
			}

			assert.Empty(t, stdout.String())
			require.NoError(t, lockfile.CheckAvailable(path), "every error path must release the lock")
			info, err := os.Stat(lockfile.Path(path))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			preserved, err := filepath.Glob(path + ".before-restore-*")
			require.NoError(t, err)
			switch name {
			case "after_preserving", "after_installing":
				require.Len(t, preserved, 1)
				inspection, err := db.InspectBackup(context.Background(), filepath.Join(preserved[0], filepath.Base(path)))
				require.NoError(t, err)
				assert.True(t, inspection.IntegrityOK, "the prior database remains usable")
				if name == "after_preserving" {
					assert.NoFileExists(t, path)
					assert.Contains(t, stderr.String(), "nothing is at "+path)
					assert.Contains(t, stderr.String(), "move those files beside the database path")
				} else {
					assert.FileExists(t, path)
					assert.Contains(t, stderr.String(), "the restored database is already installed")
					assert.NotContains(t, stderr.String(), "move those files beside the database path")
				}
			default:
				assert.Empty(t, preserved)
				after, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.True(t, bytes.Equal(before, after), "early refusal leaves the database intact")
				assert.Contains(t, stderr.String(), "nothing was replaced; the database is untouched")
			}
		})
	}
}

// A restore killed between preserving and installing (power loss, SIGKILL)
// cannot report anything. It leaves the previous database whole in its
// preserved directory, a partial .restoring file and nothing at the database
// path. Running the same restore again must install the backup, ignore the
// stale partial file, and leave the first preserved copy untouched (#277).
func TestRestoreCommandRerunsCleanlyAfterAnInterruptedReplacement(t *testing.T) {
	databaseURL, backupPath := backupOf(t)
	databasePath, err := db.ResolveSQLiteFilePath(databaseURL)
	require.NoError(t, err)
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("APP_ENV", "development")

	// The state the interrupted run left behind.
	interrupted := databasePath + ".before-restore-20261018T000000Z"
	require.NoError(t, os.Mkdir(interrupted, 0o700))
	require.NoError(t, os.Rename(databasePath, filepath.Join(interrupted, filepath.Base(databasePath))))
	require.NoError(t, os.WriteFile(databasePath+".restoring", []byte("half a database"), 0o600))
	preservedBefore, err := os.ReadFile(filepath.Join(interrupted, filepath.Base(databasePath)))
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := run(context.Background(), []string{"restore", "--from", backupPath}, strings.NewReader(""), stdout, stderr)
	require.Equalf(t, 0, exitCode, "stderr: %s", stderr.String())
	assert.NotContains(t, stdout.String(), "previous data:", "there was nothing at the path to preserve")

	inspection, err := db.InspectBackup(context.Background(), databasePath)
	require.NoError(t, err)
	assert.True(t, inspection.IntegrityOK)
	assert.NoFileExists(t, databasePath+".restoring")
	preservedAfter, err := os.ReadFile(filepath.Join(interrupted, filepath.Base(databasePath)))
	require.NoError(t, err)
	assert.True(t, bytes.Equal(preservedBefore, preservedAfter), "the first preserved copy is never touched")
	others, err := filepath.Glob(databasePath + ".before-restore-*")
	require.NoError(t, err)
	assert.Equal(t, []string{interrupted}, others)
	require.NoError(t, lockfile.CheckAvailable(databasePath))
}

func TestVerifyBackupReportsIncompatibleBaselineSeparatelyFromNewerSchema(t *testing.T) {
	t.Parallel()
	databaseURL, backupPath := backupOf(t)
	backup, err := db.Open(context.Background(), db.SQLiteFileURL(backupPath))
	require.NoError(t, err)
	_, err = backup.Exec(`PRAGMA user_version=0; UPDATE goose_db_version SET version_id=27 WHERE version_id=1`)
	require.NoError(t, err)
	require.NoError(t, backup.Close())
	var stdout, stderr bytes.Buffer
	code := runVerifyBackup(context.Background(), config.Config{DatabaseURL: databaseURL}, []string{"--from", backupPath}, &stdout, &stderr)
	require.Equalf(t, 3, code, "a backup with an incompatible baseline must not look verified: %s", stderr.String())
	require.Contains(t, stdout.String(), "integrity:       ok")
	require.Contains(t, stdout.String(), "incompatible pre-release baseline, cannot restore")
	require.NotContains(t, stdout.String(), "NEWER")
}

// A newer forward schema is also intact but unrestorable, so it must reach the
// same distinct status rather than exit 0 after printing the warning.
func TestVerifyBackupReportsNewerSchemaWithAnUnrestorableExitStatus(t *testing.T) {
	t.Parallel()
	databaseURL, backupPath := backupOf(t)
	backup, err := db.Open(context.Background(), db.SQLiteFileURL(backupPath))
	require.NoError(t, err)
	_, err = backup.Exec(`UPDATE goose_db_version SET version_id = 9999 WHERE version_id = (SELECT MAX(version_id) FROM goose_db_version)`)
	require.NoError(t, err)
	require.NoError(t, backup.Close())
	var stdout, stderr bytes.Buffer
	code := runVerifyBackup(context.Background(), config.Config{DatabaseURL: databaseURL}, []string{"--from", backupPath}, &stdout, &stderr)
	require.Equalf(t, 3, code, "a backup newer than this build must not look verified: %s", stderr.String())
	require.Contains(t, stdout.String(), "integrity:       ok")
	require.Contains(t, stdout.String(), "this backup is NEWER than this build and cannot be restored by it")
	require.NotContains(t, stdout.String(), "incompatible pre-release baseline")
}
