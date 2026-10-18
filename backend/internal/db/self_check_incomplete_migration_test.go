package db

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/migrations"
)

// Migration 0017 (#313) rebuilds both self-check tables to admit an
// incomplete verdict. Historical runs keep their recorded verdicts, counts,
// samples and summaries — a pre-#313 failed run over unknown basis stays
// failed — and the rebuilt child still cascades from its renamed parent.
func TestSelfCheckIncompleteMigrationPreservesHistoricalRuns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, _ := newMFATestRepository(t)
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 16)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO books (id, code, name, owner_user_id, created_at, updated_at, updated_by_user_id)
		VALUES (1, 'personal', 'Personal', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 1);
		INSERT INTO self_check_runs (id, book_id, trigger, status, failed_check_count, error_summary, started_at, finished_at, created_at)
		VALUES (7, 1, 'scheduled', 'failed', 1, '', '2026-10-01T00:00:00Z', '2026-10-01T00:00:01Z', '2026-10-01T00:00:00Z'),
		       (8, 1, 'manual', 'errored', 0, 'disk full', '2026-10-02T00:00:00Z', '2026-10-02T00:00:01Z', '2026-10-02T00:00:00Z'),
		       (9, 1, 'manual', 'running', 0, '', '2026-10-03T00:00:00Z', NULL, '2026-10-03T00:00:00Z');
		INSERT INTO self_check_results (id, run_id, check_id, status, finding_count, sample_json, summary)
		VALUES (70, 7, 'lot_reconciliation', 'failed', 1, '[42]', '1 lots have unknown projected basis; basis reconciliation is unavailable'),
		       (71, 7, 'attachments', 'not_applicable', 0, '[]', 'not implemented');
	`)
	require.NoError(t, err)

	_, err = provider.Up(ctx)
	require.NoError(t, err)

	type runRow struct {
		status, errorSummary string
		failed, incomplete   int64
		finished             *string
	}
	readRun := func(id int64) runRow {
		var row runRow
		require.NoError(t, database.QueryRowContext(ctx, `
			SELECT status, error_summary, failed_check_count, incomplete_check_count, finished_at
			FROM self_check_runs WHERE id = ?`, id).
			Scan(&row.status, &row.errorSummary, &row.failed, &row.incomplete, &row.finished))
		return row
	}
	require.Equal(t, "failed", readRun(7).status, "an old verdict is history, never restated")
	require.EqualValues(t, 1, readRun(7).failed)
	require.EqualValues(t, 0, readRun(7).incomplete)
	require.Equal(t, "errored", readRun(8).status)
	require.Equal(t, "disk full", readRun(8).errorSummary)
	require.Equal(t, "running", readRun(9).status)
	require.Nil(t, readRun(9).finished)

	var status, sample, unavailableSample, summary string
	var findings, unavailable int64
	require.NoError(t, database.QueryRowContext(ctx, `
		SELECT status, finding_count, sample_json, unavailable_count, unavailable_sample_json, summary
		FROM self_check_results WHERE id = 70`).
		Scan(&status, &findings, &sample, &unavailable, &unavailableSample, &summary))
	require.Equal(t, "failed", status)
	require.EqualValues(t, 1, findings)
	require.Equal(t, "[42]", sample)
	require.EqualValues(t, 0, unavailable)
	require.Equal(t, "[]", unavailableSample)
	require.Equal(t, "1 lots have unknown projected basis; basis reconciliation is unavailable", summary)

	var parent string
	require.NoError(t, database.QueryRowContext(ctx,
		`SELECT "table" FROM pragma_foreign_key_list('self_check_results')`).Scan(&parent))
	require.Equal(t, "self_check_runs", parent, "renaming the rebuilt parent rewrites the child's reference")

	// The new states are representable, and contradictory ones are refused.
	_, err = database.ExecContext(ctx, `
		UPDATE self_check_runs SET status = 'incomplete', incomplete_check_count = 1, finished_at = '2026-10-03T00:00:01Z' WHERE id = 9;
		INSERT INTO self_check_results (run_id, check_id, status, finding_count, unavailable_count, unavailable_sample_json)
		VALUES (9, 'lot_reconciliation', 'incomplete', 0, 2, '[3, 4]');
	`)
	require.NoError(t, err)
	for name, statement := range map[string]string{
		"incomplete result with a finding":           `INSERT INTO self_check_results (run_id, check_id, status, finding_count, unavailable_count) VALUES (9, 'entry_balance', 'incomplete', 1, 1)`,
		"incomplete result with nothing unavailable": `INSERT INTO self_check_results (run_id, check_id, status, finding_count, unavailable_count) VALUES (9, 'entry_balance', 'incomplete', 0, 0)`,
		"passed result hiding unavailable coverage":  `INSERT INTO self_check_results (run_id, check_id, status, finding_count, unavailable_count) VALUES (9, 'entry_balance', 'passed', 0, 1)`,
		"incomplete run with a failed check":         `UPDATE self_check_runs SET failed_check_count = 1 WHERE id = 9`,
		"passed run with incomplete coverage":        `UPDATE self_check_runs SET status = 'passed' WHERE id = 9`,
	} {
		_, err = database.ExecContext(ctx, statement)
		require.Error(t, err, name)
	}

	// A downgrade cannot hold an incomplete verdict, so it is refused rather
	// than rewriting one.
	_, err = provider.DownTo(ctx, 16)
	require.Error(t, err)

	_, err = database.ExecContext(ctx, `DELETE FROM self_check_runs WHERE id = 9`)
	require.NoError(t, err)
	var remaining int64
	require.NoError(t, database.QueryRowContext(ctx, `SELECT COUNT(*) FROM self_check_results WHERE run_id = 9`).Scan(&remaining))
	require.Zero(t, remaining, "results still cascade from their run")

	_, err = provider.DownTo(ctx, 16)
	require.NoError(t, err)
	require.NoError(t, database.QueryRowContext(ctx, `SELECT status FROM self_check_runs WHERE id = 7`).Scan(&status))
	require.Equal(t, "failed", status)
	_, err = provider.Up(ctx)
	require.NoError(t, err)
}
