package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serial: t.Setenv changes the process-wide temporary directory.
func TestFreshDatabaseHelpersEscapeTemporaryRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sqlite #?%23 space ü")
	require.NoError(t, os.Mkdir(root, 0o700))
	t.Setenv("TMPDIR", root)
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		open func(*testing.T) *sql.DB
	}{
		{"fresh", func(t *testing.T) *sql.DB {
			database := openTestDatabase(t)
			require.NoError(t, Migrate(ctx, database))
			return database
		}},
		{"recurring", func(t *testing.T) *sql.DB { return newRecurringTestDatabase(t) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, second := tc.open(t), tc.open(t)
			var paths []string
			for _, database := range []*sql.DB{first, second} {
				var path string
				require.NoError(t, database.QueryRowContext(ctx,
					"SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&path))
				require.Contains(t, path, root+string(filepath.Separator))
				require.FileExists(t, path)
				paths = append(paths, path)
				var migrations int
				require.NoError(t, database.QueryRowContext(ctx,
					"SELECT COUNT(*) FROM goose_db_version WHERE is_applied = 1").Scan(&migrations))
				assert.Positive(t, migrations)
			}
			assert.NotEqual(t, paths[0], paths[1])
			_, err := first.ExecContext(ctx, "CREATE TABLE fixture_marker (id INTEGER PRIMARY KEY)")
			require.NoError(t, err)
			assert.False(t, sqliteObjectExists(t, second, "table", "fixture_marker"))
		})
	}
}
