package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

// #230 (R18 slice 1): operational_v1 values positions as they stood on a
// chosen date, priced by live_raw_v1 at that date, without writing anything.

func priceStockOn(t *testing.T, f *investmentsTestFixture, date string, value int64) {
	t.Helper()
	pricing := NewPricingService(db.NewPricingRepository(f.database))
	_, err := pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID,
		BaseCommodityID: f.stockCommodityID, QuoteCommodityID: f.eurCommodityID, QuoteType: "close",
		AdjustmentBasis: "raw", ValuationDate: date, PriceValue: value, PriceScale: 2})
	require.NoError(t, err)
}

func datedValuation(t *testing.T, f *investmentsTestFixture, asOf string) (GainsProfile, []UnrealizedGainEntry) {
	t.Helper()
	profile, entries, err := f.investmentService.UnrealizedGainsProjection(context.Background(), asOf)
	require.NoError(t, err)
	return profile, entries
}

func TestDatedValuationValuesAHoldingALaterSaleClosedAtItsDatedPrice(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-05-01", 2, 4000)
	priceStockOn(t, f, "2026-05-28", 2500)
	priceStockOn(t, f, "2026-06-15", 9900) // after the valuation date: never used for it
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-07-01", 2))
	require.NoError(t, err)
	buyOn(t, f, "2026-08-01", 5, 5000) // bought after the valuation date

	before := buyReplacementPreviewSnapshot(t, f.database)
	profile, entries := datedValuation(t, f, "2026-06-01")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "a dated valuation writes nothing")

	assert.Equal(t, GainsProfile{Name: "operational_v1", Purpose: "operational", BasisMethod: "operational_elections",
		ValuationAsOf: "2026-06-01", KnowledgeCutoff: profile.KnowledgeCutoff, LedgerKnowledge: "current_effective",
		QuotePolicy: "live_raw_v1", StaleAfterDays: 7, FXMethod: "none", Rounding: "exact_half_up"}, profile)
	assert.NotEmpty(t, profile.KnowledgeCutoff)
	require.Len(t, entries, 1, "the later purchase is not held on the valuation date")
	position := entries[0]
	assert.Equal(t, "2", position.QuantityValue.String(), "held then though sold since")
	assertMoneyValue(t, 4000, 2, position.RemainingCostBasisValue, position.RemainingCostBasisScale, "basis held then")
	assert.Equal(t, "2026-05-28", position.LatestPriceDate)
	assert.Equal(t, "usable", position.PriceStatus)
	assert.Equal(t, "2026-06-01", position.ValuationAsOf)
	require.NotNil(t, position.MarketValueValue)
	assertMoneyValue(t, 5000, 2, *position.MarketValueValue, *position.MarketValueScale, "2 × 25.00")
	require.NotNil(t, position.UnrealizedGainValue)
	assertMoneyValue(t, 1000, 2, *position.UnrealizedGainValue, *position.UnrealizedGainScale, "50.00 − 40.00")
	assert.Equal(t, db.PositionSideLong, position.PositionSide)
}

func TestDatedValuationOfAShortCoveredSince(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	shortSaleOn(t, f, "2026-03-02", 10, 10000)
	priceStockOn(t, f, "2026-03-10", 800)
	_, err := f.investmentService.ShortCover(context.Background(), shortInput(f, "2026-04-01", 10, 7000))
	require.NoError(t, err)

	_, entries := datedValuation(t, f, "2026-03-15")
	require.Len(t, entries, 1)
	position := entries[0]
	assert.Equal(t, db.PositionSideShort, position.PositionSide)
	assert.Equal(t, "10", position.QuantityValue.String(), "owed units then")
	require.NotNil(t, position.MarketValueValue)
	assertMoneyValue(t, -8000, 2, *position.MarketValueValue, *position.MarketValueScale, "owed at 8.00")
	require.NotNil(t, position.UnrealizedGainValue)
	assertMoneyValue(t, 2000, 2, *position.UnrealizedGainValue, *position.UnrealizedGainScale, "100.00 opened − 80.00 to cover")

	_, current := datedValuation(t, f, "")
	assert.Empty(t, current, "covered today")
}

func TestDatedValuationAtTodayMatchesTheCurrentProjection(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-05-01", 3, 3001)
	buyOn(t, f, "2026-05-15", 3, 4507)
	_, err := f.investmentService.Sell(ctx, sellInput(f, "2026-07-01", 4))
	require.NoError(t, err)
	priceStockOn(t, f, "2026-07-02", 1733)

	today, current := datedValuation(t, f, "")
	_, dated := datedValuation(t, f, today.ValuationAsOf)
	require.Len(t, dated, len(current))
	for i := range current {
		live, replayed := current[i], dated[i]
		assert.Equal(t, live.QuantityValue.String(), replayed.QuantityValue.String())
		assertMoneyValue(t, live.RemainingCostBasisValue, live.RemainingCostBasisScale, replayed.RemainingCostBasisValue, replayed.RemainingCostBasisScale, "same basis")
		assert.Equal(t, live.LatestPriceDate, replayed.LatestPriceDate)
		assert.Equal(t, live.PriceStatus, replayed.PriceStatus)
		require.NotNil(t, live.UnrealizedGainValue)
		require.NotNil(t, replayed.UnrealizedGainValue)
		assertMoneyValue(t, *live.UnrealizedGainValue, *live.UnrealizedGainScale, *replayed.UnrealizedGainValue, *replayed.UnrealizedGainScale, "same gain")
	}
}

func TestDatedValuationDisclosesStaleAndFutureOnlyPrices(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixtureWithOptions(t, false) // no trade-implied fill price
	buyOn(t, f, "2026-05-01", 2, 4000)
	priceStockOn(t, f, "2026-05-02", 2100)
	priceStockOn(t, f, "2026-07-01", 2600)

	_, stale := datedValuation(t, f, "2026-05-10")
	require.Len(t, stale, 1)
	assert.Equal(t, "stale", stale[0].PriceStatus, "eight days old is retained but stale")
	require.NotNil(t, stale[0].MarketValueValue, "stale is a warning, not a value gate")

	_, beforePrice := datedValuation(t, f, "2026-05-01")
	require.Len(t, beforePrice, 1)
	assert.Equal(t, "future_only", beforePrice[0].PriceStatus)
	assert.Nil(t, beforePrice[0].MarketValueValue)
	assert.Equal(t, "future_only", beforePrice[0].ValuationUnavailable)
	assert.Equal(t, "2", beforePrice[0].QuantityValue.String(), "quantity and basis remain without a price")
}

func TestDatedValuationRejectsFutureAndMalformedDates(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	for _, date := range []string{"2099-01-01", "2026-13-01", "yesterday"} {
		_, _, err := f.investmentService.UnrealizedGainsProjection(context.Background(), date)
		var validation ValidationError
		require.ErrorAs(t, err, &validation, date)
	}
}
