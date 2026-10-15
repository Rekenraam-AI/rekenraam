package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// #247. Finish used to read the checkpoint's statement_account_sequence with a
// follow-up query whose error it discarded, so a failed read committed the
// checkpoint at sequence 0: before every posting on the statement date, which
// left later same-day edits of the postings it verified unguarded. The
// boundary now comes from the posting versions finish has just written. It is
// the greatest same-day sequence among the selected postings — not among every
// posting that day, and not among selected postings from earlier days.
func TestFinishReconciliationSetsStatementSequenceFromSelectedSameDayPostings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)

	earlier := postEUR(t, f, "2026-03-30", 4)
	first := postEUR(t, f, "2026-03-31", 10)
	second := postEUR(t, f, "2026-03-31", 7)
	postEUR(t, f, "2026-03-31", 5) // sequence 3, left unselected

	session := startCashReconciliation(t, f, "2026-03-31", 21)
	selected := make([]int64, 0, 3)
	for _, candidate := range session.Candidates {
		switch candidate.TransactionID {
		case earlier.ID, first.ID, second.ID:
			selected = append(selected, candidate.PostingID)
		}
	}
	require.Len(t, selected, 3)
	_, err := f.transactionService.UpdateReconciliationSelection(ctx, ReconciliationSelectionInput{
		OriginType: "browser_api", OwnerUserID: f.ownerUserID, SessionID: session.ID, PostingVersionIDs: selected,
	})
	require.NoError(t, err)
	_, err = f.transactionService.FinishReconciliation(ctx, FinishReconciliationInput{
		OriginType: "browser_api", OwnerUserID: f.ownerUserID, SessionID: session.ID, ChangeReason: "statement verified",
	})
	require.NoError(t, err)

	var checkpointID, sequence int64
	require.NoError(t, f.database.QueryRowContext(ctx, `
		SELECT id, statement_account_sequence FROM reconciliation_checkpoints WHERE session_id = ?
	`, session.ID).Scan(&checkpointID, &sequence))
	require.Equal(t, int64(2), sequence, "the boundary sits after the second 31 March posting, before the unselected third")
	require.Equal(t, 3, checkpointPostingCount(t, f, checkpointID))
	require.Equal(t, SelfCheckPassed, checkpointIntegrityResult(t, f).Status)
}
