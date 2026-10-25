package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R13 slices 1, 3, 4, 5 and 8: the returns read is wired, echoes its policy and scope, and
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
	// Slice 3: the one-day series is the opening point and the closing day.
	assert.Equal(t, "day", body.TimeWeightedSeries.Interval)
	require.Len(t, body.TimeWeightedSeries.Points, 2)
	assert.Equal(t, "2026-01-31", body.TimeWeightedSeries.Points[0].Date)
	assert.Equal(t, body.TimeWeighted, body.TimeWeightedSeries.Points[1].TimeWeighted)
	require.NotNil(t, body.TimeWeightedSeries.Valuation)
	// Slice 4: the closing value split three ways, keys by dimension.
	assert.Equal(t, "2026-02-01", body.Allocation.Date)
	assert.NotNil(t, body.Allocation.Unknown)
	for _, row := range body.Allocation.ByAccount {
		assert.NotNil(t, row.AccountID)
		assert.Nil(t, row.AssetType)
		assert.Nil(t, row.CurrencyID)
	}
	for _, row := range body.Allocation.ByAssetType {
		assert.NotNil(t, row.AssetType)
		assert.Nil(t, row.AccountID)
	}

	// Slice 5: no benchmark unless asked; one is read at the series' dates.
	assert.Nil(t, body.Benchmark)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		path+fmt.Sprintf("&benchmark_commodity_id=%d&benchmark_basis=total_return", instrument.CommodityID), nil, http.StatusOK)
	var compared investmentReturnsResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&compared))
	require.NotNil(t, compared.Benchmark)
	assert.Equal(t, instrument.CommodityID, compared.Benchmark.CommodityID)
	assert.Equal(t, "total_return", compared.Benchmark.Basis)
	require.Len(t, compared.Benchmark.Points, 2)
	assert.Equal(t, compared.Benchmark.TimeWeighted, compared.Benchmark.Points[1].TimeWeighted)
	assert.NotNil(t, compared.Benchmark.Splits, "an empty list, never null")
	require.NotNil(t, compared.Benchmark.Valuation)
	assert.Equal(t, "live_total_return_v1", compared.Benchmark.Valuation.SelectionPolicy)
	assert.Equal(t, body.TimeWeighted, compared.TimeWeighted)

	// Slice 8: instrument returns only when asked; the key is absent otherwise.
	require.Nil(t, body.InstrumentReturns)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path, nil, http.StatusOK)
	var raw map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(res.Body).Decode(&raw))
	assert.NotContains(t, raw, "instrument_returns")
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path+"&instruments=true", nil, http.StatusOK)
	var withInstruments investmentReturnsResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&withInstruments))
	require.NotNil(t, withInstruments.InstrumentReturns)
	assert.Equal(t, "instrument_returns_v1", withInstruments.InstrumentReturns.Policy)
	assert.Equal(t, "ex_date", withInstruments.InstrumentReturns.IncomeRecognition)
	require.Len(t, withInstruments.InstrumentReturns.Rows, 1)
	assert.Equal(t, instrument.CommodityID, withInstruments.InstrumentReturns.Rows[0].CommodityID)
	assert.NotNil(t, withInstruments.InstrumentReturns.NotAttributed, "an empty list, never null")
	assert.Equal(t, body.TimeWeighted, withInstruments.TimeWeighted)

	for _, query := range []string{
		fmt.Sprintf("?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d&instruments=yes", f.commodityID),
		"?from=2026-02-01&to=2026-02-01",
		fmt.Sprintf("?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d&benchmark_commodity_id=%d", f.commodityID, f.commodityID),
		fmt.Sprintf("?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d&benchmark_commodity_id=x", f.commodityID),
		fmt.Sprintf("?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d&benchmark_basis=total_return", f.commodityID),
		fmt.Sprintf("?from=2026-02-01&to=2026-02-01&reporting_currency_id=%d&benchmark_commodity_id=%d&benchmark_basis=adjusted", f.commodityID, instrument.CommodityID),
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
