package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #187: an account posting may post on a later (or earlier) day than the
// transaction date. These cases pin the dated journal the service builds.

type postedOnFixture struct {
	*investmentsTestFixture
	savingsAccountID  int64
	clearingAccountID int64
}

func newPostedOnFixture(t *testing.T) postedOnFixture {
	t.Helper()
	f := newInvestmentsTestFixture(t)
	return postedOnFixture{
		investmentsTestFixture: f,
		savingsAccountID:       seedTestAccount(t, f.database, "active", true),
		clearingAccountID:      seedTransferClearingAccount(t, f.database),
	}
}

func seedTransferClearingAccount(t *testing.T, database *sql.DB) int64 {
	t.Helper()
	ctx := context.Background()
	result, err := database.ExecContext(ctx, `
		INSERT INTO accounts (book_id, system_role, created_at, created_by_user_id)
		VALUES (?, 'transfer_clearing', '2026-01-01T00:00:00Z', 1)
	`, BookID)
	require.NoError(t, err)
	accountID, err := result.LastInsertId()
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, `
		INSERT INTO account_versions (
			account_id, version_seq, effective_from, recorded_at, changed_by_user_id,
			change_reason, status, opened_on, code, name, account_class,
			account_kind, allows_postings
		) VALUES (?, 1, '0001-01-01', '2026-01-01T00:00:00Z', 1, 'test fixture', 'active', '0001-01-01', 'CLEARING', 'Transfer Clearing', 'asset', 'receivable', 1)
	`, accountID)
	require.NoError(t, err)
	return accountID
}

type datedPosting struct {
	date      string
	accountID int64
	value     string
}

func journalShape(transaction Transaction) []datedPosting {
	shape := []datedPosting{}
	for _, entry := range transaction.JournalEntries {
		for _, posting := range entry.Postings {
			shape = append(shape, datedPosting{entry.EntryDate, posting.AccountID, posting.QuantityValue.String()})
		}
	}
	return shape
}

func (f postedOnFixture) create(t *testing.T, spec TransactionInput) (Transaction, error) {
	t.Helper()
	return f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api", Spec: spec,
	})
}

func TestPostedOnCardSpendMovesTheWholeEntryToThePostingDay(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	created, err := f.create(t, TransactionInput{
		TransactionDate: "2026-03-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, PostedOn: "2026-03-03", CommodityID: f.eurCommodityID, QuantityValue: exact.New(-2500), QuantityScale: 2},
			{AccountID: f.incomeAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(2500), QuantityScale: 2},
		}}},
	})
	require.NoError(t, err)

	assert.Equal(t, "2026-03-01", created.TransactionDate, "the transaction date stays the purchase date")
	assert.Equal(t, []datedPosting{
		{"2026-03-03", f.cashAccountID, "-2500"},
		{"2026-03-03", f.incomeAccountID, "2500"},
	}, journalShape(created), "one account: categories follow its posting day, no clearing")
}

func TestPostedOnTwoDayTransferBridgesThroughTransferClearing(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	created, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.savingsAccountID, PostedOn: "2026-06-03", CommodityID: f.eurCommodityID, QuantityValue: exact.New(10000), QuantityScale: 2},
			{AccountID: f.cashAccountID, PostedOn: "2026-06-01", CommodityID: f.eurCommodityID, QuantityValue: exact.New(-10000), QuantityScale: 2},
		}}},
	})
	require.NoError(t, err)

	assert.Equal(t, []datedPosting{
		{"2026-06-01", f.cashAccountID, "-10000"},
		{"2026-06-01", f.clearingAccountID, "10000"},
		{"2026-06-03", f.savingsAccountID, "10000"},
		{"2026-06-03", f.clearingAccountID, "-10000"},
	}, journalShape(created), "one balanced entry per date, ordered by date")

	register, err := f.transactionService.Register(context.Background(), f.savingsAccountID, ListTransactionsInput{})
	require.NoError(t, err)
	require.Len(t, register.Entries, 1)
	assert.Equal(t, "2026-06-03", register.Entries[0].EntryDate, "savings lists the transfer on its arrival day")
}

