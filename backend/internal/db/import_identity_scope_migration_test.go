package db

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/migrations"
)

// Migration 0016 (#302) adds the source fingerprint an account-scoped import
// identity was derived from. It downgrades and re-applies on a book with no
// account-scoped identity.
func TestImportIdentityScopeMigrationRoundTrips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, _ := newMFATestRepository(t)
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 15)
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, `SELECT source_fingerprint FROM import_commit_identities`)
	require.Error(t, err, "0015 has no source fingerprint")
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, `SELECT source_fingerprint FROM import_commit_identities`)
	require.NoError(t, err)
}
