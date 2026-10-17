package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #296: a generic ledger write is bound to what the owner reviewed — the
// transaction version the form was composed against, and the checkpoint set
// the reconciliation-impact preview named — not only to the service's own
// read inside the request (T-94).

// reviewBindingBook is one EUR checking account with a 10.00 spend on the 4th.
type reviewBindingBook struct {
	c        ledgerAuditClient
	checking int64
	expense  int64
	eur      int64
	spend    transactionResponse
}

func newReviewBindingBook(t *testing.T) reviewBindingBook {
	t.Helper()
	handler, _ := newSetupTestHandler(t)
	cookie, csrf, eur := setupAccountAPITest(t, handler)
	c := ledgerAuditClient{t: t, handler: handler, cookie: cookie, csrf: csrf}
	mutateAccount(t, handler, cookie, csrf, http.MethodPost, "/api/v1/setup/system-accounts", http.StatusCreated)
	checking := createLedgerAccount(t, handler, cookie, csrf, "Binding checking", "asset", "checking", eur, 2)
	expense := createCategoryForSession(t, handler, cookie, csrf, `{"name":"Binding groceries","category_type":"expense"}`)
	opening := systemAccountID(t, handler, cookie, "opening_balance")
	createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-01",
		posting(checking.ID, 100000, 2, eur), posting(opening, -100000, 2, eur)), http.StatusCreated)
	spend := createTransactionForSession(t, handler, cookie, csrf, balancedBody("2026-05-04",
		posting(checking.ID, -1000, 2, eur), posting(expense.ID, 1000, 2, eur)), http.StatusCreated)
	return reviewBindingBook{c: c, checking: checking.ID, expense: expense.ID, eur: eur, spend: spend}
}

// spendBody is the browser's full-spec PATCH body for the spend at an amount
// (minor units), as a form composed against the given version would send it.
func (b reviewBindingBook) spendBody(amount, description string, extra string) string {
	lines := b.spend.JournalEntries[0].Postings
	accountLine, expenseLine := lines[0], lines[1]
	if accountLine.AccountID != b.checking {
		accountLine, expenseLine = expenseLine, accountLine
	}
	body := `{"transaction_date":"2026-05-04","description":"` + description + `","change_reason":"receipt",
		"journal_entries":[{"entry_date":"2026-05-04","postings":[
			{"line_key":"` + accountLine.LineKey + `","account_id":` + strconvFormatInt(b.checking) + `,"commodity_id":` + strconvFormatInt(b.eur) + `,"quantity_value":"-` + amount + `","quantity_scale":2},
			{"line_key":"` + expenseLine.LineKey + `","account_id":` + strconvFormatInt(b.expense) + `,"commodity_id":` + strconvFormatInt(b.eur) + `,"quantity_value":"` + amount + `","quantity_scale":2}]}]`
	if extra != "" {
		body += "," + extra
	}
	return body + "}"
}

// reconcile finishes a statement for checking at the given balance with every
// candidate selected and returns the new checkpoint id.
func (b reviewBindingBook) reconcile(statementDate, balance string) int64 {
	c := b.c
	c.t.Helper()
	var started reconciliationSessionResponse
	c.json(http.MethodPost, "/api/v1/accounts/"+strconvFormatInt(b.checking)+"/reconciliations/start", `{
		"commodity_id":`+strconvFormatInt(b.eur)+`,"source_kind":"statement","statement_date":"`+statementDate+`",
		"statement_balance_value":"`+balance+`","statement_balance_scale":2}`, http.StatusCreated, &started)
	ids := make([]string, 0, len(started.Candidates))
	for _, candidate := range started.Candidates {
		ids = append(ids, strconvFormatInt(candidate.PostingID))
	}
	c.json(http.MethodPost, "/api/v1/reconciliations/"+strconvFormatInt(started.ID)+"/preview",
		`{"posting_version_ids":[`+strings.Join(ids, ",")+`]}`, http.StatusOK, nil)
	finishReconciliationForSession(c.t, c.handler, c.cookie, c.csrf, started.ID, `{"change_reason":"matched statement"}`, http.StatusOK)
	for _, checkpoint := range listCheckpointsForSession(c.t, c.handler, c.cookie, b.checking) {
		if checkpoint.SessionID != nil && *checkpoint.SessionID == started.ID {
			return checkpoint.ID
		}
	}
	require.FailNow(c.t, "checkpoint not found")
	return 0
}

func impactIDs(impact reconciliationImpactResponse) []int64 {
	ids := make([]int64, 0, len(impact.AffectedCheckpoints))
	for _, checkpoint := range impact.AffectedCheckpoints {
		ids = append(ids, checkpoint.CheckpointID)
	}
	return ids
}

