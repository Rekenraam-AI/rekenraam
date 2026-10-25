package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dividend ex dates (investment-operation-refactor-plan.md, "Dividend ex
// dates"): optional, no later than the payment date, kept through
// corrections and carried from a provider dividend, so R13 recognises the
// income on its ex date (D2).

// dividendExDate reads the effective dividend's ex date as the correction
// terms expose it.
func dividendExDate(t *testing.T, service *InvestmentService, owner, transactionID int64) string {
	t.Helper()
	chain, err := service.CorrectionChain(context.Background(), owner, transactionID)
	require.NoError(t, err)
	require.NotNil(t, chain.EffectiveDividend)
	return chain.EffectiveDividend.ExDate
}

// operationExDates lists the 'ex' dates of every operation a transaction is
// the primary journal of.
func operationExDates(t *testing.T, f *returnsFixture, transactionID int64) []string {
	t.Helper()
	rows, err := f.database.Query(`SELECT d.event_date FROM investment_operation_dates d
		JOIN investment_operation_journal_links l ON l.operation_id = d.operation_id AND l.role = 'primary'
		JOIN transaction_versions tv ON tv.id = l.transaction_version_id
		WHERE tv.transaction_id = ? AND d.date_role = 'ex'`, transactionID)
	require.NoError(t, err)
	defer rows.Close()
	dates := []string{}
	for rows.Next() {
		var date string
		require.NoError(t, rows.Scan(&date))
		dates = append(dates, date)
	}
	require.NoError(t, rows.Err())
	return dates
}

func TestDividendExDateIsOptionalAndNoLaterThanPayment(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)

	none := f.dividendOf(t, "2026-02-20", "", &f.stockCommodityID, 2000, 0, false)
	assert.Empty(t, dividendExDate(t, f.investmentService, f.ownerUserID, none.ID))
	assert.Empty(t, operationExDates(t, f, none.ID))

	sameDay := f.dividendOf(t, "2026-02-20", "2026-02-20", &f.stockCommodityID, 2000, 0, false)
	assert.Equal(t, "2026-02-20", dividendExDate(t, f.investmentService, f.ownerUserID, sameDay.ID))

	// A separate tax payment carries its distribution's ex date (I3b).
	tax := f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 300, 0, true)
	assert.Equal(t, "2026-02-10", dividendExDate(t, f.investmentService, f.ownerUserID, tax.ID))
	assert.Equal(t, []string{"2026-02-10"}, operationExDates(t, f, tax.ID))

	before := countTransactions(t, f.database)
	for _, exDate := range []string{"2026-02-21", "2026-13-01", "10-02-2026"} {
		_, err := f.investmentService.Dividend(context.Background(), DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-02-20",
			ExDate: NullableValue(exDate), CommodityID: NullableValue(f.stockCommodityID), CashAccountID: f.brokerCashID,
			CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.incomeAccountID, AmountValue: 2000, AmountScale: 2})
		var validation ValidationError
		require.ErrorAs(t, err, &validation, exDate)
		assert.Contains(t, validation.Message, "ex-date")
	}
	assert.Equal(t, before, countTransactions(t, f.database), "a refused ex date writes nothing")
}

func TestDividendExDateFollowsTheCorrectionPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	original := f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 0, false)

	replacement := func(transactionID int64, exDate NullablePatch[string]) (ReplaceInvestmentDividendResult, error) {
		return f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
			TransactionID: transactionID, Reason: "fix", Replacement: DividendInput{TransactionDate: "2026-02-20",
				ExDate: exDate, CashAccountID: f.brokerCashID, CashCommodityID: f.eurCommodityID,
				IncomeAccountID: &f.incomeAccountID, AmountValue: 2100, AmountScale: 2}})
	}

	// Omission preserves the recorded ex date.
	kept, err := replacement(original.ID, NullablePatch[string]{})
	require.NoError(t, err)
	assert.Equal(t, "2026-02-10", dividendExDate(t, f.investmentService, f.ownerUserID, kept.Replacement.ID))

	// A value replaces it; one after the fixed payment date is refused.
	_, err = replacement(kept.Replacement.ID, NullableValue("2026-02-21"))
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	moved, err := replacement(kept.Replacement.ID, NullableValue("2026-02-11"))
	require.NoError(t, err)
	assert.Equal(t, "2026-02-11", dividendExDate(t, f.investmentService, f.ownerUserID, moved.Replacement.ID))

	// Null clears it.
	cleared, err := replacement(moved.Replacement.ID, NullablePatch[string]{Set: true})
	require.NoError(t, err)
	assert.Empty(t, dividendExDate(t, f.investmentService, f.ownerUserID, cleared.Replacement.ID))
	assert.Empty(t, operationExDates(t, f, cleared.Replacement.ID))

	// A reversal repeats the original's ex date, so the two cancel on one day.
	other := f.dividendOf(t, "2026-03-20", "2026-03-10", &f.stockCommodityID, 500, 0, false)
	reversal, err := f.investmentService.ReverseDividend(ctx, ReverseInvestmentDividendInput{OwnerUserID: f.ownerUserID,
		TransactionID: other.ID, Reason: "duplicate"})
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-03-10"}, operationExDates(t, f, reversal.ID))
	requireKnownBasisSelfCheckPasses(t, f.database)
}

