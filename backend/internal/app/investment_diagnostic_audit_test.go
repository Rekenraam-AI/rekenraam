package app

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #308: expectations are hand-derived from two inbound units with basis
// unknown, 80.00, zero, 120.00, then withdrawn. All amounts below are cents;
// no diagnostic or expected financial amount is produced by replay.
func TestInvestmentDiagnosticAuditResolutionLifecycleAndRestore(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		family, quantity string
		cash             int64
		decisions        int
		remaining        [3]int64 // 80.00, known zero, 120.00
		disposed         [3]int64
		gain             [3]int64
	}{
		{"partial_sale", "1", 5000, 1, [3]int64{4000, 0, 6000}, [3]int64{4000, 0, 6000}, [3]int64{1000, 5000, -1000}},
		{"closed_sale", "0", 10000, 1, [3]int64{}, [3]int64{8000, 0, 12000}, [3]int64{2000, 10000, -2000}},
		{"write_off", "0", 0, 1, [3]int64{}, [3]int64{8000, 0, 12000}, [3]int64{-8000, 0, -12000}},
		{"cash_in_lieu", "2", 800, 1, [3]int64{6400, 0, 9600}, [3]int64{1600, 0, 2400}, [3]int64{-800, 800, -1600}},
		{"closed_cash_in_lieu", "0", 10800, 2, [3]int64{}, [3]int64{8000, 0, 12000}, [3]int64{2800, 10800, -1200}},
		{"capital_return_then_sale", "0", 11000, 1, [3]int64{}, [3]int64{7000, 0, 11000}, [3]int64{3000, 10000, -1000}},
	} {
		t.Run(tc.family, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := newInvestmentsTestFixture(t)
			seedExternalTransferEquity(t, f.database)
			inbound, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", "2020-03-01"))
			require.NoError(t, err)
			lotID := *inbound.LotID
			check := func(phase, quantity string, cash int64, decisions, index int, restore bool) {
				t.Helper()
				inspect := func(current *investmentsTestFixture) SelfCheckRun {
					t.Helper()
					var q exact.Coefficient
					var scale int
					var basis *string
					var basisScale *int
					var knowledge, opening string
					var openingBasis *string
					require.NoError(t, current.database.QueryRow(`SELECT s.remaining_quantity_value, s.remaining_quantity_scale,
						s.remaining_cost_basis_value, s.remaining_cost_basis_scale, s.basis_knowledge, l.opening_basis_knowledge, l.cost_basis_value
						FROM investment_lot_state s JOIN investment_lots l ON l.id=s.lot_id WHERE l.id=?`, lotID).
						Scan(&q, &scale, &basis, &basisScale, &knowledge, &opening, &openingBasis))
					wantQuantity, ok := new(big.Rat).SetString(quantity)
					require.True(t, ok)
					require.Zero(t, financialRat(q.String(), scale).Cmp(wantQuantity), phase+": quantity")
					require.Equal(t, "unknown", opening, "immutable opening evidence stays unknown")
					require.Nil(t, openingBasis)
					require.Equal(t, cash, accountBalance(t, current, current.cashAccountID), phase+": cash")
					if index < 0 {
						require.Equal(t, "unknown", knowledge)
						require.Nil(t, basis)
						require.Nil(t, basisScale)
					} else {
						require.Equal(t, "known", knowledge)
						require.NotNil(t, basis)
						require.NotNil(t, basisScale)
						require.Zero(t, financialRat(*basis, *basisScale).Cmp(big.NewRat(tc.remaining[index], 100)), phase+": remaining basis")
					}
					gains, err := current.investmentService.ListRealizedGains(ctx, GainsReportParams{})
					require.NoError(t, err)
					require.Len(t, gains, decisions)
					disposed, gain := new(big.Rat), new(big.Rat)
					for _, entry := range gains {
						require.Equal(t, knowledge, entry.BasisKnowledge)
						disposed.Sub(disposed, financialRat(strconv.FormatInt(entry.DisposedBasisValue, 10), entry.DisposedBasisScale))
						gain.Add(gain, financialRat(strconv.FormatInt(entry.RealizedGainValue, 10), entry.RealizedGainScale))
					}
					if index >= 0 {
						require.Zero(t, disposed.Cmp(big.NewRat(tc.disposed[index], 100)), phase+": disposed basis")
						require.Zero(t, gain.Cmp(big.NewRat(tc.gain[index], 100)), phase+": realized result")
					}
					if tc.family == "capital_return_then_sale" && decisions > 0 {
						returns, err := current.investmentService.ListCapitalReturns(ctx, GainsReportParams{})
						require.NoError(t, err)
						require.Len(t, returns, 1)
						if index < 0 {
							require.Equal(t, 1, returns[0].UnknownLots)
						} else {
							require.Zero(t, returns[0].UnknownLots)
							reduction, excess := int64(1000), int64(0)
							if index == 1 {
								reduction, excess = 0, 1000
							}
							requireScaled(t, reduction, 2, exact.ScaledIntFromCoefficient(returns[0].ReductionValue, returns[0].ReductionScale), "capital reduction")
							requireScaled(t, excess, 2, exact.ScaledIntFromCoefficient(returns[0].ExcessValue, returns[0].ExcessScale), "excess stays outside disposal gain")
						}
					}
					before := exportedBundle(t, current)
					run := mustRunInvestmentSelfCheck(t, current)
					require.Len(t, run.Results, 13)
					require.Equal(t, before, exportedBundle(t, current), "self-check does not rewrite financial/exported evidence")
					for _, result := range run.Results {
						expected := SelfCheckPassed
						count := int64(0)
						if result.CheckID == CheckAttachments {
							expected = SelfCheckNotApplicable
						} else if result.CheckID == CheckLotReconciliation && index < 0 {
							// #313: supported unknown basis is incomplete
							// coverage, not an integrity finding.
							expected, count = SelfCheckIncomplete, 1
							require.Equal(t, "1 lots have unknown projected basis; basis reconciliation is unavailable", result.Summary)
							require.Empty(t, result.Sample)
							require.Equal(t, []int64{lotID}, result.UnavailableSample)
						}
						require.Equal(t, expected, result.Status, "%s/%s: %s", phase, result.CheckID, result.Summary)
						require.Zero(t, result.FindingCount, phase+"/"+result.CheckID)
						require.Equal(t, count, result.UnavailableCount, phase+"/"+result.CheckID)
					}
					require.Zero(t, run.FailedCheckCount)
					if index < 0 {
						require.Equal(t, SelfCheckIncomplete, run.Status)
						require.EqualValues(t, 1, run.IncompleteCheckCount)
					} else {
						require.Equal(t, SelfCheckPassed, run.Status)
						require.Zero(t, run.IncompleteCheckCount)
					}
					requireConservationOracle(t, current)
					t.Logf("%s: overall=%s lot=%s findings=%d unavailable=%d", phase, run.Status,
						resultFor(t, run, CheckLotReconciliation).Status, resultFor(t, run, CheckLotReconciliation).FindingCount,
						resultFor(t, run, CheckLotReconciliation).UnavailableCount)
					return run
				}
				run := inspect(f)
				if restore {
					before := exportedBundle(t, f)
					backup := filepath.Join(t.TempDir(), "backup.sqlite")
					_, err := db.OnlineBackupSQLiteDatabase(ctx, f.database, backup, db.OnlineBackupOptions{})
					require.NoError(t, err)
					url := db.SQLiteFileURL(filepath.Join(t.TempDir(), "restored.sqlite"))
					_, err = db.RestoreSQLiteDatabase(ctx, backup, url)
					require.NoError(t, err)
					database, err := db.Open(ctx, url)
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, database.Close()) })
					restored := *f
					restored.database = database
					accounts := db.NewAccountRepository(database)
					restored.accountService = NewAccountService(accounts, db.NewInstitutionRepository(database), nil)
					restored.transactionService = NewTransactionService(db.NewTransactionRepository(database), db.NewPayeeRepository(database), accounts, db.NewCommodityRepository(database))
					restored.investmentService = NewInvestmentService(db.NewInvestmentRepository(database), restored.accountService, restored.transactionService, NewPricingService(db.NewPricingRepository(database)))
					require.Equal(t, before, exportedBundle(t, &restored))
					// #313: the recorded run is history: restore carries its
					// verdict, counts and unavailable scope unchanged.
					recorded, hasRun, err := selfCheckOver(t, f.database).LatestSelfCheck(ctx)
					require.NoError(t, err)
					require.True(t, hasRun)
					restoredRecord, hasRun, err := selfCheckOver(t, database).LatestSelfCheck(ctx)
					require.NoError(t, err)
					require.True(t, hasRun)
					require.Equal(t, recorded, restoredRecord, "restore preserves the recorded run")
					require.Equal(t, run.ID, restoredRecord.ID)
					require.Equal(t, run.Status, restoredRecord.Status)
					require.Equal(t, run.IncompleteCheckCount, restoredRecord.IncompleteCheckCount)
					require.Equal(t, run.Results, inspect(&restored).Results, "restore preserves every diagnostic result")
				}
			}
			check("inbound", "2", 0, 0, -1, false)
			switch tc.family {
			case "partial_sale":
				_, err = f.investmentService.Sell(ctx, tradeOn(f, f.holdingAccountID, "2026-07-01", 1, 5000))
			case "closed_sale":
				_, err = f.investmentService.Sell(ctx, tradeOn(f, f.holdingAccountID, "2026-07-01", 2, 10000))
			case "write_off":
				_, err = f.investmentService.WriteOff(ctx, backdatedWriteOff(f, "2026-07-01", 2))
			case "cash_in_lieu", "closed_cash_in_lieu":
				split, splitErr := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 5, 4))
				require.NoError(t, splitErr)
				check("split", "5/2", 0, 0, -1, false)
				_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
				require.NoError(t, err)
				if tc.family == "closed_cash_in_lieu" {
					_, err = f.investmentService.Sell(ctx, tradeOn(f, f.holdingAccountID, "2026-07-01", 2, 10000))
				}
			case "capital_return_then_sale":
				_, err = f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
				require.NoError(t, err)
				_, err = f.investmentService.Sell(ctx, tradeOn(f, f.holdingAccountID, "2026-07-01", 2, 10000))
			}
			require.NoError(t, err)
			check("unresolved", tc.quantity, tc.cash, tc.decisions, -1, true)
			resolveAcknowledged(t, f, resolveInput(f, inbound.Transaction.ID, 8000))
			check("positive_80", tc.quantity, tc.cash, tc.decisions, 0, true)
			replaceResolutionAcknowledged(t, f, correctResolutionInput(f, inbound.Transaction.ID, 0))
			check("known_zero", tc.quantity, tc.cash, tc.decisions, 1, true)
			replaceResolutionAcknowledged(t, f, correctResolutionInput(f, inbound.Transaction.ID, 12000))
			check("replacement_120", tc.quantity, tc.cash, tc.decisions, 2, false)
			reverseResolutionAcknowledged(t, f, correctResolutionInput(f, inbound.Transaction.ID, 0))
			check("withdrawn", tc.quantity, tc.cash, tc.decisions, -1, true)
		})
	}
}

