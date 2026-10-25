package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

// R13 benchmark cases (docs/plans/investment-returns-plan.md, slice 5). The
// expected figures are hand-derived ratios of the literal observations below,
// never values read back from the calculator. The portfolio is the returns
// fixture's S1 base (TEST 100 at the opening boundary, 110 at the close).

func (f *returnsFixture) commodity(t *testing.T, code, kind string, scale int) int64 {
	t.Helper()
	res, err := f.database.Exec(`INSERT INTO commodities(book_id,code,kind,is_builtin,created_at,created_by_user_id) VALUES(1,?,?,0,'2026-01-01T00:00:00Z',1)`, code, kind)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = f.database.Exec(`INSERT INTO commodity_versions(commodity_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,symbol,display_symbol,name,standard_scale,max_quantity_scale) VALUES(?,1,'2026-01-01','2026-01-01T00:00:00Z',1,'fixture','active',?,?,?,?,6)`,
		id, code, code, code, scale)
	require.NoError(t, err)
	return id
}

// observe records one manual observation of base in quote (cents) on a basis.
func (f *returnsFixture) observe(t *testing.T, base, quote int64, basis, date string, cents int64) {
	t.Helper()
	_, err := f.pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID,
		BaseCommodityID: base, QuoteCommodityID: quote, QuoteType: "manual", AdjustmentBasis: basis,
		IsManual: true, ValuationDate: date, PriceValue: cents, PriceScale: 2})
	require.NoError(t, err)
}

func (f *returnsFixture) readBenchmark(t *testing.T, from, to string, benchmarkID int64, basis string) ReturnsResult {
	t.Helper()
	result, err := f.investmentService.ReadReturns(context.Background(), ReturnsInput{From: from, To: to,
		ReportingCommodityID: f.eurCommodityID, BenchmarkCommodityID: benchmarkID, BenchmarkBasis: basis})
	require.NoError(t, err)
	require.NotNil(t, result.Benchmark)
	benchmark := result.Benchmark
	// The benchmark is read at the portfolio series' own dates, and its
	// headline is its closing point.
	require.Len(t, benchmark.Points, len(result.TimeWeightedSeries.Points))
	for i, point := range benchmark.Points {
		require.Equal(t, result.TimeWeightedSeries.Points[i].Date, point.Date)
	}
	require.Equal(t, benchmark.TimeWeighted, benchmark.Points[len(benchmark.Points)-1].TimeWeighted)
	return result
}

func benchmarkPoint(t *testing.T, benchmark *ReturnsBenchmark, date string) ReturnFigure {
	t.Helper()
	for _, point := range benchmark.Points {
		if point.Date == date {
			return point.TimeWeighted
		}
	}
	t.Fatalf("no benchmark point on %s", date)
	return ReturnFigure{}
}

func requireUnavailableReturn(t *testing.T, reason, date string, figure ReturnFigure) {
	t.Helper()
	require.Nil(t, figure.Value)
	require.Equal(t, reason, figure.Reason)
	require.Equal(t, date, figure.Date)
}

