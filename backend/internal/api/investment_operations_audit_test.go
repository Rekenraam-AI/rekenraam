package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// #272/#313: the reference book's intentional unknown basis is disclosed as
// incomplete coverage — neither an all-green pass nor a failure. Check the
// actual HTTP consumer, its persisted reread, and every independent register
// figure.
func TestReferenceBookInvestmentAuditSelfCheckAndRegister(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	seeded := seedReferenceBook(t, handler)
	assertReferenceBookRegister(t, handler, seeded)
	response := seeded.request(t, handler, http.MethodPost, "/api/v1/maintenance/self-check", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var run selfCheckRunResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &run))
	requireReferenceBookIncomplete(t, run)

	response = seeded.request(t, handler, http.MethodGet, "/api/v1/maintenance/self-check", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var status selfCheckStatusResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &status))
	require.True(t, status.HasRun)
	require.NotNil(t, status.LatestRun)
	require.Equal(t, run.ID, status.LatestRun.ID)
	requireReferenceBookIncomplete(t, *status.LatestRun)
	assertReferenceBookRegister(t, handler, seeded)
}

func requireReferenceBookIncomplete(t *testing.T, run selfCheckRunResponse) {
	t.Helper()
	require.Equal(t, "incomplete", run.Status)
	require.EqualValues(t, 0, run.FailedCheckCount)
	require.EqualValues(t, 1, run.IncompleteCheckCount)
	require.Len(t, run.Results, 13)
	for _, result := range run.Results {
		switch result.CheckID {
		case "lot_reconciliation":
			require.Equal(t, "incomplete", result.Status)
			require.EqualValues(t, 0, result.FindingCount)
			require.Empty(t, result.Sample)
			require.EqualValues(t, 1, result.UnavailableCount)
			require.Len(t, result.UnavailableSample, 1)
			require.Equal(t, "1 lots have unknown projected basis; basis reconciliation is unavailable", result.Summary)
		case "attachments":
			require.Equal(t, "not_applicable", result.Status)
			require.EqualValues(t, 0, result.UnavailableCount)
		default:
			require.Equal(t, "passed", result.Status, "%s: %s", result.CheckID, result.Summary)
			require.EqualValues(t, 0, result.UnavailableCount)
			require.NotNil(t, result.UnavailableSample)
		}
	}
}
