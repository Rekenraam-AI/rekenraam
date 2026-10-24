package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R13 slice 1: the returns read is wired, echoes its policy and scope, and
// keeps a reason beside every missing figure. Calculation literals live in
// the app package's tests.
func TestInvestmentReturnsAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "RET")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	// Paid from checking, outside the default investment scope: a contribution.
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy",
		tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)

	path := fmt.Sprintf("/api/v1/investments/returns?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d", f.commodityID)
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path, nil, http.StatusOK)
	var body investmentReturnsResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "returns_v1", body.Policy)
	assert.Equal(t, "2026-01-31", body.OpeningDate)
	assert.Equal(t, 1, body.Days)
	assert.Equal(t, "default_investment_accounts", body.AccountScope)
	assert.Contains(t, body.ResolvedAccountIDs, holding.ID)
	assert.NotContains(t, body.ResolvedAccountIDs, f.cashAccount.ID)
	require.NotNil(t, body.Contributions.Amount)
	assert.Equal(t, "4000", body.Contributions.Amount.QuantityValue.String())
	require.NotNil(t, body.TimeWeighted.Value)
	assert.Equal(t, "0.000000", *body.TimeWeighted.Value)
	require.NotNil(t, body.TimeWeightedAnnualized.Reason)
	assert.Equal(t, "period_shorter_than_year", *body.TimeWeightedAnnualized.Reason)
	assert.Nil(t, body.TimeWeightedAnnualized.Value)
	require.NotNil(t, body.Valuation)
	assert.Equal(t, f.commodityID, body.Valuation.CommodityID)
	assert.NotNil(t, body.AttributionFallbacks)

	for _, query := range []string{
		"?from=2026-02-01&to=2026-02-01",
		fmt.Sprintf("?from=2026-02-02&to=2026-02-01&reporting_currency_id=%d", f.commodityID),
		fmt.Sprintf("?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d&account_id=%d", f.commodityID, f.incomeAccount.ID),
		fmt.Sprintf("?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d&max_staleness_days=40", f.commodityID),
	} {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/returns"+query, nil, http.StatusBadRequest)
		var failure errorResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&failure))
		assert.Equal(t, "VALIDATION_FAILED", failure.Error.Code, query)
	}
}