// B1/B2: price return reads raw observations only and total return reads
// total_return observations only; neither borrows the other's prices.
func TestReturnsBenchmarkBasesReadTheirOwnObservations(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)
	index := f.commodity(t, "IDX", "security", 0)
	f.observe(t, index, f.eurCommodityID, "raw", "2026-01-31", 20000)
	f.observe(t, index, f.eurCommodityID, "raw", "2026-07-31", 22000)
	f.observe(t, index, f.eurCommodityID, "raw", "2027-01-31", 21000)
	f.observe(t, index, f.eurCommodityID, "total_return", "2026-01-31", 10000)
	f.observe(t, index, f.eurCommodityID, "total_return", "2026-02-14", 99900)
	f.observe(t, index, f.eurCommodityID, "total_return", "2027-01-31", 11500)

	price := f.readBenchmark(t, "2026-02-01", "2027-01-31", index, "")
	benchmark := price.Benchmark
	require.Equal(t, BenchmarkBasisPriceReturn, benchmark.Basis, "price return is the default")
	require.Equal(t, index, benchmark.CommodityID)
	requireReturn(t, "0.050000", benchmark.TimeWeighted) // 210 / 200 − 1
	requireReturn(t, "0.050000", benchmark.TimeWeightedAnnualized)
	requireReturn(t, "0.000000", benchmarkPoint(t, benchmark, "2026-01-31"))
	requireReturn(t, "0.000000", benchmarkPoint(t, benchmark, "2026-02-07")) // 7-day-old opening price
	requireUnavailableReturn(t, ReturnReasonValuationUnavailable, "2026-02-14", benchmarkPoint(t, benchmark, "2026-02-14"))
	requireReturn(t, "0.100000", benchmarkPoint(t, benchmark, "2026-08-01")) // 220 / 200 − 1
	require.Equal(t, SelectionPolicyLiveRaw, benchmark.Valuation.SelectionPolicy)
	require.False(t, benchmark.Valuation.Complete)
	// The portfolio is untouched: its figures and coverage are its own.
	requireReturn(t, "0.090909", price.TimeWeighted)
	require.True(t, price.Valuation.Complete)
	for _, gap := range price.TimeWeightedSeries.Valuation.Gaps {
		require.NotEqual(t, index, gap.CommodityID, "benchmark gaps stay in the benchmark's coverage")
	}

	total := f.readBenchmark(t, "2026-02-01", "2027-01-31", index, BenchmarkBasisTotalReturn)
	benchmark = total.Benchmark
	require.Equal(t, BenchmarkBasisTotalReturn, benchmark.Basis)
	requireReturn(t, "0.150000", benchmark.TimeWeighted) // 115 / 100 − 1
	requireReturn(t, "0.150000", benchmark.TimeWeightedAnnualized)
	requireReturn(t, "8.990000", benchmarkPoint(t, benchmark, "2026-02-14")) // 999 / 100 − 1
	requireUnavailableReturn(t, ReturnReasonValuationUnavailable, "2026-08-01", benchmarkPoint(t, benchmark, "2026-08-01"))
	require.Equal(t, SelectionPolicyLiveTotalReturn, benchmark.Valuation.SelectionPolicy)
	for _, use := range benchmark.Valuation.Used {
		require.NotEqual(t, "2026-07-31", use.ObservationDate, "a raw price never values a total-return benchmark")
	}
}

// B3: a total-return benchmark with only raw observations has no value; the
// opening gap removes every point, and the gap says the basis is wrong.
func TestReturnsBenchmarkTotalReturnRefusesRawPrices(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)
	index := f.commodity(t, "IDX", "security", 0)
	f.observe(t, index, f.eurCommodityID, "raw", "2026-01-31", 5000)
	f.observe(t, index, f.eurCommodityID, "raw", "2027-01-31", 5500)

	benchmark := f.readBenchmark(t, "2026-02-01", "2027-01-31", index, BenchmarkBasisTotalReturn).Benchmark
	requireUnavailableReturn(t, ReturnReasonValuationUnavailable, "2026-01-31", benchmark.TimeWeighted)
	requireUnavailableReturn(t, ReturnReasonValuationUnavailable, "2026-01-31", benchmark.TimeWeightedAnnualized)
	for _, point := range benchmark.Points {
		requireUnavailableReturn(t, ReturnReasonValuationUnavailable, "2026-01-31", point.TimeWeighted)
	}
	require.Len(t, benchmark.Valuation.Gaps, 1)
	require.Equal(t, index, benchmark.Valuation.Gaps[0].CommodityID)
	require.Equal(t, "incompatible_quote_basis", benchmark.Valuation.Gaps[0].Reason)
}

// B4: a benchmark quoted in another currency is converted into the reporting
// currency, so the investor's FX experience is part of it.
func TestReturnsBenchmarkConvertsThroughItsQuoteCurrency(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)
	usd := f.commodity(t, "USD", "currency", 2)
	index := f.commodity(t, "SPX", "security", 0)
	f.observe(t, index, usd, "raw", "2026-01-31", 10000)
	f.observe(t, index, usd, "raw", "2027-01-31", 11000)
	f.observe(t, usd, f.eurCommodityID, "raw", "2026-01-31", 90)
	f.observe(t, usd, f.eurCommodityID, "raw", "2027-01-31", 80)

	benchmark := f.readBenchmark(t, "2026-02-01", "2027-01-31", index, BenchmarkBasisPriceReturn).Benchmark
	// 100 USD × 0.90 = 90 EUR → 110 USD × 0.80 = 88 EUR: 88 / 90 − 1.
	requireReturn(t, "-0.022222", benchmark.TimeWeighted)
	used := false
	for _, use := range benchmark.Valuation.Used {
		used = used || (use.CommodityID == index && use.ViaCommodityID == usd)
	}
	require.True(t, used, "the coverage names the chained leg")
}

