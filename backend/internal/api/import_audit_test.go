package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The #271 import audit (area 4 of #260) keeps its evidence here:
// docs/reviews/import-audit-2026-10-17.md. Every amount and date below is
// derived by hand from the file bytes and the documented parsing rules
// (import_locale.go's contract, the import plan), never read back from the app.

// uploadImportForAudit posts one file with optional form fields and returns
// the staged preview.
func uploadImportForAudit(t *testing.T, c ledgerAuditClient, filename string, contents []byte, fields map[string]string) startImportResponse {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(contents)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/imports", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set(csrfTokenHeader, c.csrf)
	setSameOrigin(req)
	req.AddCookie(c.cookie)
	res := httptest.NewRecorder()
	c.handler.ServeHTTP(res, req)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	var started startImportResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&started))
	return started
}

// allImportRowsForAudit follows next_cursor, as the review screen does.
func allImportRowsForAudit(t *testing.T, c ledgerAuditClient, batchID int64) []importStagedRowResponse {
	t.Helper()
	var rows []importStagedRowResponse
	suffix := "?limit=100"
	for {
		page := getImportBatchForSession(t, c.handler, c.cookie, batchID, suffix, http.StatusOK)
		rows = append(rows, page.Rows...)
		if page.NextCursor == nil {
			return rows
		}
		suffix = "?limit=100&cursor=" + *page.NextCursor
	}
}

func rowIDsForAudit(rows []importStagedRowResponse) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func dedupeCountsForAudit(rows []importStagedRowResponse) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.DedupeStatus]++
	}
	return counts
}

