package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
)

// R13 slice 9 (instrument_returns_v1): saved runs carry the per-instrument
// read only when asked, and a run saved before the flag existed keeps
// verifying and reopening byte for byte.

func priceForReturnsRun(t *testing.T, handler http.Handler, f investmentAPITestFixture, commodityID int64, date, price string) {
	t.Helper()
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/pricing/manual", manualPriceRequest{
		BaseCommodityID: commodityID, QuoteCommodityID: f.commodityID, ValuationDate: date, PriceValue: price, PriceScale: 0,
		BaseQuantityValue: "1", Reason: "statement", ObservedAt: date + "T12:00:00Z"}, http.StatusCreated)
}

// The golden document was saved by the slice 6 code (before instruments
// existed); its bytes and digests are inserted exactly as stored.
func TestReturnsRunSavedBeforeInstrumentsReopensUnchanged(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	raw, err := os.ReadFile(filepath.Join("testdata", "returns-run-slice6.json"))
	require.NoError(t, err)
	var golden struct {
		CapturedAt     string `json:"captured_at"`
		CreatedAt      string `json:"created_at"`
		SnapshotSHA256 string `json:"snapshot_sha256"`
		InputsSHA256   string `json:"inputs_sha256"`
		OutputSHA256   string `json:"output_sha256"`
		InputsJSON     string `json:"inputs_json"`
		OutputJSON     string `json:"output_json"`
	}
	require.NoError(t, json.Unmarshal(raw, &golden))
	require.NotContains(t, golden.InputsJSON, "instruments")
	require.NotContains(t, golden.OutputJSON, "instrument_returns")
	ctx := context.Background()
	var owner, audit int64
	require.NoError(t, database.QueryRowContext(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&owner))
	require.NoError(t, database.QueryRowContext(ctx, `SELECT max(id) FROM audit_events`).Scan(&audit))
	result, err := database.ExecContext(ctx, `INSERT INTO investment_returns_runs
 (book_id, schema_version, captured_at, created_at, created_by_user_id, audit_event_id,
 snapshot_sha256, inputs_sha256, output_sha256, inputs_json, output_json) VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.BookID, golden.CapturedAt, golden.CreatedAt, owner, audit, golden.SnapshotSHA256, golden.InputsSHA256, golden.OutputSHA256,
		golden.InputsJSON, golden.OutputJSON)
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)

	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/returns/runs/"+strconv.FormatInt(id, 10), nil, http.StatusOK)
	var reopened db.ReturnsRunRecord
	require.NoError(t, json.NewDecoder(res.Body).Decode(&reopened))
	assert.Equal(t, golden.InputsJSON, string(reopened.InputsJSON))
	assert.Equal(t, golden.OutputJSON, string(reopened.OutputJSON))
	assert.Equal(t, golden.OutputSHA256, reopened.OutputSHA256)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/returns/runs", nil, http.StatusOK)
	var page struct {
		Runs []db.ReturnsRunRecord `json:"runs"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Runs, 1)
	assert.Equal(t, golden.InputsJSON, string(page.Runs[0].InputsJSON))

	// A new run without the flag keeps the slice 6 request shape.
	request := map[string]any{"from": "2026-02-01", "to": "2026-03-31", "reporting_currency_id": f.commodityID, "instruments": false}
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/returns/runs", request, http.StatusCreated)
	var plain db.ReturnsRunRecord
	require.NoError(t, json.NewDecoder(res.Body).Decode(&plain))
	assert.NotContains(t, string(plain.InputsJSON), "instruments")
	assert.NotContains(t, string(plain.OutputJSON), "instrument_returns")

	backup := filepath.Join(t.TempDir(), "restored.sqlite")
	require.NoError(t, db.BackupSQLiteDatabase(ctx, database, backup))
	restored, err := db.Open(ctx, db.SQLiteFileURL(backup))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	readOnly, err := db.OpenReadOnly(ctx, db.SQLiteFileURL(backup))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, readOnly.Close()) })
	check, err := app.NewSelfCheckService(db.NewSelfCheckRepository(restored, readOnly)).RunSelfCheck(ctx, "manual")
	require.NoError(t, err)
	assert.Equal(t, app.SelfCheckPassed, check.Status)
}

