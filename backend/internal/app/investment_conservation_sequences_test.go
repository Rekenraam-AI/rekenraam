package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #190: named multi-family correction and revision sequences. Each mixes
// families in one book, so revisions of one family land beside revisions of
// another, and ends with the independent conservation oracle as well as the
// ordinary self-checks.

func acknowledgedSell(t *testing.T, f *investmentsTestFixture, input InvestmentTradeInput) InvestmentTradeResult {
	t.Helper()
	ctx := context.Background()
	impact, err := f.investmentService.TradeReconciliationImpact(ctx, InvestmentImpactSell, input)
	require.NoError(t, err)
	if impact.GainImpact != nil {
		input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	result, err := f.investmentService.Sell(ctx, input)
	require.NoError(t, err)
	return result
}

// fifoSellInput names the method a replacement sale must state explicitly.
func fifoSellInput(f *investmentsTestFixture, date string, quantity int64) InvestmentTradeInput {
	input := sellInput(f, date, quantity)
	input.CostBasisMethod = "fifo"
	return input
}

func revisionRows(t *testing.T, f *investmentsTestFixture, table string) int {
	t.Helper()
	var n int
	require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&n))
	return n
}

// A revised short cover in one holding beside a revised internal transfer and
// its destination sale in two others, then the cover's own replacement.
func TestOracleShortCoverRevisionsBesideARevisedTransferAndSale(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	shortHolding := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")

	buy := buyOn(t, f, "2026-01-10", 1, 1000)
	chainTransfer(t, f, f.holdingAccountID, destination, *buy.LotID, "2026-03-01")
	acknowledgedSell(t, f, tradeOn(f, destination, "2026-04-01", 1, 1500))
	_, err := acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, buy, "2026-01-10", 1, 1200))
	require.NoError(t, err)
	require.Equal(t, 1, revisionRows(t, f, "investment_transfer_link_revisions"))

	shortAt := func(date string, quantity, cash int64) InvestmentTradeInput {
		input := shortInput(f, date, quantity, cash)
		input.HoldingAccountID = shortHolding
		return input
	}
	_, err = f.investmentService.ShortSale(ctx, shortAt("2026-03-02", 10, 10000))
	require.NoError(t, err)
	coverInputAt := coverInput(f, "2026-05-01", 4, 3000, "lifo")
	coverInputAt.HoldingAccountID = shortHolding
	cover, err := f.investmentService.ShortCover(ctx, coverInputAt)
	require.NoError(t, err)
	shortSaleAcknowledged(t, f, shortAt("2026-04-01", 5, 7500))
	require.Equal(t, 2, revisionRows(t, f, "investment_disposal_revisions"), "the destination sale and the cover are each revised once")
	// Check while the cover's revision is effective: replacing the cover
	// below retires it, which would hide a reader that ignores it (#188).
	requireConservationOracle(t, f)

	replacement := coverInput(f, "2026-05-01", 3, 2250, "lifo")
	replacement.HoldingAccountID = shortHolding
	input := ReplaceInvestmentSaleInput{OwnerUserID: f.ownerUserID, TransactionID: cover.Transaction.ID,
		Reason: "three were covered", Replacement: replacement}
	impact, err := f.investmentService.ReplaceShortCoverReconciliationImpact(ctx, input)
	require.NoError(t, err)
	if impact.GainImpact != nil {
		input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	}
	_, err = f.investmentService.ReplaceShortCover(ctx, input)
	require.NoError(t, err)
	shortSelfCheckPasses(t, f)
}

