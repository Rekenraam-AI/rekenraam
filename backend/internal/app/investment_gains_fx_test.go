package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

func gainsFXCurrency(t *testing.T, f *investmentsTestFixture) int64 {
	t.Helper()
	f.transactionService.SetPricingRepository(db.NewPricingRepository(f.database))
	return seedTestCurrencyCommodity(t, f.database, "USD")
}
func gainsFXRate(t *testing.T, f *investmentsTestFixture, usd int64, date string, value int64) {
	t.Helper()
	pricing := NewPricingService(db.NewPricingRepository(f.database))
	_, err := pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID, BaseCommodityID: f.eurCommodityID, QuoteCommodityID: usd, QuoteType: "official_fixing", AdjustmentBasis: "raw", ValuationDate: date, PriceValue: value, PriceScale: 2})
	require.NoError(t, err)
}
func convertedGains(t *testing.T, f *investmentsTestFixture, date string, input GainsFXInput) GainsFXProjection {
	t.Helper()
	ctx := context.Background()
	profile, positions := datedValuation(t, f, date)
	realized, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
	require.NoError(t, err)
	converted, err := f.investmentService.ConvertGains(ctx, profile, GainsReportParams{}, realized, positions, input)
	require.NoError(t, err)
	return converted
}
func assertFXAmount(t *testing.T, value int64, scale int, figure GainsFXFigure) {
	t.Helper()
	require.NotNil(t, figure.Amount)
	requireScaled(t, value, scale, exact.ScaledIntFromCoefficient(figure.Amount.Value, figure.Amount.Scale), "reporting amount")
}

func TestGainsFXMethodsUseDistinctBasisAndProceedsDatesWithoutWriting(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	usd := gainsFXCurrency(t, f)
	buyOn(t, f, "2026-01-01", 2, 4000)
	sale := sellInput(f, "2026-03-01", 1)
	sale.CashAmountValue = 5000
	_, err := f.investmentService.Sell(context.Background(), sale)
	require.NoError(t, err)
	priceStockOn(t, f, "2026-03-31", 6000)
	gainsFXRate(t, f, usd, "2026-01-01", 50)
	gainsFXRate(t, f, usd, "2026-03-01", 80)
	gainsFXRate(t, f, usd, "2026-03-31", 120)
	before := buyReplacementPreviewSnapshot(t, f.database)
	for _, tc := range []struct {
		method               string
		realized, unrealized int64
	}{{GainsFXValuationDateAll, 3600, 4800}, {GainsFXHistoricBasis, 3000, 6200}} {
		t.Run(tc.method, func(t *testing.T) {
			got := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: usd, Method: tc.method})
			assert.Equal(t, tc.method, got.Profile.FXMethod)
			assert.Equal(t, 7, got.Profile.FXMaxAgeDays)
			assert.Equal(t, usd, got.Profile.ReportingCurrencyID)
			assertFXAmount(t, tc.realized, 2, got.RealizedTotal)
			assertFXAmount(t, tc.unrealized, 2, got.UnrealizedTotal)
			require.Len(t, got.Realized, 1)
			require.Len(t, got.Realized[0].Components, 2)
			for _, part := range got.Realized[0].Components {
				require.Positive(t, part.ObservationID)
				require.NotNil(t, part.Rate)
				require.NotNil(t, part.BaseQuantity)
				assert.Equal(t, part.RequestedDate, part.ObservationDate)
			}
			assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
		})
	}
	_, original := datedValuation(t, f, "2026-03-31")
	require.Len(t, original, 1)
	assertMoneyValue(t, 4000, 2, *original[0].UnrealizedGainValue, *original[0].UnrealizedGainScale, "native gain retained")
}

