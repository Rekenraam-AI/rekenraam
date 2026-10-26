package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

// serial: t.Setenv changes the process-wide temporary directory and app configuration.
func TestRecoveryAndRestoreHelpersEscapeTemporaryRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sqlite #?%23 space ü")
	require.NoError(t, os.Mkdir(root, 0o700))
	t.Setenv("TMPDIR", root)
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	t.Setenv("APP_ENV", "development")
	ctx := context.Background()

	t.Run("recovery", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			open func(*testing.T) string
			exit int
		}{
			{"migrated", func(t *testing.T) string { return setupRecoveryCommandDatabase(t, "old-password") }, 0},
			{"pre-migration", setupPreMigrationRecoveryDatabase, 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				databaseURL := tc.open(t)
				path, err := db.ResolveSQLiteFilePath(databaseURL)
				require.NoError(t, err)
				require.Contains(t, path, root+string(filepath.Separator))
				require.FileExists(t, path)
				t.Setenv("DATABASE_URL", databaseURL)
				backup := filepath.Join(t.TempDir(), "recovery #?%23.sqlite")
				stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
				require.Equal(t, tc.exit, run(ctx, []string{"recover-owner", "--password-stdin", "--backup-path", backup},
					strings.NewReader("new-password\n"), stdout, stderr), stderr.String())
				if tc.exit != 0 {
					assert.Contains(t, stderr.String(), "incompatible pre-release baseline")
				}
				reader := openSQLiteReadOnly(t, backup)
				t.Cleanup(func() { assert.NoError(t, reader.Close()) })
				assert.True(t, sqliteTableExists(t, reader, "users"))
				_, err = reader.ExecContext(ctx, "CREATE TABLE forbidden_write (id INTEGER)")
				require.ErrorContains(t, err, "readonly")
			})
		}
	})

	for _, name := range []string{"backup", "sealed-backup"} {
		t.Run(name, func(t *testing.T) {
			var databaseURL, backup string
			if name == "sealed-backup" {
				databaseURL, backup, _ = sealedBackupOf(t)
			} else {
				databaseURL, backup = backupOf(t)
			}
			path, err := db.ResolveSQLiteFilePath(databaseURL)
			require.NoError(t, err)
			require.Contains(t, path, root+string(filepath.Separator))
			require.FileExists(t, path)
			require.FileExists(t, backup)
			t.Setenv("DATABASE_URL", databaseURL)
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			require.Equal(t, 0, run(ctx, []string{"restore", "--from", backup},
				strings.NewReader(""), stdout, stderr), stderr.String())
			reader := openSQLiteReadOnly(t, path)
			t.Cleanup(func() { assert.NoError(t, reader.Close()) })
			var sealed int
			require.NoError(t, reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_mfa_totp").Scan(&sealed))
			if name == "sealed-backup" {
				assert.Equal(t, 1, sealed)
			} else {
				assert.Zero(t, sealed)
			}
			_, err = reader.ExecContext(ctx, "CREATE TABLE forbidden_write (id INTEGER)")
			require.ErrorContains(t, err, "readonly")
		})
	}
}
