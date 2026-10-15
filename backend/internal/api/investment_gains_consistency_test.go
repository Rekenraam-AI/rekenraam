package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func gainsDurableRows(t *testing.T, database *sql.DB) map[string][][]any {
	t.Helper()
	result := map[string][][]any{}
	// Include the financial, observation, audit and saved-run tables, comparing
	// complete rows so updates as well as accidental inserts are detected.
	for _, table := range []string{"audit_events", "transactions", "transaction_versions", "journal_entries", "posting_lines", "posting_versions",
		"investment_operations", "investment_lots", "investment_lot_state", "investment_lot_events", "investment_position_basis_state",
		"investment_disposal_decisions", "investment_disposal_revisions", "investment_disposal_revision_allocations",
		"price_observations", "investment_gains_runs"} {
		rows, err := database.Query("SELECT * FROM " + table + " ORDER BY rowid")
		require.NoError(t, err)
		columns, err := rows.Columns()
		require.NoError(t, err)
		for rows.Next() {
			values, destinations := make([]any, len(columns)), make([]any, len(columns))
			for i := range values {
				destinations[i] = &values[i]
			}
			require.NoError(t, rows.Scan(destinations...))
			result[table] = append(result[table], values)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
	}
	return result
}

func TestLiveGainsAPICorrectionBetweenCompositionStages(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"valuation_date_all", "historic_basis_valuation_close"} {
		t.Run(method, func(t *testing.T) {
			handler, database, investments := newSetupTestHandlerWithInvestment(t, HandlerOptions{})
			f := bootstrapInvestmentAPITest(t, handler)
			instrument := createInstrumentForSession(t, handler, f, "CONSISTENT")
			holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
			res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)
			var buy investmentTradeResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&buy))
			doPricingRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/prices", priceObservationRequest{
				BaseCommodityID: instrument.CommodityID, QuoteCommodityID: f.commodityID, QuoteType: "close", AdjustmentBasis: "raw",
				ValuationDate: "2026-03-31", PriceValue: 3000, PriceScale: 2,
				Derivation: json.RawMessage(`{}`), SeriesMetadata: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`)}, http.StatusCreated)
			var afterCorrection map[string][][]any
			investments.SetGainsReadStageForTest(func(stage string) {
				if stage != "operational" {
					return
				}
				// Run the real authenticated correction endpoint while GET is paused.
				path := "/api/v1/investments/transactions/" + itoa(buy.Transaction.ID) + "/replace-buy"
				request := investmentBuyReplacementRequest{Reason: "correct broker cost", Replacement: tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 8000)}
				doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, path, request, http.StatusCreated)
				afterCorrection = gainsDurableRows(t, database)
			})
			path := "/api/v1/investments/gains?valuation_as_of=2026-03-31&alternative_method=person_pooled_average&reporting_currency_id=" + itoa(f.commodityID) + "&fx_method=" + method
			res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path, nil, http.StatusOK)
			var body investmentGainsResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			require.NotNil(t, afterCorrection)
			require.Len(t, body.Unrealized, 1)
			assertMoneyValue(t, 2000, 2, *body.Unrealized[0].UnrealizedGainValue, *body.Unrealized[0].UnrealizedGainScale)
			require.Len(t, body.Alternative.Unrealized, 1)
			assertMoneyValue(t, 2000, 2, *body.Alternative.Unrealized[0].UnrealizedGainValue, *body.Alternative.Unrealized[0].UnrealizedGainScale)
			amount := body.Reporting.UnrealizedTotal.Amount
			require.NotNil(t, amount)
			assert.Zero(t, exact.ScaledIntFromCoefficient(amount.Value, amount.Scale).Cmp(exact.ScaledIntFromInt64(2000, 2)))
			assert.Equal(t, afterCorrection, gainsDurableRows(t, database))
		})
	}
}

func TestLiveStatementAPIObservationWithdrawalBetweenNativeAndReporting(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"valuation_date_all", "historic_basis_valuation_close"} {
		t.Run(method, func(t *testing.T) {
			handler, database, investments := newSetupTestHandlerWithInvestment(t, HandlerOptions{})
			f := bootstrapInvestmentAPITest(t, handler)
			instrument := createInstrumentForSession(t, handler, f, "STABLEQUOTE")
			holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
			doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)
			res := doPricingRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/prices", priceObservationRequest{
				BaseCommodityID: instrument.CommodityID, QuoteCommodityID: f.commodityID, QuoteType: "close", AdjustmentBasis: "raw",
				ValuationDate: "2026-03-31", PriceValue: 3000, PriceScale: 2,
				Derivation: json.RawMessage(`{}`), SeriesMetadata: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`)}, http.StatusCreated)
			var quote priceObservationResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&quote))
			var afterWithdrawal map[string][][]any
			investments.SetGainsReadStageForTest(func(stage string) {
				if stage != "statement" {
					return
				}
				doPricingRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/prices/"+itoa(quote.ID)+"/void", priceObservationVoidRequest{VoidReason: "withdraw during statement conversion"}, http.StatusOK)
				afterWithdrawal = gainsDurableRows(t, database)
			})
			path := "/api/v1/investments/gains/statement?from=2026-03-01&to=2026-03-31&reporting_currency_id=" + itoa(f.commodityID) + "&fx_method=" + method
			res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path, nil, http.StatusOK)
			var body investmentGainsStatementResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			require.NotNil(t, afterWithdrawal)
			require.Len(t, body.Rows, 1)
			assertMoneyValue(t, 2000, 2, *body.Rows[0].Closing.UnrealizedGainValue, *body.Rows[0].Closing.UnrealizedGainScale)
			amount := body.Reporting.Closing.Amount
			require.NotNil(t, amount)
			assert.Zero(t, exact.ScaledIntFromCoefficient(amount.Value, amount.Scale).Cmp(exact.ScaledIntFromInt64(2000, 2)))
			assert.Equal(t, afterWithdrawal, gainsDurableRows(t, database))
			investments.SetGainsReadStageForTest(nil)
			// A failed GET after its replay stages also creates no run or audit.
			doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, path+"&fx_max_age_days=0", nil, http.StatusBadRequest)
			doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?alternative_method=invalid", nil, http.StatusBadRequest)
			assert.Equal(t, afterWithdrawal, gainsDurableRows(t, database))
		})
	}
}
