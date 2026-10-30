package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttentionAuthenticationEmptyReadPurityAndPartialFailure(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/attention", nil))
	require.Equal(t, http.StatusUnauthorized, res.Code)
	cookie, _, _ := setupAccountAPITest(t, handler)
	var before, after int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&before))
	res = doInvestmentRequest(t, handler, cookie, "", http.MethodGet, "/api/v1/attention", nil, http.StatusOK)
	assert.Equal(t, "no-store", res.Header().Get("Cache-Control"))
	var result attentionResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
	assert.True(t, result.Complete)
	assert.False(t, result.HasFailures)
	assert.Zero(t, result.TotalKnown)
	require.Len(t, result.Sources, 11)
	for _, source := range result.Sources {
		require.NotNil(t, source.Count, source.ID)
		assert.Zero(t, *source.Count, source.ID)
		assert.NotEmpty(t, source.Unit, source.ID)
	}
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&after))
	assert.Equal(t, before, after)
	// One failed source must leave healthy zeroes and an explicit null/error.
	_, err := database.Exec(`DROP TABLE pricing_refresh_state`)
	require.NoError(t, err)
	res = doInvestmentRequest(t, handler, cookie, "", http.MethodGet, "/api/v1/attention", nil, http.StatusOK)
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
	assert.False(t, result.Complete)
	assert.True(t, result.HasFailures)
	for _, source := range result.Sources {
		if source.ID == "refresh" {
			assert.Nil(t, source.Count)
			assert.Equal(t, "error", source.State)
			assert.Equal(t, "INTERNAL_ERROR", source.ErrorCode)
		} else {
			assert.Equal(t, "ready", source.State, source.ID)
		}
	}
	assert.NotContains(t, res.Body.String(), "no such table")
}

func TestAttentionCountsBatchesOnceAndLatestRunRecovery(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	cookie, _, _ := setupAccountAPITest(t, handler)
	_, err := database.Exec(`INSERT INTO import_batches (id, book_id, source_kind, status, created_at) VALUES
 (901,1,'csv','previewing','2026-01-01T00:00:00Z'),
 (902,1,'csv','discarded','2026-01-01T00:00:00Z'),
 (903,1,'csv','previewing','2026-01-01T00:00:00Z'),
 (904,1,'csv','failed','2026-01-01T00:00:00Z');
 INSERT INTO import_staged_rows (batch_id,book_id,row_index,dedupe_fingerprint,dedupe_status) VALUES
 (901,1,0,'one','new'),(901,1,1,'two','needs_attention'),
 (902,1,0,'three','new'),(903,1,0,'four','duplicate');
 INSERT INTO self_check_runs (book_id,trigger,status,failed_check_count,started_at,created_at)
 VALUES (1,'manual','failed',1,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');`)
	require.NoError(t, err)
	read := func() attentionResponse {
		res := doInvestmentRequest(t, handler, cookie, "", http.MethodGet, "/api/v1/attention", nil, http.StatusOK)
		var result attentionResponse
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
		return result
	}
	result := read()
	assert.Equal(t, 3, result.TotalKnown) // Two batches, one latest failed check.
	assert.True(t, result.HasFailures)
	assert.True(t, result.Complete)
	_, err = database.Exec(`INSERT INTO self_check_runs (book_id,trigger,status,started_at,created_at) VALUES (1,'manual','passed','2026-01-02T00:00:00Z','2026-01-02T00:00:00Z')`)
	require.NoError(t, err)
	result = read()
	assert.Equal(t, 2, result.TotalKnown)
	assert.True(t, result.HasFailures) // The failed import still needs attention.
}

