package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/exact"
)

func TestCryptoInstrumentAndTradeAPIUsesExactScale24(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	body := investmentInstrumentRequest{CommodityCode: "BTC-API", InstrumentType: "crypto", DisplayName: "Bitcoin API", QuantityScale: 24, PriceScale: 12, Identifiers: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`), QuoteCommodityID: &f.commodityID}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/instruments", body, http.StatusCreated)
	var instrument investmentInstrumentResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&instrument))
	assert.Equal(t, 24, instrument.QuantityScale)
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	trade := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1000000000000000000000001", 10000)
	trade.QuantityScale = 24
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", trade, http.StatusCreated)
	trade.QuantityValue = exact.MustParse("1")
	trade.CashAmountValue = 1
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", trade, http.StatusCreated)
	body.CommodityCode = "TOO-PRECISE"
	body.QuantityScale = 25
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/instruments", body, http.StatusBadRequest)
	body.InstrumentType = "stock"
	body.QuantityScale = 24
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/instruments", body, http.StatusBadRequest)
}
func TestCryptoCSVMultipartAPIStagesThenCommitsLots(t *testing.T) {
	t.Parallel()
	handler, database := newImportConnectionsTestHandler(t, app.NoOpProber{})
	f := bootstrapInvestmentAPITest(t, handler)
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/instruments", investmentInstrumentRequest{CommodityCode: "BTC-CSV", InstrumentType: "crypto", DisplayName: "Bitcoin CSV", QuantityScale: 24, PriceScale: 12, Identifiers: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`), QuoteCommodityID: &f.commodityID}, http.StatusCreated)
	var instrument investmentInstrumentResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&instrument))
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	file, err := writer.CreateFormFile("file", "crypto.csv")
	require.NoError(t, err)
	// The helper's accounts open on this date; no browser-clock assumption here.
	_, err = fmt.Fprintln(file, "date,side,quantity,amount,external_ref,memo\n2026-02-01,BUY,0.123456789012345678901234,10.00,api-crypto-1,CSV buy")
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("format", "crypto_csv"))
	require.NoError(t, writer.WriteField("holding_account_id", itoa(holding.ID)))
	require.NoError(t, writer.WriteField("cash_account_id", itoa(f.cashAccount.ID)))
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/imports", &payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set(csrfTokenHeader, f.csrfToken)
	setSameOrigin(request)
	request.AddCookie(f.sessionCookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	var staged startImportResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&staged))
	assert.Equal(t, "crypto_csv", staged.Batch.SourceKind)
	var count int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&count))
	assert.Zero(t, count)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/imports/"+itoa(staged.Batch.ID)+"/commit", map[string]any{}, http.StatusOK)
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&count))
	assert.Equal(t, 1, count)
}
func TestQuoteRefreshAPIAcceptsWorkAndRequiresAuthCSRF(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	doPricingRequest(t, handler, nil, "", http.MethodPost, "/api/v1/pricing/quotes/refresh", map[string]any{}, http.StatusUnauthorized)
	doPricingRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/pricing/quotes/refresh", map[string]any{}, http.StatusForbidden)
	res := doPricingRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/quotes/refresh", map[string]any{}, http.StatusAccepted)
	var accepted quoteRefreshAccepted
	require.NoError(t, json.NewDecoder(res.Body).Decode(&accepted))
	assert.Zero(t, accepted.QueuedCount)
	doPricingRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/quotes/refresh", map[string]any{"unknown": true}, http.StatusBadRequest)
}