// A run asked for instruments freezes exactly the live instrument view:
// values, reasons, coverage and the not-attributed list, after a corrected
// and a cancelled dividend; a later correction restates only the live read.
func TestReturnsRunFreezesInstrumentReturns(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	first := createInstrumentForSession(t, handler, f, "SLVA")
	second := createInstrumentForSession(t, handler, f, "SLVB")
	firstHolding := createHoldingAccountForSession(t, handler, f, first.ID)
	secondHolding := createHoldingAccountForSession(t, handler, f, second.ID)
	for _, holding := range []struct{ account, commodity int64 }{{firstHolding.ID, first.CommodityID}, {secondHolding.ID, second.CommodityID}} {
		buy := tradeRequestBody(f, holding.account, holding.commodity, "10", 100000)
		buy.TransactionDate = "2026-01-20"
		doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", buy, http.StatusCreated)
	}
	for _, date := range []string{"2026-01-31", "2026-02-10", "2026-02-28"} {
		priceForReturnsRun(t, handler, f, first.CommodityID, date, "100")
	}
	// The second instrument has no closing price: its sleeve shows the gap.
	priceForReturnsRun(t, handler, f, second.CommodityID, "2026-01-31", "100")
	dividend := func(commodityID *int64, amount int64) dividendRequest {
		request := dividendRequest{TransactionDate: "2026-02-20", ExDate: nullableField[string]{app.NullableValue("2026-02-10")},
			CashAccountID: f.cashAccount.ID, CashCommodityID: f.commodityID, IncomeAccountID: &f.incomeAccount.ID,
			AmountValue: moneyCoefficient(amount), AmountScale: 2}
		if commodityID != nil {
			request.CommodityID = nullableField[int64]{app.NullableValue(*commodityID)}
		}
		return request
	}
	post := func(body dividendRequest) transactionResponse {
		res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/dividend", body, http.StatusCreated)
		var transaction transactionResponse
		require.NoError(t, json.NewDecoder(res.Body).Decode(&transaction))
		return transaction
	}
	// Corrected: 20 recorded for the second instrument, moved to the first.
	corrected := post(dividend(&second.CommodityID, 2000))
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/transactions/"+strconv.FormatInt(corrected.ID, 10)+"/replace-dividend",
		investmentDividendReplacementRequest{Reason: "payer was the first", Replacement: dividend(&first.CommodityID, 2000)}, http.StatusCreated)
	var replaced investmentDividendReplacementResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&replaced))
	// Cancelled: 50 for the first instrument, reversed.
	cancelled := post(dividend(&first.CommodityID, 5000))
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/transactions/"+strconv.FormatInt(cancelled.ID, 10)+"/reverse-dividend",
		investmentSaleReversalRequest{Reason: "paid in error"}, http.StatusCreated)
	// Unassigned: no sleeve carries it, and it is listed.
	unassigned := post(dividend(nil, 700))

	live := func() json.RawMessage {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, fmt.Sprintf(
			"/api/v1/investments/returns?from=2026-02-01&to=2026-02-28&reporting_currency_id=%d&instruments=true", f.commodityID), nil, http.StatusOK)
		return json.RawMessage(bytes.TrimSpace(res.Body.Bytes()))
	}
	before := live()
	var auditsBefore, postingsBefore int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&auditsBefore))
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM posting_versions`).Scan(&postingsBefore))
	request := map[string]any{"from": "2026-02-01", "to": "2026-02-28", "reporting_currency_id": f.commodityID, "instruments": true}
	res = doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/returns/runs", request, http.StatusCreated)
	var run db.ReturnsRunRecord
	require.NoError(t, json.NewDecoder(res.Body).Decode(&run))
	require.True(t, db.ValidReturnsRunDocument(run))
	var audits, postings int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&audits))
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM posting_versions`).Scan(&postings))
	assert.Equal(t, auditsBefore+1, audits, "only the run's own audit event")
	assert.Equal(t, postingsBefore, postings)
	var inputs struct {
		Request app.ReturnsRunRequest `json:"request"`
	}
	require.NoError(t, json.Unmarshal(run.InputsJSON, &inputs))
	assert.True(t, inputs.Request.Instruments, "the selection is retained")

	// Live and saved agree byte for byte apart from the knowledge instant.
	// Every valuation block repeats that instant.
	var liveDoc struct {
		KnowledgeCutoff string `json:"knowledge_cutoff"`
	}
	require.NoError(t, json.Unmarshal(before, &liveDoc))
	assert.Equal(t, string(run.OutputJSON), string(bytes.ReplaceAll(before,
		[]byte(strconv.Quote(liveDoc.KnowledgeCutoff)), []byte(strconv.Quote(run.CapturedAt)))))
	var savedDoc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(run.OutputJSON, &savedDoc))
	var frozen investmentReturnsResponse
	require.NoError(t, json.Unmarshal(run.OutputJSON, &frozen))
	require.NotNil(t, frozen.InstrumentReturns)
	instruments := frozen.InstrumentReturns
	assert.Equal(t, "instrument_returns_v1", instruments.Policy)
	assert.Equal(t, "ex_date", instruments.IncomeRecognition)
	require.Len(t, instruments.Rows, 2)
	rows := map[int64]instrumentReturnsRowResponse{}
	for _, row := range instruments.Rows {
		rows[row.CommodityID] = row
	}
	firstRow, secondRow := rows[first.CommodityID], rows[second.CommodityID]
	require.NotNil(t, firstRow.Income.Amount)
	assert.Equal(t, "2000", firstRow.Income.Amount.QuantityValue.String(), "the corrected 20 moved here; the reversed 50 left")
	require.NotNil(t, firstRow.TimeWeighted.Value)
	assert.Equal(t, "0.020000", *firstRow.TimeWeighted.Value)
	require.NotNil(t, secondRow.Income.Amount)
	assert.Equal(t, "0", secondRow.Income.Amount.QuantityValue.String())
	assert.Nil(t, secondRow.ClosingValue.Amount)
	require.NotNil(t, secondRow.ClosingValue.Reason)
	assert.Equal(t, app.ReturnReasonValuationUnavailable, *secondRow.ClosingValue.Reason)
	require.NotNil(t, secondRow.Valuation)
	require.Len(t, secondRow.Valuation.Gaps, 1)
	assert.Equal(t, second.CommodityID, secondRow.Valuation.Gaps[0].CommodityID)
	require.Len(t, instruments.NotAttributed, 1)
	assert.Equal(t, unassigned.ID, instruments.NotAttributed[0].TransactionID)
	assert.Equal(t, "unassigned_instrument", instruments.NotAttributed[0].Reason)
	assert.Nil(t, instruments.NotAttributedGain.Amount, "a sleeve gain is unavailable")

	// Reversing the replacement restates the live read; the run is unchanged.
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost,
		"/api/v1/investments/transactions/"+strconv.FormatInt(replaced.ReplacementTransaction.ID, 10)+"/reverse-dividend",
		investmentSaleReversalRequest{Reason: "not paid"}, http.StatusCreated)
	assert.NotEqual(t, string(before), string(live()))
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/returns/runs/"+strconv.FormatInt(run.ID, 10), nil, http.StatusOK)
	var reopened db.ReturnsRunRecord
	require.NoError(t, json.NewDecoder(res.Body).Decode(&reopened))
	assert.Equal(t, run, reopened)

	// The flag and the output must agree: either side alone is refused.
	rehash := func(record db.ReturnsRunRecord) db.ReturnsRunRecord {
		record.InputsSHA256, record.OutputSHA256 = db.GainsRunDigest(record.InputsJSON), db.GainsRunDigest(record.OutputJSON)
		return record
	}
	dropped := run
	dropped.InputsJSON = bytes.Replace(run.InputsJSON, []byte(`,"instruments":true`), nil, 1)
	require.NotEqual(t, run.InputsJSON, dropped.InputsJSON)
	assert.False(t, db.ValidReturnsRunDocument(rehash(dropped)), "output with instruments, request without")
	delete(savedDoc, "instrument_returns")
	stripped := run
	var err error
	stripped.OutputJSON, err = json.Marshal(savedDoc)
	require.NoError(t, err)
	assert.False(t, db.ValidReturnsRunDocument(rehash(stripped)), "request with instruments, output without")
}
