package app

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

func TestPaymentInLieuRecordsUnassignedCashWithoutInventingAShort(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	input := paymentInLieuInput(f, expense, 5000)
	input.HoldingAccountID, input.CommodityID, input.ExDate = NullablePatch[int64]{}, NullablePatch[int64]{}, NullablePatch[string]{}
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err := f.investmentService.PaymentInLieuReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	payment, err := f.investmentService.PaymentInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, int64(-5000), accountBalance(t, f, f.cashAccountID))
	require.Equal(t, int64(5000), accountBalance(t, f, expense))
	var holding, instrument sql.NullInt64
	var exDate sql.NullString
	require.NoError(t, f.database.QueryRow(`SELECT holding_account_id, commodity_id, ex_date FROM investment_payment_in_lieu_facts`).Scan(&holding, &instrument, &exDate))
	require.False(t, holding.Valid)
	require.False(t, instrument.Valid)
	require.False(t, exDate.Valid)
	facts := exportedBundle(t, f)["investment-payment-in-lieu-facts.csv"]
	require.Len(t, facts, 1)
	require.Equal(t, "", facts[0]["holding_account_id"])
	require.Equal(t, "", facts[0]["commodity_id"])
	require.Equal(t, "", facts[0]["ex_date"])
	result := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentAttribution)
	require.Equal(t, SelfCheckIncomplete, result.Status)
	require.Equal(t, []int64{payment.ID}, result.UnavailableSample)
}

func TestPaymentAttributionRepairPreservesReconciledCashAndOriginalFacts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	original, err := f.investmentService.repository.PaymentInLieuOperationByTransactionID(ctx, BookID, payment.ID)
	require.NoError(t, err)
	checkpoint := reconcileCash(t, f, "2026-04-30", 950)
	lotBefore := shortLotState(t, f)
	currentID := payment.ID
	for _, step := range []struct {
		name       string
		holding    NullablePatch[int64]
		instrument NullablePatch[int64]
		exDate     NullablePatch[string]
		want       db.PaymentInLieuFact
		status     string
	}{
		{"omission preserves", NullablePatch[int64]{}, NullablePatch[int64]{}, NullablePatch[string]{}, original.Fact, SelfCheckPassed},
		{"value repairs", NullablePatch[int64]{}, NullablePatch[int64]{}, NullableValue("2026-03-01"), db.PaymentInLieuFact{HoldingAccountID: f.holdingAccountID, CommodityID: f.stockCommodityID, ExDate: "2026-03-01"}, SelfCheckWarning},
		{"null clears", NullablePatch[int64]{Set: true}, NullablePatch[int64]{Set: true}, NullablePatch[string]{Set: true}, db.PaymentInLieuFact{}, SelfCheckIncomplete},
		{"partial stays partial", NullableValue(f.holdingAccountID), NullablePatch[int64]{}, NullablePatch[string]{}, db.PaymentInLieuFact{HoldingAccountID: f.holdingAccountID}, SelfCheckIncomplete},
		{"assign supported", NullableValue(f.holdingAccountID), NullableValue(f.stockCommodityID), NullableValue("2026-04-01"), original.Fact, SelfCheckPassed},
	} {
		t.Run(step.name, func(t *testing.T) {
			replacement := paymentInLieuInput(f, expense, 5000)
			replacement.HoldingAccountID, replacement.CommodityID, replacement.ExDate = step.holding, step.instrument, step.exDate
			input := ReplaceInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: currentID, Reason: step.name, Replacement: replacement}
			before := buyReplacementPreviewSnapshot(t, f.database)
			impact, err := f.investmentService.ReplacePaymentInLieuReconciliationImpact(ctx, input)
			require.NoError(t, err)
			require.Empty(t, impact.AffectedCheckpoints)
			require.Nil(t, impact.GainImpact)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "preview is fully rolled back")
			result, err := f.investmentService.ReplacePaymentInLieu(ctx, input)
			require.NoError(t, err)
			currentID = result.Replacement.ID
			record, err := f.investmentService.repository.PaymentInLieuOperationByTransactionID(ctx, BookID, currentID)
			require.NoError(t, err)
			require.Equal(t, step.want, record.Fact)
			require.Equal(t, int64(95000), accountBalance(t, f, f.cashAccountID))
			require.Equal(t, int64(5000), accountBalance(t, f, expense))
			require.Equal(t, lotBefore, shortLotState(t, f))
			require.Equal(t, []int64{checkpoint}, activeCheckpointIDs(t, f))
			chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, payment.ID)
			require.NoError(t, err)
			require.Equal(t, step.status, chain.EffectivePaymentInLieu.Attribution.Status)
			var inverseAudit, replacementAudit int64
			require.NoError(t, f.database.QueryRow(`SELECT created_audit_event_id FROM transactions WHERE id = ?`, result.Inverse.ID).Scan(&inverseAudit))
			require.NoError(t, f.database.QueryRow(`SELECT created_audit_event_id FROM transactions WHERE id = ?`, currentID).Scan(&replacementAudit))
			require.Equal(t, inverseAudit, replacementAudit, "one owner action, one audit event")
		})
	}
	old, err := f.investmentService.repository.PaymentInLieuOperationByTransactionID(ctx, BookID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, original.Fact, old.Fact, "original attribution is immutable history")
	requireInvestmentSelfCheckPasses(t, f)
}

