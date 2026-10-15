package app

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/exact"
)

func personPooledGains(t *testing.T, f *investmentsTestFixture, date string, period GainsReportParams) AlternativeGains {
	t.Helper()
	result, err := f.investmentService.AlternativeGainsProjection(context.Background(), "person_pooled_average", "  My jurisdiction context  ", date, period)
	require.NoError(t, err)
	assert.Equal(t, "person_pooled_average_v1", result.Profile.Name)
	assert.Equal(t, "person_pooled_average", result.Profile.BasisMethod)
	assert.Equal(t, "owner_security_cost_currency_position_side", result.Profile.PoolScope)
	assert.Equal(t, "My jurisdiction context", result.Profile.JurisdictionNote)
	assert.Equal(t, "internal_pool_neutral", result.Profile.TransferPolicy)
	return result
}

func requirePersonPoolConservation(t *testing.T, result AlternativeGains, quantity, basis int64) {
	t.Helper()
	q, b := exact.NewScaledInt(), exact.NewScaledInt()
	for _, sale := range result.Realized {
		q.AddCoefficient(sale.QuantityValue, sale.QuantityScale)
		sign := int64(-1)
		if sale.PositionSide == "short" {
			sign = 1
		}
		b.AddInt64(sign*sale.DisposedBasisValue, sale.DisposedBasisScale)
	}
	for _, position := range result.Unrealized {
		q.AddCoefficient(position.QuantityValue, position.QuantityScale)
		b.AddInt64(position.RemainingCostBasisValue, position.RemainingCostBasisScale)
	}
	requireScaled(t, quantity, 0, q, "acquisitions = disposed + remaining quantity")
	requireScaled(t, basis, 2, b, "acquisitions = disposed + remaining pooled basis")
}

func TestPersonPooledGainsBuysInTwoAccountsSaleFromOne(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	buyOn(t, f, "2026-01-01", 4, 4000)
	_, err := f.investmentService.Buy(context.Background(), tradeOn(f, second, "2026-01-02", 4, 12000))
	require.NoError(t, err)
	_, err = f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 2))
	require.NoError(t, err)
	priceStockOn(t, f, "2026-02-02", 3000)
	before := buyReplacementPreviewSnapshot(t, f.database)
	for range 2 {
		result := personPooledGains(t, f, "2026-02-02", GainsReportParams{})
		require.Len(t, result.Realized, 1)
		require.Len(t, result.Unrealized, 2)
		assertMoneyValue(t, -4000, 2, result.Realized[0].DisposedBasisValue, result.Realized[0].DisposedBasisScale, "sale uses owner pool, not selling account's 20 basis")
		for _, position := range result.Unrealized {
			wantQuantity, wantBasis := int64(4), int64(8000)
			if position.AccountID == f.holdingAccountID {
				wantQuantity, wantBasis = 2, 4000
			}
			requireScaled(t, wantQuantity, 0, exact.ScaledIntFromCoefficient(position.QuantityValue, position.QuantityScale), "quantity stays in account")
			assertMoneyValue(t, wantBasis, 2, position.RemainingCostBasisValue, position.RemainingCostBasisScale, "remaining basis shares owner pool")
			require.NotNil(t, position.MarketValueValue)
			require.NotNil(t, position.UnrealizedGainValue)
			assertMoneyValue(t, wantBasis/2, 2, *position.UnrealizedGainValue, *position.UnrealizedGainScale, "priced pooled unrealized gain")
		}
		requirePersonPoolConservation(t, result, 8, 16000)
		assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	}
	operational, err := f.investmentService.ListRealizedGains(context.Background(), GainsReportParams{})
	require.NoError(t, err)
	assertMoneyValue(t, -2000, 2, operational[0].DisposedBasisValue, operational[0].DisposedBasisScale, "operational election unchanged")
	// Earlier valuation excludes the sale while an independent later disposal period includes it.
	dated := personPooledGains(t, f, "2026-01-03", GainsReportParams{From: "2026-02-01", To: "2026-02-01"})
	require.Len(t, dated.Realized, 1)
	q := exact.NewScaledInt()
	for _, position := range dated.Unrealized {
		q.AddCoefficient(position.QuantityValue, position.QuantityScale)
	}
	requireScaled(t, 8, 0, q, "dated pool before sale")
	requireInvestmentSelfCheckPasses(t, f)
}

