package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// R13 returns_v1 against real ledgers. Every expected figure is a literal
// from the plan's independent table (docs/plans/investment-returns-plan.md);
// none is derived from the code under test. The portfolio is the default
// investment scope: broker cash plus the TEST holding. The fixture's checking
// account is outside it.

type returnsFixture struct {
	*investmentsTestFixture
	pricing      *PricingService
	brokerCashID int64
	equityID     int64
	feeID        int64
}

func newReturnsFixture(t *testing.T) *returnsFixture {
	t.Helper()
	base := newInvestmentsTestFixture(t)
	now := time.Date(2027, 2, 1, 12, 0, 0, 0, time.UTC)
	base.investmentService.SetNowForTest(func() time.Time { return now })
	base.transactionService.now = func() time.Time { return now }
	f := &returnsFixture{
		investmentsTestFixture: base,
		pricing:                NewPricingService(db.NewPricingRepository(base.database)),
		brokerCashID:           seedTestAccountWithClass(t, base.database, "active", true, "asset", "brokerage_cash"),
		equityID:               seedTestAccountWithClass(t, base.database, "active", true, "equity", "equity"),
		feeID:                  seedTestAccountWithClass(t, base.database, "active", true, "expense", "expense"),
	}
	f.pricing.SetNowForTest(func() time.Time { return now })

	// Before the period: checking is funded, 1,100 moves to the broker and
	// 10 TEST are bought for 1,000. At the opening boundary (2026-01-31)
	// TEST closes at 100, so V(open) = 100 cash + 1,000 = 1,100.
	f.post(t, "2026-01-02", f.cashAccountID, f.equityID, 1000000)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 110000)
	_, err := f.investmentService.Buy(context.Background(), InvestmentTradeInput{
		OwnerUserID: f.ownerUserID, TransactionDate: "2026-01-02", CommodityID: f.stockCommodityID,
		HoldingAccountID: f.holdingAccountID, CashAccountID: f.brokerCashID,
		QuantityValue: exact.New(10), CashAmountValue: 100000, CashAmountScale: 2, CashCommodityID: f.eurCommodityID,
	})
	require.NoError(t, err)
	f.price(t, "2026-01-31", 10000)
	return f
}

// post moves an EUR amount (in cents) into debit from credit.
func (f *returnsFixture) post(t *testing.T, date string, debit, credit int64, cents int64) {
	t.Helper()
	_, err := f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api",
		Spec: TransactionInput{TransactionDate: date, JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: debit, CommodityID: f.eurCommodityID, QuantityValue: exact.New(cents), QuantityScale: 2},
			{AccountID: credit, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-cents), QuantityScale: 2},
		}}}},
	})
	require.NoError(t, err)
}

func (f *returnsFixture) price(t *testing.T, date string, cents int64) {
	t.Helper()
	_, err := f.pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID,
		BaseCommodityID: f.stockCommodityID, QuoteCommodityID: f.eurCommodityID, QuoteType: "close", AdjustmentBasis: "raw",
		IsManual: true, ValuationDate: date, PriceValue: cents, PriceScale: 2})
	require.NoError(t, err)
}

func (f *returnsFixture) dividend(t *testing.T, date string, cashAccountID int64, instrument *int64, cents int64) {
	t.Helper()
	_, err := f.investmentService.Dividend(context.Background(), DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: date,
		CommodityID: NullablePatch[int64]{Set: true, Value: instrument}, CashAccountID: cashAccountID,
		CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.incomeAccountID, AmountValue: cents, AmountScale: 2})
	require.NoError(t, err)
}

func (f *returnsFixture) read(t *testing.T, from, to string) ReturnsResult {
	t.Helper()
	result, err := f.investmentService.ReadReturns(context.Background(), ReturnsInput{From: from, To: to, ReportingCommodityID: f.eurCommodityID})
	require.NoError(t, err)
	return result
}

// requireReturnsAmount compares a reporting-currency amount with whole euros.
func requireReturnsAmount(t *testing.T, euros int64, amount ReturnsAmount) {
	t.Helper()
	require.Empty(t, amount.Reason)
	require.NotNil(t, amount.Amount)
	require.Equal(t, 0, exact.ScaledIntFromCoefficient(amount.Amount.QuantityValue, amount.Amount.QuantityScale).
		Cmp(exact.ScaledIntFromInt64(euros*100, 2)), "expected %d, got %se-%d", euros, amount.Amount.QuantityValue, amount.Amount.QuantityScale)
}

