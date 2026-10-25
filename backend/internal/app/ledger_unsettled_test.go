package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// #410: net worth recognises a trade on its trade date. The units post that
// day and the cash on the settlement day, so between the two the posted
// balances counted both the bought units and the cash not yet paid. Net
// worth now carries that cash as an unsettled amount on the account that
// settles it; account balances stay posting-based. Fixture (returns): 9,000
// EUR in checking and broker cash and 10 TEST before any case trades.

func (f *returnsFixture) netWorthAt(t *testing.T, asOf string) NetWorthResult {
	t.Helper()
	result, err := f.transactionService.NetWorth(context.Background(), NetWorthInput{OwnerUserID: f.ownerUserID, AsOf: asOf})
	require.NoError(t, err)
	return result
}

// requireUnsettled checks the unsettled amounts as exact whole euros per
// account; an empty map means none.
func requireUnsettled(t *testing.T, f *returnsFixture, unsettled []UnsettledAmount, want map[int64]int64) {
	t.Helper()
	require.Len(t, unsettled, len(want))
	for _, amount := range unsettled {
		require.Equal(t, f.eurCommodityID, amount.Amount.CommodityID)
		euros, ok := want[amount.AccountID]
		require.Truef(t, ok, "unexpected unsettled amount on account %d", amount.AccountID)
		value := exact.ScaledIntFromCoefficient(amount.Amount.QuantityValue, amount.Amount.QuantityScale)
		require.Zerof(t, value.Cmp(exact.ScaledIntFromInt64(euros*100, 2)), "account %d: want %d, got %s", amount.AccountID, euros, value.String())
	}
}

// N1, the #410 reproducer: before the fix 01-31 and 02-01 read 9,000 EUR +
// 11 TEST.
func TestNetWorthRecognisesABuyOnItsTradeDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)

	cases := []struct {
		date      string
		euros     int64
		units     int64
		unsettled map[int64]int64
	}{
		{"2026-01-30", 9000, 10, map[int64]int64{}},
		{"2026-01-31", 8900, 11, map[int64]int64{f.brokerCashID: -100}},
		{"2026-02-01", 8900, 11, map[int64]int64{f.brokerCashID: -100}},
		{"2026-02-02", 8900, 11, map[int64]int64{}},
	}
	for _, tc := range cases {
		worth := f.netWorthAt(t, tc.date)
		requireAmount(t, worth.Totals, f.eurCommodityID, tc.euros*100, 2, tc.date+" EUR")
		requireAmount(t, worth.Totals, f.stockCommodityID, tc.units, 0, tc.date+" TEST")
		requireUnsettled(t, f, worth.Unsettled, tc.unsettled)
	}

	// The account balance itself stays on the posting basis.
	balances, err := f.transactionService.AccountBalances(context.Background(), AccountBalancesInput{OwnerUserID: f.ownerUserID, AsOf: "2026-01-31"})
	require.NoError(t, err)
	for _, account := range balances.Accounts {
		if account.AccountID == f.brokerCashID {
			requireAmount(t, account.DirectBalances, f.eurCommodityID, 10000, 2, "broker cash posted balance")
		}
	}
}

// N2: a sale settling later is a receivable from the trade date.
func TestNetWorthRecognisesASaleReceivableOnItsTradeDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	_, err := f.investmentService.Sell(context.Background(), InvestmentTradeInput{
		OwnerUserID: f.ownerUserID, TransactionDate: "2026-02-03", SettlementDate: "2026-02-05", CommodityID: f.stockCommodityID,
		HoldingAccountID: f.holdingAccountID, CashAccountID: f.brokerCashID,
		QuantityValue: exact.New(2), CashAmountValue: 25000, CashAmountScale: 2, CashCommodityID: f.eurCommodityID,
	})
	require.NoError(t, err)

	worth := f.netWorthAt(t, "2026-02-04")
	requireAmount(t, worth.Totals, f.eurCommodityID, 925000, 2, "trade-date EUR")
	requireAmount(t, worth.Totals, f.stockCommodityID, 8, 0, "units after the sale")
	requireUnsettled(t, f, worth.Unsettled, map[int64]int64{f.brokerCashID: 250})

	worth = f.netWorthAt(t, "2026-02-05")
	requireAmount(t, worth.Totals, f.eurCommodityID, 925000, 2, "settled EUR")
	requireUnsettled(t, f, worth.Unsettled, map[int64]int64{})
}

