package api

import (
	"net/http"
	"os"
	"runtime"
	"runtime/metrics"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// serial: opt-in, process-wide heap sampling; run alone without race instrumentation.
func TestReferenceBookReadPerformanceAudit(t *testing.T) {
	if os.Getenv("REKENRAAM_AUDIT_READS") != "1" {
		t.Skip("#280 measurement: REKENRAAM_AUDIT_READS=1 scripts/go-backend.sh test -count=1 -v -run '^TestReferenceBookReadPerformanceAudit$' ./internal/api")
	}
	handler, _ := newSetupTestHandler(t)
	seeded := seedReferenceBook(t, handler)
	// Correctness comes from #266's hand-derived register, not a timed response.
	assertReferenceBookRegister(t, handler, seeded)
	paths := []string{
		"/api/v1/transactions?limit=50",
		"/api/v1/ledger/account-balances?as_of=2026-03-31&include_system=true",
		"/api/v1/reports/net-worth?start_date=2026-02-01&end_date=2026-03-31&bucket=month",
		"/api/v1/reports/spending?start_date=2026-02-01&end_date=2026-03-31&group_by=category",
		"/api/v1/reports/cashflow?start_date=2026-02-01&end_date=2026-03-31&bucket=month",
		"/api/v1/investments/positions?as_of=2026-03-31",
		"/api/v1/investments/gains?as_of=2026-03-31",
		"/api/v1/budgets/month?period_start=2026-02-01",
		"/api/v1/forecasts/balances?horizon_days=30",
	}
	for _, path := range paths {
		response := seeded.request(t, handler, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", path, response.Body.String())
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		samples := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
		metrics.Read(samples)
		baseline := samples[0].Value.Uint64()
		peak := baseline
		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			ticker := time.NewTicker(time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					metrics.Read(samples)
					peak = max(peak, samples[0].Value.Uint64())
				}
			}
		}()
		started := time.Now()
		const reads = 20
		for range reads {
			response := seeded.request(t, handler, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, response.Code, "%s: %s", path, response.Body.String())
		}
		elapsed := time.Since(started)
		close(stop)
		<-done
		metrics.Read(samples)
		peak = max(peak, samples[0].Value.Uint64())
		runtime.ReadMemStats(&after)
		t.Logf("%s: reads=%d mean=%s allocated/read=%dB sampled-peak-heap-delta=%dB", path, reads,
			elapsed/reads, (after.TotalAlloc-before.TotalAlloc)/reads, peak-baseline)
	}
}
