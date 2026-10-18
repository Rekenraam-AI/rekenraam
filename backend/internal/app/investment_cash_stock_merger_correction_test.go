package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #184: a cash-and-stock merger is corrected by its own reversal and
// replacement, and a merger dated behind later activity is admitted by replay
// at its slot. The default merger: 100 shares bought for 1,000.00 become 50
// NEWCO and 300.00 at a fraction of 0.25, realizing +50.00 and carrying
// 750.00.

func reverseCashStockMergerInput(f *investmentsTestFixture, transactionID int64) ReverseCashStockMergerInput {
	return ReverseCashStockMergerInput{OwnerUserID: f.ownerUserID, TransactionID: transactionID, Reason: "wrong merger terms"}
}

func replaceCashStockMergerInput(f *investmentsTestFixture, transactionID, newID int64, date, payable string,
	numerator, denominator, cash int64, fraction string, fractionScale int) ReplaceCashStockMergerInput {
	return ReplaceCashStockMergerInput{OwnerUserID: f.ownerUserID, TransactionID: transactionID, Reason: "wrong merger terms",
		EffectiveOn: date, PayableOn: payable, DestinationCommodityID: newID,
		RatioNumerator: numerator, RatioDenominator: denominator,
		CashAccountID: f.cashAccountID, CurrencyID: f.eurCommodityID, CashValue: cash, CashScale: 2,
		CashBasisFractionValue: exact.Coefficient(fraction), CashBasisFractionScale: fractionScale, Memo: "merger"}
}

// defaultCashStockMerger records the contract example.
func defaultCashStockMerger(t *testing.T, f *investmentsTestFixture, newID int64) CashStockMergerResult {
	t.Helper()
	merged, err := f.investmentService.CashStockMerger(context.Background(),
		cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2))
	require.NoError(t, err)
	return merged
}

func TestCashStockMergerReversalReturnsOldLotsAndRemovesTheCashResult(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	bought := buyOn(t, f, "2026-02-01", 100, 100000)
	clearingBefore := commodityTradingBalance(t, f, f.eurCommodityID)
	merged := defaultCashStockMerger(t, f, newID)

	// The exchange, spin-off and sale corrections do not reach a merger.
	_, err := f.investmentService.ReverseShareExchange(ctx, reverseShareExchangeInput(f, merged.Transaction.ID))
	require.ErrorIs(t, err, ErrShareExchangeNotFound)
	_, err = f.investmentService.ReverseSpinOff(ctx, reverseSpinOffInput(f, merged.Transaction.ID))
	require.ErrorIs(t, err, ErrSpinOffNotFound)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, merged.Transaction.ID)
	require.NoError(t, err)
	assert.True(t, chain.CanCorrectCashStockMerger)
	assert.False(t, chain.CanCorrectShareExchange)

	// The merger's own +50.00 cash result is removed: that needs the shared
	// acknowledgement.
	input := reverseCashStockMergerInput(f, merged.Transaction.ID)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReverseCashStockMerger(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	impact, err := f.investmentService.ReverseCashStockMergerReconciliationImpact(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "refusal and preview write nothing")
	require.NotNil(t, impact.GainImpact)
	require.Len(t, impact.GainImpact.Changes, 1)
	removed := impact.GainImpact.Changes[0]
	assert.Equal(t, db.GainImpactRemoved, removed.Kind)
	requireScaled(t, 30000, 2, removed.Before.Proceeds, "the cash was the decision's proceeds")
	transactionsBefore, auditsBefore := f.transactionCount(t), auditEventCount(t, f)
	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	inverse, err := f.investmentService.ReverseCashStockMerger(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, "2026-06-01", inverse.TransactionDate, "the inverse keeps the merger date")
	require.NotNil(t, inverse.CorrectionOfTransactionID)
	assert.Equal(t, merged.Transaction.ID, *inverse.CorrectionOfTransactionID)
	require.Len(t, inverse.JournalEntries, 2, "the security legs and the cash leg are both inverted")
	assert.Equal(t, [2]string{"2026-06-01", "2026-06-05"}, [2]string{inverse.JournalEntries[0].EntryDate, inverse.JournalEntries[1].EntryDate})
	assert.Equal(t, transactionsBefore+1, f.transactionCount(t))
	assert.Equal(t, auditsBefore+1, auditEventCount(t, f), "one audit event for the whole correction")
	assert.Equal(t, "100", positionQuantity(t, f, f.holdingAccountID, f.stockCommodityID), "the old lot returns")
	requireScaled(t, 100000, 2, lotRemainingBasis(t, f, *bought.LotID), "with its whole basis")
	assert.Equal(t, "0", positionQuantity(t, f, f.holdingAccountID, newID))
	assert.Equal(t, "closed", lotStatus(t, f, merged.Plan.Links[0].DestinationLotID), "the new lot is retired, not deleted")
	assert.Zero(t, clearingBefore.Cmp(commodityTradingBalance(t, f, f.eurCommodityID)), "the cash leg is inverted")
	assert.Zero(t, commodityTradingBalance(t, f, newID).Sign())
	gains, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
	require.NoError(t, err)
	assert.Empty(t, gains, "the cash decision left effective history")
	var decisions, realizations int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_disposal_decisions`).Scan(&decisions))
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_lot_events
		WHERE event_kind = 'disposal' AND quantity_value = '0'`).Scan(&realizations))
	assert.Equal(t, [2]int{1, 1}, [2]int{decisions, realizations}, "the decision and realization stay as evidence")
	requireInvestmentSelfCheckPasses(t, f)

	chain, err = f.investmentService.CorrectionChain(ctx, f.ownerUserID, merged.Transaction.ID)
	require.NoError(t, err)
	assert.False(t, chain.CanCorrectCashStockMerger)
	assert.Nil(t, chain.EffectiveCashStockMerger)
	_, err = f.investmentService.ReverseCashStockMerger(ctx, input)
	require.ErrorIs(t, err, ErrCashStockMergerAlreadyCorrected)
	_, err = f.investmentService.ReverseCashStockMerger(ctx, reverseCashStockMergerInput(f, bought.Transaction.ID))
	require.ErrorIs(t, err, ErrCashStockMergerNotFound)
}

