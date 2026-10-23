package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #257: a correction or backdated cover that would leave an effective payment
// in lieu without the short it was owed on is refused inside the write with
// the payment named, and nothing is written. The owner reverses the payment
// first; #389 separately diagnoses unsupported attribution.

func requirePaymentInLieuDependency(t *testing.T, err error, payment Transaction) {
	t.Helper()
	var dependency InvestmentPaymentInLieuDependencyError
	require.ErrorAs(t, err, &dependency)
	require.ErrorIs(t, err, ErrInvestmentPaymentInLieuDependency)
	assert.Equal(t, payment.ID, dependency.TransactionID)
	assert.NotZero(t, dependency.OperationID)
}

func TestReverseShortSaleNamesTheDependentPaymentInLieu(t *testing.T) {
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
	requirePaymentInLieuDependency(t, err, payment)
	_, err = f.investmentService.ReverseShortSale(ctx, input)
	requirePaymentInLieuDependency(t, err, payment)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "nothing written")

	_, err = f.investmentService.ReversePaymentInLieu(ctx, ReverseInvestmentPaymentInLieuInput{
		OwnerUserID: f.ownerUserID, TransactionID: payment.ID, Reason: "never owed"})
	require.NoError(t, err)
	_, err = f.investmentService.ReverseShortSale(ctx, input)
	require.NoError(t, err, "with the payment reversed first the short can go")
	requireInvestmentSelfCheckPasses(t, f)
}

func TestReplaceShortSaleAfterTheExDateNamesTheDependentPaymentInLieu(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	opening := shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)

	moved := ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: opening.Transaction.ID,
		Reason: "opened after the ex-date", Replacement: shortInput(f, "2026-04-01", 10, 100000)}
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceShortSaleReconciliationImpact(ctx, moved)
	requirePaymentInLieuDependency(t, err, payment)
	_, err = f.investmentService.ReplaceShortSale(ctx, moved)
	requirePaymentInLieuDependency(t, err, payment)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "nothing written")

	// A correction that keeps the short open at the close stands.
	_, err = f.investmentService.ReplaceShortSale(ctx, ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID,
		TransactionID: opening.Transaction.ID, Reason: "proceeds were 1,200.00",
		Replacement: shortInput(f, "2026-03-31", 10, 120000)})
	require.NoError(t, err)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestBackdatedShortCoverBeforeTheExDateNamesTheDependentPaymentInLieu(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)

	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.shortCoverReconciliationImpact(ctx, coverInput(f, "2026-03-20", 10, 70000, "fifo"))
	requirePaymentInLieuDependency(t, err, payment)
	_, err = f.investmentService.ShortCover(ctx, coverInput(f, "2026-03-20", 10, 70000, "fifo"))
	requirePaymentInLieuDependency(t, err, payment)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "nothing written")

	// A partial cover leaves owed units at the close; covering on the ex-date
	// itself leaves the holder at the close before it.
	_, err = f.investmentService.ShortCover(ctx, coverInput(f, "2026-03-20", 4, 28000, "fifo"))
	require.NoError(t, err)
	_, err = f.investmentService.ShortCover(ctx, coverInput(f, "2026-04-01", 6, 42000, "fifo"))
	require.NoError(t, err)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestReplaceShortCoverBeforeTheExDateNamesTheDependentPaymentInLieu(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	cover, err := f.investmentService.ShortCover(ctx, coverInput(f, "2026-05-01", 10, 70000, "fifo"))
	require.NoError(t, err)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)

	input := ReplaceInvestmentSaleInput{OwnerUserID: f.ownerUserID, TransactionID: cover.Transaction.ID,
		Reason: "covered in March", Replacement: coverInput(f, "2026-03-20", 10, 70000, "fifo")}
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceShortCoverReconciliationImpact(ctx, input)
	requirePaymentInLieuDependency(t, err, payment)
	_, err = f.investmentService.ReplaceShortCover(ctx, input)
	requirePaymentInLieuDependency(t, err, payment)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "nothing written")

	// Reversing the cover restores owed units and never strands a payment.
	reverseShortCoverAcknowledged(t, f, cover.Transaction.ID, "no such cover")
	requireInvestmentSelfCheckPasses(t, f)
}

// The race: the reversal is planned while the short has no payment in lieu,
// then a payment is recorded before the write commits. The write's own
// transaction finds and names it.
func TestShortSaleReversalRechecksPaymentInLieuDependencyInsideTheWrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	opening := shortSaleOn(t, f, "2026-03-02", 10, 100000)
	operation, params, err := f.investmentService.prepareShortSaleReversalWrite(ctx, ReverseInvestmentBuyInput{
		OwnerUserID: f.ownerUserID, TransactionID: opening.Transaction.ID, Reason: "never shorted"})
	require.NoError(t, err)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)

	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.repository.ReverseBuy(ctx, params, operation)
	requirePaymentInLieuDependency(t, mapShortSaleCorrectionError(err), payment)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "nothing written")
	requireInvestmentSelfCheckPasses(t, f)
}

// A payment that already lacks its short (named by an attribution warning)
// does not block an unrelated command.
func TestUnentitledPaymentInLieuDoesNotBlockUnrelatedCommands(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	_, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	damagePaymentInLieuHolding(t, f)

	_, err = f.investmentService.ShortCover(ctx, coverInput(f, "2026-05-01", 10, 70000, "fifo"))
	require.NoError(t, err)
}
