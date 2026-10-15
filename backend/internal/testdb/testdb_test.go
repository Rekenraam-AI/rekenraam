package testdb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

func TestOpenReturnsIndependentMigratedCopies(t *testing.T) {
	assertMigratedFixturesIsolated(t)
}

func TestOpenEscapesFixturePaths(t *testing.T) {
	t.Parallel()

	// Go names the unnamed subtest #00, which t.TempDir preserves in its path.
	for _, name := range []string{"", "percent%23&equals=plus+"} {
		t.Run(name, assertMigratedFixturesIsolated)
	}
}

// serial: t.Setenv changes the process-wide temporary directory.
func TestTemplateAndFixturesEscapeTemporaryRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sqlite #?%23 space ü")
	require.NoError(t, os.Mkdir(root, 0o700))
	t.Setenv("TMPDIR", root)
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)

	// Exercise template construction even if another test populated the cache.
	data, err := buildTemplate()
	require.NoError(t, err)
	require.NotEmpty(t, data)
	t.Run("", assertMigratedFixturesIsolated)
}

func assertMigratedFixturesIsolated(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	first, firstURL := Open(t)
	second, secondURL := Open(t)
	assert.NotEqual(t, firstURL, secondURL)

	for _, database := range []*sql.DB{first, second} {
		var migrationCount int
		require.NoError(t, database.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM goose_db_version WHERE is_applied = 1").Scan(&migrationCount))
		assert.Positive(t, migrationCount)

		var users int
		require.NoError(t, database.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&users))
		assert.Zero(t, users, "each fixture starts with the migrated, unmodified template")
	}

	_, err := first.ExecContext(ctx, "CREATE TABLE fixture_marker (id INTEGER PRIMARY KEY)")
	require.NoError(t, err)
	var markers int
	require.NoError(t, second.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_schema WHERE name = 'fixture_marker'").Scan(&markers))
	assert.Zero(t, markers, "fixture writes must not leak into another copy")

	// Returned URLs must reopen the same file, not just a different migrated copy.
	reopened, err := db.Open(ctx, firstURL)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, reopened.Close()) })
	require.NoError(t, reopened.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_schema WHERE name = 'fixture_marker'").Scan(&markers))
	assert.Equal(t, 1, markers)
}
