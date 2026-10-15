package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func alternativeGains(t *testing.T, f *investmentsTestFixture, method, date string, period GainsReportParams) AlternativeGains {
	t.Helper()
	result, err := f.investmentService.AlternativeGainsProjection(context.Background(), method, date, period)
	require.NoError(t, err)
	return result
}

func TestAlternativeGainsMethodsConserveQuantityAndBasisAcrossPartialSales(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"fifo", "lifo", "average_cost"} {
		t.Run(method, func(t *testing.T) {
			f := newInvestmentsTestFixture(t)
			buyOn(t, f, "2026-01-01", 4, 4000)
			buyOn(t, f, "2026-01-02", 4, 12000)
			for _, date := range []string{"2026-02-01", "2026-03-01"} {
				_, err := f.investmentService.Sell(context.Background(), sellInput(f, date, 2))
				require.NoError(t, err)
			}
			result := alternativeGains(t, f, method, "2026-03-02", GainsReportParams{})
			require.Len(t, result.Realized, 2)
			require.Len(t, result.Unrealized, 1)
			quantity, basis := exact.NewScaledInt(), exact.NewScaledInt()
			for _, sale := range result.Realized {
				quantity.AddCoefficient(sale.QuantityValue, sale.QuantityScale)
				basis.AddInt64(-sale.DisposedBasisValue, sale.DisposedBasisScale)
			}
			pos := result.Unrealized[0]
			quantity.AddCoefficient(pos.QuantityValue, pos.QuantityScale)
			basis.AddInt64(pos.RemainingCostBasisValue, pos.RemainingCostBasisScale)
			requireScaled(t, 8, 0, quantity, "acquired = disposed + remaining units")
			requireScaled(t, 16000, 2, basis, "acquired = disposed + remaining basis")
			want := map[string]int64{"fifo": 12000, "lifo": 4000, "average_cost": 8000}[method]
			assertMoneyValue(t, want, 2, pos.RemainingCostBasisValue, pos.RemainingCostBasisScale, "method-specific remainder")
			assert.Equal(t, "alternative_method_v1", result.Profile.Name)
			assert.Equal(t, method, result.Profile.BasisMethod)
			assert.Equal(t, "selected_method", result.Profile.SpecificLotFallback)
		})
	}
}

func TestAlternativeGainsEqualOperationalWhenMethodsMatch(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"fifo", "lifo", "average_cost"} {
		t.Run(method, func(t *testing.T) {
			f := newInvestmentsTestFixture(t)
			setHoldingCostBasisMethod(t, f, method)
			buyOn(t, f, "2026-01-01", 3, 3001)
			buyOn(t, f, "2026-01-02", 3, 6007)
			_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 4))
			require.NoError(t, err)
			result := alternativeGains(t, f, method, "2026-02-02", GainsReportParams{})
			operational, err := f.investmentService.ListRealizedGains(context.Background(), GainsReportParams{})
			require.NoError(t, err)
			require.Len(t, result.Realized, 1)
			require.Len(t, operational, 1)
			a, b := result.Realized[0], operational[0]
			assertMoneyValue(t, b.DisposedBasisValue, b.DisposedBasisScale, a.DisposedBasisValue, a.DisposedBasisScale, "same disposed basis")
			assertMoneyValue(t, b.RealizedGainValue, b.RealizedGainScale, a.RealizedGainValue, a.RealizedGainScale, "same realized gain")
			_, positions := datedValuation(t, f, "2026-02-02")
			assertMoneyValue(t, positions[0].RemainingCostBasisValue, positions[0].RemainingCostBasisScale, result.Unrealized[0].RemainingCostBasisValue, result.Unrealized[0].RemainingCostBasisScale, "same remaining basis")
		})
	}
}

