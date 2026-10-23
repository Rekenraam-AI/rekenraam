package app

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func coverageAmountString(t *testing.T, amount *GainsFXAmount) string {
	t.Helper()
	require.NotNil(t, amount)
	return fmt.Sprintf("%se-%d", amount.Value.String(), amount.Scale)
}

func coverageItem(t *testing.T, items []DividendCoverageItem, kind string) DividendCoverageItem {
	t.Helper()
	for _, item := range items {
		if item.Kind == kind {
			return item
		}
	}
	t.Fatalf("no %s coverage item in %+v", kind, items)
	return DividendCoverageItem{}
}

// Known, unassigned and cash-only payments stay distinct, exact and per
// currency; repairs move them into attribution without a write on read.
func TestDividendCoverageSeparatesKnownUnassignedAndCashOnlyIncome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f, fallbackBatch, fallbackRow, fallbackTransaction, payer := legacyDividendFallback(t, "85.00")
	tax := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	withholding, withholdingScale := int64(1500), 2
	_, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-04-01", CommodityID: NullableValue(payer),
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID,
		AmountValue: 10000, AmountScale: 2, WithholdingValue: &withholding, WithholdingScale: &withholdingScale, WithholdingAccountID: &tax})
	require.NoError(t, err)
	_, zeroTaxPayer, _, err := f.investmentSvc.ResolveOrCreateInstrumentForImport(ctx, f.ownerUserID, 0, "", "US5949181045", "MSFT_US_EQ")
	require.NoError(t, err)
	_, err = f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-04-02", CommodityID: NullableValue(zeroTaxPayer),
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 500, AmountScale: 2})
	require.NoError(t, err)
	unassigned, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-05-01",
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 2000, AmountScale: 2})
	require.NoError(t, err)
	_, noIncomePayer, _, err := f.investmentSvc.ResolveOrCreateInstrumentForImport(ctx, f.ownerUserID, 0, "", "", "IDLE_US_EQ")
	require.NoError(t, err)

	// An ordinary income row from the same provider and a dividend row still
	// pending review are not committed dividend cash.
	sourceBatch, err := f.importRepo.ImportBatchByID(ctx, BookID, fallbackBatch)
	require.NoError(t, err)
	batch, _ := f.stageDividendRow(t, sourceBatch.ConnectionID.Int64, trading212Dividend{Reference: "pending-dividend", Ticker: "AAPL_US_EQ", ISIN: "US0378331005", Amount: "3.00", Currency: "EUR", PaidOn: "2026-06-01", Type: "DIVIDEND"})
	cashRows, _, err := f.importService.stageParseResult(ctx, batch, ParseResult{Rows: []StagedRow{{DedupeFingerprint: "interest-cash", Date: "2026-06-02", Amount: "7.00", CommodityHint: "EUR", Memo: "Dividend interest", Raw: map[string]string{"kind": "cash_movement"}}}})
	require.NoError(t, err)
	f.resolveRowGeneric(t, batch, cashRows[len(cashRows)-1].ID, f.cashAccountID)
	committed, err := f.importService.CommitImportBatch(ctx, CommitImportBatchInput{OwnerUserID: f.ownerUserID, BatchID: batch})
	require.NoError(t, err)
	require.Equal(t, 1, committed.CommittedCount)

	before := csvIncomeDurableCount(t, f)
	coverage, err := f.investmentSvc.DividendCoverage(ctx, DividendCoverageInput{})
	require.NoError(t, err)
	require.Equal(t, before, csvIncomeDurableCount(t, f), "coverage reads never write")

	require.Len(t, coverage.Instruments, 2)
	known := coverage.Instruments[0]
	require.Equal(t, payer, known.CommodityID)
	require.Equal(t, "EUR", known.CurrencyCode)
	require.Equal(t, CoverageKnown, known.IncomeState)
	require.Equal(t, CoverageKnown, known.WithholdingState)
	require.Equal(t, 1, known.DividendCount)
	require.Equal(t, "10000e-2", coverageAmountString(t, known.Gross))
	require.Equal(t, "1500e-2", coverageAmountString(t, known.Withholding))
	require.Equal(t, "8500e-2", coverageAmountString(t, known.NetCash))
	require.Equal(t, 1, known.ImportedUnclassifiedCount, "source-identified cash is disclosed, not added")
	require.Equal(t, "8500e-2", coverageAmountString(t, known.ImportedUnclassifiedCash))
	zero := coverage.Instruments[1]
	require.Equal(t, zeroTaxPayer, zero.CommodityID)
	require.Equal(t, CoverageKnownZero, zero.WithholdingState)
	require.Equal(t, "0", zero.Withholding.Value.String())
	require.Zero(t, zero.ImportedUnclassifiedCount)
	require.Nil(t, zero.ImportedUnclassifiedCash)

	require.Len(t, coverage.UnassignedTotals, 1)
	total := coverage.UnassignedTotals[0]
	require.Equal(t, "EUR", total.CurrencyCode)
	require.Equal(t, "10500e-2", coverageAmountString(t, &total.Cash))
	require.Equal(t, 2, total.ItemCount)
	require.Equal(t, 1, total.PayerUnassignedCount)
	require.Equal(t, 1, total.EconomicsUnresolvedCount)

	require.Len(t, coverage.Unassigned, 2)
	require.Equal(t, CoverageItemNamedUnassigned, coverage.Unassigned[0].Kind, "newest first")
	named := coverageItem(t, coverage.Unassigned, CoverageItemNamedUnassigned)
	require.Equal(t, unassigned.ID, named.TransactionID)
	require.Equal(t, CoverageKnown, named.GrossState)
	require.Equal(t, "2000e-2", coverageAmountString(t, named.Gross))
	require.Equal(t, CoverageKnownZero, named.WithholdingState)
	require.Nil(t, named.Withholding)
	require.Equal(t, []string{CoverageReasonPayerUnassigned}, named.Reasons)
	require.Nil(t, named.Source)
	cash := coverageItem(t, coverage.Unassigned, CoverageItemImportedCash)
	require.Equal(t, fallbackTransaction, cash.TransactionID)
	require.Equal(t, "8500e-2", coverageAmountString(t, &cash.Cash))
	require.Equal(t, CoverageUnresolved, cash.GrossState, "cash alone is not gross income")
	require.Nil(t, cash.Gross)
	require.Equal(t, CoverageUnresolved, cash.WithholdingState, "absent withholding is not zero")
	require.Nil(t, cash.Withholding)
	require.Equal(t, CoveragePayerSourceIdentified, cash.PayerState)
	require.Equal(t, &payer, cash.SourcePayerCommodityID)
	require.Equal(t, []string{CoverageReasonPayerUnlinked, CoverageReasonGrossUnresolved, CoverageReasonWithholdingUnresolved}, cash.Reasons)
	require.Equal(t, &DividendCoverageSource{BatchID: fallbackBatch, RowID: fallbackRow, Instrument: "AAPL_US_EQ US0378331005", Date: "2026-03-01", CashAmount: "85.00", Currency: "EUR"}, cash.Source)

	scoped, err := f.investmentSvc.DividendCoverage(ctx, DividendCoverageInput{CommodityID: noIncomePayer, SummaryOnly: true})
	require.NoError(t, err)
	require.Empty(t, scoped.Instruments, "no recorded income is not a zero row")
	require.Empty(t, scoped.Unassigned)
	require.Len(t, scoped.UnassignedTotals, 1, "book-wide unassigned scope stays visible beside an instrument")

	// Owner repair of the cash fallback and native correction of the
	// unassigned payer move both into attribution.
	original, err := f.importService.transactionService.Transaction(ctx, fallbackTransaction)
	require.NoError(t, err)
	_, err = f.importService.ReviewImportDividend(ctx, ReviewImportDividendInput{OwnerUserID: f.ownerUserID, BatchID: fallbackBatch, RowID: fallbackRow,
		ExpectedTransactionVersionID: original.VersionID, Reason: "statement", Resolution: reviewedDividendResolution(f, "85.00", "0")})
	require.NoError(t, err)
	_, err = f.investmentSvc.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID, TransactionID: unassigned.ID, Reason: "assign payer",
		Replacement: DividendInput{TransactionDate: "2026-05-01", CommodityID: NullableValue(zeroTaxPayer), CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID,
			IncomeAccountID: &f.categoryAccountID, AmountValue: 2000, AmountScale: 2}})
	require.NoError(t, err)
	coverage, err = f.investmentSvc.DividendCoverage(ctx, DividendCoverageInput{})
	require.NoError(t, err)
	require.Empty(t, coverage.Unassigned)
	require.Empty(t, coverage.UnassignedTotals)
	require.Equal(t, 2, coverage.Instruments[0].DividendCount)
	require.Equal(t, "18500e-2", coverageAmountString(t, coverage.Instruments[0].Gross))
	require.Zero(t, coverage.Instruments[0].ImportedUnclassifiedCount)
	require.Equal(t, 2, coverage.Instruments[1].DividendCount)
	require.Equal(t, "2500e-2", coverageAmountString(t, coverage.Instruments[1].Gross))
}

