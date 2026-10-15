package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

func TestUnknownProjectedBasisAPIKeepsQuantityAndReturnsNullBasis(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "NULLBASIS")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	// Use real unknown acquisition evidence so both the current lot state and
	// dated replay retain unknown basis. A partial sale supplies a price while
	// leaving two units, without damaging the rebuildable projection by hand.
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/transfers/external/in", externalTransferInRequest{
			EffectiveOn: "2026-02-01", HoldingAccountID: holding.ID, CommodityID: instrument.CommodityID,
			QuantityValue: exact.New(3), CostCommodityID: f.commodityID, BasisKnowledge: "unknown",
		}, http.StatusCreated)
	sale := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 2000)
	sale.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/sell", sale, http.StatusCreated)
	for _, test := range []struct{ path, key string }{
		{"/api/v1/investments/lots", "lots"},
		{"/api/v1/investments/positions", "positions"},
		{"/api/v1/investments/gains", "unrealized"},
	} {
		t.Run(test.key, func(t *testing.T) {
			result := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, test.path, nil, http.StatusOK)
			var body map[string]json.RawMessage
			require.NoError(t, json.NewDecoder(result.Body).Decode(&body))
			var rows []map[string]any
			require.NoError(t, json.Unmarshal(body[test.key], &rows))
			require.Len(t, rows, 1)
			require.Equal(t, "unknown", rows[0]["basis_knowledge"])
			for _, key := range []string{"remaining_cost_basis_value", "remaining_cost_basis_scale"} {
				require.Contains(t, rows[0], key, "NULL is explicit, not an omitted field")
				require.Nil(t, rows[0][key])
			}
			if test.key == "lots" {
				require.Equal(t, "2", rows[0]["remaining_quantity_value"])
			} else {
				require.Equal(t, "2", rows[0]["quantity_value"])
			}
			if test.key == "unrealized" {
				require.Equal(t, "unknown_basis", rows[0]["gain_unavailable"])
				require.NotContains(t, rows[0], "unrealized_gain_value")
				require.Contains(t, rows[0], "market_value_value")
			}
		})
	}
}
