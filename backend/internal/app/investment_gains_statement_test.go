package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #231 (R18 slice 2): the period statement reports opening unrealized at the
// day before the period, realized and income in it, and closing unrealized at
// its end, per currency, with absent figures never shown as zero.

func gainsStatement(t *testing.T, f *investmentsTestFixture, from, to string) GainsStatement {
	t.Helper()
	statement, err := f.investmentService.GainsStatement(context.Background(), from, to)
	require.NoError(t, err)
	return statement
}

func onlyStatementRow(t *testing.T, f *investmentsTestFixture, statement GainsStatement) GainsStatementRow {
	t.Helper()
	require.Len(t, statement.Rows, 1)
	require.Equal(t, f.eurCommodityID, statement.Rows[0].CommodityID)
	return statement.Rows[0]
}

func assertStatementAmount(t *testing.T, value int64, scale int, got *StatementAmount, msg string) {
	t.Helper()
	require.NotNil(t, got, msg)
	assertMoneyValue(t, value, scale, got.Value, got.Scale, msg)
}

func dividendOn(t *testing.T, f *investmentsTestFixture, date string, gross int64) Transaction {
	t.Helper()
	input := cashDividendInput(f, 0, gross, 0)
	input.TransactionDate = date
	dividend, err := f.investmentService.Dividend(context.Background(), input)
	require.NoError(t, err)
	return dividend
}

func TestGainsStatementBuyAndSellWithinThePeriod(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-03-05", 2, 4000)
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-03-20", 2))
	require.NoError(t, err)

	statement := gainsStatement(t, f, "2026-03-01", "2026-03-31")
	assert.Equal(t, "operational_v1", statement.Profile.Name)
	assert.Equal(t, "2026-03-31", statement.Profile.ValuationAsOf)
	row := onlyStatementRow(t, f, statement)
	assert.Equal(t, "2026-02-28", row.Opening.ValuationAsOf, "the day before the period")
	assert.Equal(t, "2026-03-31", row.Closing.ValuationAsOf)
	assert.Zero(t, row.Opening.PositionCount)
	assertStatementAmount(t, 0, 0, row.Opening.Gain, "nothing held is a known zero")
	assertStatementAmount(t, 0, 0, row.Closing.Gain, "nothing held at the end")
	assert.Equal(t, 1, row.DisposalCount)
	assertStatementAmount(t, 6000, 2, row.Realized, "100.00 − 40.00")
	assertStatementAmount(t, 0, 0, &row.Income, "no income")
	assertStatementAmount(t, 0, 0, row.UnrealizedChange, "no unrealized movement")
	assertStatementAmount(t, 6000, 2, row.Total, "the whole result is realized")
}

func TestGainsStatementPositionOpenedBeforeThePeriod(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-02-01", 2, 4000)
	priceStockOn(t, f, "2026-02-27", 2500)
	priceStockOn(t, f, "2026-03-30", 3000)
	priceStockOn(t, f, "2026-04-02", 9900) // after the period: never used
	dividendOn(t, f, "2026-03-15", 300)
	reversed := dividendOn(t, f, "2026-03-16", 999)
	_, err := f.investmentService.ReverseDividend(ctx, ReverseInvestmentDividendInput{
		OwnerUserID: f.ownerUserID, TransactionID: reversed.ID, Reason: "paid in error"})
	require.NoError(t, err)
	dividendOn(t, f, "2026-04-01", 500) // after the period

	row := onlyStatementRow(t, f, gainsStatement(t, f, "2026-03-01", "2026-03-31"))
	assert.Equal(t, 1, row.Opening.PositionCount)
	assertStatementAmount(t, 1000, 2, row.Opening.Gain, "2 × 25.00 − 40.00")
	assert.Equal(t, "2026-02-27", row.Opening.LatestPriceDate)
	assert.Equal(t, "2026-02-27", row.Opening.EarliestPriceDate)
	assertStatementAmount(t, 2000, 2, row.Closing.Gain, "2 × 30.00 − 40.00")
	assert.Equal(t, "2026-03-30", row.Closing.LatestPriceDate)
	assert.Zero(t, row.DisposalCount)
	assertStatementAmount(t, 0, 0, row.Realized, "nothing disposed")
	assert.Equal(t, 1, row.IncomeCount, "a reversed dividend is not income")
	assertStatementAmount(t, 300, 2, &row.Income, "3.00 paid in the period")
	assertStatementAmount(t, 1000, 2, row.UnrealizedChange, "20.00 − 10.00")
	assertStatementAmount(t, 1300, 2, row.Total, "change plus income")
}

