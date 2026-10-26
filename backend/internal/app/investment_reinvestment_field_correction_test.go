package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// #395 slice 3: a reinvested dividend's date, holding, instrument and cost
// currency are correctable through the acquisition lifecycle (T-116's typed
// old/new-position replay). It stays an income-funded acquisition: the
// replacement opens a reinvested_dividend lot and posts income, never cash.

func replaceReinvestment(f *investmentsTestFixture, transactionID int64, reason string, replacement ReinvestedDividendInput) ReplaceReinvestedDividendInput {
	return ReplaceReinvestedDividendInput{OwnerUserID: f.ownerUserID, TransactionID: transactionID, Reason: reason, Replacement: replacement}
}

func acknowledgedReplaceReinvestedDividend(ctx context.Context, s *InvestmentService, input ReplaceReinvestedDividendInput) (ReplaceReinvestedDividendResult, error) {
	if input.GainImpactAcknowledgement == "" {
		if impact, err := s.ReplaceReinvestedDividendReconciliationImpact(ctx, input); err == nil && impact.GainImpact != nil {
			input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
		}
	}
	return s.ReplaceReinvestedDividend(ctx, input)
}

// Moving the January reinvestment after the February buy changes which lot
// FIFO consumes for the March sale: bought shares at 20.00 instead of
// reinvested ones at 2.00, so basis 10.00 → 100.00 and gain 140.00 → 50.00.
func TestReplaceReinvestedDividendMovesDateAndRestatesLaterSale(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	original := reinvestmentWithLaterSale(t, f)
	input := replaceReinvestment(f, original.Transaction.ID, "reinvested in mid-February",
		reinvestmentInput(f, "2026-02-15", 10, 2000))

	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplaceReinvestedDividendReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "preview writes nothing")
	require.NotNil(t, impact.GainImpact)
	require.Len(t, impact.GainImpact.Changes, 1)
	requireScaled(t, 1000, 2, impact.GainImpact.Changes[0].Before.DisposedBasis, "basis before")
	requireScaled(t, 10000, 2, impact.GainImpact.Changes[0].After.DisposedBasis, "basis after")
	requireScaled(t, 5000, 2, impact.GainImpact.Changes[0].After.Gain, "gain after")
	_, err = f.investmentService.ReplaceReinvestedDividend(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	require.Zero(t, correctionCount(t, f))

	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	result, err := f.investmentService.ReplaceReinvestedDividend(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-01-01"}, journalDates(t, f, result.Inverse.ID))
	assert.Equal(t, []string{"2026-02-15"}, journalDates(t, f, result.Replacement.Transaction.ID))
	require.Equal(t, int64(-2000), incomeBalance(t, f), "income is unchanged in amount, now dated in February")
	gains, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
	require.NoError(t, err)
	require.Len(t, gains, 1)
	assertMoneyValue(t, 5000, 2, gains[0].RealizedGainValue, gains[0].RealizedGainScale, "replayed sale")
	assert.Equal(t, "15", positionQuantity(t, f, f.holdingAccountID, f.stockCommodityID))
	var kind string
	require.NoError(t, f.database.QueryRow(`SELECT o.operation_kind FROM investment_lots l
		JOIN investment_operations o ON o.id = l.operation_id WHERE l.id = ?`, *result.Replacement.LotID).Scan(&kind))
	assert.Equal(t, "reinvested_dividend", kind, "the replacement stays an income-funded acquisition")
	var retired, active int
	require.NoError(t, f.database.QueryRow(`SELECT
		(SELECT count(*) FROM price_observations WHERE source_transaction_version_id = ? AND voided_at IS NOT NULL),
		(SELECT count(*) FROM price_observations WHERE source_transaction_version_id = ? AND voided_at IS NULL AND valuation_date = '2026-02-15')`,
		original.Transaction.VersionID, result.Replacement.Transaction.VersionID).Scan(&retired, &active))
	assert.Equal(t, 1, retired, "the original implied price is retired")
	assert.Equal(t, 1, active, "the replacement's implied price is on its corrected date")
	requireInvestmentSelfCheckPasses(t, f)
}

// Pooled (average-cost) disposals replay too: moving the reinvestment after
// the sale leaves only the bought shares in the pool on the sale date, so its
// basis goes from (20.00 + 200.00) / 20 × 5 = 55.00 to 100.00.
func TestReplaceReinvestedDividendDateReplaysAverageCostSale(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	setHoldingCostBasisMethod(t, f, "average_cost")
	original := reinvestOn(t, f, "2026-01-01", 10, 2000)
	buyOn(t, f, "2026-02-01", 10, 20000)
	sale := sellInput(f, "2026-03-01", 5)
	sale.CashAmountValue = 15000
	_, err := f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)

	input := replaceReinvestment(f, original.Transaction.ID, "reinvested in April", reinvestmentInput(f, "2026-04-01", 10, 2000))
	impact, err := f.investmentService.ReplaceReinvestedDividendReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, impact.GainImpact)
	requireScaled(t, 5500, 2, impact.GainImpact.Changes[0].Before.DisposedBasis, "pooled basis before")
	requireScaled(t, 10000, 2, impact.GainImpact.Changes[0].After.DisposedBasis, "pooled basis after")
	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	_, err = f.investmentService.ReplaceReinvestedDividend(ctx, input)
	require.NoError(t, err)
	gains, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
	require.NoError(t, err)
	require.Len(t, gains, 1)
	assertMoneyValue(t, 5000, 2, gains[0].RealizedGainValue, gains[0].RealizedGainScale, "150.00 − 100.00")
	assert.Equal(t, "15", positionQuantity(t, f, f.holdingAccountID, f.stockCommodityID))
	requireInvestmentSelfCheckPasses(t, f)
}

