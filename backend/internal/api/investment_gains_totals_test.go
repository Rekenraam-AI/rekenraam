package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/exact"
)

// These pin the operational wire contract of native realized totals (#399):
// first-appearance currency order, one exact total per currency at the depth
// its sum needed, NULL with a count when any disposal's basis is unknown, and
// an int64 range failure at full scale (no trailing-zero normalization, unlike
// the statement) that wins even over an unknown total.
func realizedEntry(currency, value int64, scale int, knowledge string) app.RealizedGainEntry {
	return app.RealizedGainEntry{CostCommodityID: currency, RealizedGainValue: value, RealizedGainScale: scale, BasisKnowledge: knowledge}
}

func realizedTotalsJSON(t *testing.T, realized []app.RealizedGainEntry) string {
	t.Helper()
	response, err := gainsProjectionResponse(app.GainsProfile{}, realized, nil)
	require.NoError(t, err)
	body, err := json.Marshal(response.RealizedTotals)
	require.NoError(t, err)
	return string(body)
}

func TestNativeRealizedTotalsWireContract(t *testing.T) {
	for _, tc := range []struct {
		name     string
		realized []app.RealizedGainEntry
		want     string
	}{
		{"empty is an empty list", nil, `[]`},
		{"first appearance order, mixed scales summed exactly", []app.RealizedGainEntry{
			realizedEntry(9, 1, 2, "known"), realizedEntry(3, 5, 0, "known"),
			realizedEntry(9, 20, 0, "known"), realizedEntry(9, -2, 2, "known"), realizedEntry(3, 125, 3, "known"),
		}, `[{"cost_commodity_id":9,"total_gain_value":"1999","total_gain_scale":2,"basis_knowledge":"known","unresolved_count":0},` +
			`{"cost_commodity_id":3,"total_gain_value":"5125","total_gain_scale":3,"basis_knowledge":"known","unresolved_count":0}]`},
		{"known zero keeps its scale", []app.RealizedGainEntry{realizedEntry(4, 0, 2, "known")},
			`[{"cost_commodity_id":4,"total_gain_value":"0","total_gain_scale":2,"basis_knowledge":"known","unresolved_count":0}]`},
		{"one unknown disposal withholds its currency only", []app.RealizedGainEntry{
			realizedEntry(9, 100, 2, "known"), realizedEntry(9, 0, 0, "unknown"), realizedEntry(3, 7, 0, "known"), realizedEntry(9, 0, 0, "unknown"),
		}, `[{"cost_commodity_id":9,"total_gain_value":null,"total_gain_scale":null,"basis_knowledge":"unknown","unresolved_count":2},` +
			`{"cost_commodity_id":3,"total_gain_value":"7","total_gain_scale":0,"basis_knowledge":"known","unresolved_count":0}]`},
		{"only unknown disposals", []app.RealizedGainEntry{realizedEntry(9, 0, 0, "unknown")},
			`[{"cost_commodity_id":9,"total_gain_value":null,"total_gain_scale":null,"basis_knowledge":"unknown","unresolved_count":1}]`},
		{"above JavaScript safe integer is a string", []app.RealizedGainEntry{
			realizedEntry(9, 9007199254740993, 0, "known"), realizedEntry(9, 1, 0, "known"),
		}, `[{"cost_commodity_id":9,"total_gain_value":"9007199254740994","total_gain_scale":0,"basis_knowledge":"known","unresolved_count":0}]`},
		{"int64 bounds fit", []app.RealizedGainEntry{realizedEntry(9, math.MaxInt64, 0, "known"), realizedEntry(3, math.MinInt64, 0, "known")},
			`[{"cost_commodity_id":9,"total_gain_value":"9223372036854775807","total_gain_scale":0,"basis_knowledge":"known","unresolved_count":0},` +
				`{"cost_commodity_id":3,"total_gain_value":"-9223372036854775808","total_gain_scale":0,"basis_knowledge":"known","unresolved_count":0}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, realizedTotalsJSON(t, tc.realized))
		})
	}
}

func TestNativeRealizedTotalsRangeFailures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		realized []app.RealizedGainEntry
	}{
		{"sum above int64", []app.RealizedGainEntry{realizedEntry(9, math.MaxInt64, 0, "known"), realizedEntry(9, 1, 0, "known")}},
		{"sum below int64", []app.RealizedGainEntry{realizedEntry(9, math.MinInt64, 0, "known"), realizedEntry(9, -1, 0, "known")}},
		// 922337203685477581 aligned to scale 1 is 9223372036854775810: it
		// would fit once its trailing zero were dropped, but the operational
		// total keeps the full scale and refuses.
		{"full scale is not normalized", []app.RealizedGainEntry{realizedEntry(9, 922337203685477581, 0, "known"), realizedEntry(9, 0, 1, "known")}},
		{"range failure wins over unknown basis", []app.RealizedGainEntry{
			realizedEntry(9, math.MaxInt64, 0, "known"), realizedEntry(9, 0, 0, "unknown"), realizedEntry(9, 1, 0, "known"),
		}},
		{"later currency overflow fails the whole read", []app.RealizedGainEntry{
			realizedEntry(3, 1, 0, "known"), realizedEntry(9, math.MaxInt64, 0, "known"), realizedEntry(9, 1, 0, "known"),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := gainsProjectionResponse(app.GainsProfile{}, tc.realized, nil)
			require.ErrorIs(t, err, exact.ErrInt64Range)
		})
	}
}

// The saved gains run formats through the same projection; its bytes are
// immutable once stored.
func TestGainsRunOutputBytesForRealizedTotals(t *testing.T) {
	transactionID := int64(41)
	output, err := formatGainsRun(app.GainsRunProjection{
		Profile: app.GainsProfile{BasisMethod: "fifo", From: "2026-01-01", To: "2026-12-31"},
		Realized: []app.RealizedGainEntry{
			{AccountID: 5, CommodityID: 6, CostCommodityID: 9, DisposalDate: "2026-02-02", TransactionID: &transactionID,
				QuantityValue: "1", DisposedBasisValue: 1099, DisposedBasisScale: 2, ProceedsValue: 11, ProceedsScale: 0,
				RealizedGainValue: 1, RealizedGainScale: 2, BasisKnowledge: "known", PositionSide: "long"},
			{AccountID: 5, CommodityID: 6, CostCommodityID: 9, DisposalDate: "2026-02-04",
				QuantityValue: "1", ProceedsValue: 30, ProceedsScale: 0, BasisKnowledge: "unknown", PositionSide: "long"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, `{"profile":{"from":"2026-01-01","to":"2026-12-31","name":"","purpose":"","basis_method":"fifo","valuation_as_of":"","knowledge_cutoff":"","ledger_knowledge":"","quote_policy":"","stale_after_days":0,"fx_method":"","rounding":""},`+
		`"capital_returns":[],"currencies":[],`+
		`"realized":[{"account_id":5,"commodity_id":6,"cost_commodity_id":9,"disposal_date":"2026-02-02","transaction_id":41,"quantity_value":"1","quantity_scale":0,"disposed_basis_value":"1099","disposed_basis_scale":2,"proceeds_value":"11","proceeds_scale":0,"realized_gain_value":"1","realized_gain_scale":2,"basis_knowledge":"known","position_side":"long"},`+
		`{"account_id":5,"commodity_id":6,"cost_commodity_id":9,"disposal_date":"2026-02-04","quantity_value":"1","quantity_scale":0,"disposed_basis_value":null,"disposed_basis_scale":null,"proceeds_value":"30","proceeds_scale":0,"realized_gain_value":null,"realized_gain_scale":null,"basis_knowledge":"unknown","position_side":"long"}],`+
		`"unrealized":[],"realized_totals":[{"cost_commodity_id":9,"total_gain_value":null,"total_gain_scale":null,"basis_knowledge":"unknown","unresolved_count":1}]}`, string(output))
}

// Each sale's gain fits int64, but their exact total in one currency does not:
// the read refuses instead of rounding or wrapping. Separate cash accounts keep
// every ledger balance representable.
func TestRealizedTotalOutOfRangeIsLedgerOverflowOverHTTP(t *testing.T) {
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "WIDE")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	secondCash := createLedgerAccount(t, handler, f.sessionCookie, f.csrfToken, "Second cash", "asset", "checking", f.commodityID, 6)
	for i, cashID := range []int64{f.cashAccount.ID, secondCash.ID} {
		buy := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 1)
		buy.TransactionDate = fmt.Sprintf("2026-03-%02d", 2*i+1)
		buy.CashAccountID = cashID
		doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
		sell := buy
		sell.TransactionDate = fmt.Sprintf("2026-03-%02d", 2*i+2)
		sell.CashAmountValue = moneyCoefficient(500000000000000)
		doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sell, http.StatusCreated)
	}
	for _, day := range []string{"2026-03-02", "2026-03-04"} {
		one := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains?from="+day+"&to="+day, nil, http.StatusOK)
		require.Contains(t, one.Body.String(), `"realized_totals":[{"cost_commodity_id":`+fmt.Sprint(f.commodityID)+`,"total_gain_value":"4999999999999990000","total_gain_scale":6,`)
	}
	refused := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/gains", nil, http.StatusUnprocessableEntity)
	require.Contains(t, refused.Body.String(), `"LEDGER_OVERFLOW"`)
}