func TestGainsFXMissingStaleAndInclusiveWindowNeverSubstituteZero(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, date string
		available  bool
	}{{"missing", "", false}, {"stale_beyond_window", "2026-03-23", false}, {"seven_days_inclusive", "2026-03-24", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newInvestmentsTestFixture(t)
			usd := gainsFXCurrency(t, f)
			buyOn(t, f, "2026-01-01", 2, 4000)
			priceStockOn(t, f, "2026-03-31", 3000)
			if tc.date != "" {
				gainsFXRate(t, f, usd, tc.date, 120)
			}
			got := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: usd, Method: GainsFXValuationDateAll, MaxAgeDays: 7})
			if tc.available {
				assertFXAmount(t, 2400, 2, got.UnrealizedTotal)
			} else {
				assert.Nil(t, got.UnrealizedTotal.Amount)
				require.Len(t, got.Unrealized, 1)
				assert.Nil(t, got.Unrealized[0].Amount)
				assert.Equal(t, RateMissingNoObservation, got.Unrealized[0].Components[0].UnavailableReason)
				assert.Equal(t, tc.date, got.Unrealized[0].Components[0].NearestObservationDate)
			}
		})
	}
}

func TestGainsFXSameCurrencyIsExactNoOpIncludingSubCentAmounts(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	f.transactionService.SetPricingRepository(db.NewPricingRepository(f.database))
	buyOn(t, f, "2026-01-01", 1, 1000)
	sale := sellInput(f, "2026-03-01", 1)
	sale.CashAmountValue = 10001
	sale.CashAmountScale = 3
	_, err := f.investmentService.Sell(context.Background(), sale)
	require.NoError(t, err)
	for _, method := range []string{GainsFXValuationDateAll, GainsFXHistoricBasis} {
		got := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: f.eurCommodityID, Method: method})
		assertFXAmount(t, 1, 3, got.RealizedTotal)
		for _, c := range got.Realized[0].Components {
			assert.Zero(t, c.ObservationID)
			assert.Nil(t, c.Rate)
		}
	}
}

func TestGainsFXMixedCurrenciesDoNotExposePartialTotals(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	usd := gainsFXCurrency(t, f)
	buyOn(t, f, "2026-01-01", 1, 1000)
	dollars := tradeOn(f, f.holdingAccountID, "2026-01-01", 2, 4000)
	dollars.CashCommodityID = usd
	_, err := f.investmentService.Buy(context.Background(), dollars)
	require.NoError(t, err)
	got := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: usd, Method: GainsFXValuationDateAll})
	require.Len(t, got.Unrealized, 2)
	assert.Nil(t, got.Unrealized[0].Amount)
	require.NotNil(t, got.Unrealized[1].Amount)
	assert.Nil(t, got.UnrealizedTotal.Amount)
	gainsFXRate(t, f, usd, "2026-03-31", 150)
	complete := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: usd, Method: GainsFXValuationDateAll})
	assertFXAmount(t, 0, 2, complete.UnrealizedTotal)
}