func TestDividendCoverageCursorPagesUnassignedItemsWithoutGapsOrDuplicates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestTestFixture(t)
	want := map[int64]bool{}
	for _, date := range []string{"2026-01-05", "2026-01-05", "2026-02-01", "2026-01-01", "2026-03-01"} {
		tx, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: date,
			CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 100, AmountScale: 2})
		require.NoError(t, err)
		want[tx.ID] = true
	}
	seen := map[int64]bool{}
	var dates []string
	cursor := ""
	for pages := 0; ; pages++ {
		require.Less(t, pages, 5)
		page, err := f.investmentSvc.DividendCoverage(ctx, DividendCoverageInput{Cursor: cursor, Limit: 2})
		require.NoError(t, err)
		require.Equal(t, 5, page.UnassignedTotals[0].ItemCount)
		for _, item := range page.Unassigned {
			require.False(t, seen[item.TransactionID])
			seen[item.TransactionID] = true
			dates = append(dates, item.Date)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	require.Equal(t, want, seen)
	require.Equal(t, []string{"2026-03-01", "2026-02-01", "2026-01-05", "2026-01-05", "2026-01-01"}, dates)
	for _, bad := range []string{"x", "2026-13-01_1_1", "2026-01-01_0_1", "2026-01-01_1"} {
		_, err := f.investmentSvc.DividendCoverage(ctx, DividendCoverageInput{Cursor: bad})
		var validation ValidationError
		require.ErrorAs(t, err, &validation, bad)
	}
}