func TestPersonPooledGainsInternalTransfersAreNeutralAcrossAccounts(t *testing.T) {
	t.Parallel()
	for _, lineage := range []string{"source_lots", "pooled_lot"} {
		t.Run(lineage, func(t *testing.T) {
			f := newInvestmentsTestFixture(t)
			if lineage == "pooled_lot" {
				setHoldingCostBasisMethod(t, f, "average_cost")
			}
			first := buyOn(t, f, "2026-01-01", 4, 4001)
			buyOn(t, f, "2026-01-02", 4, 12001)
			destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
			beforeTransfer := personPooledGains(t, f, "2026-01-03", GainsReportParams{})
			input := internalTransferFromLot(f, destination, *first.LotID, exact.New(2), 0)
			input.EffectiveOn = "2026-02-01"
			if lineage == "pooled_lot" {
				input = pooledTransferInput(f, destination, "2026-02-01", exact.New(2), 0)
			}
			_, err := f.investmentService.InternalTransfer(context.Background(), input)
			require.NoError(t, err)
			afterTransfer := personPooledGains(t, f, "2026-02-02", GainsReportParams{})
			assert.Empty(t, beforeTransfer.Realized)
			assert.Empty(t, afterTransfer.Realized)
			requirePersonPoolConservation(t, beforeTransfer, 8, 16002)
			requirePersonPoolConservation(t, afterTransfer, 8, 16002)
			beforeGain, afterGain := exact.NewScaledInt(), exact.NewScaledInt()
			for _, p := range beforeTransfer.Unrealized {
				require.NotNil(t, p.UnrealizedGainValue)
				beforeGain.AddInt64(*p.UnrealizedGainValue, *p.UnrealizedGainScale)
			}
			for _, p := range afterTransfer.Unrealized {
				require.NotNil(t, p.UnrealizedGainValue)
				afterGain.AddInt64(*p.UnrealizedGainValue, *p.UnrealizedGainScale)
			}
			assert.Zero(t, beforeGain.Cmp(afterGain), "internal transfer preserves pooled unrealized gain at the same quote")
			_, err = f.investmentService.Sell(context.Background(), destinationSale(f, destination, "2026-03-01", "fifo"))
			require.NoError(t, err)
			snapshot := buyReplacementPreviewSnapshot(t, f.database)
			result := personPooledGains(t, f, "2026-03-02", GainsReportParams{})
			require.Len(t, result.Realized, 1)
			assertMoneyValue(t, -200025, 4, result.Realized[0].DisposedBasisValue, result.Realized[0].DisposedBasisScale, "destination sale uses original owner's pool rate")
			requirePersonPoolConservation(t, result, 8, 16002)
			assert.Equal(t, snapshot, buyReplacementPreviewSnapshot(t, f.database))
		})
	}
}

func TestPersonPooledGainsShortPoolsAreSeparateFromLong(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	shortSaleOn(t, f, "2026-01-01", 4, 4000)
	input := shortInput(f, "2026-01-02", 4, 12000)
	input.HoldingAccountID = second
	_, err := f.investmentService.ShortSale(context.Background(), input)
	require.NoError(t, err)
	_, err = f.investmentService.ShortCover(context.Background(), shortInput(f, "2026-02-01", 2, 3000))
	require.NoError(t, err)
	result := personPooledGains(t, f, "2026-02-02", GainsReportParams{})
	require.Len(t, result.Realized, 1)
	assertMoneyValue(t, 1000, 2, result.Realized[0].RealizedGainValue, result.Realized[0].RealizedGainScale, "pooled opening proceeds less cover")
	requirePersonPoolConservation(t, result, 8, 16000)
}

