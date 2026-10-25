package app

import (
	"context"
	"testing"

	"rekenraam/backend/internal/exact"

	"github.com/stretchr/testify/require"
)

// #409: a trade is recognised on its trade date. Its units post that day and
// its cash on the settlement day, so reading each entry on its own date left
// the bought units and the cash not yet paid both inside the portfolio
// between the two. Every case keeps TEST at 100, so no figure may move.
// Fixture: V(open) = 100 broker cash + 10 TEST at 100 = 1,100.

func (f *returnsFixture) settledBuy(t *testing.T, tradeDate, settlementDate string, cashAccountID int64) Transaction {
	t.Helper()
	trade, err := f.investmentService.Buy(context.Background(), InvestmentTradeInput{
		OwnerUserID: f.ownerUserID, TransactionDate: tradeDate, SettlementDate: settlementDate, CommodityID: f.stockCommodityID,
		HoldingAccountID: f.holdingAccountID, CashAccountID: cashAccountID,
		QuantityValue: exact.New(1), CashAmountValue: 10000, CashAmountScale: 2, CashCommodityID: f.eurCommodityID,
	})
	require.NoError(t, err)
	return trade.Transaction
}

// requireAllocationIsClosingValue checks that the allocation's denominator is
// the closing value, so a pending settlement is not counted by one and
// dropped by the other.
func requireAllocationIsClosingValue(t *testing.T, result ReturnsResult) {
	t.Helper()
	require.Equal(t, AllocationBasisClosingValue, result.Allocation.Basis)
	sum := exact.NewScaledInt()
	for _, row := range result.Allocation.ByAccount {
		sum.AddCoefficient(row.Value.QuantityValue, row.Value.QuantityScale)
	}
	closing := result.ClosingValue.Amount
	require.Zero(t, sum.Cmp(exact.ScaledIntFromCoefficient(closing.QuantityValue, closing.QuantityScale)))
}

// R1, the #409 reproducer: traded on the opening boundary from broker cash,
// settled inside the period. Before the fix: opening 1,200, TTWROR −0.083333.
func TestReturnsRecognisesATradeSettledInsideThePeriodOnItsTradeDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1100, result.OpeningValue)
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 0, result.Withdrawals)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// R2: the same trade paid from checking, outside the portfolio. Its
// contribution is recognised on the trade date, before the period, so the
// period has no flow. Before the fix: a contribution of 100 on 02-02.
func TestReturnsRecognisesAnOutsideFundedTradeOnItsTradeDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-01-31", "2026-02-02", f.cashAccountID)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1200, result.OpeningValue)
	requireReturnsAmount(t, 1200, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// R3: traded inside the period from broker cash, settled after it ends. The
// cash leg dated after `to` still belongs to the period. Before the fix:
// closing 1,200, TTWROR 0.090909.
func TestReturnsCountsASettlementAfterThePeriodInTheClosingValue(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-02-27", "2026-03-02", f.brokerCashID)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
	requireAllocationIsClosingValue(t, result)
}

// R4: traded inside the period from checking, settled after it ends: a
// contribution of 100 on the trade date. Before the fix: no contribution,
// TTWROR 0.090909.
func TestReturnsCountsAnOutsideFundedTradeSettlingAfterThePeriod(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-02-27", "2026-03-02", f.cashAccountID)
	// The inflow needs V(02-26); 01-31's price is beyond the 7-day window.
	f.price(t, "2026-02-26", 10000)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1200, result.ClosingValue)
	requireReturnsAmount(t, 100, result.Contributions)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
	requireReturn(t, "0.000000", result.MoneyWeightedPeriod)
}

// R5: a reversal of R1's trade cancels it on the trade date as well.
func TestReturnsReversedSettledTradeLeavesNoFigure(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	trade := f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)
	_, err := acknowledgedReverseBuy(context.Background(), f.investmentService, ReverseInvestmentBuyInput{
		OwnerUserID: f.ownerUserID, TransactionID: trade.ID, Reason: "wrong account"})
	require.NoError(t, err)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1100, result.OpeningValue)
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 0, result.Withdrawals)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// Dividends receivable across the portfolio boundary (review 2026-10-25). The
// fixture's 10 TEST sit in the portfolio; a dividend of 20 for them goes ex on
// 02-10 and is paid on 02-20 into checking, outside the portfolio. The
// receivable is the portfolio's until the cash moves, so it is held by the
// entitled holding and the withdrawal happens on the payment date.
func (f *returnsFixture) dividendIntoAccount(t *testing.T, cashAccountID int64) {
	t.Helper()
	_, err := f.investmentService.Dividend(context.Background(), DividendInput{
		OwnerUserID: f.ownerUserID, TransactionDate: "2026-02-20", ExDate: NullableValue("2026-02-10"),
		CommodityID: NullableValue(f.stockCommodityID), CashAccountID: cashAccountID,
		CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.incomeAccountID, AmountValue: 2000, AmountScale: 2})
	require.NoError(t, err)
}

