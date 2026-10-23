package app

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

// #255: a short holder's payment in lieu of a dividend is a named cash
// charge against an expense account, with an optional recovery. It never
// touches the short lot. #388 makes holding/instrument/ex-date optional
// attribution; #389 diagnoses historical support without authorizing cash.

func paymentInLieuInput(f *investmentsTestFixture, expenseAccountID int64, owed int64) PaymentInLieuInput {
	return PaymentInLieuInput{OwnerUserID: f.ownerUserID, HoldingAccountID: NullableValue(f.holdingAccountID),
		CommodityID: NullableValue(f.stockCommodityID), ExDate: NullableValue("2026-04-01"), TransactionDate: "2026-04-03",
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, ExpenseAccountID: expenseAccountID,
		AmountValue: owed, AmountScale: 2, Memo: "ACME payment in lieu"}
}

func withRecovery(input PaymentInLieuInput, accountID, recovered int64) PaymentInLieuInput {
	scale := 2
	input.RecoveryValue, input.RecoveryScale, input.RecoveryAccountID = &recovered, &scale, &accountID
	return input
}

func shortLotState(t *testing.T, f *investmentsTestFixture) [4]string {
	t.Helper()
	var state [4]string
	require.NoError(t, f.database.QueryRow(`SELECT s.remaining_quantity_value, s.remaining_quantity_scale,
		s.remaining_cost_basis_value, s.status FROM investment_lots l
		JOIN investment_lot_state s ON s.lot_id = l.id WHERE l.position_side = 'short'`).Scan(&state[0], &state[1], &state[2], &state[3]))
	return state
}

func TestPaymentInLieuChargesExpenseAndLeavesTheShortLot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	recovery := seedTestAccountWithClass(t, f.database, "active", true, "income", "income")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	lotBefore := shortLotState(t, f)
	cashBefore := accountBalance(t, f, f.cashAccountID)

	payment, err := f.investmentService.PaymentInLieu(ctx, withRecovery(paymentInLieuInput(f, expense, 5000), recovery, 750))
	require.NoError(t, err)
	require.Equal(t, cashBefore-4250, accountBalance(t, f, f.cashAccountID), "cash pays 50.00 owed less 7.50 recovered")
	require.Equal(t, int64(5000), accountBalance(t, f, expense))
	require.Equal(t, int64(-750), accountBalance(t, f, recovery))
	require.Equal(t, lotBefore, shortLotState(t, f), "the short lot and its opening proceeds are untouched")

	var holding, commodity int64
	var exDate, exRole, payRole string
	require.NoError(t, f.database.QueryRow(`SELECT f.holding_account_id, f.commodity_id, f.ex_date,
		(SELECT event_date FROM investment_operation_dates WHERE operation_id = o.id AND date_role = 'ex'),
		(SELECT event_date FROM investment_operation_dates WHERE operation_id = o.id AND date_role = 'payment')
		FROM investment_operations o JOIN investment_payment_in_lieu_facts f ON f.operation_id = o.id
		WHERE o.operation_kind = 'payment_in_lieu'`).Scan(&holding, &commodity, &exDate, &exRole, &payRole))
	require.Equal(t, []any{f.holdingAccountID, f.stockCommodityID, "2026-04-01", "2026-04-01", "2026-04-03"},
		[]any{holding, commodity, exDate, exRole, payRole})
	var owed, recovered string
	require.NoError(t, f.database.QueryRow(`SELECT
		(SELECT amount_value FROM investment_operation_components WHERE component_kind = 'payment_in_lieu'),
		(SELECT amount_value FROM investment_operation_components WHERE component_kind = 'payment_in_lieu_recovery')`).Scan(&owed, &recovered))
	require.Equal(t, []string{"-5000", "750"}, []string{owed, recovered}, "components carry the owner's cash flow")

	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, payment.ID)
	require.NoError(t, err)
	require.True(t, chain.CanCorrectPaymentInLieu)
	require.Equal(t, "2026-04-01", chain.EffectivePaymentInLieu.ExDate)
	require.Equal(t, expense, chain.EffectivePaymentInLieu.ExpenseAccountID)
	require.Equal(t, recovery, *chain.EffectivePaymentInLieu.RecoveryAccountID)
	requireInvestmentSelfCheckPasses(t, f)

	// The short still covers exactly as opened: the charge is not folded in.
	cover, err := f.investmentService.ShortCover(ctx, coverInput(f, "2026-05-01", 10, 80000, "fifo"))
	require.NoError(t, err)
	gain, err := shortCoverResult(db.DisposalDecisionRecord{ProceedsValue: cover.DisposalDecision.ProceedsValue,
		ProceedsScale: cover.DisposalDecision.ProceedsScale, DisposedBasisValue: cover.DisposalDecision.DisposedBasisValue,
		DisposedBasisScale: cover.DisposalDecision.DisposedBasisScale})
	require.NoError(t, err)
	cents, err := gain.TruncatedTo(2).Int64()
	require.NoError(t, err)
	require.Equal(t, int64(20000), cents, "1,000.00 opened, 800.00 covered")
	requireInvestmentSelfCheckPasses(t, f)
}