func idList(ids ...int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconvFormatInt(id))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func TestStaleBrowserSaveAfterNewerAmountEditIsRefusedAndKeepsTheNewerAmount(t *testing.T) {
	t.Parallel()
	b := newReviewBindingBook(t)
	c := b.c
	path := "/api/v1/transactions/" + strconvFormatInt(b.spend.ID)
	tabA := b.spend.VersionID // both tabs open the 10.00 spend

	// Tab B changes the amount to 20.00.
	var newer transactionResponse
	c.json(http.MethodPatch, path, b.spendBody("2000", "Groceries", `"expected_version_id":`+strconvFormatInt(tabA)), http.StatusOK, &newer)
	assert.Equal(t, auditRat(t, "980.00"), c.balance(b.checking, b.eur, "2026-05-04"), "1000 - 20.00")

	// Tab A saves a description-only change from its stale form, which still
	// carries 10.00. It must not revert tab B's amount.
	c.refused(http.MethodPatch, path, b.spendBody("1000", "Weekly groceries", `"expected_version_id":`+strconvFormatInt(tabA)),
		http.StatusConflict, "TRANSACTION_VERSION_STALE")
	assert.Equal(t, auditRat(t, "980.00"), c.balance(b.checking, b.eur, "2026-05-04"), "the newer amount is kept")
	current := readTransactionForSession(t, c.handler, c.cookie, b.spend.ID, http.StatusOK)
	assert.Equal(t, newer.VersionID, current.VersionID, "the refusal wrote no version")
	assert.Equal(t, "Groceries", current.Description)

	// A browser edit must name the version it was composed against.
	c.refused(http.MethodPatch, path, b.spendBody("2000", "Weekly groceries", ""), http.StatusBadRequest, "VALIDATION_FAILED")

	// Reloaded, tab A saves its description against the newer version.
	var saved transactionResponse
	c.json(http.MethodPatch, path, b.spendBody("2000", "Weekly groceries", `"expected_version_id":`+strconvFormatInt(current.VersionID)), http.StatusOK, &saved)
	assert.Equal(t, "Weekly groceries", saved.Description)
	assert.Equal(t, auditRat(t, "980.00"), c.balance(b.checking, b.eur, "2026-05-04"))

	// The lifecycle writes take the same optional binding.
	c.refused(http.MethodPost, path+"/void", `{"change_reason":"duplicate","expected_version_id":`+strconvFormatInt(current.VersionID)+`}`,
		http.StatusConflict, "TRANSACTION_VERSION_STALE")
	assert.Equal(t, "posted", readTransactionForSession(t, c.handler, c.cookie, b.spend.ID, http.StatusOK).Status)
	c.json(http.MethodPost, path+"/void", `{"change_reason":"duplicate","expected_version_id":`+strconvFormatInt(saved.VersionID)+`}`, http.StatusOK, nil)
}

func TestOverrideReviewedAgainstOneCheckpointIsRefusedOnceTheSetGrows(t *testing.T) {
	t.Parallel()
	b := newReviewBindingBook(t)
	c := b.c
	path := "/api/v1/transactions/" + strconvFormatInt(b.spend.ID)

	first := b.reconcile("2026-05-10", "99000") // 1000 - 10.00
	// The owner opens the spend after the statement marked it reconciled.
	opened := readTransactionForSession(t, c.handler, c.cookie, b.spend.ID, http.StatusOK)
	edit := b.spendBody("1500", "Groceries", `"expected_version_id":`+strconvFormatInt(opened.VersionID))
	impact := reconciliationImpactForUpdate(t, c.handler, c.cookie, b.spend.ID, edit)
	require.Equal(t, []int64{first}, impactIDs(impact), "the owner reviews one checkpoint")

	// Another tab finishes a later statement in between.
	createTransactionForSession(t, c.handler, c.cookie, c.csrf, balancedBody("2026-05-12",
		posting(b.checking, -500, 2, b.eur), posting(b.expense, 500, 2, b.eur)), http.StatusCreated)
	second := b.reconcile("2026-05-12", "98500")

	overridden := strings.TrimSuffix(edit, "}") + `,"reconciliation_override":true,"reconciliation_override_checkpoint_ids":` + idList(first) + `}`
	c.refused(http.MethodPatch, path, overridden, http.StatusConflict, "RECONCILIATION_IMPACT_STALE")
	checkpoints := c.checkpoints(b.checking)
	assert.Equal(t, "active", checkpoints[first].Status, "nothing is invalidated")
	assert.Equal(t, "active", checkpoints[second].Status, "the unseen statement survives")
	assert.Equal(t, auditRat(t, "985.00"), c.balance(b.checking, b.eur, "2026-05-12"), "the edit was not applied")

	// An override must name the set it acknowledges.
	bare := strings.TrimSuffix(edit, "}") + `,"reconciliation_override":true}`
	c.refused(http.MethodPatch, path, bare, http.StatusBadRequest, "VALIDATION_FAILED")

	// Reviewed again, the override invalidates exactly the acknowledged set.
	impact = reconciliationImpactForUpdate(t, c.handler, c.cookie, b.spend.ID, edit)
	require.Equal(t, []int64{first, second}, impactIDs(impact))
	var saved transactionResponse
	c.json(http.MethodPatch, path, strings.TrimSuffix(edit, "}")+`,"reconciliation_override":true,"reconciliation_override_checkpoint_ids":`+idList(second, first)+`}`,
		http.StatusOK, &saved)
	assert.ElementsMatch(t, []int64{first, second}, saved.InvalidatedCheckpointIDs)
	assert.Equal(t, auditRat(t, "980.00"), c.balance(b.checking, b.eur, "2026-05-12"), "1000 - 15.00 - 5.00")
}

