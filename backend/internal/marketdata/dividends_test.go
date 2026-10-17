package marketdata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture follows Twelve Data's documented GetDividends shape (OpenAPI,
// checked 2026-10-17): meta + dividends[{ex_date, amount}], nothing else.
func TestTwelveDataDividendsFixtureWireSemantics(t *testing.T) {
	fixture, err := os.ReadFile("testdata/twelve_data_dividends.json")
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/dividends", r.URL.Path)
		assert.Equal(t, "ASML", r.URL.Query().Get("symbol"))
		assert.Equal(t, "XAMS", r.URL.Query().Get("mic_code"))
		assert.Equal(t, "false", r.URL.Query().Get("adjust"), "split-adjusted history must never replace the announced amount")
		assert.Equal(t, "2025-10-17", r.URL.Query().Get("start_date"))
		assert.Equal(t, "2026-10-17", r.URL.Query().Get("end_date"))
		assert.Equal(t, "secret", r.URL.Query().Get("apikey"))
		_, _ = w.Write(fixture)
	}))
	defer server.Close()
	registry := NewRegistry(NewTwelveDataProvider(server.Client(), server.URL, "secret"))
	provider, ok := registry.DividendProvider("twelve_data")
	require.True(t, ok)
	observations, err := provider.FetchDividends(context.Background(), DividendRequest{ProviderInstrumentID: "ASML", MIC: "XAMS", StartDate: "2025-10-17", EndDate: "2026-10-17"})
	require.NoError(t, err)
	require.Len(t, observations, 4)
	first := observations[0]
	assert.Equal(t, "2026-07-29", first.ExDate)
	assert.True(t, first.AmountKnown)
	assert.Equal(t, int64(16), first.AmountValue)
	assert.Equal(t, 1, first.AmountScale)
	assert.Equal(t, "EUR", first.ListingCurrency)
	assert.Equal(t, "twelve_data:dividend:ASML:XAMS:2026-07-29", first.ProviderObservationID)
	assert.JSONEq(t, `{"meta":{"symbol":"ASML","currency":"EUR","mic_code":"XAMS"},"dividend":{"ex_date":"2026-07-29","amount":1.6},"adjust":false}`, first.RawJSON)
	assert.False(t, observations[2].AmountKnown, "an unparseable amount stays identifiable evidence without a value")
	assert.Equal(t, "2026-02-02", observations[2].ExDate)

	_, ok = NewRegistry(NewYahooFinanceProvider(nil, "")).DividendProvider("yahoo_finance")
	assert.False(t, ok, "only verified mappings are dividend providers")
}

func TestTwelveDataDividendsRefusesAmbiguousOrForeignEvidence(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"different market", `{"meta":{"symbol":"ASML","mic_code":"XNAS","currency":"USD"},"dividends":[]}`},
		{"different symbol", `{"meta":{"symbol":"ASM","mic_code":"XAMS","currency":"EUR"},"dividends":[]}`},
		{"invalid ex-date", `{"meta":{"symbol":"ASML","mic_code":"XAMS","currency":"EUR"},"dividends":[{"ex_date":"28/04/2026","amount":1}]}`},
		{"duplicate ex-date", `{"meta":{"symbol":"ASML","mic_code":"XAMS","currency":"EUR"},"dividends":[{"ex_date":"2026-04-28","amount":1},{"ex_date":"2026-04-28","amount":2}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer server.Close()
			_, err := NewTwelveDataProvider(server.Client(), server.URL, "secret").FetchDividends(context.Background(), DividendRequest{ProviderInstrumentID: "ASML", MIC: "XAMS", StartDate: "2026-01-01", EndDate: "2026-12-31"})
			require.Error(t, err)
		})
	}
}

func TestTwelveDataDividendsPlanEntitlementIsTerminal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":403,"message":"plan","status":"error"}`))
	}))
	defer server.Close()
	_, err := NewTwelveDataProvider(server.Client(), server.URL, "secret").FetchDividends(context.Background(), DividendRequest{ProviderInstrumentID: "ASML", MIC: "XAMS", StartDate: "2026-01-01", EndDate: "2026-12-31"})
	var quoteErr *QuoteError
	require.ErrorAs(t, err, &quoteErr)
	assert.True(t, quoteErr.Terminal)
	_, err = NewTwelveDataProvider(nil, "", "").FetchDividends(context.Background(), DividendRequest{})
	require.ErrorIs(t, err, ErrMissingSecret)
	_, err = NewTwelveDataProvider(nil, "", "secret").FetchDividends(context.Background(), DividendRequest{ProviderInstrumentID: "ASML", StartDate: "2026-01-01", EndDate: "2026-12-31"})
	require.ErrorIs(t, err, ErrUnsupportedPair, "a listing without a market identifier is not an identity")
}
