package app

import (
	"context"
	"testing"

	"rekenraam/backend/internal/exact"

	"github.com/stretchr/testify/require"
)

// #409: a trade is recognised on its trade date. Its units post that day and
// its cash on the settlement day, so reading each entry on its own date left
// the bought units and the cash not yet paid both inside the portfolio
// between the two. Every case keeps TEST at 100, so no figure may move.
// Fixture: V(open) = 100 broker cash + 10 TEST at 100 = 1,100.

func (f *returnsFixture) settledBuy(t *testing.T, tradeDate, settlementDate string, cashAccountID int64) Transaction {
	t.Helper()
	trade, err := f.investmentService.Buy(context.Background(), InvestmentTradeInput{
		OwnerUserID: f.ownerUserID, TransactionDate: tradeDate, SettlementDate: settlementDate, CommodityID: f.stockCommodityID,
		HoldingAccountID: f.holdingAccountID, CashAccountID: cashAccountID,
		QuantityValue: exact.New(1), CashAmountValue: 10000, CashAmountScale: 2, CashCommodityID: f.eurCommodityID,
	})
	require.NoError(t, err)
	return trade.Transaction
}

// requireAllocationIsClosingValue checks that the allocation's denominator is
// the closing value, so a pending settlement is not counted by one and
// dropped by the other.
func requireAllocationIsClosingValue(t *testing.T, result ReturnsResult) {
	t.Helper()
	require.Equal(t, AllocationBasisClosingValue, result.Allocation.Basis)
	sum := exact.NewScaledInt()
	for _, row := range result.Allocation.ByAccount {
		sum.AddCoefficient(row.Value.QuantityValue, row.Value.QuantityScale)
	}
	closing := result.ClosingValue.Amount
	require.Zero(t, sum.Cmp(exact.ScaledIntFromCoefficient(closing.QuantityValue, closing.QuantityScale)))
}

// R1, the #409 reproducer: traded on the opening boundary from broker cash,
// settled inside the period. Before the fix: opening 1,200, TTWROR −0.083333.
func TestReturnsRecognisesATradeSettledInsideThePeriodOnItsTradeDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1100, result.OpeningValue)
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 0, result.Withdrawals)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// R2: the same trade paid from checking, outside the portfolio. Its
// contribution is recognised on the trade date, before the period, so the
// period has no flow. Before the fix: a contribution of 100 on 02-02.
func TestReturnsRecognisesAnOutsideFundedTradeOnItsTradeDate(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-01-31", "2026-02-02", f.cashAccountID)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1200, result.OpeningValue)
	requireReturnsAmount(t, 1200, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
}

// R3: traded inside the period from broker cash, settled after it ends. The
// cash leg dated after `to` still belongs to the period. Before the fix:
// closing 1,200, TTWROR 0.090909.
func TestReturnsCountsASettlementAfterThePeriodInTheClosingValue(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-02-27", "2026-03-02", f.brokerCashID)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
	requireAllocationIsClosingValue(t, result)
}

// R4: traded inside the period from checking, settled after it ends: a
// contribution of 100 on the trade date. Before the fix: no contribution,
// TTWROR 0.090909.
func TestReturnsCountsAnOutsideFundedTradeSettlingAfterThePeriod(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.settledBuy(t, "2026-02-27", "2026-03-02", f.cashAccountID)
	// The inflow needs V(02-26); 01-31's price is beyond the 7-day window.
	f.price(t, "2026-02-26", 10000)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1200, result.ClosingValue)
	requireReturnsAmount(t, 100, result.Contributions)
	requireReturnsAmount(t, 0, result.Gain)
	requireReturn(t, "0.000000", result.TimeWeighted)
	requireReturn(t, "0.000000", result.MoneyWeightedPeriod)
}

// R5: a reversal of R1's trade cancels it on the trade date as well.
func TestReturnsReversedSettledTradeLeavesNoFigure(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	trade := f.settledBuy(t, "2026-01-31", "2026-02-02", f.brokerCashID)
	_, err := acknowledgedReverseBuy(context.Background(), f.investmentService, ReverseInvestmentBuyInput{
		OwnerUserID: f.ownerUserID, TransactionID: trade.ID, Reason: "wrong account"})
	require.NoError(t, err)
	f.price(t, "2026-02-28", 10000)

	result := f.read(t, "2026-02-01", "2026-02-28")
	requireReturnsAmount(t, 1100, result.OpeningValue)
	requireReturnsAmount(t, 1100, result.ClosingValue)
	requireReturnsAmount(t, 0, result.Contributions)
	requireReturnsAmount(t, 0, result.Withdrawals)
	requireReturn(t, "0.000000", result.TimeWeighted)
}
