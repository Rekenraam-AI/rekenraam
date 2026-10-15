package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

func requireCapitalReturnAmount(t *testing.T, value int64, scale int, got *GainsFXAmount) {
	t.Helper()
	require.NotNil(t, got)
	requireScaled(t, value, scale, coefScaled(got.Value, got.Scale), "capital-return disclosure")
}

func TestGainsStatementCapitalReturnExcessIsSeparateFromLimitedTotal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                       string
		receipt, reduction, excess int64
	}{
		{"no_excess", 500, 500, 0}, {"excess", 1000, 700, 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newInvestmentsTestFixture(t)
			buyOn(t, f, "2026-05-01", 1, 700)
			priceStockOn(t, f, "2026-05-31", 700)
			priceStockOn(t, f, "2026-06-30", 700)
			input := capitalReturnInput(f, "2026-06-01", tc.receipt)
			input.PaymentOn = "2026-06-10"
			_, err := f.investmentService.CapitalReturn(context.Background(), input)
			require.NoError(t, err)
			before := buyReplacementPreviewSnapshot(t, f.database)
			statement := gainsStatement(t, f, "2026-06-01", "2026-06-30")
			row := onlyStatementRow(t, f, statement)
			require.Len(t, statement.CapitalReturns, 1)
			requireCapitalReturnAmount(t, tc.receipt, 2, row.CapitalReturn.Receipt)
			requireCapitalReturnAmount(t, tc.reduction, 2, row.CapitalReturn.Reduction)
			requireCapitalReturnAmount(t, tc.excess, 2, row.CapitalReturn.Excess)
			assert.Equal(t, 1, row.CapitalReturn.ReceiptCount)
			assert.Zero(t, row.CapitalReturn.UnresolvedCount)
			assertStatementAmount(t, tc.reduction, 2, row.Total, "basis reduction enters unrealized change once; excess excluded")
			assertStatementAmount(t, 0, 0, row.Realized, "not a disposal")
			assertStatementAmount(t, 0, 0, &row.Income, "not dividend income")
			assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "no read-side writes")
		})
	}
}

func TestGainsStatementCapitalReturnPaymentCohortDiffersFromBasisMovementPeriod(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-05-01", 1, 700)
	priceStockOn(t, f, "2026-05-01", 700)
	priceStockOn(t, f, "2026-05-31", 700)
	priceStockOn(t, f, "2026-06-30", 700)
	input := capitalReturnInput(f, "2026-05-15", 1000)
	input.PaymentOn = "2026-06-10"
	_, err := f.investmentService.CapitalReturn(context.Background(), input)
	require.NoError(t, err)
	may := gainsStatement(t, f, "2026-05-02", "2026-05-31")
	row := onlyStatementRow(t, f, may)
	assert.Zero(t, row.CapitalReturn.ReceiptCount)
	requireCapitalReturnAmount(t, 0, 0, row.CapitalReturn.Excess)
	assertStatementAmount(t, 700, 2, row.Total, "basis changes on effective date")
	june := gainsStatement(t, f, "2026-06-01", "2026-06-30")
	row = onlyStatementRow(t, f, june)
	requireCapitalReturnAmount(t, 1000, 2, row.CapitalReturn.Receipt)
	requireCapitalReturnAmount(t, 700, 2, row.CapitalReturn.Reduction)
	requireCapitalReturnAmount(t, 300, 2, row.CapitalReturn.Excess)
	assertStatementAmount(t, 0, 0, row.Total, "receipt cohort does not shift the basis movement")
	require.Len(t, june.CapitalReturns, 1)
	assert.Equal(t, input.EffectiveOn, june.CapitalReturns[0].EffectiveOn)
	assert.Equal(t, input.PaymentOn, june.CapitalReturns[0].PaymentOn)
	for _, period := range [][2]string{{"2026-06-01", "2026-06-09"}, {"2026-06-11", "2026-06-30"}} {
		assert.Empty(t, gainsStatement(t, f, period[0], period[1]).CapitalReturns)
	}
	assert.Len(t, gainsStatement(t, f, input.PaymentOn, input.PaymentOn).CapitalReturns, 1, "inclusive payment boundary")
}

func TestGainsStatementCapitalReturnUnknownWithholdsWholeReductionAndExcess(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	_, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-01-01", ""))
	require.NoError(t, err)
	buyOn(t, f, "2026-02-01", 2, 600)
	_, err = f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-03-01", 2000))
	require.NoError(t, err)
	statement := gainsStatement(t, f, "2026-03-01", "2026-03-31")
	row := onlyStatementRow(t, f, statement)
	requireCapitalReturnAmount(t, 2000, 2, row.CapitalReturn.Receipt)
	assert.Nil(t, row.CapitalReturn.Reduction)
	assert.Nil(t, row.CapitalReturn.Excess)
	assert.Equal(t, 1, row.CapitalReturn.UnresolvedCount)
	assert.Equal(t, 1, row.CapitalReturn.UnknownLots)
	require.Len(t, statement.CapitalReturns, 1)
	assert.True(t, statement.CapitalReturns[0].HasExcess, "independent known excess remains inspectable, not a headline total")
	require.Len(t, statement.CapitalReturns[0].Effects, 2)
	assert.Equal(t, db.InvestmentBasisUnknown, statement.CapitalReturns[0].Effects[0].BasisKnowledge)
}

