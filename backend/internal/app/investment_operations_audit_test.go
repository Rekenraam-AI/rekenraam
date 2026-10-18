package app

import (
	"context"
	"math/big"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #272: an independent command-by-command register. Opening 10 units for
// 100.00 and 10 for 130.00 gives 20 units and 230.00 basis/opening proceeds.
// Seven units consume 70.00 FIFO, 91.00 LIFO/specific (new lot), or 80.50
// average. Closing the other thirteen must consume the exact remainder.
// A short's 50.00 payment less 7.50 recovery is a separate 42.50 expense;
// it must not alter either cover's result, even across backup and restore.
func TestInvestmentAuditCommandRegisterAndRestoredExport(t *testing.T) {
	t.Parallel()
	for _, side := range []string{"long", "short"} {
		for _, method := range []struct {
			name       string
			firstBasis int64
		}{
			{"fifo", 7000}, {"lifo", 9100}, {"average_cost", 8050}, {"specific_lot", 9100},
		} {
			t.Run(side+"/"+method.name, func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				f := newInvestmentsTestFixture(t)
				sign := int64(-1) // cash paid on a long opening; gained basis is negative
				open := f.investmentService.Buy
				closePosition := f.investmentService.Sell
				if side == "short" {
					sign, open, closePosition = 1, f.investmentService.ShortSale, f.investmentService.ShortCover
				}
				check := func(units, basis, cash, gain int64, decisions int) {
					t.Helper()
					lots, err := f.investmentService.ListLots(ctx, f.holdingAccountID, f.stockCommodityID)
					require.NoError(t, err)
					quantity, remainingBasis, realized := new(big.Rat), new(big.Rat), new(big.Rat)
					for _, lot := range lots {
						require.Equal(t, side, lot.PositionSide)
						quantity.Add(quantity, financialRat(lot.RemainingQuantityValue.String(), lot.RemainingQuantityScale))
						remainingBasis.Add(remainingBasis, financialRat(strconv.FormatInt(lot.RemainingCostBasisValue, 10), lot.RemainingCostBasisScale))
					}
					require.Zero(t, quantity.Cmp(big.NewRat(units, 1)), "owed/owned units")
					require.Zero(t, remainingBasis.Cmp(big.NewRat(basis, 100)), "remaining basis/opening proceeds")
					require.Equal(t, cash, accountBalance(t, f, f.cashAccountID), "net cash in cents")
					gains, err := f.investmentService.ListRealizedGains(ctx, GainsReportParams{})
					require.NoError(t, err)
					require.Len(t, gains, decisions)
					for _, entry := range gains {
						require.Equal(t, side, entry.PositionSide)
						realized.Add(realized, financialRat(strconv.FormatInt(entry.RealizedGainValue, 10), entry.RealizedGainScale))
					}
					require.Zero(t, realized.Cmp(big.NewRat(gain, 100)), "realized total excludes the separate payment expense")
					requireConservationOracle(t, f)
					requireEffectiveMovementsConsistent(t, f)
					run := mustRunInvestmentSelfCheck(t, f)
					require.Equal(t, SelfCheckPassed, run.Status, "%+v", run.Results)
				}
				a, err := open(ctx, tradeOn(f, f.holdingAccountID, "2026-01-02", 10, 10000))
				require.NoError(t, err)
				check(10, 10000, sign*10000, 0, 0)
				b, err := open(ctx, tradeOn(f, f.holdingAccountID, "2026-02-02", 10, 13000))
				require.NoError(t, err)
				check(20, 23000, sign*23000, 0, 0)
				cash := sign * 23000
				if side == "short" {
					expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
					recovery := seedTestAccountWithClass(t, f.database, "active", true, "income", "income")
					_, err := f.investmentService.PaymentInLieu(ctx, withRecovery(paymentInLieuInput(f, expense, 5000), recovery, 750))
					require.NoError(t, err)
					require.Equal(t, int64(5000), accountBalance(t, f, expense))
					require.Equal(t, int64(-750), accountBalance(t, f, recovery))
					cash -= 4250
					check(20, 23000, cash, 0, 0)
				}
				first := tradeOn(f, f.holdingAccountID, "2026-05-01", 7, 7700)
				first.CostBasisMethod = method.name
				if method.name == "specific_lot" {
					first.LotAllocations = []InvestmentLotAllocationInput{{LotID: *b.LotID, QuantityValue: exact.New(7)}}
				}
				_, err = closePosition(ctx, first)
				require.NoError(t, err)
				cash -= sign * 7700
				check(13, 23000-method.firstBasis, cash, sign*(method.firstBasis-7700), 1)

				// Preserve the partly depleted position and method election, not just
				// a closed book. All CSV data must survive the native restore unchanged.
				before := exportedBundle(t, f)
				backup := filepath.Join(t.TempDir(), "backup.sqlite")
				_, err = db.OnlineBackupSQLiteDatabase(ctx, f.database, backup, db.OnlineBackupOptions{})
				require.NoError(t, err)
				restoredURL := db.SQLiteFileURL(filepath.Join(t.TempDir(), "restored.sqlite"))
				_, err = db.RestoreSQLiteDatabase(ctx, backup, restoredURL)
				require.NoError(t, err)
				restored, err := db.Open(ctx, restoredURL)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, restored.Close()) })
				f.database = restored
				accounts := db.NewAccountRepository(restored)
				f.accountService = NewAccountService(accounts, db.NewInstitutionRepository(restored), nil)
				f.transactionService = NewTransactionService(db.NewTransactionRepository(restored), db.NewPayeeRepository(restored), accounts, db.NewCommodityRepository(restored))
				f.investmentService = NewInvestmentService(db.NewInvestmentRepository(restored), f.accountService, f.transactionService, NewPricingService(db.NewPricingRepository(restored)))
				require.Equal(t, before, exportedBundle(t, f))
				check(13, 23000-method.firstBasis, cash, sign*(method.firstBasis-7700), 1)

				last := tradeOn(f, f.holdingAccountID, "2026-06-01", 13, 12000)
				last.CostBasisMethod = method.name
				if method.name == "specific_lot" {
					last.LotAllocations = []InvestmentLotAllocationInput{
						{LotID: *a.LotID, QuantityValue: exact.New(10)}, {LotID: *b.LotID, QuantityValue: exact.New(3)},
					}
				}
				if side == "short" {
					closePosition = f.investmentService.ShortCover
				} else {
					closePosition = f.investmentService.Sell
				}
				wrongFamily := last
				wrongFamily.LotAllocations = nil
				wrongFamily.CostBasisMethod = "average_cost"
				if method.name == "average_cost" {
					wrongFamily.CostBasisMethod = "fifo"
				}
				unchanged := buyReplacementPreviewSnapshot(t, restored)
				_, err = closePosition(ctx, wrongFamily)
				require.ErrorContains(t, err, "cannot switch into or out of average_cost")
				require.Equal(t, unchanged, buyReplacementPreviewSnapshot(t, restored), "restored method lock refuses atomically")
				_, err = closePosition(ctx, last)
				require.NoError(t, err)
				check(0, 0, cash-sign*12000, sign*3300, 2)
				var locks int
				require.NoError(t, restored.QueryRow(`SELECT COUNT(*) FROM investment_position_basis_state`).Scan(&locks))
				require.Zero(t, locks, "full closure releases the method lock")
			})
		}
	}
}
