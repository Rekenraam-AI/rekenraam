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

// Two EUR lots at 10 and 30 per unit, beside an independent USD position.
func mixedWriteOffPosition(t *testing.T) (*investmentsTestFixture, int64, int64, int64, int64) {
	t.Helper()
	f := newInvestmentsTestFixture(t)
	first := buyOn(t, f, "2026-01-01", 2, 2000)
	second := buyOn(t, f, "2026-02-01", 2, 6000)
	usd := seedTestCurrencyCommodity(t, f.database, "USD")
	dollars := tradeOn(f, f.holdingAccountID, "2026-01-01", 2, 10000)
	dollars.CashCommodityID = usd
	other, err := f.investmentService.Buy(context.Background(), dollars)
	require.NoError(t, err)
	return f, usd, *first.LotID, *second.LotID, *other.LotID
}

func requireWriteOffCurrency(t *testing.T, f *investmentsTestFixture, result InvestmentTradeResult, currency int64) {
	t.Helper()
	require.Equal(t, currency, result.DisposalDecision.CostCommodityID)
	requireZeroProceeds(t, f, result.Transaction.ID)
	transaction, err := f.transactionService.Transaction(context.Background(), result.Transaction.ID)
	require.NoError(t, err)
	for _, entry := range transaction.JournalEntries {
		for _, posting := range entry.Postings {
			require.Equal(t, f.stockCommodityID, posting.CommodityID, "only security legs")
		}
	}
}

func requireUntouchedDollarLot(t *testing.T, f *investmentsTestFixture, lotID int64) {
	t.Helper()
	var quantity, basis string
	require.NoError(t, f.database.QueryRow(`SELECT remaining_quantity_value, remaining_cost_basis_value
		FROM current_investment_lots WHERE id = ?`, lotID).Scan(&quantity, &basis))
	require.Equal(t, "2", quantity)
	require.Equal(t, "10000", basis)
}

func TestWriteOffMixedCurrenciesEachMethodConsumesOnlySelectedCurrency(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"fifo", "lifo", "average_cost", "specific_lot"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			f, _, first, second, dollars := mixedWriteOffPosition(t)
			ctx := context.Background()
			input := backdatedWriteOff(f, "2026-03-01", 1)
			input.CostCommodityID = f.eurCommodityID
			input.CostBasisMethod = method
			wantLot, wantLoss := first, int64(-1000)
			switch method {
			case "lifo", "specific_lot":
				wantLot, wantLoss = second, -3000
			case "average_cost":
				wantLoss = -2000
			}
			if method == "specific_lot" {
				input.LotAllocations = []InvestmentLotAllocationInput{{LotID: second, QuantityValue: exact.New(1)}}
			}
			before := buyReplacementPreviewSnapshot(t, f.database)
			preview, err := f.investmentService.PreviewWriteOff(ctx, input)
			require.NoError(t, err)
			_, err = f.investmentService.WriteOffReconciliationImpact(ctx, input)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "previews roll back")
			require.Equal(t, f.eurCommodityID, preview.DisposalDecision.CostCommodityID)
			requireScaled(t, wantLoss, 2, exact.ScaledIntFromInt64(preview.RealizedGain, preview.RealizedGainScale), "selected currency loss")
			result, err := f.investmentService.WriteOff(ctx, input)
			require.NoError(t, err)
			require.Equal(t, preview.Allocations, result.Allocations)
			require.Equal(t, wantLot, result.Allocations[0].LotID)
			requireWriteOffCurrency(t, f, result, f.eurCommodityID)
			requireUntouchedDollarLot(t, f, dollars)
			requireSelfCheckPasses(t, f)
		})
	}
}

