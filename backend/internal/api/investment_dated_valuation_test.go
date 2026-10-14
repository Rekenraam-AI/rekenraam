package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #230: valuation_as_of replays positions to a date under operational_v1 and
// names the profile; a future or malformed date is a validation error.
func TestInvestmentGainsValuationAsOfAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ASOF")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy",
		tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 5000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)

	type gains struct {
		Profile    gainsProfileResponse     `json:"profile"`
		Unrealized []unrealizedGainResponse `json:"unrealized"`
	}
	read := func(query string) gains {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains"+query, nil, http.StatusOK)
		var body gains
		require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
		return body
	}

	current := read("")
	assert.Empty(t, current.Unrealized, "sold today")
	assert.Equal(t, "operational_v1", current.Profile.Name)
	assert.Equal(t, "current_effective", current.Profile.LedgerKnowledge)
	assert.Equal(t, "live_raw_v1", current.Profile.QuotePolicy)
	assert.Equal(t, "none", current.Profile.FXMethod)
	assert.NotEmpty(t, current.Profile.ValuationAsOf)
	assert.NotEmpty(t, current.Profile.KnowledgeCutoff)

	february := read("?valuation_as_of=2026-02-15")
	assert.Equal(t, "2026-02-15", february.Profile.ValuationAsOf)
	require.Len(t, february.Unrealized, 1)
	position := february.Unrealized[0]
	assert.Equal(t, "2", position.QuantityValue.String())
	assert.Equal(t, "2026-02-01", position.LatestPriceDate, "the fill price on or before the date")
	require.NotNil(t, position.UnrealizedGainValue)
	assert.Equal(t, "2026-02-15", position.ValuationAsOf)

	for _, query := range []string{"?valuation_as_of=2099-01-01", "?valuation_as_of=2026-02-30"} {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains"+query, nil, http.StatusBadRequest)
		var failure errorResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&failure))
		assert.Equal(t, "VALIDATION_FAILED", failure.Error.Code, query)
	}
}