// Before payment: 100 cash + 10 × 98 + the 20 receivable, nothing withdrawn,
// and the ex-date price drop is offset by the dividend.
func TestReturnsKeepAnOutsidePaidDividendReceivableUntilPayment(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2026-02-10", 9800)
	f.dividendIntoAccount(t, f.cashAccountID)

	result := f.read(t, "2026-02-01", "2026-02-15")
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Withdrawals)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// The payment crosses the boundary: a withdrawal of 20 on 02-20.
func TestReturnsWithdrawAnOutsidePaidDividendOnPayment(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2026-02-10", 9800)
	f.price(t, "2026-02-20", 9800)
	f.dividendIntoAccount(t, f.cashAccountID)

	result := f.read(t, "2026-02-16", "2026-02-25")
	requireReturnsAmount(t, 1100, result.OpeningValue)
	requireReturnsAmount(t, 20, result.Withdrawals)
	requireReturnsAmount(t, 1080, result.ClosingValue)
}

// Paid into an account that opens only on the payment date: net worth carries
// the receivable on that account, and the portfolio still holds it.
func TestReturnsHoldADividendReceivableForAnAccountNotYetOpen(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2026-02-10", 9800)
	f.accountService.now = f.transactionService.now
	account, err := f.accountService.CreateAccount(context.Background(), CreateAccountInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api", Operation: "account.create",
		Name: "New dividend account", AccountClass: "asset", AccountKind: "brokerage_cash",
		DefaultCommodityID: &f.eurCommodityID, OpenedOn: "2026-02-20", EffectiveFrom: "2026-02-20"})
	require.NoError(t, err)
	f.dividendIntoAccount(t, account.ID)

	worth := f.netWorthAt(t, "2026-02-15")
	requireUnsettled(t, f, worth.Unsettled, map[int64]int64{account.ID: 20})
	result := f.read(t, "2026-02-01", "2026-02-15")
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Withdrawals)
}

// A replacement moves the payer from TEST (held in scope) to OTHER (held
// outside), paid into broker cash in scope: the replacement's cash enters
// from outside (a contribution of 20), and the inverse cancels the original
// inside. The receivable OTHER's holding carries is not position evidence for
// the inverse (review 2026-10-25, P1), so nothing is withdrawn and no gain
// appears.
func TestReturnsCorrectedPayerAcrossScopeCreatesNoGain(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 200000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	other := seedTestSecurityCommodity(t, f.investmentsTestFixture, "OTHER")
	outside := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	_, err := f.investmentService.Buy(context.Background(), InvestmentTradeInput{
		OwnerUserID: f.ownerUserID, TransactionDate: "2026-01-20", CommodityID: other,
		HoldingAccountID: outside, CashAccountID: f.brokerCashID, CashCommodityID: f.eurCommodityID,
		QuantityValue: exact.New(10), CashAmountValue: 100000, CashAmountScale: 2})
	require.NoError(t, err)
	for _, date := range []string{"2026-01-31", "2026-02-10", "2026-02-19", "2026-02-20", "2026-02-28"} {
		f.price(t, date, 10000)
		f.priceOf(t, other, f.eurCommodityID, date, 10000)
	}
	dividend := f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 0, false)
	_, err = f.investmentService.ReplaceDividend(context.Background(), ReplaceInvestmentDividendInput{
		OwnerUserID: f.ownerUserID, TransactionID: dividend.ID, Reason: "correct payer",
		Replacement: DividendInput{TransactionDate: "2026-02-20", CommodityID: NullableValue(other),
			CashAccountID: f.brokerCashID, CashCommodityID: f.eurCommodityID,
			IncomeAccountID: &f.incomeAccountID, AmountValue: 2000, AmountScale: 2}})
	require.NoError(t, err)

	portfolio, _ := f.readInstruments(t, "2026-02-01", "2026-02-28", f.holdingAccountID, f.brokerCashID)
	requireReturnsAmount(t, 0, portfolio.Gain)
	requireReturnsAmount(t, 20, portfolio.Contributions)
	requireReturnsAmount(t, 0, portfolio.Withdrawals)
	requireReturn(t, "0.000000", portfolio.TimeWeighted)
}

// A USD dividend for the portfolio's TEST, paid into checking outside, with
// no USD rate, then reversed: the chain nets to zero USD on each day before
// any conversion, so the portfolio needs no USD rate (review 2026-10-25, P2).
func TestReturnsReversedForeignDividendPaidOutsideNeedsNoRate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	for _, date := range []string{"2026-02-10", "2026-02-19", "2026-02-20", "2026-02-28"} {
		f.price(t, date, 10000)
	}
	usd := f.commodity(t, "USD", "currency", 2)
	dividend, err := f.investmentService.Dividend(context.Background(), DividendInput{
		OwnerUserID: f.ownerUserID, TransactionDate: "2026-02-20", ExDate: NullableValue("2026-02-10"),
		CommodityID: NullableValue(f.stockCommodityID), CashAccountID: f.cashAccountID,
		CashCommodityID: usd, IncomeAccountID: &f.incomeAccountID, AmountValue: 2000, AmountScale: 2})
	require.NoError(t, err)
	_, err = f.investmentService.ReverseDividend(context.Background(), ReverseInvestmentDividendInput{
		OwnerUserID: f.ownerUserID, TransactionID: dividend.ID, Reason: "wrong entry"})
	require.NoError(t, err)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
}
