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
// Raw quotes are per unit of their own day, so a price-return benchmark holds
// the units one unit becomes under the instrument's recorded splits, exactly
// as a holding's ledger units do, and prices them in the basis of the quote
// actually used; a split is not a loss, even beside a stale quote. A
// total-return series is a continuous index level and is never split-adjusted.

// Benchmark bases: which observations price the series. A price-return
// benchmark excludes distributions; total-return reads only observations the
// owner recorded as total_return. The basis is never inferred from the data.
const (
	BenchmarkBasisPriceReturn = "price_return"
	BenchmarkBasisTotalReturn = "total_return"
)

// ReturnReasonConflictingSplits: holdings record different split factors for
// the benchmark instrument on one day, so a value spanning that day is unknown.
const ReturnReasonConflictingSplits = "conflicting_splits"

// ReturnsBenchmark is the benchmark's cumulative return at the portfolio
// series' points, and its headline figures (the closing point).
type ReturnsBenchmark struct {
	CommodityID            int64
	Basis                  string
	TimeWeighted           ReturnFigure
	TimeWeightedAnnualized ReturnFigure
	Points                 []ReturnsSeriesPoint
	// Splits are the per-day split factors a price-return value used, in date
	// order; empty for total return.
	Splits    []ReturnsBenchmarkSplit
	Valuation ValuationCoverage
}

// ReturnsBenchmarkSplit is one day's split factor: every unit held at the end
// of the previous day became Factor units (a holding's successive splits that
// day compounded). Exact, so it is never rounded.
type ReturnsBenchmarkSplit struct {
	EffectiveOn string
	Factor      *big.Rat
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

// benchmarkSplitDays combines recorded splits (in date order) into one factor
// per day. A holding's successive splits on one day compound (2:1 twice is
// 4:1, as its ledger units are); holdings that recorded the same event must
// agree on the day's factor. A day they disagree on has no factor and is a
// conflict, rather than a guess.
func benchmarkSplitDays(recorded []db.ReturnsSplit) ([]ReturnsBenchmarkSplit, []string) {
	var days []ReturnsBenchmarkSplit
	var conflicts []string
	for start := 0; start < len(recorded); {
		end := start
		for end < len(recorded) && recorded[end].EffectiveOn == recorded[start].EffectiveOn {
			end++
		}
		byHolding := map[int64]*big.Rat{}
		for _, split := range recorded[start:end] {
			if byHolding[split.AccountID] == nil {
				byHolding[split.AccountID] = big.NewRat(1, 1)
			}
			byHolding[split.AccountID].Mul(byHolding[split.AccountID], big.NewRat(split.RatioNumerator, split.RatioDenominator))
		}
		var factor *big.Rat
		agree := true
		for _, holding := range byHolding {
			agree = agree && (factor == nil || factor.Cmp(holding) == 0)
			factor = holding
		}
		if agree {
			days = append(days, ReturnsBenchmarkSplit{EffectiveOn: recorded[start].EffectiveOn, Factor: factor})
		} else {
			conflicts = append(conflicts, recorded[start].EffectiveOn)
		}
		start = end
	}
	return days, conflicts
}

// benchmarkSeries reads the benchmark's value at each series date. One unit
// held from the opening day is C(d)/C(open) units on d, where C multiplies the
// split factors through a day. A quote observed on q prices a unit of q's
// basis, so the holding is worth price(q) × C(q)/C(open), whichever side of
// the opening day q falls; recorded splits must therefore cover the staleness
// window before the opening day too. The opening value outranks a point's own
// gap, as the portfolio series orders its required gaps; a value whose span
// (q to the opening day) crosses a conflicting day is unknown; a point-only
// gap removes that point alone. Prices are positive, so the growth factor is.
func benchmarkSeries(commodityID int64, basis string, rates *RateTable, recorded []db.ReturnsSplit, openDate string, dates []string, days int) ReturnsBenchmark {
	splitDays, conflicts := benchmarkSplitDays(recorded)
	used := map[string]bool{}
	value := func(date string) (*big.Rat, ReturnFigure) {
		price, observed, ok := rates.ConvertExactObserved(exact.New(1), 0, commodityID, date)
		if !ok {
			return nil, unavailableReturn(ReturnReasonValuationUnavailable, date)
		}
		low, high := min(observed, openDate), max(observed, openDate)
		for _, conflict := range conflicts {
			if conflict > low && conflict <= high {
				return nil, unavailableReturn(ReturnReasonConflictingSplits, conflict)
			}
		}
		for _, split := range splitDays {
			if split.EffectiveOn <= low || split.EffectiveOn > high {
				continue
			}
			used[split.EffectiveOn] = true
			if observed > openDate {
				price.Mul(price, split.Factor)
			} else {
				price.Quo(price, split.Factor)
			}
		}
		return price, ReturnFigure{}
	}

	benchmark := ReturnsBenchmark{CommodityID: commodityID, Basis: basis, Points: make([]ReturnsSeriesPoint, 0, len(dates))}
	opening, openFailure := value(openDate)
	var closingGrowth *big.Rat
	for i, date := range dates {
		point := ReturnsSeriesPoint{Date: date}
		current, failure := value(date)
		switch {
		case opening == nil:
			point.TimeWeighted = openFailure
		case current == nil:
			point.TimeWeighted = failure
		default:
			growth := new(big.Rat).Quo(current, opening)
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

	// splitDays is in date order, so the disclosure is too.
	benchmark.Splits = []ReturnsBenchmarkSplit{}
	for _, split := range splitDays {
		if used[split.EffectiveOn] {
			benchmark.Splits = append(benchmark.Splits, split)
		}
	}
	return benchmark
}
