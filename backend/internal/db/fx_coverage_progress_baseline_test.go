package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFXCoverageBaselinePreservesQueuedWork(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, _ := newMFATestRepository(t)
	_, err := database.Exec(`INSERT INTO books(id,code,name,owner_user_id,created_at,updated_at,updated_by_user_id)
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
	require.NoError(t, Migrate(ctx, database))
	assert.Contains(t, readTrigger(), "bw.attempts = 0")
	assert.Contains(t, readTrigger(), "'$.next') IS NULL")
	assert.Contains(t, readTrigger(), "'$.retry') IS NULL")
	current, err := r.BackgroundWorkByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, item, current, "migration must not reinterpret or discard existing work")
	assert.Equal(t, before, readTrigger())
	current, err = r.BackgroundWorkByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, item, current)
}
