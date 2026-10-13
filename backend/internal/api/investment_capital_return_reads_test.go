package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestCapitalReturnReadAPIExcessRevisionReplacementAndReversal(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "EXCESS")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 700)
	buy.TransactionDate = "2026-01-01"
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	var bought investmentTradeResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &bought))
	request := capitalReturnRequest{HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID,
		CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, EffectiveOn: "2026-02-01", PaymentOn: "2026-02-10",
		AmountValue: exact.New(1000), AmountScale: 2, SourceEvidence: json.RawMessage(`{"notice":"ROC-171","exact":9007199254740993}`)}
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/return-of-capital", request, http.StatusCreated)
	var posted capitalReturnResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &posted))
	read := func() investmentGainsResponse {
		t.Helper()
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?from=2026-02-10&to=2026-02-10", nil, http.StatusOK)
		var gains investmentGainsResponse
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &gains))
		return gains
	}
	gains := read()
	require.Len(t, gains.CapitalReturns, 1)
	entry := gains.CapitalReturns[0]
	require.Equal(t, request.EffectiveOn, entry.EffectiveOn)
	require.Equal(t, request.PaymentOn, entry.PaymentOn)
	require.NotEmpty(t, entry.HoldingName)
	require.NotEmpty(t, entry.InstrumentName)
	require.NotEmpty(t, entry.CurrencyCode)
	require.Contains(t, entry.SourceEvidenceJSON, "9007199254740993", "preserve exact source tokens")
	require.True(t, entry.Effective)
	require.True(t, entry.HasExcess)
	require.Nil(t, entry.RevisionID)
	requireScaledAPIAmount(t, 300, 2, entry.ExcessValue, entry.ExcessScale)
	requireScaledAPIAmount(t, 700, 2, entry.ReductionValue, entry.ReductionScale)
	require.Empty(t, gains.Realized)
	require.Empty(t, gains.RealizedTotals)
	// Correct upstream basis twice: changed lot identities must never mix sets.
	for _, amount := range []string{"500", "1200"} {
		body := map[string]any{"reason": "basis corrected", "replacement": map[string]any{
			"transaction_date": "2026-01-01", "commodity_id": instrument.CommodityID, "holding_account_id": holding.ID,
			"cash_account_id": f.cashAccount.ID, "quantity_value": "1", "quantity_scale": 0,
			"cash_amount_value": amount, "cash_amount_scale": 2, "cash_commodity_id": f.commodityID,
		}}
		path := fmt.Sprintf("/api/v1/investments/transactions/%d/replace-buy", bought.Transaction.ID)
		res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, body, http.StatusCreated)
		var replaced investmentBuyReplacementResponse
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &replaced))
		bought = replaced.Replacement
	}
	gains = read()
	require.Len(t, gains.CapitalReturns, 1)
	require.Equal(t, 3, gains.CapitalReturns[0].RevisionSeq)
	require.False(t, gains.CapitalReturns[0].HasExcess)
	requireScaledAPIAmount(t, 0, 0, gains.CapitalReturns[0].ExcessValue, gains.CapitalReturns[0].ExcessScale)
	require.Equal(t, *bought.LotID, gains.CapitalReturns[0].Effects[0].LotID)
	chainPath := fmt.Sprintf("/api/v1/investments/transactions/%d/correction-chain", posted.Transaction.ID)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, chainPath, nil, http.StatusOK)
	var chain investmentCorrectionChainResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &chain))
	require.Len(t, chain.CapitalReturns, 3)
	requireScaledAPIAmount(t, 300, 2, chain.CapitalReturns[0].ExcessValue, chain.CapitalReturns[0].ExcessScale)
	requireScaledAPIAmount(t, 500, 2, chain.CapitalReturns[1].ExcessValue, chain.CapitalReturns[1].ExcessScale)
	require.False(t, chain.CapitalReturns[0].Effective)
	require.True(t, chain.CapitalReturns[2].Effective)
	request.AmountValue = exact.New(1500)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		fmt.Sprintf("/api/v1/investments/transactions/%d/replace-return-of-capital", posted.Transaction.ID),
		capitalReturnReplacementRequest{Reason: "receipt correction", Replacement: request}, http.StatusCreated)
	var corrected capitalReturnReplacementResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &corrected))
	gains = read()
	require.Len(t, gains.CapitalReturns, 1)
	require.Equal(t, corrected.Replacement.Transaction.ID, gains.CapitalReturns[0].TransactionID)
	requireScaledAPIAmount(t, 300, 2, gains.CapitalReturns[0].ExcessValue, gains.CapitalReturns[0].ExcessScale)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		fmt.Sprintf("/api/v1/investments/transactions/%d/reverse-return-of-capital", corrected.Replacement.Transaction.ID),
		map[string]any{"reason": "withdrawn notice"}, http.StatusCreated)
	require.Empty(t, read().CapitalReturns)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, chainPath, nil, http.StatusOK)
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &chain))
	require.Len(t, chain.CapitalReturns, 4)
	for _, entry := range chain.CapitalReturns {
		require.False(t, entry.Effective)
	}
}

func requireScaledAPIAmount(t *testing.T, value int64, scale int, actual *exact.Coefficient, actualScale *int) {
	t.Helper()
	require.NotNil(t, actual)
	require.NotNil(t, actualScale)
	require.Zero(t, exact.ScaledIntFromCoefficient(*actual, *actualScale).Cmp(exact.ScaledIntFromInt64(value, scale)))
}

func TestCapitalReturnReadAPIUnknownIsNullRatherThanZero(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "UNKNOWNROC")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	in := externalTransferInRequest{EffectiveOn: "2026-01-01", HoldingAccountID: holding.ID, BasisKnowledge: "unknown",
		CommodityID: instrument.CommodityID, QuantityValue: exact.New(2), CostCommodityID: f.commodityID}
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", in, http.StatusCreated)
	request := capitalReturnRequest{HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID,
		CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, EffectiveOn: "2026-02-01", PaymentOn: "2026-02-10", AmountValue: exact.New(1000), AmountScale: 2}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/return-of-capital", request, http.StatusCreated)
	var posted capitalReturnResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &posted))
	for _, path := range []string{"/api/v1/investments/gains", fmt.Sprintf("/api/v1/investments/transactions/%d/correction-chain", posted.Transaction.ID)} {
		res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path, nil, http.StatusOK)
		var body struct {
			CapitalReturns []capitalReturnReadResponse `json:"capital_returns"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		require.Len(t, body.CapitalReturns, 1)
		entry := body.CapitalReturns[0]
		require.Equal(t, 1, entry.UnknownLots)
		require.Nil(t, entry.ReductionValue)
		require.Nil(t, entry.ReductionScale)
		require.Nil(t, entry.ExcessValue)
		require.Nil(t, entry.ExcessScale)
		require.Nil(t, entry.Effects[0].ExcessValue)
		require.Equal(t, "unknown", entry.Effects[0].BasisKnowledge)
		require.NotEmpty(t, entry.Effects[0].AllocatedValue)
	}
}
