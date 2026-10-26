package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

func TestDividendAttributionBaselinePreservesRecordedShape(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	input := cashDividendInput(f, 0, 10000, 0)
	input.CommodityID = NullableValue(f.stockCommodityID)
	assigned, err := f.investmentService.Dividend(ctx, input)
	require.NoError(t, err)
	input.CommodityID = NullablePatch[int64]{}
	historical, err := f.investmentService.Dividend(ctx, input)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	require.NoError(t, db.Migrate(ctx, f.database))
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	var payer *int64
	var shape bool
	require.NoError(t, f.database.QueryRow(`SELECT f.commodity_id,f.withholding_only FROM investment_dividend_facts f JOIN investment_operation_journal_links l ON l.operation_id=f.operation_id AND l.role='primary' WHERE l.transaction_version_id=?`, historical.VersionID).Scan(&payer, &shape))
	require.Nil(t, payer)
	require.False(t, shape)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, assigned.ID)
	require.NoError(t, err)
	require.Equal(t, &f.stockCommodityID, chain.EffectiveDividend.CommodityID)
	_, err = f.database.Exec(`UPDATE investment_dividend_facts SET commodity_id=NULL`)
	require.ErrorContains(t, err, "immutable")
	_, err = f.database.Exec(`DELETE FROM investment_dividend_facts`)
	require.ErrorContains(t, err, "immutable")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestDividendAttributionBaselineDiagnosesDamagedDuplicateSourceClaims(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	additional := stageAdditionalDividendClaim(t, f, july, "legacy-second-claim")
	_, err := f.database.Exec(`DROP TRIGGER investment_event_payment_identity_insert; DROP TRIGGER investment_event_payment_identity_update`)
	require.NoError(t, err)

	input := DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: NullableValue(f.instrument.CommodityID),
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2}
	original, err := f.investmentSvc.Dividend(ctx, input)
	require.NoError(t, err)
	replaced, err := f.investmentSvc.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: 1, TransactionID: original.ID, Reason: "legacy correction", Replacement: input})
	require.NoError(t, err)
	// Simulate damaged storage after deliberately removing the admission guards.
	// Self-check must still detect two claims across the correction chain.
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
	require.NoError(t, db.Migrate(ctx, f.database))
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
