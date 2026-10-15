package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
)

func TestIncomeCoverageAPIDisclosesCashOnlyImportAndFollowsRepair(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	handler, database := newSetupTestHandler(t)
	f := bootstrapInvestmentAPITest(t, handler)
	instrument := createInstrumentForSession(t, handler, f, "APIPAYER")
	batch, rowID := seedAPIDividendRow(t, database, `{"date":"2026-03-01","amount":"85.00","commodity_hint":"USD","memo":"Legacy API dividend","external_ref":"legacy"}`)
	legacy := createTransactionForSession(t, handler, f.sessionCookie, f.csrfToken, fmt.Sprintf(`{"transaction_date":"2026-03-01","description":"Legacy API dividend","journal_entries":[{"entry_date":"2026-03-01","entry_kind":"ordinary","postings":[{"account_id":%d,"commodity_id":%d,"quantity_value":"8500","quantity_scale":2},{"account_id":%d,"commodity_id":%d,"quantity_value":"-8500","quantity_scale":2}]}]}`, f.cashAccount.ID, f.commodityID, f.incomeAccount.ID, f.commodityID), http.StatusCreated)
	repository := db.NewImportRepository(database)
	row, err := repository.ImportStagedRowByID(ctx, rowID)
	require.NoError(t, err)
	_, err = repository.CommitImportedTransaction(ctx, db.CommitImportedTransactionParams{Identity: db.CreateImportCommitIdentityParams{BookID: app.BookID, DedupeFingerprint: row.DedupeFingerprint, SourceKind: "trading212", AccountID: f.cashAccount.ID, CreatedAt: "2026-03-02T00:00:00Z"}, Row: db.CommitImportStagedRowParams{RowID: rowID, CommitStatus: "committed"}}, func(*sql.Tx) (int64, error) { return legacy.ID, nil })
	require.NoError(t, err)

	doInvestmentRequest(t, handler, nil, "", http.MethodGet, "/api/v1/investments/income-coverage", nil, http.StatusUnauthorized)
	for _, query := range []string{"?limit=101", "?limit=-1", "?commodity_id=0", "?cursor=bad"} {
		doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/income-coverage"+query, nil, http.StatusBadRequest)
	}
	read := func(query string) app.DividendCoverage {
		res := doInvestmentRequest(t, handler, f.sessionCookie, "", http.MethodGet, "/api/v1/investments/income-coverage"+query, nil, http.StatusOK)
		var coverage app.DividendCoverage
		require.NoError(t, json.NewDecoder(res.Body).Decode(&coverage))
		return coverage
	}
	coverage := read("")
	require.Len(t, coverage.Unassigned, 1)
	item := coverage.Unassigned[0]
	require.Equal(t, app.CoverageItemImportedCash, item.Kind)
	require.Equal(t, legacy.ID, item.TransactionID)
	require.Equal(t, "8500", item.Cash.Value.String())
	require.Equal(t, app.CoverageUnresolved, item.GrossState)
	require.Nil(t, item.Gross)
	require.Equal(t, app.CoverageUnresolved, item.WithholdingState)
	require.Equal(t, app.CoveragePayerSourceIdentified, item.PayerState)
	require.Equal(t, &instrument.CommodityID, item.SourcePayerCommodityID)
	require.Equal(t, batch, item.Source.BatchID)
	require.Equal(t, "85.00", item.Source.CashAmount)
	require.Len(t, coverage.Instruments, 1)
	require.Equal(t, app.CoverageNone, coverage.Instruments[0].IncomeState)
	require.Nil(t, coverage.Instruments[0].Gross)
	require.Equal(t, 1, coverage.Instruments[0].ImportedUnclassifiedCount)

	summary := read(fmt.Sprintf("?commodity_id=%d&limit=0", instrument.CommodityID))
	require.Empty(t, summary.Unassigned)
	require.Nil(t, summary.NextCursor)
	require.Len(t, summary.UnassignedTotals, 1)
	require.Len(t, summary.Instruments, 1)

	gross, tax := "85.00", "0"
	request := reviewImportDividendRequest{Resolution: app.ImportRowResolution{AccountID: f.cashAccount.ID, CommodityID: f.commodityID, InstrumentCommodityID: instrument.CommodityID, CategoryID: &f.incomeAccount.ID, GrossAmount: &gross, WithholdingAmount: &tax}, Reason: "Statement payer declaration", ExpectedTransactionVersionID: legacy.VersionID}
	doInvestmentRequest(t, handler, f.sessionCookie, f.csrfToken, http.MethodPost, fmt.Sprintf("/api/v1/imports/%d/rows/%d/review-dividend", batch, rowID), request, http.StatusOK)
	coverage = read("")
	require.Empty(t, coverage.Unassigned)
	require.Empty(t, coverage.UnassignedTotals)
	require.Len(t, coverage.Instruments, 1)
	require.Equal(t, app.CoverageKnown, coverage.Instruments[0].IncomeState)
	require.Equal(t, app.CoverageKnownZero, coverage.Instruments[0].WithholdingState)
	require.Equal(t, "8500", coverage.Instruments[0].Gross.Value.String())
}
