package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

func TestCashInLieuAPIDisposesFractionAndNamesUnavailableSplit(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ICIL")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "3", 3000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	splitRequest := investmentSplitRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID,
		CommodityID: instrument.CommodityID, RatioNumerator: 3, RatioDenominator: 2}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/splits", splitRequest, http.StatusCreated)
	var split investmentSplitResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&split))

	path := "/api/v1/investments/cash-in-lieu"
	request := cashInLieuRequest{SettledTransactionID: split.Transaction.ID, DisposalOn: "2026-02-01", PaymentOn: "2026-02-05",
		QuantityValue: exact.New(5), QuantityScale: 1, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID,
		ProceedsValue: 800, ProceedsScale: 2}
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/preview", request, http.StatusOK)
	var preview cashInLieuPreviewResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&preview))
	require.Len(t, preview.Allocations, 1)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path, request, http.StatusForbidden)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusCreated)
	var posted investmentTradeResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&posted))
	require.NotNil(t, posted.DisposalDecision)
	require.Len(t, posted.Transaction.JournalEntries, 2)

	request.SettledTransactionID = posted.Transaction.ID
	refused := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusConflict)
	assert.Contains(t, refused.Body.String(), "INVESTMENT_CASH_IN_LIEU_SPLIT_UNAVAILABLE", "a non-split transaction is not a split")
}

func TestCashInLieuAPICorrectReverseAndPreviewValidParentReversal(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "CILC")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "3", 3000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	splitRequest := investmentSplitRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID, RatioNumerator: 3, RatioDenominator: 2}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/splits", splitRequest, http.StatusCreated)
	var split investmentSplitResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&split))
	input := cashInLieuRequest{SettledTransactionID: split.Transaction.ID, DisposalOn: "2026-02-01", PaymentOn: "2026-02-05", QuantityValue: exact.New(5), QuantityScale: 1, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, ProceedsValue: 800, ProceedsScale: 2}
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/cash-in-lieu/preview", input, http.StatusOK)
	var initialPreview cashInLieuPreviewResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&initialPreview))
	require.Zero(t, exact.ScaledIntFromCoefficient(*initialPreview.DisposedBasisValue, *initialPreview.DisposedBasisScale).Cmp(exact.ScaledIntFromInt64(3333333, 6)))
	require.Zero(t, exact.ScaledIntFromCoefficient(*initialPreview.RealizedGainValue, *initialPreview.RealizedGainScale).Cmp(exact.ScaledIntFromInt64(4666667, 6)))
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/cash-in-lieu", input, http.StatusCreated)
	var posted investmentTradeResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&posted))
	contextPath := fmt.Sprintf("/api/v1/investments/transactions/%d/trade-correction-context", posted.Transaction.ID)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, contextPath, nil, http.StatusOK)
	var source investmentTradeCorrectionContextResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&source))
	require.Equal(t, split.Transaction.ID, source.SettledTransactionID)
	require.False(t, source.CanReplaceSale, "generic sale correction must not be offered for cash in lieu")
	require.Len(t, source.AvailableLots, 1)
	correctionPath := fmt.Sprintf("/api/v1/investments/transactions/%d/replace-cash-in-lieu", posted.Transaction.ID)
	input.QuantityValue, input.QuantityScale, input.ProceedsValue, input.DisposalOn, input.PaymentOn = exact.New(25), 2, 850, "2026-02-02", "2026-02-08"
	input.CostBasisMethod = "specific_lot"
	input.LotAllocations = []investmentLotAllocationRequest{{LotID: source.AvailableLots[0].LotID, QuantityValue: input.QuantityValue, QuantityScale: input.QuantityScale}}
	replacement := cashInLieuReplacementRequest{Reason: "fraction and payment corrected", Replacement: input}
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, correctionPath+"/preview", replacement, http.StatusOK)
	var preview cashInLieuPreviewResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&preview))
	require.NotNil(t, preview.Impact.GainImpact)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, correctionPath, replacement, http.StatusForbidden)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, correctionPath, replacement, http.StatusConflict)
	replacement.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, correctionPath, replacement, http.StatusCreated)
	var corrected investmentSaleReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&corrected))
	require.Equal(t, preview.Allocations, corrected.Replacement.Allocations)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, correctionPath, replacement, http.StatusConflict)
	// A valid parent reversal previews through the effective successor disposal.
	splitReversePath := fmt.Sprintf("/api/v1/investments/transactions/%d/reverse-split/reconciliation-impact", split.Transaction.ID)
	reason := investmentSaleReversalRequest{Reason: "withdrawn"}
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, splitReversePath, reason, http.StatusOK)
	chainPath := fmt.Sprintf("/api/v1/investments/transactions/%d/correction-chain", corrected.Replacement.Transaction.ID)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, chainPath, nil, http.StatusOK)
	var chain investmentCorrectionChainResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&chain))
	require.True(t, chain.CanCorrectCashInLieu)
	reversePath := fmt.Sprintf("/api/v1/investments/transactions/%d/reverse-cash-in-lieu", corrected.Replacement.Transaction.ID)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, reversePath+"/reconciliation-impact", reason, http.StatusOK)
	var impact reconciliationImpactResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&impact))
	if impact.GainImpact != nil {
		reason.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, reversePath, reason, http.StatusForbidden)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, reversePath, reason, http.StatusCreated)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, splitReversePath, investmentSaleReversalRequest{Reason: "withdrawn"}, http.StatusOK)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, fmt.Sprintf("/api/v1/investments/transactions/%d/reverse-cash-in-lieu", split.Transaction.ID), reason, http.StatusNotFound)
}

