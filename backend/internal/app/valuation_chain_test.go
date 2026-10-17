package app

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

// TestReportingRateChainsASecurityThroughItsQuoteCurrency covers #281: a
// security priced only in USD converts into a EUR report by chaining its USD
// price with USD's EUR rate, each leg under the on-or-before and staleness
// rules, rounded once, and never when the second leg is missing.
func TestReportingRateChainsASecurityThroughItsQuoteCurrency(t *testing.T) {
	t.Parallel()
	const asOf = "2026-03-31"
	for _, tc := range []struct {
		name          string
		directEUR     bool
		usdRateDate   string // "" means no USD→EUR rate at all
		want          *big.Int
		wantStale     bool
		wantGapReason string
		wantNearest   string
		wantVia       bool
	}{
		{name: "chained", usdRateDate: asOf, want: big.NewInt(80325)},
		{name: "stale_second_leg", usdRateDate: "2026-03-27", want: big.NewInt(80325), wantStale: true},
		{name: "second_leg_outside_window", usdRateDate: "2026-03-01", wantGapReason: RateMissingNoObservation, wantNearest: "2026-03-01", wantVia: true},
		{name: "second_leg_missing", wantGapReason: RateMissingNoObservation, wantVia: true},
		{name: "direct_rate_wins", directEUR: true, usdRateDate: asOf, want: big.NewInt(81000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixtureWithOptions(t, false)
			ctx := context.Background()
			pricing := NewPricingService(db.NewPricingRepository(f.database))
			res, err := f.database.Exec(`INSERT INTO commodities(book_id,code,kind,is_builtin,created_at,created_by_user_id) VALUES(1,'USD','currency',1,'2026-01-01T00:00:00Z',1)`)
			require.NoError(t, err)
			usdID, err := res.LastInsertId()
			require.NoError(t, err)
			_, err = f.database.Exec(`INSERT INTO commodity_versions(commodity_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,symbol,display_symbol,name,standard_scale,max_quantity_scale) VALUES(?,1,'2026-01-01','2026-01-01T00:00:00Z',1,'fixture','active','USD','$','Dollar',2,6)`, usdID)
			require.NoError(t, err)
			price := func(base, quote int64, date string, value int64, scale int) {
				_, err := pricing.CreatePrice(ctx, PriceObservationInput{OwnerUserID: 1, BaseCommodityID: base, QuoteCommodityID: quote, QuoteType: "manual", AdjustmentBasis: "raw", IsManual: true, ValuationDate: date, PriceValue: value, PriceScale: scale})
				require.NoError(t, err)
			}
			price(f.stockCommodityID, usdID, asOf, 7000, 2) // 70.00 USD
			if tc.usdRateDate != "" {
				price(usdID, f.eurCommodityID, tc.usdRateDate, 85, 2) // 0.85 EUR
			}
			if tc.directEUR {
				price(f.stockCommodityID, f.eurCommodityID, asOf, 6000, 2) // 60.00 EUR
			}
			f.transactionService.SetPricingRepository(db.NewPricingRepository(f.database))

			table, err := f.transactionService.NewRateTable(ctx, ReportingCurrencyInput{CommodityID: f.eurCommodityID}, []int64{f.stockCommodityID}, asOf, asOf)
			require.NoError(t, err)
			// 13.5 units: 13.5 × 70 USD × 0.85 = 803.25 EUR; direct 13.5 × 60 = 810.00.
			converted, ok := table.Convert("135", 1, f.stockCommodityID, asOf)
			coverage := table.Coverage("EUR")

			if tc.want == nil {
				require.False(t, ok, "a missing second leg must not fabricate a rate")
				require.False(t, coverage.Complete)
				require.Empty(t, coverage.Used)
				require.Len(t, coverage.Gaps, 1)
				gap := coverage.Gaps[0]
				require.Equal(t, f.stockCommodityID, gap.CommodityID)
				require.Equal(t, tc.wantGapReason, gap.Reason)
				require.Equal(t, tc.wantNearest, gap.NearestObservationDate)
				if tc.wantVia {
					require.Equal(t, usdID, gap.ViaCommodityID, "the gap names the missing leg")
				}
				return
			}

			require.True(t, ok)
			require.Zero(t, tc.want.Cmp(converted), "got %s", converted)
			require.True(t, coverage.Complete)
			require.Len(t, coverage.Used, 1)
			use := coverage.Used[0]
			require.Equal(t, f.stockCommodityID, use.CommodityID)
			require.Equal(t, asOf, use.ObservationDate)
			require.Equal(t, tc.wantStale, use.Stale)
			if tc.directEUR {
				require.False(t, use.Derived)
				require.Zero(t, use.ViaCommodityID)
				return
			}
			require.True(t, use.Derived, "a chained rate is flagged derived")
			require.Equal(t, usdID, use.ViaCommodityID)
			require.NotZero(t, use.ViaObservationID)
			require.Equal(t, tc.usdRateDate, use.ViaObservationDate)
		})
	}
}
