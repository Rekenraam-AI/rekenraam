package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// #240. A posting's reconciliation_status records that a statement once
// verified it; it was also the only thing that kept the posting out of later
// reconciliations. Invalidating or voiding the checkpoint that verified it
// changed checkpoint state without touching that status, so the posting could
// never be selected again while the balance it belonged to was no longer an
// active starting point: no new reconciliation of the period could balance.
//
// The lock is now the checkpoint link. A posting stays out of new sessions only
// while a covering checkpoint — an active one, or one on an active one's
// previous-checkpoint chain — links its posting line.

func postEURWithOverride(t *testing.T, f *investmentsTestFixture, date string, amount int64) Transaction {
	t.Helper()
	created, err := f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api", ReconciliationOverride: true,
		ChangeReason: "entered late, statement already reconciled",
		Spec: TransactionInput{
			TransactionDate: date,
			JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
				{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(amount)},
				{AccountID: f.incomeAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-amount)},
			}}},
		},
	})
	require.NoError(t, err)
	return created
}

func startCashReconciliation(t *testing.T, f *investmentsTestFixture, statementDate string, balance int64) ReconciliationSession {
	t.Helper()
	session, err := f.transactionService.StartReconciliation(context.Background(), StartReconciliationInput{
		OriginType: "browser_api", OwnerUserID: f.ownerUserID, AccountID: f.cashAccountID,
		CommodityID: f.eurCommodityID, StatementDate: statementDate, StatementBalanceValue: exact.New(balance),
	})
	require.NoError(t, err)
	return session
}

func candidateDates(session ReconciliationSession) []string {
	dates := make([]string, 0, len(session.Candidates))
	for _, candidate := range session.Candidates {
		dates = append(dates, candidate.EntryDate)
	}
	return dates
}

func cashCheckpoint(t *testing.T, f *investmentsTestFixture, checkpointID int64) ReconciliationCheckpoint {
	t.Helper()
	checkpoints, err := f.transactionService.ReconciliationCheckpoints(context.Background(), ListReconciliationCheckpointsInput{
		AccountID: f.cashAccountID, CommodityID: f.eurCommodityID,
	})
	require.NoError(t, err)
	for _, checkpoint := range checkpoints {
		if checkpoint.ID == checkpointID {
			return checkpoint
		}
	}
	t.Fatalf("checkpoint %d not found", checkpointID)
	return ReconciliationCheckpoint{}
}

func voidCashCheckpoint(t *testing.T, f *investmentsTestFixture, checkpointID int64) {
	t.Helper()
	_, err := f.transactionService.VoidReconciliationCheckpoint(context.Background(), VoidReconciliationInput{
		OriginType: "browser_api", OwnerUserID: f.ownerUserID, ID: checkpointID, ChangeReason: "wrong statement",
	})
	require.NoError(t, err)
}

