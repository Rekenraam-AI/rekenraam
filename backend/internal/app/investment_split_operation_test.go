package app

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

func TestJournalFreeSplitReversalBeforeAndAfterEntitlement(t *testing.T) {
	t.Parallel()
	for _, entitled := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "adjusted"}[entitled], func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
			require.NoError(t, err)
			if entitled {
				buyOn(t, f, "2026-02-01", 10, 10000)
			}
			input := ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "wrong split"}
			before := buyReplacementPreviewSnapshot(t, f.database)
			_, err = f.investmentService.ReverseSplitReconciliationImpact(ctx, input)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			result, err := f.investmentService.ReverseSplit(ctx, input)
			require.NoError(t, err)
			if entitled {
				require.Positive(t, result.ID)
				require.Equal(t, "-10", holdingPosting(t, f, result))
			} else {
				require.Zero(t, result.ID)
			}
			requireInvestmentSelfCheckPasses(t, f)
			_, err = f.investmentService.ReverseSplit(ctx, input)
			require.ErrorIs(t, err, ErrInvestmentSplitAlreadyCorrected)
		})
	}
}

func TestSplitOperationReplacementCrossesZeroBothWays(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-02-01", 10, 10000)
	zero, err := f.investmentService.Split(ctx, splitInput(f, "2026-01-01", 2, 1))
	require.NoError(t, err)
	input := ReplaceInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: zero.OperationID, Reason: "correct effective date", EffectiveOn: "2026-03-01", RatioNumerator: 2, RatioDenominator: 1}
	before := buyReplacementPreviewSnapshot(t, f.database)
	preview, err := f.investmentService.PreviewSplitReplacement(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "10", preview.Plan.DeltaValue.String())
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	positive, err := f.investmentService.ReplaceSplit(ctx, input)
	require.NoError(t, err)
	require.Zero(t, positive.Inverse.ID)
	require.Positive(t, positive.Replacement.ID)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(20, 0)))
	requireInvestmentSelfCheckPasses(t, f)
	input.OperationID = positive.OperationID
	input.EffectiveOn = "2026-01-01"
	input.Memo = "corrected source memo"
	negative, err := f.investmentService.ReplaceSplit(ctx, input)
	require.NoError(t, err)
	require.Positive(t, negative.Inverse.ID)
	require.Zero(t, negative.Replacement.ID)
	require.Positive(t, negative.OperationID)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(10, 0)))
	requireInvestmentSelfCheckPasses(t, f)
	activity, _, err := f.investmentService.SplitActivity(ctx, f.ownerUserID, 0, 50)
	require.NoError(t, err)
	require.Equal(t, "corrected source memo", activity[0].Memo)
}

