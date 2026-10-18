package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #273: prove stable HTTP refusals and zero financial writes together,
// including late acknowledgement failures after the actual writer replay.
func TestInvestmentAuditRefusalsPreserveExportedFinancialState(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	old := createInstrumentForSession(t, handler, f, "AUDIT273OLD")
	newInstrument := createInstrumentForSession(t, handler, f, "AUDIT273NEW")
	source := createHoldingAccountForSession(t, handler, f, old.ID)
	destination := createHoldingAccountForSession(t, handler, f, old.ID)
	newHolding := createHoldingAccountForSession(t, handler, f, newInstrument.ID)
	snapshot := func() map[string]string {
		t.Helper()
		var out bytes.Buffer
		require.NoError(t, app.NewExportService(db.NewExportRepository(database)).WriteBundle(context.Background(), &out, app.ExportFilter{}))
		archive, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
		require.NoError(t, err)
		files := map[string]string{}
		for _, file := range archive.File {
			if !strings.HasSuffix(file.Name, ".csv") {
				continue // the manifest's export time is not financial state
			}
			reader, err := file.Open()
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			files[file.Name] = string(data)
		}
		require.Contains(t, files, "investment-operations.csv")
		return files
	}
	refuse := func(path string, body any, status int, code string) {
		t.Helper()
		before := snapshot()
		response := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, body, status)
		var envelope errorResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
		require.Equal(t, code, envelope.Error.Code)
		require.Equal(t, before, snapshot(), "refusal preserves every exported CSV: %s", path)
	}
	buy := tradeRequestBody(f, source.ID, old.CommodityID, "3", 3000)
	buy.TransactionDate = "2026-01-01"
	var bought investmentTradeResponse
	require.NoError(t, json.NewDecoder(doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken,
		http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated).Body).Decode(&bought))
	for _, rejected := range []struct {
		path string
		body any
		code string
	}{
		{"splits", investmentSplitRequest{EffectiveOn: "2026-02-01", HoldingAccountID: source.ID,
			CommodityID: old.CommodityID, RatioNumerator: 1, RatioDenominator: 7}, "INVESTMENT_SPLIT_FRACTION_UNREPRESENTABLE"},
		{"share-exchanges", shareExchangeRequest{EffectiveOn: "2026-02-01", HoldingAccountID: source.ID,
			DestinationHoldingID: newHolding.ID, CommodityID: old.CommodityID, DestinationCommodityID: newInstrument.CommodityID,
			RatioNumerator: 1, RatioDenominator: 7}, "INVESTMENT_EXCHANGE_FRACTION_UNREPRESENTABLE"},
		{"spin-offs", spinOffRequest{EffectiveOn: "2026-02-01", HoldingAccountID: source.ID,
			DestinationHoldingID: newHolding.ID, CommodityID: old.CommodityID, DestinationCommodityID: newInstrument.CommodityID,
			RatioNumerator: 1, RatioDenominator: 7, BasisFractionValue: "25", BasisFractionScale: 2}, "INVESTMENT_SPIN_OFF_FRACTION_UNREPRESENTABLE"},
		{"cash-and-stock-mergers", cashStockMergerRequest{EffectiveOn: "2026-02-01", PayableOn: "2026-02-05", HoldingAccountID: source.ID,
			DestinationHoldingID: newHolding.ID, CommodityID: old.CommodityID, DestinationCommodityID: newInstrument.CommodityID,
			RatioNumerator: 1, RatioDenominator: 7, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID,
			CashValue: 100, CashScale: 2, CashBasisFractionValue: "25", CashBasisFractionScale: 2}, "INVESTMENT_EXCHANGE_FRACTION_UNREPRESENTABLE"},
	} {
		t.Run(rejected.path, func(t *testing.T) {
			refuse("/api/v1/investments/"+rejected.path+"/preview", rejected.body, http.StatusUnprocessableEntity, rejected.code)
			refuse("/api/v1/investments/"+rejected.path, rejected.body, http.StatusUnprocessableEntity, rejected.code)
		})
	}
	var split investmentSplitResponse
	require.NoError(t, json.NewDecoder(doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/splits", investmentSplitRequest{EffectiveOn: "2026-03-01", HoldingAccountID: source.ID,
			CommodityID: old.CommodityID, RatioNumerator: 2, RatioDenominator: 1}, http.StatusCreated).Body).Decode(&split))
	sale := tradeRequestBody(f, source.ID, old.CommodityID, "6", 9000)
	sale.TransactionDate = "2026-05-01"
	var sold investmentTradeResponse
	require.NoError(t, json.NewDecoder(doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken,
		http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated).Body).Decode(&sold))
	refuse(fmt.Sprintf("/api/v1/investments/transactions/%d/reverse-split", split.Transaction.ID),
		investmentSaleReversalRequest{Reason: "wrong ratio"}, http.StatusConflict, "INVESTMENT_SPLIT_DEPENDENCY")
	refuse("/api/v1/investments/transfers/internal", internalTransferRequest{EffectiveOn: "2026-04-01",
		SourceAccountID: source.ID, DestinationAccountID: destination.ID, CommodityID: old.CommodityID, CostCommodityID: f.commodityID,
		LotAllocations: []investmentLotAllocationRequest{{LotID: *bought.LotID, QuantityValue: exact.New(1)}}}, http.StatusConflict, "INVESTMENT_SALE_DEPENDENCY")
	refuse(fmt.Sprintf("/api/v1/investments/transactions/%d/reverse-buy", bought.Transaction.ID),
		investmentSaleReversalRequest{Reason: "duplicate buy"}, http.StatusConflict, "INVESTMENT_BUY_DEPENDENCY")
	for _, acknowledgement := range []struct{ token, code string }{
		{"", "INVESTMENT_GAIN_IMPACT_ACKNOWLEDGEMENT_REQUIRED"},
		{"stale", "INVESTMENT_GAIN_IMPACT_ACKNOWLEDGEMENT_STALE"},
	} {
		refuse(fmt.Sprintf("/api/v1/investments/transactions/%d/reverse-sale", sold.Transaction.ID),
			investmentSaleReversalRequest{Reason: "duplicate sale", GainImpactAcknowledgement: acknowledgement.token}, http.StatusConflict, acknowledgement.code)
	}
}