func TestAlternativeGainsReadLeavesLotsElectionsJournalsAndAuditUnchanged(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	first := buyOn(t, f, "2026-01-01", 4, 4000)
	buyOn(t, f, "2026-01-02", 4, 12000)
	input := sellInput(f, "2026-02-01", 2)
	input.CostBasisMethod = "specific_lot"
	input.LotAllocations = []InvestmentLotAllocationInput{{LotID: *first.LotID, QuantityValue: exact.New(2)}}
	_, err := f.investmentService.Sell(context.Background(), input)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	for _, method := range []string{"fifo", "lifo", "average_cost", "fifo"} {
		result := alternativeGains(t, f, method, "2026-02-02", GainsReportParams{From: "2026-02-01", To: "2026-02-01"})
		require.Len(t, result.Realized, 1)
		assert.Equal(t, "2026-02-01", result.Profile.From)
		assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	}
	_, err = f.investmentService.AlternativeGainsProjection(context.Background(), "specific_lot", "", GainsReportParams{})
	require.Error(t, err)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestAlternativeGainsConserveBasisThroughTransfersAndDestinationSales(t *testing.T) {
	t.Parallel()
	for _, pooled := range []bool{false, true} {
		for _, method := range []string{"fifo", "lifo", "average_cost"} {
			t.Run(method+map[bool]string{false: "/source_lots", true: "/pooled_lot"}[pooled], func(t *testing.T) {
				f := newInvestmentsTestFixture(t)
				if pooled {
					setHoldingCostBasisMethod(t, f, "average_cost")
				}
				first := buyOn(t, f, "2026-01-01", 4, 4000)
				buyOn(t, f, "2026-01-02", 4, 12000)
				destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
				input := internalTransferFromLot(f, destination, *first.LotID, exact.New(2), 0)
				input.EffectiveOn = "2026-02-01"
				if pooled {
					input = pooledTransferInput(f, destination, "2026-02-01", exact.New(2), 0)
				}
				_, err := f.investmentService.InternalTransfer(context.Background(), input)
				require.NoError(t, err)
				sale := destinationSale(f, destination, "2026-03-01", "fifo")
				_, err = f.investmentService.Sell(context.Background(), sale)
				require.NoError(t, err)
				before := buyReplacementPreviewSnapshot(t, f.database)
				result := alternativeGains(t, f, method, "2026-03-02", GainsReportParams{})
				require.Len(t, result.Realized, 1)
				require.Len(t, result.Unrealized, 2)
				basis := exact.ScaledIntFromInt64(-result.Realized[0].DisposedBasisValue, result.Realized[0].DisposedBasisScale)
				quantity := exact.ScaledIntFromCoefficient(result.Realized[0].QuantityValue, result.Realized[0].QuantityScale)
				for _, pos := range result.Unrealized {
					basis.AddInt64(pos.RemainingCostBasisValue, pos.RemainingCostBasisScale)
					quantity.AddCoefficient(pos.QuantityValue, pos.QuantityScale)
					if pos.AccountID == f.holdingAccountID {
						want := map[string]int64{"fifo": 14000, "lifo": 10000, "average_cost": 12000}[method]
						assertMoneyValue(t, want, 2, pos.RemainingCostBasisValue, pos.RemainingCostBasisScale, "source retains acquisition less alternative transfer basis")
					} else {
						want := map[string]int64{"fifo": 1000, "lifo": 3000, "average_cost": 2000}[method]
						assertMoneyValue(t, want, 2, pos.RemainingCostBasisValue, pos.RemainingCostBasisScale, "destination retains transferred basis less its disposal")
						assertMoneyValue(t, -want, 2, result.Realized[0].DisposedBasisValue, result.Realized[0].DisposedBasisScale, "destination disposal uses the transferred alternative basis")
					}
				}
				requireScaled(t, 16000, 2, basis, "internal transfer conserves basis across positions")
				requireScaled(t, 8, 0, quantity, "internal transfer conserves quantity across positions")
				assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			})
		}
	}
}

func TestAlternativeGainsPeriodAndValuationAreIndependent(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-01-01", 4, 4000)
	buyOn(t, f, "2026-01-02", 4, 12000)
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-03-01", 2))
	require.NoError(t, err)
	before := alternativeGains(t, f, "lifo", "2026-02-01", GainsReportParams{From: "2026-03-01", To: "2026-03-31"})
	require.Len(t, before.Realized, 1)
	require.Len(t, before.Unrealized, 1)
	assert.Equal(t, "8", before.Unrealized[0].QuantityValue.String())
	after := alternativeGains(t, f, "lifo", "2026-04-01", GainsReportParams{From: "2026-02-01", To: "2026-02-28"})
	assert.Empty(t, after.Realized)
	require.Len(t, after.Unrealized, 1)
	assert.Equal(t, "6", after.Unrealized[0].QuantityValue.String())
}

func TestAlternativeGainsShortMethodsConserveOpeningProceeds(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"fifo", "lifo", "average_cost"} {
		t.Run(method, func(t *testing.T) {
			f := newInvestmentsTestFixture(t)
			shortSaleOn(t, f, "2026-01-01", 4, 4000)
			shortSaleOn(t, f, "2026-01-02", 4, 12000)
			_, err := f.investmentService.ShortCover(context.Background(), shortInput(f, "2026-02-01", 2, 3000))
			require.NoError(t, err)
			result := alternativeGains(t, f, method, "2026-02-02", GainsReportParams{})
			require.Len(t, result.Realized, 1)
			require.Len(t, result.Unrealized, 1)
			sale, pos := result.Realized[0], result.Unrealized[0]
			assert.Equal(t, "short", sale.PositionSide)
			assert.Equal(t, "short", pos.PositionSide)
			basis := exact.ScaledIntFromInt64(sale.DisposedBasisValue, sale.DisposedBasisScale)
			basis.AddInt64(pos.RemainingCostBasisValue, pos.RemainingCostBasisScale)
			requireScaled(t, 16000, 2, basis, "covered + remaining opening proceeds")
			want := map[string]int64{"fifo": -1000, "lifo": 3000, "average_cost": 1000}[method]
			assertMoneyValue(t, want, 2, sale.RealizedGainValue, sale.RealizedGainScale, "opening proceeds less cover cost")
		})
	}
}

