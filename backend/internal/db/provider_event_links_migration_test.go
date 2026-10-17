package db

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/migrations"
)

// Migration 0011 (#111) rebuilds investment_event_suggestions for links. An
// upgrade keeps every existing suggestion exactly; a downgrade refuses to drop
// an owner's link evidence.
func TestProviderEventLinkMigrationPreservesSuggestionsAndRefusesLossyDowngrade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, _, _ := migratedInvestmentTestDatabase(t)
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 10)
	require.NoError(t, err)

	var sourceID int64
	require.NoError(t, database.QueryRowContext(ctx, `SELECT id FROM market_data_sources WHERE code = 'manual'`).Scan(&sourceID))
	insertEvent := func(identity string) int64 {
		result, err := database.ExecContext(ctx, `INSERT INTO investment_provider_events (book_id, source_id, provider_event_id, event_family, event_date, status, created_at)
			VALUES (1, ?, ?, 'dividend', '2026-06-01', 'new', '2026-06-01T00:00:00Z')`, sourceID, identity)
		require.NoError(t, err)
		id, err := result.LastInsertId()
		require.NoError(t, err)
		return id
	}
	legacyEvent := insertEvent("legacy")
	_, err = database.ExecContext(ctx, `INSERT INTO investment_event_suggestions (id, book_id, provider_event_id, confidence_bps, status, proposed_transaction_json, failure_reason, created_at, updated_at)
		VALUES (41, 1, ?, 9000, 'failed', '{"kind":"split"}', 'unsupported', '2026-06-01T00:00:00Z', '2026-06-02T00:00:00Z')`, legacyEvent)
	require.NoError(t, err)

	_, err = provider.Up(ctx)
	require.NoError(t, err)
	var status, proposal, reason, updated string
	var linked any
	require.NoError(t, database.QueryRowContext(ctx, `SELECT status, proposed_transaction_json, failure_reason, updated_at, linked_transaction_id FROM investment_event_suggestions WHERE id = 41`).
		Scan(&status, &proposal, &reason, &updated, &linked))
	assert.Equal(t, []any{"failed", `{"kind":"split"}`, "unsupported", "2026-06-02T00:00:00Z", nil}, []any{status, proposal, reason, updated, linked})

	_, err = database.ExecContext(ctx, `INSERT INTO investment_event_suggestions (book_id, provider_event_id, confidence_bps, status, created_at, updated_at)
		VALUES (1, ?, 0, 'suggested', '2026-06-01T00:00:00Z', '2026-06-01T00:00:00Z')`, legacyEvent)
	require.Error(t, err, "one suggestion per provider event")
	_, err = database.ExecContext(ctx, `INSERT INTO investment_event_suggestions (book_id, provider_event_id, confidence_bps, status, created_at, updated_at)
		VALUES (1, ?, 0, 'linked', '2026-06-01T00:00:00Z', '2026-06-01T00:00:00Z')`, insertEvent("unlinked"))
	require.Error(t, err, "a linked suggestion names its transaction")

	// A link survives a refused downgrade untouched. The blank book has no
	// transaction, so the fixture names one with foreign keys briefly off.
	conn, err := database.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `UPDATE investment_event_suggestions SET status = 'linked', linked_transaction_id = 999, failure_reason = '' WHERE id = 41`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	_, err = provider.DownTo(ctx, 10)
	require.ErrorContains(t, err, "CHECK constraint failed")
	var count int
	require.NoError(t, database.QueryRowContext(ctx, `SELECT count(*) FROM investment_event_suggestions WHERE status = 'linked' AND linked_transaction_id = 999`).Scan(&count))
	assert.Equal(t, 1, count)
}