func TestPostedOnSplitWithTwoAccountsKeepsCategoriesOnTheTransactionDate(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	created, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, PostedOn: "2026-06-02", CommodityID: f.eurCommodityID, QuantityValue: exact.New(-150), QuantityScale: 0},
			{AccountID: f.savingsAccountID, PostedOn: "2026-06-04", CommodityID: f.eurCommodityID, QuantityValue: exact.New(100), QuantityScale: 0},
			{AccountID: f.incomeAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(50), QuantityScale: 0},
		}}},
	})
	require.NoError(t, err)

	assert.Equal(t, []datedPosting{
		{"2026-06-01", f.incomeAccountID, "50"},
		{"2026-06-01", f.clearingAccountID, "-50"},
		{"2026-06-02", f.cashAccountID, "-150"},
		{"2026-06-02", f.clearingAccountID, "150"},
		{"2026-06-04", f.savingsAccountID, "100"},
		{"2026-06-04", f.clearingAccountID, "-100"},
	}, journalShape(created))
}

func TestPostedOnBridgeAlignsMixedScalesPerCommodity(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	// -1.5 at scale 1 against +1.50 at scale 2: balanced, but only when
	// aligned. The bridge carries the deeper scale.
	created, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-15), QuantityScale: 1},
			{AccountID: f.savingsAccountID, PostedOn: "2026-06-02", CommodityID: f.eurCommodityID, QuantityValue: exact.New(150), QuantityScale: 2},
		}}},
	})
	require.NoError(t, err)
	require.Len(t, created.JournalEntries, 2)
	bridge := created.JournalEntries[0].Postings[1]
	assert.Equal(t, f.clearingAccountID, bridge.AccountID)
	assert.Equal(t, "15", bridge.QuantityValue.String())
	assert.Equal(t, 1, bridge.QuantityScale)
}

func TestPostedOnRejectsAnUnbalancedTransactionBeforeBridging(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	_, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-100)},
			{AccountID: f.savingsAccountID, PostedOn: "2026-06-02", CommodityID: f.eurCommodityID, QuantityValue: exact.New(90)},
		}}},
	})
	var validation ValidationError
	require.ErrorAs(t, err, &validation, "clearing must never absorb an imbalance")
	assert.Zero(t, f.transactionCount(t))
}

func TestPostedOnRejectsAPostedDateOnACategory(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	_, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-100)},
			{AccountID: f.incomeAccountID, PostedOn: "2026-06-02", CommodityID: f.eurCommodityID, QuantityValue: exact.New(100)},
		}}},
	})
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Contains(t, validation.Message, "account posting")
}

func TestPostedOnRejectsSeveralJournalEntries(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	leg := func(accountID int64, value int64, postedOn string) PostingInput {
		return PostingInput{AccountID: accountID, PostedOn: postedOn, CommodityID: f.eurCommodityID, QuantityValue: exact.New(value)}
	}
	_, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{
			{Postings: []PostingInput{leg(f.cashAccountID, -1, "2026-06-02"), leg(f.incomeAccountID, 1, "")}},
			{Postings: []PostingInput{leg(f.cashAccountID, -1, ""), leg(f.incomeAccountID, 1, "")}},
		},
	})
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
}

func TestPostedOnEditThatMovesAPostingOutOfAReconciledPeriodIsGuarded(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)
	ctx := context.Background()

	spec := func(postedOn string) TransactionInput {
		return TransactionInput{
			TransactionDate: "2026-01-20",
			JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
				{AccountID: f.cashAccountID, PostedOn: postedOn, CommodityID: f.eurCommodityID, QuantityValue: exact.New(40)},
				{AccountID: f.incomeAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-40)},
			}}},
		}
	}
	created, err := f.create(t, spec("2026-01-20"))
	require.NoError(t, err)
	checkpointID := reconcileCash(t, f.investmentsTestFixture, "2026-01-31", 40)

	update := UpdateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api", TransactionID: created.ID,
		Spec: spec("2026-02-02"), ChangeReason: "bank posted it in February",
	}
	_, err = f.transactionService.UpdateTransaction(ctx, update)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired, "leaving a reconciled period changes its balance")
	assert.Equal(t, []int64{checkpointID}, activeCheckpointIDs(t, f.investmentsTestFixture))

	update.ReconciliationOverride = true
	updated, err := f.transactionService.UpdateTransaction(ctx, update)
	require.NoError(t, err)
	assert.Equal(t, "2026-02-02", updated.JournalEntries[0].EntryDate)
	assert.Empty(t, activeCheckpointIDs(t, f.investmentsTestFixture), "the override invalidates the checkpoint")
}

