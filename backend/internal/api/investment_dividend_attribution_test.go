package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDividendAttributionHTTPDistinguishesOmissionNullAndValueForEveryCashShape(t *testing.T) {
	t.Parallel()
	for _, shape := range []string{"dividend", "embedded_tax", "separate_tax"} {
		for _, entryNull := range []bool{false, true} {
			t.Run(shape+"/entry_null_"+strconv.FormatBool(entryNull), func(t *testing.T) {
				t.Parallel()
				handler, _ := newSetupTestHandler(t)
				f := bootstrapInvestmentAPITest(t, handler)
				instrument := createInstrumentForSession(t, handler, f, "PAYER")
				tax := createLedgerAccount(t, handler, f.sessionCookie, f.csrfToken, "Tax", "expense", "expense", f.commodityID, 2)
				financial := func() map[string]any {
					body := map[string]any{"transaction_date": "2026-04-01", "cash_account_id": f.cashAccount.ID,
						"cash_commodity_id": f.commodityID, "income_account_id": f.incomeAccount.ID, "amount_value": "10000", "amount_scale": 2}
					if shape == "embedded_tax" {
						body["withholding_value"], body["withholding_scale"], body["withholding_account_id"] = "1500", 2, tax.ID
					} else if shape == "separate_tax" {
						delete(body, "income_account_id")
						body["withholding_only"], body["amount_value"], body["withholding_account_id"] = true, "200", tax.ID
					}
					return body
				}
				body := financial()
				if entryNull {
					body["commodity_id"] = nil
				}
				res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", body, http.StatusCreated)
				var original transactionResponse
				require.NoError(t, json.NewDecoder(res.Body).Decode(&original))
				base := "/api/v1/investments/transactions/" + strconv.FormatInt(original.ID, 10)
				read := func() *investmentCorrectionDividendTerms {
					res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
					var chain investmentCorrectionChainResponse
					require.NoError(t, json.NewDecoder(res.Body).Decode(&chain))
					require.NotNil(t, chain.EffectiveDividend)
					return chain.EffectiveDividend
				}
				require.Nil(t, read().CommodityID)
				current := original.ID
				for _, step := range []string{"assign", "omit", "clear", "omit_after_clear"} {
					replacement := financial()
					if step == "assign" {
						replacement["commodity_id"] = instrument.CommodityID
					}
					if step == "clear" {
						replacement["commodity_id"] = nil
					}
					request := map[string]any{"reason": step, "replacement": replacement}
					path := "/api/v1/investments/transactions/" + strconv.FormatInt(current, 10) + "/replace-dividend"
					preview := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/reconciliation-impact", request, http.StatusOK)
					var impact reconciliationImpactResponse
					require.NoError(t, json.NewDecoder(preview.Body).Decode(&impact))
					require.Empty(t, impact.AffectedCheckpoints)
					res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusCreated)
					var result investmentDividendReplacementResponse
					require.NoError(t, json.NewDecoder(res.Body).Decode(&result))
					current = result.ReplacementTransaction.ID
					terms := read()
					require.Equal(t, shape == "separate_tax", terms.WithholdingOnly)
					if step == "assign" || step == "omit" {
						require.Equal(t, &instrument.CommodityID, terms.CommodityID)
					} else {
						require.Nil(t, terms.CommodityID)
					}
					require.NotNil(t, terms.Attribution)
					require.Equal(t, "passed", terms.Attribution.Status, "manual unassigned payer is coverage, not a new source claim")
				}
				invalid := financial()
				invalid["commodity_id"] = 0
				doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", invalid, http.StatusBadRequest)
			})
		}
	}
}
