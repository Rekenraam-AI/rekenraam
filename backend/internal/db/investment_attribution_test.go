package db

import (
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// Independent quantities: 10 borrowed, 4 covered on March 30, 6 on March 31.
// The April 1 entitlement close owes zero; a cover on April 1 leaves 6 owed
// at that close. Neither the current projection nor opening existence answers it.
func TestPaymentInLieuAttributionDatedQuantities(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, coverDate string
		warnings        []string
	}{
		{"covered at close", "2026-03-31", []string{"no_short_at_entitlement_close"}},
		{"covered on ex-date", "2026-04-01", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			movements := []EffectiveLotMovement{
				{AccountID: 1, CommodityID: 2, CostCommodityID: 3, EventDate: "2026-03-02", QuantityValue: exact.New(10)},
				{AccountID: 1, CommodityID: 2, CostCommodityID: 3, EventDate: "2026-03-30", QuantityValue: exact.New(-4)},
				{AccountID: 1, CommodityID: 2, CostCommodityID: 3, EventDate: tc.coverDate, QuantityValue: exact.New(-6)},
			}
			payments := []paymentInLieuAttributionRecord{{TransactionID: 7, PaymentDate: "2026-04-03",
				Fact: PaymentInLieuFact{HoldingAccountID: 1, CommodityID: 2, ExDate: "2026-04-01"}}}
			got := evaluatePaymentInLieuAttributions(payments, movements)
			require.Len(t, got, 1)
			require.False(t, got[0].Unavailable)
			require.Equal(t, tc.warnings, got[0].WarningReasons)
		})
	}
}

func TestPaymentInLieuAttributionSeparatesCurrenciesAndMissingContext(t *testing.T) {
	t.Parallel()
	payments := []paymentInLieuAttributionRecord{
		{TransactionID: 1, PaymentDate: "2026-03-31", Fact: PaymentInLieuFact{HoldingAccountID: 1, CommodityID: 2, ExDate: "2026-04-01"}},
		{TransactionID: 2, PaymentDate: "2026-04-03", Fact: PaymentInLieuFact{HoldingAccountID: 1}},
		{TransactionID: 3, PaymentDate: "2026-04-03", Fact: PaymentInLieuFact{HoldingAccountID: 1, CommodityID: 2, ExDate: "2026-03-01"}},
		{TransactionID: 4, PaymentDate: "2026-03-31", Fact: PaymentInLieuFact{ExDate: "2026-04-01"}},
		{TransactionID: 5, PaymentDate: "2026-02-28", Fact: PaymentInLieuFact{HoldingAccountID: 1, CommodityID: 2, ExDate: "2026-03-01"}},
	}
	// A corrupt negative group cannot cancel the independent positive group.
	// Financial checks own that corruption; attribution still finds support.
	movements := []EffectiveLotMovement{
		{AccountID: 1, CommodityID: 2, CostCommodityID: 3, EventDate: "2026-03-02", QuantityValue: exact.New(5), QuantityScale: 1},
		{AccountID: 1, CommodityID: 2, CostCommodityID: 4, EventDate: "2026-03-02", QuantityValue: exact.New(-5), QuantityScale: 1},
	}
	got := evaluatePaymentInLieuAttributions(payments, movements)
	require.Equal(t, []string{"ex_date_after_payment"}, got[0].WarningReasons)
	require.True(t, got[1].Unavailable)
	require.Empty(t, got[1].WarningReasons)
	require.Equal(t, []string{"no_short_at_entitlement_close"}, got[2].WarningReasons)
	require.True(t, got[3].Unavailable)
	require.Equal(t, []string{"ex_date_after_payment"}, got[3].WarningReasons)
	require.Equal(t, []string{"ex_date_after_payment", "no_short_at_entitlement_close"}, got[4].WarningReasons)
}
