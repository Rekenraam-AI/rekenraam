package app

import (
	"context"
	"sync"
	"testing"
	"time"

	sqlite "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
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

// N7: an obligation recognised before its settling account opens still
// counts on the trade date. Before the fix it was dropped: 9,000 EUR.
func TestNetWorthCountsUnsettledCashBeforeItsAccountOpens(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.accountService.now = f.transactionService.now
	account, err := f.accountService.CreateAccount(context.Background(), CreateAccountInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api", Operation: "account.create", Name: "New settlement account", AccountClass: "asset", AccountKind: "brokerage_cash",
		DefaultCommodityID: &f.eurCommodityID, OpenedOn: "2026-02-02", EffectiveFrom: "2026-02-02",
	})
	require.NoError(t, err)
	f.settledBuy(t, "2026-01-31", "2026-02-02", account.ID)
	before := f.netWorthAt(t, "2026-01-31")
	after := f.netWorthAt(t, "2026-02-02")
	requireUnsettled(t, f, before.Unsettled, map[int64]int64{account.ID: -100})
	requireAmount(t, before.Totals, f.eurCommodityID, 890000, 2, "cash plus the unpaid 100 obligation")
	requireAmount(t, before.Totals, f.stockCommodityID, 11, 0, "units acquired on trade day")
	requireAmount(t, after.Totals, f.eurCommodityID, 890000, 2, "settled cash")
	requireUnsettled(t, f, after.Unsettled, map[int64]int64{})
}

// N8: posted balances and unsettled trades come from one snapshot. A buy
// committed between the two reads used to pair the old units with the new
// unsettled cash, a false loss of 100.
func TestNetWorthReadsPostedAndUnsettledFromOneSnapshot(t *testing.T) {
	t.Parallel()
	requireNetWorthOneSnapshot(t, false)
}

// N9: the same for the series.
func TestNetWorthSeriesReadsPostedAndUnsettledFromOneSnapshot(t *testing.T) {
	t.Parallel()
	requireNetWorthOneSnapshot(t, true)
}

// requireNetWorthOneSnapshot pauses the read at the operation links, the
// unsettled trades' query, commits a settling buy from another connection,
// and checks that the read still pairs units with cash from one state.
func requireNetWorthOneSnapshot(t *testing.T, series bool) {
	t.Helper()
	f := newReturnsFixture(t)
	ctx := context.Background()
	var sequence int
	var name, path string
	require.NoError(t, f.database.QueryRow(`PRAGMA database_list`).Scan(&sequence, &name, &path))
	other, err := db.Open(ctx, db.SQLiteFileURL(path))
	require.NoError(t, err)
	defer other.Close()
	accounts := db.NewAccountRepository(other)
	accountService := NewAccountService(accounts, db.NewInstitutionRepository(other), nil)
	transactions := NewTransactionService(db.NewTransactionRepository(other), db.NewPayeeRepository(other), accounts, db.NewCommodityRepository(other))
	writer := NewInvestmentService(db.NewInvestmentRepository(other), accountService, transactions, nil)
	writer.now = f.investmentService.now
	transactions.now = f.transactionService.now
	reached, release := make(chan struct{}), make(chan struct{})
	var once, unblock sync.Once
	defer unblock.Do(func() { close(release) })
	conn, err := f.database.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, conn.Raw(func(raw any) error {
		raw.(*sqlite.SQLiteConn).RegisterAuthorizer(func(op int, table, column, catalog string) int {
			if op == sqlite.SQLITE_READ && table == "investment_operation_journal_links" {
				once.Do(func() { close(reached); <-release })
			}
			return sqlite.SQLITE_OK
		})
		return nil
	}))
	require.NoError(t, conn.Close())
	type readResult struct {
		worth NetWorthResult
		err   error
	}
	result := make(chan readResult, 1)
	go func() {
		if series {
			read, err := f.transactionService.NetWorthSeries(ctx, NetWorthSeriesInput{StartDate: "2026-01-31", EndDate: "2026-01-31", Bucket: "day"})
			if err != nil {
				result <- readResult{err: err}
				return
			}
			result <- readResult{worth: NetWorthResult{Totals: read.Buckets[0].Totals, Unsettled: read.Buckets[0].Unsettled}}
			return
		}
		worth, err := f.transactionService.NetWorth(ctx, NetWorthInput{OwnerUserID: f.ownerUserID, AsOf: "2026-01-31"})
		result <- readResult{worth, err}
	}()
	select {
	case <-reached:
	case <-time.After(10 * time.Second):
		t.Fatal("did not reach pending read")
	}
	_, err = writer.Buy(ctx, InvestmentTradeInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-01-31", SettlementDate: "2026-02-02",
		CommodityID: f.stockCommodityID, HoldingAccountID: f.holdingAccountID, CashAccountID: f.brokerCashID,
		CashCommodityID: f.eurCommodityID, QuantityValue: exact.New(1), CashAmountValue: 10000, CashAmountScale: 2})
	require.NoError(t, err)
	unblock.Do(func() { close(release) })
	var got readResult
	select {
	case got = <-result:
	case <-time.After(10 * time.Second):
		t.Fatal("read did not finish")
	}
	require.NoError(t, got.err)
	// The snapshot began before the buy, so the read sees neither side of it.
	var euros, units exact.Coefficient
	for _, amount := range got.worth.Totals {
		if amount.CommodityID == f.eurCommodityID {
			euros = amount.QuantityValue
		}
		if amount.CommodityID == f.stockCommodityID {
			units = amount.QuantityValue
		}
	}
	require.Equal(t, "10", units.String(), "first posted read preceded the buy")
	require.Equal(t, "900000", euros.String(), "old units must pair with old cash; mixed state creates a false 100 loss")
}