// Moving the reinvestment after the sale that consumed it leaves the sale
// without shares on its date: the sale is named and nothing is written.
func TestReplaceReinvestedDividendDateAfterDependentSaleNamesSale(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	original := reinvestOn(t, f, "2026-01-01", 10, 2000)
	sold, err := f.investmentService.Sell(ctx, sellInput(f, "2026-02-01", 5))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)

	_, err = acknowledgedReplaceReinvestedDividend(ctx, f.investmentService, replaceReinvestment(f, original.Transaction.ID,
		"paid in March", reinvestmentInput(f, "2026-03-01", 10, 2000)))
	var dependency InvestmentBuyDependencyError
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, saleOperationIDForTest(t, f, sold.Transaction.ID), dependency.OperationID)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	assert.Zero(t, correctionCount(t, f))
}

// A reinvestment booked to the wrong holding moves: the source's FIFO sale
// now consumes the bought lot, the new holding has the reinvested shares, and
// both positions replay under one audit event.
func TestReplaceReinvestedDividendMovesHoldingAndReplaysBothPositions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	other := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	original := reinvestmentWithLaterSale(t, f)
	replacement := reinvestmentInput(f, "2026-01-01", 10, 2000)
	replacement.HoldingAccountID = other
	input := replaceReinvestment(f, original.Transaction.ID, "reinvested in the other broker", replacement)
	impact, err := f.investmentService.ReplaceReinvestedDividendReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, impact.GainImpact)
	requireScaled(t, 10000, 2, impact.GainImpact.Changes[0].After.DisposedBasis, "the sale now takes bought shares")

	result, err := acknowledgedReplaceReinvestedDividend(ctx, f.investmentService, input)
	require.NoError(t, err)
	assert.Equal(t, "5", positionQuantity(t, f, f.holdingAccountID, f.stockCommodityID))
	assert.Equal(t, "10", positionQuantity(t, f, other, f.stockCommodityID))
	var lotAccount int64
	require.NoError(t, f.database.QueryRow(`SELECT account_id FROM investment_lots WHERE id = ?`, *result.Replacement.LotID).Scan(&lotAccount))
	assert.Equal(t, other, lotAccount)
	var audits int
	require.NoError(t, f.database.QueryRow(`SELECT count(DISTINCT created_audit_event_id) FROM investment_disposal_revisions`).Scan(&audits))
	assert.Equal(t, 1, audits)
	requireInvestmentSelfCheckPasses(t, f)
}

