package marketdata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

// DividendProvider reports per-share cash dividend announcements for one
// explicitly identified listing (#111). Like PriceProvider it returns facts
// only: it never sees database IDs, holdings or accounts, and it never
// decides what the owner received.
type DividendProvider interface {
	Provider
	FetchDividends(context.Context, DividendRequest) ([]DividendObservation, error)
}

type DividendRequest struct {
	ProviderInstrumentID string
	MIC                  string
	StartDate            string
	EndDate              string
}

// DividendObservation is one provider row. Twelve Data's documented contract
// (OpenAPI GetDividends, checked 2026-10-17) carries only ex_date and amount;
// meta.currency is the listing's trading currency, not a stated dividend
// currency, so ListingCurrency is evidence, never the payment currency.
// AmountKnown is false when the amount was absent or not an exact positive
// decimal; the row is still identifiable and kept as informational evidence.
type DividendObservation struct {
	ProviderInstrumentID  string
	MIC                   string
	ExDate                string
	AmountKnown           bool
	AmountValue           int64
	AmountScale           int
	ListingCurrency       string
	ProviderObservationID string
	RawJSON               string
}

func (r *Registry) DividendProvider(code string) (DividendProvider, bool) {
	provider, ok := r.Provider(code)
	if !ok {
		return nil, false
	}
	dividends, ok := provider.(DividendProvider)
	return dividends, ok
}

// FetchDividends calls GET /dividends with adjust=false, so split-adjusted
// history never replaces the amount announced for the shares held then. The
// endpoint needs a paid Twelve Data plan (x-starting-plan grow); a key without
// that entitlement gets a provider error the worker records as terminal.
func (p *TwelveDataProvider) FetchDividends(ctx context.Context, request DividendRequest) ([]DividendObservation, error) {
	if p.apiKey == "" {
		return nil, ErrMissingSecret
	}
	if request.ProviderInstrumentID == "" || request.MIC == "" {
		return nil, ErrUnsupportedPair
	}
	for _, date := range []string{request.StartDate, request.EndDate} {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return nil, errors.New("dividend request needs a calendar date range")
		}
	}
	query := url.Values{"symbol": {request.ProviderInstrumentID}, "mic_code": {request.MIC}, "start_date": {request.StartDate}, "end_date": {request.EndDate}, "adjust": {"false"}, "apikey": {p.apiKey}}
	var data struct {
		Code   int    `json:"code"`
		Status string `json:"status"`
		Meta   struct {
			Symbol   string `json:"symbol"`
			Currency string `json:"currency"`
			MIC      string `json:"mic_code"`
		} `json:"meta"`
		Dividends []struct {
			ExDate string          `json:"ex_date"`
			Amount json.RawMessage `json:"amount"`
		} `json:"dividends"`
	}
	if err := quoteJSON(ctx, p.client, p.baseURL+"/dividends?"+query.Encode(), nil, &data); err != nil {
		return nil, err
	}
	if data.Status == "error" {
		return nil, &QuoteError{Status: data.Code, Terminal: data.Code >= 400 && data.Code < 500 && data.Code != 429}
	}
	if data.Meta.Symbol != request.ProviderInstrumentID || data.Meta.MIC != request.MIC {
		return nil, errors.New("dividend provider returned a different instrument or market")
	}
	observations := make([]DividendObservation, 0, len(data.Dividends))
	seen := map[string]bool{}
	for _, row := range data.Dividends {
		if _, err := time.Parse(time.DateOnly, row.ExDate); err != nil {
			return nil, errors.New("dividend provider returned an invalid ex-date")
		}
		if row.ExDate < request.StartDate || row.ExDate > request.EndDate {
			continue
		}
		// One announcement per listing and ex-date is the documented shape.
		// Two rows for one ex-date are ambiguous evidence, not two payments.
		if seen[row.ExDate] {
			return nil, errors.New("dividend provider returned two amounts for one ex-date")
		}
		seen[row.ExDate] = true
		observation := DividendObservation{ProviderInstrumentID: request.ProviderInstrumentID, MIC: request.MIC, ExDate: row.ExDate,
			ListingCurrency:       strings.ToUpper(data.Meta.Currency),
			ProviderObservationID: "twelve_data:dividend:" + request.ProviderInstrumentID + ":" + request.MIC + ":" + row.ExDate}
		amount := string(bytes.Trim(bytes.TrimSpace(row.Amount), `"`))
		if value, scale, err := quoteDecimal(amount); err == nil {
			observation.AmountKnown, observation.AmountValue, observation.AmountScale = true, value, scale
		}
		raw, _ := json.Marshal(map[string]any{"meta": data.Meta, "dividend": map[string]any{"ex_date": row.ExDate, "amount": json.RawMessage(nonEmptyJSON(row.Amount))}, "adjust": false})
		observation.RawJSON = string(raw)
		observations = append(observations, observation)
	}
	return observations, nil
}

func nonEmptyJSON(value json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(value)) == 0 {
		return json.RawMessage("null")
	}
	return value
}
