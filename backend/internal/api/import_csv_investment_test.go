package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSVInvestmentImportAPIReviewCommitActivityAndExport(t *testing.T) {
	t.Parallel()
	handler, _ := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "CSVPAYER")
	tax := createLedgerAccount(t, handler, f.sessionCookie, f.csrfToken, "Tax", "expense", "expense", f.commodityID, 2)
	config := fmt.Sprintf(`{"delimiter":"semicolon","date_column":"Date","amount_column":"Cash","date_layout":"YMD","decimal_separator":",","external_ref_column":"Ref","investment":{"kind":"column","kind_column":"Type","dividend_value":"DIV","withholding_value":"TAX","instrument_column":"Code","gross_column":"Gross","withholding_column":"Tax","cash_account_id":%d,"income_account_id":%d,"withholding_account_id":%d}}`, f.cashAccount.ID, f.incomeAccount.ID, tax.ID)
	created := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/import-profiles", createImportProfileRequest{Name: "Income", AdapterKind: "csv", ConfigJSON: config}, http.StatusCreated)
	var profile importProfileResponse
	require.NoError(t, json.NewDecoder(created.Body).Decode(&profile))
	staged := stageCSVInvestmentForSession(t, handler, f.sessionCookie, f.csrfToken, profile.ID, "Date;Type;Code;Cash;Gross;Tax;Ref\n2026-03-01;DIV;UNKNOWN;85,00;100,00;15,00;div\n2026-03-02;TAX;"+instrument.CommodityCode+";-5,00;;;tax\n")
	require.Len(t, staged.Rows, 2)
	previewCommitImportBatchForSession(t, handler, f.sessionCookie, staged.Batch.ID, http.StatusBadRequest)
	commitImportBatchForSession(t, handler, f.sessionCookie, f.csrfToken, staged.Batch.ID, "{}", http.StatusBadRequest)
	var resolution map[string]any
	require.NoError(t, json.Unmarshal([]byte(staged.Rows[0].ResolutionJSON), &resolution))
	resolution["instrument_commodity_id"] = instrument.CommodityID
	encoded, err := json.Marshal(resolution)
	require.NoError(t, err)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPatch, "/api/v1/imports/"+strconv.FormatInt(staged.Batch.ID, 10), patchImportBatchRequest{RowResolutions: []rowResolutionPatchRequest{{RowID: staged.Rows[0].ID, DedupeStatus: "new", ResolutionJSON: string(encoded)}}}, http.StatusNoContent)
	preview := previewCommitImportBatchForSession(t, handler, f.sessionCookie, staged.Batch.ID, http.StatusOK)
	require.Equal(t, 2, preview.IncludableCount)
	committed := commitImportBatchForSession(t, handler, f.sessionCookie, f.csrfToken, staged.Batch.ID, "{}", http.StatusOK)
	require.Equal(t, 2, committed.CommittedCount)
	listed := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/transactions?commodity_id="+strconv.FormatInt(instrument.CommodityID, 10), nil, http.StatusOK)
	var activity struct {
		Transactions []transactionResponse `json:"transactions"`
	}
	require.NoError(t, json.NewDecoder(listed.Body).Decode(&activity))
	require.Len(t, activity.Transactions, 2)
	for _, transaction := range activity.Transactions {
		require.True(t, transaction.NeedsReview)
	}
	bundle := downloadBundle(t, handler, f.sessionCookie, "")
	facts := bundle.table(t, "investment-dividend-facts.csv")
	require.Len(t, facts.rows, 2)
	for _, row := range facts.rows {
		require.Equal(t, strconv.FormatInt(instrument.CommodityID, 10), facts.column(row, "commodity_id"))
	}
}

func stageCSVInvestmentForSession(t *testing.T, handler http.Handler, cookie *http.Cookie, csrfToken string, profileID int64, contents string) startImportResponse {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("profile_id", strconv.FormatInt(profileID, 10)))
	file, err := writer.CreateFormFile("file", "income.csv")
	require.NoError(t, err)
	_, err = file.Write([]byte(contents))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/imports", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set(csrfTokenHeader, csrfToken)
	request.AddCookie(cookie)
	setSameOrigin(request)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	var staged startImportResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&staged))
	return staged
}

func TestCSVInvestmentImportAPIReconciliationConflictLeavesBatchReviewable(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "CSVRECON")
	seed := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", map[string]any{
		"transaction_date": "2026-02-01", "cash_account_id": f.cashAccount.ID, "cash_commodity_id": f.commodityID,
		"income_account_id": f.incomeAccount.ID, "amount_value": "10000", "amount_scale": 2,
	}, http.StatusCreated)
	var transaction transactionResponse
	require.NoError(t, json.NewDecoder(seed.Body).Decode(&transaction))
	reconcilePostingForSession(t, handler, f.sessionCookie, f.csrfToken, f.cashAccount.ID, f.commodityID, postingByAccount(t, transaction, f.cashAccount.ID), "2026-04-01")
	config := fmt.Sprintf(`{"delimiter":"semicolon","date_column":"Date","amount_column":"Cash","date_layout":"YMD","decimal_separator":",","investment":{"kind":"dividend","instrument_column":"Code","gross_column":"Gross","cash_account_id":%d,"income_account_id":%d}}`, f.cashAccount.ID, f.incomeAccount.ID)
	created := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/import-profiles", createImportProfileRequest{Name: "Income", AdapterKind: "csv", ConfigJSON: config}, http.StatusCreated)
	var profile importProfileResponse
	require.NoError(t, json.NewDecoder(created.Body).Decode(&profile))
	staged := stageCSVInvestmentForSession(t, handler, f.sessionCookie, f.csrfToken, profile.ID, "Date;Code;Cash;Gross\n2026-03-01;"+instrument.CommodityCode+";85,00;85,00\n")
	var before int
	require.NoError(t, database.QueryRow("SELECT count(*) FROM audit_events").Scan(&before))
	preview := previewCommitImportBatchForSession(t, handler, f.sessionCookie, staged.Batch.ID, http.StatusOK)
	require.Len(t, preview.ReconciliationIssues, 1)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/imports/"+strconv.FormatInt(staged.Batch.ID, 10)+"/commit", map[string]any{}, http.StatusConflict)
	var after int
	require.NoError(t, database.QueryRow("SELECT count(*) FROM audit_events").Scan(&after))
	require.Equal(t, before, after)
	var status string
	require.NoError(t, database.QueryRow("SELECT status FROM import_batches WHERE id = ?", staged.Batch.ID).Scan(&status))
	require.Equal(t, "previewing", status)
	require.Len(t, preview.ReconciliationCheckpoints, 1)
	committed := commitImportBatchForSession(t, handler, f.sessionCookie, f.csrfToken, staged.Batch.ID, reviewedImportOverrideBody(t, handler, f.sessionCookie, staged.Batch.ID), http.StatusOK)
	require.Equal(t, 1, committed.CommittedCount)
}
