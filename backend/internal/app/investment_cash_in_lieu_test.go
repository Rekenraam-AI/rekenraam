package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// T-147: cash in lieu of a split's fraction. The contract example: 3 shares
// split 3-for-2 hold 4.5; the 0.5 fraction settles for 8.00 EUR.

func cashInLieuSetup(t *testing.T) (*investmentsTestFixture, InvestmentSplitResult) {
	t.Helper()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-05-01", 3, 3000)
	split, err := f.investmentService.Split(context.Background(), splitInput(f, "2026-06-01", 3, 2))
	require.NoError(t, err)
	return f, split
}

func cashInLieuInput(f *investmentsTestFixture, splitTransactionID int64) CashInLieuInput {
	return CashInLieuInput{OwnerUserID: f.ownerUserID, SettledTransactionID: splitTransactionID,
		DisposalOn: "2026-06-01", PaymentOn: "2026-06-05", QuantityValue: exact.New(5), QuantityScale: 1,
		CashAccountID: f.cashAccountID, CurrencyID: f.eurCommodityID, ProceedsValue: 800, ProceedsScale: 2,
		Memo: "cash in lieu"}
}

func TestCashInLieuDisposesFractionWithCashOnItsPaymentDate(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	preview, impact, err := f.investmentService.PreviewCashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
	require.NoError(t, err)
	assert.Empty(t, impact.AffectedCheckpoints)
	result, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
	require.NoError(t, err)
	assert.Equal(t, preview.Allocations, result.Allocations, "preview takes what the commit takes")

	require.Len(t, result.Transaction.JournalEntries, 2, "security and cash on their own dates")
	assert.Equal(t, "2026-06-01", result.Transaction.JournalEntries[0].EntryDate)
	assert.Equal(t, "2026-06-05", result.Transaction.JournalEntries[1].EntryDate)
	var kind string
	var linkedSplit int64
	require.NoError(t, f.database.QueryRow(`SELECT o.operation_kind, f.settled_operation_id
		FROM investment_cash_in_lieu_facts f JOIN investment_operations o ON o.id = f.operation_id`).Scan(&kind, &linkedSplit))
	assert.Equal(t, "cash_in_lieu", kind)
	assert.Equal(t, transferOperationID(t, f, split.Transaction.ID), linkedSplit)

	quantity, remaining := openPositionBasis(t, f, f.holdingAccountID)
	requireScaled(t, 4, 0, quantity, "4.5 less the 0.5 fraction")
	disposed := saleEffectiveBasis(t, f, result.Transaction.ID)
	disposed.AddScaled(remaining)
	requireScaled(t, 3000, 2, disposed, "the fraction's basis plus the rest conserve 30.00")
	requireInvestmentSelfCheckPasses(t, f)
}

// #194: enter the fraction after recording a later sale. The fraction takes
// 30 / 9 = 3.333333 at the currency's allocation scale; selling the
// remaining four units then takes the exact remainder, 26.666667.
func TestCashInLieuBackdatedBehindALaterSaleRevisesIt(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	sale := sellInput(f, "2026-07-01", 4)
	sale.CostBasisMethod = "fifo"
	sold, err := f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)
	requireScaled(t, 26666666, 6, saleEffectiveBasis(t, f, sold.Transaction.ID), "four ninths of twice the opening basis, truncated")

	input := cashInLieuInput(f, split.Transaction.ID)
	input.CostBasisMethod = "fifo"
	before := buyReplacementPreviewSnapshot(t, f.database)
	preview, impact, err := f.investmentService.PreviewCashInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "preview leaves every durable row unchanged")
	require.NotNil(t, impact.GainImpact)
	require.Len(t, impact.GainImpact.Changes, 1, "only the existing sale is restated")
	change := impact.GainImpact.Changes[0]
	require.Equal(t, db.GainImpactRevised, change.Kind)
	require.Equal(t, sold.Transaction.ID, change.TransactionID)
	require.NotNil(t, change.Before)
	require.NotNil(t, change.After)
	require.Equal(t, "2026-07-01", change.After.DisposalDate)
	require.Equal(t, "fifo", change.After.CostBasisMethod)
	requireScaled(t, 26666666, 6, change.Before.DisposedBasis, "original sale basis")
	requireScaled(t, 26666667, 6, change.After.DisposedBasis, "sale takes the remainder after the fraction")
	require.NotEmpty(t, impact.GainImpact.Acknowledgement)

	_, err = f.investmentService.CashInLieu(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "unacknowledged replay writes nothing")

	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	result, err := f.investmentService.CashInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, preview.Allocations, result.Allocations)
	requireScaled(t, 3333333, 6, saleEffectiveBasis(t, f, result.Transaction.ID), "fraction takes one ninth of the opening basis")
	requireScaled(t, 26666667, 6, saleEffectiveBasis(t, f, sold.Transaction.ID), "acknowledged commit revises the later sale")
	var original exact.Coefficient
	var originalScale, revisions int
	require.NoError(t, f.database.QueryRow(`SELECT d.disposed_basis_value, d.disposed_basis_scale,
		(SELECT COUNT(*) FROM investment_disposal_revisions r WHERE r.decision_id = d.id)
		FROM investment_disposal_decisions d WHERE d.transaction_id = ?`, sold.Transaction.ID).
		Scan(&original, &originalScale, &revisions))
	requireScaled(t, 26666666, 6, exact.ScaledIntFromCoefficient(original, originalScale), "original decision remains audit evidence")
	require.Equal(t, 1, revisions)
	quantity, basis := openPositionBasis(t, f, f.holdingAccountID)
	require.Zero(t, quantity.Sign(), "the fraction and sale close the holding")
	require.Zero(t, basis.Sign(), "all opening basis is disposed")
	requireSelfCheckPasses(t, f)
}