// TestImportAuditEncodingAndDecimalMatrixCommitsHandDerivedAmounts drives each
// encoding × decimal convention × format through upload, review and the real
// ledger commit, then reads the account balance. One account per file keeps
// each expected balance a sum of that file's rows.
func TestImportAuditEncodingAndDecimalMatrixCommitsHandDerivedAmounts(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	cookie, csrf, eur, _, groceries := bootstrapImportAPITest(t, handler)
	c := ledgerAuditClient{t: t, handler: handler, cookie: cookie, csrf: csrf}

	// An API profile with neither date_layout nor decimal_separator, so the
	// file itself settles both (the UI always writes explicit values).
	var detecting importProfileResponse
	c.json(http.MethodPost, "/api/v1/import-profiles", `{"name":"Detecting","adapter_kind":"csv","config":"{\"delimiter\":\"semicolon\",\"date_column\":\"Datum\",\"payee_column\":\"Naam\",\"amount_column\":\"Bedrag\"}"}`, http.StatusCreated, &detecting)
	var declared importProfileResponse
	c.json(http.MethodPost, "/api/v1/import-profiles", `{"name":"Declared","adapter_kind":"csv","config":"{\"delimiter\":\"semicolon\",\"date_column\":\"Datum\",\"payee_column\":\"Naam\",\"amount_column\":\"Bedrag\",\"date_layout\":\"DMY\",\"decimal_separator\":\",\"}"}`, http.StatusCreated, &declared)
	detectingID := strconv.FormatInt(detecting.ID, 10)
	declaredID := strconv.FormatInt(declared.ID, 10)

	cases := []struct {
		name, file string
		body       []byte
		fields     map[string]string
		encoding   string // reported text encoding / source
		dates      []string
		payees     []string
		balance    string // hand sum of the rows
	}{
		{
			name: "CSV UTF-8 with BOM, decimal comma detected", file: "bom.csv",
			body:   append([]byte("\xef\xbb\xbf"), "Datum;Naam;Bedrag\n01/02/2026;Café;-1.234,56\n13/02/2026;Bäcker;-2,05\n"...),
			fields: map[string]string{"profile_id": detectingID}, encoding: "utf-8/bom",
			// 13/02 proves DMY for the file; "1.234,56" and "2,05" prove the comma.
			dates: []string{"2026-02-01", "2026-02-13"}, payees: []string{"Café", "Bäcker"},
			balance: "-1236.61", // -1234.56 - 2.05
		},
		{
			name: "CSV Windows-1252 detected, round thousands settled by the file", file: "cp1252.csv",
			body:   []byte("Datum;Naam;Bedrag\n01/03/2026;Caf\xe9 \x80;-10,10\n14/03/2026;B\xe4cker;-3.000\n15/03/2026;M\xfcller;-0,95\n"),
			fields: map[string]string{"profile_id": detectingID}, encoding: "windows-1252/detected",
			// "-3.000" alone is undecidable; the file's "10,10"/"0,95" make it 3000 (T-36 class).
			dates: []string{"2026-03-01", "2026-03-14", "2026-03-15"}, payees: []string{"Café €", "Bäcker", "Müller"},
			balance: "-3011.05", // -10.10 - 3000 - 0.95
		},
		{
			name: "CSV Windows-1252 selected, declared DMY and comma", file: "declared.csv",
			body:   []byte("Datum;Naam;Bedrag\n01/04/2026;Caf\xe9 \x80;-10,10\n02/04/2026;B\xe4cker;1.000\n"),
			fields: map[string]string{"profile_id": declaredID, "text_encoding": "windows-1252"}, encoding: "windows-1252/selected",
			// Every date is ambiguous; the declared DMY decides. "1.000" under a declared comma is 1000.
			dates: []string{"2026-04-01", "2026-04-02"}, payees: []string{"Café €", "Bäcker"},
			balance: "989.9", // -10.10 + 1000
		},
		{
			name: "CSV Latin-1 selected, declared DMY and comma", file: "latin1.csv",
			body:   []byte("Datum;Naam;Bedrag\n05/01/2026;Gr\xfcn;-7,5\n"),
			fields: map[string]string{"profile_id": declaredID, "text_encoding": "iso-8859-1"}, encoding: "iso-8859-1/selected",
			dates: []string{"2026-01-05"}, payees: []string{"Grün"},
			balance: "-7.5",
		},
		{
			name: "QIF Latin-1 selected, dotted dates and decimal comma detected", file: "latin1.qif",
			body:   []byte("!Type:Bank\nD15.06.2026\nT-1.234,56\nPM\xfcller\n^\nD01.06.2026\nT-1.000\nPGr\xfcn\n^\n"),
			fields: map[string]string{"text_encoding": "iso-8859-1"}, encoding: "iso-8859-1/selected",
			// 15.06 proves DMY; "1.234,56" proves the comma, so "-1.000" is -1000.
			dates: []string{"2026-06-15", "2026-06-01"}, payees: []string{"Müller", "Grün"},
			balance: "-2234.56", // -1234.56 - 1000
		},
		{
			name: "QIF UTF-8 with BOM, US dates and decimal point detected", file: "bom.qif",
			body:     append([]byte("\xef\xbb\xbf"), "!Type:Bank\nD07/15/2026\nT1,234.56\nPZoë\n^\nD07/01/2026\nT-1,000\nPØre\n^\n"...),
			encoding: "utf-8/bom",
			// 07/15 proves MDY; "1,234.56" proves the point, so "-1,000" is -1000.
			dates: []string{"2026-07-15", "2026-07-01"}, payees: []string{"Zoë", "Øre"},
			balance: "234.56", // 1234.56 - 1000
		},
	}
	for i, tc := range cases {
		account := createLedgerAccount(t, handler, cookie, csrf, fmt.Sprintf("Audit %d", i), "asset", "checking", eur, 2)
		fields := tc.fields
		if fields == nil {
			fields = map[string]string{}
		}
		started := uploadImportForAudit(t, c, tc.file, tc.body, fields)
		assert.Equal(t, tc.encoding, started.Meta.TextEncoding+"/"+started.Meta.EncodingSource, tc.name)
		require.Len(t, started.Rows, len(tc.dates), tc.name)
		for j, row := range started.Rows {
			var normalized struct {
				Date      string `json:"date"`
				PayeeHint string `json:"payee_hint"`
			}
			require.NoError(t, json.Unmarshal([]byte(row.NormalizedJSON), &normalized))
			assert.Equal(t, tc.dates[j], normalized.Date, tc.name)
			assert.Equal(t, tc.payees[j], normalized.PayeeHint, tc.name)
		}
		assert.Empty(t, started.Warnings, tc.name)
		patchImportBatchForSession(t, handler, cookie, csrf, started.Batch.ID, resolutionPatchBody(t, account.ID, eur, groceries.ID, rowIDsForAudit(started.Rows)...), http.StatusNoContent)
		commit := commitImportBatchForSession(t, handler, cookie, csrf, started.Batch.ID, "", http.StatusOK)
		require.Equal(t, len(tc.dates), commit.CommittedCount, tc.name)
		assert.Equal(t, auditRat(t, tc.balance), c.balance(account.ID, eur, "2026-12-31"), tc.name)
	}
}

