package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApproveTransactionPreservesJournalBalancesOrderAndReconciliation(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	cookie, csrf, commodityID := setupAccountAPITest(t, handler)
	checking := createLedgerAccount(t, handler, cookie, csrf, "Approval checking", "asset", "checking", commodityID, 2)
	expense := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Approval expense","category_type":"expense"}`)
	tag := createTagForSession(t, handler, cookie, csrf, `{"name":"Review","kind":"flag"}`)
	body := balancedBody("2026-06-07", posting(checking.ID, -450, 2, commodityID), posting(expense.ID, 450, 2, commodityID))
	body = strings.Replace(body, `"transaction_date"`, `"needs_review":true,"tag_ids":[`+strconvFormatInt(tag.ID)+`],"transaction_date"`, 1)
	transaction := createTransactionForSession(t, handler, cookie, csrf, body, http.StatusCreated)
	reconcilePostingForSession(t, handler, cookie, csrf, checking.ID, commodityID, transaction.JournalEntries[0].Postings[0], "2026-06-07")
	before := readTransactionForSession(t, handler, cookie, transaction.ID, http.StatusOK)
	registerBefore := accountRegisterForSession(t, handler, cookie, checking.ID, "")
	orderBefore := journalOrder(t, handler, cookie)
	approved := mutateTransaction(t, handler, cookie, csrf, http.MethodPost, "/api/v1/transactions/"+strconvFormatInt(transaction.ID)+"/approve", `{}`, http.StatusOK)
	require.False(t, approved.NeedsReview)
	require.Len(t, approved.JournalEntries, len(before.JournalEntries), "review approval must not remove the journal")
	for i, entry := range before.JournalEntries {
		require.Len(t, approved.JournalEntries[i].Postings, len(entry.Postings))
		for j, posting := range entry.Postings {
			actual := approved.JournalEntries[i].Postings[j]
			assert.Equal(t, posting.LineKey, actual.LineKey)
			assert.Equal(t, posting.PostingLineID, actual.PostingLineID)
			assert.Equal(t, posting.AccountID, actual.AccountID)
			assert.Equal(t, posting.CommodityID, actual.CommodityID)
			assert.Equal(t, posting.QuantityValue, actual.QuantityValue)
			assert.Equal(t, posting.QuantityScale, actual.QuantityScale)
			assert.Equal(t, posting.AccountDaySequence, actual.AccountDaySequence)
			assert.Equal(t, posting.ReconciliationStatus, actual.ReconciliationStatus)
		}
	}
	registerAfter := accountRegisterForSession(t, handler, cookie, checking.ID, "")
	require.Len(t, registerAfter.Entries, 1)
	assert.Equal(t, registerBefore.Entries[0].RunningBalance, registerAfter.Entries[0].RunningBalance)
	assert.Equal(t, orderBefore, journalOrder(t, handler, cookie))
	checkpoints := listReconciliationCheckpointsForSession(t, handler, cookie, checking.ID, "")
	require.Len(t, checkpoints.Checkpoints, 1)
	assert.Equal(t, "active", checkpoints.Checkpoints[0].Status)
	assert.Equal(t, before.Status, approved.Status)
	assert.Equal(t, before.TransactionDaySequence, approved.TransactionDaySequence)
	assert.Equal(t, before.TagIDs, approved.TagIDs)
	versions := countTransactionVersions(t, database, transaction.ID)
	again := mutateTransaction(t, handler, cookie, csrf, http.MethodPost, "/api/v1/transactions/"+strconvFormatInt(transaction.ID)+"/approve", `{}`, http.StatusOK)
	assert.Equal(t, approved.VersionID, again.VersionID)
	assert.Equal(t, versions, countTransactionVersions(t, database, transaction.ID))
	assert.Equal(t, approved.JournalEntries, again.JournalEntries)
}

func TestApproveDraftPreservesJournalWithoutPostingIt(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	cookie, csrf, commodityID := setupAccountAPITest(t, handler)
	checking := createLedgerAccount(t, handler, cookie, csrf, "Draft review checking", "asset", "checking", commodityID, 2)
	expense := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Draft review expense","category_type":"expense"}`)
	draft := generateRecurringDraft(t, handler, database, cookie, "2026-06-07", checking.ID, expense.ID, commodityID, 450)
	body := balancedBody("2026-06-07", posting(checking.ID, -450, 2, commodityID), posting(expense.ID, 450, 2, commodityID))
	body = strings.Replace(body, `"transaction_date"`, `"needs_review":true,"transaction_date"`, 1)
	draft = mutateTransaction(t, handler, cookie, csrf, http.MethodPatch, "/api/v1/transactions/"+strconvFormatInt(draft.ID), body, http.StatusOK)
	require.True(t, draft.NeedsReview)
	approved := mutateTransaction(t, handler, cookie, csrf, http.MethodPost, "/api/v1/transactions/"+strconvFormatInt(draft.ID)+"/approve", `{}`, http.StatusOK)
	assert.False(t, approved.NeedsReview)
	assert.Equal(t, "draft", approved.Status)
	require.Len(t, approved.JournalEntries, 1)
	require.Len(t, approved.JournalEntries[0].Postings, 2)
	assert.Equal(t, draft.JournalEntries[0].Postings[0].QuantityValue, approved.JournalEntries[0].Postings[0].QuantityValue)
	assert.Equal(t, draft.JournalEntries[0].Postings[1].QuantityValue, approved.JournalEntries[0].Postings[1].QuantityValue)
}