func TestLifecycleOverrideIsBoundToTheReviewedCheckpointSet(t *testing.T) {
	t.Parallel()
	b := newReviewBindingBook(t)
	c := b.c
	path := "/api/v1/transactions/" + strconvFormatInt(b.spend.ID)

	first := b.reconcile("2026-05-10", "99000")
	var impact reconciliationImpactResponse
	c.json(http.MethodGet, path+"/void/reconciliation-impact", "", http.StatusOK, &impact)
	require.Equal(t, []int64{first}, impactIDs(impact))

	createTransactionForSession(t, c.handler, c.cookie, c.csrf, balancedBody("2026-05-12",
		posting(b.checking, -500, 2, b.eur), posting(b.expense, 500, 2, b.eur)), http.StatusCreated)
	second := b.reconcile("2026-05-12", "98500")

	c.refused(http.MethodPost, path+"/void", `{"change_reason":"duplicate","reconciliation_override":true,"reconciliation_override_checkpoint_ids":`+idList(first)+`}`,
		http.StatusConflict, "RECONCILIATION_IMPACT_STALE")
	assert.Equal(t, "posted", readTransactionForSession(t, c.handler, c.cookie, b.spend.ID, http.StatusOK).Status)
	assert.Equal(t, "active", c.checkpoints(b.checking)[second].Status)

	var voided transactionResponse
	c.json(http.MethodPost, path+"/void", `{"change_reason":"duplicate","reconciliation_override":true,"reconciliation_override_checkpoint_ids":`+idList(first, second)+`}`,
		http.StatusOK, &voided)
	assert.ElementsMatch(t, []int64{first, second}, voided.InvalidatedCheckpointIDs)
}

// A reorder has no preview: its refusal names the checkpoints the override
// would invalidate, and the override is bound to that named set.
func TestReorderRefusalNamesTheCheckpointsAndTheOverrideIsBoundToThem(t *testing.T) {
	t.Parallel()
	b := newReviewBindingBook(t)
	c := b.c
	later := createTransactionForSession(t, c.handler, c.cookie, c.csrf, balancedBody("2026-05-04",
		posting(b.checking, -300, 2, b.eur), posting(b.expense, 300, 2, b.eur)), http.StatusCreated)
	// Reconcile on the 4th, between the spend (seq 1) and the later one (seq 2):
	// the statement saw only the spend.
	var started reconciliationSessionResponse
	c.json(http.MethodPost, "/api/v1/accounts/"+strconvFormatInt(b.checking)+"/reconciliations/start", `{
		"commodity_id":`+strconvFormatInt(b.eur)+`,"source_kind":"statement","statement_date":"2026-05-04",
		"statement_balance_value":"99000","statement_balance_scale":2}`, http.StatusCreated, &started)
	var selected []string
	for _, candidate := range started.Candidates {
		if candidate.TransactionID != later.ID {
			selected = append(selected, strconvFormatInt(candidate.PostingID))
		}
	}
	c.json(http.MethodPost, "/api/v1/reconciliations/"+strconvFormatInt(started.ID)+"/preview",
		`{"posting_version_ids":[`+strings.Join(selected, ",")+`]}`, http.StatusOK, nil)
	finishReconciliationForSession(t, c.handler, c.cookie, c.csrf, started.ID, `{"change_reason":"matched statement"}`, http.StatusOK)
	var checkpoint int64
	for id := range c.checkpoints(b.checking) {
		checkpoint = id
	}

	place := "/api/v1/transactions/" + strconvFormatInt(later.ID) + "/place"
	res := c.do(http.MethodPost, place, `{"anchor_transaction_id":`+strconvFormatInt(b.spend.ID)+`,"position":"before"}`)
	require.Equalf(t, http.StatusConflict, res.Code, res.Body.String())
	var refusal struct {
		Error struct {
			Code          string  `json:"code"`
			CheckpointIDs []int64 `json:"checkpoint_ids"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&refusal))
	assert.Equal(t, "CONFLICT", refusal.Error.Code)
	assert.Equal(t, []int64{checkpoint}, refusal.Error.CheckpointIDs, "the refusal names what the override would invalidate")

	c.refused(http.MethodPost, place, `{"anchor_transaction_id":`+strconvFormatInt(b.spend.ID)+`,"position":"before","reconciliation_override":true,"reconciliation_override_checkpoint_ids":[]}`,
		http.StatusConflict, "RECONCILIATION_IMPACT_STALE")
	assert.Equal(t, "active", c.checkpoints(b.checking)[checkpoint].Status)
	var placed transactionResponse
	c.json(http.MethodPost, place, `{"anchor_transaction_id":`+strconvFormatInt(b.spend.ID)+`,"position":"before","reconciliation_override":true,"reconciliation_override_checkpoint_ids":`+idList(checkpoint)+`}`,
		http.StatusOK, &placed)
	assert.Equal(t, []int64{checkpoint}, placed.InvalidatedCheckpointIDs)
}