func TestPostedOnTransferPassesSelfCheckAndExportsEachLegOnItsDay(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)
	ctx := context.Background()

	_, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-10000), QuantityScale: 2},
			{AccountID: f.savingsAccountID, PostedOn: "2026-06-03", CommodityID: f.eurCommodityID, QuantityValue: exact.New(10000), QuantityScale: 2},
		}}},
	})
	require.NoError(t, err)

	run, err := NewSelfCheckService(db.NewSelfCheckRepository(f.database, f.database)).RunSelfCheck(ctx, "manual")
	require.NoError(t, err)
	for _, result := range run.Results {
		assert.NotEqual(t, "fail", result.Status, "%s: %s", result.CheckID, result.Summary)
	}
	assert.Zero(t, run.FailedCheckCount)

	var out bytes.Buffer
	_, err = NewExportService(db.NewExportRepository(f.database)).WriteLedgerCSV(ctx, &out)
	require.NoError(t, err)
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(out.String(), utf8ByteOrderMark))).ReadAll()
	require.NoError(t, err)
	column := map[string]int{}
	for index, name := range records[0] {
		column[name] = index
	}
	entryDates := map[string][]string{}
	for _, record := range records[1:] {
		accountID := record[column["account_id"]]
		entryDates[accountID] = append(entryDates[accountID], record[column["entry_date"]])
	}
	assert.Equal(t, map[string][]string{
		strconv.FormatInt(f.cashAccountID, 10):     {"2026-06-01"},
		strconv.FormatInt(f.savingsAccountID, 10):  {"2026-06-03"},
		strconv.FormatInt(f.clearingAccountID, 10): {"2026-06-01", "2026-06-03"},
	}, entryDates, "each leg exports on its own posting day")
}

func (f postedOnFixture) moveDay(t *testing.T, transaction Transaction, accountID int64, direction string, override bool) (Transaction, error) {
	t.Helper()
	var lineID int64
	for _, entry := range transaction.JournalEntries {
		for _, posting := range entry.Postings {
			if posting.AccountID == accountID {
				lineID = posting.PostingLineID
			}
		}
	}
	require.NotZero(t, lineID)
	return f.transactionService.MovePostingToDay(context.Background(), MovePostingToDayInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api", AccountID: accountID, PostingLineID: lineID,
		Direction: direction, ReconciliationOverride: override,
	})
}

func TestMovePostingToDayMovesACardSpendByCalendarDay(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	created, err := f.create(t, TransactionInput{
		TransactionDate: "2026-03-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-25)},
			{AccountID: f.incomeAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(25)},
		}}},
	})
	require.NoError(t, err)

	later, err := f.moveDay(t, created, f.cashAccountID, "later", false)
	require.NoError(t, err)
	assert.Equal(t, "2026-03-01", later.TransactionDate)
	assert.Equal(t, []datedPosting{
		{"2026-03-02", f.cashAccountID, "-25"},
		{"2026-03-02", f.incomeAccountID, "25"},
	}, journalShape(later), "the category follows the only account")

	back, err := f.moveDay(t, later, f.cashAccountID, "earlier", false)
	require.NoError(t, err)
	before, err := f.moveDay(t, back, f.cashAccountID, "earlier", false)
	require.NoError(t, err)
	assert.Equal(t, []datedPosting{
		{"2026-02-28", f.cashAccountID, "-25"},
		{"2026-02-28", f.incomeAccountID, "25"},
	}, journalShape(before), "calendar days, across a month edge")
}

