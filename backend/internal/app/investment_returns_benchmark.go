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
//
// Raw quotes are per current unit, so a price-return benchmark holds the units
// one unit becomes under the instrument's recorded splits, exactly as a
// holding's ledger units do; a split is not a loss. A total-return series is
// a continuous index level and is never split-adjusted.

// Benchmark bases: which observations price the series. A price-return
// benchmark excludes distributions; total-return reads only observations the
// owner recorded as total_return. The basis is never inferred from the data.
const (
	BenchmarkBasisPriceReturn = "price_return"
	BenchmarkBasisTotalReturn = "total_return"
)

// ReturnReasonConflictingSplits: holdings record different split ratios for
// the benchmark instrument on one day, so its unit count is unknown from then.
const ReturnReasonConflictingSplits = "conflicting_splits"

// ReturnsBenchmark is the benchmark's cumulative return at the portfolio
// series' points, and its headline figures (the closing point).
type ReturnsBenchmark struct {
	CommodityID            int64
	Basis                  string
	TimeWeighted           ReturnFigure
	TimeWeightedAnnualized ReturnFigure
	Points                 []ReturnsSeriesPoint
	// Splits are the instrument's splits applied to a price-return benchmark,
	// one per day; empty for total return.
	Splits    []db.ReturnsSplit
	Valuation ValuationCoverage
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

// benchmarkSplits keeps one split per day: holdings that recorded the same
// event agree on its ratio. Different ratios on one day are a conflict, and
// the first such day is returned instead of guessing.
func benchmarkSplits(recorded []db.ReturnsSplit) ([]db.ReturnsSplit, string) {
	var splits []db.ReturnsSplit
	for _, split := range recorded {
		if last := len(splits) - 1; last >= 0 && splits[last].EffectiveOn == split.EffectiveOn {
			if splits[last].RatioNumerator != split.RatioNumerator || splits[last].RatioDenominator != split.RatioDenominator {
				return splits[:last], split.EffectiveOn
			}
			continue
		}
		splits = append(splits, split)
	}
	return splits, ""
}

// benchmarkSeries reads the benchmark's value at each series date: one unit
// held from the opening day, times the splits applied by then. The opening
// value outranks a point's own gap, and that outranks a split conflict at or
// before it, as the portfolio series orders its required gaps; a point-only
// gap removes that point alone. Prices are positive, so the growth factor is.
func benchmarkSeries(commodityID int64, basis string, rates *RateTable, recorded []db.ReturnsSplit, openDate string, dates []string, days int) ReturnsBenchmark {
	splits, conflictOn := benchmarkSplits(recorded)
	unit := func(date string) (*big.Rat, bool) {
		value, ok := rates.ConvertExact(exact.New(1), 0, commodityID, date)
		if !ok {
			return nil, false
		}
		for _, split := range splits {
			if split.EffectiveOn <= date {
				value.Mul(value, big.NewRat(split.RatioNumerator, split.RatioDenominator))
			}
		}
		return value, true
	}
	benchmark := ReturnsBenchmark{CommodityID: commodityID, Basis: basis, Splits: splits, Points: make([]ReturnsSeriesPoint, 0, len(dates))}
	if benchmark.Splits == nil {
		benchmark.Splits = []db.ReturnsSplit{}
	}
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
		case conflictOn != "" && date >= conflictOn:
			point.TimeWeighted = unavailableReturn(ReturnReasonConflictingSplits, conflictOn)
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
