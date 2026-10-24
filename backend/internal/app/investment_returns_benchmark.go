package app

import (
	"context"
	"math/big"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// R13 benchmark (docs/plans/investment-returns-plan.md, slice 5). A benchmark
// is one owner-selected security or crypto commodity read as a hypothetical
// single unit held through the period: no flows, so its time-weighted chain
// telescopes to the ratio of its end values, and requiring the portfolio's
// evaluation days would only add failures that change no figure. It is valued
// in the reporting currency by its own rate table, so its gaps never mark the
// portfolio's coverage incomplete. Manual observations are as valid as any.

// Benchmark bases: which observations price the series. A price-return
// benchmark excludes distributions; total-return reads only observations the
// owner recorded as total_return. The basis is never inferred from the data.
const (
	BenchmarkBasisPriceReturn = "price_return"
	BenchmarkBasisTotalReturn = "total_return"
)

// ReturnsBenchmark is the benchmark's cumulative return at the portfolio
// series' points, and its headline figures (the closing point).
type ReturnsBenchmark struct {
	CommodityID            int64
	Basis                  string
	TimeWeighted           ReturnFigure
	TimeWeightedAnnualized ReturnFigure
	Points                 []ReturnsSeriesPoint
	Valuation              ValuationCoverage
}

// newBenchmarkRates loads the benchmark's observations over the read's
// window under the basis's selection.
func (r *returnsReader) newBenchmarkRates(ctx context.Context, commodityID int64, basis string, reporting ReportingCurrencyInput, boundary db.ObservationBoundary) (*RateTable, error) {
	pricing := r.pricing
	if basis == BenchmarkBasisTotalReturn {
		pricing = pricing.WithTotalReturnSecurities()
	}
	table, err := newRateTableAtBoundary(ctx, pricing, reporting, []int64{commodityID}, r.openDate, r.to, boundary)
	if err != nil {
		return nil, err
	}
	if basis == BenchmarkBasisTotalReturn {
		table.selectionPolicy = SelectionPolicyLiveTotalReturn
	}
	return table, nil
}

// benchmarkSeries reads one unit's value at each series date. The opening value
// outranks a point's own gap, as in the portfolio series; a point-only gap
// removes that point alone. Prices are positive, so the growth factor is.
func benchmarkSeries(commodityID int64, basis string, rates *RateTable, openDate string, dates []string, days int) ReturnsBenchmark {
	unit := func(date string) (*big.Rat, bool) {
		return rates.ConvertExact(exact.New(1), 0, commodityID, date)
	}
	benchmark := ReturnsBenchmark{CommodityID: commodityID, Basis: basis, Points: make([]ReturnsSeriesPoint, 0, len(dates))}
	opening, openKnown := unit(openDate)
	var closingGrowth *big.Rat
	for i, date := range dates {
		point := ReturnsSeriesPoint{Date: date}
		value, known := unit(date)
		switch {
		case !openKnown:
			point.TimeWeighted = unavailableReturn(ReturnReasonValuationUnavailable, openDate)
		case !known:
			point.TimeWeighted = unavailableReturn(ReturnReasonValuationUnavailable, date)
		default:
			growth := new(big.Rat).Quo(value, opening)
			point.TimeWeighted = availableReturn(formatReturnRat(new(big.Rat).Sub(growth, big.NewRat(1, 1))))
			if i == len(dates)-1 {
				closingGrowth = growth
			}
		}
		benchmark.Points = append(benchmark.Points, point)
	}
	benchmark.TimeWeighted = benchmark.Points[len(benchmark.Points)-1].TimeWeighted
	benchmark.TimeWeightedAnnualized = benchmark.TimeWeighted
	if closingGrowth != nil {
		benchmark.TimeWeightedAnnualized = annualizedReturn(closingGrowth, days)
	}
	return benchmark
}