func TestPaymentInLieuAllowsUnsupportedAttributionButRetainsFinancialChecks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	input := paymentInLieuInput(f, expense, 5000)
	payment, err := f.investmentService.PaymentInLieu(ctx, input)
	require.NoError(t, err)
	requirePaymentAttributionWarning(t, f, payment.ID)
	early := input
	early.TransactionDate = "2026-03-31"
	earlyPayment, err := f.investmentService.PaymentInLieu(ctx, early)
	require.NoError(t, err)
	result := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentAttribution)
	require.Contains(t, result.WarningSample, SelfCheckWarningSample{TransactionID: earlyPayment.ID, Reason: db.AttributionExDateAfterPayment})
	before := buyReplacementPreviewSnapshot(t, f.database)
	var validation ValidationError
	_, err = f.investmentService.PaymentInLieu(ctx, withRecovery(input, expense, 5000))
	require.ErrorAs(t, err, &validation)
	income := seedTestAccountWithClass(t, f.database, "active", true, "income", "income")
	before = buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, income, 5000))
	require.ErrorAs(t, err, &validation)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestPaymentInLieuPreparedBeforeShortCorrectionKeepsCashAuthority(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	opening := shortSaleOn(t, f, "2026-03-02", 10, 100000)
	params, err := f.investmentService.preparePaymentInLieuWrite(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	_, err = f.investmentService.ReverseShortSale(ctx, ReverseInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: opening.Transaction.ID, Reason: "never shorted"})
	require.NoError(t, err)
	payment, err := f.investmentService.repository.CreatePaymentInLieu(ctx, params, nil)
	require.NoError(t, err)
	requirePaymentAttributionWarning(t, f, payment.ID)
}

