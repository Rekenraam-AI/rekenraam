package marketdata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// Keep errors.Is/As classification without rendering a credential-bearing
// URL (url.Error), transport text, or response body in health/history/logs.
type fxHTTPError struct {
	message string
	cause   error
}

func (e *fxHTTPError) Error() string { return e.message }
func (e *fxHTTPError) Unwrap() error { return e.cause }

func getBytes(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &fxHTTPError{message: "invalid market data request", cause: err}
	}
	request.Header.Set("Accept", "application/json, application/xml, text/xml;q=0.9, */*;q=0.1")
	response, err := httpClient(client).Do(request)
	if err != nil {
		return nil, &fxHTTPError{message: "market data connection failed", cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("market data request failed: %d %s", response.StatusCode, http.StatusText(response.StatusCode))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 10<<20))
	if err != nil {
		return nil, &fxHTTPError{message: "market data response read failed", cause: err}
	}
	return body, nil
}

func trimBaseURL(baseURL string, fallback string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = fallback
	}
	return strings.TrimRight(baseURL, "/")
}