func TestJournalFreeSplitCashInLieuRetainsIdentityAndReviewsReversal(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 3, 2))
	require.NoError(t, err)
	buyOn(t, f, "2026-02-01", 1, 10000)
	input := cashInLieuInput(f, 0)
	input.SettledOperationID = split.OperationID
	input.DisposalOn = "2026-03-01"
	input.PaymentOn = "2026-03-05"
	input.QuantityValue = exact.New(5)
	input.QuantityScale = 1
	cash, err := f.investmentService.CashInLieu(ctx, input)
	require.NoError(t, err)
	_, err = f.investmentService.ReverseSplit(ctx, ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "wrong ratio"})
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	preview, err := f.investmentService.ReverseSplitReconciliationImpact(ctx, ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "wrong ratio"})
	require.NoError(t, err)
	require.NotNil(t, preview.GainImpact)
	require.Len(t, preview.GainImpact.Changes, 1)
	context, err := f.investmentService.TradeCorrectionContext(ctx, f.ownerUserID, cash.Transaction.ID)
	require.NoError(t, err)
	require.Equal(t, split.OperationID, context.SettledOperationID)
	require.Zero(t, context.SettledTransactionID)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestJournalFreeSplitReversalAcknowledgesGainsAndGuardsAdjustedQuantity(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	buyOn(t, f, "2026-02-01", 10, 10000)
	_, err = f.investmentService.Sell(ctx, sellInput(f, "2026-04-01", 5))
	require.NoError(t, err)
	reconcileHoldingThrough(t, f, "2026-05-01", 15)
	input := ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "wrong corporate action"}
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReverseSplitReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Len(t, impact.AffectedCheckpoints, 1)
	require.Len(t, impact.GainImpact.Changes, 1)
	requireScaled(t, 5000, 2, impact.GainImpact.Changes[0].After.DisposedBasis, "the unsplit units carry twice the basis")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.investmentService.ReverseSplit(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input.ReconciliationOverride = unboundOverride
	_, err = f.investmentService.ReverseSplit(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	_, err = f.investmentService.ReverseSplit(ctx, input)
	require.NoError(t, err)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(5, 0)))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestJournalFreeSplitReplacementRetainsSameDayRootSlot(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	buyOn(t, f, "2026-03-01", 10, 10000)
	_, err = f.investmentService.ReplaceSplit(ctx, ReplaceInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "correct ratio", EffectiveOn: "2026-03-01", RatioNumerator: 3, RatioDenominator: 1})
	require.NoError(t, err)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(10, 0)), "later same-day opening remains unentitled")
	buyOn(t, f, "2026-02-01", 1, 1000)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(13, 0)))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestSplitOperationCorrectionsExportAndRestoreBothJournalShapes(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-02-01", 10, 10000)
	zero, err := f.investmentService.Split(ctx, splitInput(f, "2026-01-01", 2, 1))
	require.NoError(t, err)
	input := ReplaceInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: zero.OperationID, Reason: "correct date", EffectiveOn: "2026-03-01", RatioNumerator: 2, RatioDenominator: 1}
	positive, err := f.investmentService.ReplaceSplit(ctx, input)
	require.NoError(t, err)
	input.OperationID = positive.OperationID
	input.EffectiveOn = "2026-01-01"
	final, err := f.investmentService.ReplaceSplit(ctx, input)
	require.NoError(t, err)
	_, err = f.investmentService.ReverseSplit(ctx, ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: final.OperationID, Reason: "withdraw duplicate"})
	require.NoError(t, err)
	files := exportedBundle(t, f)
	zeroIDs := map[string]bool{strconv.FormatInt(zero.OperationID, 10): true, strconv.FormatInt(final.OperationID, 10): true}
	for _, row := range files["investment-operations.csv"] {
		if zeroIDs[row["operation_id"]] || row["operation_kind"] == "reversal" {
			require.Empty(t, row["transaction_id"])
		}
	}
	require.Len(t, files["investment-split-facts.csv"], 3)
	var inverseLinked bool
	for _, row := range files["investment-operation-journal-links.csv"] {
		if row["operation_id"] == strconv.FormatInt(final.OperationID, 10) {
			inverseLinked = true
			require.Equal(t, "reversal", row["role"])
		}
	}
	require.True(t, inverseLinked)
	requireInvestmentSelfCheckPasses(t, f)
	backup := filepath.Join(t.TempDir(), "backup.sqlite")
	_, err = db.OnlineBackupSQLiteDatabase(ctx, f.database, backup, db.OnlineBackupOptions{})
	require.NoError(t, err)
	restoredURL := db.SQLiteFileURL(filepath.Join(t.TempDir(), "restored.sqlite"))
	_, err = db.RestoreSQLiteDatabase(ctx, backup, restoredURL)
	require.NoError(t, err)
	restored, err := db.Open(ctx, restoredURL)
	require.NoError(t, err)
	defer restored.Close()
	f.database = restored
	accounts := db.NewAccountRepository(restored)
	f.accountService = NewAccountService(accounts, db.NewInstitutionRepository(restored), nil)
	f.transactionService = NewTransactionService(db.NewTransactionRepository(restored), db.NewPayeeRepository(restored), accounts, db.NewCommodityRepository(restored))
	f.investmentService = NewInvestmentService(db.NewInvestmentRepository(restored), f.accountService, f.transactionService, nil)
	activity, _, err := f.investmentService.SplitActivity(ctx, f.ownerUserID, 0, 50)
	require.NoError(t, err)
	require.Len(t, activity, 3)
	require.Equal(t, "reversed", activity[0].Standing)
	buyOn(t, f, "2026-01-01", 1, 1000)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(11, 0)), "withdrawn ratios remain audit evidence only")
	requireInvestmentSelfCheckPasses(t, f)
}