func TestReturnsWithoutFlows(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)

	result := f.read(t, "2026-02-01", "2027-01-31")
	require.Equal(t, ReturnsPolicy, result.Policy)
	require.Equal(t, "2026-01-31", result.OpeningDate)
	require.Equal(t, 365, result.Days)
	require.Equal(t, ReturnsScopeDefaultInvestment, result.AccountScope)
	require.ElementsMatch(t, []int64{f.brokerCashID, f.holdingAccountID}, result.ResolvedAccountIDs)
	requireReturnsAmount(t, 1100, result.OpeningValue)
	requireReturnsAmount(t, 1200, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 100, result.Gain)
	requireReturn(t, "0.090909", result.TimeWeighted)
	requireReturn(t, "0.090909", result.TimeWeightedAnnualized)
	requireReturn(t, "0.090909", result.MoneyWeighted)
	require.True(t, result.Valuation.Complete)
}

// S2: deposit, dividend and fee inside the portfolio, withdrawal to checking.
func TestReturnsClassifyFlowsAndPerformance(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2026-05-01", 10500)
	f.post(t, "2026-05-02", f.brokerCashID, f.cashAccountID, 50000)
	f.dividend(t, "2026-07-31", f.brokerCashID, &f.stockCommodityID, 2000)
	f.post(t, "2026-10-31", f.feeID, f.brokerCashID, 500)
	f.price(t, "2026-10-31", 12000)
	f.post(t, "2026-11-01", f.cashAccountID, f.brokerCashID, 30000)
	f.price(t, "2027-01-31", 12500)

	result := f.read(t, "2026-02-01", "2027-01-31")
	requireReturnsAmount(t, 1100, result.OpeningValue)
	requireReturnsAmount(t, 1565, result.ClosingValue)
	requireReturnsAmount(t, 500, result.Contributions)
	requireReturnsAmount(t, 300, result.Withdrawals)
	requireReturnsAmount(t, 265, result.Gain)
	requireReturn(t, "0.187954", result.TimeWeighted)
	requireReturn(t, "0.189483", result.MoneyWeighted)
	requireReturn(t, "0.189483", result.MoneyWeightedPeriod)
	require.Empty(t, result.AttributionFallbacks)
}

// S3: a dividend for the portfolio's holding paid into checking is income
// and a same-day withdrawal; nothing is written to make it so.
func TestReturnsAttributeDividendPaidOutsideToTheHolding(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.dividend(t, "2026-07-31", f.cashAccountID, &f.stockCommodityID, 2000)
	f.price(t, "2026-07-31", 10500)
	f.price(t, "2027-01-31", 11000)
	before := f.transactionCount(t)

	result := f.read(t, "2026-02-01", "2027-01-31")
	require.Equal(t, before, f.transactionCount(t), "returns never write")
	requireReturnsAmount(t, 20, result.Withdrawals)
	requireReturn(t, "0.109881", result.TimeWeighted)
	requireReturn(t, "0.110074", result.MoneyWeighted)
	require.Empty(t, result.AttributionFallbacks)

	// The same dividend is outside a portfolio that excludes the holding: then
	// it is neither return nor a flow of broker cash.
	only, err := f.investmentService.ReadReturns(context.Background(), ReturnsInput{From: "2026-02-01", To: "2027-01-31",
		ReportingCommodityID: f.eurCommodityID, AccountIDs: []int64{f.brokerCashID}})
	require.NoError(t, err)
	require.Equal(t, ReturnsScopeSelectedAccounts, only.AccountScope)
	requireReturnsAmount(t, 100, only.OpeningValue)
	requireReturnsAmount(t, 0, only.Withdrawals)
	requireReturn(t, "0.000000", only.TimeWeighted)
}