func TestPersonPooledGainsUnknownBasisPropagatesAcrossAccounts(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	seedExternalTransferEquity(t, f.database)
	_, err := f.investmentService.ExternalTransferIn(context.Background(), unknownTransferInInput(f, "2026-01-01", ""))
	require.NoError(t, err)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	_, err = f.investmentService.Buy(context.Background(), tradeOn(f, second, "2026-01-02", 4, 4000))
	require.NoError(t, err)
	sale := sellInput(f, "2026-02-01", 1)
	sale.HoldingAccountID = second
	_, err = f.investmentService.Sell(context.Background(), sale)
	require.NoError(t, err)
	snapshot := buyReplacementPreviewSnapshot(t, f.database)
	result := personPooledGains(t, f, "2026-02-02", GainsReportParams{})
	require.Len(t, result.Realized, 1)
	assert.Equal(t, "unknown", result.Realized[0].BasisKnowledge)
	for _, position := range result.Unrealized {
		assert.Equal(t, "unknown", position.BasisKnowledge)
		assert.Nil(t, position.UnrealizedGainValue)
	}
	assert.Equal(t, snapshot, buyReplacementPreviewSnapshot(t, f.database))
}

func TestPersonPooledGainsSequentialPartialSalesConserveRemainder(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	buyOn(t, f, "2026-01-01", 2, 101)
	_, err := f.investmentService.Buy(context.Background(), tradeOn(f, second, "2026-01-02", 1, 100))
	require.NoError(t, err)
	for i, account := range []int64{f.holdingAccountID, second, f.holdingAccountID} {
		input := sellInput(f, []string{"2026-02-01", "2026-02-02", "2026-02-03"}[i], 1)
		input.HoldingAccountID = account
		_, err = f.investmentService.Sell(context.Background(), input)
		require.NoError(t, err)
		result := personPooledGains(t, f, input.TransactionDate, GainsReportParams{})
		requirePersonPoolConservation(t, result, 3, 201)
		if i == 2 {
			assert.Empty(t, result.Unrealized)
		}
	}
}

func TestPersonPooledGainsInvalidNoteAndFailureLeaveNoWrites(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-01-01", 4, 4000)
	_, err := f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 1))
	require.NoError(t, err)
	for _, method := range []string{"fifo", "person_pooled_average"} {
		note := "context"
		if method == "person_pooled_average" {
			note = strings.Repeat("界", 501)
		}
		_, err = f.investmentService.AlternativeGainsProjection(context.Background(), method, note, "2026-02-02", GainsReportParams{})
		var invalid ValidationError
		require.ErrorAs(t, err, &invalid)
	}
	_, err = f.database.Exec(`CREATE TRIGGER fail_person_pool BEFORE INSERT ON investment_lot_events
 WHEN NEW.event_kind = 'disposal' BEGIN SELECT RAISE(ABORT, 'injected pooled failure'); END`)
	require.NoError(t, err)
	snapshot := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.AlternativeGainsProjection(context.Background(), "person_pooled_average", "", "2026-02-02", GainsReportParams{})
	require.ErrorContains(t, err, "injected pooled failure")
	assert.Equal(t, snapshot, buyReplacementPreviewSnapshot(t, f.database))
}

func TestPersonPooledGainsCostCurrenciesAndPositionSidesNeverNet(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	buyOn(t, f, "2026-01-01", 2, 2000)
	usd := seedTestCurrencyCommodity(t, f.database, "USD")
	dollars := tradeOn(f, second, "2026-01-02", 2, 6000)
	dollars.CashCommodityID = usd
	_, err := f.investmentService.Buy(context.Background(), dollars)
	require.NoError(t, err)
	third := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	short := shortInput(f, "2026-01-03", 2, 10000)
	short.HoldingAccountID = third
	_, err = f.investmentService.ShortSale(context.Background(), short)
	require.NoError(t, err)
	result := personPooledGains(t, f, "2026-01-04", GainsReportParams{})
	require.Len(t, result.Unrealized, 3)
	for _, p := range result.Unrealized {
		want := int64(2000)
		if p.CostCommodityID == usd {
			want = 6000
		}
		if p.PositionSide == "short" {
			want = 10000
		}
		assertMoneyValue(t, want, 2, p.RemainingCostBasisValue, p.RemainingCostBasisScale, "separate currency/side pools")
	}
}

