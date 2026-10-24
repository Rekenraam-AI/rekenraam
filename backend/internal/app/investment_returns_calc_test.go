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

func TestTimeWeightedReturnWithoutFlowsIsTheValueRatio(t *testing.T) {
	t.Parallel()
	growth, figure := timeWeightedReturn("2026-01-31", rat(1100), nil, rat(1200))
	requireReturn(t, "0.090909", figure)
	requireReturn(t, "0.090909", annualizedReturn(growth, 365))
}

func TestTimeWeightedReturnLinksInflowAtStartAndOutflowAtEnd(t *testing.T) {
	t.Parallel()
	// S2: inflow 500 on 05-02 (V(05-01) = 1150), outflow 300 on 11-01
	// (V(11-01) = 1515). Only the breakpoints the convention needs are given.
	days := []returnsFlowDay{
		{Date: "2026-05-02", In: rat(500), Out: rat(0), Before: rat(1150)},
		{Date: "2026-11-01", In: rat(0), Out: rat(300), After: rat(1515)},
	}
	growth, figure := timeWeightedReturn("2026-01-31", rat(1100), days, rat(1565))
	// The middle run 1650 → 1815 is not a breakpoint: it telescopes away,
	// because V(05-02) and V(10-31) cancel in the daily product.
	require.Equal(t, 0, growth.Cmp(new(big.Rat).Mul(new(big.Rat).Mul(big.NewRat(1150, 1100), big.NewRat(1815, 1650)), big.NewRat(1565, 1515))))
	requireReturn(t, "0.187954", figure)
}

func TestTimeWeightedReturnOutflowOnlyDay(t *testing.T) {
	t.Parallel()
	// S3: a 20 withdrawal at the end of 07-31, V(07-31) = 1150.
	days := []returnsFlowDay{{Date: "2026-07-31", In: rat(0), Out: rat(20), After: rat(1150)}}
	_, figure := timeWeightedReturn("2026-01-31", rat(1100), days, rat(1200))
	requireReturn(t, "0.109881", figure)
}

func TestTimeWeightedReturnFromNothingStartsAtTheFirstInflow(t *testing.T) {
	t.Parallel()
	days := []returnsFlowDay{{Date: "2026-03-01", In: rat(1000), Out: rat(0), Before: rat(0)}}
	_, figure := timeWeightedReturn("2026-01-31", rat(0), days, rat(1050))
	requireReturn(t, "0.050000", figure)
}

func TestTimeWeightedReturnFailureStates(t *testing.T) {
	t.Parallel()
	_, figure := timeWeightedReturn("2026-01-31", rat(0), nil, rat(0))
	require.Equal(t, ReturnReasonNoCapital, figure.Reason)

	_, figure = timeWeightedReturn("2026-01-31", rat(-100), nil, rat(50))
	require.Equal(t, ReturnReasonNonPositiveCapital, figure.Reason)
	require.Equal(t, "2026-01-31", figure.Date)

	// A loss beyond the capital (a short) is a defined period return, but the
	// next sub-period starts from negative capital and is not.
	days := []returnsFlowDay{{Date: "2026-06-01", In: rat(0), Out: rat(10), After: rat(-50)}}
	_, figure = timeWeightedReturn("2026-01-31", rat(100), days, rat(-40))
	require.Equal(t, ReturnReasonNonPositiveCapital, figure.Reason)
	require.Equal(t, "2026-06-01", figure.Date)
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
		name     string
		flows    []returnsCashflow
		expected string
	}{
		{"E2 one year", []returnsCashflow{{0, rat(-1000)}, {365, rat(1100)}}, "0.100000"},
		{"S1", []returnsCashflow{{0, rat(-1100)}, {365, rat(1200)}}, "0.090909"},
		{"S2", []returnsCashflow{{0, rat(-1100)}, {90, rat(-500)}, {274, rat(300)}, {365, rat(1565)}}, "0.189483"},
		{"S3", []returnsCashflow{{0, rat(-1100)}, {181, rat(20)}, {365, rat(1200)}}, "0.110074"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, figure := moneyWeightedReturn(tc.flows)
			requireReturn(t, tc.expected, figure)
		})
	}
}

func TestMoneyWeightedReturnRefusesUnprovenRoots(t *testing.T) {
	t.Parallel()
	// E1: two sign changes in both the flows and their running sum.
	_, figure := moneyWeightedReturn([]returnsCashflow{{0, rat(-1000)}, {180, rat(2300)}, {365, rat(-1320)}})
	require.Equal(t, ReturnReasonRootNotUnique, figure.Reason)

	_, figure = moneyWeightedReturn([]returnsCashflow{{0, rat(-1000)}, {365, rat(0)}})
	require.Equal(t, ReturnReasonNoRoot, figure.Reason)

	// Two sign changes in the flows, but one in the running sum: Norström
	// proves the root unique, so it is solved.
	_, figure = moneyWeightedReturn([]returnsCashflow{{0, rat(-1000)}, {100, rat(200)}, {200, rat(-100)}, {365, rat(1000)}})
	require.Empty(t, figure.Reason)
}

func TestPeriodMoneyWeightedReturnRestatesOverDays(t *testing.T) {
	t.Parallel()
	requireReturn(t, "0.100000", periodMoneyWeightedReturn(0.1, 365))
	requireReturn(t, "0.048672", periodMoneyWeightedReturn(0.1, 182))
}

func TestFormatReturnRoundsHalfAwayFromZero(t *testing.T) {
	t.Parallel()
	require.Equal(t, "0.000001", formatReturnRat(big.NewRat(5, 10_000_000)))
	require.Equal(t, "-0.000001", formatReturnRat(big.NewRat(-5, 10_000_000)))
	require.Equal(t, "0.000000", formatReturnRat(big.NewRat(4, 10_000_000)))
	require.Equal(t, "-1.250000", formatReturnRat(big.NewRat(-5, 4)))
}