// The new units were sold; reversing the merger would leave that sale without
// units. The sale is named and nothing is written.
func TestCashStockMergerReversalNamesANewInstrumentSaleWithoutWriting(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	merged := defaultCashStockMerger(t, f, newID)
	sale := sellInput(f, "2026-07-01", 20)
	sale.CommodityID = newID
	sold, err := f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)

	_, err = f.investmentService.ReverseCashStockMergerReconciliationImpact(ctx, reverseCashStockMergerInput(f, merged.Transaction.ID))
	var dependency InvestmentTransferDependencyError
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, transferOperationID(t, f, sold.Transaction.ID), dependency.OperationID)
	_, err = f.investmentService.ReverseCashStockMerger(ctx, reverseCashStockMergerInput(f, merged.Transaction.ID))
	require.ErrorIs(t, err, ErrInvestmentTransferDependency)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

// A new lot moved on to another holding; reversing the merger would remove it,
// so the onward transfer is named.
func TestCashStockMergerReversalNamesAnOnwardTransfer(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	destinationID := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	buyOn(t, f, "2026-02-01", 100, 100000)
	merged := defaultCashStockMerger(t, f, newID)
	onward := internalTransferFromLot(f, destinationID, merged.Plan.Links[0].DestinationLotID, exact.New(10), 0)
	onward.CommodityID, onward.EffectiveOn = newID, "2026-07-01"
	moved, err := f.investmentService.InternalTransfer(ctx, onward)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)

	_, err = f.investmentService.ReverseCashStockMerger(ctx, reverseCashStockMergerInput(f, merged.Transaction.ID))
	var dependency InvestmentTransferDependencyError
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, transferOperationID(t, f, moved.Transaction.ID), dependency.OperationID)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

// A cash in lieu settles the fraction the merger left; correcting the merger
// first would leave it settling nothing, so it is named.
func TestCashStockMergerCorrectionNamesASettlingCashInLieu(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 3, 3000)
	merged, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 3, 2, 900, "2", 1))
	require.NoError(t, err)
	settled, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, merged.Transaction.ID))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)

	_, err = f.investmentService.ReverseCashStockMerger(ctx, reverseCashStockMergerInput(f, merged.Transaction.ID))
	var dependency InvestmentTransferDependencyError
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, transferOperationID(t, f, settled.Transaction.ID), dependency.OperationID)
	_, err = f.investmentService.ReplaceCashStockMerger(ctx, replaceCashStockMergerInput(f, merged.Transaction.ID, newID,
		"2026-06-01", "2026-06-05", 3, 2, 900, "3", 1))
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