func TestGainsStatementCapitalReturnUsesEffectiveRevisionsCorrectionAndReversal(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buy := buyOn(t, f, "2026-05-01", 1, 700)
	receipt, err := f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
	require.NoError(t, err)
	_, err = acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, buy, "2026-05-01", 1, 500))
	require.NoError(t, err)
	statement := gainsStatement(t, f, "2026-06-01", "2026-06-30")
	requireCapitalReturnAmount(t, 500, 2, statement.Rows[0].CapitalReturn.Excess)
	assert.Equal(t, 2, statement.CapitalReturns[0].RevisionSeq)
	corrected, err := f.investmentService.ReplaceCapitalReturn(ctx, ReplaceInvestmentCapitalReturnInput{
		OwnerUserID: f.ownerUserID, TransactionID: receipt.Transaction.ID, Reason: "correct receipt", Replacement: capitalReturnInput(f, "2026-06-01", 1500),
	})
	require.NoError(t, err)
	statement = gainsStatement(t, f, "2026-06-01", "2026-06-30")
	require.Len(t, statement.CapitalReturns, 1)
	requireCapitalReturnAmount(t, 1000, 2, statement.Rows[0].CapitalReturn.Excess)
	assert.Equal(t, corrected.Replacement.Transaction.ID, statement.CapitalReturns[0].TransactionID)
	_, err = f.investmentService.ReverseCapitalReturn(ctx, ReverseInvestmentCapitalReturnInput{OwnerUserID: f.ownerUserID, TransactionID: corrected.Replacement.Transaction.ID, Reason: "withdrawn"})
	require.NoError(t, err)
	statement = gainsStatement(t, f, "2026-06-01", "2026-06-30")
	assert.Empty(t, statement.CapitalReturns)
	assert.Zero(t, statement.Rows[0].CapitalReturn.ReceiptCount)
	requireCapitalReturnAmount(t, 0, 0, statement.Rows[0].CapitalReturn.Excess)
}

func TestGainsFXStatementCapitalReturnDatesCoverageAndUnknown(t *testing.T) {
	t.Parallel()
	for _, method := range []string{GainsFXValuationDateAll, GainsFXHistoricBasis} {
		for _, coverage := range []string{"complete", "effective_before_period", "missing_payment_rate", "unknown_basis"} {
			t.Run(method+"/"+coverage, func(t *testing.T) {
				f := newInvestmentsTestFixture(t)
				ctx := context.Background()
				usd := gainsFXCurrency(t, f)
				if coverage == "unknown_basis" {
					seedExternalTransferEquity(t, f.database)
					_, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
					require.NoError(t, err)
				} else {
					buyOn(t, f, "2026-05-01", 1, 700)
				}
				priceStockOn(t, f, "2026-05-31", 700)
				priceStockOn(t, f, "2026-06-30", 700)
				input := capitalReturnInput(f, "2026-06-01", 1000)
				if coverage == "effective_before_period" {
					input.EffectiveOn = "2026-05-15"
				}
				input.PaymentOn = "2026-06-10"
				_, err := f.investmentService.CapitalReturn(ctx, input)
				require.NoError(t, err)
				for _, rate := range []struct {
					date  string
					value int64
				}{{"2026-05-01", 100}, {"2026-05-15", 110}, {"2026-05-31", 100}, {"2026-06-01", 110}, {"2026-06-30", 130}} {
					gainsFXRate(t, f, usd, rate.date, rate.value)
				}
				if coverage != "missing_payment_rate" {
					gainsFXRate(t, f, usd, "2026-06-10", 120)
				}
				before := buyReplacementPreviewSnapshot(t, f.database)
				got, err := f.investmentService.ReadGainsStatement(ctx, "2026-06-01", "2026-06-30", &GainsFXInput{CommodityID: usd, Method: method})
				require.NoError(t, err)
				fx := got.Reporting
				require.NotNil(t, fx)
				if coverage == "unknown_basis" {
					require.NotNil(t, fx.CapitalReturnReceipt.Amount)
					assert.Nil(t, fx.CapitalReturnReduction.Amount)
					assert.Nil(t, fx.CapitalReturnExcess.Amount)
					assert.Equal(t, "unresolved_basis", fx.CapitalReturnExcess.Components[0].UnavailableReason)
				} else if coverage == "missing_payment_rate" && method == GainsFXHistoricBasis {
					assert.Nil(t, fx.CapitalReturnReceipt.Amount)
					assert.Nil(t, fx.CapitalReturnExcess.Amount)
					assertFXAmount(t, 770, 2, fx.CapitalReturnReduction)
					require.NotNil(t, fx.Total.Amount, "separate disclosure gap does not erase the limited-scope total")
				} else if method == GainsFXHistoricBasis {
					assertFXAmount(t, 1200, 2, fx.CapitalReturnReceipt)
					assertFXAmount(t, 770, 2, fx.CapitalReturnReduction)
					assertFXAmount(t, 360, 2, fx.CapitalReturnExcess)
					assert.Equal(t, input.EffectiveOn, fx.CapitalReturnReduction.Components[0].RequestedDate)
					assert.Equal(t, input.PaymentOn, fx.CapitalReturnExcess.Components[0].RequestedDate)
				} else {
					assertFXAmount(t, 1300, 2, fx.CapitalReturnReceipt)
					assertFXAmount(t, 910, 2, fx.CapitalReturnReduction)
					assertFXAmount(t, 390, 2, fx.CapitalReturnExcess)
					assert.Equal(t, "2026-06-30", fx.CapitalReturnExcess.Components[0].RequestedDate)
				}
				assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			})
		}
	}
}

