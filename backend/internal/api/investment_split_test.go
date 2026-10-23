package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvestmentSplitAPIPreviewCommitAndNamedRefusals(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ISPLIT")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "10", 10000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/buy", buy, http.StatusCreated)

	request := investmentSplitRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID,
		CommodityID: instrument.CommodityID, RatioNumerator: 3, RatioDenominator: 2,
		SourceEvidence: json.RawMessage(`{"notice":"3-for-2"}`)}
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost,
		"/api/v1/investments/splits/preview", request, http.StatusOK)
	var preview investmentSplitPreviewResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&preview))
	assert.Equal(t, "5", preview.Plan.QuantityDeltaValue.String())
	assert.Equal(t, 0, preview.Plan.QuantityDeltaScale)
	require.Len(t, preview.Plan.Effects, 1)
	assert.Equal(t, "15", preview.Plan.Effects[0].QuantityAfterValue.String())
	assert.Empty(t, preview.Impact.AffectedCheckpoints)

	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/splits", request, http.StatusForbidden)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/splits", request, http.StatusCreated)
	var split investmentSplitResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&split))
	require.Len(t, split.Transaction.JournalEntries, 1)
	assert.Len(t, split.Transaction.JournalEntries[0].Postings, 2)

	// 15 × 1/7 has no exact decimal at the instrument's quantity scale.
	request.EffectiveOn, request.RatioNumerator, request.RatioDenominator = "2026-03-01", 1, 7
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/splits", request, http.StatusUnprocessableEntity)
	assert.Contains(t, res.Body.String(), "INVESTMENT_SPLIT_FRACTION_UNREPRESENTABLE")
	empty := createHoldingAccountForSession(t, handler, f, instrument.ID)
	request.HoldingAccountID, request.RatioNumerator, request.RatioDenominator = empty.ID, 2, 1
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/splits", request, http.StatusCreated)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&split))
	require.Nil(t, split.Transaction)
	require.Positive(t, split.OperationID)
	require.Equal(t, "0", split.Plan.QuantityDeltaValue.String())
}

func TestInvestmentSplitCorrectionAPIReplaceReverseAndLabels(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ISPLITFIX")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "10", 10000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/buy", buy, http.StatusCreated)
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/splits", investmentSplitRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID,
			CommodityID: instrument.CommodityID, RatioNumerator: 2, RatioDenominator: 1}, http.StatusCreated)
	var split investmentSplitResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&split))
	splitPath := func(suffix string) string {
		return "/api/v1/investments/transactions/" + strconv.FormatInt(split.Transaction.ID, 10) + suffix
	}

	// A backdated purchase before the split posts a labelled adjustment.
	backdated := tradeRequestBody(f, holding.ID, instrument.CommodityID, "3", 3000)
	backdated.TransactionDate = "2026-01-15"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/buy", backdated, http.StatusCreated)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/accounts/"+strconv.FormatInt(holding.ID, 10)+"/register", nil, http.StatusOK)
	assert.Contains(t, res.Body.String(), `"system_label":"split_adjustment"`)

	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		splitPath("/correction-chain"), nil, http.StatusOK)
	var chain investmentCorrectionChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&chain))
	assert.True(t, chain.CanCorrectSplit)
	require.NotNil(t, chain.EffectiveSplit)
	assert.Equal(t, int64(2), chain.EffectiveSplit.RatioNumerator)

	replacement := investmentSplitReplacementRequest{Reason: "it was 3-for-1", EffectiveOn: "2026-02-01",
		RatioNumerator: 3, RatioDenominator: 1}
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost,
		splitPath("/replace-split/preview"), replacement, http.StatusOK)
	var preview investmentSplitPreviewResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&preview))
	assert.Equal(t, "26", preview.Plan.QuantityDeltaValue.String())
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, splitPath("/replace-split"), replacement, http.StatusForbidden)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		splitPath("/replace-split"), replacement, http.StatusCreated)
	var replaced investmentSplitReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&replaced))
	assert.Equal(t, split.Transaction.ID, *replaced.CorrectedTransactionID)

	// The original is now corrected; the replacement can be reversed.
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		splitPath("/reverse-split"), investmentSaleReversalRequest{Reason: "again"}, http.StatusConflict)
	assert.Contains(t, res.Body.String(), "INVESTMENT_SPLIT_ALREADY_CORRECTED")
	reversePath := "/api/v1/investments/transactions/" + strconv.FormatInt(replaced.Replacement.ID, 10) + "/reverse-split"
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost,
		reversePath+"/reconciliation-impact", investmentSaleReversalRequest{Reason: "no split happened"}, http.StatusOK)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		reversePath, investmentSaleReversalRequest{Reason: "no split happened"}, http.StatusCreated)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/transactions/999999/reverse-split", investmentSaleReversalRequest{Reason: "x"}, http.StatusNotFound)
	assert.Contains(t, res.Body.String(), "NOT_FOUND")
}

