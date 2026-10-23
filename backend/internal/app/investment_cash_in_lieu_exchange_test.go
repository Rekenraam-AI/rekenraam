package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #181: cash in lieu of the fraction a share exchange or spin-off leaves. 3
// shares bought for 30.00 exchange 3-for-2 into 4.5 new units; the issuer pays
// 8.00 EUR for the 0.5 fraction, which is disposed in the new instrument with
// the ordinary sale legs.

// openCommodityPosition folds a holding's open lots of one instrument exactly.
func openCommodityPosition(t *testing.T, f *investmentsTestFixture, accountID, commodityID int64) (*exact.ScaledInt, *exact.ScaledInt) {
	t.Helper()
	rows, err := f.database.Query(`SELECT remaining_quantity_value, remaining_quantity_scale,
		remaining_cost_basis_value, remaining_cost_basis_scale FROM current_investment_lots
		WHERE account_id = ? AND commodity_id = ? AND status = 'open'`, accountID, commodityID)
	require.NoError(t, err)
	defer rows.Close()
	quantity, basis := exact.NewScaledInt(), exact.NewScaledInt()
	for rows.Next() {
		var qty, cost exact.Coefficient
		var qtyScale, costScale int
		require.NoError(t, rows.Scan(&qty, &qtyScale, &cost, &costScale))
		quantity.AddCoefficient(qty, qtyScale)
		basis.AddCoefficient(cost, costScale)
	}
	require.NoError(t, rows.Err())
	return quantity, basis
}

func exchangeCashInLieuSetup(t *testing.T) (*investmentsTestFixture, int64, ShareExchangeResult) {
	t.Helper()
	f := newInvestmentsTestFixture(t)
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-05-01", 3, 3000)
	exchange, err := f.investmentService.ShareExchange(context.Background(), shareExchangeInput(f, newID, "2026-06-01", 3, 2))
	require.NoError(t, err)
	return f, newID, exchange
}

func TestCashInLieuOfExchangeFractionDisposesNewUnitsWithSaleLegs(t *testing.T) {
	t.Parallel()
	f, newID, exchange := exchangeCashInLieuSetup(t)
	ctx := context.Background()
	input := cashInLieuInput(f, exchange.Transaction.ID)
	preview, _, err := f.investmentService.PreviewCashInLieu(ctx, input)
	require.NoError(t, err)
	result, err := f.investmentService.CashInLieu(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, preview.Allocations, result.Allocations, "preview takes what the commit takes")

	// The ordinary sale legs: the new instrument out of its holding on the
	// disposal date, the cash in on the payment date.
	require.Len(t, result.Transaction.JournalEntries, 2)
	security, cash := result.Transaction.JournalEntries[0], result.Transaction.JournalEntries[1]
	assert.Equal(t, "2026-06-01", security.EntryDate)
	assert.Equal(t, "2026-06-05", cash.EntryDate)
	for _, posting := range security.Postings {
		assert.Equal(t, newID, posting.CommodityID, "the fraction is the new instrument")
		if posting.AccountID == f.holdingAccountID {
			assert.Equal(t, "-5", posting.QuantityValue.String())
			assert.Equal(t, 1, posting.QuantityScale)
		}
	}
	var linked int64
	require.NoError(t, f.database.QueryRow(`SELECT settled_operation_id FROM investment_cash_in_lieu_facts`).Scan(&linked))
	assert.Equal(t, transferOperationID(t, f, exchange.Transaction.ID), linked)

	quantity, remaining := openCommodityPosition(t, f, f.holdingAccountID, newID)
	requireScaled(t, 4, 0, quantity, "4.5 new units less the 0.5 fraction")
	disposed := saleEffectiveBasis(t, f, result.Transaction.ID)
	disposed.AddScaled(remaining)
	requireScaled(t, 3000, 2, disposed, "the fraction's basis plus the rest conserve the carried 30.00")
	var decided int64
	require.NoError(t, f.database.QueryRow(`SELECT commodity_id FROM investment_disposal_decisions
		WHERE transaction_id = ?`, result.Transaction.ID).Scan(&decided))
	assert.Equal(t, newID, decided)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestCashInLieuOfSpinOffFractionDisposesDistributedUnits(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "SPINCO")
	buyOn(t, f, "2026-05-01", 3, 3000)
	// 1 new unit per 2 parent units: 1.5 new units at 20 % of the basis.
	spinOff, err := f.investmentService.SpinOff(ctx, spinOffInput(f, newID, "2026-06-01", 1, 2, "2", 1))
	require.NoError(t, err)
	_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, spinOff.Transaction.ID))
	require.NoError(t, err)

	quantity, _ := openCommodityPosition(t, f, f.holdingAccountID, newID)
	requireScaled(t, 1, 0, quantity, "1.5 distributed units less the 0.5 fraction")
	parent, _ := openCommodityPosition(t, f, f.holdingAccountID, f.stockCommodityID)
	requireScaled(t, 3, 0, parent, "the parent keeps its units")
	requireInvestmentSelfCheckPasses(t, f)
}

// The missing destination units still prevent reversal; ratio repair uses replay.
func TestExchangeSettledByCashInLieuReplaysReplacementAndRefusesImpossibleReversal(t *testing.T) {
	t.Parallel()
	f, newID, exchange := exchangeCashInLieuSetup(t)
	ctx := context.Background()
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, exchange.Transaction.ID))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReverseShareExchange(ctx, reverseShareExchangeInput(f, exchange.Transaction.ID))
	var dependency InvestmentTransferDependencyError
	require.ErrorAs(t, err, &dependency)
	require.Equal(t, transferOperationID(t, f, cil.Transaction.ID), dependency.OperationID)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input := replaceShareExchangeInput(f, exchange.Transaction.ID, newID, "2026-06-01", 2, 1)
	preview, err := f.investmentService.PreviewShareExchangeReplacement(ctx, input)
	require.NoError(t, err)
	input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	_, err = f.investmentService.ReplaceShareExchange(ctx, input)
	require.NoError(t, err)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestCashInLieuRefusesSettlingAReversedExchange(t *testing.T) {
	t.Parallel()
	f, _, exchange := exchangeCashInLieuSetup(t)
	ctx := context.Background()
	_, err := f.investmentService.ReverseShareExchange(ctx, ReverseShareExchangeInput{OwnerUserID: f.ownerUserID,
		TransactionID: exchange.Transaction.ID, Reason: "wrong ratio"})
	require.NoError(t, err)
	_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, exchange.Transaction.ID))
	require.ErrorIs(t, err, ErrCashInLieuSplitNotFound)
}

// A transaction that is not a split, exchange or spin-off settles nothing.
func TestCashInLieuRefusesAnOrdinaryBuy(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buy := buyOn(t, f, "2026-05-01", 3, 3000)
	_, err := f.investmentService.CashInLieu(context.Background(), cashInLieuInput(f, buy.Transaction.ID))
	require.ErrorIs(t, err, ErrCashInLieuSplitNotFound)
	_, err = f.investmentService.repository.CashInLieuSettledOperation(context.Background(), BookID, buy.Transaction.ID)
	require.ErrorIs(t, err, db.ErrNotFound)
}