// TestImportAuditLargeBatchPartialCommitAndRepeatImportAreIdempotent stages
// 250 rows (past the 200-row page and the old silent clamp), commits all but
// one through the real ledger, then shows that a repeat import of the same
// file, after owner corrections to committed rows, only offers the missing row.
func TestImportAuditLargeBatchPartialCommitAndRepeatImportAreIdempotent(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	cookie, csrf, eur, checking, groceries := bootstrapImportAPITest(t, handler)
	c := ledgerAuditClient{t: t, handler: handler, cookie: cookie, csrf: csrf}

	// Row i spends i + 0.01: the total of 250 rows is 31375 + 2.50 = 31377.50.
	var file strings.Builder
	for i := 1; i <= 250; i++ {
		file.WriteString(qifRow(fmt.Sprintf("2026-03-%02d", (i%28)+1), fmt.Sprintf("-%d.01", i), fmt.Sprintf("Payee %d", i)))
	}
	statement := []byte(file.String())

	started := uploadImportForAudit(t, c, "large.qif", statement, nil)
	rows := allImportRowsForAudit(t, c, started.Batch.ID)
	require.Len(t, rows, 250, "paging returns every staged row once")
	assert.Equal(t, map[string]int{"new": 250}, dedupeCountsForAudit(rows))

	// Leave row 8 (8.01) unresolved: it fails, the other 249 commit.
	resolved := append(append([]int64{}, rowIDsForAudit(rows[:7])...), rowIDsForAudit(rows[8:])...)
	patchImportBatchForSession(t, handler, cookie, csrf, started.Batch.ID, resolutionPatchBody(t, checking.ID, eur, groceries.ID, resolved...), http.StatusNoContent)
	partial := commitImportBatchForSession(t, handler, cookie, csrf, started.Batch.ID, "", http.StatusOK)
	assert.Equal(t, "partially_committed", partial.Status)
	assert.Equal(t, 249, partial.CommittedCount)
	assert.Equal(t, 1, partial.FailedCount)
	assert.Equal(t, auditRat(t, "-31369.49"), c.balance(checking.ID, eur, "2026-12-31"), "31377.50 - 8.01")

	committed := allImportRowsForAudit(t, c, started.Batch.ID)
	require.Equal(t, "failed", committed[7].CommitStatus)
	assert.Equal(t, "missing account resolution", committed[7].CommitError)
	for _, row := range committed {
		var raw map[string]string
		require.NoError(t, json.Unmarshal([]byte(row.RawJSON), &raw), "raw source evidence is retained")
		assert.Equal(t, "large.qif", raw["_source_file"])
	}

	// Owner corrections to committed rows: row 1 (-1.01) becomes -1.11, row 2
	// (-2.01) is voided. A repeat import must not undo or duplicate either.
	first := readTransactionForSession(t, handler, cookie, *committed[0].CommittedTransactionID, http.StatusOK)
	var categoryLine, accountLine string
	for _, posting := range first.JournalEntries[0].Postings {
		if posting.AccountID == checking.ID {
			accountLine = posting.LineKey
		} else {
			categoryLine = posting.LineKey
		}
	}
	mutateTransaction(t, handler, cookie, csrf, http.MethodPatch, "/api/v1/transactions/"+strconvFormatInt(first.ID), `{
		"expected_version_id":`+strconvFormatInt(first.VersionID)+`,"transaction_date":"2026-03-02","description":"",
		"journal_entries":[{"entry_date":"2026-03-02","postings":[
			{"line_key":"`+accountLine+`","account_id":`+strconvFormatInt(checking.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"-111","quantity_scale":2},
			{"line_key":"`+categoryLine+`","account_id":`+strconvFormatInt(groceries.ID)+`,"commodity_id":`+strconvFormatInt(eur)+`,"quantity_value":"111","quantity_scale":2}]}]}`, http.StatusOK)
	mutateTransaction(t, handler, cookie, csrf, http.MethodPost, "/api/v1/transactions/"+strconvFormatInt(*committed[1].CommittedTransactionID)+"/void", `{"change_reason":"duplicate charge"}`, http.StatusOK)
	corrected := auditRat(t, "-31367.58") // -31369.49 - 0.10 + 2.01
	assert.Equal(t, corrected, c.balance(checking.ID, eur, "2026-12-31"))

	// The repeat import offers only the row that never committed.
	again := uploadImportForAudit(t, c, "large.qif", statement, nil)
	againRows := allImportRowsForAudit(t, c, again.Batch.ID)
	assert.Equal(t, map[string]int{"duplicate": 249, "new": 1}, dedupeCountsForAudit(againRows))
	require.Equal(t, "new", againRows[7].DedupeStatus)
	patchImportBatchForSession(t, handler, cookie, csrf, again.Batch.ID, resolutionPatchBody(t, checking.ID, eur, groceries.ID, againRows[7].ID), http.StatusNoContent)
	retry := commitImportBatchForSession(t, handler, cookie, csrf, again.Batch.ID, "", http.StatusOK)
	assert.Equal(t, 1, retry.CommittedCount)
	assert.Equal(t, 249, retry.SkippedCount)
	assert.Equal(t, auditRat(t, "-31375.59"), c.balance(checking.ID, eur, "2026-12-31"), "-31367.58 - 8.01; corrections kept")

	// A third import of the same file commits nothing.
	third := uploadImportForAudit(t, c, "large.qif", statement, nil)
	assert.Equal(t, map[string]int{"duplicate": 250}, dedupeCountsForAudit(allImportRowsForAudit(t, c, third.Batch.ID)))
	none := commitImportBatchForSession(t, handler, cookie, csrf, third.Batch.ID, "", http.StatusOK)
	assert.Equal(t, 0, none.CommittedCount)
	assert.Equal(t, auditRat(t, "-31375.59"), c.balance(checking.ID, eur, "2026-12-31"))
}

// TestImportAuditQIFReexportIdentityIgnoresFileNameAndPosition is the #299
// regression. A QIF row's identity is its content plus its occurrence among
// identical rows in the file, so an overlapping re-export that starts one row
// earlier, or the same rows under another file name, flags the already
// committed rows as duplicates instead of posting them twice.
func TestImportAuditQIFReexportIdentityIgnoresFileNameAndPosition(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	cookie, csrf, eur, checking, groceries := bootstrapImportAPITest(t, handler)
	c := ledgerAuditClient{t: t, handler: handler, cookie: cookie, csrf: csrf}

	commitAll := func(started startImportResponse) commitImportBatchResponse {
		t.Helper()
		rows := allImportRowsForAudit(t, c, started.Batch.ID)
		patchImportBatchForSession(t, handler, cookie, csrf, started.Batch.ID, resolutionPatchBody(t, checking.ID, eur, groceries.ID, rowIDsForAudit(rows)...), http.StatusNoContent)
		return commitImportBatchForSession(t, handler, cookie, csrf, started.Batch.ID, "", http.StatusOK)
	}

	coffeeAndLunch := qifRow("06/01/26", "-10.00", "Coffee") + qifRow("06/02/26", "-20.00", "Lunch")
	first := uploadImportForAudit(t, c, "bank.qif", []byte(coffeeAndLunch), nil)
	require.Equal(t, map[string]int{"new": 2}, dedupeCountsForAudit(allImportRowsForAudit(t, c, first.Batch.ID)))
	committed := commitAll(first)
	require.Equal(t, 2, committed.CommittedCount)
	require.Equal(t, "-30", c.balance(checking.ID, eur, "2026-12-31")) // -10.00 - 20.00

	// The same file name, one extra earlier row: Bus is new, Coffee and Lunch
	// moved down one position but are the same transactions.
	overlapping := uploadImportForAudit(t, c, "bank.qif", []byte(qifRow("05/31/26", "-5.00", "Bus")+coffeeAndLunch), nil)
	overlapRows := allImportRowsForAudit(t, c, overlapping.Batch.ID)
	assert.Equal(t, map[string]int{"new": 1, "duplicate": 2}, dedupeCountsForAudit(overlapRows))
	for _, row := range overlapRows {
		assert.Equalf(t, row.RowIndex == 0, row.DedupeStatus == "new", "only Bus (row 0) is new, row %d is %s", row.RowIndex, row.DedupeStatus)
	}

	// The original rows under another file name.
	renamed := uploadImportForAudit(t, c, "bank-2026-06.qif", []byte(coffeeAndLunch), nil)
	assert.Equal(t, map[string]int{"duplicate": 2}, dedupeCountsForAudit(allImportRowsForAudit(t, c, renamed.Batch.ID)))

	overlapCommit := commitAll(overlapping)
	assert.Equal(t, 1, overlapCommit.CommittedCount)
	assert.Equal(t, 2, overlapCommit.SkippedCount)
	assert.Equal(t, "-35", c.balance(checking.ID, eur, "2026-12-31"), "only Bus may post: -30 - 5.00")

	renamedCommit := commitAll(renamed)
	assert.Equal(t, 0, renamedCommit.CommittedCount)
	assert.Equal(t, 2, renamedCommit.SkippedCount)
	assert.Equal(t, "-35", c.balance(checking.ID, eur, "2026-12-31"), "the renamed copy must post nothing")
}
