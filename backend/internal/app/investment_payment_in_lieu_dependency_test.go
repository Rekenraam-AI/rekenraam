package app

import (
	"context"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"testing"
)

// #388 retires #257's explanatory dependency. Position-changing commands
// still conserve real quantities/basis and retain journal/version/impact guards.
func requirePaymentAttributionWarning(t *testing.T, f *investmentsTestFixture, paymentID int64) {
	t.Helper()
	result := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentAttribution)
	require.Equal(t, SelfCheckWarning, result.Status)
	require.Contains(t, result.WarningSample, SelfCheckWarningSample{TransactionID: paymentID, Reason: db.AttributionNoShort})
	require.Equal(t, SelfCheckPassed, resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentFoundation).Status)
}

func TestReverseShortSaleLeavesPaymentCashAndAttributionWarning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	opening := shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	input := ReverseInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: opening.Transaction.ID, Reason: "never shorted"}
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReverseShortSaleReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.investmentService.ReverseShortSale(ctx, input)
	require.NoError(t, err)
	require.Equal(t, int64(-5000), accountBalance(t, f, f.cashAccountID))
	require.Equal(t, int64(5000), accountBalance(t, f, expense))
	requirePaymentAttributionWarning(t, f, payment.ID)
	// A payment's attribution never authorizes an actual cover without units.
	before = buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ShortCover(ctx, coverInput(f, "2026-05-01", 1, 7000, "fifo"))
	require.Error(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestReplaceShortSaleAfterExDateLeavesPaymentCashAndWarning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	opening := shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	moved := ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: opening.Transaction.ID, Reason: "date", Replacement: shortInput(f, "2026-04-01", 10, 100000)}
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceShortSaleReconciliationImpact(ctx, moved)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	replaced, err := f.investmentService.ReplaceShortSale(ctx, moved)
	require.NoError(t, err)
	require.Equal(t, int64(95000), accountBalance(t, f, f.cashAccountID))
	requirePaymentAttributionWarning(t, f, payment.ID)
	_, err = f.investmentService.ReplaceShortSale(ctx, ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: replaced.Replacement.Transaction.ID,
		Reason: "actually March", Replacement: shortInput(f, "2026-03-31", 10, 100000)})
	require.NoError(t, err)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestBackdatedFullShortCoverLeavesPaymentEffectiveWithWarning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.shortCoverReconciliationImpact(ctx, coverInput(f, "2026-03-20", 10, 70000, "fifo"))
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.investmentService.ShortCover(ctx, coverInput(f, "2026-03-20", 10, 70000, "fifo"))
	require.NoError(t, err)
	require.Equal(t, int64(25000), accountBalance(t, f, f.cashAccountID))
	require.Equal(t, int64(5000), accountBalance(t, f, expense))
	requirePaymentAttributionWarning(t, f, payment.ID)
}

func TestReplaceShortCoverBeforeExDateLeavesPaymentEffectiveWithWarning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	cover, err := f.investmentService.ShortCover(ctx, coverInput(f, "2026-05-01", 10, 70000, "fifo"))
	require.NoError(t, err)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	input := ReplaceInvestmentSaleInput{OwnerUserID: f.ownerUserID, TransactionID: cover.Transaction.ID, Reason: "March", Replacement: coverInput(f, "2026-03-20", 10, 70000, "fifo")}
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplaceShortCoverReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	if impact.GainImpact != nil {
		input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	_, err = f.investmentService.ReplaceShortCover(ctx, input)
	require.NoError(t, err)
	requirePaymentAttributionWarning(t, f, payment.ID)
}

func TestShortSaleCorrectionPreparedBeforePaymentDoesNotNeedAttributionPermission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	opening := shortSaleOn(t, f, "2026-03-02", 10, 100000)
	operation, params, err := f.investmentService.prepareShortSaleReversalWrite(ctx, ReverseInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: opening.Transaction.ID, Reason: "never shorted"})
	require.NoError(t, err)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	_, err = f.investmentService.repository.ReverseBuy(ctx, params, operation)
	require.NoError(t, err)
	requirePaymentAttributionWarning(t, f, payment.ID)
}

func TestUnentitledPaymentInLieuDoesNotBlockUnrelatedCommands(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	shortSaleOn(t, f, "2026-04-02", 10, 100000)
	_, err = f.investmentService.ShortCover(ctx, coverInput(f, "2026-05-01", 10, 70000, "fifo"))
	require.NoError(t, err)
	requirePaymentAttributionWarning(t, f, payment.ID)
}
