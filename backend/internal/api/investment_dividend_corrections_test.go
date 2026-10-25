package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/exact"
)

// T-115: wire contract for dividend and reinvested-dividend corrections.

func expectInvestmentConflict(t *testing.T, handler http.Handler, f investmentAPITestFixture, path string, body any, code string) {
	t.Helper()
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, body, http.StatusConflict)
	var envelope errorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&envelope))
	assert.Equal(t, code, envelope.Error.Code)
}

func TestReplaceAndReverseDividendAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "DIVPAY")
	dividend := func(amount int64) dividendRequest {
		return dividendRequest{CommodityID: nullableField[int64]{app.NullableValue(instrument.CommodityID)}, TransactionDate: "2026-04-01", CashAccountID: f.cashAccount.ID, CashCommodityID: f.commodityID,
			IncomeAccountID: &f.incomeAccount.ID, AmountValue: moneyCoefficient(amount), AmountScale: 2}
	}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", dividend(5000), http.StatusCreated)
	var original transactionResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&original))
	base := "/api/v1/investments/transactions/" + strconv.FormatInt(original.ID, 10)
	replace := investmentDividendReplacementRequest{Reason: "statement shows 55.00", Replacement: dividend(5500)}

	previewRes := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, base+"/replace-dividend/reconciliation-impact", replace, http.StatusOK)
	var preview map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(previewRes.Body).Decode(&preview))
	assert.JSONEq(t, `[]`, string(preview["affected_checkpoints"]))
	_, hasGainImpact := preview["gain_impact"]
	assert.False(t, hasGainImpact, "a cash dividend replays no disposal")

	replacedRes := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/replace-dividend", replace, http.StatusCreated)
	var replaced investmentDividendReplacementResponse
	require.NoError(t, json.NewDecoder(replacedRes.Body).Decode(&replaced))
	assert.Equal(t, original.ID, replaced.CorrectedTransactionID)
	require.NotNil(t, replaced.InverseTransaction.CorrectionOfTransactionID)
	assert.Equal(t, original.ID, *replaced.InverseTransaction.CorrectionOfTransactionID)
	assert.Equal(t, original.ID, *replaced.ReplacementTransaction.CorrectionOfTransactionID)

	chainRes := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
	var chain investmentCorrectionChainResponse
	require.NoError(t, json.NewDecoder(chainRes.Body).Decode(&chain))
	require.NotNil(t, chain.EffectiveDividend)
	assert.Equal(t, &instrument.CommodityID, chain.EffectiveDividend.CommodityID)
	activityRes := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/transactions?commodity_id="+strconv.FormatInt(instrument.CommodityID, 10), nil, http.StatusOK)
	var activity struct {
		Transactions []transactionResponse `json:"transactions"`
	}
	require.NoError(t, json.NewDecoder(activityRes.Body).Decode(&activity))
	require.Len(t, activity.Transactions, 3, "original, inverse and replacement appear once")
	bundle := downloadBundle(t, handler, f.sessionCookie, "")
	facts := bundle.table(t, "investment-dividend-facts.csv")
	require.Len(t, facts.rows, 2)
	for _, row := range facts.rows {
		assert.Equal(t, strconv.FormatInt(instrument.CommodityID, 10), facts.column(row, "commodity_id"))
		assert.NotEmpty(t, facts.column(row, "operation_id"))
		assert.NotEmpty(t, facts.column(row, "audit_event_id"))
	}

	expectInvestmentConflict(t, handler, f, base+"/replace-dividend", replace, "INVESTMENT_DIVIDEND_ALREADY_CORRECTED")
	expectInvestmentConflict(t, handler, f, base+"/reverse-dividend", investmentSaleReversalRequest{Reason: "again"}, "INVESTMENT_DIVIDEND_ALREADY_CORRECTED")
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/reverse-dividend", investmentSaleReversalRequest{}, http.StatusBadRequest)

	replacementBase := "/api/v1/investments/transactions/" + strconv.FormatInt(replaced.ReplacementTransaction.ID, 10)
	reversedRes := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, replacementBase+"/reverse-dividend",
		investmentSaleReversalRequest{Reason: "paid in error"}, http.StatusCreated)
	var reversed investmentSaleReversalResponse
	require.NoError(t, json.NewDecoder(reversedRes.Body).Decode(&reversed))
	assert.Equal(t, replaced.ReplacementTransaction.ID, reversed.CorrectedTransactionID)
	// The reversal command is fenced to dividends.
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost,
		"/api/v1/investments/transactions/"+strconv.FormatInt(reversed.Transaction.ID, 10)+"/reverse-reinvested-dividend/reconciliation-impact",
		investmentSaleReversalRequest{Reason: "wrong family"}, http.StatusNotFound)
}

func TestReplaceAndReverseReinvestedDividendAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "REIN")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	reinvestment := func(quantity int64, amount int64) reinvestedDividendRequest {
		return reinvestedDividendRequest{TransactionDate: "2026-01-02", CommodityID: instrument.CommodityID, HoldingAccountID: holding.ID,
			IncomeAccountID: &f.incomeAccount.ID, QuantityValue: exact.New(quantity), QuantityScale: 0,
			AmountValue: moneyCoefficient(amount), AmountScale: 2, CashCommodityID: f.commodityID}
	}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/reinvested-dividend", reinvestment(10, 2000), http.StatusCreated)
	var original investmentTradeResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&original))
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "10", 20000)
	buy.TransactionDate = "2026-02-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "5", 15000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)

	base := "/api/v1/investments/transactions/" + strconv.FormatInt(original.Transaction.ID, 10)
	replace := investmentReinvestmentReplacementRequest{Reason: "broker restated", Replacement: reinvestment(4, 4000)}
	previewRes := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, base+"/replace-reinvested-dividend/reconciliation-impact", replace, http.StatusOK)
	var preview reconciliationImpactResponse
	require.NoError(t, json.NewDecoder(previewRes.Body).Decode(&preview))
	require.NotNil(t, preview.GainImpact)
	require.Len(t, preview.GainImpact.Changes, 1)
	change := preview.GainImpact.Changes[0]
	assert.Equal(t, "revised", change.ChangeKind)
	assertGainCoefficient(t, 14000, 2, change.Before.RealizedGainValue, change.Before.RealizedGainScale)
	assertGainCoefficient(t, 9000, 2, change.After.RealizedGainValue, change.After.RealizedGainScale)

	expectInvestmentConflict(t, handler, f, base+"/replace-reinvested-dividend", replace, "INVESTMENT_GAIN_IMPACT_ACKNOWLEDGEMENT_REQUIRED")
	replace.GainImpactAcknowledgement = "stale"
	expectInvestmentConflict(t, handler, f, base+"/replace-reinvested-dividend", replace, "INVESTMENT_GAIN_IMPACT_ACKNOWLEDGEMENT_STALE")
	replace.GainImpactAcknowledgement = preview.GainImpact.Acknowledgement
	replacedRes := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/replace-reinvested-dividend", replace, http.StatusCreated)
	var replaced investmentBuyReplacementResponse
	require.NoError(t, json.NewDecoder(replacedRes.Body).Decode(&replaced))
	require.NotNil(t, replaced.Replacement.LotID)
	expectInvestmentConflict(t, handler, f, base+"/reverse-reinvested-dividend",
		investmentSaleReversalRequest{Reason: "again"}, "INVESTMENT_REINVESTMENT_ALREADY_CORRECTED")

	// Reversing the replacement replays the sale onto the bought shares.
	replacementBase := "/api/v1/investments/transactions/" + strconv.FormatInt(replaced.Replacement.Transaction.ID, 10)
	reverse := investmentSaleReversalRequest{Reason: "the fund paid cash"}
	reverseImpact := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, replacementBase+"/reverse-reinvested-dividend/reconciliation-impact", reverse, http.StatusOK)
	var reversePreview reconciliationImpactResponse
	require.NoError(t, json.NewDecoder(reverseImpact.Body).Decode(&reversePreview))
	reverse.GainImpactAcknowledgement = reversePreview.GainImpact.Acknowledgement
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, replacementBase+"/reverse-reinvested-dividend", reverse, http.StatusCreated)
	// A buy is not a reinvestment, and a reinvestment is not a dividend.
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, replacementBase+"/reverse-dividend/reconciliation-impact",
		investmentSaleReversalRequest{Reason: "wrong family"}, http.StatusNotFound)
}