func TestGainsFXStatementConvertsIncomeAndBothBoundaries(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	usd := gainsFXCurrency(t, f)
	buyOn(t, f, "2026-01-01", 2, 4000)
	priceStockOn(t, f, "2026-02-28", 2500)
	sale := sellInput(f, "2026-03-01", 1)
	sale.CashAmountValue = 5000
	_, err := f.investmentService.Sell(context.Background(), sale)
	require.NoError(t, err)
	priceStockOn(t, f, "2026-03-31", 6000)
	dividendOn(t, f, "2026-03-15", 300)
	for _, r := range []struct {
		date  string
		value int64
	}{{"2026-01-01", 50}, {"2026-02-28", 60}, {"2026-03-01", 80}, {"2026-03-15", 90}, {"2026-03-31", 120}} {
		gainsFXRate(t, f, usd, r.date, r.value)
	}
	statement := gainsStatement(t, f, "2026-03-01", "2026-03-31")
	before := buyReplacementPreviewSnapshot(t, f.database)
	for _, tc := range []struct {
		method                 string
		opening, income, total int64
	}{{GainsFXValuationDateAll, 600, 360, 8160}, {GainsFXHistoricBasis, 1000, 270, 8470}} {
		result, err := f.investmentService.ConvertGainsStatement(context.Background(), statement, GainsFXInput{CommodityID: usd, Method: tc.method})
		require.NoError(t, err)
		assertFXAmount(t, tc.opening, 2, result.Opening)
		assertFXAmount(t, tc.income, 2, result.Income)
		assertFXAmount(t, tc.total, 2, result.Total)
		assert.NotEmpty(t, result.Opening.Components)
		assert.Equal(t, statement.Profile.KnowledgeCutoff, result.Profile.KnowledgeCutoff)
	}
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestGainsFXShortOpeningProceedsKeepTheirPositiveSign(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	usd := gainsFXCurrency(t, f)
	shortSaleOn(t, f, "2026-01-01", 2, 4000)
	_, err := f.investmentService.ShortCover(context.Background(), shortInput(f, "2026-03-01", 1, 1000))
	require.NoError(t, err)
	priceStockOn(t, f, "2026-03-31", 1500)
	gainsFXRate(t, f, usd, "2026-01-01", 50)
	gainsFXRate(t, f, usd, "2026-03-01", 80)
	gainsFXRate(t, f, usd, "2026-03-31", 120)
	got := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: usd, Method: GainsFXHistoricBasis})
	assertFXAmount(t, 200, 2, got.RealizedTotal)    // 20×.5 − 10×.8
	assertFXAmount(t, -800, 2, got.UnrealizedTotal) // 20×.5 − 15×1.2
}

func TestGainsFXHistoricTransfersUseOriginalAcquisitionEvidence(t *testing.T) {
	t.Parallel()
	for _, knownDate := range []bool{true, false} {
		t.Run(map[bool]string{true: "known_original_date", false: "unknown_original_date"}[knownDate], func(t *testing.T) {
			f := newInvestmentsTestFixture(t)
			usd := gainsFXCurrency(t, f)
			seedExternalTransferEquity(t, f.database)
			input := ExternalTransferInInput{OwnerUserID: f.ownerUserID, EffectiveOn: "2026-03-01", HoldingAccountID: f.holdingAccountID, CommodityID: f.stockCommodityID, QuantityValue: exact.New(2), CarriedBasisValue: 4000, CarriedBasisScale: 2, CostCommodityID: f.eurCommodityID, SourceEvidenceJSON: `{"statement":"historic FX"}`}
			if knownDate {
				input.OriginalAcquiredOn = "2026-01-01"
			}
			_, err := f.investmentService.ExternalTransferIn(context.Background(), input)
			require.NoError(t, err)
			priceStockOn(t, f, "2026-03-31", 3000)
			gainsFXRate(t, f, usd, "2026-01-01", 50)
			gainsFXRate(t, f, usd, "2026-03-31", 120)
			got := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: usd, Method: GainsFXHistoricBasis})
			if knownDate {
				assertFXAmount(t, 5200, 2, got.UnrealizedTotal)
				assert.Equal(t, "2026-01-01", got.Unrealized[0].Components[1].RequestedDate)
			} else {
				assert.Nil(t, got.UnrealizedTotal.Amount)
				assert.Equal(t, "unknown_acquisition_date", got.Unrealized[0].Components[1].UnavailableReason)
			}
			native := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: f.eurCommodityID, Method: GainsFXHistoricBasis})
			assertFXAmount(t, 2000, 2, native.UnrealizedTotal)
		})
	}
}

func TestGainsFXUnknownBasisAndUnpricedBoundariesStayAbsent(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixtureWithOptions(t, false)
	usd := gainsFXCurrency(t, f)
	seedExternalTransferEquity(t, f.database)
	_, err := f.investmentService.ExternalTransferIn(context.Background(), unknownTransferInInput(f, "2026-02-01", ""))
	require.NoError(t, err)
	gainsFXRate(t, f, usd, "2026-03-31", 120)
	for _, method := range []string{GainsFXValuationDateAll, GainsFXHistoricBasis} {
		got := convertedGains(t, f, "2026-03-31", GainsFXInput{CommodityID: usd, Method: method})
		assert.Nil(t, got.UnrealizedTotal.Amount)
		assert.Equal(t, "unpriced", got.Unrealized[0].Components[0].UnavailableReason)
		assert.Equal(t, "unresolved_basis", got.Unrealized[0].Components[1].UnavailableReason)
	}
}

