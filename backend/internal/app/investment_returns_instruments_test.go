package app

import (
	"context"
	"testing"
	"time"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"

	"github.com/stretchr/testify/require"
)

// R13 slice 8: instrument_returns_v1 cases I1–I12 from the returns plan,
// hand-derived there. Every read is made twice, with and without the flag,
// and the portfolio part must be identical (I10).

// newBareReturnsFixture is newReturnsFixture without its opening position:
// checking holds 10,000 and the broker nothing, so each case's sleeve starts
// empty and its literals are the plan's.
func newBareReturnsFixture(t *testing.T) *returnsFixture {
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
	f.post(t, "2026-01-02", f.cashAccountID, f.equityID, 1000000)
	return f
}

// priceOf records a manual price of one unit of base in quote.
func (f *returnsFixture) priceOf(t *testing.T, base, quote int64, date string, cents int64) {
	t.Helper()
	quoteType := "close"
	var kind string
	require.NoError(t, f.database.QueryRow(`SELECT kind FROM commodities WHERE id = ?`, base).Scan(&kind))
	if kind == "currency" {
		quoteType = "manual"
	}
	_, err := f.pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID,
		BaseCommodityID: base, QuoteCommodityID: quote, QuoteType: quoteType, AdjustmentBasis: "raw",
		IsManual: true, ValuationDate: date, PriceValue: cents, PriceScale: 2})
	require.NoError(t, err)
}

// trade buys or sells TEST in a holding with broker cash, settling on settle.
func (f *returnsFixture) trade(t *testing.T, buy bool, holding int64, date, settle string, quantity, cents int64) Transaction {
	t.Helper()
	input := InvestmentTradeInput{OwnerUserID: f.ownerUserID, TransactionDate: date, SettlementDate: settle,
		CommodityID: f.stockCommodityID, HoldingAccountID: holding, CashAccountID: f.brokerCashID,
		QuantityValue: exact.New(quantity), CashAmountValue: cents, CashAmountScale: 2, CashCommodityID: f.eurCommodityID}
	var result InvestmentTradeResult
	var err error
	if buy {
		result, err = f.investmentService.Buy(context.Background(), input)
	} else {
		result, err = f.investmentService.Sell(context.Background(), input)
	}
	require.NoError(t, err)
	return result.Transaction
}

// buyWithFee buys 10 TEST for 1,000 plus a 5 commission, all paid from broker
// cash on the settlement date unless the fee is paid separately.
func (f *returnsFixture) buyWithFee(t *testing.T, date, settle string, feeAccount *int64, feeOn string) {
	t.Helper()
	gross, net := int64(-100000), int64(-100500)
	charge := InvestmentTradeChargeInput{Kind: "commission", AmountValue: -500, AmountScale: 2, CommodityID: f.eurCommodityID,
		Treatment: "clearing_included", CashAccountID: feeAccount, PaidOn: feeOn}
	if feeAccount != nil {
		net = -100000
	}
	_, err := f.investmentService.Buy(context.Background(), InvestmentTradeInput{OwnerUserID: f.ownerUserID,
		TransactionDate: date, SettlementDate: settle, CommodityID: f.stockCommodityID,
		HoldingAccountID: f.holdingAccountID, CashAccountID: f.brokerCashID, CashCommodityID: f.eurCommodityID,
		QuantityValue: exact.New(10), GrossAmountValue: &gross, GrossAmountScale: 2, NetSettlementValue: &net, NetSettlementScale: 2,
		Charges: []InvestmentTradeChargeInput{charge}})
	require.NoError(t, err)
}

// dividendOf records a TEST dividend into broker cash, gross with optional
// withholding, and its ex date when one is given (recorded as an operation
// date, as a payment in lieu's is).
func (f *returnsFixture) dividendOf(t *testing.T, date, exDate string, instrument *int64, gross, withheld int64, withholdingOnly bool) {
	t.Helper()
	input := DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: date, CommodityID: NullablePatch[int64]{Set: true, Value: instrument},
		CashAccountID: f.brokerCashID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.incomeAccountID,
		AmountValue: gross, AmountScale: 2, WithholdingOnly: withholdingOnly}
	if withholdingOnly {
		input.IncomeAccountID = nil
	}
	if withheld > 0 || withholdingOnly {
		input.WithholdingAccountID = &f.feeID
	}
	if withheld > 0 {
		input.WithholdingValue = &withheld
	}
	transaction, err := f.investmentService.Dividend(context.Background(), input)
	require.NoError(t, err)
	if exDate != "" {
		_, err = f.database.Exec(`INSERT INTO investment_operation_dates (operation_id, date_role, event_date)
			SELECT l.operation_id, 'ex', ? FROM investment_operation_journal_links l
			JOIN transaction_versions tv ON tv.id = l.transaction_version_id WHERE tv.transaction_id = ?`, exDate, transaction.ID)
		require.NoError(t, err)
	}
}

