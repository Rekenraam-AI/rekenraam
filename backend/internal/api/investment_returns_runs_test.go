package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
)

// R13 slice 6's named acceptance: a saved returns run reproduces byte for
// byte after a backdated buy inside its period, while the live read restates.
func TestReturnsRunReproducesAfterBackdatedBuy(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "RUNRET")
	holding := createHoldingAccountForSession(t, handler, f, instrument.ID)
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy",
		tradeRequestBody(f, holding.ID, instrument.CommodityID, "2", 4000), http.StatusCreated)

	base := "/api/v1/investments/returns/runs"
	// A raw body with the optional fields genuinely absent, not sent as null.
	request := map[string]any{"from": "2026-02-01", "to": "2026-03-31", "reporting_currency_id": f.commodityID}
	res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base, request, http.StatusCreated)
	var run db.ReturnsRunRecord
	require.NoError(t, json.NewDecoder(res.Body).Decode(&run))
	require.True(t, db.ValidReturnsRunDocument(run), string(run.InputsJSON))
	assert.Equal(t, base+"/"+strconv.FormatInt(run.ID, 10), res.Header().Get("Location"))
	var frozen investmentReturnsResponse
	require.NoError(t, json.Unmarshal(run.OutputJSON, &frozen))
	assert.Equal(t, run.CapturedAt, frozen.KnowledgeCutoff, "the run's knowledge instant is its capture")
	assert.Equal(t, "default_investment_accounts", frozen.AccountScope)
	require.NotNil(t, frozen.Contributions.Amount)
	assert.Equal(t, "4000", frozen.Contributions.Amount.QuantityValue.String())
	var inputs struct {
		Request        app.ReturnsRunRequest `json:"request"`
		SnapshotSHA256 string                `json:"snapshot_sha256"`
	}
	require.NoError(t, json.Unmarshal(run.InputsJSON, &inputs))
	assert.Equal(t, app.StalenessDays(app.DefaultRateStalenessDays), inputs.Request.MaxStalenessDays, "defaults are recorded as resolved")
	assert.Equal(t, []int64{}, inputs.Request.AccountIDs)
	assert.Nil(t, inputs.Request.BenchmarkCommodityID)
	assert.Equal(t, run.SnapshotSHA256, inputs.SnapshotSHA256)

	// A backdated buy inside the period restates the live read only.
	later := tradeRequestBody(f, holding.ID, instrument.CommodityID, "1", 2500)
	later.TransactionDate = "2026-03-01"
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, "/api/v1/investments/buy", later, http.StatusCreated)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		fmt.Sprintf("/api/v1/investments/returns?from=2026-02-01&to=2026-03-31&reporting_currency_id=%d", f.commodityID), nil, http.StatusOK)
	var live investmentReturnsResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&live))
	require.NotNil(t, live.Contributions.Amount)
	assert.Equal(t, "6500", live.Contributions.Amount.QuantityValue.String())
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/"+strconv.FormatInt(run.ID, 10)+"?download=true", nil, http.StatusOK)
	assert.Contains(t, res.Header().Get("Content-Disposition"), "rekenraam-returns-run-"+strconv.FormatInt(run.ID, 10))
	var reopened db.ReturnsRunRecord
	require.NoError(t, json.NewDecoder(res.Body).Decode(&reopened))
	assert.Equal(t, run, reopened, "saved document and provenance never change")

	// The list carries inputs only.
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base, nil, http.StatusOK)
	var page struct {
		Runs       []db.ReturnsRunRecord `json:"runs"`
		NextCursor *string               `json:"next_cursor"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Runs, 1)
	assert.Equal(t, run.InputsJSON, page.Runs[0].InputsJSON)
	assert.Empty(t, page.Runs[0].OutputJSON)
	assert.Nil(t, page.NextCursor)

	// The portable archive includes the run even when the ledger is scoped
	// outside its period; backup/restore keeps its exact bytes.
	bundle := downloadBundle(t, handler, f.sessionCookie, "?from=2026-05-01&to=2026-05-31")
	table := bundle.table(t, "investment-returns-runs.csv")
	require.Len(t, table.rows, 1)
	assert.Equal(t, string(run.InputsJSON), table.column(table.rows[0], "inputs_json"))
	assert.Equal(t, string(run.OutputJSON), table.column(table.rows[0], "output_json"))
	assert.Equal(t, run.OutputSHA256, table.column(table.rows[0], "output_sha256"))
	ctx := context.Background()
	backup := filepath.Join(t.TempDir(), "restored.sqlite")
	require.NoError(t, db.BackupSQLiteDatabase(ctx, database, backup))
	restored, err := db.Open(ctx, db.SQLiteFileURL(backup))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	reader := app.NewInvestmentService(db.NewInvestmentRepository(restored), nil, nil, nil)
	record, err := reader.ReturnsRun(ctx, run.ID)
	require.NoError(t, err)
	record.BookID = 0
	assert.Equal(t, run, record)
	readOnly, err := db.OpenReadOnly(ctx, db.SQLiteFileURL(backup))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, readOnly.Close()) })
	self := app.NewSelfCheckService(db.NewSelfCheckRepository(restored, readOnly))
	check, err := self.RunSelfCheck(ctx, "manual")
	require.NoError(t, err)
	assert.Equal(t, app.SelfCheckPassed, check.Status)

	// SQLite refuses ordinary mutation; out-of-band corruption is named, and
	// retrieval refuses rather than rebuilding.
	_, err = restored.ExecContext(ctx, `UPDATE investment_returns_runs SET output_json = '{}' WHERE id = ?`, run.ID)
	require.ErrorContains(t, err, "immutable")
	_, err = restored.ExecContext(ctx, `DELETE FROM investment_returns_runs WHERE id = ?`, run.ID)
	require.ErrorContains(t, err, "immutable")
	_, err = restored.ExecContext(ctx, `DROP TRIGGER investment_returns_runs_no_update`)
	require.NoError(t, err)
	// A consistent rehash of a changed figure still breaks the descriptor.
	tampered := []byte(`{"policy":"returns_v1"}`)
	_, err = restored.ExecContext(ctx, `UPDATE investment_returns_runs SET output_json = ?, output_sha256 = ? WHERE id = ?`,
		string(tampered), db.GainsRunDigest(tampered), run.ID)
	require.NoError(t, err)
	_, err = reader.ReturnsRun(ctx, run.ID)
	require.ErrorIs(t, err, app.ErrReturnsRunIntegrity)
	// The list verifies whole documents too: it never summarizes a run that
	// a single read refuses.
	_, _, err = reader.ListReturnsRuns(ctx, 0)
	require.ErrorIs(t, err, app.ErrReturnsRunIntegrity)
	check, err = self.RunSelfCheck(ctx, "manual")
	require.NoError(t, err)
	found := false
	for _, result := range check.Results {
		if result.CheckID == app.CheckInvestmentFoundation {
			found = true
			assert.Equal(t, app.SelfCheckFailed, result.Status)
			assert.Contains(t, result.Summary, "saved returns runs failed document integrity")
			assert.Contains(t, result.Sample, run.ID)
		}
	}
	require.True(t, found)
}

func TestReturnsRunValidationPaginationAndReadOnlyRetrieval(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	base := "/api/v1/investments/returns/runs"
	valid := app.ReturnsRunRequest{From: "2026-02-01", To: "2026-02-28", ReportingCurrencyID: f.commodityID}
	doInvestmentRequest(t, handler, nil, "", http.MethodGet, base, nil, http.StatusUnauthorized)
	doInvestmentRequest(t, handler, nil, "", http.MethodPost, base, valid, http.StatusUnauthorized)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodPost, base, valid, http.StatusForbidden)
	zero, total := int64(0), "total_return"
	for _, input := range []any{
		app.ReturnsRunRequest{},
		app.ReturnsRunRequest{From: "2026-02-30", To: "2026-03-01", ReportingCurrencyID: f.commodityID},
		app.ReturnsRunRequest{From: "2026-03-01", To: "2026-02-01", ReportingCurrencyID: f.commodityID},
		app.ReturnsRunRequest{From: "2026-02-01", To: "2099-01-01", ReportingCurrencyID: f.commodityID},
		app.ReturnsRunRequest{From: "2026-02-01", To: "2026-02-28"},
		app.ReturnsRunRequest{From: "2026-02-01", To: "2026-02-28", ReportingCurrencyID: f.commodityID, MaxStalenessDays: app.StalenessDays(32)},
		// Only omission takes the default: an explicit zero is refused as the
		// live read refuses it, and the non-nullable field refuses null.
		app.ReturnsRunRequest{From: "2026-02-01", To: "2026-02-28", ReportingCurrencyID: f.commodityID, MaxStalenessDays: app.StalenessDays(0)},
		map[string]any{"from": "2026-02-01", "to": "2026-02-28", "reporting_currency_id": f.commodityID, "max_staleness_days": nil},
		app.ReturnsRunRequest{From: "2026-02-01", To: "2026-02-28", ReportingCurrencyID: f.commodityID, AccountIDs: []int64{0}},
		app.ReturnsRunRequest{From: "2026-02-01", To: "2026-02-28", ReportingCurrencyID: f.commodityID, BenchmarkCommodityID: &zero},
		app.ReturnsRunRequest{From: "2026-02-01", To: "2026-02-28", ReportingCurrencyID: f.commodityID, BenchmarkBasis: &total},
		map[string]any{"from": "2026-02-01", "to": "2026-02-28", "reporting_currency_id": f.commodityID, "output": map[string]any{}},
		map[string]any{"from": "2026-02-01", "to": "2026-02-28", "reporting_currency_id": f.commodityID, "knowledge_cutoff": "2026-01-01T00:00:00Z"},
	} {
		doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base, input, http.StatusBadRequest)
	}
	var count int
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM investment_returns_runs`).Scan(&count))
	assert.Zero(t, count)
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE operation = 'investment.returns_run.create'`).Scan(&count))
	assert.Zero(t, count)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		fmt.Sprintf("/api/v1/investments/returns?from=2026-02-01&to=2026-02-28&reporting_currency_id=%d&max_staleness_days=0", f.commodityID), nil, http.StatusBadRequest)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/99999", nil, http.StatusNotFound)
	for _, suffix := range []string{"/0", "/bad", "?cursor=0", "?cursor=", "?cursor=bogus", "?cursor=1&cursor=2"} {
		doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+suffix, nil, http.StatusBadRequest)
	}

	ids := []int64{}
	for range 21 {
		res := doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, base, valid, http.StatusCreated)
		var run db.ReturnsRunRecord
		require.NoError(t, json.NewDecoder(res.Body).Decode(&run))
		ids = append(ids, run.ID)
	}
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE operation = 'investment.returns_run.create'`).Scan(&count))
	assert.Equal(t, 21, count, "one audit event per saved run")
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&count))
	before := count
	res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base, nil, http.StatusOK)
	var page struct {
		Runs       []db.ReturnsRunRecord `json:"runs"`
		NextCursor *string               `json:"next_cursor"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Runs, 20)
	require.NotNil(t, page.NextCursor)
	for i, run := range page.Runs {
		assert.Equal(t, ids[20-i], run.ID)
	}
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"?cursor="+*page.NextCursor, nil, http.StatusOK)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Runs, 1)
	assert.Equal(t, ids[0], page.Runs[0].ID)
	assert.Nil(t, page.NextCursor)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"/"+strconv.FormatInt(ids[0], 10), nil, http.StatusOK)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet,
		fmt.Sprintf("/api/v1/investments/returns?from=2026-02-01&to=2026-02-28&reporting_currency_id=%d", f.commodityID), nil, http.StatusOK)
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&count))
	assert.Equal(t, before, count, "GETs, including the live read, never save a run or audit")
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM investment_returns_runs`).Scan(&count))
	assert.Equal(t, 21, count)

	// Integrity is per page: a damaged oldest run, the first page's lookahead
	// row, refuses only the page it belongs to.
	_, err := database.Exec(`DROP TRIGGER investment_returns_runs_no_update`)
	require.NoError(t, err)
	_, err = database.Exec(`UPDATE investment_returns_runs SET output_json = '{}' WHERE id = ?`, ids[0])
	require.NoError(t, err)
	res = doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base, nil, http.StatusOK)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	require.Len(t, page.Runs, 20)
	require.NotNil(t, page.NextCursor)
	doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, base+"?cursor="+*page.NextCursor, nil, http.StatusInternalServerError)
}
