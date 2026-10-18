package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/marketdata"
)

func TestQuoteWorkerDurableDedupeRevisionHealthAndNoLedgerWrites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestTestFixture(t)
	instrument, _ := cryptoTestHolding(t, f)
	var calls atomic.Int32
	var value atomic.Int64
	value.Store(65000123)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This read uses the single-connection pool: it would hang if the provider
		// were called inside a DB transaction.
		var count int
		assert.NoError(t, f.database.QueryRowContext(r.Context(), `SELECT count(*) FROM transactions`).Scan(&count))
		calls.Add(1)
		_, _ = fmt.Fprintf(w, `{"bitcoin":{"eur":%d,"last_updated_at":1781265600}}`, value.Load())
	}))
	defer server.Close()
	service := NewPricingService(db.NewPricingRepository(f.database), marketdata.NewRegistry(marketdata.NewCoinGeckoProvider(server.Client(), server.URL, "secret")))
	now := time.Date(2026, 6, 12, 12, 10, 0, 0, time.UTC)
	service.SetNowForTest(func() time.Time { return now })
	var sourceID int64
	require.NoError(t, f.database.QueryRow(`SELECT id FROM market_data_sources WHERE code='coingecko'`).Scan(&sourceID))
	_, err := service.SaveSourceAssignment(ctx, PricingSourceAssignmentInput{OwnerUserID: 1, CommodityID: instrument.CommodityID, QuoteCommodityID: f.eurCommodityID, SourceID: sourceID, EffectiveFrom: "2020-01-01"})
	require.NoError(t, err)
	for range 2 {
		count, err := service.QueueQuoteRefresh(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	}
	items, err := service.repository.ListBackgroundWorkByStatus(ctx, BookID, quoteRefreshWorkKind, "pending", 50)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Zero(t, calls.Load())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Reconstruct service to prove accepted work survives process loss.
	resumed := NewPricingService(db.NewPricingRepository(f.database), service.providerRegistry)
	resumed.SetNowForTest(func() time.Time { return now })
	resumed.runDueQuoteRefresh(ctx, logger, "worker")
	stored, err := service.repository.ListPriceObservations(ctx, BookID, instrument.CommodityID, f.eurCommodityID, 100, true)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, int64(65000123), stored[0].PriceValue)
	assert.False(t, stored[0].IsManual)
	assert.Equal(t, "2026-06-12T12:00:00Z", stored[0].ObservedAt.String)
	for range 2 {
		_, err := service.QueueQuoteRefresh(ctx, 1)
		require.NoError(t, err)
		resumed.runDueQuoteRefresh(ctx, logger, "worker")
	}
	stored, err = service.repository.ListPriceObservations(ctx, BookID, instrument.CommodityID, f.eurCommodityID, 100, true)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	value.Store(65000456)
	_, err = service.QueueQuoteRefresh(ctx, 1)
	require.NoError(t, err)
	resumed.runDueQuoteRefresh(ctx, logger, "worker")
	stored, err = service.repository.ListPriceObservations(ctx, BookID, instrument.CommodityID, f.eurCommodityID, 100, true)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	require.True(t, stored[0].SupersedesObservationID.Valid)
	assert.Equal(t, stored[1].ID, stored[0].SupersedesObservationID.Int64)
	health, err := service.SourceHealth(ctx)
	require.NoError(t, err)
	require.Len(t, health, 1)
	assert.Equal(t, "ok", health[0].Status)
	// An identical provider refetch cannot undo the owner's withdrawal.
	_, err = service.VoidPrice(ctx, VoidPriceInput{OwnerUserID: 1, ObservationID: stored[0].ID, VoidReason: "owner rejected this provider quote"})
	require.NoError(t, err)
	_, err = service.QueueQuoteRefresh(ctx, 1)
	require.NoError(t, err)
	resumed.runDueQuoteRefresh(ctx, logger, "worker")
	stored, err = service.repository.ListPriceObservations(ctx, BookID, instrument.CommodityID, f.eurCommodityID, 100, true)
	require.NoError(t, err)
	require.Len(t, stored, 2, "no replacement is fabricated for an identical withdrawn quote")
	assert.True(t, stored[0].VoidedAt.Valid)
	var transactions, lots int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM transactions`).Scan(&transactions))
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&lots))
	assert.Zero(t, transactions)
	assert.Zero(t, lots)
}
func TestQuoteWorkerTerminalFailureRetryRecoveryAndRateLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		status   int
		terminal bool
	}{{"credentials", 401, true}, {"rate limit", 429, false}, {"server", 503, false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newInvestTestFixture(t)
			instrument, _ := cryptoTestHolding(t, f)
			ctx := context.Background()
			var status atomic.Int32
			status.Store(int32(tc.status))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "600")
				w.WriteHeader(int(status.Load()))
				if status.Load() == 200 {
					_, _ = w.Write([]byte(`{"bitcoin":{"eur":65000,"last_updated_at":1781265600}}`))
				}
			}))
			defer server.Close()
			service := NewPricingService(db.NewPricingRepository(f.database), marketdata.NewRegistry(marketdata.NewCoinGeckoProvider(server.Client(), server.URL, "secret")))
			now := time.Date(2026, 6, 12, 12, 10, 0, 0, time.UTC)
			service.SetNowForTest(func() time.Time { return now })
			var sourceID int64
			require.NoError(t, f.database.QueryRow(`SELECT id FROM market_data_sources WHERE code='coingecko'`).Scan(&sourceID))
			_, err := service.SaveSourceAssignment(ctx, PricingSourceAssignmentInput{OwnerUserID: 1, CommodityID: instrument.CommodityID, QuoteCommodityID: f.eurCommodityID, SourceID: sourceID, EffectiveFrom: "2020-01-01"})
			require.NoError(t, err)
			_, err = service.QueueQuoteRefresh(ctx, 1)
			require.NoError(t, err)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			service.runDueQuoteRefresh(ctx, logger, "worker")
			expected := "pending"
			if tc.terminal {
				expected = "failed"
			}
			items, err := service.repository.ListBackgroundWorkByStatus(ctx, BookID, quoteRefreshWorkKind, expected, 50)
			require.NoError(t, err)
			require.Len(t, items, 1)
			health, err := service.SourceHealth(ctx)
			require.NoError(t, err)
			require.Len(t, health, 1)
			assert.Equal(t, "error", health[0].Status)
			if tc.status == 429 {
				assert.Equal(t, now.Add(10*time.Minute).Format(time.RFC3339), items[0].AvailableAt)
			}
			if tc.terminal {
				failed, err := service.FailedBackgroundWork(ctx)
				require.NoError(t, err)
				require.Len(t, failed, 1)
				_, err = service.RetryBackgroundWork(ctx, failed[0].ID)
				require.NoError(t, err)
			} else {
				// Exhaust the bounded retry policy through real claims.
				for range maxFXCoverageAttempts - 1 {
					now = now.Add(24 * time.Hour)
					service.runDueQuoteRefresh(ctx, logger, "worker")
				}
				failed, err := service.FailedBackgroundWork(ctx)
				require.NoError(t, err)
				require.Len(t, failed, 1)
				_, err = service.RetryBackgroundWork(ctx, failed[0].ID)
				require.NoError(t, err)
			}
			status.Store(200)
			service.runDueQuoteRefresh(ctx, logger, "worker")
			failed, err := service.FailedBackgroundWork(ctx)
			require.NoError(t, err)
			assert.Empty(t, failed)
		})
	}
}
