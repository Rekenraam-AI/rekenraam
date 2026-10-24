package app

import (
	"math"
	"math/big"
	"sort"
)

// R13 returns_v1 arithmetic (docs/plans/investment-returns-plan.md). The
// functions here are pure: they take values and flows already converted into
// the reporting currency and know nothing about the ledger, so the plan's
// solver cases can pin them with literal inputs.

// Reasons a return figure is unavailable. Returned rather than described, so
// a screen can translate them.
const (
	ReturnReasonValuationUnavailable  = "valuation_unavailable"
	ReturnReasonFlowUnavailable       = "flow_unavailable"
	ReturnReasonUnclassifiedShort     = "unclassified_short"
	ReturnReasonNonPositiveCapital    = "non_positive_capital"
	ReturnReasonNoCapital             = "no_capital"
	ReturnReasonNoRoot                = "no_root"
	ReturnReasonRootNotUnique         = "root_not_unique"
	ReturnReasonPeriodShorterThanYear = "period_shorter_than_year"
	ReturnReasonNonPositiveGrowth     = "non_positive_growth"
	ReturnReasonNumericalLimit        = "numerical_limit"
)

// returnFractionPlaces is the presentation precision of every return: a
// decimal fraction, so six places is 0.0001 %.
const returnFractionPlaces = 6

// ReturnFigure is one return, or why it is missing. Date names the day a
// dated reason (a valuation gap, non-positive capital) applies to.
type ReturnFigure struct {
	Value  *string
	Reason string
	Date   string
}

func availableReturn(value string) ReturnFigure { return ReturnFigure{Value: &value} }

func unavailableReturn(reason, date string) ReturnFigure {
	return ReturnFigure{Reason: reason, Date: date}
}

// returnsDayFlow is one day's external flows as non-negative magnitudes,
// after correction chains have been netted.
type returnsDayFlow struct {
	In, Out *big.Rat
}

// timeWeightedReturn links Portfolio Performance's documented daily factor,
// inflows at the start of the day and outflows at its end:
//
//	1 + r(d) = (V(d) + Out(d)) / (V(d−1) + In(d))
//
// values holds V at the opening day and at every evaluation day the caller
// chose: the closing day, d−1 of every inflow, d of every outflow, and every
// day whose validity could not be proven without a price (the plan's
// "Evaluation days"). Runs between evaluation days telescope, so the product
// is exactly the daily one. A 0/0 factor is 1; any other non-positive
// denominator makes the figure unavailable, dated at the capital it measured.
func timeWeightedReturn(openDate string, values map[string]*big.Rat, flows map[string]returnsDayFlow) (*big.Rat, ReturnFigure) {
	if values[openDate].Sign() == 0 && len(flows) == 0 {
		return nil, unavailableReturn(ReturnReasonNoCapital, "")
	}
	dates := make([]string, 0, len(values)+len(flows))
	for date := range values {
		if date > openDate {
			dates = append(dates, date)
		}
	}
	for date := range flows {
		if _, ok := values[date]; !ok {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)

	product := big.NewRat(1, 1)
	prev, prevDate := values[openDate], openDate
	for _, date := range dates {
		flow, hasFlow := flows[date]
		start := prev
		if hasFlow && flow.In.Sign() > 0 {
			start = new(big.Rat).Add(prev, flow.In)
		}
		value, evaluated := values[date]
		if !evaluated {
			// An inflow-only day without a needed valuation: its factor and the
			// next run's share V(d), which cancels.
			prev, prevDate = start, date
			continue
		}
		numerator := value
		if hasFlow && flow.Out.Sign() > 0 {
			numerator = new(big.Rat).Add(value, flow.Out)
		}
		switch {
		case start.Sign() == 0 && numerator.Sign() == 0:
		case start.Sign() <= 0:
			return nil, unavailableReturn(ReturnReasonNonPositiveCapital, prevDate)
		default:
			product.Mul(product, new(big.Rat).Quo(numerator, start))
		}
		prev, prevDate = value, date
	}
	return product, availableReturn(formatReturnRat(new(big.Rat).Sub(product, big.NewRat(1, 1))))
}

// annualizedReturn compounds a period growth factor to 365 days. Shorter
// periods have no annual figure: extrapolating a month to a year produces
// numbers nobody earned.
func annualizedReturn(growth *big.Rat, days int) ReturnFigure {
	if days < 365 {
		return unavailableReturn(ReturnReasonPeriodShorterThanYear, "")
	}
	if growth.Sign() <= 0 {
		return unavailableReturn(ReturnReasonNonPositiveGrowth, "")
	}
	g, _ := growth.Float64()
	return availableReturn(formatReturnFloat(math.Pow(g, 365/float64(days)) - 1))
}

// returnsCashflow is one investor-perspective amount at a day offset:
// negative is money put in, positive is money taken out (or held at the end).
type returnsCashflow struct {
	Offset int
	Amount *big.Rat
}

// returnsLogGrowthLimit bounds |ln(1 + period return)|; exp beyond it leaves
// float64.
const returnsLogGrowthLimit = 700

// moneyWeightedGrowth solves Σ c_i (1+irr)^(−t_i/365) = 0 in log-growth
// space over the period, u = ln(1 + period return), so the terms are
// c_i · exp(−u · t_i / days) and a one-day 10 % return is u ≈ 0.095, not an
// annual rate beyond any fixed bracket.
//
// A root is solved only when Descartes' rule of signs proves it unique: one
// sign change in the time-ordered flows means exactly one root with
// irr > −1. Norström's running-sum criterion is deliberately not used; it
// proves only a unique non-negative rate (−1800, +2270, −309, +10 also has
// roots at −95 % and −90 %). Under Descartes a sign change of the NPV must
// exist, so failing to bracket one is a numerical limit, not an absent root.
func moneyWeightedGrowth(flows []returnsCashflow, days int) (float64, string) {
	merged := mergeReturnsCashflows(flows)
	positive, negative := false, false
	for _, flow := range merged {
		positive = positive || flow.Amount.Sign() > 0
		negative = negative || flow.Amount.Sign() < 0
	}
	if !positive || !negative {
		return 0, ReturnReasonNoRoot
	}
	if signChanges(merged) != 1 {
		return 0, ReturnReasonRootNotUnique
	}

	// Scale by the largest magnitude so the float sum stays well-conditioned.
	largest := new(big.Rat)
	for _, flow := range merged {
		if magnitude := new(big.Rat).Abs(flow.Amount); magnitude.Cmp(largest) > 0 {
			largest = magnitude
		}
	}
	amounts := make([]float64, len(merged))
	for i, flow := range merged {
		amounts[i], _ = new(big.Rat).Quo(flow.Amount, largest).Float64()
	}
	npv := func(u float64) float64 {
		total := 0.0
		for i, flow := range merged {
			total += amounts[i] * math.Exp(-u*float64(flow.Offset)/float64(days))
		}
		return total
	}
	low, high := -1.0, 1.0
	lowValue, highValue := npv(low), npv(high)
	for (lowValue > 0) == (highValue > 0) && lowValue != 0 && highValue != 0 {
		if high >= returnsLogGrowthLimit {
			return 0, ReturnReasonNumericalLimit
		}
		low, high = 2*low, 2*high
		lowValue, highValue = npv(low), npv(high)
	}
	switch {
	case lowValue == 0:
		return low, ""
	case highValue == 0:
		return high, ""
	}
	for range 200 {
		mid := low + (high-low)/2
		value := npv(mid)
		if value == 0 {
			return mid, ""
		}
		if (value > 0) == (lowValue > 0) {
			low, lowValue = mid, value
		} else {
			high = mid
		}
	}
	return low + (high-low)/2, ""
}

// growthReturn restates a period log growth u as a return over ratio periods:
// 1 for the period figure, 365/days for the annual one.
func growthReturn(u float64, ratio float64) ReturnFigure {
	value := math.Expm1(u * ratio)
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return unavailableReturn(ReturnReasonNumericalLimit, "")
	}
	return availableReturn(formatReturnFloat(value))
}