func TestPaymentAttributionRepairLateFailureRollsBackTheWholeCorrection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	replacement := paymentInLieuInput(f, expense, 5000)
	replacement.ExDate = NullablePatch[string]{Set: true}
	input := ReplaceInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: payment.ID, Reason: "unknown ex-date", Replacement: replacement}
	_, err = f.database.Exec(`CREATE TRIGGER reject_attribution_repair AFTER INSERT ON investment_payment_in_lieu_facts
		WHEN NEW.ex_date IS NULL BEGIN SELECT RAISE(ABORT, 'forced attribution failure'); END`)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.ErrorContains(t, err, "forced attribution failure")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestPaymentAttributionRejectsInvalidSuppliedReferencesAndDates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	for _, change := range []func(*PaymentInLieuInput){
		func(p *PaymentInLieuInput) { p.HoldingAccountID = NullableValue(f.cashAccountID) },
		func(p *PaymentInLieuInput) { p.HoldingAccountID = NullableValue(int64(987654)) },
		func(p *PaymentInLieuInput) { p.CommodityID = NullableValue(f.eurCommodityID) },
		func(p *PaymentInLieuInput) { p.CommodityID = NullableValue(int64(0)) },
		func(p *PaymentInLieuInput) { p.ExDate = NullableValue("") },
		func(p *PaymentInLieuInput) { p.ExDate = NullableValue("2026-02-31") },
	} {
		input := paymentInLieuInput(f, expense, 5000)
		change(&input)
		before := buyReplacementPreviewSnapshot(t, f.database)
		_, err := f.investmentService.PaymentInLieu(ctx, input)
		var validation ValidationError
		require.ErrorAs(t, err, &validation)
		require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	}
}

func TestPaymentAttributionDoesNotRequireDatedPostingEligibility(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	archived := seedTestAccountWithClass(t, f.database, "archived", false, "asset", "security_holding")
	input := paymentInLieuInput(f, expense, 5000)
	input.HoldingAccountID = NullableValue(archived)
	input.ExDate = NullableValue("2025-02-01") // before the holding/instrument versions exist
	payment, err := f.investmentService.PaymentInLieu(context.Background(), input)
	require.NoError(t, err)
	requirePaymentAttributionWarning(t, f, payment.ID)
}

func TestClearingAttributionAfterUnusedHoldingChangesKindPreservesCash(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	input := paymentInLieuInput(f, expense, 5000)
	payment, err := f.investmentService.PaymentInLieu(ctx, input)
	require.NoError(t, err)
	// Attribution posts nothing to the holding; its ordinary structure edit
	// remains valid. The inverse must retain that original attribution exactly.
	updated, err := f.accountService.UpdateAccount(ctx, UpdateAccountInput{OwnerUserID: f.ownerUserID, AccountID: f.holdingAccountID, OriginType: "browser_api", Operation: "account.update",
		Name: "Unused holding reclassified", AccountClass: "asset", AccountKind: "cash", DefaultCommodityID: &f.eurCommodityID,
		ChangeReason: "No security activity"})
	require.NoError(t, err)
	require.Equal(t, "cash", updated.AccountKind)
	_, err = f.investmentService.ReversePaymentInLieuReconciliationImpact(ctx, ReverseInvestmentPaymentInLieuInput{
		OwnerUserID: f.ownerUserID, TransactionID: payment.ID, Reason: "Preview original cash reversal"})
	require.NoError(t, err)
	input.HoldingAccountID = NullablePatch[int64]{Set: true}
	preview, err := f.investmentService.ReplacePaymentInLieuReconciliationImpact(ctx, ReplaceInvestmentPaymentInLieuInput{
		OwnerUserID: f.ownerUserID, TransactionID: payment.ID, Reason: "Holding no longer attributed", Replacement: input})
	require.NoError(t, err)
	require.Empty(t, preview.AffectedCheckpoints)
	result, err := f.investmentService.ReplacePaymentInLieu(ctx, ReplaceInvestmentPaymentInLieuInput{
		OwnerUserID: f.ownerUserID, TransactionID: payment.ID, Reason: "Holding no longer attributed", Replacement: input})
	require.NoError(t, err)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, result.Replacement.ID)
	require.NoError(t, err)
	require.Zero(t, chain.EffectivePaymentInLieu.HoldingAccountID)
}