func TestWriteOffMixedCurrenciesRejectsAmbiguousAndInvalidElectionsAtomically(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"omitted currency", "historical omitted currency", "omitted currency with specific lots", "negative currency", "instrument as currency", "missing currency", "currency without lots", "selected position insufficient", "cross currency election", "historical cross currency election", "future lot election"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, _, first, _, dollars := mixedWriteOffPosition(t)
			ctx := context.Background()
			input := backdatedWriteOff(f, "2026-03-01", 1)
			input.CostCommodityID = f.eurCommodityID
			switch name {
			case "omitted currency", "historical omitted currency":
				input.CostCommodityID = 0
			case "omitted currency with specific lots":
				input.CostCommodityID = 0
				input.CostBasisMethod = "specific_lot"
				input.LotAllocations = []InvestmentLotAllocationInput{{LotID: first, QuantityValue: exact.New(1)}}
			case "negative currency":
				input.CostCommodityID = -1
			case "instrument as currency":
				input.CostCommodityID = f.stockCommodityID
			case "missing currency":
				input.CostCommodityID = 999999
			case "currency without lots":
				input.CostCommodityID = seedTestCurrencyCommodity(t, f.database, "GBP")
			case "selected position insufficient":
				input.QuantityValue = exact.New(5) // Six total units, only four EUR.
			case "cross currency election", "historical cross currency election":
				input.QuantityValue = exact.New(2)
				input.CostBasisMethod = "specific_lot"
				// Refuse even after the first valid allocation has been written.
				input.LotAllocations = []InvestmentLotAllocationInput{{LotID: first, QuantityValue: exact.New(1)}, {LotID: dollars, QuantityValue: exact.New(1)}}
			case "future lot election":
				future := buyOn(t, f, "2026-04-01", 1, 1000)
				input.CostBasisMethod = "specific_lot"
				input.LotAllocations = []InvestmentLotAllocationInput{{LotID: *future.LotID, QuantityValue: exact.New(1)}}
			}
			if name == "historical omitted currency" || name == "historical cross currency election" {
				_, err := f.investmentService.Sell(ctx, tradeOn(f, f.holdingAccountID, "2026-04-01", 1, 1000))
				require.NoError(t, err)
			}
			before := buyReplacementPreviewSnapshot(t, f.database)
			_, err := f.investmentService.PreviewWriteOff(ctx, input)
			require.Error(t, err)
			_, err = f.investmentService.WriteOffReconciliationImpact(ctx, input)
			require.Error(t, err)
			_, err = f.investmentService.WriteOff(ctx, input)
			require.Error(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			requireUntouchedDollarLot(t, f, dollars)
			requireSelfCheckPasses(t, f)
		})
	}
}

func TestWriteOffSelectedCurrencyUsesClosedHistoricalLotsAndGuardsGainsAndCheckpoints(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"fifo", "specific_lot"} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			f, _, first, _, dollars := mixedWriteOffPosition(t)
			ctx := context.Background()
			sale := tradeOn(f, f.holdingAccountID, "2026-04-01", 3, 9000)
			sold, err := f.investmentService.Sell(ctx, sale)
			require.NoError(t, err)
			var status string
			require.NoError(t, f.database.QueryRow(`SELECT status FROM current_investment_lots WHERE id = ?`, first).Scan(&status))
			require.Equal(t, "closed", status)
			checkpoint := reconcileHolding(t, f, "2026-05-01", 3)
			input := backdatedWriteOff(f, "2026-03-01", 1)
			input.CostCommodityID = f.eurCommodityID
			input.CostBasisMethod = method
			if method == "specific_lot" {
				input.LotAllocations = []InvestmentLotAllocationInput{{LotID: first, QuantityValue: exact.New(1)}}
			}
			before := buyReplacementPreviewSnapshot(t, f.database)
			preview, err := f.investmentService.PreviewWriteOff(ctx, input)
			require.NoError(t, err)
			require.Equal(t, first, preview.Allocations[0].LotID)
			impact, err := f.investmentService.WriteOffReconciliationImpact(ctx, input)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			require.Len(t, impact.AffectedCheckpoints, 1)
			require.Equal(t, checkpoint, impact.AffectedCheckpoints[0].CheckpointID)
			change := gainChangeFor(t, impact, sold.Transaction.ID)
			require.Equal(t, f.eurCommodityID, change.After.CostCommodityID)
			input.ReconciliationOverride = unboundOverride
			_, err = f.investmentService.WriteOff(ctx, input)
			require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
			input.ReconciliationOverride = CheckpointOverride{}
			_, err = f.investmentService.WriteOff(ctx, input)
			require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			input.ReconciliationOverride = unboundOverride
			result, err := f.investmentService.WriteOff(ctx, input)
			require.NoError(t, err)
			require.Equal(t, preview.Allocations, result.Allocations)
			requireWriteOffCurrency(t, f, result, f.eurCommodityID)
			requireUntouchedDollarLot(t, f, dollars)
			requireSelfCheckPasses(t, f)
		})
	}
}

