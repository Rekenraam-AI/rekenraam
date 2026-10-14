package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

func TestObservationSelectionThroughHoldingsAndLatestPrices(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, quote, basis, date string
		want                     bool
	}{
		{"raw_close", "close", "raw", "2026-06-05", true},
		{"future_only", "close", "raw", "2099-06-05", false},
		{"adjusted_close", "adjusted_close", "split_adjusted", "2026-06-05", false},
		{"raw_bid", "bid", "raw", "2026-06-05", false},
		{"adjusted_manual", "manual", "total_return", "2026-06-05", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixtureWithOptions(t, false)
			buyOn(t, f, "2026-01-01", 2, 20000)
			pricing := NewPricingService(db.NewPricingRepository(f.database))
			_, err := pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: 1, BaseCommodityID: f.stockCommodityID, QuoteCommodityID: f.eurCommodityID, QuoteType: tc.quote, AdjustmentBasis: tc.basis, ValuationDate: tc.date, PriceValue: 12300, PriceScale: 2})
			require.NoError(t, err)
			gains, err := f.investmentService.ListUnrealizedGains(context.Background())
			require.NoError(t, err)
			require.Len(t, gains, 1)
			latest, err := pricing.LatestPricesForPairs(context.Background(), [][2]int64{{f.stockCommodityID, f.eurCommodityID}})
			require.NoError(t, err)
			if tc.want {
				require.NotNil(t, gains[0].MarketValueValue)
				require.Len(t, latest, 1)
			} else {
				require.Nil(t, gains[0].MarketValueValue)
				require.Empty(t, latest)
			}
		})
	}
}

func TestObservationSelectionFXManualBeatsLaterProviderAndAdjusted(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixtureWithOptions(t, false)
	ctx := context.Background()
	pricing := NewPricingService(db.NewPricingRepository(f.database))
	var usdID int64
	res, err := f.database.Exec(`INSERT INTO commodities(book_id,code,kind,is_builtin,created_at,created_by_user_id) VALUES(1,'USD','currency',1,'2026-01-01T00:00:00Z',1)`)
	require.NoError(t, err)
	usdID, err = res.LastInsertId()
	require.NoError(t, err)
	_, err = f.database.Exec(`INSERT INTO commodity_versions(commodity_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,symbol,display_symbol,name,standard_scale,max_quantity_scale) VALUES(?,1,'2026-01-01','2026-01-01T00:00:00Z',1,'fixture','active','USD','$','Dollar',2,6)`, usdID)
	require.NoError(t, err)
	create := func(quote, basis string, manual bool, value int64, recorded string) {
		stamp, err := time.Parse(time.RFC3339, recorded)
		require.NoError(t, err)
		pricing.SetNowForTest(func() time.Time { return stamp })
		_, err = pricing.CreatePrice(ctx, PriceObservationInput{OwnerUserID: 1, BaseCommodityID: usdID, QuoteCommodityID: f.eurCommodityID, QuoteType: quote, AdjustmentBasis: basis, IsManual: manual, ValuationDate: "2026-06-05", PriceValue: value, PriceScale: 2})
		require.NoError(t, err)
	}
	create("manual", "raw", true, 80, "2026-06-05T10:00:00Z")
	create("official_fixing", "not_applicable", false, 90, "2026-06-05T11:00:00Z")
	create("adjusted_close", "total_return", false, 99, "2026-06-05T12:00:00Z")
	f.transactionService.SetPricingRepository(db.NewPricingRepository(f.database))
	table, err := f.transactionService.NewRateTable(ctx, ReportingCurrencyInput{CommodityID: f.eurCommodityID}, []int64{usdID}, "2026-06-05", "2026-06-05")
	require.NoError(t, err)
	rate, found := table.rateFor(usdID, "2026-06-05")
	require.True(t, found)
	require.Equal(t, int64(80), rate.PriceValue)
}

