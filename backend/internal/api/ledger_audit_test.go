package api

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The #270 ledger audit (area 3 of #260) keeps its whole-journey evidence
// here: docs/reviews/ledger-reconciliation-audit-2026-10-17.md. Every figure
// below is derived by hand from the steps and the conventions (posting dates,
// ordering, lifecycle and reconciliation rules), never read back from the app.

type ledgerAuditClient struct {
	t       *testing.T
	handler http.Handler
	cookie  *http.Cookie
	csrf    string
}

func (c ledgerAuditClient) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set(csrfTokenHeader, c.csrf)
		setSameOrigin(req)
	}
	req.AddCookie(c.cookie)
	res := httptest.NewRecorder()
	c.handler.ServeHTTP(res, req)
	return res
}

func (c ledgerAuditClient) json(method, path, body string, want int, into any) {
	c.t.Helper()
	res := c.do(method, path, body)
	require.Equalf(c.t, want, res.Code, "%s %s: %s", method, path, res.Body.String())
	if into != nil {
		require.NoError(c.t, json.NewDecoder(res.Body).Decode(into))
	}
}

// refused asserts the status and the stable envelope code of a refusal.
func (c ledgerAuditClient) refused(method, path, body string, wantStatus int, wantCode string) {
	c.t.Helper()
	res := c.do(method, path, body)
	require.Equalf(c.t, wantStatus, res.Code, "%s %s: %s", method, path, res.Body.String())
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(c.t, json.NewDecoder(res.Body).Decode(&envelope))
	assert.Equalf(c.t, wantCode, envelope.Error.Code, "%s %s", method, path)
}

// balance reads one account's direct balance in one commodity as of a date, as
// a reduced decimal so "5.00" and "5" compare equal and nothing is a float.
func (c ledgerAuditClient) balance(accountID, commodityID int64, asOf string) string {
	c.t.Helper()
	var response accountBalancesResponse
	c.json(http.MethodGet, "/api/v1/ledger/account-balances?include_system=true&as_of="+asOf, "", http.StatusOK, &response)
	for _, account := range response.Accounts {
		if account.AccountID != accountID {
			continue
		}
		for _, quantity := range account.DirectBalances {
			if quantity.CommodityID == commodityID {
				return auditDecimal(c.t, quantity.QuantityValue.String(), quantity.QuantityScale)
			}
		}
		return "0"
	}
	require.FailNowf(c.t, "account missing from balances", "account %d", accountID)
	return ""
}

func (c ledgerAuditClient) register(accountID int64) []accountRegisterEntryResponse {
	c.t.Helper()
	var response accountRegisterResponse
	c.json(http.MethodGet, "/api/v1/accounts/"+strconvFormatInt(accountID)+"/register?limit=200", "", http.StatusOK, &response)
	return response.Entries
}

func (c ledgerAuditClient) checkpoints(accountID int64) map[int64]reconciliationCheckpointResponse {
	c.t.Helper()
	byID := map[int64]reconciliationCheckpointResponse{}
	for _, checkpoint := range listCheckpointsForSession(c.t, c.handler, c.cookie, accountID) {
		byID[checkpoint.ID] = checkpoint
	}
	return byID
}

func auditDecimal(t *testing.T, value string, scale int) string {
	t.Helper()
	coefficient, ok := new(big.Int).SetString(value, 10)
	require.Truef(t, ok, "coefficient %q", value)
	return new(big.Rat).SetFrac(coefficient, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)).RatString()
}

// auditRat reduces a hand-written decimal ("2805.50") the same way.
func auditRat(t *testing.T, decimal string) string {
	t.Helper()
	value, ok := new(big.Rat).SetString(decimal)
	require.Truef(t, ok, "decimal %q", decimal)
	return value.RatString()
}

