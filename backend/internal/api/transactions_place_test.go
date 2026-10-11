package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #185: placing a transaction before or after another states one same-day
// fact. It applies in the journal order and in every register holding both on
// that day; an account reordered on its own keeps that order where the fact
// does not contradict it; reconciled boundaries stay guarded per account.

// registerOrder lists an account's transactions oldest first.
func registerOrder(t *testing.T, handler http.Handler, sessionCookie *http.Cookie, accountID int64) []int64 {
	t.Helper()
	register := accountRegisterForSession(t, handler, sessionCookie, accountID, "")
	ids := make([]int64, 0, len(register.Entries))
	for _, entry := range register.Entries {
		ids = append(ids, entry.TransactionID)
	}
	slices.Reverse(ids)
	return ids
}

// journalOrder lists the journal's transactions oldest first.
func journalOrder(t *testing.T, handler http.Handler, sessionCookie *http.Cookie) []int64 {
	t.Helper()
	list := listTransactionsForSession(t, handler, sessionCookie, "")
	ids := make([]int64, 0, len(list.Transactions))
	for _, transaction := range list.Transactions {
		ids = append(ids, transaction.ID)
	}
	slices.Reverse(ids)
	return ids
}

func transferBody(date string, fromID, toID, amount, commodityID int64) string {
	return `{"transaction_date":"` + date + `","transaction_kind":"transfer","journal_entries":[{"entry_date":"` + date +
		`","entry_kind":"transfer_leg","postings":[` + posting(fromID, -amount, 2, commodityID) + `,` +
		posting(toID, amount, 2, commodityID) + `]}]}`
}

func placeForSession(t *testing.T, handler http.Handler, sessionCookie *http.Cookie, csrfToken string, transactionID, anchorID int64,
	position string, override bool, wantStatus int) transactionResponse {
	t.Helper()
	body := `{"anchor_transaction_id":` + strconvFormatInt(anchorID) + `,"position":"` + position + `"`
	if override {
		body += `,"reconciliation_override":true`
	}
	return mutateTransaction(t, handler, sessionCookie, csrfToken, http.MethodPost,
		"/api/v1/transactions/"+strconvFormatInt(transactionID)+"/place", body+`}`, wantStatus)
}

func TestPlaceTransactionAppliesInTheJournalAndEverySharedAccount(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	sessionCookie, csrfToken, commodityID := setupAccountAPITest(t, handler)
	checking := createLedgerAccount(t, handler, sessionCookie, csrfToken, "Place Checking", "asset", "checking", commodityID, 2)
	savings := createLedgerAccount(t, handler, sessionCookie, csrfToken, "Place Savings", "asset", "savings", commodityID, 2)
	expense := createCategoryForSession(t, handler, sessionCookie, csrfToken, `{"name":"Place Expense","category_type":"expense"}`)

	groceries := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-07",
		posting(checking.ID, -1000, 2, commodityID), posting(expense.ID, 1000, 2, commodityID)), http.StatusCreated)
	deposit := createTransactionForSession(t, handler, sessionCookie, csrfToken,
		transferBody("2026-06-07", savings.ID, checking.ID, 5000, commodityID), http.StatusCreated)
	require.Equal(t, []int64{groceries.ID, deposit.ID}, journalOrder(t, handler, sessionCookie))

	// The deposit really came before the groceries.
	placeForSession(t, handler, sessionCookie, csrfToken, deposit.ID, groceries.ID, "before", false, http.StatusOK)
	assert.Equal(t, []int64{deposit.ID, groceries.ID}, journalOrder(t, handler, sessionCookie), "the journal order")
	assert.Equal(t, []int64{deposit.ID, groceries.ID}, registerOrder(t, handler, sessionCookie, checking.ID), "the shared account")
	assert.Equal(t, []int64{groceries.ID}, registerOrder(t, handler, sessionCookie, expense.ID), "a category holding one of them is unchanged")
	assert.Equal(t, []int64{deposit.ID}, registerOrder(t, handler, sessionCookie, savings.ID))

	// Saying it again changes nothing.
	again := placeForSession(t, handler, sessionCookie, csrfToken, deposit.ID, groceries.ID, "before", false, http.StatusOK)
	assert.Equal(t, deposit.ID, again.ID)
	assert.Equal(t, []int64{deposit.ID, groceries.ID}, journalOrder(t, handler, sessionCookie))
}

// Two same-day transfers between the same accounts: the savings statement lists
// them the other way round, which is reordered there only. A later fact about
// a third transaction does not disturb that account's order.
func TestPlaceTransactionKeepsAnAccountOnlyOrderItDoesNotContradict(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	sessionCookie, csrfToken, commodityID := setupAccountAPITest(t, handler)
	checking := createLedgerAccount(t, handler, sessionCookie, csrfToken, "Local Checking", "asset", "checking", commodityID, 2)
	savings := createLedgerAccount(t, handler, sessionCookie, csrfToken, "Local Savings", "asset", "savings", commodityID, 2)
	expense := createCategoryForSession(t, handler, sessionCookie, csrfToken, `{"name":"Local Expense","category_type":"expense"}`)

	first := createTransactionForSession(t, handler, sessionCookie, csrfToken,
		transferBody("2026-06-07", checking.ID, savings.ID, 1000, commodityID), http.StatusCreated)
	second := createTransactionForSession(t, handler, sessionCookie, csrfToken,
		transferBody("2026-06-07", checking.ID, savings.ID, 2000, commodityID), http.StatusCreated)
	coffee := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-07",
		posting(checking.ID, -300, 2, commodityID), posting(expense.ID, 300, 2, commodityID)), http.StatusCreated)

	savingsLine := second.JournalEntries[0].Postings[1]
	require.Equal(t, savings.ID, savingsLine.AccountID)
	movePostingForSession(t, handler, sessionCookie, csrfToken, savings.ID, savingsLine.PostingLineID, "earlier", http.StatusOK)
	require.Equal(t, []int64{second.ID, first.ID}, registerOrder(t, handler, sessionCookie, savings.ID), "only in savings")
	require.Equal(t, []int64{first.ID, second.ID, coffee.ID}, registerOrder(t, handler, sessionCookie, checking.ID))

	placeForSession(t, handler, sessionCookie, csrfToken, coffee.ID, first.ID, "before", false, http.StatusOK)
	assert.Equal(t, []int64{coffee.ID, first.ID, second.ID}, registerOrder(t, handler, sessionCookie, checking.ID))
	assert.Equal(t, []int64{coffee.ID, first.ID, second.ID}, journalOrder(t, handler, sessionCookie))
	assert.Equal(t, []int64{second.ID, first.ID}, registerOrder(t, handler, sessionCookie, savings.ID), "the savings-only order stays")

	// A fact that contradicts it does change it, in every shared account.
	placeForSession(t, handler, sessionCookie, csrfToken, first.ID, second.ID, "before", false, http.StatusOK)
	assert.Equal(t, []int64{first.ID, second.ID}, registerOrder(t, handler, sessionCookie, savings.ID))
}

