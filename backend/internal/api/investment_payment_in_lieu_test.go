package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #255: wire contract for a short holder's payment in lieu of a dividend.
func TestPaymentInLieuEntryAndCorrectionAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "PILCO")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	expense := createLedgerAccount(t, handler, f.sessionCookie, f.csrfToken, "Borrow costs", "expense", "expense", f.commodityID, 2)
	post := func(path string, body any, status int) *http.Response {
		return doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, body, status).Result()
	}
	request := func(owed int64) paymentInLieuRequest {
		return paymentInLieuRequest{HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID, ExDate: "2026-03-01",
			TransactionDate: "2026-03-05", CashAccountID: f.cashAccount.ID, CashCommodityID: f.commodityID,
			ExpenseAccountID: expense.ID, AmountValue: moneyCoefficient(owed), AmountScale: 2}
	}

	res := post("/api/v1/investments/payment-in-lieu", request(500), http.StatusUnprocessableEntity)
	var refusal errorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&refusal))
	assert.Equal(t, "INVESTMENT_PAYMENT_IN_LIEU_NOT_SHORT", refusal.Error.Code)

	res = post("/api/v1/investments/short-sale", tradeRequestBody(f, holding.ID, instrument.CommodityID, "10", 10000), http.StatusCreated)
	var shortSale investmentTradeResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&shortSale))
	post("/api/v1/investments/payment-in-lieu/reconciliation-impact", request(500), http.StatusOK)
	res = post("/api/v1/investments/payment-in-lieu", request(500), http.StatusCreated)
	var created transactionResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&created))

	base := "/api/v1/investments/transactions/" + strconv.FormatInt(created.ID, 10)
	chainRes := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
	var chain investmentCorrectionChainResponse
	require.NoError(t, json.NewDecoder(chainRes.Body).Decode(&chain))
	require.True(t, chain.CanCorrectPaymentInLieu)
	require.NotNil(t, chain.EffectivePaymentInLieu)
	assert.Equal(t, "2026-03-01", chain.EffectivePaymentInLieu.ExDate)
	assert.Equal(t, holding.ID, chain.EffectivePaymentInLieu.HoldingAccountID)

	replace := investmentPaymentInLieuReplacementRequest{Reason: "broker charged 6.00", Replacement: request(600)}
	post(base+"/replace-payment-in-lieu/reconciliation-impact", replace, http.StatusOK)
	res = post(base+"/replace-payment-in-lieu", replace, http.StatusCreated)
	var replaced investmentDividendReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&replaced))
	assert.Equal(t, created.ID, replaced.CorrectedTransactionID)
	expectInvestmentConflict(t, handler, f, base+"/replace-payment-in-lieu", replace, "INVESTMENT_PAYMENT_IN_LIEU_ALREADY_CORRECTED")

	replacementBase := "/api/v1/investments/transactions/" + strconv.FormatInt(replaced.ReplacementTransaction.ID, 10)
	reverse := investmentSaleReversalRequest{Reason: "charged in error"}
	// #257: the short sale cannot be reversed from under an effective payment.
	shortBase := "/api/v1/investments/transactions/" + strconv.FormatInt(shortSale.Transaction.ID, 10)
	reverseShort := investmentSaleReversalRequest{Reason: "never shorted"}
	expectInvestmentConflict(t, handler, f, shortBase+"/reverse-short-sale/reconciliation-impact", reverseShort, "INVESTMENT_PAYMENT_IN_LIEU_DEPENDENCY")
	expectInvestmentConflict(t, handler, f, shortBase+"/reverse-short-sale", reverseShort, "INVESTMENT_PAYMENT_IN_LIEU_DEPENDENCY")
	post(replacementBase+"/reverse-payment-in-lieu/reconciliation-impact", reverse, http.StatusOK)
	post(replacementBase+"/reverse-payment-in-lieu", reverse, http.StatusCreated)
	post(shortBase+"/reverse-short-sale", reverseShort, http.StatusCreated)

	bundle := downloadBundle(t, handler, f.sessionCookie, "")
	facts := bundle.table(t, "investment-payment-in-lieu-facts.csv")
	require.Len(t, facts.rows, 3, "original, replacement and reversal each carry the entitlement")
	for _, row := range facts.rows {
		assert.Equal(t, "2026-03-01", facts.column(row, "ex_date"))
		assert.Equal(t, strconv.FormatInt(holding.ID, 10), facts.column(row, "holding_account_id"))
	}
}