// B5/B6: a missing closing price removes the headline, dated the closing
// day; a period shorter than a year has no annual figure.
func TestReturnsBenchmarkClosingGapAndShortPeriod(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)
	f.price(t, "2026-06-30", 10500)
	index := f.commodity(t, "IDX", "security", 0)
	f.observe(t, index, f.eurCommodityID, "raw", "2026-01-31", 20000)
	f.observe(t, index, f.eurCommodityID, "raw", "2026-06-30", 21000)

	year := f.readBenchmark(t, "2026-02-01", "2027-01-31", index, "").Benchmark
	requireUnavailableReturn(t, ReturnReasonValuationUnavailable, "2027-01-31", year.TimeWeighted)
	requireUnavailableReturn(t, ReturnReasonValuationUnavailable, "2027-01-31", year.TimeWeightedAnnualized)
	requireReturn(t, "0.000000", benchmarkPoint(t, year, "2026-02-07"))

	short := f.readBenchmark(t, "2026-02-01", "2026-06-30", index, "").Benchmark
	requireReturn(t, "0.050000", short.TimeWeighted) // 210 / 200 − 1
	requireUnavailableReturn(t, ReturnReasonPeriodShorterThanYear, "", short.TimeWeightedAnnualized)
}

// Requesting a benchmark changes no portfolio figure, and an absent one is nil.
func TestReturnsBenchmarkLeavesThePortfolioReadUnchanged(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)
	index := f.commodity(t, "IDX", "security", 0)
	f.observe(t, index, f.eurCommodityID, "raw", "2026-01-31", 20000)

	plain := f.read(t, "2026-02-01", "2027-01-31")
	require.Nil(t, plain.Benchmark)
	with := f.readBenchmark(t, "2026-02-01", "2027-01-31", index, "")
	with.Benchmark = nil
	require.Equal(t, plain, with)
}

func TestReturnsBenchmarkValidation(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	read := func(benchmarkID int64, basis string) error {
		_, err := f.investmentService.ReadReturns(context.Background(), ReturnsInput{From: "2026-02-01", To: "2027-01-31",
			ReportingCommodityID: f.eurCommodityID, BenchmarkCommodityID: benchmarkID, BenchmarkBasis: basis})
		return err
	}
	for name, err := range map[string]error{
		"currency":          read(f.eurCommodityID, ""),
		"unknown commodity": read(999999, ""),
		"basis without one": read(0, BenchmarkBasisTotalReturn),
		"unknown basis":     read(f.stockCommodityID, "dividend_adjusted"),
	} {
		var validation ValidationError
		require.ErrorAs(t, err, &validation, name)
	}
	require.NoError(t, read(f.stockCommodityID, BenchmarkBasisTotalReturn), "a held security can be its own benchmark")
}

