package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// #272: the reference book's intentional unknown basis is a disclosed
// self-check limitation, not evidence for claiming an all-green run. Check
// the actual HTTP consumer and reread every independent register figure.
func TestReferenceBookInvestmentAuditSelfCheckAndRegister(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	seeded := seedReferenceBook(t, handler)
	assertReferenceBookRegister(t, handler, seeded)
	response := seeded.request(t, handler, http.MethodPost, "/api/v1/maintenance/self-check", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var run selfCheckRunResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &run))
	require.Equal(t, "failed", run.Status)
	require.EqualValues(t, 1, run.FailedCheckCount)
	for _, result := range run.Results {
		if result.CheckID == "lot_reconciliation" {
			require.Equal(t, "failed", result.Status)
			require.EqualValues(t, 1, result.FindingCount)
			require.Contains(t, result.Summary, "1 lots have unknown projected basis; basis reconciliation is unavailable")
		} else if result.CheckID == "attachments" {
			require.Equal(t, "not_applicable", result.Status)
		} else {
			require.Equal(t, "passed", result.Status, "%s: %s", result.CheckID, result.Summary)
		}
	}
	assertReferenceBookRegister(t, handler, seeded)
}