// I2 reached by correction: a dividend recorded without its ex date is
// not_held for an owner who sold before the pay date. Replacing it with the
// ex date moves the whole flow into the sleeve; the inverse journal is read
// with the original's facts, so nothing stays behind on the payment date.
func TestInstrumentReturnsReplacementAddingTheExDateAttributesTheDividend(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-01-31", 10000)
	f.price(t, "2026-02-10", 9800)
	f.trade(t, false, f.holdingAccountID, "2026-02-15", "", 10, 105000)
	dividend := f.dividendOf(t, "2026-02-20", "", &f.stockCommodityID, 2000, 300, false)

	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	require.Len(t, instruments.NotAttributed, 1)
	require.Equal(t, ReturnsFallbackNotHeld, instruments.NotAttributed[0].Reason)
	requireReturnsAmount(t, 0, sleeveRow(t, instruments, f.stockCommodityID).Income)

	withholding := int64(300)
	_, err := f.investmentService.ReplaceDividend(context.Background(), ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
		TransactionID: dividend.ID, Reason: "record the ex date", Replacement: DividendInput{TransactionDate: "2026-02-20",
			ExDate: NullableValue("2026-02-10"), CashAccountID: f.brokerCashID, CashCommodityID: f.eurCommodityID,
			IncomeAccountID: &f.incomeAccountID, AmountValue: 2000, AmountScale: 2,
			WithholdingValue: &withholding, WithholdingAccountID: &f.feeID}})
	require.NoError(t, err)

	portfolio, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 17, row.Income)
	requireReturnsAmount(t, 1067, row.Withdrawals)
	requireReturnsAmount(t, 67, row.Gain)
	requireReturn(t, "0.068214", row.TimeWeighted)
	requireReturnsAmount(t, 67, portfolio.Gain)
	requireReturnsAmount(t, 0, instruments.NotAttributedGain)
	// The original and its inverse are both not_held on the payment date and
	// cancel; only they are listed.
	for _, entry := range instruments.NotAttributed {
		assert.Equal(t, "2026-02-20", entry.Date)
	}
}

// R3 with an ex date: a reversed dividend recognised on its ex date leaves
// no income in the sleeve.
func TestInstrumentReturnsReversedExDateDividendCancels(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-01-31", 10000)
	f.price(t, "2026-02-28", 10000)
	dividend := f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 0, false)
	_, err := f.investmentService.ReverseDividend(context.Background(), ReverseInvestmentDividendInput{OwnerUserID: f.ownerUserID,
		TransactionID: dividend.ID, Reason: "duplicate"})
	require.NoError(t, err)

	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 0, row.Income)
	requireReturnsAmount(t, 0, row.Withdrawals)
	requireReturnsAmount(t, 0, row.Gain)
	require.Empty(t, instruments.NotAttributed)
}

func TestProviderDividendCompletionRecordsTheEventExDate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")

	// The payment cannot precede the ex date it was paid for.
	before := countTransactions(t, f.database)
	_, err := f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, f.completion("2026-07-28", 1600))
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Contains(t, validation.Message, "ex-date")
	assert.Equal(t, before, countTransactions(t, f.database))
	assert.Equal(t, "suggested", suggestionForExDate(t, f.suggestions(t), "2026-07-29").Status)

	accepted, err := f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, f.completion("2026-08-14", 1600))
	require.NoError(t, err)
	require.NotNil(t, accepted.GeneratedTransactionID)
	assert.Equal(t, "2026-07-29", dividendExDate(t, f.investmentSvc, 1, *accepted.GeneratedTransactionID))
	requireKnownBasisSelfCheckPasses(t, f.database)
}

func TestProviderDividendLinkRequiresAMatchingRecordedExDate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	record := func(exDate NullablePatch[string]) Transaction {
		transaction, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", ExDate: exDate,
			CommodityID: NullableValue(f.instrument.CommodityID), CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID,
			IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2})
		require.NoError(t, err)
		return transaction
	}
	other := record(NullableValue("2026-07-28"))
	matching := record(NullableValue("2026-07-29"))
	unrecorded := record(NullablePatch[string]{})
	f.fetch(t)

	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	candidates := []int64{}
	for _, candidate := range july.LinkCandidates {
		candidates = append(candidates, candidate.TransactionID)
	}
	assert.ElementsMatch(t, []int64{matching.ID, unrecorded.ID}, candidates)

	_, err := f.investmentSvc.LinkSuggestion(ctx, 1, 0, "req", july.ID, other.ID)
	var validation ValidationError
	require.ErrorAs(t, err, &validation, "a dividend paid for another ex date is not this event")
	linked, err := f.investmentSvc.LinkSuggestion(ctx, 1, 0, "req", july.ID, unrecorded.ID)
	require.NoError(t, err)
	assert.Equal(t, "linked", linked.Status)
	assert.Empty(t, dividendExDate(t, f.investmentSvc, 1, unrecorded.ID), "linking writes no facts")
}