func TestReversePaymentInLieuRequiresReconciliationOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	checkpointID := reconcileCash(t, f, "2026-04-30", 950)

	input := ReverseInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: payment.ID, Reason: "broker reversed the charge"}
	impact, err := f.investmentService.ReversePaymentInLieuReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Len(t, impact.AffectedCheckpoints, 1)
	require.Equal(t, checkpointID, impact.AffectedCheckpoints[0].CheckpointID)
	require.Nil(t, impact.GainImpact)
	_, err = f.investmentService.ReversePaymentInLieu(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	require.Equal(t, []int64{checkpointID}, activeCheckpointIDs(t, f))

	input.ReconciliationOverride = unboundOverride
	reversal, err := f.investmentService.ReversePaymentInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, payment.ID, *reversal.CorrectionOfTransactionID)
	require.Empty(t, activeCheckpointIDs(t, f))
	require.Equal(t, int64(100000), accountBalance(t, f, f.cashAccountID))
	require.Zero(t, accountBalance(t, f, expense))
	var facts int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_payment_in_lieu_facts`).Scan(&facts))
	require.Equal(t, 2, facts, "the reversal repeats the entitlement facts")
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, payment.ID)
	require.NoError(t, err)
	require.Nil(t, chain.EffectiveTransactionID)
	require.False(t, chain.CanCorrectPaymentInLieu)
	_, err = f.investmentService.ReversePaymentInLieu(ctx, input)
	require.ErrorIs(t, err, ErrInvestmentPaymentInLieuAlreadyCorrected)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestReplacePaymentInLieuCorrectsAmountAndRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	borrow := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)

	moved := paymentInLieuInput(f, borrow, 6000)
	moved.ExDate = NullableValue("2026-02-31")
	_, err = f.investmentService.ReplacePaymentInLieu(ctx, ReplaceInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID,
		TransactionID: payment.ID, Reason: "wrong ex-date", Replacement: moved})
	var validation ValidationError
	require.ErrorAs(t, err, &validation, "a supplied ex-date must be a calendar date")

	input := ReplaceInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: payment.ID,
		Reason: "statement shows 60.00 owed, 9.00 recovered", Replacement: withRecovery(paymentInLieuInput(f, borrow, 6000), borrow, 900)}
	input.Replacement.HoldingAccountID, input.Replacement.CommodityID, input.Replacement.ExDate = NullablePatch[int64]{}, NullablePatch[int64]{}, NullablePatch[string]{}
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplacePaymentInLieuReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Empty(t, impact.AffectedCheckpoints)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "preview writes nothing")

	result, err := f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, payment.ID, *result.Inverse.CorrectionOfTransactionID)
	require.Equal(t, payment.ID, *result.Replacement.CorrectionOfTransactionID)
	require.Equal(t, int64(100000-5100), accountBalance(t, f, f.cashAccountID))
	require.Zero(t, accountBalance(t, f, expense))
	require.Equal(t, int64(5100), accountBalance(t, f, borrow), "60.00 charged, 9.00 recovered into the same account")
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, result.Replacement.ID, *chain.EffectiveTransactionID)
	require.True(t, chain.CanCorrectPaymentInLieu)
	require.Equal(t, "2026-04-01", chain.EffectivePaymentInLieu.ExDate, "omitted entitlement fields keep the original's")
	var operation string
	require.NoError(t, f.database.QueryRow(`SELECT audit.operation FROM transactions tx
		JOIN audit_events audit ON audit.id = tx.created_audit_event_id WHERE tx.id = ?`, result.Replacement.ID).Scan(&operation))
	require.Equal(t, "investment.payment_in_lieu.replace", operation)
	requireInvestmentSelfCheckPasses(t, f)

	_, err = f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.ErrorIs(t, err, ErrInvestmentPaymentInLieuAlreadyCorrected)
	_, err = f.investmentService.ReversePaymentInLieu(ctx, ReverseInvestmentPaymentInLieuInput{
		OwnerUserID: f.ownerUserID, TransactionID: result.Replacement.ID, Reason: "charged in error"})
	require.NoError(t, err)
	require.Equal(t, int64(100000), accountBalance(t, f, f.cashAccountID))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestPaymentInLieuReplacementRollsBackLateFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	reconcileCash(t, f, "2026-04-30", 950)
	input := ReplaceInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: payment.ID,
		Reason: "wrong amount", ReconciliationOverride: unboundOverride, Replacement: paymentInLieuInput(f, expense, 5500)}
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.database.Exec(`CREATE TRIGGER reject_pil_checkpoint AFTER UPDATE ON reconciliation_checkpoints
		WHEN NEW.status = 'invalidated' BEGIN SELECT RAISE(ABORT, 'forced checkpoint failure'); END`)
	require.NoError(t, err)
	_, err = f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.ErrorContains(t, err, "forced checkpoint failure")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "inverse, replacement and facts all roll back")
	_, err = f.database.Exec(`DROP TRIGGER reject_pil_checkpoint`)
	require.NoError(t, err)
	_, err = f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, int64(94500), accountBalance(t, f, f.cashAccountID))
	requireInvestmentSelfCheckPasses(t, f)
}

// Unsupported attribution is advisory; recorded cash remains valid.
// damagePaymentInLieuHolding points every payment in lieu at the cash
// account, which was never short, bypassing the immutability trigger.
func damagePaymentInLieuHolding(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	_, err := f.database.Exec(`DROP TRIGGER investment_payment_in_lieu_facts_no_update`)
	require.NoError(t, err)
	_, err = f.database.Exec(`UPDATE investment_payment_in_lieu_facts SET holding_account_id = ?`, f.cashAccountID)
	require.NoError(t, err)
}

func TestSelfCheckNamesPaymentInLieuWithoutAShortLot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	_, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	requireInvestmentSelfCheckPasses(t, f)
	// Short corrections refuse to strand the payment (#257), so the backstop
	// is exercised on damaged facts.
	damagePaymentInLieuHolding(t, f)
	result := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentAttribution)
	require.Equal(t, SelfCheckWarning, result.Status)
	require.Equal(t, db.AttributionNoShort, result.WarningSample[0].Reason)
}

func TestPaymentInLieuCorrectionsExportTheirEntitlement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	reversal, err := f.investmentService.ReversePaymentInLieu(ctx, ReverseInvestmentPaymentInLieuInput{
		OwnerUserID: f.ownerUserID, TransactionID: payment.ID, Reason: "charged twice"})
	require.NoError(t, err)
	files := exportedBundle(t, f)
	operations := map[string]map[string]string{}
	for _, row := range files["investment-operations.csv"] {
		operations[row["transaction_id"]] = row
	}
	original := operations[strconv.FormatInt(payment.ID, 10)]
	reversed := operations[strconv.FormatInt(reversal.ID, 10)]
	require.Equal(t, "payment_in_lieu", original["operation_kind"])
	require.Equal(t, []string{"reversal", "reverse", original["operation_id"]},
		[]string{reversed["operation_kind"], reversed["correction_mode"], reversed["correction_of_operation_id"]})
	facts := files["investment-payment-in-lieu-facts.csv"]
	require.Len(t, facts, 2)
	for _, fact := range facts {
		require.Equal(t, []string{strconv.FormatInt(f.holdingAccountID, 10), strconv.FormatInt(f.stockCommodityID, 10), "2026-04-01"},
			[]string{fact["holding_account_id"], fact["commodity_id"], fact["ex_date"]})
	}
	require.ElementsMatch(t, []string{original["operation_id"], reversed["operation_id"]},
		[]string{facts[0]["operation_id"], facts[1]["operation_id"]})
}