// A dividend for an instrument held only outside the portfolio but paid into
// broker cash is a contribution, not return.
func TestReturnsTreatOutsideHoldingIncomeAsContribution(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.dividend(t, "2026-07-31", f.brokerCashID, &f.stockCommodityID, 2000)
	f.price(t, "2027-01-31", 11000)

	result, err := f.investmentService.ReadReturns(context.Background(), ReturnsInput{From: "2026-02-01", To: "2027-01-31",
		ReportingCommodityID: f.eurCommodityID, AccountIDs: []int64{f.brokerCashID}})
	require.NoError(t, err)
	requireReturnsAmount(t, 100, result.OpeningValue)
	requireReturnsAmount(t, 120, result.ClosingValue)
	requireReturnsAmount(t, 20, result.Contributions)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// S4: an unassigned dividend cannot be tied to a position, so the cash side
// decides and the read says so.
func TestReturnsDiscloseUnassignedDividendFallback(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.dividend(t, "2026-07-31", f.cashAccountID, nil, 2000)
	f.dividend(t, "2026-08-31", f.brokerCashID, nil, 1000)
	f.price(t, "2027-01-31", 11000)

	result := f.read(t, "2026-02-01", "2027-01-31")
	require.Len(t, result.AttributionFallbacks, 2)
	outside, inside := result.AttributionFallbacks[0], result.AttributionFallbacks[1]
	require.Equal(t, ReturnsFallbackUnassignedInstrument, outside.Reason)
	require.False(t, outside.Counted)
	require.Equal(t, "2026-07-31", outside.Date)
	require.Equal(t, "dividend", outside.OperationKind)
	require.Equal(t, ReturnsFallbackUnassignedInstrument, inside.Reason)
	require.True(t, inside.Counted)
	requireReturnsAmount(t, 0, result.Withdrawals)
	requireReturnsAmount(t, 1210, result.ClosingValue)
	// 1,210 / 1,100: the counted 10 is return, the uncounted 20 is not.
	requireReturn(t, "0.100000", result.TimeWeighted)
}

// S5/S6: a missing closing price removes only what needs it; a short period
// has no annual figure.
func TestReturnsFailureStatesAndShortPeriods(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)

	missing := f.read(t, "2026-02-01", "2027-01-31")
	requireReturnsAmount(t, 1100, missing.OpeningValue)
	require.Equal(t, ReturnReasonValuationUnavailable, missing.ClosingValue.Reason)
	require.Equal(t, "2027-01-31", missing.ClosingValue.Date)
	require.Equal(t, ReturnReasonValuationUnavailable, missing.TimeWeighted.Reason)
	require.Equal(t, ReturnReasonValuationUnavailable, missing.MoneyWeighted.Reason)
	require.Equal(t, ReturnReasonValuationUnavailable, missing.Gain.Reason)
	require.False(t, missing.Valuation.Complete)
	require.Len(t, missing.Valuation.Gaps, 1)
	require.Equal(t, f.stockCommodityID, missing.Valuation.Gaps[0].CommodityID)

	f.price(t, "2026-06-30", 10500)
	short := f.read(t, "2026-02-01", "2026-06-30")
	require.Equal(t, 150, short.Days)
	requireReturn(t, "0.045455", short.TimeWeighted)
	require.Equal(t, ReturnReasonPeriodShorterThanYear, short.TimeWeightedAnnualized.Reason)
	require.Equal(t, ReturnReasonPeriodShorterThanYear, short.MoneyWeighted.Reason)
	requireReturn(t, "0.045455", short.MoneyWeightedPeriod)
}

func TestReturnsRejectInvalidInput(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	ctx := context.Background()
	for _, input := range []ReturnsInput{
		{From: "2026-03-01", To: "2026-02-01", ReportingCommodityID: f.eurCommodityID},
		{From: "2026-02-01", To: "2027-02-02", ReportingCommodityID: f.eurCommodityID},
		{From: "2026-02-01", To: "2026-03-01"},
		{From: "2026-02-01", To: "2026-03-01", ReportingCommodityID: f.eurCommodityID, MaxStalenessDays: 32},
		{From: "2026-02-01", To: "2026-03-01", ReportingCommodityID: f.eurCommodityID, AccountIDs: []int64{f.incomeAccountID}},
	} {
		_, err := f.investmentService.ReadReturns(ctx, input)
		var validation ValidationError
		require.ErrorAs(t, err, &validation, "%+v", input)
	}
}
