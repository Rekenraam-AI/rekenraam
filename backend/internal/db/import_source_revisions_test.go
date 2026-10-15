package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSourceRevisionCheckPreservesDatabaseErrors(t *testing.T) {
	t.Parallel()
	database := openTestDatabase(t)
	require.NoError(t, database.Close())
	err := NewImportRepository(database).CheckSourceRevision(context.Background(), CommitImportSourceRevisionParams{
		BookID: 1, IdentityID: 1, StagedRowID: 2, SourceOperationID: 1,
	})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrImportSourceRevisionConflict)
	require.Contains(t, err.Error(), "database is closed")
}

func TestUpdateImportBatchStatusRequiresOccurredAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := newRecurringTestDatabase(t)
	result, err := database.ExecContext(ctx, `
		INSERT INTO import_batches (book_id, source_kind, status, created_at)
		VALUES (1, 'csv', 'previewing', '2026-10-01T00:00:00Z')
	`)
	require.NoError(t, err)
	batchID, err := result.LastInsertId()
	require.NoError(t, err)

	repository := NewImportRepository(database)
	err = repository.UpdateImportBatchStatus(ctx, UpdateImportBatchStatusParams{
		BatchID: batchID, Status: "discarded", EventKind: "discarded",
	})
	require.ErrorContains(t, err, "occurred_at is required")

	var status string
	var events int
	require.NoError(t, database.QueryRowContext(ctx, `SELECT status FROM import_batches WHERE id = ?`, batchID).Scan(&status))
	require.NoError(t, database.QueryRowContext(ctx, `SELECT COUNT(*) FROM import_batch_events WHERE batch_id = ?`, batchID).Scan(&events))
	require.Equal(t, "previewing", status)
	require.Zero(t, events)

	// The event is stamped with the caller's time, not the batch's creation.
	require.NoError(t, repository.UpdateImportBatchStatus(ctx, UpdateImportBatchStatusParams{
		BatchID: batchID, Status: "discarded", EventKind: "discarded", OccurredAt: "2026-10-15T12:00:00Z",
	}))
	var createdAt string
	require.NoError(t, database.QueryRowContext(ctx,
		`SELECT created_at FROM import_batch_events WHERE batch_id = ?`, batchID).Scan(&createdAt))
	require.Equal(t, "2026-10-15T12:00:00Z", createdAt)
}