// A split entered behind a sale, a backdated buy behind both, then the sale's
// replacement: split, disposal and adjustment-journal revisions together.
func TestOracleSplitBehindASaleWithABackdatedBuyAndSaleReplacement(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-01-02", 10, 10000)
	sale := acknowledgedSell(t, f, sellInput(f, "2026-04-01", 4))
	_, err := acknowledgedSplit(ctx, f.investmentService, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	acknowledgedBuy(t, f, tradeOn(f, f.holdingAccountID, "2026-02-01", 2, 2400))
	require.Positive(t, revisionRows(t, f, "investment_split_revisions"), "the split restates for the backdated lot")
	requireConservationOracle(t, f)

	_, err = acknowledgedReplaceSale(ctx, f.investmentService, ReplaceInvestmentSaleInput{OwnerUserID: f.ownerUserID,
		TransactionID: sale.Transaction.ID, Reason: "six were sold", Replacement: fifoSellInput(f, "2026-04-01", 6)})
	require.NoError(t, err)
	requireSelfCheckPasses(t, f)
}

// A return of capital behind a later sale, a backdated buy that reallocates
// it, then that buy's reversal: capital-return revisions in both directions.
func TestOracleReturnOfCapitalAcrossABackdatedBuyAndItsReversal(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-05-01", 2, 2000)
	acknowledgedSell(t, f, sellInput(f, "2026-07-01", 1))
	input := capitalReturnInput(f, "2026-06-01", 400)
	preview, err := f.investmentService.PreviewCapitalReturn(ctx, input)
	require.NoError(t, err)
	if preview.Impact.GainImpact != nil {
		input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	}
	_, err = f.investmentService.CapitalReturn(ctx, input)
	require.NoError(t, err)

	backdated := acknowledgedBuy(t, f, tradeOn(f, f.holdingAccountID, "2026-04-01", 2, 2000))
	require.Positive(t, capitalReturnRevisionCount(t, f), "the receipt is shared with the backdated lot")
	requireConservationOracle(t, f)

	_, err = acknowledgedReverseBuy(ctx, f.investmentService, ReverseInvestmentBuyInput{OwnerUserID: f.ownerUserID,
		TransactionID: backdated.Transaction.ID, Reason: "entered twice"})
	require.NoError(t, err)
	requireInvestmentSelfCheckPasses(t, f)
}

// A spin-off, a later sale of the parent and an upstream buy correction that
// revises the spin-off link and the sale together, then a sale of the child.
func TestOracleSpinOffWithParentSaleAndUpstreamCorrection(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	childID := seedTestSecurityCommodity(t, f, "CHILD")
	buy := buyOn(t, f, "2026-01-02", 2, 2000)
	_, err := f.investmentService.SpinOff(ctx, spinOffInput(f, childID, "2026-06-01", 1, 1, "25", 2))
	require.NoError(t, err)
	acknowledgedSell(t, f, sellInput(f, "2026-07-01", 1))

	_, err = acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, buy, "2026-01-02", 2, 3000))
	require.NoError(t, err)
	require.Positive(t, revisionRows(t, f, "investment_transfer_link_revisions"), "the spin-off carries the corrected basis")
	require.Positive(t, revisionRows(t, f, "investment_disposal_revisions"), "the parent sale is restated")
	requireConservationOracle(t, f)

	childSale := sellInput(f, "2026-08-01", 1)
	childSale.CommodityID = childID
	acknowledgedSell(t, f, childSale)
	requireSpinOffSelfCheckPasses(t, f)
}

// Unknown inbound basis moved between accounts and partly sold stays
// unknown everywhere until a sourced resolution, which then replays through
// the transfer and the sale.
func TestOracleUnknownInboundThroughTransferSaleAndResolution(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	inbound, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", "2020-03-01"))
	require.NoError(t, err)
	moved := internalTransferFromLot(f, destination, *inbound.LotID, exact.New(1), 0)
	moved.EffectiveOn = "2026-06-01"
	_, err = f.investmentService.InternalTransfer(ctx, moved)
	require.NoError(t, err)
	sold := acknowledgedSell(t, f, tradeOn(f, destination, "2026-07-01", 1, 6000))
	require.Equal(t, db.InvestmentBasisUnknown, sold.Allocations[0].BasisKnowledge)
	requireConservationOracle(t, f)

	resolveAcknowledged(t, f, resolveInput(f, inbound.Transaction.ID, 8000))
	requireInvestmentSelfCheckPasses(t, f)
}
