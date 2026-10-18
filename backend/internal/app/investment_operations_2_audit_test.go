package app

import (
	"context"
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #273: literal, hand-derived figures across a pooled transfer, split,
// spin-off, cash-and-stock merger and sale, followed by an upstream price
// correction and a lineage-changing backdated buy. Checking after each
// command keeps a later correction from hiding an intermediate bad reader.
func TestInvestmentAuditTransferCorporateActionChain(t *testing.T) {
	t.Parallel()
	for _, lineage := range []string{db.InternalTransferPooledLot, db.InternalTransferSourceLots} {
		t.Run(lineage, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := newInvestmentsTestFixture(t)
			setHoldingCostBasisMethod(t, f, "average_cost")
			destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
			child := seedTestSecurityCommodity(t, f, "AUDITCHILD")
			merged := seedTestSecurityCommodity(t, f, "AUDITMERGED")
			check := func(sourceUnits, sourceBasis, parentUnits, parentBasis, childUnits, childBasis, mergedUnits, mergedBasis, cash, gain int64, decisions int) {
				t.Helper()
				for _, expected := range []struct{ account, commodity, units, basis int64 }{
					{f.holdingAccountID, f.stockCommodityID, sourceUnits, sourceBasis},
					{destination, f.stockCommodityID, parentUnits, parentBasis},
					{destination, child, childUnits, childBasis},
					{destination, merged, mergedUnits, mergedBasis},
				} {
					lots, err := f.investmentService.ListLots(ctx, expected.account, expected.commodity)
					require.NoError(t, err)
					units, basis := new(big.Rat), new(big.Rat)
					for _, lot := range lots {
						require.Equal(t, db.InvestmentBasisKnown, lot.BasisKnowledge)
						units.Add(units, financialRat(lot.RemainingQuantityValue.String(), lot.RemainingQuantityScale))
						basis.Add(basis, financialRat(strconv.FormatInt(lot.RemainingCostBasisValue, 10), lot.RemainingCostBasisScale))
					}
					require.Zerof(t, units.Cmp(big.NewRat(expected.units, 1)), "units in %d/%d", expected.account, expected.commodity)
					require.Zerof(t, basis.Cmp(big.NewRat(expected.basis, 100)), "basis in %d/%d", expected.account, expected.commodity)
				}
				require.Equal(t, cash, accountBalance(t, f, f.cashAccountID), "cash in cents")
				gains, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
				require.NoError(t, err)
				require.Len(t, gains, decisions)
				total := new(big.Rat)
				for _, entry := range gains {
					require.Equal(t, db.InvestmentBasisKnown, entry.BasisKnowledge)
					total.Add(total, financialRat(strconv.FormatInt(entry.RealizedGainValue, 10), entry.RealizedGainScale))
				}
				require.Zero(t, total.Cmp(big.NewRat(gain, 100)), "realized gain")
				requireEffectiveMovementsConsistent(t, f)
				requireConservationOracle(t, f)
				run := mustRunInvestmentSelfCheck(t, f)
				require.Equal(t, SelfCheckPassed, run.Status, "%+v", run.Results)
			}
			buyOn(t, f, "2026-01-10", 10, 10000)
			check(10, 10000, 0, 0, 0, 0, 0, 0, -10000, 0, 0)
			second := buyOn(t, f, "2026-02-10", 10, 30000)
			check(20, 40000, 0, 0, 0, 0, 0, 0, -40000, 0, 0)
			move := pooledTransferInput(f, destination, "2026-03-01", exact.New(8), 0)
			move.DestinationLineage = lineage
			transfer, err := f.investmentService.InternalTransfer(ctx, move)
			require.NoError(t, err)
			check(12, 24000, 8, 16000, 0, 0, 0, 0, -40000, 0, 0)
			split := splitInput(f, "2026-04-01", 2, 1)
			split.HoldingAccountID = destination
			_, err = f.investmentService.Split(ctx, split)
			require.NoError(t, err)
			check(12, 24000, 16, 16000, 0, 0, 0, 0, -40000, 0, 0)
			spin := spinOffInput(f, child, "2026-05-01", 1, 2, "25", 2)
			spin.HoldingAccountID = destination
			_, err = f.investmentService.SpinOff(ctx, spin)
			require.NoError(t, err)
			check(12, 24000, 16, 12000, 8, 4000, 0, 0, -40000, 0, 0)
			merger := cashStockMergerInput(f, merged, "2026-06-01", "2026-06-05", 1, 2, 4000, "5", 1)
			merger.HoldingAccountID = destination
			_, err = f.investmentService.CashStockMerger(ctx, merger)
			require.NoError(t, err)
			check(12, 24000, 0, 0, 8, 4000, 8, 6000, -36000, -2000, 1)
			sale := tradeOn(f, destination, "2026-07-01", 4, 8000)
			sale.CommodityID = merged
			_, err = f.investmentService.Sell(ctx, sale)
			require.NoError(t, err)
			check(12, 24000, 0, 0, 8, 4000, 4, 3000, -28000, 3000, 2)

			// Total acquisition basis becomes 600.00. Eight of twenty units
			// carry 240.00: 60.00 to the child, 90.00 realized by the merger,
			// 90.00 to its new stock, of which the sale consumes 45.00.
			_, err = acknowledgedReplaceBuy(ctx, f.investmentService,
				replaceBuyPrice(f, second, "2026-02-10", 10, 50000))
			require.NoError(t, err)
			check(12, 36000, 0, 0, 8, 6000, 4, 4500, -48000, -1500, 2)

			// Ten earlier units costing 600.00 make thirty at 1,200.00.
			// Eight now carry 320.00, leaving 880.00 at source, 80.00 in
			// the child, 60.00 in unsold new stock, and 180.00 disposed.
			backdated := tradeOn(f, f.holdingAccountID, "2026-01-01", 10, 60000)
			before := buyReplacementPreviewSnapshot(t, f.database)
			impact, err := f.investmentService.TradeReconciliationImpact(ctx, InvestmentImpactBuy, backdated)
			if lineage == db.InternalTransferSourceLots {
				require.ErrorIs(t, err, ErrInvestmentBuyDependency)
				_, err = f.investmentService.Buy(ctx, backdated)
				var dependency InvestmentBuyDependencyError
				require.ErrorAs(t, err, &dependency)
				require.Equal(t, transferOperationID(t, f, transfer.Transaction.ID), dependency.OperationID)
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				check(12, 36000, 0, 0, 8, 6000, 4, 4500, -48000, -1500, 2)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, impact.GainImpact)
			require.Len(t, impact.GainImpact.Changes, 2)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "preview rolls back every position")
			backdated.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
			_, err = f.investmentService.Buy(ctx, backdated)
			require.NoError(t, err)
			check(22, 88000, 0, 0, 8, 8000, 4, 6000, -108000, -6000, 2)
		})
	}
}
