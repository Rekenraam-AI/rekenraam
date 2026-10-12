package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// #191: a kind outside InvestmentOperationKinds has no capability-matrix row,
// so neither the journal-backed nor the journal-free writer records it.
func TestWritersRefuseAnUnlistedOperationKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, ownerID, currencyID := migratedInvestmentTestDatabase(t)
	cashA := createInvestmentTestAccount(t, database, currencyID)
	cashB := createInvestmentTestAccount(t, database, currencyID)
	count := func() (operations, transactions int) {
		require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM investment_operations`).Scan(&operations))
		require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&transactions))
		return operations, transactions
	}
	operationsBefore, transactionsBefore := count()

	_, err := NewTransactionRepository(database).CreateTransaction(ctx, CreateTransactionParams{
		BookID: 1, ActorUserID: ownerID, RequestID: "unlisted-kind", OriginType: "browser_api",
		Operation: "investment.future", CreatedAt: "2026-06-15T10:00:00Z", ChangeReason: "unlisted kind",
		Spec: TransactionSpec{Status: "posted", TransactionKind: "investment", InvestmentOperationKind: "future_kind",
			TransactionDate: "2026-06-15", MetadataJSON: "{}",
			JournalEntries: []JournalEntrySpec{{EntryDate: "2026-06-15", EntryKind: "investment", Postings: []PostingSpec{
				{LineKey: "a", AccountID: cashA, QuantityValue: exact.New(100), QuantityScale: 2, CommodityID: currencyID, ReconciliationStatus: "uncleared", MetadataJSON: "{}"},
				{LineKey: "b", AccountID: cashB, QuantityValue: exact.New(-100), QuantityScale: 2, CommodityID: currencyID, ReconciliationStatus: "uncleared", MetadataJSON: "{}"},
			}}}},
		AccountRuleDependencies: accountRuleDependenciesFor(t, database, cashA, cashB),
	})
	require.ErrorIs(t, err, ErrUnknownInvestmentOperationKind)

	tx, err := database.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	header := CreateTransactionParams{BookID: 1, ActorUserID: ownerID, OriginType: "browser_api",
		Operation: "investment.future", CreatedAt: "2026-06-15T10:00:00Z", AuditOnly: true,
		Spec: TransactionSpec{Status: "posted", TransactionKind: "investment", InvestmentOperationKind: "future_kind",
			TransactionDate: "2026-06-15"}}
	auditEventID, err := openInvestmentCommandAuditTx(ctx, tx, header)
	require.NoError(t, err)
	_, err = insertJournalFreeInvestmentOperationTx(ctx, tx, header, auditEventID)
	require.ErrorIs(t, err, ErrUnknownInvestmentOperationKind)
	require.NoError(t, tx.Rollback())

	operations, transactions := count()
	require.Equal(t, operationsBefore, operations)
	require.Equal(t, transactionsBefore, transactions)
}