func TestHistoricalCashInLieuAPIEntryPreviewCommitAllParents(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		for _, reversed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reversed=%t", kind, reversed), func(t *testing.T) {
				handler, _ := newSetupTestHandler(t)
				f := bootstrapInvestmentAPITest(t, handler)
				old := createInstrumentForSession(t, handler, f, "HISTOLD")
				holding := createHoldingAccountForSession(t, handler, f, old.ID)
				buy := tradeRequestBody(f, holding.ID, old.CommodityID, "3", 3000)
				buy.TransactionDate = "2026-01-01"
				doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
				destination, destinationCommodity := holding.ID, old.CommodityID
				if kind != "split" {
					security := createInstrumentForSession(t, handler, f, "HISTNEW")
					destination = createHoldingAccountForSession(t, handler, f, security.ID).ID
					destinationCommodity = security.CommodityID
					if reversed {
						independent := tradeRequestBody(f, destination, destinationCommodity, "2", 2000)
						independent.TransactionDate = "2026-01-02"
						doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", independent, http.StatusCreated)
					}
				}
				terms := map[string]any{"effective_on": "2026-02-01", "holding_account_id": holding.ID, "commodity_id": old.CommodityID, "ratio_numerator": 3, "ratio_denominator": 2}
				resource := map[string]string{"split": "splits", "share_exchange": "share-exchanges", "spin_off": "spin-offs", "cash_and_stock_merger": "cash-and-stock-mergers"}[kind]
				action := map[string]string{"split": "split", "share_exchange": "share-exchange", "spin_off": "spin-off", "cash_and_stock_merger": "cash-and-stock-merger"}[kind]
				if kind != "split" {
					terms["destination_holding_account_id"], terms["destination_commodity_id"] = destination, destinationCommodity
				}
				if kind == "spin_off" {
					terms["basis_fraction_value"], terms["basis_fraction_scale"] = "2", 1
				}
				if kind == "cash_and_stock_merger" {
					terms["payable_on"], terms["cash_account_id"], terms["currency_id"] = "2026-02-05", f.cashAccount.ID, f.commodityID
					terms["cash_value"], terms["cash_scale"] = "300", 2
					terms["cash_basis_fraction_value"], terms["cash_basis_fraction_scale"] = "25", 2
				}
				res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/"+resource, terms, http.StatusCreated)
				var parent struct {
					Transaction transactionResponse `json:"transaction"`
				}
				require.NoError(t, json.NewDecoder(res.Body).Decode(&parent))
				change := map[string]any{"reason": "Correct original action"}
				changePath := fmt.Sprintf("/api/v1/investments/transactions/%d/", parent.Transaction.ID)
				if reversed {
					changePath += "reverse-" + action
					res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, changePath+"/reconciliation-impact", change, http.StatusOK)
					var impact reconciliationImpactResponse
					require.NoError(t, json.NewDecoder(res.Body).Decode(&impact))
					if impact.GainImpact != nil {
						change["gain_impact_acknowledgement"] = impact.GainImpact.Acknowledgement
					}
				} else {
					for key, value := range terms {
						if key != "holding_account_id" && key != "commodity_id" {
							change[key] = value
						}
					}
					change["ratio_numerator"] = 5
					changePath += "replace-" + action
				}
				doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, changePath, change, http.StatusCreated)
				chainPath := fmt.Sprintf("/api/v1/investments/transactions/%d/correction-chain", parent.Transaction.ID)
				res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, chainPath, nil, http.StatusOK)
				var chain investmentCorrectionChainResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&chain))
				require.NotNil(t, chain.CashInLieuParent)
				require.True(t, chain.CashInLieuParent.Historical)
				require.Equal(t, destination, chain.CashInLieuParent.HoldingAccountID)
				require.Equal(t, destinationCommodity, chain.CashInLieuParent.CommodityID)
				parentID := chain.CashInLieuParent.OperationID
				lotPath := fmt.Sprintf("/api/v1/investments/operations/%d/cash-in-lieu-lots?currency_id=%d&disposal_on=2026-02-01", parentID, f.commodityID)
				res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, lotPath, nil, http.StatusOK)
				var lots cashInLieuLotsResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&lots))
				require.Len(t, lots.Lots, 1)
				input := cashInLieuRequest{SettledOperationID: parentID, DisposalOn: "2026-02-01", PaymentOn: "2026-02-05", QuantityValue: exact.New(5), QuantityScale: 1, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, ProceedsValue: 800, ProceedsScale: 2, CostBasisMethod: "specific_lot", LotAllocations: []investmentLotAllocationRequest{{LotID: lots.Lots[0].LotID, QuantityValue: exact.New(5), QuantityScale: 1}}}
				res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/cash-in-lieu/preview", input, http.StatusOK)
				var preview cashInLieuPreviewResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&preview))
				expected := map[string]int64{"split": 200, "share_exchange": 200, "spin_off": 40, "cash_and_stock_merger": 150}[kind]
				if reversed {
					expected = 500
				}
				require.Zero(t, exact.ScaledIntFromCoefficient(*preview.DisposedBasisValue, *preview.DisposedBasisScale).Cmp(exact.ScaledIntFromInt64(expected, 2)))
				require.Zero(t, exact.ScaledIntFromCoefficient(*preview.RealizedGainValue, *preview.RealizedGainScale).Cmp(exact.ScaledIntFromInt64(800-expected, 2)))
				res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/cash-in-lieu", input, http.StatusCreated)
				var posted investmentTradeResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&posted))
				require.Equal(t, preview.Allocations, posted.Allocations)
				res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, fmt.Sprintf("/api/v1/investments/transactions/%d/trade-correction-context", posted.Transaction.ID), nil, http.StatusOK)
				var source investmentTradeCorrectionContextResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&source))
				require.Equal(t, parentID, source.SettledOperationID)
				// An inverse journal is never offered as a new provenance parent.
				require.Len(t, chain.Operations, 2)
				if reversed {
					res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, fmt.Sprintf("/api/v1/investments/transactions/%d/correction-chain", *chain.Operations[1].TransactionID), nil, http.StatusOK)
					var inverse investmentCorrectionChainResponse
					require.NoError(t, json.NewDecoder(res.Body).Decode(&inverse))
					require.Nil(t, inverse.CashInLieuParent)
				}
			})
		}
	}
}