func TestCashInLieuBackdatedBehindAFullSaleNamesItAndWritesNothing(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	sale := sellInput(f, "2026-07-01", 0)
	sale.QuantityValue, sale.QuantityScale = exact.New(45), 1
	sale.CostBasisMethod = "fifo"
	sold, err := f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	input := cashInLieuInput(f, split.Transaction.ID)
	_, _, err = f.investmentService.PreviewCashInLieu(ctx, input)
	var dependency InvestmentSaleDependencyError
	require.ErrorAs(t, err, &dependency)
	require.Equal(t, transferOperationID(t, f, sold.Transaction.ID), dependency.OperationID)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "impossible preview writes nothing")

	_, err = f.investmentService.CashInLieu(ctx, input)
	require.ErrorAs(t, err, &dependency)
	require.Equal(t, transferOperationID(t, f, sold.Transaction.ID), dependency.OperationID)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "impossible commit writes nothing")
	requireSelfCheckPasses(t, f)
}

func TestCashInLieuRefusesWholeShares(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	input := cashInLieuInput(f, split.Transaction.ID)
	input.QuantityValue, input.QuantityScale = exact.New(1), 0
	_, err := f.investmentService.CashInLieu(context.Background(), input)
	require.ErrorIs(t, err, ErrCashInLieuNotFraction)
}

func TestCashInLieuRefusesDisposalBeforeItsSplit(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	input := cashInLieuInput(f, split.Transaction.ID)
	input.DisposalOn = "2026-05-15"
	_, err := f.investmentService.CashInLieu(context.Background(), input)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
}

// #392: the original three units still support a half-unit disposal.
func TestSplitReversalKeepsValidCashInLieuAndRevisesItsBasis(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
	require.NoError(t, err)
	_, err = acknowledgedReverseSplit(ctx, f.investmentService, ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, TransactionID: split.Transaction.ID, Reason: "Entered in error"})
	require.NoError(t, err)
	quantity, basis := openPositionBasis(t, f, f.holdingAccountID)
	requireScaled(t, 25, 1, quantity, "3 original units less the half-share")
	requireScaled(t, 2500, 2, basis, "30 less the revised disposal basis 5")
	requireScaled(t, 500, 2, saleEffectiveBasis(t, f, cil.Transaction.ID), "original units set the half-share basis")
	require.Equal(t, int64(-2200), accountBalance(t, f, f.cashAccountID))
}

func TestCashInLieuOfReversedSplitIsRefused(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	_, err := acknowledgedReverseSplit(ctx, f.investmentService, ReverseInvestmentSplitInput{
		OwnerUserID: f.ownerUserID, TransactionID: split.Transaction.ID, Reason: "entered in error"})
	require.NoError(t, err)
	_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
	require.ErrorIs(t, err, ErrCashInLieuSplitNotFound)
}

// T-150: reversing a cash in lieu restores the fraction and frees its split
// for correction again.
func TestReverseCashInLieuRestoresFractionAndFreesSplit(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
	require.NoError(t, err)
	input := ReverseInvestmentSaleInput{OwnerUserID: f.ownerUserID, TransactionID: cil.Transaction.ID, Reason: "paid in shares instead"}
	impact, err := f.investmentService.ReverseCashInLieuReconciliationImpact(ctx, input)
	require.NoError(t, err)
	if impact.GainImpact != nil {
		input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	_, err = f.investmentService.ReverseCashInLieu(ctx, input)
	require.NoError(t, err)
	quantity, _ := openPositionBasis(t, f, f.holdingAccountID)
	requireScaled(t, 45, 1, quantity, "the fraction is held again")
	_, err = f.investmentService.ReverseSale(ctx, input)
	require.ErrorIs(t, err, ErrInvestmentSaleNotFound, "a cash in lieu is not a sale")
	requireInvestmentSelfCheckPasses(t, f)

	_, err = acknowledgedReverseSplit(ctx, f.investmentService, ReverseInvestmentSplitInput{
		OwnerUserID: f.ownerUserID, TransactionID: split.Transaction.ID, Reason: "entered in error"})
	require.NoError(t, err, "with the cash in lieu reversed, the split may be corrected")
}

func TestReplaceCashInLieuKeepsItsSplitAndPreviewMatchesCommit(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
	require.NoError(t, err)
	replacement := cashInLieuInput(f, 0)
	replacement.ProceedsValue, replacement.PaymentOn = 850, "2026-06-08"
	input := ReplaceCashInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: cil.Transaction.ID,
		Reason: "broker paid 8.50", Replacement: replacement}
	impact, err := f.investmentService.ReplaceCashInLieuReconciliationImpact(ctx, input)
	require.NoError(t, err)
	if impact.GainImpact != nil {
		input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	result, err := f.investmentService.ReplaceCashInLieu(ctx, input)
	require.NoError(t, err)
	require.Len(t, result.Replacement.Transaction.JournalEntries, 2)
	assert.Equal(t, "2026-06-08", result.Replacement.Transaction.JournalEntries[1].EntryDate)
	var linked int64
	require.NoError(t, f.database.QueryRow(`SELECT f.settled_operation_id FROM investment_cash_in_lieu_facts f
		JOIN effective_investment_operations o ON o.id = f.operation_id`).Scan(&linked))
	assert.Equal(t, transferOperationID(t, f, split.Transaction.ID), linked, "the successor settles the same split")
	requireInvestmentSelfCheckPasses(t, f)
}
