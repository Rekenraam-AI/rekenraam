package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An instrument's view lists its activity through the commodity filter
// (#186): a transaction matches when one of its postings is in that
// commodity, so a buy appears under the stock and a cash-only dividend does
// not.
func TestListTransactionsFiltersByPostingCommodity(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buy := buyOn(t, f, "2026-01-10", 10, 10000)
	dividend, err := f.investmentService.Dividend(ctx, DividendInput{
		OwnerUserID: f.ownerUserID, TransactionDate: "2026-02-01",
		CommodityID: &f.stockCommodityID, CashAccountID: f.cashAccountID,
		CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.incomeAccountID,
		AmountValue: 500, AmountScale: 2,
	})
	require.NoError(t, err)

	ids := func(commodityID int64) []int64 {
		t.Helper()
		listed, err := f.transactionService.ListTransactions(ctx, ListTransactionsInput{CommodityID: commodityID})
		require.NoError(t, err)
		out := []int64{}
		for _, transaction := range listed.Transactions {
			out = append(out, transaction.ID)
		}
		return out
	}

	assert.Equal(t, []int64{buy.Transaction.ID}, ids(f.stockCommodityID))
	assert.ElementsMatch(t, []int64{buy.Transaction.ID, dividend.ID}, ids(f.eurCommodityID))
	assert.Empty(t, ids(f.stockCommodityID+f.eurCommodityID+1000))
}