func TestJournalFreeSplitImpossibleReversalNamesDependentAndRollsBack(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	buyOn(t, f, "2026-02-01", 10, 10000)
	sale, err := f.investmentService.Sell(ctx, sellInput(f, "2026-04-01", 15))
	require.NoError(t, err)
	var saleID int64
	require.NoError(t, f.database.QueryRow(`SELECT operation_id FROM investment_disposal_decisions WHERE transaction_id = ?`, sale.Transaction.ID).Scan(&saleID))
	before := buyReplacementPreviewSnapshot(t, f.database)
	input := ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "wrong notice"}
	for _, preview := range []bool{true, false} {
		if preview {
			_, err = f.investmentService.ReverseSplitReconciliationImpact(ctx, input)
		} else {
			_, err = f.investmentService.ReverseSplit(ctx, input)
		}
		var dependency InvestmentSplitDependencyError
		require.ErrorAs(t, err, &dependency)
		require.Equal(t, saleID, dependency.OperationID)
		require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	}
	requireInvestmentSelfCheckPasses(t, f)
}

func TestJournalFreeSplitReplacementLateFailureRollsBackBothHeaders(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	_, err = f.database.Exec(`CREATE TRIGGER refuse_split_replacement BEFORE INSERT ON investment_split_facts
 WHEN NEW.operation_id <> 1 BEGIN SELECT RAISE(ABORT,'late split refusal'); END`)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	input := ReplaceInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "correct ratio", EffectiveOn: "2026-03-01", RatioNumerator: 3, RatioDenominator: 1}
	for _, preview := range []bool{true, false} {
		if preview {
			_, err = f.investmentService.PreviewSplitReplacement(ctx, input)
		} else {
			_, err = f.investmentService.ReplaceSplit(ctx, input)
		}
		require.Error(t, err)
		require.Contains(t, err.Error(), "late split refusal")
		require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	}
	requireInvestmentSelfCheckPasses(t, f)
}

func TestJournalFreeSplitReversalExemptionRequiresNonzeroInverseJournal(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	buyOn(t, f, "2026-02-01", 10, 10000)
	inverse, err := f.investmentService.ReverseSplit(ctx, ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "wrong split"})
	require.NoError(t, err)
	requireInvestmentSelfCheckPasses(t, f)
	_, err = f.database.Exec(`DROP TRIGGER investment_operation_journal_links_no_delete`)
	require.NoError(t, err)
	_, err = f.database.Exec(`DELETE FROM investment_operation_journal_links WHERE transaction_version_id IN
 (SELECT id FROM transaction_versions WHERE transaction_id = ?)`, inverse.ID)
	require.NoError(t, err)
	foundation := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentFoundation)
	require.Equal(t, SelfCheckFailed, foundation.Status)
	require.Contains(t, foundation.Summary, "splits post a different journal delta")
}

func TestJournalFreeImportedSplitCannotCorrectWithoutCommittedSource(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	input := splitInput(f, "2026-03-01", 2, 1)
	input.OriginType = "import"
	split, err := f.investmentService.Split(ctx, input)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReverseSplit(ctx, ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, OperationID: split.OperationID, Reason: "wrong split"})
	require.ErrorIs(t, err, ErrInvestmentImportedSplit)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}
