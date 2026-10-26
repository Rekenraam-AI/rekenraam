package db

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tool consumer for scripts/build-release-seed.sh. Installed fixtures remain
// frozen; this validates a proposed new fixture through the actual baseline.
func TestValidateReleaseSeedFixture(t *testing.T) {
	t.Parallel()
	path := os.Getenv("SEED_INPUT")
	if path == "" {
		t.Skip("SEED_INPUT is unset; invoked by scripts/build-release-seed.sh")
	}
	seed, err := os.ReadFile(path)
	require.NoError(t, err)
	database := openTestDatabase(t)
	require.NoError(t, Migrate(context.Background(), database))
	tx, err := database.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer rollbackTx(context.Background(), tx)
	_, err = tx.Exec(`PRAGMA defer_foreign_keys=ON`)
	require.NoError(t, err)
	_, err = tx.Exec(string(seed))
	require.NoError(t, err, "generated fixture must satisfy real admission triggers")
	require.NoError(t, tx.Commit(), "generated fixture must satisfy every foreign key")
	before := captureLedgerState(t, database)
	if sourcePath := os.Getenv("SEED_SOURCE"); sourcePath != "" {
		source, err := OpenReadOnly(context.Background(), SQLiteFileURL(sourcePath))
		require.NoError(t, err)
		defer source.Close()
		require.Equal(t, captureLedgerState(t, source), before,
			"generated SQL must preserve source row counts and exact financial facts, without trigger-created side effects")
	}
	require.NoError(t, Migrate(context.Background(), database))
	require.Equal(t, before, captureLedgerState(t, database))
	assertUpgradedBookIsIntact(t, database)
}
