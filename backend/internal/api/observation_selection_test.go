package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObservationSelectionAPIHoldingsAndReportingFX(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, quote, basis, date, reason string
		manual, approx                   bool
		want                             string
	}{
		{name: "manual_over_later_provider", quote: "official_fixing", basis: "not_applicable", want: "8000"},
		{name: "exact_over_later_approximate", quote: "trade_implied", basis: "raw", approx: true, want: "8000"},
		{name: "raw_over_adjusted_manual", quote: "manual", basis: "total_return", manual: true, want: "8000"},
		{name: "future_only", quote: "manual", basis: "raw", date: "2099-01-01", reason: "future_only"},
		{name: "adjusted_only", quote: "adjusted_close", basis: "split_adjusted", reason: "incompatible_quote_basis"},
		{name: "recorded_after_cutoff", quote: "manual", basis: "raw", reason: "not_known_at_cutoff"},
		{name: "stale_only", quote: "manual", basis: "raw", date: "2026-01-01", reason: "no_observation_in_window"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler, database := newSetupTestHandler(t)
			f := newReportingFixture(t, handler)
			inv := investmentAPITestFixture{sessionCookie: f.sessionCookie, csrfToken: f.csrfToken, commodityID: f.usdID, cashAccount: f.checking}
			instrument := createInstrumentForSession(t, handler, inv, "POLICY")
			holding := createHoldingAccountForSession(t, handler, inv, instrument.ID)
			doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", map[string]any{
				"effective_on": "2026-01-01", "holding_account_id": holding.ID, "commodity_id": instrument.CommodityID,
				"quantity_value": "2", "quantity_scale": 0, "carried_basis_value": "10000", "carried_basis_scale": 2,
				"cost_commodity_id": f.eurID, "original_acquired_on": "2026-01-01", "source_evidence": map[string]any{"statement": "selection matrix"},
			}, http.StatusCreated)
			create := func(base int64, quote, basis, date string, value int64, manual, approx bool) {
				res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/prices", map[string]any{
					"base_commodity_id": base, "quote_commodity_id": f.eurID, "quote_type": quote, "adjustment_basis": basis,
					"valuation_date": date, "price_value": value, "price_scale": 2, "is_manual": manual,
				}, http.StatusCreated)
				var p priceObservationResponse
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &p))
				if approx {
					_, err := database.Exec(`UPDATE price_observations SET is_approximate=1 WHERE id=?`, p.ID)
					require.NoError(t, err)
				}
				if tc.name == "recorded_after_cutoff" {
					_, err := database.Exec(`UPDATE price_observations SET recorded_at='2099-01-01T00:00:00Z' WHERE id=?`, p.ID)
					require.NoError(t, err)
				}
			}
			for _, base := range []int64{f.usdID, instrument.CommodityID} {
				if tc.want != "" {
					create(base, "manual", "raw", "2026-06-05", 80, tc.name != "exact_over_later_approximate", false)
				}
				kind := tc.quote
				if base == instrument.CommodityID && kind == "official_fixing" {
					kind = "close"
				}
				date := tc.date
				if date == "" {
					date = "2026-06-05"
				}
				create(base, kind, tc.basis, date, 90, tc.manual, tc.approx)
			}
			createTransactionForSession(t, handler, f.sessionCookie, f.csrfToken, balancedBody("2026-06-05", posting(f.checking.ID, -10000, 2, f.usdID), posting(f.groceries.ID, 10000, 2, f.usdID)), http.StatusCreated)
			report := readSpending(t, handler, f.sessionCookie, "?start_date=2026-06-05&end_date=2026-06-05&group_by=category&reporting_currency_id="+strconvFormatInt(f.eurID))
			require.Len(t, report.CommodityTotals, 1)
			require.Equal(t, "10000", report.CommodityTotals[0].QuantityValue.String())
			require.NotNil(t, report.Valuation)
			require.Equal(t, "live_raw_v1", report.Valuation.SelectionPolicy)
			require.NotEmpty(t, report.Valuation.KnowledgeCutoff)
			res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains", nil, http.StatusOK)
			var gains investmentGainsResponse
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &gains))
			require.Len(t, gains.Unrealized, 1)
			pos := gains.Unrealized[0]
			require.Equal(t, "2", pos.QuantityValue.String())
			require.NotEmpty(t, pos.KnowledgeCutoff)
			if tc.want != "" {
				require.Equal(t, tc.want, report.ConvertedTotal.QuantityValue.String())
				require.Equal(t, moneyCoefficient(80), *pos.LatestPriceValue)
				require.Equal(t, moneyCoefficient(160), *pos.MarketValueValue)
			} else {
				require.Nil(t, report.ConvertedTotal)
				require.Len(t, report.Valuation.Gaps, 1)
				require.Equal(t, tc.reason, report.Valuation.Gaps[0].Reason)
				if tc.name == "stale_only" {
					require.Equal(t, "stale", pos.PriceStatus)
					require.NotNil(t, pos.MarketValueValue)
				} else {
					require.Equal(t, tc.reason, pos.ValuationUnavailable)
					require.Nil(t, pos.MarketValueValue)
				}
			}
		})
	}
}

func TestReportingCoverageRetainsEachSelectedObservationAndApproximation(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := newReportingFixture(t, handler)
	for _, date := range []string{"2026-06-01", "2026-06-02"} {
		recordRate(t, handler, f, date, 80, 2)
		createTransactionForSession(t, handler, f.sessionCookie, f.csrfToken, balancedBody(date, posting(f.checking.ID, -10000, 2, f.usdID), posting(f.groceries.ID, 10000, 2, f.usdID)), http.StatusCreated)
	}
	_, err := database.Exec(`UPDATE price_observations SET is_approximate=1 WHERE valuation_date='2026-06-02'`)
	require.NoError(t, err)
	report := readSpending(t, handler, f.sessionCookie, "?start_date=2026-06-01&end_date=2026-06-02&group_by=category&reporting_currency_id="+strconvFormatInt(f.eurID))
	require.NotNil(t, report.ConvertedTotal)
	require.Equal(t, "16000", report.ConvertedTotal.QuantityValue.String())
	require.Len(t, report.Valuation.RatesUsed, 2)
	require.False(t, report.Valuation.RatesUsed[0].Approximate)
	require.True(t, report.Valuation.RatesUsed[1].Approximate)
	require.NotEqual(t, report.Valuation.RatesUsed[0].ObservationID, report.Valuation.RatesUsed[1].ObservationID)
}
