package app

import (
	"context"
	"database/sql"
	"fmt"
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

// Cancel in the provider request, close/reopen SQLite, and reclaim using the
// production pricing lifecycle. No mock worker or fabricated successful run.
func TestPricingWorkersRecoverInterruptedHTTPAfterDatabaseReopen(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{fxCoverageWorkKind, quoteRefreshWorkKind, dividendEventWorkKind} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			var database *sql.DB
			var original *PricingService
			payload := `{"reason":"manual"}`
			var enqueue func(*PricingService) error
			switch kind {
			case fxCoverageWorkKind:
				database, original = newPricingRefreshTestService(t, []string{"USD", "EUR"}, marketdata.NewRegistry())
			case quoteRefreshWorkKind:
				f := newInvestTestFixture(t)
				instrument, _ := cryptoTestHolding(t, f)
				database = f.database
				original = NewPricingService(db.NewPricingRepository(database), marketdata.NewRegistry())
				source := marketDataSourceIDByCode(t, database, "coingecko")
				_, err := original.SaveSourceAssignment(ctx, PricingSourceAssignmentInput{OwnerUserID: 1, CommodityID: instrument.CommodityID,
					QuoteCommodityID: f.eurCommodityID, SourceID: source, EffectiveFrom: "2020-01-01"})
				require.NoError(t, err)
				enqueue = func(s *PricingService) error { _, err := s.QueueQuoteRefresh(ctx, 1); return err }
			case dividendEventWorkKind:
				f := newDividendEventFixture(t)
				database, original = f.database, f.pricing
				enqueue = func(s *PricingService) error { _, err := s.QueueDividendEventFetch(ctx, 1); return err }
			}
			_, err := database.Exec(`DELETE FROM background_work_items`)
			require.NoError(t, err)
			started := make(chan struct{}, 1)
			var interrupted atomic.Bool
			interrupted.Store(true)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if interrupted.Load() {
					select {
					case started <- struct{}{}:
					default:
					}
					<-r.Context().Done()
					return
				}
				switch r.URL.Path {
				case "/simple/price":
					_, _ = fmt.Fprint(w, `{"bitcoin":{"eur":65000,"last_updated_at":1781265600}}`)
				case "/dividends":
					_, _ = fmt.Fprint(w, `{"meta":{"symbol":"ASML","currency":"EUR","mic_code":"XAMS","exchange_timezone":"Europe/Amsterdam"},"dividends":[{"ex_date":"2026-07-29","amount":1.6}]}`)
				case "/rates":
					_, _ = fmt.Fprintf(w, `{"base":%q,"date":%q,"rates":{%q:1}}`, r.URL.Query().Get("base"), r.URL.Query().Get("date"), r.URL.Query().Get("quotes"))
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			registry := marketdata.DefaultRegistryAt(server.URL, "", "stub-key", "stub-key")
			original.providerRegistry = registry
			now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
			if kind != dividendEventWorkKind {
				now = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
			}
			original.SetNowForTest(func() time.Time { return now })
			if kind == fxCoverageWorkKind {
				source := marketDataSourceIDByCode(t, database, "frankfurter")
				_, err = original.SavePolicy(ctx, PricingPolicyInput{OwnerUserID: 1, DefaultSourceID: &source, WeekendPolicy: "skip", RoundingMode: "half_up"})
				require.NoError(t, err)
				_, err = original.repository.EnqueueBackgroundWork(ctx, BookID, kind, payload, now.Format(time.RFC3339))
			} else {
				err = enqueue(original)
			}
			require.NoError(t, err)
			var filename string
			var seq int
			var schema string
			require.NoError(t, database.QueryRow(`PRAGMA database_list`).Scan(&seq, &schema, &filename))
			var beforeTransactions, beforeLots int
			require.NoError(t, database.QueryRow(`SELECT count(*) FROM transactions`).Scan(&beforeTransactions))
			require.NoError(t, database.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&beforeLots))
			workCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			done := original.StartBackgroundWorker(workCtx, testLogger())
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("worker did not enter provider request")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("worker did not stop")
			}
			items, err := original.repository.ListBackgroundWorkByStatus(ctx, BookID, kind, "running", 10)
			require.NoError(t, err)
			require.Len(t, items, 1)
			assert.Equal(t, 1, items[0].Attempts)
			require.NoError(t, database.Close())
			reopened, err := db.Open(ctx, db.SQLiteFileURL(filename))
			require.NoError(t, err)
			defer reopened.Close()
			resumed := NewPricingService(db.NewPricingRepository(reopened), registry)
			resumed.SetNowForTest(func() time.Time { return now })
			run := func() {
				switch kind {
				case fxCoverageWorkKind:
					resumed.runDueFXCoverage(ctx, testLogger(), "resumed")
				case quoteRefreshWorkKind:
					resumed.runDueQuoteRefresh(ctx, testLogger(), "resumed")
				case dividendEventWorkKind:
					resumed.runDueDividendEvents(ctx, testLogger(), "resumed")
				}
			}
			run()
			assert.Equal(t, int32(1), calls.Load(), "startup must respect the unexpired lease")
			interrupted.Store(false)
			now = now.Add(15 * time.Minute)
			// Crash-equivalent window: domain effects commit, then completion
			// fails. The next lease must repeat HTTP without repeating effects.
			_, err = reopened.Exec(`CREATE TRIGGER interrupt_work_completion BEFORE UPDATE OF status ON background_work_items WHEN NEW.status='completed' BEGIN SELECT RAISE(ABORT, 'injected completion failure'); END`)
			require.NoError(t, err)
			run()
			item, err := resumed.repository.BackgroundWorkByID(ctx, items[0].ID)
			require.NoError(t, err)
			expected := "running"
			if kind == fxCoverageWorkKind {
				// FX catches a returned completion error and schedules a retry;
				// quote/dividend leave the completed effect under its live lease.
				expected = "pending"
			}
			assert.Equal(t, expected, item.Status)
			assert.Equal(t, 2, item.Attempts)
			var observationsBeforeRepeat, suggestionsBeforeRepeat int
			require.NoError(t, reopened.QueryRow(`SELECT count(*) FROM price_observations`).Scan(&observationsBeforeRepeat))
			require.NoError(t, reopened.QueryRow(`SELECT count(*) FROM investment_event_suggestions`).Scan(&suggestionsBeforeRepeat))
			_, err = reopened.Exec(`DROP TRIGGER interrupt_work_completion`)
			require.NoError(t, err)
			now = now.Add(15 * time.Minute)
			run()
			item, err = resumed.repository.BackgroundWorkByID(ctx, items[0].ID)
			require.NoError(t, err)
			assert.Equal(t, "completed", item.Status)
			assert.Equal(t, 3, item.Attempts)
			run()
			var transactions, lots, observations, suggestions int
			require.NoError(t, reopened.QueryRow(`SELECT count(*) FROM transactions`).Scan(&transactions))
			require.NoError(t, reopened.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&lots))
			require.NoError(t, reopened.QueryRow(`SELECT count(*) FROM price_observations`).Scan(&observations))
			require.NoError(t, reopened.QueryRow(`SELECT count(*) FROM investment_event_suggestions`).Scan(&suggestions))
			assert.Equal(t, beforeTransactions, transactions)
			assert.Equal(t, beforeLots, lots)
			assert.Equal(t, observationsBeforeRepeat, observations)
			assert.Equal(t, suggestionsBeforeRepeat, suggestions)
			switch kind {
			case fxCoverageWorkKind:
				assert.Equal(t, 2, observations)
				assert.Zero(t, suggestions)
			case quoteRefreshWorkKind:
				assert.Equal(t, 1, observations)
				assert.Zero(t, suggestions)
			case dividendEventWorkKind:
				assert.Equal(t, 1, suggestions)
			}
		})
	}
}
