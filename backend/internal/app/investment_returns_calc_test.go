package app

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expected figures come from the plan's independent table
// (docs/plans/investment-returns-plan.md, "Expected cases"), computed outside
// the app. They are literals here, never re-derived from the calculator.

func rat(value int64) *big.Rat { return big.NewRat(value, 1) }

func requireReturn(t *testing.T, expected string, figure ReturnFigure) {
	t.Helper()
	require.Empty(t, figure.Reason)
	require.NotNil(t, figure.Value)
	require.Equal(t, expected, *figure.Value)
}

func values(pairs ...any) map[string]*big.Rat {
	result := map[string]*big.Rat{}
	for i := 0; i < len(pairs); i += 2 {
		result[pairs[i].(string)] = rat(int64(pairs[i+1].(int)))
	}
	return result
}

func inflow(amount int64) returnsDayFlow  { return returnsDayFlow{In: rat(amount), Out: rat(0)} }
func outflow(amount int64) returnsDayFlow { return returnsDayFlow{In: rat(0), Out: rat(amount)} }

func TestTimeWeightedReturnWithoutFlowsIsTheValueRatio(t *testing.T) {
	t.Parallel()
	growth, figure := timeWeightedReturn("2026-01-31", values("2026-01-31", 1100, "2027-01-31", 1200), nil)
	requireReturn(t, "0.090909", figure)
	requireReturn(t, "0.090909", annualizedReturn(growth, 365))
}

func TestTimeWeightedReturnLinksInflowAtStartAndOutflowAtEnd(t *testing.T) {
	t.Parallel()
	// S2: inflow 500 on 05-02 (V(05-01) = 1150), outflow 300 on 11-01
	// (V(11-01) = 1515). The run 1650 → 1815 is not evaluated: V(05-02) and
	// V(10-31) cancel in the daily product.
	growth, figure := timeWeightedReturn("2026-01-31",
		values("2026-01-31", 1100, "2026-05-01", 1150, "2026-11-01", 1515, "2027-01-31", 1565),
		map[string]returnsDayFlow{"2026-05-02": inflow(500), "2026-11-01": outflow(300)})
	require.Equal(t, 0, growth.Cmp(new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(1150, 1100), big.NewRat(1815, 1650)), big.NewRat(1565, 1515))))
	requireReturn(t, "0.187954", figure)
}

func TestTimeWeightedReturnOutflowOnlyDay(t *testing.T) {
	t.Parallel()
	// S3: a 20 withdrawal at the end of 07-31, V(07-31) = 1150.
	_, figure := timeWeightedReturn("2026-01-31", values("2026-01-31", 1100, "2026-07-31", 1150, "2027-01-31", 1200),
		map[string]returnsDayFlow{"2026-07-31": outflow(20)})
	requireReturn(t, "0.109881", figure)
}

func TestTimeWeightedReturnFromNothingStartsAtTheFirstInflow(t *testing.T) {
	t.Parallel()
	_, figure := timeWeightedReturn("2026-01-31", values("2026-01-31", 0, "2026-02-28", 0, "2027-01-31", 1050),
		map[string]returnsDayFlow{"2026-03-01": inflow(1000)})
	requireReturn(t, "0.050000", figure)
}

func TestTimeWeightedReturnFailureStates(t *testing.T) {
	t.Parallel()
	_, figure := timeWeightedReturn("2026-01-31", values("2026-01-31", 0, "2027-01-31", 0), nil)
	require.Equal(t, ReturnReasonNoCapital, figure.Reason)

	_, figure = timeWeightedReturn("2026-01-31", values("2026-01-31", -100, "2027-01-31", 50), nil)
	require.Equal(t, ReturnReasonNonPositiveCapital, figure.Reason)
	require.Equal(t, "2026-01-31", figure.Date)

	// R4: an evaluated day with negative capital is the denominator of the
	// next factor, whatever the boundary values say.
	_, figure = timeWeightedReturn("2026-01-31", values("2026-01-31", 100, "2026-07-29", 100, "2026-07-30", -100, "2026-07-31", 100, "2027-01-31", 100), nil)
	require.Equal(t, ReturnReasonNonPositiveCapital, figure.Reason)
	require.Equal(t, "2026-07-30", figure.Date)
}

func TestAnnualizationNeedsAYearAndPositiveGrowth(t *testing.T) {
	t.Parallel()
	require.Equal(t, ReturnReasonPeriodShorterThanYear, annualizedReturn(big.NewRat(11, 10), 364).Reason)
	require.Equal(t, ReturnReasonNonPositiveGrowth, annualizedReturn(big.NewRat(-1, 10), 730).Reason)
	// Portfolio Performance's documented example: 30 % over 730 days.
	requireReturn(t, "0.140175", annualizedReturn(big.NewRat(13, 10), 730))
}

func TestMoneyWeightedReturnLiteralCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		flows          []returnsCashflow
		days           int
		annual, period string
	}{
		{"E2 one year", []returnsCashflow{{0, rat(-1000)}, {365, rat(1100)}}, 365, "0.100000", "0.100000"},
		{"S1", []returnsCashflow{{0, rat(-1100)}, {365, rat(1200)}}, 365, "0.090909", "0.090909"},
		{"S2", []returnsCashflow{{0, rat(-1100)}, {90, rat(-500)}, {274, rat(300)}, {365, rat(1565)}}, 365, "0.189483", "0.189483"},
		{"S3", []returnsCashflow{{0, rat(-1100)}, {181, rat(20)}, {365, rat(1200)}}, 365, "0.110074", "0.110074"},
		// R6: the annual equivalent of 10 % in a day (~1.3e15) is far beyond
		// the old fixed annual bracket, but the period figure exists. The
		// service withholds the annual figure below 365 days anyway.
		{"R6 one day", []returnsCashflow{{0, rat(-100)}, {1, rat(110)}}, 1, "", "0.100000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			growth, reason := moneyWeightedGrowth(tc.flows, tc.days)
			require.Empty(t, reason)
			requireReturn(t, tc.period, growthReturn(growth, 1))
			if tc.annual != "" {
				requireReturn(t, tc.annual, growthReturn(growth, 365/float64(tc.days)))
			}
		})
	}
}