// Dividends receivable (reports plan, extending #410): a dividend that
// records an ex date is recognised on it. Its cash counts as unsettled on the
// account it is paid into from the ex date until payment, or with the
// opposite sign when it is paid first. The fixture's 10 TEST pay into broker
// cash; amounts are whole euros.

// N10: gross 20, 3 withheld, ex 02-10, paid 02-20: a receivable of the net 17
// from the ex date. Without an ex date the dividend keeps its payment date.
func TestNetWorthRecognisesADividendReceivableOnItsExDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 300, false)

	cases := []struct {
		date      string
		euros     int64
		unsettled map[int64]int64
	}{
		{"2026-02-09", 9000, map[int64]int64{}},
		{"2026-02-10", 9017, map[int64]int64{f.brokerCashID: 17}},
		{"2026-02-19", 9017, map[int64]int64{f.brokerCashID: 17}},
		{"2026-02-20", 9017, map[int64]int64{}},
	}
	for _, tc := range cases {
		worth := f.netWorthAt(t, tc.date)
		requireAmount(t, worth.Totals, f.eurCommodityID, tc.euros*100, 2, tc.date)
		requireUnsettled(t, f, worth.Unsettled, tc.unsettled)
	}

	g := newReturnsFixture(t)
	g.dividendOf(t, "2026-02-20", "", &g.stockCommodityID, 2000, 300, false)
	worth := g.netWorthAt(t, "2026-02-19")
	requireAmount(t, worth.Totals, g.eurCommodityID, 900000, 2, "no ex date: nothing before payment")
	requireUnsettled(t, g, worth.Unsettled, map[int64]int64{})
}

// N11: a due-bill dividend paid 02-10 that goes ex 02-12 is early cash, offset
// until the ex date, so net worth shows no gain before recognition.
func TestNetWorthOffsetsADividendPaidBeforeItsExDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.dividendOf(t, "2026-02-10", "2026-02-12", &f.stockCommodityID, 2000, 0, false)

	worth := f.netWorthAt(t, "2026-02-10")
	requireAmount(t, worth.Totals, f.eurCommodityID, 900000, 2, "paid, not yet recognised")
	requireUnsettled(t, f, worth.Unsettled, map[int64]int64{f.brokerCashID: -20})
	worth = f.netWorthAt(t, "2026-02-12")
	requireAmount(t, worth.Totals, f.eurCommodityID, 902000, 2, "recognised on the ex date")
	requireUnsettled(t, f, worth.Unsettled, map[int64]int64{})

	balances, err := f.transactionService.AccountBalances(context.Background(), AccountBalancesInput{OwnerUserID: f.ownerUserID, AsOf: "2026-02-10"})
	require.NoError(t, err)
	for _, account := range balances.Accounts {
		if account.AccountID == f.brokerCashID {
			requireAmount(t, account.DirectBalances, f.eurCommodityID, 12000, 2, "the posted balance has the cash")
		}
	}
}

// N12: a reversed ex-dated dividend leaves nothing on any day, and a
// replacement that adds the ex date recognises the dividend from it: the
// inverse journal keeps the original's (payment) date and cancels it there.
func TestNetWorthDividendCorrectionsRecogniseOneDividend(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newReturnsFixture(t)
	reversed := f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 0, false)
	_, err := f.investmentService.ReverseDividend(ctx, ReverseInvestmentDividendInput{OwnerUserID: f.ownerUserID,
		TransactionID: reversed.ID, Reason: "duplicate"})
	require.NoError(t, err)
	for _, date := range []string{"2026-02-09", "2026-02-10", "2026-02-20"} {
		worth := f.netWorthAt(t, date)
		requireAmount(t, worth.Totals, f.eurCommodityID, 900000, 2, "reversed "+date)
		requireUnsettled(t, f, worth.Unsettled, map[int64]int64{})
	}

	g := newReturnsFixture(t)
	original := g.dividendOf(t, "2026-02-20", "", &g.stockCommodityID, 2000, 0, false)
	_, err = g.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: g.ownerUserID,
		TransactionID: original.ID, Reason: "record the ex date", Replacement: DividendInput{TransactionDate: "2026-02-20",
			ExDate: NullableValue("2026-02-10"), CashAccountID: g.brokerCashID, CashCommodityID: g.eurCommodityID,
			IncomeAccountID: &g.incomeAccountID, AmountValue: 2000, AmountScale: 2}})
	require.NoError(t, err)
	worth := g.netWorthAt(t, "2026-02-09")
	requireAmount(t, worth.Totals, g.eurCommodityID, 900000, 2, "before the ex date")
	worth = g.netWorthAt(t, "2026-02-10")
	requireAmount(t, worth.Totals, g.eurCommodityID, 902000, 2, "the replacement from its ex date")
	requireUnsettled(t, g, worth.Unsettled, map[int64]int64{g.brokerCashID: 20})
	worth = g.netWorthAt(t, "2026-02-20")
	requireAmount(t, worth.Totals, g.eurCommodityID, 902000, 2, "paid once")
	requireUnsettled(t, g, worth.Unsettled, map[int64]int64{})
}

// N13: the portfolio values the receivable on the same day as net worth.
// Closing 02-15: broker 100 + receivable 17 + 10 TEST at 98.
func TestReturnsPortfolioHoldsTheDividendReceivable(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2026-02-10", 9800)
	f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 300, false)

	result := f.read(t, "2026-02-01", "2026-02-15")
	requireReturnsAmount(t, 1097, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 0, result.Withdrawals)
	worth := f.netWorthAt(t, "2026-02-15")
	requireUnsettled(t, f, worth.Unsettled, map[int64]int64{f.brokerCashID: 17})
}
