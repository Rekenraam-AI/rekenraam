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

// #181: the cash-and-stock merger endpoints, their preview and the named
// refusals. 10 shares bought for 100.00 become 5 new shares and 30.00 of cash
// at a 25 % election: 25.00 is realized, 75.00 carried.
func TestCashStockMergerAPIPreviewCommitAndNamedRefusals(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	old := createInstrumentForSession(t, handler, f, "OLDCO")
	successor := createInstrumentForSession(t, handler, f, "NEWCO")
	holding := createHoldingAccountForSession(t, handler, f, old.ID)
	successorHolding := createHoldingAccountForSession(t, handler, f, successor.ID)
	buy := tradeRequestBody(f, holding.ID, old.CommodityID, "10", 10000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/buy", buy, http.StatusCreated)

	request := cashStockMergerRequest{EffectiveOn: "2026-02-01", PayableOn: "2026-02-03", HoldingAccountID: holding.ID,
		DestinationHoldingID: successorHolding.ID, CommodityID: old.CommodityID,
		DestinationCommodityID: successor.CommodityID, RatioNumerator: 2, RatioDenominator: 4,
		CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, CashValue: 3000, CashScale: 2,
		CashBasisFractionValue: "250", CashBasisFractionScale: 3, SourceEvidence: json.RawMessage(`{"notice":"merger"}`)}
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost,
		"/api/v1/investments/cash-and-stock-mergers/preview", request, http.StatusOK)
	var preview cashStockMergerPreviewResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&preview))
	assert.Equal(t, [2]int64{1, 2}, [2]int64{preview.Plan.RatioNumerator, preview.Plan.RatioDenominator}, "stored in lowest terms")
	assert.Equal(t, "25", preview.Plan.CashBasisFractionValue.String(), "the election in lowest terms")
	assert.Equal(t, 2, preview.Plan.CashBasisFractionScale)
	require.Len(t, preview.Plan.Links, 1)
	assert.Nil(t, preview.Plan.Links[0].DestinationLotID)
	require.Len(t, preview.Plan.Totals, 1)
	require.NotNil(t, preview.Plan.Totals[0].ResultValue)

	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/cash-and-stock-mergers", request, http.StatusForbidden)
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/cash-and-stock-mergers", request, http.StatusCreated)
	var merged cashStockMergerResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&merged))
	require.Len(t, merged.Transaction.JournalEntries, 2)
	assert.Equal(t, "2026-02-01", merged.Transaction.JournalEntries[0].EntryDate)
	assert.Equal(t, "2026-02-03", merged.Transaction.JournalEntries[1].EntryDate)
	link := merged.Plan.Links[0]
	require.NotNil(t, link.DestinationLotID)
	assert.Equal(t, "5", link.DestinationQuantityValue.String())
	require.NotNil(t, link.RealizedBasisValue)
	require.NotNil(t, link.CarriedBasisValue)
	total := merged.Plan.Totals[0]
	assert.Zero(t, exact.ScaledIntFromCoefficient(total.CashValue, total.CashScale).Cmp(exact.ScaledIntFromInt64(3000, 2)),
		"the shares total the cash, at the allocation scale")
	require.NotNil(t, total.ResultValue)
	assert.Equal(t, preview.Plan.Totals[0].ResultValue, total.ResultValue)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, fmt.Sprintf(
		"/api/v1/transactions/%d", merged.Transaction.ID), nil, http.StatusOK)
	assert.Contains(t, res.Body.String(), `"system_label":"cash_and_stock_merger"`)

	// Nothing of the old instrument is left to merge again; cash before the
	// effective date and cash outside the lots' cost currency are refused.
	request.EffectiveOn, request.PayableOn = "2026-03-01", "2026-03-01"
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/cash-and-stock-mergers", request, http.StatusConflict)
	assert.Contains(t, res.Body.String(), "INVESTMENT_EXCHANGE_NO_HOLDINGS")
	request.PayableOn = "2026-02-15"
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/cash-and-stock-mergers", request, http.StatusBadRequest)
	assert.Contains(t, res.Body.String(), "VALIDATION_FAILED")

	// A merger dated behind a later sale of the new instrument is out of order.
	sell := tradeRequestBody(f, successorHolding.ID, successor.CommodityID, "1", 2000)
	sell.TransactionDate = "2026-04-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/sell", sell, http.StatusCreated)
	buy.TransactionDate = "2026-02-10"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/buy", buy, http.StatusCreated)
	backdated := cashStockMergerRequest{EffectiveOn: "2026-02-15", PayableOn: "2026-02-15", HoldingAccountID: holding.ID,
		DestinationHoldingID: successorHolding.ID, CommodityID: old.CommodityID,
		DestinationCommodityID: successor.CommodityID, RatioNumerator: 1, RatioDenominator: 1,
		CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, CashValue: 100, CashScale: 2,
		CashBasisFractionValue: "0", CashBasisFractionScale: 0}
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/cash-and-stock-mergers", backdated, http.StatusConflict)
	assert.Contains(t, res.Body.String(), "INVESTMENT_EVENT_OUT_OF_ORDER")
}