func TestPersonPooledGainsCorrectedHistoryAndStructuralEntitlements(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	first := buyOn(t, f, "2026-01-01", 4, 4000)
	buyOn(t, f, "2026-01-02", 4, 12000)
	_, err := f.investmentService.Split(context.Background(), splitInput(f, "2026-02-01", 2, 1))
	require.NoError(t, err)
	_, err = f.investmentService.Sell(context.Background(), sellInput(f, "2026-03-01", 4))
	require.NoError(t, err)
	replacement := tradeOn(f, f.holdingAccountID, "2026-01-01", 4, 8000)
	_, err = acknowledgedReplaceBuy(context.Background(), f.investmentService, ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: first.Transaction.ID, Reason: "correct pooled source", Replacement: replacement})
	require.NoError(t, err)
	result := personPooledGains(t, f, "2026-03-02", GainsReportParams{})
	requirePersonPoolConservation(t, result, 16, 20000)
	assertMoneyValue(t, -5000, 2, result.Realized[0].DisposedBasisValue, result.Realized[0].DisposedBasisScale, "corrected post-split average")
	newID := seedTestSecurityCommodity(t, f, "PERSONNEW")
	_, err = f.investmentService.ShareExchange(context.Background(), shareExchangeInput(f, newID, "2026-04-01", 1, 1))
	require.NoError(t, err)
	snapshot := buyReplacementPreviewSnapshot(t, f.database)
	result = personPooledGains(t, f, "2026-04-02", GainsReportParams{})
	require.Len(t, result.Unrealized, 1)
	assert.Equal(t, newID, result.Unrealized[0].CommodityID)
	requirePersonPoolConservation(t, result, 16, 20000)
	assert.Equal(t, snapshot, buyReplacementPreviewSnapshot(t, f.database))
}

func TestPersonPooledGainsExternalOutboundRemovesPoolBasisWithoutRealization(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	seedExternalTransferEquity(t, f.database)
	first := buyOn(t, f, "2026-01-01", 4, 4000)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	_, err := f.investmentService.Buy(context.Background(), tradeOn(f, second, "2026-01-02", 4, 12000))
	require.NoError(t, err)
	_, err = f.investmentService.ExternalTransferOut(context.Background(), transferOutOfLot(f, "2026-02-01", *first.LotID, 2))
	require.NoError(t, err)
	snapshot := buyReplacementPreviewSnapshot(t, f.database)
	result := personPooledGains(t, f, "2026-02-02", GainsReportParams{})
	assert.Empty(t, result.Realized)
	requirePersonPoolConservation(t, result, 6, 12000) // owner pool loses 2 × 20
	assert.Equal(t, snapshot, buyReplacementPreviewSnapshot(t, f.database))
}

func TestPersonPooledGainsUnknownPoolExhaustionStartsFreshKnownPool(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	seedExternalTransferEquity(t, f.database)
	unknown := unknownTransferInInput(f, "2026-01-01", "")
	unknown.QuantityValue, unknown.QuantityScale = exact.New(2), 0
	_, err := f.investmentService.ExternalTransferIn(context.Background(), unknown)
	require.NoError(t, err)
	second := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	_, err = f.investmentService.Buy(context.Background(), tradeOn(f, second, "2026-01-02", 2, 2000))
	require.NoError(t, err)
	_, err = f.investmentService.Sell(context.Background(), sellInput(f, "2026-02-01", 2))
	require.NoError(t, err)
	sale := sellInput(f, "2026-02-02", 2)
	sale.HoldingAccountID = second
	_, err = f.investmentService.Sell(context.Background(), sale)
	require.NoError(t, err)
	buyOn(t, f, "2026-03-01", 1, 3000)
	result := personPooledGains(t, f, "2026-03-02", GainsReportParams{})
	require.Len(t, result.Realized, 2)
	for _, sale := range result.Realized {
		assert.Equal(t, "unknown", sale.BasisKnowledge)
	}
	require.Len(t, result.Unrealized, 1)
	assert.Equal(t, "known", result.Unrealized[0].BasisKnowledge)
	assertMoneyValue(t, 3000, 2, result.Unrealized[0].RemainingCostBasisValue, result.Unrealized[0].RemainingCostBasisScale, "new pool after exhaustion")
}
