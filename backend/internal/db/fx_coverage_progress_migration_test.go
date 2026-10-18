package db

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/migrations"
)

func TestFXCoverageProgressMigrationPreservesQueuedWork(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, _ := newMFATestRepository(t)
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 17)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO books(id,code,name,owner_user_id,created_at,updated_at,updated_by_user_id)
	 VALUES(1,'personal','Personal',1,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z',1)`)
	require.NoError(t, err)
	r := NewBackgroundWorkRepository(database)
	const payload = `{"reason":"transaction_entered","start_date":"2026-01-01","currency_id":2}`
	item, err := r.EnqueueBackgroundWork(ctx, 1, "pricing.fx_coverage", payload, "2026-01-01T00:00:00Z")
	require.NoError(t, err)
	readTrigger := func() string {
		var definition string
		require.NoError(t, database.QueryRow(`SELECT sql FROM sqlite_master WHERE type='trigger' AND name='fx_work_after_posting_version_insert'`).Scan(&definition))
		return definition
	}
	before := readTrigger()
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	assert.Contains(t, readTrigger(), "bw.attempts = 0")
	assert.Contains(t, readTrigger(), "'$.next') IS NULL")
	assert.Contains(t, readTrigger(), "'$.retry') IS NULL")
	current, err := r.BackgroundWorkByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, item, current, "migration must not reinterpret or discard existing work")
	_, err = provider.DownTo(ctx, 17)
	require.NoError(t, err)
	assert.Equal(t, before, readTrigger())
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	current, err = r.BackgroundWorkByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, item, current)
}