// TestLedgerAuditJourneyBalancesOnBothDateAxesThroughLifecycleAndReReconciliation
// is the #270 journey: a small EUR book whose every balance is hand-derived.
// It checks posting-date (entry) balances and transaction-date selection,
// same-day ordering, metadata-only edits under a checkpoint, a back-dated
// edit that invalidates it, a void of the replacement checkpoint, a third
// reconciliation, and a session finished from a stale start.
func TestLedgerAuditJourneyBalancesOnBothDateAxesThroughLifecycleAndReReconciliation(t *testing.T) {
	t.Parallel()

	handler, _ := newSetupTestHandler(t)
	cookie, csrf, eur := setupAccountAPITest(t, handler)
	c := ledgerAuditClient{t: t, handler: handler, cookie: cookie, csrf: csrf}
	mutateAccount(t, handler, cookie, csrf, http.MethodPost, "/api/v1/setup/system-accounts", http.StatusCreated)
	checking := createLedgerAccount(t, handler, cookie, csrf, "Audit checking", "asset", "checking", eur, 2)
	savings := createLedgerAccount(t, handler, cookie, csrf, "Audit savings", "asset", "savings", eur, 2)
	card := createLedgerAccount(t, handler, cookie, csrf, "Audit card", "liability", "credit_card", eur, 2)
	groceries := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Audit groceries","category_type":"expense"}`)
	dining := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Audit dining","category_type":"expense"}`)
	salary := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Audit salary","category_type":"income"}`)
	opening := systemAccountID(t, handler, cookie, "opening_balance")
	clearing := systemAccountID(t, handler, cookie, "transfer_clearing")
	eq := func(want, got string, msg string) {
		t.Helper()
		assert.Equal(t, auditRat(t, want), got, msg)
	}

	// T1–T5: opening balance, salary, expense, refund, three-way card split.
	createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-01",
		posting(checking.ID, 100000, 2, eur), posting(opening, -100000, 2, eur)), http.StatusCreated)
	createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-03",
		posting(checking.ID, 250000, 2, eur), posting(salary.ID, -250000, 2, eur)), http.StatusCreated)
	groceryRun := createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-04",
		posting(checking.ID, -8240, 2, eur), posting(groceries.ID, 8240, 2, eur)), http.StatusCreated)
	createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-05",
		posting(checking.ID, 1240, 2, eur), posting(groceries.ID, -1240, 2, eur)), http.StatusCreated)
	createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-06",
		posting(card.ID, -12000, 2, eur), posting(groceries.ID, 4500, 2, eur), posting(dining.ID, 5000, 2, eur),
		posting(groceries.ID, 2500, 2, eur)), http.StatusCreated)

	// 1000 + 2500 - 82.40 + 12.40 = 3430.00; the card owes 120.00.
	eq("3430.00", c.balance(checking.ID, eur, "2026-05-06"), "checking after T1–T4")
	eq("-120.00", c.balance(card.ID, eur, "2026-05-06"), "card after the split")
	eq("140.00", c.balance(groceries.ID, eur, "2026-05-06"), "groceries: 82.40 - 12.40 + 45 + 25")
	eq("50.00", c.balance(dining.ID, eur, "2026-05-06"), "dining: split line")

	// T6: 500.00 leaves checking on the 7th and reaches savings on the 9th.
	transfer := createTransactionForSession(t, handler, cookie, csrf, `{
		"transaction_date":"2026-05-07","description":"To savings",
		"journal_entries":[{"entry_date":"2026-05-07","postings":[
			{"account_id":`+strconvFormatInt(checking.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"-50000","quantity_scale":2,"posted_on":"2026-05-07"},
			{"account_id":`+strconvFormatInt(savings.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"50000","quantity_scale":2,"posted_on":"2026-05-09"}]}]}`, http.StatusCreated)

	// Posting-date axis: the money is in transit on the 7th and 8th.
	for _, day := range []string{"2026-05-07", "2026-05-08"} {
		eq("2930.00", c.balance(checking.ID, eur, day), "checking once T6 leaves, "+day)
		eq("0", c.balance(savings.ID, eur, day), "savings before T6 arrives, "+day)
		eq("500.00", c.balance(clearing, eur, day), "T6 in transit, "+day)
	}
	eq("500.00", c.balance(savings.ID, eur, "2026-05-09"), "T6 arrives")
	eq("0", c.balance(clearing, eur, "2026-05-09"), "nothing in transit")

	// Transaction-date axis: a range on the 7th by transaction date holds the
	// whole transfer, savings' leg included, although that leg posts on the
	// 9th. (The entry-date list does not yet pin account_id to the matching
	// entry: #298.)
	var byTransactionDate transactionsResponse
	c.json(http.MethodGet, "/api/v1/transactions?after_date=2026-05-07&before_date=2026-05-07&account_id="+strconvFormatInt(savings.ID), "", http.StatusOK, &byTransactionDate)
	require.Len(t, byTransactionDate.Transactions, 1, "savings' leg belongs to the 7th by transaction date")
	assert.Equal(t, transfer.ID, byTransactionDate.Transactions[0].ID)
	require.Len(t, transfer.JournalEntries, 2, "one dated entry per posting day")
	assert.Equal(t, []string{"2026-05-07", "2026-05-09"}, []string{transfer.JournalEntries[0].EntryDate, transfer.JournalEntries[1].EntryDate})

	// T7 and T8 on the 10th: coffee 4.50, then the card payment 120.00.
	coffee := createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-10",
		posting(checking.ID, -450, 2, eur), posting(dining.ID, 450, 2, eur)), http.StatusCreated)
	payment := createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-10",
		posting(checking.ID, -12000, 2, eur), posting(card.ID, 12000, 2, eur)), http.StatusCreated)
	eq("2805.50", c.balance(checking.ID, eur, "2026-05-10"), "2930 - 4.50 - 120")
	eq("0", c.balance(card.ID, eur, "2026-05-10"), "-120 + 120")

	// Place the payment before the coffee: the order changes everywhere both
	// appear, the day-end balance does not, and the running balance follows
	// the new order: 2930 - 120 = 2810.00, then - 4.50 = 2805.50.
	c.json(http.MethodPost, "/api/v1/transactions/"+strconvFormatInt(payment.ID)+"/place",
		`{"anchor_transaction_id":`+strconvFormatInt(coffee.ID)+`,"position":"before"}`, http.StatusOK, nil)
	tenth := []accountRegisterEntryResponse{}
	for _, entry := range c.register(checking.ID) {
		if entry.EntryDate == "2026-05-10" {
			tenth = append(tenth, entry)
		}
	}
	require.Len(t, tenth, 2)
	// The register lists newest first, so the later row comes first.
	assert.Equal(t, []int64{coffee.ID, payment.ID}, []int64{tenth[0].TransactionID, tenth[1].TransactionID})
	eq("2805.50", auditDecimal(t, tenth[0].RunningBalance.QuantityValue.String(), tenth[0].RunningBalance.QuantityScale), "after the coffee")
	eq("2810.00", auditDecimal(t, tenth[1].RunningBalance.QuantityValue.String(), tenth[1].RunningBalance.QuantityScale), "after the payment")
	eq("2805.50", c.balance(checking.ID, eur, "2026-05-10"), "reorder never changes the day-end balance")

	// Reconcile checking through the 10th at 2805.50 (first checkpoint).
	reconcileAll := func(statementDate, balance string) (reconciliationSessionResponse, int64) {
		t.Helper()
		var started reconciliationSessionResponse
		c.json(http.MethodPost, "/api/v1/accounts/"+strconvFormatInt(checking.ID)+"/reconciliations/start", `{
			"commodity_id":`+strconvFormatInt(eur)+`,"source_kind":"statement","statement_date":"`+statementDate+`",
			"statement_balance_value":"`+balance+`","statement_balance_scale":2}`, http.StatusCreated, &started)
		ids := make([]string, 0, len(started.Candidates))
		for _, candidate := range started.Candidates {
			ids = append(ids, strconvFormatInt(candidate.PostingID))
		}
		var previewed reconciliationSessionResponse
		c.json(http.MethodPost, "/api/v1/reconciliations/"+strconvFormatInt(started.ID)+"/preview",
			`{"posting_version_ids":[`+strings.Join(ids, ",")+`]}`, http.StatusOK, &previewed)
		eq("0", auditDecimal(t, previewed.Difference.QuantityValue.String(), previewed.Difference.QuantityScale), "statement "+statementDate+" matches every candidate")
		finishReconciliationForSession(t, handler, cookie, csrf, started.ID, `{"change_reason":"matched statement"}`, http.StatusOK)
		for _, checkpoint := range listCheckpointsForSession(t, handler, cookie, checking.ID) {
			if checkpoint.SessionID != nil && *checkpoint.SessionID == started.ID {
				return started, checkpoint.ID
			}
		}
		require.FailNow(t, "checkpoint not found")
		return started, 0
	}
	first, firstCheckpoint := reconcileAll("2026-05-10", "280550")
	assert.Nil(t, first.StartingCheckpointID, "the first statement starts from zero")
	assert.Len(t, first.Candidates, 7, "T1–T4, T6's checking leg, T7, T8")

	// A metadata-only edit of a reconciled transaction needs no override and
	// leaves the balance and the checkpoint alone.
	edited := mutateTransaction(t, handler, cookie, csrf, http.MethodPatch, "/api/v1/transactions/"+strconvFormatInt(groceryRun.ID), `{
		"transaction_date":"2026-05-04","description":"Weekly groceries",
		"journal_entries":[{"entry_date":"2026-05-04","postings":[
			{"line_key":"`+groceryRun.JournalEntries[0].Postings[0].LineKey+`","account_id":`+strconvFormatInt(checking.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"-8240","quantity_scale":2},
			{"line_key":"`+groceryRun.JournalEntries[0].Postings[1].LineKey+`","account_id":`+strconvFormatInt(dining.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"8240","quantity_scale":2}]}]}`, http.StatusOK)
	assert.Empty(t, edited.InvalidatedCheckpointIDs)
	assert.Equal(t, "active", c.checkpoints(checking.ID)[firstCheckpoint].Status)
	assert.Equal(t, "reconciled", postingByAccount(t, edited, checking.ID).ReconciliationStatus, "a metadata-only edit keeps the verified status (#297)")
	eq("2805.50", c.balance(checking.ID, eur, "2026-05-10"), "a category move changes no account balance")
	eq("57.60", c.balance(groceries.ID, eur, "2026-05-10"), "140 - 82.40 moved to dining")

	// A back-dated amount edit inside the period is refused, previewed with
	// the checkpoint, and with the override invalidates exactly that set.
	amountEdit := `{"expected_version_id":` + strconvFormatInt(edited.VersionID) + `,
		"transaction_date":"2026-05-04","description":"Weekly groceries","change_reason":"receipt was 92.40",
		"journal_entries":[{"entry_date":"2026-05-04","postings":[
			{"line_key":"` + groceryRun.JournalEntries[0].Postings[0].LineKey + `","account_id":` + strconvFormatInt(checking.ID) + `,"commodity_id":` + strconvFormatInt(eur) + `,"quantity_value":"-9240","quantity_scale":2},
			{"line_key":"` + groceryRun.JournalEntries[0].Postings[1].LineKey + `","account_id":` + strconvFormatInt(dining.ID) + `,"commodity_id":` + strconvFormatInt(eur) + `,"quantity_value":"9240","quantity_scale":2}]}]`
	c.refused(http.MethodPatch, "/api/v1/transactions/"+strconvFormatInt(groceryRun.ID), amountEdit+`}`, http.StatusConflict, "CONFLICT")
	impact := reconciliationImpactForUpdate(t, handler, cookie, groceryRun.ID, amountEdit+`}`)
	require.Len(t, impact.AffectedCheckpoints, 1)
	assert.Equal(t, firstCheckpoint, impact.AffectedCheckpoints[0].CheckpointID)
	eq("2805.50", c.balance(checking.ID, eur, "2026-05-10"), "refusal and preview write nothing")
	overridden := mutateTransaction(t, handler, cookie, csrf, http.MethodPatch, "/api/v1/transactions/"+strconvFormatInt(groceryRun.ID),
		amountEdit+`,"reconciliation_override":true,"reconciliation_override_checkpoint_ids":[`+strconvFormatInt(firstCheckpoint)+`]}`, http.StatusOK)
	assert.Equal(t, []int64{firstCheckpoint}, overridden.InvalidatedCheckpointIDs, "preview equals write")
	// No statement verified 92.40, so the edited line is uncleared again; its
	// counter-posting changed amount too (#297).
	for _, entry := range overridden.JournalEntries {
		for _, edited := range entry.Postings {
			assert.Equalf(t, "uncleared", edited.ReconciliationStatus, "account %d", edited.AccountID)
			assert.Emptyf(t, edited.ClearedOn, "account %d", edited.AccountID)
		}
	}
	eq("2795.50", c.balance(checking.ID, eur, "2026-05-10"), "2805.50 - 10.00")
	eq("3407.60", c.balance(checking.ID, eur, "2026-05-04"), "1000 + 2500 - 92.40 on the edited day")

	// Reconcile again from zero: the first checkpoint no longer covers
	// anything, every posting is offered, and the once-reconciled ones start
	// selected for re-verification (#240). The amount-edited posting is
	// uncleared, so it is offered unselected, like any new posting (#297).
	second, secondCheckpoint := reconcileAll("2026-05-10", "279550")
	assert.Nil(t, second.StartingCheckpointID, "the invalidated checkpoint is no starting balance")
	assert.Len(t, second.Candidates, 7)
	editedPosting := postingByAccount(t, overridden, checking.ID).ID
	require.Len(t, second.SelectedPostings, 6, "every unchanged once-reconciled posting starts selected")
	for _, selected := range second.SelectedPostings {
		assert.Equal(t, "reconciled", selected.ReconciliationStatus, "only once-reconciled postings start selected")
		assert.NotEqual(t, editedPosting, selected.PostingID, "the amount-edited posting is not preselected")
	}

	// Void the second checkpoint (a wrong statement), then reconcile a third
	// time; the history keeps every checkpoint with its reason.
	c.json(http.MethodPost, "/api/v1/reconciliation-checkpoints/"+strconvFormatInt(secondCheckpoint)+"/void",
		`{"change_reason":"statement was a draft"}`, http.StatusOK, nil)
	third, thirdCheckpoint := reconcileAll("2026-05-10", "279550")
	assert.Nil(t, third.StartingCheckpointID, "voiding the only active checkpoint leaves no starting balance")
	history := c.checkpoints(checking.ID)
	require.Len(t, history, 3)
	assert.Equal(t, "invalidated", history[firstCheckpoint].Status)
	assert.Equal(t, "receipt was 92.40", history[firstCheckpoint].InvalidationReason)
	assert.Equal(t, "voided", history[secondCheckpoint].Status)
	assert.Equal(t, "statement was a draft", history[secondCheckpoint].VoidReason)
	assert.Equal(t, "active", history[thirdCheckpoint].Status)
	eq("2795.50", auditDecimal(t, history[thirdCheckpoint].StatementBalance.QuantityValue.String(), history[thirdCheckpoint].StatementBalance.QuantityScale), "third statement")

	// A stale start is refused at commit, not only at preview: two sessions
	// open from the third checkpoint, one finishes, and the other — already
	// previewed at a zero difference — is refused when it finishes.
	small := createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-11",
		posting(checking.ID, -500, 2, eur), posting(dining.ID, 500, 2, eur)), http.StatusCreated)
	smallPosting := strconvFormatInt(postingByAccount(t, small, checking.ID).ID)
	startFromThird := func() reconciliationSessionResponse {
		var started reconciliationSessionResponse
		c.json(http.MethodPost, "/api/v1/accounts/"+strconvFormatInt(checking.ID)+"/reconciliations/start", `{
			"commodity_id":`+strconvFormatInt(eur)+`,"source_kind":"statement","statement_date":"2026-05-11",
			"statement_balance_value":"279050","statement_balance_scale":2}`, http.StatusCreated, &started)
		require.NotNil(t, started.StartingCheckpointID)
		assert.Equal(t, thirdCheckpoint, *started.StartingCheckpointID)
		eq("2795.50", auditDecimal(t, started.StartingBalance.QuantityValue.String(), started.StartingBalance.QuantityScale), "starts from the third statement")
		var previewed reconciliationSessionResponse
		c.json(http.MethodPost, "/api/v1/reconciliations/"+strconvFormatInt(started.ID)+"/preview",
			`{"posting_version_ids":[`+smallPosting+`]}`, http.StatusOK, &previewed)
		eq("0", auditDecimal(t, previewed.Difference.QuantityValue.String(), previewed.Difference.QuantityScale), "2795.50 - 5.00 = 2790.50")
		return started
	}
	stale, winner := startFromThird(), startFromThird()
	finishReconciliationForSession(t, handler, cookie, csrf, winner.ID, `{"change_reason":"matched statement"}`, http.StatusOK)
	c.refused(http.MethodPost, "/api/v1/reconciliations/"+strconvFormatInt(stale.ID)+"/finish",
		`{"change_reason":"matched statement"}`, http.StatusConflict, "RECONCILIATION_START_STALE")
	assert.Len(t, c.checkpoints(checking.ID), 4, "the refused finish wrote no checkpoint")
	eq("2790.50", c.balance(checking.ID, eur, "2026-05-11"), "after the 5.00 spend")

	// Lifecycle on a system draft: excluded while a draft, then posted,
	// edited, voided, unvoided, soft-deleted and restored. Every step moves
	// today's balance by exactly its own amount and nothing is hard-deleted.
	today := readTransactionForSession(t, handler, cookie, small.ID, http.StatusOK).CreatedAt[:10]
	var template struct {
		ID int64 `json:"id"`
	}
	c.json(http.MethodPost, "/api/v1/recurring/templates", `{
		"name":"Audit lunch","frequency":"daily","starts_on":"`+today+`","lead_days":0,"description":"Lunch plan",
		"postings":[
			{"account_id":`+strconvFormatInt(checking.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"-1500","quantity_scale":2},
			{"account_id":`+strconvFormatInt(dining.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"1500","quantity_scale":2}]}`, http.StatusCreated, &template)
	c.json(http.MethodPost, "/api/v1/recurring/templates/"+strconvFormatInt(template.ID)+"/run-now", `{}`, http.StatusOK, nil)
	var drafts transactionsResponse
	c.json(http.MethodGet, "/api/v1/transactions?status=draft", "", http.StatusOK, &drafts)
	require.Len(t, drafts.Transactions, 1)
	draft := drafts.Transactions[0]
	lifecycle := func(action string) transactionResponse {
		return mutateTransaction(t, handler, cookie, csrf, http.MethodPost, "/api/v1/transactions/"+strconvFormatInt(draft.ID)+"/"+action,
			`{"change_reason":"audit `+action+`"}`, http.StatusOK)
	}
	eq("2790.50", c.balance(checking.ID, eur, today), "a draft is outside the ledger")
	c.refused(http.MethodPost, "/api/v1/transactions/"+strconvFormatInt(draft.ID)+"/void", `{"change_reason":"no"}`, http.StatusConflict, "CONFLICT")
	assert.Equal(t, "posted", lifecycle("post").Status)
	eq("2775.50", c.balance(checking.ID, eur, today), "posting the draft: - 15.00")
	c.refused(http.MethodDelete, "/api/v1/transactions/"+strconvFormatInt(draft.ID), "", http.StatusConflict, "CONFLICT")
	posted := readTransactionForSession(t, handler, cookie, draft.ID, http.StatusOK)
	lineKeys := []string{posted.JournalEntries[0].Postings[0].LineKey, posted.JournalEntries[0].Postings[1].LineKey}
	accounts := []int64{posted.JournalEntries[0].Postings[0].AccountID, posted.JournalEntries[0].Postings[1].AccountID}
	amount := func(accountID int64) string {
		if accountID == checking.ID {
			return "-1800"
		}
		return "1800"
	}
	mutateTransaction(t, handler, cookie, csrf, http.MethodPatch, "/api/v1/transactions/"+strconvFormatInt(draft.ID), `{
		"transaction_date":"`+today+`","description":"Lunch plan",
		"journal_entries":[{"entry_date":"`+today+`","postings":[
			{"line_key":"`+lineKeys[0]+`","account_id":`+strconvFormatInt(accounts[0])+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"`+amount(accounts[0])+`","quantity_scale":2},
			{"line_key":"`+lineKeys[1]+`","account_id":`+strconvFormatInt(accounts[1])+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"`+amount(accounts[1])+`","quantity_scale":2}]}]}`, http.StatusOK)
	eq("2772.50", c.balance(checking.ID, eur, today), "edited to 18.00")
	assert.Equal(t, "voided", lifecycle("void").Status)
	eq("2790.50", c.balance(checking.ID, eur, today), "a voided entry leaves the ledger")
	assert.Equal(t, "voided", readTransactionForSession(t, handler, cookie, draft.ID, http.StatusOK).Status, "and stays visible")
	assert.Equal(t, "posted", lifecycle("unvoid").Status)
	eq("2772.50", c.balance(checking.ID, eur, today), "unvoid restores the edited 18.00")
	lifecycle("soft-delete")
	eq("2790.50", c.balance(checking.ID, eur, today), "a soft-deleted entry leaves the ledger")
	readTransactionForSession(t, handler, cookie, draft.ID, http.StatusNotFound)
	var trash deletedTransactionsResponse
	c.json(http.MethodGet, "/api/v1/transactions/deleted", "", http.StatusOK, &trash)
	require.Len(t, trash.Transactions, 1, "the trash holds it")
	restored := lifecycle("restore")
	assert.Empty(t, restored.DeletedAt)
	eq("2772.50", c.balance(checking.ID, eur, today), "restored")
	assert.Equal(t, int64(5), restored.VersionSeq, "draft, post, edit, void, unvoid; the delete flag adds no version")
	assert.Len(t, c.checkpoints(checking.ID), 4, "nothing after the 11th touched a checkpoint")
}