// A replacement with another election, cash, ratio and instrument restates
// the merger's own cash result, which needs the acknowledgement; a stale one
// is refused. The chain then explains the replacement.
func TestCashStockMergerReplacementRestatesTheCashResultWithAcknowledgement(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	otherID := seedTestSecurityCommodity(t, f, "OTHERCO")
	bought := buyOn(t, f, "2026-02-01", 100, 100000)
	merged := defaultCashStockMerger(t, f, newID)

	// 0.5 of 1,000.00 against 400.00 of cash: -100.00, and 500.00 carried.
	input := replaceCashStockMergerInput(f, merged.Transaction.ID, otherID, "2026-06-01", "2026-06-10", 2, 2, 40000, "5", 1)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err := f.investmentService.ReplaceCashStockMerger(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	input.GainImpactAcknowledgement = "stale"
	_, err = f.investmentService.ReplaceCashStockMerger(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementStale)
	input.GainImpactAcknowledgement = ""
	preview, err := f.investmentService.PreviewCashStockMergerReplacement(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "refusals and preview write nothing")
	require.NotNil(t, preview.Impact.GainImpact)
	require.Len(t, preview.Impact.GainImpact.Changes, 1)
	change := preview.Impact.GainImpact.Changes[0]
	assert.Equal(t, db.GainImpactReplaced, change.Kind, "the replacement's decision supersedes the merger's")
	requireScaled(t, 25000, 2, change.Before.DisposedBasis, "realized before")
	require.NotNil(t, change.After)
	requireScaled(t, 50000, 2, change.After.DisposedBasis, "realized after")
	requireScaled(t, 40000, 2, change.After.Proceeds, "the corrected cash")
	auditsBefore := auditEventCount(t, f)
	input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	result, err := f.investmentService.ReplaceCashStockMerger(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, merged.Transaction.ID, result.CorrectedTransactionID)
	require.NotNil(t, result.Replacement.CorrectionOfTransactionID)
	assert.Equal(t, merged.Transaction.ID, *result.Replacement.CorrectionOfTransactionID)
	assert.Equal(t, [2]int64{1, 1}, [2]int64{result.Plan.RatioNumerator, result.Plan.RatioDenominator}, "stored in lowest terms")
	require.Len(t, result.Plan.Links, 1)
	link := result.Plan.Links[0]
	assert.Equal(t, *bought.LotID, link.SourceLotID)
	assert.Equal(t, "100", link.DestinationQuantityValue.String())
	requireScaled(t, 50000, 2, exact.ScaledIntFromInt64(link.RealizedBasisValue, link.RealizedBasisScale), "realized")
	requireScaled(t, 50000, 2, exact.ScaledIntFromInt64(link.CarriedBasisValue, link.CarriedBasisScale), "carried")
	committed := link
	committed.DestinationLotID = 0
	assert.Equal(t, preview.Plan.Links[0], committed, "the preview showed what the commit wrote")
	require.Len(t, result.Replacement.JournalEntries, 2)
	assert.Equal(t, "2026-06-10", result.Replacement.JournalEntries[1].EntryDate, "the corrected payable date")
	_, proceeds, basis, _ := mergerDecision(t, f, result.Replacement.ID)
	requireScaled(t, 40000, 2, proceeds, "the replacement decision's proceeds")
	requireScaled(t, 50000, 2, basis, "and realized basis")
	assert.Equal(t, "0", positionQuantity(t, f, f.holdingAccountID, newID))
	assert.Equal(t, "closed", lotStatus(t, f, merged.Plan.Links[0].DestinationLotID))
	assert.Equal(t, "100", positionQuantity(t, f, f.holdingAccountID, otherID))
	requireScaled(t, 50000, 2, lotRemainingBasis(t, f, link.DestinationLotID), "the new lot carries the corrected rest")
	assert.Equal(t, auditsBefore+1, auditEventCount(t, f))
	gains, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
	require.NoError(t, err)
	require.Len(t, gains, 1, "only the replacement's cash result is effective")
	requireScaled(t, -10000, 2, exact.ScaledIntFromInt64(gains[0].RealizedGainValue, gains[0].RealizedGainScale), "the corrected result")
	var evidence string
	require.NoError(t, f.database.QueryRow(`SELECT source_evidence_json FROM investment_transfer_facts
		WHERE destination_commodity_id = ?`, otherID).Scan(&evidence))
	assert.JSONEq(t, `{"notice":"merger terms","cash_per_share":"3.00"}`, evidence, "omitted evidence keeps the recorded evidence")
	requireInvestmentSelfCheckPasses(t, f)

	// The chain explains the effective replacement.
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, merged.Transaction.ID)
	require.NoError(t, err)
	assert.True(t, chain.CanCorrectCashStockMerger)
	terms := chain.EffectiveCashStockMerger
	require.NotNil(t, terms)
	assert.Equal(t, [4]int64{f.holdingAccountID, f.holdingAccountID, otherID, f.cashAccountID},
		[4]int64{terms.HoldingAccountID, terms.DestinationHoldingAccountID, terms.DestinationCommodityID, terms.CashAccountID})
	assert.Equal(t, [2]string{"2026-06-01", "2026-06-10"}, [2]string{terms.EffectiveOn, terms.PayableOn})
	assert.Equal(t, f.eurCommodityID, terms.CurrencyID)
	requireScaled(t, 40000, 2, exact.ScaledIntFromCoefficient(terms.CashValue, terms.CashScale), "the cash")
	assert.Equal(t, [2]any{exact.Coefficient("5"), 1}, [2]any{terms.Plan.CashBasisFractionValue, terms.Plan.CashBasisFractionScale})
	require.Len(t, terms.Plan.Links, 1)
	assert.Equal(t, link.DestinationLotID, terms.Plan.Links[0].DestinationLotID)
	requireScaled(t, 50000, 2, exact.ScaledIntFromInt64(terms.Plan.Links[0].RealizedBasisValue, terms.Plan.Links[0].RealizedBasisScale), "realized")
	requireScaled(t, 40000, 2, exact.ScaledIntFromInt64(terms.Plan.Links[0].CashValue, terms.Plan.Links[0].CashScale), "the lot's cash")
	require.Len(t, terms.Plan.Totals, 1)
	requireScaled(t, -10000, 2, exact.ScaledIntFromCoefficient(terms.Plan.Totals[0].ResultValue, terms.Plan.Totals[0].ResultScale), "the result")

	_, err = f.investmentService.ReplaceCashStockMerger(ctx, input)
	require.ErrorIs(t, err, ErrCashStockMergerAlreadyCorrected)
}