// readInstruments reads with instrument returns and checks I10: the portfolio
// part equals the read without them.
func (f *returnsFixture) readInstruments(t *testing.T, from, to string, accounts ...int64) (ReturnsResult, InstrumentReturns) {
	t.Helper()
	writes := func() [3]int {
		var counts [3]int
		for index, table := range []string{"audit_events", "posting_versions", "investment_operations"} {
			require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM `+table).Scan(&counts[index]))
		}
		return counts
	}
	before := writes()
	defer func() { require.Equal(t, before, writes(), "a returns read writes nothing") }()
	input := ReturnsInput{From: from, To: to, ReportingCommodityID: f.eurCommodityID, AccountIDs: accounts}
	plain, err := f.investmentService.ReadReturns(context.Background(), input)
	require.NoError(t, err)
	input.Instruments = true
	full, err := f.investmentService.ReadReturns(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, full.InstrumentReturns)
	instruments := *full.InstrumentReturns
	full.InstrumentReturns = nil
	require.Equal(t, plain, full, "I10: requesting instrument returns changes no portfolio figure")
	require.Equal(t, InstrumentReturnsPolicy, instruments.Policy)
	require.Equal(t, InstrumentIncomeRecognitionExDate, instruments.IncomeRecognition)
	return full, instruments
}

func sleeveRow(t *testing.T, instruments InstrumentReturns, commodityID int64) InstrumentReturnsRow {
	t.Helper()
	for _, row := range instruments.Rows {
		if row.CommodityID == commodityID {
			return row
		}
	}
	require.Failf(t, "no sleeve", "commodity %d", commodityID)
	return InstrumentReturnsRow{}
}

func requireUnavailable(t *testing.T, reason string, figure ReturnFigure) {
	t.Helper()
	require.Nil(t, figure.Value)
	require.Equal(t, reason, figure.Reason)
}

// I1 (and I8): a buy settled two days later with a fee, inside a portfolio
// funded by a 1,005 deposit.
func TestInstrumentReturnsBuySettledLaterWithFee(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100500)
	f.buyWithFee(t, "2026-02-01", "2026-02-03", nil, "")
	f.price(t, "2026-02-28", 11000)

	portfolio, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 0, row.OpeningValue)
	requireReturnsAmount(t, 1005, row.Contributions)
	requireReturnsAmount(t, 0, row.Withdrawals)
	requireReturnsAmount(t, 1100, row.ClosingValue)
	requireReturnsAmount(t, 95, row.Gain)
	requireReturnsAmount(t, 0, row.Income)
	requireReturn(t, "0.094527", row.TimeWeighted)
	requireReturn(t, "0.094527", row.MoneyWeightedPeriod)
	requireUnavailable(t, ReturnReasonPeriodShorterThanYear, row.MoneyWeighted)
	require.False(t, row.Short)
	// I8: the portfolio's gain is the sleeve's; nothing is left over.
	requireReturnsAmount(t, 95, portfolio.Gain)
	requireReturnsAmount(t, 0, instruments.NotAttributedGain)
	require.Len(t, instruments.Rows, 1, "cash is never a sleeve")
}

// I1b: traded before the period, settled inside it: no sleeve flow.
func TestInstrumentReturnsTradeBeforeThePeriodSettlingInside(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-31", "2026-02-02", 10, 100000)
	f.price(t, "2026-02-28", 11000)

	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 1000, row.OpeningValue)
	requireReturnsAmount(t, 0, row.Contributions)
	requireReturnsAmount(t, 100, row.Gain)
	requireReturn(t, "0.100000", row.TimeWeighted)
}

// I1c: traded on `to`, settling after it: the inflow counts on the trade date.
func TestInstrumentReturnsTradeOnTheClosingDaySettlingAfter(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100500)
	f.buyWithFee(t, "2026-02-28", "2026-03-02", nil, "")

	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 1005, row.Contributions)
	requireReturnsAmount(t, 1000, row.ClosingValue)
	requireReturnsAmount(t, -5, row.Gain)
	requireReturn(t, "-0.004975", row.TimeWeighted)
}

// I1d: the fee paid separately from checking on 02-10 is still part of the
// purchase, recognised on the trade date.
func TestInstrumentReturnsSeparatelyPaidChargeJoinsItsTrade(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.buyWithFee(t, "2026-02-01", "2026-02-03", &f.cashAccountID, "2026-02-10")
	f.price(t, "2026-02-28", 11000)

	portfolio, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 1005, row.Contributions)
	requireReturnsAmount(t, 95, row.Gain)
	requireReturn(t, "0.094527", row.TimeWeighted)
	requireReturnsAmount(t, 95, portfolio.Gain)
	requireReturnsAmount(t, 0, instruments.NotAttributedGain)
}

// I2: income recognised on its ex date; the owner sells before it is paid.
func TestInstrumentReturnsExDateIncomeAfterExit(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-01-31", 10000)
	f.price(t, "2026-02-10", 9800)
	f.trade(t, false, f.holdingAccountID, "2026-02-15", "", 10, 105000)
	f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 300, false)

	portfolio, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 1000, row.OpeningValue)
	requireReturnsAmount(t, 1067, row.Withdrawals)
	requireReturnsAmount(t, 17, row.Income)
	requireReturnsAmount(t, 67, row.Gain)
	requireReturn(t, "0.068214", row.TimeWeighted)
	requireReturn(t, "0.129425", row.MoneyWeightedPeriod)
	require.Empty(t, instruments.NotAttributed)
	requireReturnsAmount(t, 67, portfolio.Gain)
	requireReturnsAmount(t, 0, instruments.NotAttributedGain)
}

// I2b (a): selling on the ex date keeps the entitlement.
func TestInstrumentReturnsSaleOnTheExDateKeepsTheDividend(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-01-31", 10000)
	// Selling to zero makes 02-09 an evaluation day: it needs its own price.
	f.price(t, "2026-02-09", 10000)
	f.trade(t, false, f.holdingAccountID, "2026-02-10", "", 10, 105000)
	f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 300, false)

	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 17, row.Income)
	requireReturnsAmount(t, 1067, row.Withdrawals)
	requireReturn(t, "0.067000", row.TimeWeighted)
}

// I2b (b): buying on the ex date gives no entitlement.
func TestInstrumentReturnsPurchaseOnTheExDateIsNotEntitled(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-02-10", "", 10, 100000)
	f.price(t, "2026-02-28", 10000)
	f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 0, false)

	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 0, row.Income)
	requireReturnsAmount(t, 1000, row.Contributions)
	require.Len(t, instruments.NotAttributed, 1)
	require.Equal(t, ReturnsFallbackNotHeld, instruments.NotAttributed[0].Reason)
	require.Equal(t, "2026-02-10", instruments.NotAttributed[0].Date)
}

// I3 and I3b: held inside and outside the selected accounts at the
// entitlement point. The refusal follows the flow's direction.
func TestInstrumentReturnsAmbiguousIncomeRefusesItsDirection(t *testing.T) {
	t.Parallel()
	for _, withholdingOnly := range []bool{false, true} {
		f := newBareReturnsFixture(t)
		f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 200000)
		outside := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
		f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
		f.trade(t, true, outside, "2026-01-20", "", 5, 50000)
		f.price(t, "2026-01-31", 10000)
		f.price(t, "2026-02-10", 9800)
		f.trade(t, false, f.holdingAccountID, "2026-02-15", "", 10, 105000)
		if withholdingOnly {
			f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 300, 0, true)
		} else {
			f.dividendOf(t, "2026-02-20", "2026-02-10", &f.stockCommodityID, 2000, 300, false)
		}

		_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28", f.holdingAccountID, f.brokerCashID)
		row := sleeveRow(t, instruments, f.stockCommodityID)
		requireReturnsAmount(t, 1000, row.OpeningValue)
		requireReturnsAmount(t, 0, row.ClosingValue)
		if withholdingOnly {
			// I3b: a tax payment is a contribution.
			require.Equal(t, ReturnReasonIncomeUnattributable, row.Contributions.Reason)
			requireReturnsAmount(t, 1050, row.Withdrawals)
		} else {
			requireReturnsAmount(t, 0, row.Contributions)
			require.Equal(t, ReturnReasonIncomeUnattributable, row.Withdrawals.Reason)
		}
		require.Equal(t, ReturnReasonIncomeUnattributable, row.Income.Reason)
		require.Equal(t, ReturnReasonIncomeUnattributable, row.Gain.Reason)
		requireUnavailable(t, ReturnReasonIncomeUnattributable, row.TimeWeighted)
		requireUnavailable(t, ReturnReasonIncomeUnattributable, row.MoneyWeightedPeriod)
		require.Len(t, instruments.NotAttributed, 1)
		require.Equal(t, ReturnsFallbackHeldInsideAndOutside, instruments.NotAttributed[0].Reason)
		require.Equal(t, ReturnReasonInstrumentGainUnavailable, instruments.NotAttributedGain.Reason)
	}
}

// I4: a cash-and-stock merger, valued at the consideration received.
func TestInstrumentReturnsCashAndStockMergerAtConsideration(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	newID := seedTestSecurityCommodity(t, f.investmentsTestFixture, "NEWCO")
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-02-28", 10000)
	f.priceOf(t, newID, f.eurCommodityID, "2026-03-01", 21000)
	f.priceOf(t, newID, f.eurCommodityID, "2026-03-31", 22000)
	_, err := f.investmentService.CashStockMerger(context.Background(),
		cashStockMergerInput(f.investmentsTestFixture, newID, "2026-03-01", "2026-03-05", 1, 2, 700, "1", 2))
	require.NoError(t, err)

	portfolio, instruments := f.readInstruments(t, "2026-03-01", "2026-03-31")
	source := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 1057, source.Withdrawals)
	requireReturnsAmount(t, 57, source.Gain)
	requireReturn(t, "0.057000", source.TimeWeighted)
	destination := sleeveRow(t, instruments, newID)
	requireReturnsAmount(t, 1050, destination.Contributions)
	requireReturnsAmount(t, 1100, destination.ClosingValue)
	requireReturnsAmount(t, 50, destination.Gain)
	requireReturn(t, "0.047619", destination.TimeWeighted)
	// The 7 went to checking: a portfolio withdrawal, so 1,100 − 1,000 + 7.
	requireReturnsAmount(t, 107, portfolio.Gain)
	requireReturnsAmount(t, 0, instruments.NotAttributedGain)
}

// I4b: a share exchange whose fraction is later sold for cash in lieu. The
// cash is the destination's sale, never the source's consideration.
func TestInstrumentReturnsCashInLieuIsTheDestinationsSale(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	newID := seedTestSecurityCommodity(t, f.investmentsTestFixture, "NEWCO")
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-02-28", 10000)
	f.priceOf(t, newID, f.eurCommodityID, "2026-03-01", 10000)
	f.priceOf(t, newID, f.eurCommodityID, "2026-03-31", 10000)
	exchange, err := f.investmentService.ShareExchange(context.Background(),
		shareExchangeInput(f.investmentsTestFixture, newID, "2026-03-01", 21, 20))
	require.NoError(t, err)
	inLieu := cashInLieuInput(f.investmentsTestFixture, exchange.Transaction.ID)
	inLieu.DisposalOn, inLieu.PaymentOn, inLieu.ProceedsValue = "2026-03-05", "2026-03-05", 5000
	_, err = f.investmentService.CashInLieu(context.Background(), inLieu)
	require.NoError(t, err)

	_, instruments := f.readInstruments(t, "2026-03-01", "2026-03-31")
	source := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 1050, source.Withdrawals)
	requireReturnsAmount(t, 50, source.Gain)
	requireReturn(t, "0.050000", source.TimeWeighted)
	destination := sleeveRow(t, instruments, newID)
	requireReturnsAmount(t, 1050, destination.Contributions)
	requireReturnsAmount(t, 50, destination.Withdrawals)
	requireReturnsAmount(t, 1000, destination.ClosingValue)
	requireReturnsAmount(t, 0, destination.Gain)
	requireReturn(t, "0.000000", destination.TimeWeighted)
}

// I5: a move between two in-scope holdings is no flow; a move out of the
// scope is an outflow at market.
func TestInstrumentReturnsTransfersAcrossTheScope(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	outside := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	bought, err := f.investmentService.Buy(context.Background(), InvestmentTradeInput{OwnerUserID: f.ownerUserID,
		TransactionDate: "2026-01-20", CommodityID: f.stockCommodityID, HoldingAccountID: f.holdingAccountID,
		CashAccountID: f.brokerCashID, QuantityValue: exact.New(10), CashAmountValue: 100000, CashAmountScale: 2,
		CashCommodityID: f.eurCommodityID})
	require.NoError(t, err)
	f.price(t, "2026-01-31", 10000)
	f.price(t, "2026-02-10", 10500)
	f.price(t, "2026-02-28", 10500)
	transfer := func(to int64, date string, quantity int64) {
		input := internalTransferFromLot(f.investmentsTestFixture, to, *bought.LotID, exact.New(quantity), 0)
		input.EffectiveOn = date
		_, err := f.investmentService.InternalTransfer(context.Background(), input)
		require.NoError(t, err)
	}
	transfer(second, "2026-02-05", 6)
	transfer(outside, "2026-02-10", 4)

	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28", f.holdingAccountID, second, f.brokerCashID)
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 0, row.Contributions)
	requireReturnsAmount(t, 420, row.Withdrawals)
	requireReturnsAmount(t, 630, row.ClosingValue)
	requireReturnsAmount(t, 50, row.Gain)
}

// I6, I6b and I6c: a short keeps its gain but gets no return; a same-day
// short is caught by its named operations; a same-day long round trip reads.
func TestInstrumentReturnsShortsGetNoReturn(t *testing.T) {
	t.Parallel()
	short := func(f *returnsFixture, sale bool, date string, cents int64) {
		input := InvestmentTradeInput{OwnerUserID: f.ownerUserID, TransactionDate: date, CommodityID: f.stockCommodityID,
			HoldingAccountID: f.holdingAccountID, CashAccountID: f.brokerCashID, QuantityValue: exact.New(10),
			CashAmountValue: cents, CashAmountScale: 2, CashCommodityID: f.eurCommodityID}
		var err error
		if sale {
			_, err = f.investmentService.ShortSale(context.Background(), input)
		} else {
			_, err = f.investmentService.ShortCover(context.Background(), input)
		}
		require.NoError(t, err)
	}

	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	short(f, true, "2026-02-02", 100000)
	f.price(t, "2026-02-28", 9000)
	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	require.True(t, row.Short)
	requireReturnsAmount(t, -900, row.ClosingValue)
	requireReturnsAmount(t, 1000, row.Withdrawals)
	requireReturnsAmount(t, 100, row.Gain)
	requireUnavailable(t, ReturnReasonShortPosition, row.TimeWeighted)
	requireUnavailable(t, ReturnReasonShortPosition, row.MoneyWeightedPeriod)

	f = newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	short(f, true, "2026-02-10", 100000)
	short(f, false, "2026-02-10", 90000)
	_, instruments = f.readInstruments(t, "2026-02-01", "2026-02-28")
	row = sleeveRow(t, instruments, f.stockCommodityID)
	require.True(t, row.Short, "zero at both day ends, but named short operations")
	requireReturnsAmount(t, 100, row.Gain)
	requireUnavailable(t, ReturnReasonShortPosition, row.TimeWeighted)

	f = newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-02-10", "", 10, 90000)
	f.trade(t, false, f.holdingAccountID, "2026-02-10", "", 10, 100000)
	_, instruments = f.readInstruments(t, "2026-02-01", "2026-02-28")
	row = sleeveRow(t, instruments, f.stockCommodityID)
	require.False(t, row.Short)
	requireReturnsAmount(t, 100, row.Gain)
	requireReturn(t, "0.111111", row.TimeWeighted)
}

// I7: an unassigned dividend is in no sleeve, listed, and left over.
func TestInstrumentReturnsUnassignedDividendIsNotAttributed(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-01-31", 10000)
	f.price(t, "2026-02-28", 10000)
	f.dividendOf(t, "2026-02-20", "", nil, 2000, 0, false)

	portfolio, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 0, row.Income)
	requireReturnsAmount(t, 0, row.Gain)
	require.Len(t, instruments.NotAttributed, 1)
	require.Equal(t, ReturnsFallbackUnassignedInstrument, instruments.NotAttributed[0].Reason)
	require.Zero(t, instruments.NotAttributed[0].InstrumentID)
	requireReturnsAmount(t, 20, portfolio.Gain)
	requireReturnsAmount(t, 20, instruments.NotAttributedGain)
}

// I11: a USD trade settled two days later is valued at the trade date's rate
// (D7); with no USD left in cash, nothing is left over.
func TestInstrumentReturnsForeignCurrencySettlementAtTradeDateRate(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	result, err := f.database.Exec(`INSERT INTO commodities (book_id, code, kind, is_builtin, created_at, created_by_user_id)
		VALUES (?, 'USD', 'currency', 1, '2026-01-01T00:00:00Z', 1)`, BookID)
	require.NoError(t, err)
	usd, err := result.LastInsertId()
	require.NoError(t, err)
	_, err = f.database.Exec(`INSERT INTO commodity_versions (
			commodity_id, version_seq, effective_from, recorded_at, changed_by_user_id,
			change_reason, status, symbol, display_symbol, name, standard_scale, max_quantity_scale
		) VALUES (?, 1, '2026-01-01', '2026-01-01T00:00:00Z', 1, 'test fixture', 'active', 'USD', '$', 'US dollar', 2, 2)`, usd)
	require.NoError(t, err)
	usdCash := seedTestAccountWithClass(t, f.database, "active", true, "asset", "brokerage_cash")
	_, err = f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api",
		Spec: TransactionInput{TransactionDate: "2026-01-02", JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: usdCash, CommodityID: usd, QuantityValue: exact.New(100000), QuantityScale: 2},
			{AccountID: f.equityID, CommodityID: usd, QuantityValue: exact.New(-100000), QuantityScale: 2},
		}}}},
	})
	require.NoError(t, err)
	for date, cents := range map[string]int64{"2026-01-31": 90, "2026-02-01": 90, "2026-02-03": 95, "2026-02-28": 92} {
		f.priceOf(t, usd, f.eurCommodityID, date, cents)
	}
	_, err = f.investmentService.Buy(context.Background(), InvestmentTradeInput{OwnerUserID: f.ownerUserID,
		TransactionDate: "2026-02-01", SettlementDate: "2026-02-03", CommodityID: f.stockCommodityID,
		HoldingAccountID: f.holdingAccountID, CashAccountID: usdCash, CashCommodityID: usd,
		QuantityValue: exact.New(10), CashAmountValue: 100000, CashAmountScale: 2})
	require.NoError(t, err)
	f.priceOf(t, f.stockCommodityID, usd, "2026-02-28", 11000)

	portfolio, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28")
	row := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 900, row.Contributions)
	requireReturnsAmount(t, 1012, row.ClosingValue)
	requireReturnsAmount(t, 112, row.Gain)
	requireReturn(t, "0.124444", row.TimeWeighted)
	requireReturnsAmount(t, 112, portfolio.Gain)
	requireReturnsAmount(t, 0, instruments.NotAttributedGain)
}

// I12 and I12b: a spin-off at consideration, and an unpriced child.
func TestInstrumentReturnsSpinOffAtConsideration(t *testing.T) {
	t.Parallel()
	for _, priced := range []bool{true, false} {
		f := newBareReturnsFixture(t)
		child := seedTestSecurityCommodity(t, f.investmentsTestFixture, "CHILD")
		f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
		f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
		f.price(t, "2026-02-28", 10000)
		f.price(t, "2026-03-01", 9000)
		f.price(t, "2026-03-31", 9500)
		if priced {
			f.priceOf(t, child, f.eurCommodityID, "2026-03-01", 2000)
		}
		f.priceOf(t, child, f.eurCommodityID, "2026-03-31", 2200)
		_, err := f.investmentService.SpinOff(context.Background(),
			spinOffInput(f.investmentsTestFixture, child, "2026-03-01", 1, 2, "1", 1))
		require.NoError(t, err)

		portfolio, instruments := f.readInstruments(t, "2026-03-01", "2026-03-31")
		parent, spun := sleeveRow(t, instruments, f.stockCommodityID), sleeveRow(t, instruments, child)
		requireReturnsAmount(t, 950, parent.ClosingValue)
		requireReturnsAmount(t, 110, spun.ClosingValue)
		requireReturn(t, "0.060000", portfolio.TimeWeighted)
		requireReturnsAmount(t, 60, portfolio.Gain)
		if !priced {
			// I12b: only the two sleeves' flow figures are unavailable.
			require.Equal(t, ReturnReasonFlowUnavailable, parent.Withdrawals.Reason)
			require.Equal(t, ReturnReasonFlowUnavailable, parent.Gain.Reason)
			requireUnavailable(t, ReturnReasonFlowUnavailable, parent.TimeWeighted)
			require.Equal(t, ReturnReasonFlowUnavailable, spun.Contributions.Reason)
			requireUnavailable(t, ReturnReasonFlowUnavailable, spun.TimeWeighted)
			require.Equal(t, ReturnReasonInstrumentGainUnavailable, instruments.NotAttributedGain.Reason)
			require.True(t, portfolio.Valuation.Complete, "the portfolio needed no child price on 03-01")
			continue
		}
		requireReturnsAmount(t, 100, parent.Withdrawals)
		requireReturnsAmount(t, 50, parent.Gain)
		requireReturn(t, "0.055556", parent.TimeWeighted)
		requireReturnsAmount(t, 100, spun.Contributions)
		requireReturnsAmount(t, 10, spun.Gain)
		requireReturn(t, "0.100000", spun.TimeWeighted)
		requireReturnsAmount(t, 0, instruments.NotAttributedGain)
	}
}

// I9: a plain journal entry moving two instruments is refused for both.
func TestInstrumentReturnsTwoInstrumentJournalIsRefused(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	other := seedTestSecurityCommodity(t, f.investmentsTestFixture, "OTHER")
	vault := seedTestAccountWithClass(t, f.database, "active", true, "asset", "brokerage_cash")
	outside := seedTestAccountWithClass(t, f.database, "active", true, "asset", "cash")
	move := func(date string, legs ...PostingInput) {
		_, err := f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
			OwnerUserID: f.ownerUserID, OriginType: "browser_api",
			Spec: TransactionInput{TransactionDate: date, JournalEntries: []JournalEntryInput{{Postings: legs}}},
		})
		require.NoError(t, err)
	}
	leg := func(accountID, commodityID, units int64) PostingInput {
		return PostingInput{AccountID: accountID, CommodityID: commodityID, QuantityValue: exact.New(units), QuantityScale: 0}
	}
	move("2026-01-10", leg(vault, f.stockCommodityID, 10), leg(outside, f.stockCommodityID, -10))
	// One entry: 10 TEST out of the scope, 5 OTHER in, with nothing saying
	// what was given for what.
	move("2026-02-10", leg(vault, f.stockCommodityID, -10), leg(outside, f.stockCommodityID, 10),
		leg(vault, other, 5), leg(outside, other, -5))
	_, instruments := f.readInstruments(t, "2026-02-01", "2026-02-28", vault)
	for _, commodityID := range []int64{f.stockCommodityID, other} {
		row := sleeveRow(t, instruments, commodityID)
		requireUnavailable(t, ReturnReasonUnclassifiedExchange, row.TimeWeighted)
	}
}

// D6: a reversal posts every leg mirrored, so on the same day and prices it
// cancels the exchange in both sleeves.
func TestInstrumentReturnsReversedExchangeCancels(t *testing.T) {
	t.Parallel()
	f := newBareReturnsFixture(t)
	newID := seedTestSecurityCommodity(t, f.investmentsTestFixture, "NEWCO")
	f.post(t, "2026-01-02", f.brokerCashID, f.cashAccountID, 100000)
	f.trade(t, true, f.holdingAccountID, "2026-01-20", "", 10, 100000)
	f.price(t, "2026-02-28", 10000)
	f.price(t, "2026-03-31", 10000)
	f.priceOf(t, newID, f.eurCommodityID, "2026-03-01", 10500)
	exchange, err := f.investmentService.ShareExchange(context.Background(),
		shareExchangeInput(f.investmentsTestFixture, newID, "2026-03-01", 21, 20))
	require.NoError(t, err)
	_, err = f.investmentService.ReverseShareExchange(context.Background(), ReverseShareExchangeInput{
		OwnerUserID: f.ownerUserID, TransactionID: exchange.Transaction.ID, Reason: "wrong terms"})
	require.NoError(t, err)

	_, instruments := f.readInstruments(t, "2026-03-01", "2026-03-31")
	source := sleeveRow(t, instruments, f.stockCommodityID)
	requireReturnsAmount(t, 0, source.Contributions)
	requireReturnsAmount(t, 0, source.Withdrawals)
	requireReturn(t, "0.000000", source.TimeWeighted)
	destination := sleeveRow(t, instruments, newID)
	requireReturnsAmount(t, 0, destination.Contributions)
	requireReturnsAmount(t, 0, destination.Withdrawals)
	requireReturnsAmount(t, 0, destination.Gain)
}