// B7–B11: a price-return benchmark holds the units one unit becomes, so a
// recorded split is neutral, as it is for the holding's ledger units
// (review of eeab1a2a, finding 1: raw quotes alone read a 2:1 split as −50 %).
func TestReturnsBenchmarkHoldsThroughRecordedSplits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	splitFixture := func(t *testing.T) *returnsFixture {
		f := newReturnsFixture(t)
		f.price(t, "2026-06-27", 10100)
		f.price(t, "2026-07-01", 5100)
		f.price(t, "2027-01-31", 5000)
		return f
	}

	t.Run("price return is split-neutral", func(t *testing.T) {
		t.Parallel()
		f := splitFixture(t)
		_, err := acknowledgedSplit(ctx, f.investmentService, splitInput(f.investmentsTestFixture, "2026-07-01", 2, 1))
		require.NoError(t, err)
		result := f.readBenchmark(t, "2026-02-01", "2027-01-31", f.stockCommodityID, BenchmarkBasisPriceReturn)
		requireReturn(t, "0.000000", result.TimeWeighted) // 1,100 → 100 + 20 × 50
		benchmark := result.Benchmark
		requireReturn(t, "0.000000", benchmark.TimeWeighted)                     // 100 → 2 × 50
		requireReturn(t, "0.010000", benchmarkPoint(t, benchmark, "2026-06-27")) // 101 / 100, before the split
		requireReturn(t, "0.020000", benchmarkPoint(t, benchmark, "2026-07-04")) // 2 × 51 / 100
		require.Equal(t, []db.ReturnsSplit{{EffectiveOn: "2026-07-01", RatioNumerator: 2, RatioDenominator: 1}}, benchmark.Splits)
	})

	t.Run("a total-return series is never split-adjusted", func(t *testing.T) {
		t.Parallel()
		f := splitFixture(t)
		_, err := acknowledgedSplit(ctx, f.investmentService, splitInput(f.investmentsTestFixture, "2026-07-01", 2, 1))
		require.NoError(t, err)
		f.observe(t, f.stockCommodityID, f.eurCommodityID, "total_return", "2026-01-31", 10000)
		f.observe(t, f.stockCommodityID, f.eurCommodityID, "total_return", "2027-01-31", 11000)
		benchmark := f.readBenchmark(t, "2026-02-01", "2027-01-31", f.stockCommodityID, BenchmarkBasisTotalReturn).Benchmark
		requireReturn(t, "0.100000", benchmark.TimeWeighted) // 110 / 100
		require.Empty(t, benchmark.Splits)
	})

	t.Run("one split recorded in two holdings applies once", func(t *testing.T) {
		t.Parallel()
		f := splitFixture(t)
		second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
		for _, holding := range []int64{f.holdingAccountID, second} {
			input := splitInput(f.investmentsTestFixture, "2026-07-01", 2, 1)
			input.HoldingAccountID = holding
			_, err := acknowledgedSplit(ctx, f.investmentService, input)
			require.NoError(t, err)
		}
		benchmark := f.readBenchmark(t, "2026-02-01", "2027-01-31", f.stockCommodityID, "").Benchmark
		requireReturn(t, "0.000000", benchmark.TimeWeighted)
		require.Len(t, benchmark.Splits, 1)
	})

	t.Run("conflicting ratios on one day are refused from that day", func(t *testing.T) {
		t.Parallel()
		f := splitFixture(t)
		second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
		for holding, numerator := range map[int64]int64{f.holdingAccountID: 2, second: 3} {
			input := splitInput(f.investmentsTestFixture, "2026-07-01", numerator, 1)
			input.HoldingAccountID = holding
			_, err := acknowledgedSplit(ctx, f.investmentService, input)
			require.NoError(t, err)
		}
		benchmark := f.readBenchmark(t, "2026-02-01", "2027-01-31", f.stockCommodityID, "").Benchmark
		requireReturn(t, "0.010000", benchmarkPoint(t, benchmark, "2026-06-27"))
		requireUnavailableReturn(t, ReturnReasonConflictingSplits, "2026-07-01", benchmarkPoint(t, benchmark, "2026-07-04"))
		requireUnavailableReturn(t, ReturnReasonConflictingSplits, "2026-07-01", benchmark.TimeWeighted)
		requireUnavailableReturn(t, ReturnReasonConflictingSplits, "2026-07-01", benchmark.TimeWeightedAnnualized)
		require.Empty(t, benchmark.Splits)
	})

	t.Run("a reversed split is not applied", func(t *testing.T) {
		t.Parallel()
		f := splitFixture(t)
		f.price(t, "2026-08-01", 10200)
		split, err := acknowledgedSplit(ctx, f.investmentService, splitInput(f.investmentsTestFixture, "2026-07-01", 2, 1))
		require.NoError(t, err)
		_, err = acknowledgedReverseSplit(ctx, f.investmentService, reverseSplitInput(f.investmentsTestFixture, split.Transaction.ID))
		require.NoError(t, err)
		benchmark := f.readBenchmark(t, "2026-02-01", "2026-08-01", f.stockCommodityID, "").Benchmark
		requireReturn(t, "0.020000", benchmark.TimeWeighted) // 102 / 100, no split
		require.Empty(t, benchmark.Splits)
	})
}
