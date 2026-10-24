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

// returnsFlowDay is one day with external flows. In and Out are non-negative
// magnitudes. Before is V(d−1), needed only when In > 0; After is V(d),
// needed only when Out > 0. Days without a flow never appear: their daily
// factors telescope into the neighbouring breakpoints.
type returnsFlowDay struct {
	Date          string
	In, Out       *big.Rat
	Before, After *big.Rat
}

// timeWeightedReturn links Portfolio Performance's documented daily factor,
// inflows at the start of the day and outflows at its end:
//
//	1 + r(d) = (V(d) + Out(d)) / (V(d−1) + In(d))
//
// over the minimal breakpoint set. The product is exact; only the result is
// rounded. A 0/0 factor (no capital, nothing happening) is 1; any other
// non-positive denominator makes the figure unavailable on that date.
func timeWeightedReturn(openDate string, open *big.Rat, days []returnsFlowDay, closing *big.Rat) (*big.Rat, ReturnFigure) {
	if open.Sign() == 0 && len(days) == 0 {
		return nil, unavailableReturn(ReturnReasonNoCapital, "")
	}
	product := big.NewRat(1, 1)
	link := func(numerator, denominator *big.Rat) bool {
		if denominator.Sign() == 0 && numerator.Sign() == 0 {
			return true
		}
		if denominator.Sign() <= 0 {
			return false
		}
		product.Mul(product, new(big.Rat).Quo(numerator, denominator))
		return true
	}
	prev, prevDate := open, openDate
	for _, day := range days {
		start := prev
		if day.In.Sign() > 0 {
			if !link(day.Before, prev) {
				return nil, unavailableReturn(ReturnReasonNonPositiveCapital, prevDate)
			}
			start = new(big.Rat).Add(day.Before, day.In)
			prevDate = day.Date
		}
		if day.Out.Sign() > 0 {
			if !link(new(big.Rat).Add(day.After, day.Out), start) {
				return nil, unavailableReturn(ReturnReasonNonPositiveCapital, prevDate)
			}
			prev, prevDate = day.After, day.Date
			continue
		}
		prev = start
	}
	if !link(closing, prev) {
		return nil, unavailableReturn(ReturnReasonNonPositiveCapital, prevDate)
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

// moneyWeightedReturn solves Σ c_i (1+irr)^(−t_i/365) = 0, Actual/365, but
// only once a unique root is proven: Descartes' rule of signs on the
// time-ordered flows, or Norström's criterion on their running sum. A root is
// never chosen among several.
func moneyWeightedReturn(flows []returnsCashflow) (float64, ReturnFigure) {
	merged := mergeReturnsCashflows(flows)
	positive, negative := false, false
	for _, flow := range merged {
		positive = positive || flow.Amount.Sign() > 0
		negative = negative || flow.Amount.Sign() < 0
	}
	if !positive || !negative {
		return 0, unavailableReturn(ReturnReasonNoRoot, "")
	}
	if signChanges(merged, false) != 1 && !norstromUnique(merged) {
		return 0, unavailableReturn(ReturnReasonRootNotUnique, "")
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
	npv := func(rate float64) float64 {
		total := 0.0
		for i, flow := range merged {
			total += amounts[i] * math.Pow(1+rate, -float64(flow.Offset)/365)
		}
		return total
	}
	low, high := -0.999999, 1e6
	lowValue, highValue := npv(low), npv(high)
	if lowValue == 0 {
		return low, availableReturn(formatReturnFloat(low))
	}
	if (lowValue > 0) == (highValue > 0) {
		return 0, unavailableReturn(ReturnReasonNoRoot, "")
	}
	for range 200 {
		mid := low + (high-low)/2
		value := npv(mid)
		if value == 0 {
			low, high = mid, mid
			break
		}
		if (value > 0) == (lowValue > 0) {
			low, lowValue = mid, value
		} else {
			high = mid
		}
	}
	rate := low + (high-low)/2
	return rate, availableReturn(formatReturnFloat(rate))
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

// signChanges counts sign changes in the amounts, or in their running sum
// when cumulative is set, ignoring zeros.
func signChanges(flows []returnsCashflow, cumulative bool) int {
	changes, last := 0, 0
	running := new(big.Rat)
	for _, flow := range flows {
		value := flow.Amount
		if cumulative {
			running.Add(running, flow.Amount)
			value = running
		}
		sign := value.Sign()
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

// norstromUnique is Norström's sufficient condition: the running sum starts
// non-zero and changes sign exactly once.
func norstromUnique(flows []returnsCashflow) bool {
	return len(flows) > 0 && flows[0].Amount.Sign() != 0 && signChanges(flows, true) == 1
}

// periodMoneyWeightedReturn restates an annual IRR over the period's days.
func periodMoneyWeightedReturn(rate float64, days int) ReturnFigure {
	return availableReturn(formatReturnFloat(math.Pow(1+rate, float64(days)/365) - 1))
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