func TestAttentionCountsTransactionIdentitiesAcrossPostingRowsAndVersions(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	cookie, csrf, currencyID := setupAccountAPITest(t, handler)
	account := createLedgerAccount(t, handler, cookie, csrf, "Count checking", "asset", "checking", currencyID, 2)
	category := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Count expense","category_type":"expense"}`)
	draft := generateRecurringDraft(t, handler, database, cookie, "2026-06-07", account.ID, category.ID, currencyID, 100)
	// Editing creates another durable version and two more posting rows, but
	// the action remains one draft transaction.
	mutateTransaction(t, handler, cookie, csrf, http.MethodPatch, "/api/v1/transactions/"+strconvFormatInt(draft.ID), balancedBody("2026-06-07", posting(account.ID, -200, 2, currencyID), posting(category.ID, 200, 2, currencyID)), http.StatusOK)
	res := doInvestmentRequest(t, handler, cookie, "", http.MethodGet, "/api/v1/attention", nil, http.StatusOK)
	var result attentionResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
	assert.Equal(t, 1, result.TotalKnown)
	for _, source := range result.Sources {
		if source.ID == "drafts" {
			require.NotNil(t, source.Count)
			assert.Equal(t, 1, *source.Count)
		}
	}
}

func TestAttentionRefreshFailuresDeduplicateSourcesAndClearOnRecovery(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	cookie, _, currencyID := setupAccountAPITest(t, handler)
	_, err := database.Exec(`INSERT INTO pricing_refresh_state (book_id,base_commodity_id,quote_commodity_id,source_id,last_error,updated_at)
 VALUES (1,?,?,2,'failed','2026-01-01T00:00:00Z');
 INSERT INTO market_data_ingest_runs (book_id,source_id,trigger,status,started_at) VALUES
 (1,2,'manual','failed','2026-01-01T00:00:00Z'),
 (1,2,'manual','partial','2026-01-02T00:00:00Z');`, currencyID, currencyID)
	require.NoError(t, err)
	read := func() attentionResponse {
		res := doInvestmentRequest(t, handler, cookie, "", http.MethodGet, "/api/v1/attention", nil, http.StatusOK)
		var result attentionResponse
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
		return result
	}
	result := read()
	assert.Equal(t, 1, result.TotalKnown) // Error state and two runs, one source.
	assert.True(t, result.HasFailures)
	_, err = database.Exec(`UPDATE pricing_refresh_state SET last_error=NULL;
 INSERT INTO market_data_ingest_runs (book_id,source_id,trigger,status,started_at) VALUES (1,2,'manual','succeeded','2026-01-03T00:00:00Z')`)
	require.NoError(t, err)
	result = read()
	assert.Zero(t, result.TotalKnown)
	assert.False(t, result.HasFailures)
	_, err = database.Exec(`INSERT INTO market_data_ingest_runs (book_id,source_id,trigger,status,started_at) VALUES (1,2,'manual','running','2026-01-04T00:00:00Z')`)
	require.NoError(t, err)
	result = read()
	assert.False(t, result.Complete, "a running refresh is not an empty successful check")
	for _, source := range result.Sources {
		if source.ID == "refresh" {
			assert.Equal(t, "loading", source.State)
			assert.Nil(t, source.Count)
		}
	}
}

func TestAttentionConnectionFailureSurvivesAnotherDownloadRunning(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	cookie, _, _ := setupAccountAPITest(t, handler)
	_, err := database.Exec(`INSERT INTO import_connections (book_id,source_kind,display_name,secret_ciphertext,last_fetch_status,created_at,updated_at) VALUES
 (1,'trading212','Failed connection','test-only','failed','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z'),
 (1,'trading212','Running connection','test-only','fetching','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	res := doInvestmentRequest(t, handler, cookie, "", http.MethodGet, "/api/v1/attention", nil, http.StatusOK)
	var result attentionResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
	assert.False(t, result.Complete)
	assert.True(t, result.HasFailures)
	assert.Equal(t, 1, result.TotalKnown)
	for _, source := range result.Sources {
		if source.ID == "connections" {
			assert.Equal(t, "loading", source.State)
			require.NotNil(t, source.Count)
			assert.Equal(t, 1, *source.Count)
			assert.True(t, source.Failed)
		}
	}
}