func TestMoneyWeightedReturnRefusesUnprovenRoots(t *testing.T) {
	t.Parallel()
	// E1: two sign changes.
	_, reason := moneyWeightedGrowth([]returnsCashflow{{0, rat(-1000)}, {180, rat(2300)}, {365, rat(-1320)}}, 365)
	require.Equal(t, ReturnReasonRootNotUnique, reason)

	// R2: roots at −95 %, −90 % and +11.1111 %. Norström's running-sum
	// criterion holds here (it proves only the non-negative root unique), so
	// it must not be the uniqueness proof.
	_, reason = moneyWeightedGrowth([]returnsCashflow{{0, rat(-1800)}, {365, rat(2270)}, {730, rat(-309)}, {1095, rat(10)}}, 1095)
	require.Equal(t, ReturnReasonRootNotUnique, reason)

	_, reason = moneyWeightedGrowth([]returnsCashflow{{0, rat(-1000)}, {365, rat(0)}}, 365)
	require.Equal(t, ReturnReasonNoRoot, reason)

	// A root beyond float64's exp range is a numerical limit, not absence.
	_, reason = moneyWeightedGrowth([]returnsCashflow{{0, rat(-1)}, {1, big.NewRat(1, 1).SetFrac(big.NewInt(1), big.NewInt(1).Exp(big.NewInt(10), big.NewInt(400), nil))}}, 1)
	require.Equal(t, ReturnReasonNumericalLimit, reason)
}

func TestFormatReturnRoundsHalfAwayFromZero(t *testing.T) {
	t.Parallel()
	require.Equal(t, "0.000001", formatReturnRat(big.NewRat(5, 10_000_000)))
	require.Equal(t, "-0.000001", formatReturnRat(big.NewRat(-5, 10_000_000)))
	require.Equal(t, "0.000000", formatReturnRat(big.NewRat(4, 10_000_000)))
	require.Equal(t, "-1.250000", formatReturnRat(big.NewRat(-5, 4)))
}

func TestReturnsSeriesDatesChooseTheIntervalByPeriodLength(t *testing.T) {
	t.Parallel()
	interval, dates := returnsSeriesDates("2026-01-31", "2026-02-03", 3)
	require.Equal(t, ReturnsSeriesDaily, interval)
	require.Equal(t, []string{"2026-01-31", "2026-02-01", "2026-02-02", "2026-02-03"}, dates)

	// 92 days is the last daily period; 93 is weekly.
	interval, dates = returnsSeriesDates("2026-01-31", "2026-05-03", 92)
	require.Equal(t, ReturnsSeriesDaily, interval)
	require.Len(t, dates, 93)
	interval, dates = returnsSeriesDates("2026-01-31", "2026-05-04", 93)
	require.Equal(t, ReturnsSeriesWeekly, interval)
	require.Equal(t, []string{"2026-01-31", "2026-02-07", "2026-02-14"}, dates[:3])
	require.Equal(t, "2026-05-02", dates[len(dates)-2])
	require.Equal(t, "2026-05-04", dates[len(dates)-1])

	// A weekly point on the closing day is not repeated.
	_, dates = returnsSeriesDates("2026-01-31", "2026-05-09", 98)
	require.Equal(t, []string{"2026-05-02", "2026-05-09"}, dates[len(dates)-2:])

	// Monthly from a mid-month opening day: that month's end comes first;
	// February ends on its own last day.
	interval, dates = returnsSeriesDates("2024-01-15", "2026-03-10", 786)
	require.Equal(t, ReturnsSeriesMonthly, interval)
	require.Equal(t, []string{"2024-01-15", "2024-01-31", "2024-02-29", "2024-03-31"}, dates[:4])
	require.Equal(t, []string{"2026-01-31", "2026-02-28", "2026-03-10"}, dates[len(dates)-3:])
}

func TestTimeWeightedChainRecordsCumulativeGrowthUntilItFails(t *testing.T) {
	t.Parallel()
	growth, failedOn, failure := timeWeightedChain("2026-01-31",
		values("2026-01-31", 1100, "2026-05-01", 1150, "2026-11-01", 1515, "2027-01-31", 1565),
		map[string]returnsDayFlow{"2026-05-02": inflow(500), "2026-11-01": outflow(300)})
	require.Empty(t, failedOn)
	require.Empty(t, failure.Reason)
	require.Equal(t, "0.045455", formatReturnRat(new(big.Rat).Sub(growth["2026-05-01"], rat(1))))
	require.Equal(t, "0.187954", formatReturnRat(new(big.Rat).Sub(growth["2027-01-31"], rat(1))))

	// R4: the factor ending 07-31 divides by V(07-30) < 0; growth stops there.
	growth, failedOn, failure = timeWeightedChain("2026-01-31",
		values("2026-01-31", 100, "2026-07-29", 100, "2026-07-30", -100, "2026-07-31", 100, "2027-01-31", 100), nil)
	require.Equal(t, "2026-07-31", failedOn)
	require.Equal(t, ReturnReasonNonPositiveCapital, failure.Reason)
	require.Equal(t, "2026-07-30", failure.Date)
	require.Contains(t, growth, "2026-07-29")
	require.NotContains(t, growth, "2026-07-31")
}