func mergeReturnsCashflows(flows []returnsCashflow) []returnsCashflow {
	byOffset := map[int]*big.Rat{}
	for _, flow := range flows {
		if byOffset[flow.Offset] == nil {
			byOffset[flow.Offset] = new(big.Rat)
		}
		byOffset[flow.Offset].Add(byOffset[flow.Offset], flow.Amount)
	}
	merged := make([]returnsCashflow, 0, len(byOffset))
	for offset, amount := range byOffset {
		if amount.Sign() != 0 {
			merged = append(merged, returnsCashflow{Offset: offset, Amount: amount})
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Offset < merged[j].Offset })
	return merged
}

// signChanges counts sign changes in the time-ordered amounts, ignoring zeros.
func signChanges(flows []returnsCashflow) int {
	changes, last := 0, 0
	for _, flow := range flows {
		sign := flow.Amount.Sign()
		if sign == 0 {
			continue
		}
		if last != 0 && sign != last {
			changes++
		}
		last = sign
	}
	return changes
}

// formatReturnRat rounds an exact fraction half away from zero to the
// presentation precision.
func formatReturnRat(value *big.Rat) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(returnFractionPlaces), nil)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(scale))
	quotient, remainder := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	twice := new(big.Int).Mul(new(big.Int).Abs(remainder), big.NewInt(2))
	if twice.Cmp(scaled.Denom()) >= 0 {
		if scaled.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	return formatScaledReturn(quotient)
}

func formatReturnFloat(value float64) string {
	rat := new(big.Rat)
	rat.SetFloat64(value)
	return formatReturnRat(rat)
}

func formatScaledReturn(scaled *big.Int) string {
	negative := scaled.Sign() < 0
	digits := new(big.Int).Abs(scaled).String()
	for len(digits) <= returnFractionPlaces {
		digits = "0" + digits
	}
	text := digits[:len(digits)-returnFractionPlaces] + "." + digits[len(digits)-returnFractionPlaces:]
	if negative && scaled.Sign() != 0 {
		text = "-" + text
	}
	return text
}