func TestWriteOffReplacementPinsOrChangesCurrencyAndExportsOriginalEvidence(t *testing.T) {
	t.Parallel()
	f, usd, _, _, dollars := mixedWriteOffPosition(t)
	ctx := context.Background()
	input := backdatedWriteOff(f, "2026-03-01", 1)
	input.CostCommodityID = f.eurCommodityID
	written, err := f.investmentService.WriteOff(ctx, input)
	require.NoError(t, err)
	// Compatibility: omitting the field in the same holding retains EUR.
	replacement := ReplaceInvestmentWriteOffInput{OwnerUserID: f.ownerUserID, TransactionID: written.Transaction.ID,
		Reason: "two units were delisted", Replacement: writeOffReplacement(f, "2026-03-01", 2, "fifo")}
	result, err := acknowledgedReplaceWriteOff(ctx, f.investmentService, replacement)
	require.NoError(t, err)
	requireWriteOffCurrency(t, f, result.Replacement, f.eurCommodityID)
	requireUntouchedDollarLot(t, f, dollars)
	// An explicit change moves the decision to the USD position; both sides
	// are named in gain review and the original EUR snapshots stay immutable.
	replacement.TransactionID = result.Replacement.Transaction.ID
	replacement.Replacement.CostCommodityID = usd
	replacement.Replacement.QuantityValue = exact.New(1)
	replacement.Reason = "the dollar position was delisted"
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplaceWriteOffReconciliationImpact(ctx, replacement)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	change := gainChangeFor(t, impact, result.Replacement.Transaction.ID)
	require.Equal(t, f.eurCommodityID, change.Before.CostCommodityID)
	require.Equal(t, usd, change.After.CostCommodityID)
	_, err = f.investmentService.ReplaceWriteOff(ctx, replacement)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	replacement.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	changed, err := f.investmentService.ReplaceWriteOff(ctx, replacement)
	require.NoError(t, err)
	requireWriteOffCurrency(t, f, changed.Replacement, usd)
	rows := exportedBundle(t, f)["disposal-decisions.csv"]
	for _, result := range []InvestmentTradeResult{written, result.Replacement, changed.Replacement} {
		found := false
		for _, row := range rows {
			if row["transaction_id"] == strconv.FormatInt(result.Transaction.ID, 10) {
				found = true
				require.Equal(t, strconv.FormatInt(result.DisposalDecision.CostCommodityID, 10), row["cost_commodity_id"])
				require.Equal(t, "0", row["proceeds_value"])
			}
		}
		require.True(t, found, "export preserves each currency-pinned original decision")
	}
	requireSelfCheckPasses(t, f)
	backupPath := filepath.Join(t.TempDir(), "backup.sqlite")
	_, err = db.OnlineBackupSQLiteDatabase(ctx, f.database, backupPath, db.OnlineBackupOptions{})
	require.NoError(t, err)
	restoredURL := db.SQLiteFileURL(filepath.Join(t.TempDir(), "restored.sqlite"))
	_, err = db.RestoreSQLiteDatabase(ctx, backupPath, restoredURL)
	require.NoError(t, err)
	restored, err := db.Open(ctx, restoredURL)
	require.NoError(t, err)
	defer restored.Close()
	run, err := selfCheckOver(t, restored).RunSelfCheck(ctx, "manual")
	require.NoError(t, err)
	require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentFoundation).Status)
	require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentReplay).Status)
	var restoredCurrency int64
	require.NoError(t, restored.QueryRow(`SELECT cost_commodity_id FROM investment_disposal_decisions WHERE transaction_id = ?`, changed.Replacement.Transaction.ID).Scan(&restoredCurrency))
	require.Equal(t, usd, restoredCurrency)
}
