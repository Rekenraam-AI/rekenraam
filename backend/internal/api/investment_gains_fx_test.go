package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestInvestmentGainsReportingCurrencyAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	currency := createCurrencyForSession(t, handler, f.sessionCookie, f.csrfToken, `{"code":"CHF"}`)
	instrument := createInstrumentForSession(t, handler, f, "GAINFX")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 5000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)
	base := "/api/v1/investments/gains?valuation_as_of=2026-03-31&reporting_currency_id=" + strconv.FormatInt(currency.ID, 10)
	read := func(query string) investmentGainsResponse {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+query, nil, http.StatusOK)
		var body investmentGainsResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
		require.NotNil(t, body.Reporting)
		return body
	}
	missing := read("&fx_method=valuation_date_all")
	assert.Nil(t, missing.Reporting.RealizedTotal.Amount)
	assert.Nil(t, missing.Reporting.UnrealizedTotal.Amount)
	require.Len(t, missing.Realized, 1)
	require.NotNil(t, missing.Realized[0].RealizedGainValue)
	assert.Equal(t, "none", missing.Profile.FXMethod)
	for _, rate := range []struct {
		date  string
		value int64
	}{{"2026-02-01", 50}, {"2026-02-28", 60}, {"2026-03-01", 80}, {"2026-03-24", 120}} {
		doPricingRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/prices", priceObservationRequest{BaseCommodityID: f.commodityID, QuoteCommodityID: currency.ID, QuoteType: "official_fixing", AdjustmentBasis: "raw", Derivation: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`), SeriesMetadata: json.RawMessage(`{}`), PriceValue: rate.value, PriceScale: 2, ValuationDate: rate.date}, http.StatusCreated)
	}
	for _, tc := range []struct {
		method   string
		realized int64
	}{{"valuation_date_all", 3600}, {"historic_basis_valuation_close", 3000}} {
		body := read("&fx_method=" + tc.method + "&fx_max_age_days=7")
		assert.Equal(t, tc.method, body.Reporting.Profile.FXMethod)
		assert.Equal(t, 7, body.Reporting.Profile.FXMaxAgeDays)
		amount := body.Reporting.RealizedTotal.Amount
		require.NotNil(t, amount)
		assert.Zero(t, exact.ScaledIntFromCoefficient(amount.Value, amount.Scale).Cmp(exact.ScaledIntFromInt64(tc.realized, 2)))
		require.Positive(t, body.Reporting.Realized[0].Components[0].ObservationID)
		require.NotNil(t, body.Reporting.Realized[0].Components[0].Rate)
	}
	stale := read("&fx_method=valuation_date_all&fx_max_age_days=6")
	assert.Nil(t, stale.Reporting.RealizedTotal.Amount)
	assert.Equal(t, "2026-03-24", stale.Reporting.Realized[0].Components[0].NearestObservationDate)
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains/statement?from=2026-03-01&to=2026-03-31&reporting_currency_id="+strconv.FormatInt(currency.ID, 10)+"&fx_method=historic_basis_valuation_close", nil, http.StatusOK)
	var statement investmentGainsStatementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&statement))
	require.NotNil(t, statement.Reporting)
	require.NotNil(t, statement.Reporting.Total.Amount)
	require.NotEmpty(t, statement.Rows)
	// Omitted options keep the original wire shape; same currency needs no rate.
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains", nil, http.StatusOK)
	var original map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(res.Body).Decode(&original))
	assert.NotContains(t, original, "reporting")
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?reporting_currency_id="+strconv.FormatInt(f.commodityID, 10)+"&fx_method=valuation_date_all", nil, http.StatusOK)
	var same investmentGainsResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&same))
	require.NotNil(t, same.Reporting.RealizedTotal.Amount)
}

func TestInvestmentGainsFXInvalidOptionsAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	id := strconv.FormatInt(f.commodityID, 10)
	for _, query := range []string{"fx_method=valuation_date_all", "fx_max_age_days=7", "reporting_currency_id=" + id, "reporting_currency_id=" + id + "&fx_method=bogus", "reporting_currency_id=0&fx_method=valuation_date_all", "reporting_currency_id=" + id + "&fx_method=valuation_date_all&fx_max_age_days=0", "reporting_currency_id=" + id + "&fx_method=valuation_date_all&fx_max_age_days=366", "reporting_currency_id=" + id + "&fx_method=valuation_date_all&fx_max_age_days=1.5", "reporting_currency_id=" + id + "&fx_method=valuation_date_all&fx_method=valuation_date_all", "reporting_currency_id=" + id + "&fx_method="} {
		for _, path := range []string{"/api/v1/investments/gains?", "/api/v1/investments/gains/statement?from=2026-03-01&to=2026-03-31&"} {
			res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path+query, nil, http.StatusBadRequest)
			var body errorResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			assert.Equal(t, "VALIDATION_FAILED", body.Error.Code)
		}
	}
}

func TestInvestmentGainsFXPrecisionOverflowIs422(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	currency := createCurrencyForSession(t, handler, f.sessionCookie, f.csrfToken, `{"code":"CHF"}`)
	instrument := createInstrumentForSession(t, handler, f, "FXRANGE")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 9000000000000000000), http.StatusCreated)
	doPricingRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/prices", priceObservationRequest{
		BaseCommodityID: f.commodityID, QuoteCommodityID: currency.ID, QuoteType: "official_fixing", AdjustmentBasis: "raw", PriceValue: 9000000000000000000, PriceScale: 0, BaseQuantityValue: 1, BaseQuantityScale: 12, ValuationDate: "2026-03-31", Derivation: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`), SeriesMetadata: json.RawMessage(`{}`),
	}, http.StatusCreated)
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?valuation_as_of=2026-03-31&reporting_currency_id="+strconv.FormatInt(currency.ID, 10)+"&fx_method=valuation_date_all", nil, http.StatusUnprocessableEntity)
	var body errorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "LEDGER_OVERFLOW", body.Error.Code)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains", nil, http.StatusOK)
}