func TestSeparateWithholdingAPIAndBundle(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "TAXPAY")
	taxAccount := createLedgerAccount(t, handler, f.sessionCookie, f.csrfToken, "Withholding tax", "expense", "expense", f.commodityID, 2)
	input := dividendRequest{WithholdingOnly: true, CommodityID: nullableField[int64]{app.NullableValue(instrument.CommodityID)},
		TransactionDate: "2026-04-01", CashAccountID: f.cashAccount.ID, CashCommodityID: f.commodityID,
		WithholdingAccountID: &taxAccount.ID, AmountValue: moneyCoefficient(1500), AmountScale: 2}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/dividend", input, http.StatusCreated)
	var tax transactionResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tax))
	require.Len(t, tax.JournalEntries[0].Postings, 2)
	bundle := downloadBundle(t, handler, f.sessionCookie, "")
	facts := bundle.table(t, "investment-dividend-facts.csv")
	require.Len(t, facts.rows, 1)
	assert.Equal(t, "1", facts.column(facts.rows[0], "withholding_only"))
	assert.Equal(t, strconv.FormatInt(instrument.CommodityID, 10), facts.column(facts.rows[0], "commodity_id"))
	components := bundle.table(t, "investment-operation-components.csv")
	require.Len(t, components.rows, 1)
	assert.Equal(t, "withholding", components.column(components.rows[0], "component_kind"))
	assert.Equal(t, "-1500", components.column(components.rows[0], "amount_value"))
	assert.Equal(t, strconv.FormatInt(f.commodityID, 10), components.column(components.rows[0], "commodity_id"))
}

// R13 D2: a dividend's ex date crosses the wire on entry, in its correction
// terms, through a replacement (null clears) and in the bundle's dates.
func TestDividendExDateAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "DIVEX")
	dividend := func(exDate any) map[string]any {
		return map[string]any{"commodity_id": instrument.CommodityID, "transaction_date": "2026-04-01", "ex_date": exDate,
			"cash_account_id": f.cashAccount.ID, "cash_commodity_id": f.commodityID, "income_account_id": f.incomeAccount.ID,
			"amount_value": "5000", "amount_scale": 2}
	}
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", dividend("2026-04-02"), http.StatusBadRequest)
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", dividend("2026-03-20"), http.StatusCreated)
	var original transactionResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&original))
	base := "/api/v1/investments/transactions/" + strconv.FormatInt(original.ID, 10)

	chainRes := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
	var chain investmentCorrectionChainResponse
	require.NoError(t, json.NewDecoder(chainRes.Body).Decode(&chain))
	require.NotNil(t, chain.EffectiveDividend)
	require.NotNil(t, chain.EffectiveDividend.ExDate)
	assert.Equal(t, "2026-03-20", *chain.EffectiveDividend.ExDate)
	dates := downloadBundle(t, handler, f.sessionCookie, "").table(t, "investment-operation-dates.csv")
	exRows := 0
	for _, row := range dates.rows {
		if dates.column(row, "date_role") == "ex" {
			exRows++
			assert.Equal(t, "2026-03-20", dates.column(row, "event_date"))
		}
	}
	assert.Equal(t, 1, exRows)

	replacedRes := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/replace-dividend",
		map[string]any{"reason": "no ex date known", "replacement": dividend(nil)}, http.StatusCreated)
	var replaced investmentDividendReplacementResponse
	require.NoError(t, json.NewDecoder(replacedRes.Body).Decode(&replaced))
	chainRes = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/investments/transactions/"+strconv.FormatInt(replaced.ReplacementTransaction.ID, 10)+"/correction-chain", nil, http.StatusOK)
	var raw struct {
		EffectiveDividend map[string]json.RawMessage `json:"effective_dividend"`
	}
	require.NoError(t, json.NewDecoder(chainRes.Body).Decode(&raw))
	require.NotNil(t, raw.EffectiveDividend)
	assert.NotContains(t, raw.EffectiveDividend, "ex_date", "null cleared the ex date")
}