// Moving the merger later merges a purchase made after the original date:
// every lot open at the new slot is realized and taken.
func TestCashStockMergerReplacementMovedLaterTakesTheHoldingThen(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	merged := defaultCashStockMerger(t, f, newID)
	late := buyOn(t, f, "2026-08-01", 20, 30000)

	input := replaceCashStockMergerInput(f, merged.Transaction.ID, newID, "2026-09-01", "2026-09-05", 1, 2, 36000, "25", 2)
	preview, err := f.investmentService.PreviewCashStockMergerReplacement(ctx, input)
	require.NoError(t, err)
	if preview.Impact.GainImpact != nil {
		input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	}
	result, err := f.investmentService.ReplaceCashStockMerger(ctx, input)
	require.NoError(t, err)
	require.Len(t, result.Plan.Links, 2)
	assert.Equal(t, "60", result.Plan.DestinationQuantityValue.String())
	assert.Equal(t, *late.LotID, result.Plan.Links[1].SourceLotID)
	requireScaled(t, 7500, 2, exact.ScaledIntFromInt64(result.Plan.Links[1].RealizedBasisValue, result.Plan.Links[1].RealizedBasisScale),
		"a quarter of the late lot's 300.00")
	requireScaled(t, 6000, 2, exact.ScaledIntFromInt64(result.Plan.Links[1].CashValue, result.Plan.Links[1].CashScale),
		"its 20 of 120 units share the cash")
	assert.Equal(t, "0", positionQuantity(t, f, f.holdingAccountID, f.stockCommodityID))
	assert.Equal(t, "60", positionQuantity(t, f, f.holdingAccountID, newID))
	requireInvestmentSelfCheckPasses(t, f)
}