func TestAlternativeGainsFailureRollsBackTemporaryProjection(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-01-01", 4, 4000)
	buyOn(t, f, "2026-01-02", 4, 12000)
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 2))
	require.NoError(t, err)
	_, err = f.database.Exec(`CREATE TRIGGER fail_alternative_replay BEFORE INSERT ON investment_lot_events
 WHEN NEW.event_kind = 'disposal' BEGIN SELECT RAISE(ABORT, 'injected replay failure'); END`)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.AlternativeGainsProjection(context.Background(), "average_cost", "2026-02-02", GainsReportParams{})
	require.ErrorContains(t, err, "injected replay failure")
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestAlternativeGainsUseEffectiveCorrectedHistoryAndSplits(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	first := buyOn(t, f, "2026-01-01", 4, 4000)
	buyOn(t, f, "2026-01-02", 4, 12000)
	_, err := f.investmentService.Split(context.Background(), splitInput(f, "2026-02-01", 2, 1))
	require.NoError(t, err)
	_, err = f.investmentService.Sell(context.Background(), sellInput(f, "2026-03-01", 4))
	require.NoError(t, err)
	replacement := tradeOn(f, f.holdingAccountID, "2026-01-01", 4, 8000)
	_, err = acknowledgedReplaceBuy(context.Background(), f.investmentService, ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: first.Transaction.ID, Reason: "correct cost", Replacement: replacement})
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	result := alternativeGains(t, f, "lifo", "2026-03-02", GainsReportParams{})
	require.Len(t, result.Realized, 1)
	require.Len(t, result.Unrealized, 1)
	assertMoneyValue(t, -6000, 2, result.Realized[0].DisposedBasisValue, result.Realized[0].DisposedBasisScale, "four post-split shares under LIFO")
	assertMoneyValue(t, 14000, 2, result.Unrealized[0].RemainingCostBasisValue, result.Unrealized[0].RemainingCostBasisScale, "corrected total less disposal")
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestAlternativeGainsUnknownBasisRemainsUnresolved(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	seedExternalTransferEquity(t, f.database)
	_, err := f.investmentService.ExternalTransferIn(context.Background(), unknownTransferInInput(f, "2026-01-01", ""))
	require.NoError(t, err)
	buyOn(t, f, "2026-01-02", 4, 4000)
	_, err = f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 1))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	result := alternativeGains(t, f, "average_cost", "2026-02-02", GainsReportParams{})
	require.Len(t, result.Realized, 1)
	require.Len(t, result.Unrealized, 1)
	assert.Equal(t, "unknown", result.Realized[0].BasisKnowledge)
	assert.Equal(t, "unknown", result.Unrealized[0].BasisKnowledge)
	assert.Nil(t, result.Unrealized[0].UnrealizedGainValue)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestAlternativeGainsMergerCashKeepsRecordedDateAndZeroQuantity(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	newID := seedTestSecurityCommodity(t, f, "ALTNEW")
	buyOn(t, f, "2026-01-01", 4, 4000)
	_, err := f.investmentService.CashStockMerger(context.Background(), cashStockMergerInput(f, newID, "2026-02-01", "2026-02-05", 1, 1, 2000, "25", 2))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	result := alternativeGains(t, f, "lifo", "2026-02-02", GainsReportParams{From: "2026-02-01", To: "2026-02-01"})
	require.Len(t, result.Realized, 1)
	assert.Equal(t, "2026-02-01", result.Realized[0].DisposalDate)
	assert.Equal(t, exact.New(0), result.Realized[0].QuantityValue)
	assertMoneyValue(t, 1000, 2, result.Realized[0].RealizedGainValue, result.Realized[0].RealizedGainScale, "cash less elected basis")
	result = alternativeGains(t, f, "lifo", "2026-02-02", GainsReportParams{From: "2026-02-05", To: "2026-02-05"})
	assert.Empty(t, result.Realized, "the cash payment date does not redetermine the realization date")
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestAlternativeGainsPreserveStructuralEntitlementsOrRefuseWithoutWrites(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	newID := seedTestSecurityCommodity(t, f, "ALTSTRUCT")
	buyOn(t, f, "2026-01-01", 4, 4000)
	buyOn(t, f, "2026-01-02", 4, 12000)
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 2))
	require.NoError(t, err)
	_, err = f.investmentService.ShareExchange(context.Background(), shareExchangeInput(f, newID, "2026-03-01", 1, 1))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	operational := alternativeGains(t, f, "fifo", "2026-03-02", GainsReportParams{})
	require.Len(t, operational.Unrealized, 1)
	assertMoneyValue(t, 14000, 2, operational.Unrealized[0].RemainingCostBasisValue, operational.Unrealized[0].RemainingCostBasisScale, "effective structural basis")
	_, err = f.investmentService.AlternativeGainsProjection(context.Background(), "lifo", "2026-03-02", GainsReportParams{})
	var dependency AlternativeGainsDependencyError
	require.ErrorAs(t, err, &dependency)
	require.Positive(t, dependency.OperationID)
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}
