package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/marketdata"
)

func TestFXRefreshTransportFailureDoesNotPersistOperatorSecret(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		connection, _, err := w.(http.Hijacker).Hijack()
		if assert.NoError(t, err) {
			_ = connection.Close()
		}
	}))
	defer server.Close()
	const secret = "audit-only-operator-secret"
	database, service := newPricingRefreshTestService(t, []string{"USD", "EUR", "GBP"}, marketdata.DefaultRegistryAt(server.URL, secret))
	// Force the credential-bearing source and exercise the triangulation error
	// consumer, which stores the underlying adapter error in ingest history.
	_, err := database.Exec(`UPDATE market_data_sources SET status=CASE WHEN code='open_exchange_rates_free' THEN 'active' ELSE 'disabled' END WHERE kind='provider'`)
	require.NoError(t, err)
	source := marketDataSourceIDByCode(t, database, "open_exchange_rates_free")
	_, err = service.SavePolicy(ctx, PricingPolicyInput{OwnerUserID: 1, DefaultSourceID: &source,
		TriangulationMaxHops: 1, RoundingMode: "half_up", WeekendPolicy: "skip"})
	require.NoError(t, err)
	run, err := service.runFXRefresh(ctx, 1, source, "manual")
	require.NoError(t, err)
	assert.Equal(t, "failed", run.Status)
	assert.Positive(t, calls.Load(), "the enabled credential-bearing adapter must actually request the stub")
	rows, err := database.Query(`SELECT error FROM market_data_ingest_items WHERE error IS NOT NULL`)
	require.NoError(t, err)
	defer rows.Close()
	var count int
	for rows.Next() {
		var message string
		require.NoError(t, rows.Scan(&message))
		assert.NotContains(t, message, secret)
		assert.NotContains(t, message, server.URL)
		count++
	}
	require.NoError(t, rows.Err())
	assert.Positive(t, count)
}
