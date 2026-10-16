package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

func statementRowFor(t *testing.T, statement GainsStatement, commodityID int64) GainsStatementRow {
	t.Helper()
	for _, row := range statement.Rows {
		if row.CommodityID == commodityID {
			return row
		}
	}
	t.Fatalf("no statement row for commodity %d in %+v", commodityID, statement.Rows)
	return GainsStatementRow{}
}

// Cash-only imported dividends are disclosed beside statement income, never
// added to it or the limited-scope total; dividend review moves a payment into
// income, while a statement saved before the repair keeps its disclosure (#246).
func TestGainsStatementDisclosesUnclassifiedImportedDividendCash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f, fallbackBatch, fallbackRow, fallbackTransaction, _ := legacyDividendFallback(t, "85.00")
	_, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-03-10",
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 2000, AmountScale: 2})
	require.NoError(t, err)

	before := csvIncomeDurableCount(t, f)
	statement, err := f.investmentSvc.GainsStatement(ctx, "2026-03-01", "2026-03-31")
	require.NoError(t, err)
	require.Equal(t, before, csvIncomeDurableCount(t, f), "statement reads never write")
	row := statementRowFor(t, statement, f.eurCommodityID)
	require.Equal(t, 1, row.IncomeCount, "only the named dividend is income")
	require.Equal(t, StatementAmount{Value: 2000, Scale: 2}, row.Income)
	require.Equal(t, 1, row.UnclassifiedDividends.PaymentCount)
	require.Equal(t, "8500e-2", coverageAmountString(t, &row.UnclassifiedDividends.Cash))
	require.Equal(t, &StatementAmount{Value: 2000, Scale: 2}, row.Total, "the limited-scope total excludes unclassified cash")

	// Outside the payment date, nothing is disclosed.
	later, err := f.investmentSvc.GainsStatement(ctx, "2026-03-02", "2026-03-31")
	require.NoError(t, err)
	outside := statementRowFor(t, later, f.eurCommodityID).UnclassifiedDividends
	require.Zero(t, outside.PaymentCount)
	require.Equal(t, "0e-0", coverageAmountString(t, &outside.Cash))

	// A saved statement freezes the disclosure in schema 3.
	run, err := f.investmentSvc.CreateGainsRun(ctx, GainsRunInput{OwnerUserID: f.ownerUserID,
		Request: GainsRunRequest{ReportType: "statement", From: "2026-03-01", To: "2026-03-31"}}, func(p GainsRunProjection) (json.RawMessage, error) {
		require.NotNil(t, p.Statement)
		return json.Marshal(struct {
			Profile GainsProfile                          `json:"profile"`
			Rows    []GainsStatementUnclassifiedDividends `json:"rows"`
		}{p.Profile, []GainsStatementUnclassifiedDividends{statementRowFor(t, *p.Statement, f.eurCommodityID).UnclassifiedDividends}})
	})
	require.NoError(t, err)
	require.Equal(t, 3, run.SchemaVersion)
	require.True(t, db.ValidGainsRunDocument(run))

	original, err := f.importService.transactionService.Transaction(ctx, fallbackTransaction)
	require.NoError(t, err)
	tax := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	resolution := reviewedDividendResolution(f, "100.00", "15.00")
	resolution.WithholdingAccountID = &tax
	_, err = f.importService.ReviewImportDividend(ctx, ReviewImportDividendInput{OwnerUserID: f.ownerUserID, BatchID: fallbackBatch, RowID: fallbackRow,
		ExpectedTransactionVersionID: original.VersionID, Reason: "statement", Resolution: resolution})
	require.NoError(t, err)

	repaired, err := f.investmentSvc.GainsStatement(ctx, "2026-03-01", "2026-03-31")
	require.NoError(t, err)
	row = statementRowFor(t, repaired, f.eurCommodityID)
	require.Zero(t, row.UnclassifiedDividends.PaymentCount, "repair moves the payment out of the disclosure")
	require.Equal(t, "0e-0", coverageAmountString(t, &row.UnclassifiedDividends.Cash))
	require.Equal(t, 2, row.IncomeCount)
	require.Equal(t, StatementAmount{Value: 12000, Scale: 2}, row.Income, "gross, not net cash, enters income")

	saved, err := f.investmentSvc.GainsRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, run, saved, "a saved statement is never recomputed")
	var frozen struct {
		Rows []GainsStatementUnclassifiedDividends `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(saved.OutputJSON, &frozen))
	require.Equal(t, 1, frozen.Rows[0].PaymentCount)
}
