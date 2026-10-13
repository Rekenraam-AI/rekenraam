package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestBasisResolutionDeclarationAPIRefusesIncompleteEvidence(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "SOURCE")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	in := externalTransferInRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID, BasisKnowledge: "unknown",
		CommodityID: instrument.CommodityID, QuantityValue: exact.New(2), CostCommodityID: f.commodityID}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", in, http.StatusCreated)
	var transfer externalTransferInResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &transfer))
	base := "/api/v1/investments/transactions/" + strconv.FormatInt(transfer.Transaction.ID, 10)
	snapshot := func() []int {
		counts := make([]int, 0)
		for _, table := range []string{"audit_events", "transactions", "transaction_versions", "posting_versions", "investment_operations", "investment_basis_resolutions", "investment_lot_events"} {
			var count int
			require.NoError(t, database.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count))
			counts = append(counts, count)
		}
		return counts
	}
	assertRefusals := func(path, basis string) {
		for _, tc := range []struct{ name, evidence string }{
			{"missing", ""}, {"null", "null"}, {"empty", "{}"},
			{"whitespace", `{"kind":"statement","reference":"  "}`},
			{"untyped", `{"reference":"statement"}`},
			{"case_alias_blank_reference", `{"kind":"statement","reference":"","Reference":"statement"}`},
			{"case_alias_missing_kind", `{"Kind":"statement","reference":"statement"}`},
			{"unicode_over_limit", `{"kind":"statement","reference":"` + strings.Repeat("я", 501) + `"}`},
			{"owner_missing_date", `{"kind":"owner_assertion","rationale":"purchase records"}`},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				request := map[string]any{"basis_value": basis, "basis_scale": 2, "reason": "correct basis from records"}
				if tc.evidence != "" {
					request["source_evidence"] = json.RawMessage(tc.evidence)
				}
				before := snapshot()
				for _, suffix := range []string{"/reconciliation-impact", ""} {
					res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+path+suffix, request, http.StatusBadRequest)
					require.Contains(t, res.Body.String(), "VALIDATION_FAILED")
					require.Equal(t, before, snapshot())
				}
			})
		}
	}
	assertRefusals("/resolve-basis", "8000")
	statement := map[string]any{"kind": "statement", "reference": "Synthetic " + strings.Repeat("я", 490)}
	request := map[string]any{"basis_value": "8000", "basis_scale": 2, "reason": "statement found", "source_evidence": statement}
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/resolve-basis/reconciliation-impact", request, http.StatusOK)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/resolve-basis", request, http.StatusCreated)
	assertRefusals("/replace-basis-resolution", "6000")
	owner := map[string]any{"kind": "owner_assertion", "rationale": "reconstructed from original purchase records", "known_on": "2026-10-13"}
	request["basis_value"], request["source_evidence"] = "6000", owner
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/replace-basis-resolution/reconciliation-impact", request, http.StatusOK)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/replace-basis-resolution", request, http.StatusCreated)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
	var chain map[string]any
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &chain))
	history := chain["basis_resolutions"].([]any)
	require.Len(t, history, 2)
	require.Equal(t, statement, history[0].(map[string]any)["source_evidence"])
	require.Equal(t, owner, history[1].(map[string]any)["source_evidence"])
}
