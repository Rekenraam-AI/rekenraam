package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/marketdata"
)

// #274: a chunk limit bounds execution, never the historical horizon. Drive
// the real worker through more than two chunks, including a service restart.
func TestFXCoverageWorkerFinishesEveryContinuationChunk(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rates", r.URL.Path)
		base, quote := r.URL.Query().Get("base"), r.URL.Query().Get("quotes")
		rate := "0.8"
		if base == "EUR" {
			rate = "1.25"
		}
		_, _ = fmt.Fprintf(w, `{"base":%q,"date":%q,"rates":{%q:%s}}`, base, r.URL.Query().Get("date"), quote, rate)
	}))
	defer server.Close()
	registry := marketdata.DefaultRegistryAt(server.URL, "", "", "")
	database, service := newPricingRefreshTestService(t, []string{"USD", "EUR"}, registry)
	// Remove fixture activation work; the one request below owns this horizon.
	_, err := database.Exec(`DELETE FROM background_work_items`)
	require.NoError(t, err)
	sourceID := marketDataSourceIDByCode(t, database, "frankfurter")
	_, err = service.SavePolicy(ctx, PricingPolicyInput{OwnerUserID: 1, DefaultSourceID: &sourceID,
		MaxBackfillDays: 1, WeekendPolicy: "skip", RoundingMode: "half_up"})
	require.NoError(t, err)
	_, err = service.repository.EnqueueBackgroundWork(ctx, BookID, fxCoverageWorkKind,
		`{"reason":"manual","start_date":"2026-06-09","currency_id":2}`, "2026-06-12T00:00:00Z")
	require.NoError(t, err)
	for tick := 0; tick < 12 && pendingFXCoverageWorkCount(t, database) > 0; tick++ {
		// Recreate the service between ticks: no in-memory progress can rescue it.
		service = NewPricingService(db.NewPricingRepository(database), registry)
		service.SetNowForTest(func() time.Time { return time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC) })
		service.runDueFXCoverage(ctx, testLogger(), fmt.Sprintf("worker-%d", tick))
	}
	assert.Zero(t, pendingFXCoverageWorkCount(t, database))
	for _, pair := range [][2]int64{{1, 2}, {2, 1}} {
		prices, err := service.ListPrices(ctx, pair[0], pair[1], 20, false)
		require.NoError(t, err)
		require.Len(t, prices, 4, "four publication dates per direction must survive chunking")
		for i, price := range prices {
			assert.Equal(t, fmt.Sprintf("2026-06-%02d", 12-i), price.ValuationDate)
			if pair[0] == 1 {
				assert.Equal(t, int64(8), price.PriceValue)
				assert.Equal(t, 1, price.PriceScale)
			} else {
				assert.Equal(t, int64(125), price.PriceValue)
				assert.Equal(t, 2, price.PriceScale)
			}
		}
	}
}
