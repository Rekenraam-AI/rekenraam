package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestInvestmentGainsAlternativeMethodAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ALTERNATIVE")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 2000), http.StatusCreated)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 6000)
	buy.TransactionDate = "2026-02-02"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 4000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)
	for _, method := range []string{"fifo", "lifo", "average_cost"} {
		response := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?alternative_method="+method+"&from=2026-03-01&to=2026-03-31&valuation_as_of=2026-03-02", nil, http.StatusOK)
		var body investmentGainsResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
		assert.Equal(t, "operational_v1", body.Profile.Name)
		require.NotNil(t, body.Alternative)
		alt := body.Alternative
		assert.Equal(t, "alternative_method_v1", alt.Profile.Name)
		assert.Equal(t, body.Profile.KnowledgeCutoff, alt.Profile.KnowledgeCutoff)
		assert.Equal(t, body.Profile.ValuationAsOf, alt.Profile.ValuationAsOf)
		assert.Equal(t, "recorded_entitlements", alt.Profile.StructuralActionPolicy)
		assert.Equal(t, method, alt.Profile.BasisMethod)
		assert.Equal(t, "selected_method", alt.Profile.SpecificLotFallback)
		assert.Equal(t, "2026-03-01", alt.Profile.From)
		assert.Equal(t, "2026-03-02", alt.Profile.ValuationAsOf)
		require.Len(t, alt.Realized, 1)
		require.Len(t, alt.Unrealized, 1)
		require.Len(t, alt.RealizedTotals, 1)
		want := map[string]int64{"fifo": 3000, "lifo": 1000, "average_cost": 2000}[method]
		require.NotNil(t, alt.Realized[0].RealizedGainValue)
		value := exact.New(int64(*alt.Realized[0].RealizedGainValue))
		requireScaledAPIAmount(t, want, 2, &value, alt.Realized[0].RealizedGainScale)
		value = exact.New(int64(*body.Realized[0].RealizedGainValue))
		requireScaledAPIAmount(t, 3000, 2, &value, body.Realized[0].RealizedGainScale)
	}
	for _, query := range []string{"alternative_method=specific_lot", "alternative_method=FIFO", "alternative_method=fifo&from=bad", "alternative_method=fifo&from=2026-03-01&to=2026-01-01", "alternative_method=fifo&valuation_as_of=2099-01-01"} {
		response := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?"+query, nil, http.StatusBadRequest)
		var body errorResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
		assert.Equal(t, "VALIDATION_FAILED", body.Error.Code)
	}
	response := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains", nil, http.StatusOK)
	var body investmentGainsResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
	assert.Nil(t, body.Alternative)
}

func TestAlternativeGainsOverflowIs422AndOperationalProjectionSurvives(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ALTRANGE")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	trade := func(side, date string, amount int64) {
		input := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", amount)
		input.TransactionDate = date
		doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/"+side, input, http.StatusCreated)
	}
	trade("buy", "2026-02-01", 9223372036854775807-10000)
	trade("buy", "2026-02-02", 1000)
	trade("sell", "2026-02-03", 1000)
	trade("buy", "2026-02-04", 20000)
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?alternative_method=lifo", nil, http.StatusUnprocessableEntity)
	var body errorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "LEDGER_OVERFLOW", body.Error.Code)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains", nil, http.StatusOK)
}
