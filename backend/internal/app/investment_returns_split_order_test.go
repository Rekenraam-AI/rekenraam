package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// Same-day event order around a split (combined review of cae8039d, 21106562
// and 129abf3f). A split splits the lots that exist at its replay slot, so a
// trade or delivery ordered before it that day carried pre-split units: its
// implied quote is cash per pre-split unit, and its posted units each became
// the split's factor. Expected figures are hand-derived from the fixture: 10
// TEST at 100 and broker cash 100 at the opening boundary (2026-01-31).

// O1: a trade quote ordered before the day's 2:1 split priced a pre-split
// unit, 100, which is 50 per end-of-day unit; 22 units are worth 1,100.
func TestSplitDayTradeQuoteOrderedBeforeTheSplitPricesPreSplitUnits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newReturnsFixture(t)
	f.buyFromBrokerCash(t, "2026-02-02", 10000)
	_, err := acknowledgedSplit(ctx, f.investmentService, splitInput(f.investmentsTestFixture, "2026-02-02", 2, 1))
	require.NoError(t, err)

	result := f.read(t, "2026-02-01", "2026-02-03")
	requireReturnsAmount(t, 1100, result.ClosingValue) // 22 × 50 + 0 cash
	requireReturn(t, "0.000000", result.TimeWeighted)

	_, gains, err := f.investmentService.UnrealizedGainsProjection(ctx, "2026-02-03")
	require.NoError(t, err)
	require.Len(t, gains, 1)
	require.NotNil(t, gains[0].MarketValueValue)
	require.Equal(t, 0, exact.ScaledIntFromInt64(*gains[0].MarketValueValue, int(*gains[0].MarketValueScale)).
		Cmp(exact.ScaledIntFromInt64(110000, 2)), "holdings value 22 units at 50")
}

// O2, the control: a trade ordered after the split carried post-split units,
// so its quote of 50 is already per end-of-day unit; 21 × 50 + 50 cash.
func TestSplitDayTradeQuoteOrderedAfterTheSplitIsUnchanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newReturnsFixture(t)
	_, err := acknowledgedSplit(ctx, f.investmentService, splitInput(f.investmentsTestFixture, "2026-02-02", 2, 1))
	require.NoError(t, err)
	f.buyFromBrokerCash(t, "2026-02-02", 5000)

	result := f.read(t, "2026-02-01", "2026-02-03")
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// O3: a delivery of 1 TEST ordered before the day's split moved one pre-split
// unit, worth two end-of-day units: a contribution of 100, whether the day's
// rate comes from a rescaled stale quote or a fresh post-split one.
func TestSplitDayDeliveryOrderedBeforeTheSplitCountsPreSplitUnits(t *testing.T) {
	t.Parallel()
	for name, fresh := range map[string]bool{"stale quote": false, "fresh quote": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newReturnsFixture(t)
			seedExternalTransferEquity(t, f.database)
			_, err := f.investmentService.ExternalTransferIn(context.Background(), ExternalTransferInInput{
				OwnerUserID: f.ownerUserID, EffectiveOn: "2026-02-02", HoldingAccountID: f.holdingAccountID,
				CommodityID: f.stockCommodityID, QuantityValue: exact.New(1), CostCommodityID: f.eurCommodityID, BasisKnowledge: "unknown",
			})
			require.NoError(t, err)
			_, err = acknowledgedSplit(context.Background(), f.investmentService, splitInput(f.investmentsTestFixture, "2026-02-02", 2, 1))
			require.NoError(t, err)
			if fresh {
				f.price(t, "2026-02-02", 5000)
			}
			result := f.read(t, "2026-02-01", "2026-02-03")
			requireReturnsAmount(t, 1200, result.ClosingValue) // 22 × 50 + 100
			requireReturnsAmount(t, 100, result.Contributions)
			requireReturn(t, "0.000000", result.TimeWeighted)
		})
	}
}

// The reviewer's chained-FX probe, kept: an unheld benchmark quoted in USD
// with its split recorded in an empty holding stays split-neutral, and only
// the FX move (0.90 → 0.80) remains.
func TestSplitNeutralChainedBenchmark(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	usd := f.commodity(t, "USD", "currency", 2)
	index := f.commodity(t, "CHIDX", "security", 0)
	input := splitInput(f.investmentsTestFixture, "2026-02-02", 2, 1)
	input.CommodityID = index
	_, err := acknowledgedSplit(context.Background(), f.investmentService, input)
	require.NoError(t, err)
	f.observe(t, index, usd, "raw", "2026-01-31", 10000)
	f.observe(t, index, usd, "raw", "2026-02-04", 5000)
	f.observe(t, usd, f.eurCommodityID, "raw", "2026-01-31", 90)
	f.observe(t, usd, f.eurCommodityID, "raw", "2026-02-03", 80)
	benchmark := f.readBenchmark(t, "2026-02-01", "2026-02-04", index, "").Benchmark
	requireReturn(t, "-0.111111", benchmarkPoint(t, benchmark, "2026-02-03"))
	requireReturn(t, "-0.111111", benchmark.TimeWeighted)
}

// buyFromBrokerCash buys 1 TEST into the holding for the given cents, inside
// the portfolio; the buy also records its trade-implied quote.
func (f *returnsFixture) buyFromBrokerCash(t *testing.T, date string, cents int64) {
	t.Helper()
	_, err := f.investmentService.Buy(context.Background(), InvestmentTradeInput{
		OwnerUserID: f.ownerUserID, TransactionDate: date, CommodityID: f.stockCommodityID,
		HoldingAccountID: f.holdingAccountID, CashAccountID: f.brokerCashID,
		QuantityValue: exact.New(1), CashAmountValue: cents, CashAmountScale: 2, CashCommodityID: f.eurCommodityID,
	})
	require.NoError(t, err)
}