// A replacement must change something; equivalent terms (2:4 for 1:2, 0.250
// for 0.25) and the recorded evidence sent back are not a change.
func TestCashStockMergerReplacementMustChangeTheTerms(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	merged := defaultCashStockMerger(t, f, newID)
	_, err := f.investmentService.ReplaceCashStockMerger(ctx, replaceCashStockMergerInput(f, merged.Transaction.ID, newID,
		"2026-06-01", "2026-06-05", 2, 4, 30000, "250", 3))
	assert.ErrorAs(t, err, &ValidationError{})
	same := replaceCashStockMergerInput(f, merged.Transaction.ID, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2)
	same.SourceEvidenceJSON = `{"cash_per_share":"3.00","notice":"merger terms"}`
	_, err = f.investmentService.ReplaceCashStockMerger(ctx, same)
	assert.ErrorAs(t, err, &ValidationError{})
	same.SourceEvidenceJSON = ""
	same.PayableOn = "2026-06-08"
	_, err = f.investmentService.PreviewCashStockMergerReplacement(ctx, same)
	assert.NoError(t, err, "a corrected payable date alone is a correction")
	input := reverseCashStockMergerInput(f, merged.Transaction.ID)
	input.Reason = " "
	_, err = f.investmentService.ReverseCashStockMerger(ctx, input)
	assert.ErrorAs(t, err, &ValidationError{})
}

// A merger dated behind a later sale of the old holding takes every lot open
// at its slot. The later sale is revised to the lot it can still take, a
// restated gain that needs the acknowledgement.
func TestCashStockMergerBackdatedBehindAnOldHoldingSaleRevisesIt(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	early := buyOn(t, f, "2026-01-01", 10, 10000)
	late := buyOn(t, f, "2026-08-01", 5, 10000)
	sale := sellInput(f, "2026-09-01", 3)
	sale.CostBasisMethod = "fifo"
	sold, err := f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)
	requireScaled(t, 3000, 2, saleEffectiveBasis(t, f, sold.Transaction.ID), "FIFO took the January lot")

	input := cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 1, 1000, "1", 1)
	before := shareExchangeWriteCounts(t, f.database)
	_, err = f.investmentService.CashStockMerger(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	assert.Equal(t, before, shareExchangeWriteCounts(t, f.database), "nothing is written on refusal")
	preview, err := f.investmentService.PreviewCashStockMerger(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, before, shareExchangeWriteCounts(t, f.database), "preview writes nothing")
	require.NotNil(t, preview.Impact.GainImpact)
	require.Len(t, preview.Impact.GainImpact.Changes, 1, "the merger's own new result is not a restatement")
	requireScaled(t, 6000, 2, preview.Impact.GainImpact.Changes[0].After.DisposedBasis, "three August units at 20.00")
	input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	result, err := f.investmentService.CashStockMerger(ctx, input)
	require.NoError(t, err)

	require.Len(t, result.Plan.Links, 1)
	link := result.Plan.Links[0]
	assert.Equal(t, *early.LotID, link.SourceLotID, "only the holding open in June is merged")
	assert.Equal(t, "10", link.SourceQuantityValue.String(), "with the units held in June, before the sale")
	requireScaled(t, 1000, 2, exact.ScaledIntFromInt64(link.RealizedBasisValue, link.RealizedBasisScale), "a tenth of the June basis")
	requireScaled(t, 9000, 2, exact.ScaledIntFromInt64(link.CarriedBasisValue, link.CarriedBasisScale), "the rest carries")
	committed := link
	committed.DestinationLotID = 0
	assert.Equal(t, preview.Plan.Links[0], committed, "the preview showed what the commit wrote")
	requireScaled(t, 6000, 2, saleEffectiveBasis(t, f, sold.Transaction.ID), "the sale now takes the August lot")
	requireScaled(t, 4000, 2, lotRemainingBasis(t, f, *late.LotID), "two August units remain")
	assert.Equal(t, "closed", lotStatus(t, f, *early.LotID))
	assert.Equal(t, "2", positionQuantity(t, f, f.holdingAccountID, f.stockCommodityID))
	assert.Equal(t, "10", positionQuantity(t, f, f.holdingAccountID, newID))
	requireScaled(t, 9000, 2, lotRemainingBasis(t, f, link.DestinationLotID), "the new lot opens at the carried basis")
	_, proceeds, basis, _ := mergerDecision(t, f, result.Transaction.ID)
	requireScaled(t, 1000, 2, proceeds, "the cash")
	requireScaled(t, 1000, 2, basis, "the realized part")
	requireInvestmentSelfCheckPasses(t, f)

	// The backdated merger is itself correctable; undoing it restates the
	// sale and removes its own result.
	impact, err := f.investmentService.ReverseCashStockMergerReconciliationImpact(ctx, reverseCashStockMergerInput(f, result.Transaction.ID))
	require.NoError(t, err)
	require.NotNil(t, impact.GainImpact)
	assert.Len(t, impact.GainImpact.Changes, 2)
}