func TestMovePostingToDayBridgesATransferAndUnbridgesItBack(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	created, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-100)},
			{AccountID: f.savingsAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(100)},
		}}},
	})
	require.NoError(t, err)

	moved, err := f.moveDay(t, created, f.savingsAccountID, "later", false)
	require.NoError(t, err)
	assert.Equal(t, []datedPosting{
		{"2026-06-01", f.cashAccountID, "-100"},
		{"2026-06-01", f.clearingAccountID, "100"},
		{"2026-06-02", f.savingsAccountID, "100"},
		{"2026-06-02", f.clearingAccountID, "-100"},
	}, journalShape(moved))

	back, err := f.moveDay(t, moved, f.savingsAccountID, "earlier", false)
	require.NoError(t, err)
	assert.Equal(t, []datedPosting{
		{"2026-06-01", f.cashAccountID, "-100"},
		{"2026-06-01", f.savingsAccountID, "100"},
	}, journalShape(back), "one day again: the bridge is gone")
	assert.Equal(t, postingLineIDs(created), postingLineIDs(back), "the account postings keep their lines")
}

func postingLineIDs(transaction Transaction) map[int64]int64 {
	lines := map[int64]int64{}
	for _, entry := range transaction.JournalEntries {
		for _, posting := range entry.Postings {
			lines[posting.AccountID] = posting.PostingLineID
		}
	}
	return lines
}

func TestMovePostingToDayOutOfAReconciledPeriodIsGuarded(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	created := postEUR(t, f.investmentsTestFixture, "2026-01-31", 40)
	checkpointID := reconcileCash(t, f.investmentsTestFixture, "2026-01-31", 40)

	_, err := f.moveDay(t, created, f.cashAccountID, "later", false)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	assert.Equal(t, []int64{checkpointID}, activeCheckpointIDs(t, f.investmentsTestFixture))

	moved, err := f.moveDay(t, created, f.cashAccountID, "later", true)
	require.NoError(t, err)
	assert.Equal(t, "2026-02-01", moved.JournalEntries[0].EntryDate)
	assert.Empty(t, activeCheckpointIDs(t, f.investmentsTestFixture))
}

func TestMovePostingToDayRefusesACategoryPostingAndAnotherAccount(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)
	created := postEUR(t, f.investmentsTestFixture, "2026-06-01", 10)

	_, err := f.moveDay(t, created, f.incomeAccountID, "later", false)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)

	var cashLine int64
	for _, posting := range created.JournalEntries[0].Postings {
		if posting.AccountID == f.cashAccountID {
			cashLine = posting.PostingLineID
		}
	}
	_, err = f.transactionService.MovePostingToDay(context.Background(), MovePostingToDayInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api", AccountID: f.savingsAccountID, PostingLineID: cashLine, Direction: "later",
	})
	require.ErrorIs(t, err, ErrTransactionNotFound)
}

func TestMovePostingToDayRefusesAJournalTheRuleWouldReshape(t *testing.T) {
	t.Parallel()
	f := newPostedOnFixture(t)

	// Two dated entries of one account with their own categories: one entry
	// with posted dates would move the second category to the transaction
	// date, so the move refuses rather than reshape it.
	leg := func(accountID int64, value int64) PostingInput {
		return PostingInput{AccountID: accountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(value)}
	}
	created, err := f.create(t, TransactionInput{
		TransactionDate: "2026-06-01",
		JournalEntries: []JournalEntryInput{
			{EntryDate: "2026-06-01", Postings: []PostingInput{leg(f.cashAccountID, -10), leg(f.incomeAccountID, 10)}},
			{EntryDate: "2026-06-02", Postings: []PostingInput{leg(f.savingsAccountID, -5), leg(f.incomeAccountID, 5)}},
		},
	})
	require.NoError(t, err)

	_, err = f.moveDay(t, created, f.cashAccountID, "later", false)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Contains(t, validation.Message, "edit it instead")
}