func TestObservationSelectionPriorityKnowledgeAndAgeThroughConsumers(t *testing.T) {
	t.Parallel()
	type quote struct {
		kind, basis, date, recorded string
		value                       int64
		manual, approx              bool
	}
	for _, tc := range []struct {
		name   string
		quotes []quote
		want   int64
		status string
	}{
		{"manual_before_later_provider", []quote{{kind: "manual", value: 80, manual: true}, {kind: "close", value: 90}}, 80, "usable"},
		{"override_before_later_provider", []quote{{kind: "valuation_override", value: 81}, {kind: "close", value: 90}}, 81, "usable"},
		{"exact_before_later_approximate", []quote{{kind: "close", value: 82}, {kind: "trade_implied", value: 90, approx: true}}, 82, "usable"},
		{"unadjusted_trade_fill_marker", []quote{{kind: "trade_implied", basis: "not_applicable", value: 83, approx: true}}, 83, "usable"},
		{"approximate_fallback", []quote{{kind: "trade_implied", value: 83, approx: true}}, 83, "usable"},
		{"recording_time_breaks_equal_rank", []quote{{kind: "close", value: 80}, {kind: "close", value: 84}}, 84, "usable"},
		{"id_breaks_equal_time", []quote{{kind: "close", value: 80, recorded: "2026-06-05T10:00:00Z"}, {kind: "close", value: 85, recorded: "2026-06-05T10:00:00Z"}}, 85, "usable"},
		{"newer_date_before_manual_priority", []quote{{kind: "manual", manual: true, value: 80, date: "2026-06-04"}, {kind: "close", value: 86}}, 86, "usable"},
		{"adjusted_manual_cannot_displace_raw", []quote{{kind: "close", value: 87}, {kind: "manual", manual: true, basis: "split_adjusted", value: 90}}, 87, "usable"},
		{"recorded_after_cutoff_cannot_displace_known", []quote{{kind: "close", value: 88}, {kind: "manual", manual: true, value: 90, recorded: "2026-06-06T00:00:00Z"}}, 88, "usable"},
		{"future_only", []quote{{kind: "close", value: 90, date: "2026-06-06"}}, 0, "future_only"},
		{"unknown_at_cutoff", []quote{{kind: "close", value: 90, recorded: "2026-06-06T00:00:00Z"}}, 0, "not_known_at_cutoff"},
		{"incompatible_basis_only", []quote{{kind: "manual", manual: true, basis: "total_return", value: 90}}, 0, "incompatible_quote_basis"},
		{"seven_day_age_boundary_is_inclusive", []quote{{kind: "close", value: 89, date: "2026-05-29"}}, 89, "usable"},
		{"eight_day_age_reports_refuse", []quote{{kind: "close", value: 89, date: "2026-05-28"}}, 89, "stale"},
		{"holdings_retain_labelled_stale_reports_refuse", []quote{{kind: "close", value: 89, date: "2026-05-01"}}, 89, "stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixtureWithOptions(t, false)
			ctx := context.Background()
			buyOn(t, f, "2026-01-01", 2, 20000)
			pricing := NewPricingService(db.NewPricingRepository(f.database))
			for i, q := range tc.quotes {
				date := q.date
				if date == "" {
					date = "2026-06-05"
				}
				recorded := q.recorded
				if recorded == "" {
					recorded = time.Date(2026, 6, 5, 9+i, 0, 0, 0, time.UTC).Format(time.RFC3339)
				}
				stamp, err := time.Parse(time.RFC3339, recorded)
				require.NoError(t, err)
				pricing.SetNowForTest(func() time.Time { return stamp })
				price, err := pricing.CreatePrice(ctx, PriceObservationInput{OwnerUserID: 1, BaseCommodityID: f.stockCommodityID, QuoteCommodityID: f.eurCommodityID, QuoteType: q.kind, AdjustmentBasis: q.basis, IsManual: q.manual, ValuationDate: date, PriceValue: q.value, PriceScale: 0})
				require.NoError(t, err)
				if q.approx {
					_, err = f.database.Exec(`UPDATE price_observations SET is_approximate=1 WHERE id=?`, price.ID)
					require.NoError(t, err)
				}
			}
			now := time.Date(2026, 6, 5, 23, 0, 0, 0, time.UTC)
			f.investmentService.SetNowForTest(func() time.Time { return now })
			pricing.SetNowForTest(func() time.Time { return now })
			f.transactionService.now = func() time.Time { return now }
			f.transactionService.SetPricingRepository(db.NewPricingRepository(f.database))
			positions, err := f.investmentService.Positions(ctx)
			require.NoError(t, err)
			require.Len(t, positions, 1)
			require.Equal(t, tc.status, positions[0].PriceStatus)
			require.Equal(t, "2026-06-05", positions[0].ValuationAsOf)
			gains, err := f.investmentService.ListUnrealizedGains(ctx)
			require.NoError(t, err)
			require.Len(t, gains, 1)
			latest, err := pricing.LatestPricesForPairs(ctx, [][2]int64{{f.stockCommodityID, f.eurCommodityID}})
			require.NoError(t, err)
			table, err := f.transactionService.NewRateTable(ctx, ReportingCurrencyInput{CommodityID: f.eurCommodityID}, []int64{f.stockCommodityID}, "2026-06-05", "2026-06-05")
			require.NoError(t, err)
			converted, ok := table.Convert("2", 0, f.stockCommodityID, "2026-06-05")
			if tc.want == 0 {
				require.Nil(t, gains[0].MarketValueValue)
				require.Equal(t, tc.status, gains[0].ValuationUnavailable)
				require.Empty(t, latest)
				require.False(t, ok)
				require.Equal(t, tc.status, table.Coverage("EUR").Gaps[0].Reason)
			} else {
				require.Equal(t, tc.want, *positions[0].LatestPriceValue)
				require.Equal(t, tc.want, latest[0].PriceValue)
				require.Equal(t, 0, scaledFromPointer(t, gains[0].MarketValueValue, gains[0].MarketValueScale).Cmp(exact.ScaledIntFromInt64(tc.want*2, 0)))
				if tc.status == "stale" {
					require.False(t, ok)
					require.Equal(t, tc.quotes[0].date, table.Coverage("EUR").Gaps[0].NearestObservationDate)
				} else {
					require.True(t, ok)
					require.Equal(t, tc.want*200, converted.Int64())
				}
			}
			var count int
			require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM price_observations`).Scan(&count))
			require.Equal(t, len(tc.quotes), count, "reads preserve original evidence")
		})
	}
}

func TestObservationSelectionOwnerLocalDateAndPreciseKnowledgeBoundary(t *testing.T) {
	t.Parallel()
	for _, zone := range []string{"Europe/Amsterdam", "America/Los_Angeles"} {
		t.Run(zone, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixtureWithOptions(t, false)
			buyOn(t, f, "2026-01-01", 2, 20000)
			_, err := f.database.Exec(`INSERT INTO user_preferences(user_id,time_zone,created_at,created_by_user_id,updated_at,updated_by_user_id) VALUES(1,?,'2026-01-01T00:00:00Z',1,'2026-01-01T00:00:00Z',1) ON CONFLICT(user_id) DO UPDATE SET time_zone=excluded.time_zone`, zone)
			require.NoError(t, err)
			pricing := NewPricingService(db.NewPricingRepository(f.database))
			pricing.SetNowForTest(func() time.Time { return time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC) })
			_, err = pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: 1, BaseCommodityID: f.stockCommodityID, QuoteCommodityID: f.eurCommodityID, QuoteType: "close", ValuationDate: "2026-06-06", PriceValue: 100, PriceScale: 2})
			require.NoError(t, err)
			pricing.SetNowForTest(func() time.Time { return time.Date(2026, 6, 5, 22, 30, 1, 0, time.UTC) })
			_, err = pricing.CreatePrice(context.Background(), PriceObservationInput{OwnerUserID: 1, BaseCommodityID: f.stockCommodityID, QuoteCommodityID: f.eurCommodityID, QuoteType: "manual", IsManual: true, ValuationDate: "2026-06-05", PriceValue: 999, PriceScale: 2})
			require.NoError(t, err)
			f.investmentService.SetNowForTest(func() time.Time { return time.Date(2026, 6, 5, 22, 30, 0, 999900000, time.UTC) })
			positions, err := f.investmentService.Positions(context.Background())
			require.NoError(t, err)
			require.Len(t, positions, 1)
			if zone == "Europe/Amsterdam" {
				require.Equal(t, "2026-06-06", positions[0].ValuationAsOf)
				require.Equal(t, int64(100), *positions[0].LatestPriceValue)
			} else {
				require.Equal(t, "2026-06-05", positions[0].ValuationAsOf)
				require.Nil(t, positions[0].LatestPriceValue, "a future second must not round into the knowledge boundary")
			}
		})
	}
}