// A merger dated before the whole holding was sold leaves that sale without
// units: the sale is named and nothing is written.
func TestCashStockMergerBackdatedBehindAFullSaleNamesIt(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-01-01", 10, 10000)
	sold, err := f.investmentService.Sell(ctx, sellInput(f, "2026-07-01", 10))
	require.NoError(t, err)
	before := shareExchangeWriteCounts(t, f.database)

	_, err = f.investmentService.PreviewCashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 1, 1000, "1", 1))
	var dependency InvestmentTransferDependencyError
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, transferOperationID(t, f, sold.Transaction.ID), dependency.OperationID)
	_, err = f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 1, 1000, "1", 1))
	require.ErrorAs(t, err, &dependency)
	assert.Equal(t, before, shareExchangeWriteCounts(t, f.database))
}

// A merger dated behind a later sale of the new instrument opens its lots by
// replay admission. FIFO orders by original acquisition date, so the July
// sale now takes the merged units that follow the January purchase.
func TestCashStockMergerBackdatedBehindANewInstrumentSaleRevisesIt(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	bought := buyOn(t, f, "2026-01-01", 10, 20000)
	newBuy := tradeOn(f, f.holdingAccountID, "2026-02-01", 5, 5000)
	newBuy.CommodityID = newID
	_, err := f.investmentService.Buy(ctx, newBuy)
	require.NoError(t, err)
	sale := sellInput(f, "2026-07-01", 5)
	sale.CommodityID, sale.CostBasisMethod = newID, "fifo"
	sold, err := f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)
	requireScaled(t, 5000, 2, saleEffectiveBasis(t, f, sold.Transaction.ID), "the only NEWCO lot")

	input := cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 1, 1000, "25", 2)
	preview, err := f.investmentService.PreviewCashStockMerger(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, preview.Impact.GainImpact)
	input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	result, err := f.investmentService.CashStockMerger(ctx, input)
	require.NoError(t, err)

	requireScaled(t, 7500, 2, saleEffectiveBasis(t, f, sold.Transaction.ID), "five of the ten merged units at 15.00")
	assert.Equal(t, "closed", lotStatus(t, f, *bought.LotID))
	assert.Equal(t, "10", positionQuantity(t, f, f.holdingAccountID, newID))
	requireScaled(t, 7500, 2, lotRemainingBasis(t, f, result.Plan.Links[0].DestinationLotID), "half the carried basis remains")
	requireInvestmentSelfCheckPasses(t, f)
}

// The export keeps the whole correction chain: the original merger, its
// reversal, and a replacement of another merger, each linked to what it
// corrects, with every original fact still present.
func TestCashStockMergerCorrectionsExportAsOperations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	first := defaultCashStockMerger(t, f, newID)
	impact, err := f.investmentService.ReverseCashStockMergerReconciliationImpact(ctx, reverseCashStockMergerInput(f, first.Transaction.ID))
	require.NoError(t, err)
	reverse := reverseCashStockMergerInput(f, first.Transaction.ID)
	reverse.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	reversal, err := f.investmentService.ReverseCashStockMerger(ctx, reverse)
	require.NoError(t, err)
	second, err := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, newID, "2026-06-02", "2026-06-05", 1, 2, 30000, "25", 2))
	require.NoError(t, err)
	replace := replaceCashStockMergerInput(f, second.Transaction.ID, newID, "2026-06-02", "2026-06-05", 1, 1, 30000, "25", 2)
	preview, err := f.investmentService.PreviewCashStockMergerReplacement(ctx, replace)
	require.NoError(t, err)
	if preview.Impact.GainImpact != nil {
		replace.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	}
	replaced, err := f.investmentService.ReplaceCashStockMerger(ctx, replace)
	require.NoError(t, err)

	snapshot, err := db.NewExportRepository(f.database).Snapshot(ctx)
	require.NoError(t, err)
	var out bytes.Buffer
	_, err = NewExportService(db.NewExportRepository(f.database)).writeInvestmentOperationsCSV(ctx, &out, snapshot)
	require.NoError(t, err)
	require.NoError(t, snapshot.Rollback())
	rows, err := csv.NewReader(bytes.NewReader(out.Bytes())).ReadAll()
	require.NoError(t, err)
	byTransaction := make(map[string][]string)
	for _, row := range rows[1:] {
		byTransaction[row[1]] = row
	}
	reversed := byTransaction[strconv.FormatInt(reversal.ID, 10)]
	require.Equal(t, []string{"reversal", "reverse", "wrong merger terms"}, []string{reversed[2], reversed[6], reversed[7]})
	require.Equal(t, byTransaction[strconv.FormatInt(first.Transaction.ID, 10)][0], reversed[5])
	replacement := byTransaction[strconv.FormatInt(replaced.Replacement.ID, 10)]
	original := byTransaction[strconv.FormatInt(second.Transaction.ID, 10)]
	require.Equal(t, []string{"cash_and_stock_merger", original[0], "replace", "wrong merger terms"},
		[]string{replacement[2], replacement[5], replacement[6], replacement[7]})
	var facts, decisions int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_transfer_facts WHERE cash_basis_fraction_value IS NOT NULL`).Scan(&facts))
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_disposal_decisions`).Scan(&decisions))
	assert.Equal(t, [2]int{3, 3}, [2]int{facts, decisions}, "every merger fact and cash decision stays as evidence")
	assert.Equal(t, "100", positionQuantity(t, f, f.holdingAccountID, newID))
	requireInvestmentSelfCheckPasses(t, f)
}

