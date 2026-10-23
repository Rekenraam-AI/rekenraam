package app

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/migrations"
)

func TestDividendAttributionMigrationPreservesHistoryBackfillsOnlyRecordedShapeAndRefusesLoss(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 24)
	require.NoError(t, err)
	input := cashDividendInput(f, 0, 10000, 0)
	input.CommodityID = NullableValue(f.stockCommodityID)
	assigned, err := f.investmentService.Dividend(ctx, input)
	require.NoError(t, err)
	readAssigned := func() string {
		var row string
		require.NoError(t, f.database.QueryRow(`SELECT json_array(operation_id,book_id,commodity_id,withholding_only,created_audit_event_id) FROM investment_dividend_facts`).Scan(&row))
		return row
	}
	original := readAssigned()
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	require.Equal(t, original, readAssigned())
	_, err = provider.DownTo(ctx, 24)
	require.NoError(t, err)
	require.Equal(t, original, readAssigned())
	// The old writer omitted facts for unassigned gross cash. Reproduce that
	// exact historical shape using its normal journal/components, not guessed metadata.
	input.CommodityID = NullablePatch[int64]{}
	params, err := f.investmentService.prepareDividendWrite(ctx, input)
	require.NoError(t, err)
	params.Dividend = nil
	historical, err := f.transactionService.repository.CreateTransaction(ctx, params)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	delete(before, "investment_dividend_facts")
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	after := buyReplacementPreviewSnapshot(t, f.database)
	delete(after, "investment_dividend_facts")
	require.Equal(t, before, after, "journal, components, lots, source and audit are unchanged")
	var payer *int64
	var shape bool
	require.NoError(t, f.database.QueryRow(`SELECT f.commodity_id,f.withholding_only FROM investment_dividend_facts f
		JOIN investment_operation_journal_links l ON l.operation_id=f.operation_id AND l.role='primary'
		WHERE l.transaction_version_id=?`, historical.VersionID).Scan(&payer, &shape))
	require.Nil(t, payer)
	require.False(t, shape)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, assigned.ID)
	require.NoError(t, err)
	require.Equal(t, &f.stockCommodityID, chain.EffectiveDividend.CommodityID)
	_, err = f.database.Exec(`UPDATE investment_dividend_facts SET commodity_id=NULL`)
	require.ErrorContains(t, err, "immutable")
	_, err = f.database.Exec(`DELETE FROM investment_dividend_facts`)
	require.ErrorContains(t, err, "immutable")
	stable := buyReplacementPreviewSnapshot(t, f.database)
	_, err = provider.DownTo(ctx, 24)
	require.ErrorContains(t, err, "CHECK constraint failed")
	require.Equal(t, stable, buyReplacementPreviewSnapshot(t, f.database))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestDividendAttributionMigrationPreservesAndDiagnosesLegacyDuplicateSourceClaims(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	additional := stageAdditionalDividendClaim(t, f, july, "legacy-second-claim")
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 24)
	require.NoError(t, err)
	input := DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: NullableValue(f.instrument.CommodityID),
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2}
	original, err := f.investmentSvc.Dividend(ctx, input)
	require.NoError(t, err)
	replaced, err := f.investmentSvc.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: 1, TransactionID: original.ID, Reason: "legacy correction", Replacement: input})
	require.NoError(t, err)
	// This is exactly the two-version claim the old schema allowed. New
	// candidate/storage guards must refuse it, not erase existing owner evidence.
	_, err = f.database.Exec(`UPDATE investment_event_suggestions SET linked_transaction_id=?,status='linked' WHERE id=?`, original.ID, july.ID)
	require.NoError(t, err)
	_, err = f.database.Exec(`UPDATE investment_event_suggestions SET linked_transaction_id=?,status='linked' WHERE id=?`, replaced.Replacement.ID, additional)
	require.NoError(t, err)
	fixture := &investmentsTestFixture{database: f.database}
	before := buyReplacementPreviewSnapshot(t, f.database)
	claims := func() string {
		var result string
		require.NoError(t, f.database.QueryRow(`SELECT json_group_array(json_array(id,provider_event_id,status,generated_transaction_id,linked_transaction_id))
			FROM (SELECT * FROM investment_event_suggestions ORDER BY id)`).Scan(&result))
		return result
	}
	originalClaims := claims()
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	require.Equal(t, originalClaims, claims())
	run := mustRunInvestmentSelfCheck(t, fixture)
	foundation := resultFor(t, run, CheckInvestmentFoundation)
	require.Equal(t, SelfCheckFailed, foundation.Status)
	require.Contains(t, foundation.Summary, "cash payment claimed by multiple provider events")
	for _, id := range []string{CheckEntryBalance, CheckTransactionBalance, CheckBookBalance, CheckLotReconciliation, CheckInvestmentReplay} {
		require.Equal(t, SelfCheckPassed, resultFor(t, run, id).Status, id)
	}
}
