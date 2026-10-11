package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #188: a cover revised by replay keeps reducing the units a named short owes.
// The effective lot-event view drops the cover's original events once it has
// a revision, so every reader must add the revision's allocations back. A
// forgotten cover overstates the owed units and silently explains negative
// holdings that should be flagged.

// namedShortUnits folds the dated short events through a date ("" = all).
func namedShortUnits(t *testing.T, f *investmentsTestFixture, through string) string {
	t.Helper()
	events, err := db.NewTransactionRepository(f.database).ShortLotQuantityEventsThrough(context.Background(), BookID, through)
	require.NoError(t, err)
	total := exact.NewScaledInt()
	for _, event := range events {
		total.AddCoefficient(event.QuantityValue, event.QuantityScale)
	}
	total = total.Normalized()
	require.Zero(t, total.Scale(), "these cases owe whole units")
	return total.BigInt().String()
}

// shortSaleAcknowledged commits a short sale, acknowledging the cover results
// a backdated opening restates.
func shortSaleAcknowledged(t *testing.T, f *investmentsTestFixture, input InvestmentTradeInput) {
	t.Helper()
	ctx := context.Background()
	impact, err := f.investmentService.TradeReconciliationImpact(ctx, InvestmentImpactShortSale, input)
	require.NoError(t, err)
	if impact.GainImpact != nil {
		input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	_, err = f.investmentService.ShortSale(ctx, input)
	require.NoError(t, err)
}

func disposalRevisionCount(t *testing.T, f *investmentsTestFixture) int {
	t.Helper()
	var revisions int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_disposal_revisions`).Scan(&revisions))
	return revisions
}

// revisedLIFOCover is the #188 reproduction: ten owed from March, four
// covered in May by LIFO, then five more owed from a backdated April opening
// that the May cover now takes first. 10 + 5 − 4 = 11 are owed.
func revisedLIFOCover(t *testing.T, f *investmentsTestFixture) InvestmentTradeResult {
	t.Helper()
	shortSaleOn(t, f, "2026-03-02", 10, 10000)
	cover, err := f.investmentService.ShortCover(context.Background(), coverInput(f, "2026-05-01", 4, 3000, "lifo"))
	require.NoError(t, err)
	require.Equal(t, "6", namedShortUnits(t, f, ""))
	shortSaleAcknowledged(t, f, shortInput(f, "2026-04-01", 5, 7500))
	require.Equal(t, 1, disposalRevisionCount(t, f), "the May cover is revised, not re-entered")
	return cover
}

func TestNamedShortCountsARevisedCoverOnItsDate(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	revisedLIFOCover(t, f)

	assert.Equal(t, "15", namedShortUnits(t, f, "2026-04-30"), "both openings before the cover")
	assert.Equal(t, "11", namedShortUnits(t, f, "2026-05-01"), "the revised cover on its own date")
	assert.Equal(t, "11", namedShortUnits(t, f, ""))
	worth, err := f.transactionService.NetWorth(context.Background(), NetWorthInput{AsOf: "2026-05-31"})
	require.NoError(t, err)
	assert.Empty(t, worth.UnclassifiedShorts, "eleven owed units explain a −11 holding")
	shortSelfCheckPasses(t, f)
}

// An unexplained unit beside a revised cover stays a warning: in net worth, in
// the self-check, and as a withheld converted net-worth total.
func TestUnexplainedNegativeUnitBesideARevisedCoverIsFlagged(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	revisedLIFOCover(t, f)
	priceTestStockAt(t, f, "2026-05-31", 700)
	f.transactionService.SetPricingRepository(db.NewPricingRepository(f.database))
	series := func() NetWorthSeriesBucket {
		t.Helper()
		result, err := f.transactionService.NetWorthSeries(ctx, NetWorthSeriesInput{
			StartDate: "2026-05-01", EndDate: "2026-05-31", Bucket: "month",
			Reporting: &ReportingCurrencyInput{CommodityID: f.eurCommodityID, MaxStalenessDay: 30}})
		require.NoError(t, err)
		require.Len(t, result.Buckets, 1)
		return result.Buckets[0]
	}
	healthy := series()
	require.Empty(t, healthy.UnclassifiedShorts)
	require.NotNil(t, healthy.Converted, "control: an explained short does not withhold the total")

	// Damage the April opening so the subledger owes 10, against a −11
	// journal holding: exactly one unit is unexplained.
	_, err := f.database.Exec(`DROP TRIGGER investment_lot_events_no_update`)
	require.NoError(t, err)
	_, err = f.database.Exec(`UPDATE investment_lot_events SET quantity_value = '4'
		WHERE event_date = '2026-04-01' AND lot_id IN (SELECT id FROM investment_lots WHERE position_side = 'short')`)
	require.NoError(t, err)
	require.Equal(t, "10", namedShortUnits(t, f, ""))

	unexplained := []UnclassifiedShortPosition{{AccountID: f.holdingAccountID, CommodityID: f.stockCommodityID}}
	worth, err := f.transactionService.NetWorth(ctx, NetWorthInput{AsOf: "2026-05-31"})
	require.NoError(t, err)
	assert.Equal(t, unexplained, worth.UnclassifiedShorts)
	assert.Equal(t, SelfCheckFailed, resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckCommodityPositionSign).Status)
	damaged := series()
	assert.Equal(t, unexplained, damaged.UnclassifiedShorts)
	assert.Nil(t, damaged.Converted, "an unclassified short withholds the converted total")
}

func TestNamedShortFollowsTheLatestOfRepeatedCoverRevisions(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	revisedLIFOCover(t, f)
	shortSaleAcknowledged(t, f, shortInput(f, "2026-04-02", 2, 3000))
	require.Equal(t, 2, disposalRevisionCount(t, f), "the May cover is revised a second time")

	assert.Equal(t, "17", namedShortUnits(t, f, "2026-04-30"))
	assert.Equal(t, "13", namedShortUnits(t, f, ""), "10 + 5 + 2 − 4, counting only the latest revision")
	shortSelfCheckPasses(t, f)
}

func TestNamedShortAfterCorrectingARevisedCover(t *testing.T) {
	t.Parallel()
	t.Run("replacement", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		ctx := context.Background()
		cover := revisedLIFOCover(t, f)
		input := ReplaceInvestmentSaleInput{OwnerUserID: f.ownerUserID, TransactionID: cover.Transaction.ID,
			Reason: "three were covered", Replacement: coverInput(f, "2026-05-01", 3, 2250, "lifo")}
		impact, err := f.investmentService.ReplaceShortCoverReconciliationImpact(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, impact.GainImpact)
		input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
		_, err = f.investmentService.ReplaceShortCover(ctx, input)
		require.NoError(t, err)

		assert.Equal(t, "12", namedShortUnits(t, f, ""), "the replacement counts once; the superseded revision not at all")
		shortSelfCheckPasses(t, f)
	})
	t.Run("reversal", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		cover := revisedLIFOCover(t, f)
		reverseShortCoverAcknowledged(t, f, cover.Transaction.ID, "no such cover")

		assert.Equal(t, "15", namedShortUnits(t, f, ""), "a reversed cover's revision covers nothing")
		shortSelfCheckPasses(t, f)
	})
}

func TestNamedShortAfterCorrectingTheBackdatedOpening(t *testing.T) {
	t.Parallel()
	backdatedOpening := func(t *testing.T, f *investmentsTestFixture) int64 {
		t.Helper()
		revisedLIFOCover(t, f)
		var transactionID int64
		require.NoError(t, f.database.QueryRow(`SELECT source_transaction_id FROM investment_lots
			WHERE position_side = 'short' AND opened_on = '2026-04-01'`).Scan(&transactionID))
		return transactionID
	}
	t.Run("replacement", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		ctx := context.Background()
		input := ReplaceInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: backdatedOpening(t, f),
			Reason: "six were borrowed", Replacement: shortInput(f, "2026-04-01", 6, 9000)}
		impact, err := f.investmentService.ReplaceShortSaleReconciliationImpact(ctx, input)
		require.NoError(t, err)
		if impact.GainImpact != nil {
			input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
		}
		_, err = f.investmentService.ReplaceShortSale(ctx, input)
		require.NoError(t, err)

		assert.Equal(t, "12", namedShortUnits(t, f, ""), "10 + 6 − 4")
		shortSelfCheckPasses(t, f)
	})
	t.Run("reversal", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		ctx := context.Background()
		input := ReverseInvestmentBuyInput{OwnerUserID: f.ownerUserID, TransactionID: backdatedOpening(t, f),
			Reason: "never borrowed"}
		impact, err := f.investmentService.ReverseShortSaleReconciliationImpact(ctx, input)
		require.NoError(t, err)
		if impact.GainImpact != nil {
			input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
		}
		_, err = f.investmentService.ReverseShortSale(ctx, input)
		require.NoError(t, err)

		assert.Equal(t, "6", namedShortUnits(t, f, ""), "the cover replays against the March lot alone")
		shortSelfCheckPasses(t, f)
	})
}

// The side guard reads the same history: a long opening dated inside a short
// whose only later activity is a revised cover is still an overlap.
func TestLongOpeningInsideAShortClosedByARevisedCoverIsRefused(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	shortSaleOn(t, f, "2026-03-02", 10, 10000)
	_, err := f.investmentService.ShortCover(ctx, coverInput(f, "2026-05-01", 10, 7000, "lifo"))
	require.NoError(t, err)
	// A backdated April opening is taken first by the May LIFO cover, and a
	// backdated April cover closes what remains; the May cover is revised.
	shortSaleAcknowledged(t, f, shortInput(f, "2026-04-01", 5, 7500))
	coverApril := coverInput(f, "2026-04-20", 5, 3500, "fifo")
	impact, err := f.investmentService.TradeReconciliationImpact(ctx, InvestmentImpactShortCover, coverApril)
	require.NoError(t, err)
	if impact.GainImpact != nil {
		coverApril.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	_, err = f.investmentService.ShortCover(ctx, coverApril)
	require.NoError(t, err)
	require.Zero(t, shortOpenLots(t, f))
	require.Positive(t, disposalRevisionCount(t, f))

	_, err = f.investmentService.Buy(ctx, tradeOn(f, f.holdingAccountID, "2026-04-25", 1, 1000))
	require.ErrorIs(t, err, ErrInvestmentPositionSideConflict)
	assert.ErrorContains(t, err, "2026-05-01", "the revised May cover is the conflicting activity")
}

// The same holds for every revised family, not only covers: a long holding
// closed by an internal transfer whose link replay revised still had long
// activity on the transfer date, so a short cannot open inside its lifetime.
func TestShortOpeningInsideALongClosedByARevisedTransferIsRefused(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	buy := buyOn(t, f, "2026-01-10", 1, 1000)
	chainTransfer(t, f, f.holdingAccountID, destination, *buy.LotID, "2026-03-01")
	_, err := acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, buy, "2026-01-10", 1, 2000))
	require.NoError(t, err)
	var linkRevisions int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_transfer_link_revisions`).Scan(&linkRevisions))
	require.Equal(t, 1, linkRevisions, "the transfer carries the corrected basis through a revision")

	_, err = f.investmentService.ShortSale(ctx, shortInput(f, "2026-02-01", 1, 1000))
	require.ErrorIs(t, err, ErrInvestmentPositionSideConflict)
	assert.ErrorContains(t, err, "2026-03-01", "the revised transfer out is the conflicting long activity")
}