// Instrument and cost currency both change: the source position empties and
// the replacement opens a lot in the new instrument at its new cost currency,
// funded by income in that currency.
func TestReplaceReinvestedDividendChangesInstrumentAndCostCurrency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	usd := seedTestCurrencyCommodity(t, f.database, "USD")
	corrected := seedTestSecurityCommodity(t, f, "RIGHT")
	original := reinvestOn(t, f, "2026-01-01", 10, 2000)

	replacement := reinvestmentInput(f, "2026-01-01", 10, 2200)
	replacement.CommodityID, replacement.CashCommodityID = corrected, usd
	result, err := acknowledgedReplaceReinvestedDividend(ctx, f.investmentService,
		replaceReinvestment(f, original.Transaction.ID, "wrong ticker and currency", replacement))
	require.NoError(t, err)
	assert.Equal(t, "0", positionQuantity(t, f, f.holdingAccountID, f.stockCommodityID))
	assert.Equal(t, "10", positionQuantity(t, f, f.holdingAccountID, corrected))
	var lotCommodity, lotCost int64
	var basis exact.Coefficient
	require.NoError(t, f.database.QueryRow(`SELECT commodity_id, cost_commodity_id, cost_basis_value FROM investment_lots WHERE id = ?`,
		*result.Replacement.LotID).Scan(&lotCommodity, &lotCost, &basis))
	assert.Equal(t, corrected, lotCommodity)
	assert.Equal(t, usd, lotCost)
	assert.Equal(t, exact.New(2200), basis)
	assert.Zero(t, accountCommodityBalance(t, f, f.incomeAccountID, f.eurCommodityID))
	assert.Equal(t, int64(-2200), accountCommodityBalance(t, f, f.incomeAccountID, usd))
	requireInvestmentSelfCheckPasses(t, f)
}

// A later sale elected the reinvested lot. Moving the reinvestment to another
// holding leaves the election without its lot: the sale is named.
func TestReplaceReinvestedDividendRefusesInvalidatedSpecificLotElection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	other := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	elected := reinvestOn(t, f, "2026-01-01", 10, 2000)
	buyOn(t, f, "2026-02-01", 10, 20000)
	sale := sellInput(f, "2026-03-01", 4)
	sale.CostBasisMethod = "specific_lot"
	sale.LotAllocations = []InvestmentLotAllocationInput{{LotID: *elected.LotID, QuantityValue: exact.New(4)}}
	sold, err := f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)

	replacement := reinvestmentInput(f, "2026-01-01", 10, 2000)
	replacement.HoldingAccountID = other
	_, err = acknowledgedReplaceReinvestedDividend(ctx, f.investmentService,
		replaceReinvestment(f, elected.Transaction.ID, "other broker", replacement))
	var dependency InvestmentBuyDependencyError
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, saleOperationIDForTest(t, f, sold.Transaction.ID), dependency.OperationID)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

// A transferred reinvestment's lot and original date order the destination,
// so moving it names the transfer and writes nothing.
func TestReplaceTransferredReinvestmentFieldChangeNamesTransfer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	reinvestment := reinvestOn(t, f, "2026-05-01", 3, 300)
	transfer := chainTransfer(t, f, f.holdingAccountID, destination, *reinvestment.LotID, "2026-06-01")
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err := acknowledgedReplaceReinvestedDividend(ctx, f.investmentService,
		replaceReinvestment(f, reinvestment.Transaction.ID, "earlier", reinvestmentInput(f, "2026-04-01", 3, 300)))
	var dependency InvestmentBuyDependencyError
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, transferOperationID(t, f, transfer.Transaction.ID), dependency.OperationID)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

// Moving the reinvestment out of a reconciled holding period changes the
// reconciled unit balance: the preview names the checkpoint, the commit needs
// the override, and a refusal writes nothing.
func TestReplaceReinvestedDividendDateAcrossCheckpointRequiresOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	original := reinvestOn(t, f, "2026-01-01", 10, 2000)
	checkpointID := reconcileHolding(t, f, "2026-02-01", 10)
	input := replaceReinvestment(f, original.Transaction.ID, "reinvested after the statement",
		reinvestmentInput(f, "2026-03-01", 10, 2000))
	impact, err := f.investmentService.ReplaceReinvestedDividendReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Len(t, impact.AffectedCheckpoints, 1)
	require.Equal(t, checkpointID, impact.AffectedCheckpoints[0].CheckpointID)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceReinvestedDividend(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))

	input.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{checkpointID}}
	_, err = f.investmentService.ReplaceReinvestedDividend(ctx, input)
	require.NoError(t, err)
	var status string
	require.NoError(t, f.database.QueryRow(`SELECT status FROM reconciliation_checkpoints WHERE id = ?`, checkpointID).Scan(&status))
	require.Equal(t, "invalidated", status)
	requireInvestmentSelfCheckPasses(t, f)
}