// Corruption is introduced only into disposable test books, sometimes after
// dropping immutability triggers. These are diagnostic controls, not states
// the ordinary investment writer admits. Every book also has unknown basis.
func TestInvestmentDiagnosticAuditDamageBesideUnknownBasis(t *testing.T) {
	t.Parallel()
	for _, damage := range []string{"quantity", "allocation_proceeds", "transfer_link", "resolution_bridge", "known_basis_projection", "superseded_capital_return"} {
		t.Run(damage, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := newInvestmentsTestFixture(t)
			seedExternalTransferEquity(t, f.database)
			inbound, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", "2020-03-01"))
			require.NoError(t, err)
			before := mustRunInvestmentSelfCheck(t, f)
			require.Equal(t, SelfCheckIncomplete, before.Status)
			require.Zero(t, before.FailedCheckCount)
			require.EqualValues(t, 1, before.IncompleteCheckCount)
			require.Zero(t, resultFor(t, before, CheckLotReconciliation).FindingCount)
			require.EqualValues(t, 1, resultFor(t, before, CheckLotReconciliation).UnavailableCount)
			checkID, fragment := CheckLotReconciliation, "holdings disagree"
			switch damage {
			case "quantity":
				_, err = f.database.Exec(`UPDATE investment_lot_state SET remaining_quantity_value='1' WHERE lot_id=?`, *inbound.LotID)
			case "allocation_proceeds":
				_, err = f.investmentService.Sell(ctx, tradeOn(f, f.holdingAccountID, "2026-07-01", 1, 5000))
				require.NoError(t, err)
				_, err = f.database.Exec(`DROP TRIGGER investment_disposal_allocations_no_update`)
				require.NoError(t, err)
				_, err = f.database.Exec(`UPDATE investment_disposal_allocations SET proceeds_value='6000'`)
				fragment = "disposal allocation sets disagree"
			case "transfer_link":
				destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
				_, err = f.investmentService.InternalTransfer(ctx, internalTransferFromLot(f, destination, *inbound.LotID, exact.New(1), 0))
				require.NoError(t, err)
				_, err = f.database.Exec(`DROP TRIGGER investment_transfer_lot_links_no_update`)
				require.NoError(t, err)
				_, err = f.database.Exec(`UPDATE investment_transfer_lot_links SET quantity_value='2' WHERE source_lot_id=?`, *inbound.LotID)
				checkID, fragment = CheckInvestmentFoundation, "transfer"
			case "resolution_bridge":
				resolveAcknowledged(t, f, resolveInput(f, inbound.Transaction.ID, 8000))
				_, err = f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", "2020-04-01"))
				require.NoError(t, err)
				_, err = f.database.Exec(`DROP TRIGGER investment_basis_resolutions_no_update`)
				require.NoError(t, err)
				_, err = f.database.Exec(`UPDATE investment_basis_resolutions SET basis_value='7000'`)
				checkID, fragment = CheckInvestmentFoundation, "basis resolution disagrees with its bridge or pinned transfer"
			case "known_basis_projection":
				known := buyOn(t, f, "2026-04-01", 2, 2000)
				_, err = f.database.Exec(`UPDATE investment_lot_state SET remaining_cost_basis_value='2100' WHERE lot_id=?`, *known.LotID)
				checkID, fragment = CheckInvestmentReplay, "lot balances"
			case "superseded_capital_return":
				_, err = f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
				require.NoError(t, err)
				resolveAcknowledged(t, f, resolveInput(f, inbound.Transaction.ID, 8000))
				_, err = f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-08-01", "2020-04-01"))
				require.NoError(t, err)
				_, err = f.database.Exec(`DROP TRIGGER investment_capital_return_effects_no_update`)
				require.NoError(t, err)
				_, err = f.database.Exec(`UPDATE investment_capital_return_effects SET basis_knowledge='known',
					reduction_value='0', reduction_scale=0, excess_value=allocated_value, excess_scale=allocated_scale`)
				checkID, fragment = CheckInvestmentFoundation, "capital"
			}
			require.NoError(t, err)
			snapshot := buyReplacementPreviewSnapshot(t, f.database)
			run := mustRunInvestmentSelfCheck(t, f)
			require.Equal(t, snapshot, buyReplacementPreviewSnapshot(t, f.database), "diagnostics never repair damage")
			require.Equal(t, SelfCheckFailed, run.Status)
			result := resultFor(t, run, checkID)
			require.Equal(t, SelfCheckFailed, result.Status)
			require.Positive(t, result.FindingCount)
			require.Contains(t, result.Summary, fragment)
			// #313: integrity findings and unavailable lots are counted apart.
			// Unknown basis never downgrades damage, and damage never hides the
			// unavailable scope beside it.
			expectedFailures := map[string]int64{}
			unavailableLots := int64(1)
			switch damage {
			case "quantity":
				expectedFailures[CheckLotReconciliation], expectedFailures[CheckInvestmentReplay] = 2, 1
			case "allocation_proceeds":
				expectedFailures[CheckLotReconciliation], expectedFailures[CheckInvestmentReplay] = 1, 1
			case "known_basis_projection":
				expectedFailures[CheckInvestmentReplay] = 1
			case "transfer_link":
				expectedFailures[CheckInvestmentFoundation], expectedFailures[CheckInvestmentReplay] = 1, 2
				unavailableLots = 2
			case "resolution_bridge":
				expectedFailures[CheckInvestmentFoundation], expectedFailures[CheckInvestmentReplay] = 1, 1
			case "superseded_capital_return":
				expectedFailures[CheckInvestmentFoundation] = 1
			}
			require.Len(t, run.Results, 13)
			require.EqualValues(t, len(expectedFailures), run.FailedCheckCount)
			require.EqualValues(t, 1, run.IncompleteCheckCount)
			for _, check := range run.Results {
				status := SelfCheckPassed
				unavailable := int64(0)
				if check.CheckID == CheckLotReconciliation {
					unavailable = unavailableLots
					status = SelfCheckIncomplete
				}
				if expectedFailures[check.CheckID] > 0 {
					status = SelfCheckFailed
				} else if check.CheckID == CheckAttachments {
					status = SelfCheckNotApplicable
				}
				require.Equal(t, status, check.Status, damage+"/"+check.CheckID+": "+check.Summary)
				require.Equal(t, expectedFailures[check.CheckID], check.FindingCount, damage+"/"+check.CheckID)
				require.Equal(t, unavailable, check.UnavailableCount, damage+"/"+check.CheckID)
				require.Len(t, check.UnavailableSample, int(unavailable), damage+"/"+check.CheckID)
			}
			lots := resultFor(t, run, CheckLotReconciliation)
			if damage == "resolution_bridge" || damage == "superseded_capital_return" {
				require.NotContains(t, lots.UnavailableSample, *inbound.LotID, "a resolved lot is known, so only the later inbound is unavailable")
			} else {
				require.Contains(t, lots.UnavailableSample, *inbound.LotID, "closed or open, the unknown lot stays disclosed")
			}
			require.Contains(t, lots.Summary, fmt.Sprintf("%d lots have unknown projected basis; basis reconciliation is unavailable", unavailableLots))
			if damage == "known_basis_projection" {
				require.Equal(t, "1 lots have unknown projected basis; basis reconciliation is unavailable", lots.Summary,
					"unknown position suppresses aggregate basis comparison, so replay must catch the known-lot damage")
			}
			for _, result := range run.Results {
				if result.Status == SelfCheckFailed {
					t.Logf("%s: %s findings=%d sample=%v summary=%s", damage, result.CheckID, result.FindingCount, result.Sample, result.Summary)
				}
			}
		})
	}
}