// N3: the series reads each bucket end exactly as the point-in-time read,
// unsettled amounts included.
func TestNetWorthSeriesCarriesUnsettledAmountsAtEveryBucketEnd(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)

	series, err := f.transactionService.NetWorthSeries(context.Background(), NetWorthSeriesInput{
		StartDate: "2026-01-30", EndDate: "2026-02-02", Bucket: "day"})
	require.NoError(t, err)
	require.Len(t, series.Buckets, 4)
	for _, bucket := range series.Buckets {
		worth := f.netWorthAt(t, bucket.EndDate)
		require.ElementsMatch(t, worth.Totals, bucket.Totals, bucket.EndDate)
		require.Equal(t, worth.Unsettled, bucket.Unsettled, bucket.EndDate)
	}
	requireAmount(t, series.Buckets[1].Totals, f.eurCommodityID, 890000, 2, "trade date")
}

// N4: an account filter carries the unsettled amount with the account that
// settles it, and a cashflow over that account still equals the change in
// its posted balance: net worth's change less the unsettled change.
func TestNetWorthFilterKeepsUnsettledCashWithItsSettlementAccount(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)
	ctx := context.Background()

	cash, err := f.transactionService.NetWorthSeries(ctx, NetWorthSeriesInput{StartDate: "2026-01-01", EndDate: "2026-02-28",
		Bucket: "month", Filters: ReportFilters{AccountIDs: []int64{f.brokerCashID}}})
	require.NoError(t, err)
	require.Len(t, cash.Buckets, 2)
	requireAmount(t, cash.Buckets[0].Totals, f.eurCommodityID, 0, 2, "broker cash on the trade date")
	requireUnsettled(t, f, cash.Buckets[0].Unsettled, map[int64]int64{f.brokerCashID: -100})
	requireAmount(t, cash.Buckets[1].Totals, f.eurCommodityID, 0, 2, "broker cash once settled")
	requireUnsettled(t, f, cash.Buckets[1].Unsettled, map[int64]int64{})

	cashflow, err := f.transactionService.Cashflow(ctx, CashflowInput{StartDate: "2026-02-01", EndDate: "2026-02-28",
		Bucket: "month", Filters: ReportFilters{AccountIDs: []int64{f.brokerCashID}}})
	require.NoError(t, err)
	requireAmount(t, cashflow.Buckets[0].NetMovement, f.eurCommodityID, -10000, 2, "settlement-dated cashflow")

	holding, err := f.transactionService.NetWorthSeries(ctx, NetWorthSeriesInput{StartDate: "2026-01-31", EndDate: "2026-01-31",
		Bucket: "day", Filters: ReportFilters{AccountIDs: []int64{f.holdingAccountID}}})
	require.NoError(t, err)
	requireAmount(t, holding.Buckets[0].Totals, f.stockCommodityID, 11, 0, "holding units")
	requireAmount(t, holding.Buckets[0].Totals, f.eurCommodityID, 0, 2, "no cash on the holding")
	requireUnsettled(t, f, holding.Buckets[0].Unsettled, map[int64]int64{})
}

// N5: a reversal of N1's trade leaves nothing unsettled on any day.
func TestNetWorthReversedSettledTradeLeavesNothingUnsettled(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	trade := f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)
	_, err := acknowledgedReverseBuy(context.Background(), f.investmentService, ReverseInvestmentBuyInput{
		OwnerUserID: f.ownerUserID, TransactionID: trade.ID, Reason: "wrong account"})
	require.NoError(t, err)

	for _, date := range []string{"2026-01-31", "2026-02-01", "2026-02-02"} {
		worth := f.netWorthAt(t, date)
		requireAmount(t, worth.Totals, f.eurCommodityID, 900000, 2, date+" EUR")
		requireAmount(t, worth.Totals, f.stockCommodityID, 10, 0, date+" TEST")
		requireUnsettled(t, f, worth.Unsettled, map[int64]int64{})
	}
}

// N6: a trade settled from checking moves its unsettled amount there, and a
// same-day trade has none.
func TestNetWorthUnsettledFollowsTheSettlingAccountOnly(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-02-10", "2026-02-12", f.cashAccountID)
	f.settledBuy(t, "2026-02-11", "2026-02-11", f.brokerCashID)

	worth := f.netWorthAt(t, "2026-02-11")
	requireAmount(t, worth.Totals, f.eurCommodityID, 880000, 2, "both trades recognised")
	requireAmount(t, worth.Totals, f.stockCommodityID, 12, 0, "both trades' units")
	requireUnsettled(t, f, worth.Unsettled, map[int64]int64{f.cashAccountID: -100})
}