// A correction is planned against the merger outside its write. If another
// command corrects the merger first, the stale correction must refuse inside
// the write instead of posting a second inverse.
func TestCashStockMergerCorrectionRechecksTheMergerInsideTheWrite(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	bought := buyOn(t, f, "2026-02-01", 100, 100000)
	merged := defaultCashStockMerger(t, f, newID)
	operation, reversal, err := f.investmentService.prepareCashStockMergerReversalWrite(ctx, reverseCashStockMergerInput(f, merged.Transaction.ID))
	require.NoError(t, err)
	replacedOperation, inverse, replacement, merger, err := f.investmentService.prepareCashStockMergerReplacementWrite(ctx,
		replaceCashStockMergerInput(f, merged.Transaction.ID, newID, "2026-06-01", "2026-06-05", 1, 1, 30000, "25", 2))
	require.NoError(t, err)

	impact, err := f.investmentService.ReverseCashStockMergerReconciliationImpact(ctx, reverseCashStockMergerInput(f, merged.Transaction.ID))
	require.NoError(t, err)
	first := reverseCashStockMergerInput(f, merged.Transaction.ID)
	first.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	_, err = f.investmentService.ReverseCashStockMerger(ctx, first)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.repository.ReverseCashStockMerger(ctx, reversal, operation)
	require.ErrorIs(t, err, db.ErrInvestmentOperationAlreadyCorrected)
	_, err = f.investmentService.repository.ReplaceCashStockMerger(ctx, replacedOperation, inverse, replacement, merger)
	require.ErrorIs(t, err, db.ErrInvestmentOperationAlreadyCorrected)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	requireScaled(t, 100000, 2, lotRemainingBasis(t, f, *bought.LotID), "exactly one reversal took effect")
}

// The reconciled new instrument would change. The guard runs after every
// replay and must roll all of them back; the override admits it.
func TestCashStockMergerReversalLateReconciliationRefusalRollsBackEverything(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	bought := buyOn(t, f, "2026-02-01", 100, 100000)
	merged := defaultCashStockMerger(t, f, newID)
	reconcileInstrument(t, f, newID, "2026-06-30", 50)
	input := reverseCashStockMergerInput(f, merged.Transaction.ID)
	impact, err := f.investmentService.ReverseCashStockMergerReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.NotEmpty(t, impact.AffectedCheckpoints)
	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	before := buyReplacementPreviewSnapshot(t, f.database)

	_, err = f.investmentService.ReverseCashStockMerger(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	assert.Equal(t, "closed", lotStatus(t, f, *bought.LotID), "the old lot stays merged")

	input.ReconciliationOverride = unboundOverride
	_, err = f.investmentService.ReverseCashStockMerger(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "0", positionQuantity(t, f, f.holdingAccountID, newID))
	requireScaled(t, 100000, 2, lotRemainingBasis(t, f, *bought.LotID), "the old basis is back")
	requireInvestmentSelfCheckPasses(t, f)
}
