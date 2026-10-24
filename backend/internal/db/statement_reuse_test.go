package db

import (
	"context"
	"database/sql"
	"strconv"
	"sync/atomic"
	"testing"

	sqlite "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// countTableWriteCompilations registers an authorizer on the pool's single
// connection that counts how often a statement writing table is compiled. The
// authorizer runs only while SQLite compiles a statement, never while it runs.
func countTableWriteCompilations(t *testing.T, database *sql.DB, operation int, table string) *atomic.Int64 {
	t.Helper()
	ctx := context.Background()
	var compiled atomic.Int64
	conn, err := database.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, conn.Raw(func(raw any) error {
		raw.(*sqlite.SQLiteConn).RegisterAuthorizer(func(op int, target, _, _ string) int {
			if op == operation && target == table {
				compiled.Add(1)
			}
			return sqlite.SQLITE_OK
		})
		return nil
	}))
	require.NoError(t, conn.Close())
	t.Cleanup(func() {
		conn, err := database.Conn(ctx)
		require.NoError(t, err)
		require.NoError(t, conn.Raw(func(raw any) error { raw.(*sqlite.SQLiteConn).RegisterAuthorizer(nil); return nil }))
		require.NoError(t, conn.Close())
	})
	return &compiled
}

// #404: a version with several postings compiles its posting insert once,
// not once per posting, and still writes every posting.
func TestTransactionVersionCompilesItsPostingInsertOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, ownerID, currencyID := migratedInvestmentTestDatabase(t)
	accounts := make([]int64, 4)
	for index := range accounts {
		accounts[index] = createInvestmentTestAccount(t, database, currencyID)
	}
	postingInserts := countTableWriteCompilations(t, database, sqlite.SQLITE_INSERT, "posting_versions")

	// Negative control: the authorizer counts each compilation of the
	// statement, so per-posting preparation would show one per posting.
	tx, err := database.BeginTx(ctx, nil)
	require.NoError(t, err)
	for range 2 {
		statement, err := tx.PrepareContext(ctx, `INSERT INTO posting_versions (book_id, transaction_version_id,
			journal_entry_id, posting_line_id, line_seq, account_id, account_day_sequence, quantity_value,
			quantity_scale, commodity_id, memo, reconciliation_status, cleared_on, metadata_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		require.NoError(t, err)
		require.NoError(t, statement.Close())
	}
	require.NoError(t, tx.Rollback())
	perCompilation := postingInserts.Load() / 2
	require.Positive(t, perCompilation)
	postingInserts.Store(0)

	postings := make([]PostingSpec, 0, len(accounts))
	for index, accountID := range accounts {
		amount := int64(100)
		if index%2 == 1 {
			amount = -100
		}
		postings = append(postings, PostingSpec{LineKey: "line-" + strconv.Itoa(index), AccountID: accountID,
			QuantityValue: exact.New(amount), QuantityScale: 2, CommodityID: currencyID,
			ReconciliationStatus: "uncleared", MetadataJSON: "{}"})
	}
	record, err := NewTransactionRepository(database).CreateTransaction(ctx, CreateTransactionParams{
		BookID: 1, ActorUserID: ownerID, RequestID: "statement-reuse", OriginType: "browser_api",
		Operation: "transaction.create", CreatedAt: "2026-06-15T10:00:00Z", ChangeReason: "statement reuse",
		Spec: TransactionSpec{Status: "posted", TransactionKind: "ordinary", TransactionDate: "2026-06-15",
			MetadataJSON: "{}", JournalEntries: []JournalEntrySpec{{EntryDate: "2026-06-15", EntryKind: "ordinary",
				Postings: postings}}},
		AccountRuleDependencies: accountRuleDependenciesFor(t, database, accounts...),
	})
	require.NoError(t, err)
	require.Equal(t, perCompilation, postingInserts.Load(), "one posting insert compilation for four postings")
	require.Len(t, record.JournalEntries, 1)
	require.Len(t, record.JournalEntries[0].Postings, len(accounts))
	for index, posting := range record.JournalEntries[0].Postings {
		require.Equal(t, accounts[index], posting.AccountID)
		require.EqualValues(t, index+1, posting.LineSeq)
	}
}

func TestStatementSetReusesStatementsAndReportsCompileErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := openTestDatabase(t)
	_, err := database.ExecContext(ctx, `CREATE TABLE reuse_probe (value INTEGER NOT NULL)`)
	require.NoError(t, err)
	inserts := countTableWriteCompilations(t, database, sqlite.SQLITE_INSERT, "reuse_probe")

	tx, err := database.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	statements := newStatementSet(tx)
	for value := range 3 {
		_, err := statements.ExecContext(ctx, `INSERT INTO reuse_probe (value) VALUES (?)`, value)
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), inserts.Load())
	var count int
	require.NoError(t, statements.QueryRowContext(ctx, `SELECT count(*) FROM reuse_probe WHERE value >= ?`, 0).Scan(&count))
	require.Equal(t, 3, count)

	// A statement that does not compile fails where the unprepared call did.
	_, err = statements.ExecContext(ctx, `INSERT INTO missing_table (value) VALUES (?)`, 1)
	require.ErrorContains(t, err, "no such table")
	require.ErrorContains(t, statements.QueryRowContext(ctx, `SELECT value FROM missing_table`).Scan(&count), "no such table")
	_, err = statements.QueryContext(ctx, `SELECT value FROM missing_table`)
	require.ErrorContains(t, err, "no such table")

	statements.Close()
	require.Empty(t, statements.statements)
	_, err = statements.ExecContext(ctx, `INSERT INTO reuse_probe (value) VALUES (?)`, 3)
	require.NoError(t, err, "a closed set prepares again on next use")
	require.Equal(t, int64(2), inserts.Load())
	statements.Close()
	require.NoError(t, tx.Commit())
}