func TestGainsFXRejectsInvalidMethodsCurrenciesAndAgeWindows(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	usd := gainsFXCurrency(t, f)
	profile, positions := datedValuation(t, f, "2026-03-31")
	for _, input := range []GainsFXInput{{CommodityID: usd, Method: ""}, {CommodityID: usd, Method: "unknown"}, {CommodityID: 0, Method: GainsFXValuationDateAll}, {CommodityID: f.stockCommodityID, Method: GainsFXValuationDateAll}, {CommodityID: 999999, Method: GainsFXValuationDateAll}, {CommodityID: usd, Method: GainsFXValuationDateAll, MaxAgeDays: -1}, {CommodityID: usd, Method: GainsFXValuationDateAll, MaxAgeDays: 366}} {
		_, err := f.investmentService.ConvertGains(context.Background(), profile, GainsReportParams{}, nil, positions, input)
		var validation ValidationError
		require.ErrorAs(t, err, &validation)
	}
}

func TestGainsFXUsesSourceKnowledgeCutoffAndCompatibleQuotes(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	usd := gainsFXCurrency(t, f)
	buyOn(t, f, "2026-01-01", 1, 1000)
	early := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	pricing := NewPricingService(db.NewPricingRepository(f.database))
	pricing.SetNowForTest(func() time.Time { return early })
	_, err := pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID, BaseCommodityID: f.stockCommodityID, QuoteCommodityID: f.eurCommodityID, QuoteType: "close", AdjustmentBasis: "raw", ValuationDate: "2026-03-31", PriceValue: 2000, PriceScale: 2})
	require.NoError(t, err)
	_, err = pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID, BaseCommodityID: f.eurCommodityID, QuoteCommodityID: usd, QuoteType: "official_fixing", AdjustmentBasis: "raw", ValuationDate: "2026-03-31", PriceValue: 80, PriceScale: 2})
	require.NoError(t, err)
	f.investmentService.SetNowForTest(func() time.Time { return early })
	profile, positions := datedValuation(t, f, "2026-03-31")
	// An incompatible manual quote known then and a compatible manual quote
	// recorded later cannot displace the source profile's eligible observation.
	_, err = pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID, BaseCommodityID: f.eurCommodityID, QuoteCommodityID: usd, QuoteType: "manual", AdjustmentBasis: "total_return", IsManual: true, ValuationDate: "2026-03-31", PriceValue: 150, PriceScale: 2})
	require.NoError(t, err)
	pricing.SetNowForTest(func() time.Time { return early.Add(time.Second) })
	_, err = pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: f.ownerUserID, BaseCommodityID: f.eurCommodityID, QuoteCommodityID: usd, QuoteType: "manual", AdjustmentBasis: "raw", IsManual: true, ValuationDate: "2026-03-31", PriceValue: 200, PriceScale: 2})
	require.NoError(t, err)
	got, err := f.investmentService.ConvertGains(context.Background(), profile, GainsReportParams{From: "2026-03-01", To: "2026-03-31"}, nil, positions, GainsFXInput{CommodityID: usd, Method: GainsFXValuationDateAll})
	require.NoError(t, err)
	assertFXAmount(t, 800, 2, got.UnrealizedTotal)
	assert.Equal(t, profile.KnowledgeCutoff, got.Profile.KnowledgeCutoff)
	assert.Equal(t, "2026-03-01", got.Profile.From)
	assert.Equal(t, "2026-03-31", got.Profile.To)
	assert.Equal(t, exact.New(80), got.Unrealized[0].Components[0].Rate.Value)
}