func checkpointPostingCount(t *testing.T, f *investmentsTestFixture, checkpointID int64) int {
	t.Helper()
	var count int
	require.NoError(t, f.database.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM reconciliation_checkpoint_postings WHERE checkpoint_id = ?`, checkpointID).Scan(&count))
	return count
}

// The issue's sequence: reconcile, append a backdated movement with an
// explicit override, then reconcile the same statement period again to the
// corrected ledger balance — with no balancing transaction invented.
func TestReReconcileAfterOverrideInvalidationReachesCorrectedBalance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)

	dividend := postEUR(t, f, "2026-03-01", 100)
	first := reconcileCash(t, f, "2026-04-01", 100)
	postEURWithOverride(t, f, "2026-03-01", -15)
	require.Equal(t, "invalidated", cashCheckpoint(t, f, first).Status)

	session := startCashReconciliation(t, f, "2026-04-01", 85)
	require.Nil(t, session.StartingCheckpointID, "the invalidated checkpoint is no longer a starting balance")
	require.Len(t, session.Candidates, 2, "the once-reconciled dividend needs re-verification alongside the new posting")
	var reverify ReconciliationPosting
	for _, candidate := range session.Candidates {
		if candidate.TransactionID == dividend.ID {
			reverify = candidate
		}
	}
	require.Equal(t, "reconciled", reverify.ReconciliationStatus)
	require.Len(t, session.SelectedPostings, 1, "the previously verified posting starts selected")
	require.Equal(t, reverify.PostingID, session.SelectedPostings[0].PostingID)

	second := reconcileCash(t, f, "2026-04-01", 85)
	require.Equal(t, []int64{second}, activeCheckpointIDs(t, f))
	require.Nil(t, cashCheckpoint(t, f, second).PreviousCheckpointID)
	require.Equal(t, 2, checkpointPostingCount(t, f, second))

	// The original evidence is untouched: still invalidated, still linked to
	// the version it verified, at the balance it verified.
	original := cashCheckpoint(t, f, first)
	require.Equal(t, "invalidated", original.Status)
	require.Equal(t, "100", original.StatementBalance.QuantityValue.String())
	require.Equal(t, 1, checkpointPostingCount(t, f, first))

	var transactions int
	require.NoError(t, f.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&transactions))
	require.Equal(t, 2, transactions, "no fabricated balancing transaction")
	require.Equal(t, SelfCheckPassed, checkpointIntegrityResult(t, f).Status)
}

// A partial invalidation keeps the earlier active checkpoint as the opening
// balance and offers exactly the postings the invalidated ones had verified,
// plus the late one.
func TestPartialInvalidationKeepsPrecedingOpeningBalance(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)

	postEUR(t, f, "2026-01-10", 10)
	january := reconcileCash(t, f, "2026-01-31", 10)
	postEUR(t, f, "2026-02-10", 20)
	february := reconcileCash(t, f, "2026-02-28", 30)
	postEUR(t, f, "2026-03-10", 5)
	march := reconcileCash(t, f, "2026-03-31", 35)

	postEURWithOverride(t, f, "2026-02-15", 1)
	require.Equal(t, []int64{january}, activeCheckpointIDs(t, f))
	require.Equal(t, "invalidated", cashCheckpoint(t, f, february).Status)
	require.Equal(t, "invalidated", cashCheckpoint(t, f, march).Status)

	session := startCashReconciliation(t, f, "2026-03-31", 36)
	require.NotNil(t, session.StartingCheckpointID)
	require.Equal(t, january, *session.StartingCheckpointID)
	require.Equal(t, "10", session.StartingBalance.QuantityValue.String())
	require.Equal(t, []string{"2026-02-10", "2026-02-15", "2026-03-10"}, candidateDates(session),
		"January stays covered; everything the invalidated statements verified is offered again")

	repaired := reconcileCash(t, f, "2026-03-31", 36)
	require.Equal(t, january, *cashCheckpoint(t, f, repaired).PreviousCheckpointID)
	require.ElementsMatch(t, []int64{january, repaired}, activeCheckpointIDs(t, f))
	require.Equal(t, SelfCheckPassed, checkpointIntegrityResult(t, f).Status)
}

// Voiding the latest checkpoint reopens its postings and the previous active
// checkpoint becomes the starting balance again.
func TestVoidedLatestCheckpointReopensItsPostings(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)

	postEUR(t, f, "2026-01-10", 10)
	january := reconcileCash(t, f, "2026-01-31", 10)
	postEUR(t, f, "2026-02-10", 20)
	february := reconcileCash(t, f, "2026-02-28", 30)

	voidCashCheckpoint(t, f, february)
	session := startCashReconciliation(t, f, "2026-02-28", 30)
	require.Equal(t, january, *session.StartingCheckpointID)
	require.Equal(t, []string{"2026-02-10"}, candidateDates(session))

	again := reconcileCash(t, f, "2026-02-28", 30)
	require.ElementsMatch(t, []int64{january, again}, activeCheckpointIDs(t, f))
	require.Equal(t, "voided", cashCheckpoint(t, f, february).Status)
	require.Equal(t, 1, checkpointPostingCount(t, f, february), "the voided checkpoint keeps its evidence")
}

// Voiding a checkpoint in the middle of a chain does not reopen its postings:
// the later active checkpoint chained from it still counts them in its
// statement balance, so offering them again would count them twice.
func TestVoidedMiddleCheckpointKeepsChainedPostingsCovered(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)

	postEUR(t, f, "2026-01-10", 10)
	reconcileCash(t, f, "2026-01-31", 10)
	february := postEUR(t, f, "2026-02-10", 20)
	februaryCheckpoint := reconcileCash(t, f, "2026-02-28", 30)
	postEUR(t, f, "2026-03-10", 5)
	march := reconcileCash(t, f, "2026-03-31", 35)

	voidCashCheckpoint(t, f, februaryCheckpoint)
	postEUR(t, f, "2026-04-10", 1)
	session := startCashReconciliation(t, f, "2026-04-30", 36)
	require.Equal(t, march, *session.StartingCheckpointID)
	require.Equal(t, []string{"2026-04-10"}, candidateDates(session))

	_, err := f.transactionService.ChangePostingReconciliationStatus(ctx, ChangePostingReconciliationStatusInput{
		OriginType: "browser_api", OwnerUserID: f.ownerUserID, AccountID: f.cashAccountID,
		PostingVersionIDs: []int64{currentCashPostingID(t, f, february.ID)}, Status: "uncleared",
	})
	require.ErrorIs(t, err, ErrReconciliationPosting, "a chain-covered posting stays locked")
}

// A posting whose checkpoint is gone may have its status changed by hand; one
// an active checkpoint covers may not.
func TestUncoveredReconciledPostingStatusIsChangeable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)

	posted := postEUR(t, f, "2026-01-10", 10)
	checkpoint := reconcileCash(t, f, "2026-01-31", 10)
	change := ChangePostingReconciliationStatusInput{
		OriginType: "browser_api", OwnerUserID: f.ownerUserID, AccountID: f.cashAccountID,
		PostingVersionIDs: []int64{currentCashPostingID(t, f, posted.ID)}, Status: "uncleared",
	}
	_, err := f.transactionService.ChangePostingReconciliationStatus(ctx, change)
	require.ErrorIs(t, err, ErrReconciliationPosting)

	voidCashCheckpoint(t, f, checkpoint)
	updated, err := f.transactionService.ChangePostingReconciliationStatus(ctx, change)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	for _, entry := range updated[0].JournalEntries {
		for _, posting := range entry.Postings {
			if posting.AccountID == f.cashAccountID {
				require.Equal(t, "uncleared", posting.ReconciliationStatus)
			}
		}
	}
}

// A session chains its checkpoint from the starting balance it opened on. If
// that balance stops being the account's latest active checkpoint while the
// session is open, finishing would chain from a balance the account no longer
// stands on; it is refused and a new session picks up the current one.
func TestFinishRefusesAStartingCheckpointThatIsNoLongerCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)

	postEUR(t, f, "2026-01-10", 10)
	reconcileCash(t, f, "2026-01-31", 10)
	postEUR(t, f, "2026-02-10", 20)

	selectAll := func(session ReconciliationSession) {
		ids := make([]int64, 0, len(session.Candidates))
		for _, candidate := range session.Candidates {
			ids = append(ids, candidate.PostingID)
		}
		_, err := f.transactionService.UpdateReconciliationSelection(ctx, ReconciliationSelectionInput{
			OriginType: "browser_api", OwnerUserID: f.ownerUserID, SessionID: session.ID, PostingVersionIDs: ids,
		})
		require.NoError(t, err)
	}
	finish := func(session ReconciliationSession) error {
		_, err := f.transactionService.FinishReconciliation(ctx, FinishReconciliationInput{
			OriginType: "browser_api", OwnerUserID: f.ownerUserID, SessionID: session.ID, ChangeReason: "statement verified",
		})
		return err
	}

	// Two sessions open and select on the same starting balance: the second
	// to finish would chain a sibling of the first.
	left := startCashReconciliation(t, f, "2026-02-28", 30)
	right := startCashReconciliation(t, f, "2026-02-28", 30)
	selectAll(left)
	selectAll(right)
	require.NoError(t, finish(left))
	require.ErrorIs(t, finish(right), ErrReconciliationStartStale)

	// The starting checkpoint is invalidated while a session is open.
	postEUR(t, f, "2026-03-10", 5)
	open := startCashReconciliation(t, f, "2026-03-31", 35)
	selectAll(open)
	postEURWithOverride(t, f, "2026-02-15", 1)
	require.ErrorIs(t, finish(open), ErrReconciliationStartStale)

	// A session that opened with no starting balance is stale once one exists.
	g := newInvestmentsTestFixture(t)
	postEUR(t, g, "2026-01-10", 10)
	session, err := g.transactionService.StartReconciliation(ctx, StartReconciliationInput{
		OriginType: "browser_api", OwnerUserID: g.ownerUserID, AccountID: g.cashAccountID,
		CommodityID: g.eurCommodityID, StatementDate: "2026-01-31", StatementBalanceValue: exact.New(10),
	})
	require.NoError(t, err)
	reconcileCash(t, g, "2026-01-31", 10)
	_, err = g.transactionService.FinishReconciliation(ctx, FinishReconciliationInput{
		OriginType: "browser_api", OwnerUserID: g.ownerUserID, SessionID: session.ID, ChangeReason: "statement verified",
	})
	require.ErrorIs(t, err, ErrReconciliationStartStale)
}

func currentCashPostingID(t *testing.T, f *investmentsTestFixture, transactionID int64) int64 {
	t.Helper()
	var postingID int64
	require.NoError(t, f.database.QueryRowContext(context.Background(), `
		SELECT pv.id
		FROM current_transaction_versions tv
		JOIN journal_entries je ON je.transaction_version_id = tv.id
		JOIN posting_versions pv ON pv.journal_entry_id = je.id
		WHERE tv.transaction_id = ? AND pv.account_id = ?
	`, transactionID, f.cashAccountID).Scan(&postingID))
	return postingID
}