func TestJournalFreeSplitOperationAPIAndActivityPagination(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ISPLITOPS")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	request := investmentSplitRequest{EffectiveOn: "2026-03-01", HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID, RatioNumerator: 2, RatioDenominator: 1}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/splits", request, http.StatusCreated)
	var zero investmentSplitResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&zero))
	require.Nil(t, zero.Transaction)
	path := "/api/v1/investments/operations/" + strconv.FormatInt(zero.OperationID, 10)
	replacement := investmentSplitReplacementRequest{Reason: "broker notice correction", EffectiveOn: "2026-03-01", RatioNumerator: 3, RatioDenominator: 1}
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/replace-split/preview", replacement, http.StatusOK)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/replace-split", replacement, http.StatusForbidden)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path+"/replace-split", replacement, http.StatusCreated)
	var successor investmentSplitReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&successor))
	require.Nil(t, successor.Inverse)
	require.Nil(t, successor.Replacement)
	require.Nil(t, successor.CorrectedTransactionID)
	require.Equal(t, zero.OperationID, successor.CorrectedOperationID)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/splits?limit=1", nil, http.StatusOK)
	var page splitActivityPageResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Splits, 1)
	require.Equal(t, "effective", page.Splits[0].Standing)
	require.NotNil(t, page.NextCursor)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/splits?limit=1&cursor="+*page.NextCursor, nil, http.StatusOK)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Splits, 1)
	require.Equal(t, "superseded", page.Splits[0].Standing)
	require.Nil(t, page.Splits[0].TransactionID)
	require.Nil(t, page.NextCursor)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/splits?operation_id="+strconv.FormatInt(zero.OperationID, 10), nil, http.StatusOK)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Splits, 1)
	require.Equal(t, zero.OperationID, page.Splits[0].OperationID)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/splits?operation_id=999999", nil, http.StatusNotFound)
	reversal := investmentSaleReversalRequest{Reason: "entered twice"}
	path = "/api/v1/investments/operations/" + strconv.FormatInt(successor.OperationID, 10)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/reverse-split/reconciliation-impact", reversal, http.StatusOK)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path+"/reverse-split", reversal, http.StatusCreated)
	var reversed investmentSplitReversalResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&reversed))
	require.Nil(t, reversed.Transaction)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/splits", nil, http.StatusOK)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Splits, 2)
	require.Equal(t, "reversed", page.Splits[0].Standing)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/splits?cursor=-1", nil, http.StatusBadRequest)
}

func TestJournalFreeSplitOperationCashInLieuAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ISPLITCASHOPS")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	request := investmentSplitRequest{EffectiveOn: "2026-03-01", HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID, RatioNumerator: 3, RatioDenominator: 2}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/splits", request, http.StatusCreated)
	var split investmentSplitResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&split))
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 10000)
	buy.TransactionDate = "2026-02-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	path := "/api/v1/investments/operations/" + strconv.FormatInt(split.OperationID, 10)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path+"/cash-in-lieu-lots?currency_id="+strconv.FormatInt(f.commodityID, 10)+"&disposal_on=2026-03-01", nil, http.StatusOK)
	var lots cashInLieuLotsResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&lots))
	require.Len(t, lots.Lots, 1)
	cash := cashInLieuRequest{SettledOperationID: split.OperationID, DisposalOn: "2026-03-01", PaymentOn: "2026-03-05", QuantityValue: "5", QuantityScale: 1, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, ProceedsValue: 800, ProceedsScale: 2}
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/cash-in-lieu/preview", cash, http.StatusOK)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/cash-in-lieu", cash, http.StatusCreated)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/reverse-split/reconciliation-impact", investmentSaleReversalRequest{Reason: "wrong action"}, http.StatusOK)
	var impact reconciliationImpactResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&impact))
	require.NotNil(t, impact.GainImpact)
	require.Len(t, impact.GainImpact.Changes, 1)
	require.Equal(t, "cash_in_lieu", impact.GainImpact.Changes[0].OperationKind)
	require.NotEmpty(t, impact.GainImpact.Acknowledgement)
	// Actual original units support the fraction; commit requires the reviewed gain.
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path+"/reverse-split", investmentSaleReversalRequest{Reason: "wrong action"}, http.StatusConflict)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path+"/reverse-split", investmentSaleReversalRequest{Reason: "wrong action", GainImpactAcknowledgement: impact.GainImpact.Acknowledgement}, http.StatusCreated)
}
