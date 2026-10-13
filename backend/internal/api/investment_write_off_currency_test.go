package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestWriteOffCostCurrencyAPIPreviewsCommitsAndReplacesMixedPosition(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "WOMIX")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	eur := createCurrencyForSession(t, handler, f.sessionCookie, f.csrfToken, `{"code":"EUR","name":"Euro"}`)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "3", 3000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	basis := moneyCoefficient(4000)
	transfer := externalTransferInRequest{EffectiveOn: "2026-01-02", HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID,
		QuantityValue: exact.New(2), CostCommodityID: eur.ID, CarriedBasisValue: &basis, CarriedBasisScale: 2}
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", transfer, http.StatusCreated)
	input := investmentWriteOffRequest{TransactionDate: "2026-02-01", CommodityID: instrument.CommodityID,
		HoldingAccountID: holding.ID, CostCommodityID: eur.ID, QuantityValue: exact.New(1), Reason: "issuer liquidated"}
	var countBefore, countAfter int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&countBefore))
	preview := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/write-off/preview", input, http.StatusOK)
	var planned sellPreviewResponse
	require.NoError(t, json.NewDecoder(preview.Body).Decode(&planned))
	require.Equal(t, eur.ID, planned.DisposalDecision.CostCommodityID)
	require.Zero(t, exact.ScaledIntFromInt64(-2000, 2).Cmp(exact.ScaledIntFromInt64(int64(*planned.RealizedGain), *planned.RealizedGainScale)))
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/write-off/reconciliation-impact", input, http.StatusOK)
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&countAfter))
	require.Equal(t, countBefore, countAfter)
	committed := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/write-off", input, http.StatusCreated)
	var written investmentTradeResponse
	require.NoError(t, json.NewDecoder(committed.Body).Decode(&written))
	require.Equal(t, eur.ID, written.DisposalDecision.CostCommodityID)
	base := "/api/v1/investments/transactions/" + strconv.FormatInt(written.Transaction.ID, 10)
	context := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/trade-correction-context", nil, http.StatusOK)
	var correction investmentTradeCorrectionContextResponse
	require.NoError(t, json.NewDecoder(context.Body).Decode(&correction))
	require.Equal(t, eur.ID, correction.CostCommodityID)
	input.CostCommodityID = f.commodityID
	input.CostBasisMethod = "fifo"
	replace := investmentWriteOffReplacementRequest{Reason: "the dollar position was liquidated", Replacement: input}
	preview = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, base+"/replace-write-off/reconciliation-impact", replace, http.StatusOK)
	var impact reconciliationImpactResponse
	require.NoError(t, json.NewDecoder(preview.Body).Decode(&impact))
	require.Len(t, impact.GainImpact.Changes, 1)
	require.Equal(t, eur.ID, impact.GainImpact.Changes[0].Before.CostCommodityID)
	require.Equal(t, f.commodityID, impact.GainImpact.Changes[0].After.CostCommodityID)
	replace.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	committed = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/replace-write-off", replace, http.StatusCreated)
	var replaced investmentSaleReplacementResponse
	require.NoError(t, json.NewDecoder(committed.Body).Decode(&replaced))
	require.Equal(t, f.commodityID, replaced.Replacement.DisposalDecision.CostCommodityID)
}

func TestWriteOffCostCurrencyAPIRejectsAmbiguityAndCrossCurrencyElectionAtomically(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "WOREJECT")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	eur := createCurrencyForSession(t, handler, f.sessionCookie, f.csrfToken, `{"code":"EUR","name":"Euro"}`)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 2000)
	buy.TransactionDate = "2026-01-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	buy.CashCommodityID = eur.ID
	euroCash := createLedgerAccount(t, handler, f.sessionCookie, f.csrfToken, "Euro cash", "asset", "checking", eur.ID, 2)
	buy.CashAccountID = euroCash.ID
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	var euroBuy investmentTradeResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&euroBuy))
	input := investmentWriteOffRequest{TransactionDate: "2026-02-01", CommodityID: instrument.CommodityID,
		HoldingAccountID: holding.ID, QuantityValue: exact.New(1), Reason: "delisted"}
	var before, after string
	snapshot := `SELECT json_array((SELECT count(*) FROM transactions), (SELECT count(*) FROM audit_events),
		(SELECT count(*) FROM investment_lot_events), (SELECT count(*) FROM investment_disposal_decisions),
		(SELECT group_concat(remaining_quantity_value) FROM investment_lot_state))`
	require.NoError(t, database.QueryRow(snapshot).Scan(&before))
	for _, suffix := range []string{"/preview", "/reconciliation-impact", ""} {
		res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/write-off"+suffix, input, http.StatusBadRequest)
		require.Contains(t, res.Body.String(), "multiple currencies")
	}
	input.CostCommodityID = f.commodityID
	input.CostBasisMethod = "specific_lot"
	input.LotAllocations = []investmentLotAllocationRequest{{LotID: *euroBuy.LotID, QuantityValue: exact.New(1)}}
	for _, suffix := range []string{"/preview", "/reconciliation-impact", ""} {
		res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/write-off"+suffix, input, http.StatusBadRequest)
		require.Contains(t, res.Body.String(), "VALIDATION_FAILED")
	}
	require.NoError(t, database.QueryRow(snapshot).Scan(&after))
	require.Equal(t, before, after)
}

func TestWriteOffCostCurrencyAPISelectsClosedHistoricalLot(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "WOHISTORY")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	eur := createCurrencyForSession(t, handler, f.sessionCookie, f.csrfToken, `{"code":"EUR","name":"Euro"}`)
	buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 2000)
	buy.TransactionDate = "2026-01-01"
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	var first investmentTradeResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&first))
	buy.TransactionDate, buy.CashAmountValue = "2026-01-02", moneyCoefficient(6000)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	buy.CashCommodityID = eur.ID
	euroCash := createLedgerAccount(t, handler, f.sessionCookie, f.csrfToken, "Euro cash", "asset", "checking", eur.ID, 2)
	buy.CashAccountID = euroCash.ID
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "3", 9000)
	sale.TransactionDate = "2026-04-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)
	input := investmentWriteOffRequest{TransactionDate: "2026-03-01", CommodityID: instrument.CommodityID,
		HoldingAccountID: holding.ID, CostCommodityID: f.commodityID, QuantityValue: exact.New(1), Reason: "delisted",
		CostBasisMethod: "specific_lot", LotAllocations: []investmentLotAllocationRequest{{LotID: *first.LotID, QuantityValue: exact.New(1)}}}
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/write-off/preview", input, http.StatusOK)
	var preview sellPreviewResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&preview))
	require.Equal(t, *first.LotID, preview.Allocations[0].LotID)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, "/api/v1/investments/write-off/reconciliation-impact", input, http.StatusOK)
	var impact reconciliationImpactResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&impact))
	require.NotEmpty(t, impact.GainImpact.Changes)
	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/write-off", input, http.StatusCreated)
	var written investmentTradeResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&written))
	require.Equal(t, input.CostCommodityID, written.DisposalDecision.CostCommodityID)
	require.Equal(t, preview.Allocations, written.Allocations)
}
