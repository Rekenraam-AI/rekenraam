package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// #231: the period statement is one composed read per period: opening and
// closing unrealized under operational_v1, realized and income in between,
// with absent figures null and never zero.
func TestInvestmentGainsStatementAPI(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "STMT")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	// Bought 2 for 40.00 on 2026-02-01 (implied price 20.00); sold 1 for 50.00
	// on 2026-03-01 (implied price 50.00); a 5.00 dividend in the period.
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy",
		tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 5000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", dividendRequest{
		TransactionDate: "2026-03-15", CashAccountID: f.cashAccount.ID, CashCommodityID: f.commodityID,
		IncomeAccountID: &f.incomeAccount.ID, AmountValue: 500, AmountScale: 2,
	}, http.StatusCreated)

	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/investments/gains/statement?from=2026-03-01&to=2026-03-31", nil, http.StatusOK)
	var body investmentGainsStatementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "operational_v1", body.Profile.Name)
	assert.Equal(t, "2026-03-31", body.Profile.ValuationAsOf)
	assert.Equal(t, "2026-03-01", body.From)
	assert.Equal(t, "2026-03-31", body.To)
	assert.NotEmpty(t, body.Currencies)
	require.Len(t, body.Rows, 1)
	row := body.Rows[0]
	assert.Equal(t, f.commodityID, row.CommodityID)
	amount := func(value *moneyCoefficient, scale *int) string {
		require.NotNil(t, value)
		require.NotNil(t, scale)
		// Figures keep the scale their exact arithmetic needed; compare values.
		return exact.ScaledIntFromInt64(int64(*value), *scale).Normalized().String()
	}
	equals := func(decimal string) string {
		parts := strings.SplitN(decimal, ".", 2)
		scale := 0
		if len(parts) == 2 {
			scale = len(parts[1])
		}
		coefficient, err := strconv.ParseInt(strings.Join(parts, ""), 10, 64)
		require.NoError(t, err)
		return exact.ScaledIntFromInt64(coefficient, scale).Normalized().String()
	}
	assert.Equal(t, "2026-02-28", row.Opening.ValuationAsOf)
	assert.Equal(t, equals("0.00"), amount(row.Opening.UnrealizedGainValue, row.Opening.UnrealizedGainScale), "priced at the 20.00 fill")
	assert.Equal(t, "2026-02-01", *row.Opening.LatestPriceDate)
	assert.Equal(t, equals("30.00"), amount(row.Closing.UnrealizedGainValue, row.Closing.UnrealizedGainScale), "1 × 50.00 − 20.00")
	assert.Equal(t, "2026-03-01", *row.Closing.EarliestPriceDate)
	assert.Equal(t, 1, row.Closing.StaleCount, "thirty days old at the period end")
	assert.Equal(t, equals("30.00"), amount(row.RealizedGainValue, row.RealizedGainScale), "50.00 − 20.00")
	assert.Equal(t, 1, row.DisposalCount)
	assert.Equal(t, equals("5.00"), amount(&row.IncomeValue, &row.IncomeScale))
	assert.Equal(t, 1, row.IncomeCount)
	assert.Equal(t, equals("30.00"), amount(row.UnrealizedChangeValue, row.UnrealizedChangeScale))
	assert.Equal(t, equals("65.00"), amount(row.TotalValue, row.TotalScale), "realized + income + change")

	for _, query := range []string{"", "?from=2026-03-01", "?from=2026-04-01&to=2026-03-31", "?from=2026-03-01&to=2099-01-01", "?from=2026-02-30&to=2026-03-31"} {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains/statement"+query, nil, http.StatusBadRequest)
		var failure errorResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&failure))
		assert.Equal(t, "VALIDATION_FAILED", failure.Error.Code, query)
	}
}

// An absent figure is null on the wire with its reason and counts, never "0".
func TestInvestmentGainsStatementAPIAbsentFiguresAreNull(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "ABSENT")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	// Unknown basis and no price at the opening date; the sale's implied
	// price prices the remaining unit at the closing date.
	in := externalTransferInRequest{EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID, BasisKnowledge: "unknown",
		CommodityID: instrument.CommodityID, QuantityValue: exact.New(2), CostCommodityID: f.commodityID}
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/transfers/external/in", in, http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 6000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)

	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		"/api/v1/investments/gains/statement?from=2026-03-01&to=2026-03-31", nil, http.StatusOK)
	var raw struct {
		Rows []map[string]json.RawMessage `json:"rows"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&raw))
	require.Len(t, raw.Rows, 1)
	row := raw.Rows[0]
	var opening, closing map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(row["opening"], &opening))
	require.NoError(t, json.Unmarshal(row["closing"], &closing))
	assert.JSONEq(t, `null`, string(opening["unrealized_gain_value"]))
	assert.JSONEq(t, `"unpriced_and_unresolved"`, string(opening["unavailable_reason"]))
	assert.JSONEq(t, `1`, string(opening["unpriced_count"]))
	assert.JSONEq(t, `null`, string(opening["latest_price_date"]))
	assert.JSONEq(t, `null`, string(closing["unrealized_gain_value"]))
	assert.JSONEq(t, `"unresolved_basis"`, string(closing["unavailable_reason"]))
	assert.JSONEq(t, `"2026-03-01"`, string(closing["latest_price_date"]), "the price is stated even when basis is unknown")
	assert.JSONEq(t, `null`, string(row["realized_gain_value"]))
	assert.JSONEq(t, `1`, string(row["realized_unresolved_count"]))
	assert.JSONEq(t, `"0"`, string(row["income_value"]), "no dividends is a known zero")
	assert.JSONEq(t, `null`, string(row["unrealized_change_value"]))
	assert.JSONEq(t, `null`, string(row["total_value"]))
}