func TestPlaceTransactionAcrossAReconciledBoundaryNeedsTheOverride(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	sessionCookie, csrfToken, commodityID := setupAccountAPITest(t, handler)
	checking := createLedgerAccount(t, handler, sessionCookie, csrfToken, "Place Boundary Checking", "asset", "checking", commodityID, 2)
	expense := createCategoryForSession(t, handler, sessionCookie, csrfToken, `{"name":"Place Boundary Expense","category_type":"expense"}`)

	reconciled := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-07",
		posting(checking.ID, -10000, 2, commodityID), posting(expense.ID, 10000, 2, commodityID)), http.StatusCreated)
	later := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-07",
		posting(checking.ID, -5000, 2, commodityID), posting(expense.ID, 5000, 2, commodityID)), http.StatusCreated)
	reconcilePostingForSession(t, handler, sessionCookie, csrfToken, checking.ID, commodityID, reconciled.JournalEntries[0].Postings[0], "2026-06-07")

	placeForSession(t, handler, sessionCookie, csrfToken, later.ID, reconciled.ID, "before", false, http.StatusConflict)
	assert.Equal(t, []int64{reconciled.ID, later.ID}, journalOrder(t, handler, sessionCookie), "a refusal writes nothing")

	moved := placeForSession(t, handler, sessionCookie, csrfToken, later.ID, reconciled.ID, "before", true, http.StatusOK)
	assert.NotEmpty(t, moved.InvalidatedCheckpointIDs)
	assert.Equal(t, []int64{later.ID, reconciled.ID}, registerOrder(t, handler, sessionCookie, checking.ID))
	checkpoints := listReconciliationCheckpointsForSession(t, handler, sessionCookie, checking.ID, "?commodity_id="+strconvFormatInt(commodityID))
	require.Len(t, checkpoints.Checkpoints, 1)
	assert.Equal(t, "invalidated", checkpoints.Checkpoints[0].Status)
}

func TestPlaceTransactionRefusesTransactionsOnDifferentDays(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	sessionCookie, csrfToken, commodityID := setupAccountAPITest(t, handler)
	checking := createLedgerAccount(t, handler, sessionCookie, csrfToken, "Place Days Checking", "asset", "checking", commodityID, 2)
	expense := createCategoryForSession(t, handler, sessionCookie, csrfToken, `{"name":"Place Days Expense","category_type":"expense"}`)
	monday := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-08",
		posting(checking.ID, -100, 2, commodityID), posting(expense.ID, 100, 2, commodityID)), http.StatusCreated)
	tuesday := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-09",
		posting(checking.ID, -200, 2, commodityID), posting(expense.ID, 200, 2, commodityID)), http.StatusCreated)

	placeForSession(t, handler, sessionCookie, csrfToken, tuesday.ID, monday.ID, "before", false, http.StatusBadRequest)
	placeForSession(t, handler, sessionCookie, csrfToken, tuesday.ID, tuesday.ID, "before", false, http.StatusBadRequest)
}

// Reconciling writes a new version of the reconciled transaction. It must keep
// the transaction's place in the journal: before this was found (#185), it took
// a fresh sequence and jumped to the end of its day in the main list.
func TestReconcilingKeepsTheJournalOrder(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	sessionCookie, csrfToken, commodityID := setupAccountAPITest(t, handler)
	checking := createLedgerAccount(t, handler, sessionCookie, csrfToken, "Keep Order Checking", "asset", "checking", commodityID, 2)
	expense := createCategoryForSession(t, handler, sessionCookie, csrfToken, `{"name":"Keep Order Expense","category_type":"expense"}`)
	first := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-07",
		posting(checking.ID, -100, 2, commodityID), posting(expense.ID, 100, 2, commodityID)), http.StatusCreated)
	second := createTransactionForSession(t, handler, sessionCookie, csrfToken, balancedBody("2026-06-07",
		posting(checking.ID, -200, 2, commodityID), posting(expense.ID, 200, 2, commodityID)), http.StatusCreated)

	reconcilePostingForSession(t, handler, sessionCookie, csrfToken, checking.ID, commodityID, first.JournalEntries[0].Postings[0], "2026-06-07")
	assert.Equal(t, []int64{first.ID, second.ID}, journalOrder(t, handler, sessionCookie))
	assert.Equal(t, []int64{first.ID, second.ID}, registerOrder(t, handler, sessionCookie, checking.ID))
}