func TestGainsStatementPositionClosedBeforePeriodEnd(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-02-01", 2, 4000)
	priceStockOn(t, f, "2026-02-27", 2500)
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-03-10", 2))
	require.NoError(t, err)

	row := onlyStatementRow(t, f, gainsStatement(t, f, "2026-03-01", "2026-03-31"))
	assertStatementAmount(t, 1000, 2, row.Opening.Gain, "held at the opening date")
	assert.Zero(t, row.Closing.PositionCount)
	assertStatementAmount(t, 0, 0, row.Closing.Gain, "closed before the period end")
	assertStatementAmount(t, 6000, 2, row.Realized, "100.00 − 40.00")
	assertStatementAmount(t, -1000, 2, row.UnrealizedChange, "opening gain realized")
	assertStatementAmount(t, 5000, 2, row.Total, "100.00 proceeds − 50.00 opening value")
}

func TestGainsStatementUnpricedAtABoundaryIsAbsentNotZero(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixtureWithOptions(t, false) // no trade-implied fill price
	buyOn(t, f, "2026-02-01", 2, 4000)
	priceStockOn(t, f, "2026-03-10", 2600)

	row := onlyStatementRow(t, f, gainsStatement(t, f, "2026-03-01", "2026-03-31"))
	assert.Nil(t, row.Opening.Gain, "an unpriced position leaves the opening absent")
	assert.Equal(t, StatementUnpriced, row.Opening.UnavailableReason)
	assert.Equal(t, 1, row.Opening.UnpricedCount)
	assert.Equal(t, 1, row.Opening.PositionCount)
	assert.Empty(t, row.Opening.LatestPriceDate)
	assertStatementAmount(t, 1200, 2, row.Closing.Gain, "2 × 26.00 − 40.00")
	assert.Equal(t, 1, row.Closing.StaleCount, "21 days old: retained and marked")
	assert.Nil(t, row.UnrealizedChange, "a change from an absent opening is absent")
	assert.Nil(t, row.Total)
	assertStatementAmount(t, 0, 0, row.Realized, "realized stays known")
}

func TestGainsStatementUnresolvedDisposalLeavesRealizedAbsent(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	_, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-02-01", ""))
	require.NoError(t, err)
	priceStockOn(t, f, "2026-02-27", 2500)
	_, err = f.investmentService.Sell(ctx, sellInput(f, "2026-03-10", 1))
	require.NoError(t, err)

	row := onlyStatementRow(t, f, gainsStatement(t, f, "2026-03-01", "2026-03-31"))
	assert.Nil(t, row.Opening.Gain)
	assert.Equal(t, StatementUnresolvedBasis, row.Opening.UnavailableReason)
	assert.Equal(t, 1, row.Opening.UnresolvedCount)
	assert.Nil(t, row.Realized, "a sum of only the known disposals is not the total")
	assert.Equal(t, 1, row.RealizedUnresolvedCount)
	assert.Nil(t, row.Total)
}

func TestGainsStatementRejectsInvalidPeriods(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	for _, period := range [][2]string{{"", "2026-03-31"}, {"2026-03-01", ""}, {"2026-04-01", "2026-03-31"},
		{"2026-03-01", "2099-01-01"}, {"2026-02-30", "2026-03-31"}} {
		_, err := f.investmentService.GainsStatement(context.Background(), period[0], period[1])
		var validation ValidationError
		require.ErrorAs(t, err, &validation, period)
	}
}