func TestCapitalReturnStatementAmountKeepsWideExactCoefficients(t *testing.T) {
	t.Parallel()
	sum := exact.NewScaledInt()
	value, err := exact.Parse("99999999999999999999999999999999999999")
	require.NoError(t, err)
	sum.AddCoefficient(value, 12)
	amount, err := capitalReturnStatementAmount(sum)
	require.NoError(t, err)
	assert.Equal(t, value, amount.Value)
}

func TestLiveCapitalReturnStatementAndFXShareCapturedEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-05-01", 1, 700)
	input := capitalReturnInput(f, "2026-06-01", 1000)
	receipt, err := f.investmentService.CapitalReturn(ctx, input)
	require.NoError(t, err)
	var after map[string][][]any
	f.investmentService.SetGainsReadStageForTest(func(stage string) {
		if stage != "closing" {
			return
		}
		_, err := f.investmentService.ReverseCapitalReturn(ctx, ReverseInvestmentCapitalReturnInput{OwnerUserID: f.ownerUserID, TransactionID: receipt.Transaction.ID, Reason: "withdraw during composition"})
		require.NoError(t, err)
		after = buyReplacementPreviewSnapshot(t, f.database)
	})
	got, err := f.investmentService.ReadGainsStatement(ctx, "2026-06-01", "2026-06-30", &GainsFXInput{CommodityID: f.eurCommodityID, Method: GainsFXHistoricBasis})
	require.NoError(t, err)
	require.NotNil(t, after)
	require.Len(t, got.Statement.CapitalReturns, 1)
	requireCapitalReturnAmount(t, 300, 2, got.Statement.Rows[0].CapitalReturn.Excess)
	assertFXAmount(t, 300, 2, got.Reporting.CapitalReturnExcess)
	assert.Equal(t, got.Statement.CapitalReturns[0].OperationID, got.Reporting.CapitalReturnExcess.Components[0].OperationID)
	assert.Equal(t, after, buyReplacementPreviewSnapshot(t, f.database))
}

func TestGainsStatementCapitalReturnPaymentOnlyCurrencyStillHasARow(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-05-01", 1, 700)
	input := capitalReturnInput(f, "2026-05-05", 1000)
	input.PaymentOn = "2026-06-10"
	_, err := f.investmentService.CapitalReturn(ctx, input)
	require.NoError(t, err)
	_, err = f.investmentService.Sell(ctx, sellInput(f, "2026-05-20", 1))
	require.NoError(t, err)
	row := onlyStatementRow(t, f, gainsStatement(t, f, "2026-06-01", "2026-06-30"))
	assert.Zero(t, row.Opening.PositionCount)
	assert.Zero(t, row.Closing.PositionCount)
	assert.Zero(t, row.DisposalCount)
	assertStatementAmount(t, 0, 0, row.Total, "limited-scope zero does not erase payment")
	requireCapitalReturnAmount(t, 1000, 2, row.CapitalReturn.Receipt)
	requireCapitalReturnAmount(t, 300, 2, row.CapitalReturn.Excess)
}

func TestCapitalReturnStatementAmountNormalizesOnlyExactTrailingZerosAndRefusesOverflow(t *testing.T) {
	t.Parallel()
	max := exact.MustParse("99999999999999999999999999999999999999")
	sum := exact.ScaledIntFromCoefficient(max, 0)
	sum.Align(12)
	amount, err := capitalReturnStatementAmount(sum)
	require.NoError(t, err)
	assert.Equal(t, max, amount.Value)
	assert.Zero(t, amount.Scale)
	sum.AddInt64(1, 0)
	_, err = capitalReturnStatementAmount(sum)
	require.ErrorIs(t, err, ErrGainsStatementPrecision)
}
