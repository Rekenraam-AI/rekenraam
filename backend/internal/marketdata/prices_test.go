package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuoteDecimalNeverUsesBinaryFloat(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		value      int64
		scale      int
		invalid    bool
	}{
		{"crypto exponent", "1.23456789e-8", 123456789, 16, false},
		{"large exact integer", "9007199254740993", 9007199254740993, 0, false},
		{"trailing zeros canonicalized", "123.450000", 12345, 2, false},
		{"positive exponent", "1.2e3", 1200, 0, false},
		{"negative", "-1", 0, 0, true}, {"zero", "0", 0, 0, true}, {"precision ceiling", "1e-19", 0, 0, true}, {"overflow", "9223372036854775808", 0, 0, true}, {"unbounded exponent", "1e999", 0, 0, true}, {"fraction rejected", "1/2", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, scale, err := quoteDecimal(tc.text)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.value, value)
			assert.Equal(t, tc.scale, scale)
		})
	}
}
func TestCoinGeckoUsesExplicitIDCurrencyTimestampAndDemoKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bitcoin", r.URL.Query().Get("ids"))
		assert.Equal(t, "eur", r.URL.Query().Get("vs_currencies"))
		assert.Equal(t, "true", r.URL.Query().Get("include_last_updated_at"))
		assert.Equal(t, "secret", r.Header.Get("x-cg-demo-api-key"))
		_, _ = w.Write([]byte(`{"bitcoin":{"eur":1.23456789e-8,"last_updated_at":1781265600}}`))
	}))
	defer server.Close()
	provider := NewCoinGeckoProvider(server.Client(), server.URL, "secret")
	observations, err := provider.FetchPrices(context.Background(), PriceRequest{InstrumentType: "crypto", ProviderInstrumentID: "bitcoin", QuoteCode: "EUR"})
	require.NoError(t, err)
	require.Len(t, observations, 1)
	assert.Equal(t, int64(123456789), observations[0].PriceValue)
	assert.Equal(t, 16, observations[0].PriceScale)
	assert.Equal(t, "raw", observations[0].AdjustmentBasis)
	assert.Equal(t, "spot_mid", observations[0].QuoteType)
	assert.Equal(t, "2026-06-12T12:00:00Z", observations[0].ObservedAt)
	_, err = provider.FetchPrices(context.Background(), PriceRequest{InstrumentType: "stock", ProviderInstrumentID: "bitcoin", QuoteCode: "EUR"})
	require.ErrorIs(t, err, ErrUnsupportedPair)
	_, err = NewCoinGeckoProvider(nil, "", "").FetchPrices(context.Background(), PriceRequest{})
	require.ErrorIs(t, err, ErrMissingSecret)
}
func TestTwelveDataRequestsRawCloseAndValidatesIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, currency, mic string
		fail                bool
	}{{"EU close", "EUR", "XAMS", false}, {"wrong currency", "USD", "XAMS", true}, {"wrong market", "EUR", "XPAR", true}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "none", r.URL.Query().Get("adjust"))
				assert.Equal(t, "secret", r.URL.Query().Get("apikey"))
				assert.Equal(t, "XAMS", r.URL.Query().Get("mic_code"))
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "meta": map[string]string{"symbol": "ASML", "currency": tc.currency, "mic_code": tc.mic, "exchange_timezone": "Europe/Amsterdam"}, "values": []map[string]string{{"datetime": "2026-06-11", "close": "900.123456"}}})
			}))
			defer server.Close()
			observations, err := NewTwelveDataProvider(server.Client(), server.URL, "secret").FetchPrices(context.Background(), PriceRequest{ProviderInstrumentID: "ASML", QuoteCode: "EUR", MIC: "XAMS"})
			if tc.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, observations, 1)
			assert.Equal(t, int64(900123456), observations[0].PriceValue)
			assert.Equal(t, "close", observations[0].QuoteType)
		})
	}
}
func TestYahooUnofficialRawCloseRejectsMinorCurrencyAndNulls(t *testing.T) {
	for _, tc := range []struct {
		name, currency, close string
		fail                  bool
	}{{"raw close", "EUR", "123.45", false}, {"minor currency", "GBp", "123.45", true}, {"no close", "EUR", "null", true}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "ASML.AS")
				_, _ = w.Write([]byte(`{"chart":{"error":null,"result":[{"meta":{"currency":"` + tc.currency + `","symbol":"ASML.AS","exchangeTimezoneName":"Europe/Amsterdam"},"timestamp":[1781265600],"indicators":{"quote":[{"close":[` + tc.close + `]}],"adjclose":[{"adjclose":[1]}]}}]}}`))
			}))
			defer server.Close()
			provider := NewYahooFinanceProvider(server.Client(), server.URL)
			observations, err := provider.FetchPrices(context.Background(), PriceRequest{ProviderInstrumentID: "ASML.AS", QuoteCode: "EUR"})
			if tc.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, int64(12345), observations[0].PriceValue)
			assert.Contains(t, provider.Name(), "unofficial")
			assert.Equal(t, "raw", observations[0].AdjustmentBasis)
		})
	}
}
func TestQuoteHTTPFailureRedactionRateLimitAndRedirect(t *testing.T) {
	destinationHit := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destinationHit = true }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, destination.URL, 302)
			return
		}
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(429)
		_, _ = w.Write([]byte("secret"))
	}))
	defer server.Close()
	var result any
	err := quoteJSON(context.Background(), server.Client(), server.URL+"?apikey=secret", nil, &result)
	var providerErr *QuoteError
	require.True(t, errors.As(err, &providerErr))
	assert.Equal(t, 10*time.Minute, providerErr.RetryAfter)
	assert.False(t, providerErr.Terminal)
	assert.False(t, strings.Contains(err.Error(), "secret"))
	err = quoteJSON(context.Background(), server.Client(), server.URL+"/redirect", map[string]string{"x-cg-demo-api-key": "secret"}, &result)
	require.Error(t, err)
	assert.False(t, destinationHit)
}
func TestRegistryOwnsAllPriceProviders(t *testing.T) {
	registry := DefaultRegistry("", "", "")
	for _, code := range []string{"coingecko", "twelve_data", "yahoo_finance"} {
		provider, ok := registry.PriceProvider(code)
		require.True(t, ok)
		assert.True(t, provider.Capabilities().Prices)
		_, ok = registry.FXRateProvider(code)
		assert.False(t, ok)
	}
}
