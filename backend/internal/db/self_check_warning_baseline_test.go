package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelfCheckBaselinePreservesAndValidatesAttributionWarnings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, _ := newMFATestRepository(t)
	_, err := database.Exec(`INSERT INTO books (id, code, name, owner_user_id, created_at, updated_at, updated_by_user_id)
		VALUES (1, 'personal', 'Personal', 1, '2026-01-01', '2026-01-01', 1);
		INSERT INTO self_check_runs (id, book_id, trigger, status, failed_check_count, incomplete_check_count, started_at, finished_at, created_at)
		VALUES (7, 1, 'manual', 'failed', 1, 1, 'start', 'finish', 'created');
		INSERT INTO self_check_results (id, run_id, check_id, status, finding_count, sample_json, unavailable_count, unavailable_sample_json, summary)
		VALUES (70, 7, 'lot_reconciliation', 'failed', 2, '[42]', 3, '[5,6,7]', 'original verdict');`)
	require.NoError(t, err)
	require.NoError(t, Migrate(ctx, database))
	r := NewSelfCheckRepository(database, database)
	run, records, err := r.LatestSelfCheckRun(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "failed", run.Status)
	require.EqualValues(t, 1, run.IncompleteCheckCount)
	require.Zero(t, run.WarningCheckCount)
	require.Equal(t, "start", run.StartedAt)
	require.Equal(t, "finish", run.FinishedAt.String)
	require.Equal(t, SelfCheckResultRecord{RunID: 7, CheckID: "lot_reconciliation", Status: "failed", FindingCount: 2,
		SampleJSON: "[42]", UnavailableCount: 3, UnavailableSampleJSON: "[5,6,7]", WarningSampleJSON: "[]", Summary: "original verdict"}, records[0])
	_, err = database.Exec(`UPDATE self_check_runs SET status = 'warning', failed_check_count = 0, warning_check_count = 1 WHERE id = 7;
		UPDATE self_check_results SET status = 'warning', finding_count = 0, warning_count = 1,
		warning_sample_json = '[{"transaction_id":9,"reason":"no_short_at_entitlement_close"}]' WHERE id = 70;`)
	require.NoError(t, err)
	run, records, err = r.LatestSelfCheckRun(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "warning", run.Status)
	require.EqualValues(t, 1, run.WarningCheckCount)
	require.EqualValues(t, 1, records[0].WarningCount)
	for _, statement := range []string{
		`UPDATE self_check_runs SET status = 'passed' WHERE id = 7`,
		`UPDATE self_check_runs SET status = 'incomplete' WHERE id = 7`,
		`UPDATE self_check_results SET status = 'passed' WHERE id = 70`,
		`UPDATE self_check_results SET status = 'incomplete' WHERE id = 70`,
		`UPDATE self_check_results SET finding_count = 1 WHERE id = 70`,
	} {
		_, err = database.Exec(statement)
		require.Error(t, err, statement)
	}
	_, err = database.Exec(`DELETE FROM self_check_runs WHERE id = 7`)
	require.NoError(t, err)
	var children int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM self_check_results`).Scan(&children))
	require.Zero(t, children, "rebuilt FK still cascades")
	require.NoError(t, Migrate(ctx, database))
}
