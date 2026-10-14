package marketdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #226: the e2e harness points every built-in provider at a stub host, so a
// background FX or quote fetch never reaches a live provider.
func TestDefaultRegistryAtSendsEveryProviderToTheOverrideHost(t *testing.T) {
	t.Parallel()
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.NotFound(w, nil)
	}))
	defer server.Close()
	registry := DefaultRegistryAt(server.URL, "app-id", "coingecko-key", "twelve-key")
	require.NotEmpty(t, registry.providers)
	for code, provider := range registry.providers {
		before := hits.Load()
		var err error
		switch p := provider.(type) {
		case FXRateProvider:
			// ECB accepts only a EUR base and the free Open Exchange Rates
			// plan only USD; each provider receives the base it supports.
			for _, base := range []string{"EUR", "USD"} {
				_, err = p.FetchFXRates(context.Background(), FXRateRequest{BaseCode: base, QuoteCodes: []string{"GBP"}, Date: "2026-10-14"})
				if hits.Load() > before {
					break
				}
			}
		case PriceProvider:
			// A crypto pair is the request every quote provider accepts.
			_, err = p.FetchPrices(context.Background(), PriceRequest{InstrumentType: "crypto", ProviderInstrumentID: "bitcoin", QuoteCode: "USD"})
		default:
			t.Fatalf("provider %s fetches neither FX nor prices", code)
		}
		assert.Error(t, err, code)
		assert.Greater(t, hits.Load(), before, "%s requested the override host", code)
	}
}
