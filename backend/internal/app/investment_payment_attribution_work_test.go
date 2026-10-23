package app

import (
	"context"
	"sync/atomic"
	"testing"

	sqlite "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestOrdinaryInvestmentWritesNeverReadPaymentAttributionContext(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	for range 10 {
		_, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 500))
		require.NoError(t, err)
	}
	// SQLite compiles parent-FK checks over child identity columns. Those
	// storage checks remain valid; refuse the actual entitlement context.
	var reads atomic.Int64
	connection, err := f.database.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, connection.Raw(func(raw any) error {
		raw.(*sqlite.SQLiteConn).RegisterAuthorizer(func(operation int, table, column, catalog string) int {
			if operation == sqlite.SQLITE_READ && (table == "investment_payment_in_lieu_facts" && (column == "holding_account_id" || column == "commodity_id" || column == "ex_date")) {
				reads.Add(1)
				return sqlite.SQLITE_DENY
			}
			return sqlite.SQLITE_OK
		})
		return nil
	}))
	require.NoError(t, connection.Close())
	t.Cleanup(func() {
		connection, err := f.database.Conn(ctx)
		require.NoError(t, err)
		require.NoError(t, connection.Raw(func(raw any) error { raw.(*sqlite.SQLiteConn).RegisterAuthorizer(nil); return nil }))
		require.NoError(t, connection.Close())
	})
	var count int
	err = f.database.QueryRow(`SELECT holding_account_id FROM investment_payment_in_lieu_facts LIMIT 1`).Scan(&count)
	require.Error(t, err, "authorizer really refuses attribution-context reads")
	reads.Store(0)
	input := shortInput(f, "2026-03-02", 10, 100000)
	_, err = f.investmentService.shortSaleReconciliationImpact(ctx, input)
	require.NoError(t, err)
	opening, err := f.investmentService.ShortSale(ctx, input)
	require.NoError(t, err)
	replace := ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: opening.Transaction.ID, Reason: "date", Replacement: shortInput(f, "2026-04-02", 10, 100000)}
	_, err = f.investmentService.ReplaceShortSaleReconciliationImpact(ctx, replace)
	require.NoError(t, err)
	replacement, err := f.investmentService.ReplaceShortSale(ctx, replace)
	require.NoError(t, err)
	reverse := ReverseInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: replacement.Replacement.Transaction.ID, Reason: "not shorted"}
	_, err = f.investmentService.ReverseShortSaleReconciliationImpact(ctx, reverse)
	require.NoError(t, err)
	_, err = f.investmentService.ReverseShortSale(ctx, reverse)
	require.NoError(t, err)
	require.Zero(t, reads.Load(), "ordinary writes/previews read zero attribution context and replay zero payment entitlements")
}
