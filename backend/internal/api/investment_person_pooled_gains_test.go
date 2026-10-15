package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestInvestmentGainsPersonPooledAverageAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "PERSONPOOL")
	first := createHoldingAccountForSession(t, handler, f, instrument.ID)
	second := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, first.ID, instrument.CommodityID, "2", 2000)
	buy.TransactionDate = "2026-02-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	buy = tradeRequestBody(f, second.ID, instrument.CommodityID, "2", 6000)
	buy.TransactionDate = "2026-02-02"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	sale := tradeRequestBody(f, first.ID, instrument.CommodityID, "1", 4000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)
	path := "/api/v1/investments/gains?alternative_method=person_pooled_average&jurisdiction_note=" + url.QueryEscape("  Owner context <not a rule>  ") + "&valuation_as_of=2026-03-02&from=2026-03-01&to=2026-03-31"
	for range 2 {
		response := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path, nil, http.StatusOK)
		var body investmentGainsResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
		require.NotNil(t, body.Alternative)
		alt := body.Alternative
		assert.Equal(t, "person_pooled_average_v1", alt.Profile.Name)
		assert.Equal(t, "alternative_method", alt.Profile.Purpose)
		assert.Equal(t, "person_pooled_average", alt.Profile.BasisMethod)
		assert.Equal(t, "owner_security_cost_currency_position_side", alt.Profile.PoolScope)
		assert.Equal(t, "Owner context <not a rule>", alt.Profile.JurisdictionNote)
		assert.Equal(t, body.Profile.KnowledgeCutoff, alt.Profile.KnowledgeCutoff)
		assert.Equal(t, body.Profile.ValuationAsOf, alt.Profile.ValuationAsOf)
		assert.Equal(t, "2026-03-01", alt.Profile.From)
		assert.Equal(t, "internal_pool_neutral", alt.Profile.TransferPolicy)
		require.Len(t, alt.RealizedTotals, 1)
		require.NotNil(t, alt.RealizedTotals[0].TotalGainValue)
		value := exact.New(int64(*alt.RealizedTotals[0].TotalGainValue))
		requireScaledAPIAmount(t, 2000, 2, &value, alt.RealizedTotals[0].TotalGainScale)
		require.Len(t, alt.Unrealized, 2)
		value = exact.New(int64(*body.RealizedTotals[0].TotalGainValue))
		requireScaledAPIAmount(t, 3000, 2, &value, body.RealizedTotals[0].TotalGainScale)
		assert.Empty(t, body.Profile.JurisdictionNote)
		assert.Empty(t, body.Profile.PoolScope)
	}
	doInvestmentRequest(t, handler, nil, "", http.MethodGet, path, nil, http.StatusUnauthorized)
	for _, query := range []string{
		"jurisdiction_note=orphan",
		"alternative_method=fifo&jurisdiction_note=orphan",
		"alternative_method=person_pooled_average&jurisdiction_note=" + url.QueryEscape(strings.Repeat("界", 501)),
		"alternative_method=person_pooled_average&jurisdiction_note=",
		"alternative_method=person_pooled_average&jurisdiction_note=a&jurisdiction_note=b",
		"alternative_method=person_pooled_average&alternative_method=fifo",
		"alternative_method=",
	} {
		response := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?"+query, nil, http.StatusBadRequest)
		var body errorResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
		assert.Equal(t, "VALIDATION_FAILED", body.Error.Code)
	}
}

func TestInvestmentGainsPersonPoolOverflowIs422WithoutChangingOperationalRead(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "POOLOVERFLOW")
	for range 2 {
		holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
		buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 5000000000000000000)
		buy.TransactionDate = "2026-02-01"
		doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	}
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?alternative_method=person_pooled_average", nil, http.StatusUnprocessableEntity)
	var body errorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "LEDGER_OVERFLOW", body.Error.Code)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains", nil, http.StatusOK)
}
