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

// #312: transfer, capital-return and split corrections accept
// source_evidence_edits and keep every recorded token they do not name, read
// back from the raw correction-chain response.

const apiExactSourceEvidence = `{"reference":"issuer notice","revision":9007199254740993,` +
	`"nested":{"rate":0.1000000000000000055511151231257827}}`

func assertAPIExactEvidence(t *testing.T, chain string) {
	t.Helper()
	assert.Contains(t, chain, `"revision":9007199254740993`)
	assert.NotContains(t, chain, `"revision":9007199254740992`)
	assert.Contains(t, chain, `"rate":0.1000000000000000055511151231257827`)
}

func TestTransferInCorrectionAPIPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "XINEV")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	basis := moneyCoefficient(8000)
	request := externalTransferInRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID,
		CommodityID: instrument.CommodityID, QuantityValue: exact.New(2), CarriedBasisValue: &basis,
		CarriedBasisScale: 2, CostCommodityID: f.commodityID, OriginalAcquiredOn: "2020-03-01",
		SourceEvidence: json.RawMessage(apiExactSourceEvidence)}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/transfers/external/in", request, http.StatusCreated)
	var in externalTransferInResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&in))
	base := "/api/v1/investments/transactions/" + strconv.FormatInt(in.Transaction.ID, 10)

	corrected := request
	corrected.SourceEvidence = nil
	more := moneyCoefficient(10000)
	corrected.CarriedBasisValue = &more
	replace := investmentTransferInReplacementRequest{Reason: "statement shows 100.00", Replacement: corrected,
		SourceEvidenceEdits: json.RawMessage(`{"reference":"amended notice"}`)}
	both := replace
	both.Replacement.SourceEvidence = json.RawMessage(`{"reference":"amended notice"}`)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost,
		base+"/replace-transfer-in/reconciliation-impact", both, http.StatusBadRequest)
	assert.Contains(t, res.Body.String(), "VALIDATION_FAILED", "a whole replacement and edits at once")
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		base+"/replace-transfer-in", replace, http.StatusCreated)
	var replaced investmentTransferInReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&replaced))

	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/transactions/"+
		strconv.FormatInt(replaced.Replacement.Transaction.ID, 10)+"/correction-chain", nil, http.StatusOK)
	chain := res.Body.String()
	assertAPIExactEvidence(t, chain)
	assert.Contains(t, chain, `"reference":"amended notice"`)
}

func TestCapitalReturnCorrectionAPIPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "RCEV")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 2000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	request := capitalReturnRequest{HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID,
		CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, EffectiveOn: "2026-02-01", PaymentOn: "2026-02-10",
		AmountValue: exact.New(400), AmountScale: 2, SourceEvidence: json.RawMessage(apiExactSourceEvidence)}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/return-of-capital", request, http.StatusCreated)
	var posted capitalReturnResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&posted))

	// The form clears the reference: only that key is removed.
	request.SourceEvidence = nil
	request.AmountValue = exact.New(500)
	correction := capitalReturnReplacementRequest{Reason: "notice corrected", Replacement: request,
		SourceEvidenceEdits: json.RawMessage(`{"reference":null}`)}
	path := "/api/v1/investments/transactions/" + strconv.FormatInt(posted.Transaction.ID, 10) + "/replace-return-of-capital"
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/preview", correction, http.StatusOK)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, correction, http.StatusCreated)
	var replaced capitalReturnReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&replaced))

	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/transactions/"+
		strconv.FormatInt(replaced.Replacement.Transaction.ID, 10)+"/correction-chain", nil, http.StatusOK)
	chain := res.Body.String()
	assertAPIExactEvidence(t, chain)
	assert.NotContains(t, chain, `"reference":"issuer notice"`)
}

func TestSplitCorrectionAPIPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "SPLITEV")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "10", 10000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/splits", investmentSplitRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID,
			CommodityID: instrument.CommodityID, RatioNumerator: 2, RatioDenominator: 1,
			SourceEvidence: json.RawMessage(apiExactSourceEvidence)}, http.StatusCreated)
	var split investmentSplitResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&split))
	base := "/api/v1/investments/transactions/" + strconv.FormatInt(split.Transaction.ID, 10)

	// The chain offers the recorded evidence for pre-filling the form.
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
	assertAPIExactEvidence(t, res.Body.String())

	// Unchanged terms with omitted or unchanged evidence are a no-op.
	same := investmentSplitReplacementRequest{Reason: "check", EffectiveOn: "2026-02-01", RatioNumerator: 2, RatioDenominator: 1}
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, base+"/replace-split/preview", same, http.StatusBadRequest)
	same.SourceEvidenceEdits = json.RawMessage(`{"reference":"issuer notice"}`)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, base+"/replace-split/preview", same, http.StatusBadRequest)

	// A reference edit alone is a correction and keeps every other token.
	edit := investmentSplitReplacementRequest{Reason: "notice amended", EffectiveOn: "2026-02-01", RatioNumerator: 2,
		RatioDenominator: 1, SourceEvidenceEdits: json.RawMessage(`{"reference":"amended notice"}`)}
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, base+"/replace-split/preview", edit, http.StatusOK)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/replace-split", edit, http.StatusCreated)
	var replaced investmentSplitReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&replaced))

	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/transactions/"+
		strconv.FormatInt(replaced.Replacement.ID, 10)+"/correction-chain", nil, http.StatusOK)
	chain := res.Body.String()
	assertAPIExactEvidence(t, chain)
	assert.Contains(t, chain, `"reference":"amended notice"`)
}
