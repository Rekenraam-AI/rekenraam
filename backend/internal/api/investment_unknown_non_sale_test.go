package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestUnknownNonSaleAPIKeepsExactFactsAndNullResults(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"write-off", "cash-in-lieu", "return-of-capital"} {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			handler, _ := newSetupTestHandler(t)
			f := bootstrapInvestmentAPITest(t, handler)
			instrument := createInstrumentForSession(t, handler, f, "UNRESOLVED")
			holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
			in := externalTransferInRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID, BasisKnowledge: "unknown",
				CommodityID: instrument.CommodityID, QuantityValue: exact.New(2), CostCommodityID: f.commodityID}
			res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", in, http.StatusCreated)
			var transfer externalTransferInResponse
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &transfer))
			var request any
			switch family {
			case "write-off":
				request = investmentWriteOffRequest{TransactionDate: "2026-04-01", CommodityID: instrument.CommodityID,
					HoldingAccountID: holding.ID, QuantityValue: exact.New(2), Reason: "worthless delisting", CostBasisMethod: "fifo"}
			case "cash-in-lieu":
				split := map[string]any{"holding_account_id": holding.ID, "commodity_id": instrument.CommodityID,
					"effective_on": "2026-03-01", "ratio_numerator": 5, "ratio_denominator": 4, "source_evidence": map[string]string{"notice": "split"}}
				res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/splits", split, http.StatusCreated)
				var splitBody struct {
					OperationID int64 `json:"operation_id"`
				}
				require.NoError(t, json.Unmarshal(res.Body.Bytes(), &splitBody))
				request = cashInLieuRequest{SettledOperationID: splitBody.OperationID, DisposalOn: "2026-04-01", PaymentOn: "2026-04-10",
					QuantityValue: exact.New(5), QuantityScale: 1, CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID,
					ProceedsValue: 800, ProceedsScale: 2}
			case "return-of-capital":
				request = capitalReturnRequest{HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID,
					CashAccountID: f.cashAccount.ID, CurrencyID: f.commodityID, EffectiveOn: "2026-03-01", PaymentOn: "2026-03-10",
					AmountValue: exact.New(1000), AmountScale: 2}
			}
			path := "/api/v1/investments/" + family
			res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/preview", request, http.StatusOK)
			var preview map[string]any
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &preview))
			var result map[string]any
			var nullFields []string
			switch family {
			case "write-off":
				result = preview
				nullFields = []string{"realized_gain", "realized_gain_scale"}
			case "cash-in-lieu":
				result = preview
				nullFields = []string{"disposed_basis_value", "disposed_basis_scale", "realized_gain_value", "realized_gain_scale"}
			case "return-of-capital":
				result = preview["effects"].([]any)[0].(map[string]any)
				nullFields = []string{"reduction_value", "reduction_scale", "excess_value", "excess_scale"}
				require.Equal(t, "2", result["entitled_quantity_value"])
				require.NotNil(t, result["allocated_value"])
			}
			require.Equal(t, "unknown", result["basis_knowledge"])
			for _, key := range nullFields {
				require.Contains(t, result, key)
				require.Nil(t, result[key], key)
			}
			doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path, request, http.StatusForbidden)
			res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusCreated)
			var posted map[string]any
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &posted))
			if family == "return-of-capital" {
				require.Equal(t, preview["effects"], posted["effects"])
			} else {
				decision := posted["disposal_decision"].(map[string]any)
				require.Equal(t, "unknown", decision["basis_knowledge"])
				require.Nil(t, decision["disposed_basis_value"])
				require.Nil(t, decision["disposed_basis_scale"])
				if family == "write-off" {
					require.Equal(t, "0", decision["proceeds_value"])
				}
			}
			resolvePath := fmt.Sprintf("/api/v1/investments/transactions/%d/resolve-basis", transfer.Transaction.ID)
			resolution := map[string]any{"basis_value": "8000", "basis_scale": 2, "reason": "statement found", "source_evidence": map[string]string{"kind": "statement", "reference": "old broker"}}
			res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, resolvePath+"/reconciliation-impact", resolution, http.StatusOK)
			var impact reconciliationImpactResponse
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &impact))
			if family != "return-of-capital" {
				require.Len(t, impact.GainImpact.Changes, 1)
				require.Equal(t, "unknown", impact.GainImpact.Changes[0].Before.BasisKnowledge)
				require.Equal(t, "known", impact.GainImpact.Changes[0].After.BasisKnowledge)
				resolution["gain_impact_acknowledgement"] = impact.GainImpact.Acknowledgement
			}
			doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, resolvePath, resolution, http.StatusCreated)
		})
	}
}
