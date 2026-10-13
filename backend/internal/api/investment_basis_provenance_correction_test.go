package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func TestCorrectBasisProvenanceAPIAuthenticatesPreviewsAndPinsEvidenceRevision(t *testing.T) {
	t.Parallel()
	for _, basis := range []string{"8000", "0"} {
		t.Run(basis, func(t *testing.T) {
			handler, database := newSetupTestHandler(t)
			f := bootstrapInvestmentAPITest(t, handler)
			instrument := createInstrumentForSession(t, handler, f, "PROVENANCE")
			holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
			in := externalTransferInRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID,
				BasisKnowledge: "unknown", CommodityID: instrument.CommodityID, QuantityValue: exact.New(2), CostCommodityID: f.commodityID}
			res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", in, http.StatusCreated)
			var transfer externalTransferInResponse
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &transfer))
			base := "/api/v1/investments/transactions/" + strconv.FormatInt(transfer.Transaction.ID, 10)
			original := map[string]any{"kind": "statement", "reference": "Synthetic original statement"}
			request := map[string]any{"basis_value": basis, "basis_scale": 2, "source_evidence": original, "reason": "statement found"}
			doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base+"/resolve-basis", request, http.StatusCreated)
			res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
			var chain investmentCorrectionChainResponse
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &chain))
			require.Zero(t, chain.EffectiveBasisResolution.ProvenanceRevision)
			request["expected_resolution_operation_id"] = chain.EffectiveBasisResolution.OperationID
			request["expected_provenance_revision"] = 0
			updated := map[string]any{"kind": "provider", "reference": "Synthetic corrected provider row", "record_id": "42"}
			request["source_evidence"], request["reason"] = updated, "correct the evidence reference"
			path := base + "/replace-basis-resolution"
			snapshot := func() []int {
				var counts []int
				for _, table := range []string{"audit_events", "investment_basis_resolution_provenance", "transactions", "posting_versions", "investment_operations"} {
					var count int
					require.NoError(t, database.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count))
					counts = append(counts, count)
				}
				return counts
			}
			before := snapshot()
			doInvestmentRequest(t, handler, nil, "", http.MethodPost, path, request, http.StatusUnauthorized)
			doInvestmentRequest(t, handler, nil, "", http.MethodPost, path+"/reconciliation-impact", request, http.StatusUnauthorized)
			doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path, request, http.StatusForbidden)
			res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/reconciliation-impact", request, http.StatusOK)
			require.Contains(t, res.Body.String(), `"affected_checkpoints":[]`)
			require.Equal(t, before, snapshot(), "preview and security refusals write nothing")
			res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusCreated)
			var result investmentBasisResolutionReplacementResponse
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
			require.Nil(t, result.Inverse)
			require.Nil(t, result.Replacement)
			after := snapshot()
			require.Equal(t, before[0]+1, after[0])
			require.Equal(t, before[1]+1, after[1])
			require.Equal(t, before[2:], after[2:])
			doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusConflict)
			doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, path+"/reconciliation-impact", request, http.StatusConflict)
			require.Equal(t, after, snapshot())
			res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/correction-chain", nil, http.StatusOK)
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &chain))
			require.Equal(t, 1, chain.EffectiveBasisResolution.ProvenanceRevision)
			encoded, err := json.Marshal(updated)
			require.NoError(t, err)
			require.JSONEq(t, string(encoded), string(chain.EffectiveBasisResolution.SourceEvidence))
			require.Len(t, chain.BasisResolutions, 1)
			require.Len(t, chain.BasisResolutions[0].ProvenanceCorrections, 1)
			require.NotZero(t, chain.BasisResolutions[0].ProvenanceCorrections[0].ActorUserID)
			require.NotEmpty(t, chain.BasisResolutions[0].ProvenanceCorrections[0].RequestID)
			request["expected_provenance_revision"] = 1
			doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusBadRequest)
			request["source_evidence"] = map[string]any{"kind": "owner_assertion", "rationale": "records"}
			doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusBadRequest)
			require.Equal(t, after, snapshot())
		})
	}
}
