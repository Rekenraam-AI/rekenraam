package app

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
	"rekenraam/backend/migrations"
)

func TestUnknownWriteOffKeepsZeroProceedsAndResolvesAfterClosure(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
	require.NoError(t, err)
	input := backdatedWriteOff(f, "2026-07-01", 2)
	preview, err := f.investmentService.PreviewWriteOff(ctx, input)
	require.NoError(t, err)
	require.Equal(t, db.InvestmentBasisUnknown, preview.BasisKnowledge)
	result, err := f.investmentService.WriteOff(ctx, input)
	require.NoError(t, err)
	require.Equal(t, db.InvestmentBasisUnknown, result.DisposalDecision.BasisKnowledge)
	require.Zero(t, result.DisposalDecision.ProceedsValue)
	requireHealthyUnresolvedBook(t, f)
	decision := latestDisposal(t, f)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	requireScaled(t, 8000, 2, effectiveDisposal(t, f, decision.id).basisAmount(t), "full resolved write-off basis")
	var original string
	require.NoError(t, f.database.QueryRow(`SELECT basis_knowledge FROM investment_disposal_decisions WHERE id = ?`, decision.id).Scan(&original))
	require.Equal(t, db.InvestmentBasisUnknown, original, "original stays unresolved")
	requireHealthyUnresolvedBook(t, f)
}

func TestUnknownCashInLieuSettlesSplitAndResolvesLater(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
	require.NoError(t, err)
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 5, 4))
	require.NoError(t, err)
	input := cashInLieuInput(f, split.Transaction.ID)
	preview, _, err := f.investmentService.PreviewCashInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, db.InvestmentBasisUnknown, preview.DisposalDecision.BasisKnowledge)
	result, err := f.investmentService.CashInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, preview.Allocations, result.Allocations)
	_, err = f.investmentService.Sell(ctx, sellInput(f, "2026-07-01", 2))
	require.NoError(t, err)
	requireHealthyUnresolvedBook(t, f)
	decisionID := *result.DisposalDecision.ID
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	requireScaled(t, 1600, 2, effectiveDisposal(t, f, decisionID).basisAmount(t), "half of 2.5 shares takes one fifth")
	requireHealthyUnresolvedBook(t, f)
}

func TestUnknownCapitalReturnKeepsReceiptAndResolvesBeforeClosedSale(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
	require.NoError(t, err)
	input := capitalReturnInput(f, "2026-06-01", 1000)
	preview, err := f.investmentService.PreviewCapitalReturn(ctx, input)
	require.NoError(t, err)
	result, err := f.investmentService.CapitalReturn(ctx, input)
	require.NoError(t, err)
	require.Equal(t, preview.Effects, result.Effects)
	require.Equal(t, db.InvestmentBasisUnknown, result.Effects[0].BasisKnowledge)
	requireScaled(t, 1000, 2, coefScaled(result.Effects[0].AllocatedValue, result.Effects[0].AllocatedScale), "cash allocation stays exact")
	_, err = f.investmentService.Sell(ctx, sellInput(f, "2026-07-01", 2))
	require.NoError(t, err)
	requireHealthyUnresolvedBook(t, f)
	decision := latestDisposal(t, f)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	requireScaled(t, 7000, 2, effectiveDisposal(t, f, decision.id).basisAmount(t), "resolved basis less receipt")
	requireHealthyUnresolvedBook(t, f)
}

// Independent known lots stay calculable beside unknown lots. An established
// average pool never supplies a rate from its known subset, including a new
// known lot acquired after an unresolved pooling disposal.
func TestUnknownCapitalReturnExplicitKnownEntitlementAndAveragePool(t *testing.T) {
	t.Parallel()
	for _, pooled := range []bool{false, true} {
		name := "independent lots"
		if pooled {
			name = "unknown average pool"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-01-01", ""))
			require.NoError(t, err)
			if pooled {
				setHoldingCostBasisMethod(t, f, "average_cost")
				_, err = f.investmentService.Sell(ctx, sellInput(f, "2026-02-01", 1))
				require.NoError(t, err)
			}
			known := buyOn(t, f, "2026-03-01", 2, 2000)
			input := capitalReturnInput(f, "2026-04-01", 1500)
			input.LotEntitlements = []CapitalReturnEntitlement{{LotID: *known.LotID, QuantityValue: exact.New(1)}}
			before := buyReplacementPreviewSnapshot(t, f.database)
			preview, err := f.investmentService.PreviewCapitalReturn(ctx, input)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			result, err := f.investmentService.CapitalReturn(ctx, input)
			require.NoError(t, err)
			require.Equal(t, preview.Effects, result.Effects)
			if pooled {
				require.Equal(t, db.InvestmentBasisUnknown, result.Effects[0].BasisKnowledge)
				_, basis, knowledge := lotStateByID(t, f, *known.LotID)
				require.False(t, basis.Valid)
				require.Equal(t, db.InvestmentBasisUnknown, knowledge)
			} else {
				require.Equal(t, db.InvestmentBasisKnown, result.Effects[0].BasisKnowledge)
				requireScaled(t, 1000, 2, coefScaled(result.Effects[0].ReductionValue, result.Effects[0].ReductionScale), "only entitled unit basis is eligible")
				requireScaled(t, 500, 2, coefScaled(result.Effects[0].ExcessValue, result.Effects[0].ExcessScale), "unentitled units cannot absorb excess")
			}
			requireHealthyUnresolvedBook(t, f)
			resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 6000))
			requireHealthyUnresolvedBook(t, f)
		})
	}
}

func TestUnknownNonSaleMixedKnownUnknownAndAverageDisposals(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"write_off", "cash_in_lieu", "return_of_capital"} {
		for _, method := range []string{"fifo", "average_cost"} {
			t.Run(family+"/"+method, func(t *testing.T) {
				t.Parallel()
				f := newInvestmentsTestFixture(t)
				ctx := context.Background()
				seedExternalTransferEquity(t, f.database)
				setHoldingCostBasisMethod(t, f, method)
				buyOn(t, f, "2026-01-01", 2, 2000)
				transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-02-01", ""))
				require.NoError(t, err)
				split, err := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 5, 4))
				require.NoError(t, err)
				switch family {
				case "write_off":
					input := backdatedWriteOff(f, "2026-07-01", 4)
					input.CostBasisMethod = method
					result, err := f.investmentService.WriteOff(ctx, input)
					require.NoError(t, err)
					require.Equal(t, db.InvestmentBasisUnknown, result.DisposalDecision.BasisKnowledge)
					require.Equal(t, method == "fifo", result.Allocations[0].BasisKnowledge == db.InvestmentBasisKnown)
				case "cash_in_lieu":
					input := cashInLieuInput(f, split.Transaction.ID)
					if method == "fifo" {
						input.QuantityValue, input.QuantityScale = exact.New(50), 2
						input.CostBasisMethod = "specific_lot"
						var first int64
						require.NoError(t, f.database.QueryRow(`SELECT min(id) FROM investment_lots`).Scan(&first))
						input.LotAllocations = []InvestmentLotAllocationInput{{LotID: first, QuantityValue: exact.New(25), QuantityScale: 2}, {LotID: *transfer.LotID, QuantityValue: exact.New(25), QuantityScale: 2}}
					}
					result, err := f.investmentService.CashInLieu(ctx, input)
					require.NoError(t, err)
					require.Equal(t, db.InvestmentBasisUnknown, result.DisposalDecision.BasisKnowledge)
				case "return_of_capital":
					result, err := f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-07-01", 1000))
					require.NoError(t, err)
					require.Len(t, result.Effects, 2)
					require.Equal(t, db.InvestmentBasisKnown, result.Effects[0].BasisKnowledge, "independent basis before any pooling")
					require.Equal(t, db.InvestmentBasisUnknown, result.Effects[1].BasisKnowledge)
				}
				requireHealthyUnresolvedBook(t, f)
				resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 6000))
				requireHealthyUnresolvedBook(t, f)
			})
		}
	}
}

func TestUnknownNonSaleCorrectionWithdrawalAndReversal(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"write_off", "cash_in_lieu", "return_of_capital"} {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
			require.NoError(t, err)
			var actionID int64
			switch family {
			case "write_off":
				posted, err := f.investmentService.WriteOff(ctx, backdatedWriteOff(f, "2026-07-01", 2))
				require.NoError(t, err)
				input := ReplaceInvestmentWriteOffInput{OwnerUserID: f.ownerUserID, TransactionID: posted.Transaction.ID,
					Reason: "only one share was worthless", Replacement: backdatedWriteOff(f, "2026-07-01", 1)}
				input.Replacement.CostBasisMethod = "fifo"
				before := buyReplacementPreviewSnapshot(t, f.database)
				impact, err := f.investmentService.ReplaceWriteOffReconciliationImpact(ctx, input)
				require.NoError(t, err)
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
				replaced, err := f.investmentService.ReplaceWriteOff(ctx, input)
				require.NoError(t, err)
				require.Equal(t, db.InvestmentBasisUnknown, replaced.Replacement.DisposalDecision.BasisKnowledge)
				actionID = replaced.Replacement.Transaction.ID
			case "cash_in_lieu":
				split, err := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 5, 4))
				require.NoError(t, err)
				terms := cashInLieuInput(f, split.Transaction.ID)
				posted, err := f.investmentService.CashInLieu(ctx, terms)
				require.NoError(t, err)
				terms.QuantityValue, terms.QuantityScale = exact.New(25), 2
				input := ReplaceCashInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: posted.Transaction.ID,
					Reason: "correct fractional settlement", Replacement: terms}
				before := buyReplacementPreviewSnapshot(t, f.database)
				preview, impact, err := f.investmentService.PreviewCashInLieuReplacement(ctx, input)
				require.NoError(t, err)
				require.Equal(t, db.InvestmentBasisUnknown, preview.DisposalDecision.BasisKnowledge)
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
				replaced, err := f.investmentService.ReplaceCashInLieu(ctx, input)
				require.NoError(t, err)
				actionID = replaced.Replacement.Transaction.ID
			case "return_of_capital":
				terms := capitalReturnInput(f, "2026-06-01", 1000)
				posted, err := f.investmentService.CapitalReturn(ctx, terms)
				require.NoError(t, err)
				terms.AmountValue = exact.New(1500)
				input := ReplaceInvestmentCapitalReturnInput{OwnerUserID: f.ownerUserID, TransactionID: posted.Transaction.ID,
					Reason: "correct receipt", Replacement: terms}
				before := buyReplacementPreviewSnapshot(t, f.database)
				preview, err := f.investmentService.PreviewCapitalReturnReplacement(ctx, input)
				require.NoError(t, err)
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				replaced, err := f.investmentService.ReplaceCapitalReturn(ctx, input)
				require.NoError(t, err)
				require.Equal(t, preview.Effects, replaced.Replacement.Effects)
				actionID = replaced.Replacement.Transaction.ID
			}
			requireHealthyUnresolvedBook(t, f)
			resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
			requireHealthyUnresolvedBook(t, f)
			withdraw := CorrectTransferBasisResolutionInput{OwnerUserID: f.ownerUserID, TransactionID: transfer.Transaction.ID, Reason: "statement withdrawn"}
			impact, err := f.investmentService.ReverseTransferBasisResolutionImpact(ctx, withdraw)
			require.NoError(t, err)
			if impact.GainImpact != nil {
				withdraw.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
			}
			_, err = f.investmentService.ReverseTransferBasisResolution(ctx, withdraw)
			require.NoError(t, err)
			requireHealthyUnresolvedBook(t, f)
			if family == "return_of_capital" {
				input := ReverseInvestmentCapitalReturnInput{OwnerUserID: f.ownerUserID, TransactionID: actionID, Reason: "receipt withdrawn"}
				_, err = f.investmentService.ReverseCapitalReturn(ctx, input)
			} else {
				input := ReverseInvestmentSaleInput{OwnerUserID: f.ownerUserID, TransactionID: actionID, Reason: "action withdrawn"}
				preview := f.investmentService.ReverseWriteOffReconciliationImpact
				commit := f.investmentService.ReverseWriteOff
				if family == "cash_in_lieu" {
					preview, commit = f.investmentService.ReverseCashInLieuReconciliationImpact, f.investmentService.ReverseCashInLieu
				}
				impact, e := preview(ctx, input)
				require.NoError(t, e)
				input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
				_, err = commit(ctx, input)
			}
			require.NoError(t, err)
			requireHealthyUnresolvedBook(t, f)
		})
	}
}

// A refusal after all resolution effects and replay revisions have been
// computed must roll back every row, in each newly admitted family.
func TestUnknownNonSaleResolutionLateFailureIsAtomic(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"write_off", "cash_in_lieu", "return_of_capital"} {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
			require.NoError(t, err)
			switch family {
			case "write_off":
				_, err = f.investmentService.WriteOff(ctx, backdatedWriteOff(f, "2026-07-01", 2))
			case "cash_in_lieu":
				var split InvestmentSplitResult
				split, err = f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 5, 4))
				require.NoError(t, err)
				_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
			case "return_of_capital":
				_, err = f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
				require.NoError(t, err)
				_, err = f.investmentService.Sell(ctx, sellInput(f, "2026-07-01", 2))
			}
			require.NoError(t, err)
			input := resolveInput(f, transfer.Transaction.ID, 8000)
			impact, err := f.investmentService.ResolveTransferBasisImpact(ctx, input)
			require.NoError(t, err)
			require.Len(t, impact.GainImpact.Changes, 1)
			before := buyReplacementPreviewSnapshot(t, f.database)
			_, err = f.investmentService.ResolveTransferBasis(ctx, input)
			require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
			input.BasisValue = 9000
			_, err = f.investmentService.ResolveTransferBasis(ctx, input)
			require.ErrorIs(t, err, ErrGainImpactAcknowledgementStale)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			input.BasisValue = 8000
			_, err = f.database.Exec(`CREATE TRIGGER refuse_non_sale_revision BEFORE INSERT ON investment_disposal_revisions BEGIN SELECT RAISE(ABORT, 'forced late non-sale refusal'); END`)
			require.NoError(t, err)
			_, err = f.investmentService.ResolveTransferBasis(ctx, input)
			require.ErrorContains(t, err, "forced late non-sale refusal")
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
		})
	}
}

func TestUnknownCapitalReturnSelfCheckAuditsOriginalAndRevisedKnowledge(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
	require.NoError(t, err)
	_, err = f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
	require.NoError(t, err)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	var original, revision string
	var reduction sql.NullString
	require.NoError(t, f.database.QueryRow(`SELECT basis_knowledge, reduction_value FROM investment_capital_return_effects`).Scan(&original, &reduction))
	require.Equal(t, db.InvestmentBasisUnknown, original)
	require.False(t, reduction.Valid)
	require.NoError(t, f.database.QueryRow(`SELECT basis_knowledge FROM investment_capital_return_revision_effects`).Scan(&revision))
	require.Equal(t, db.InvestmentBasisKnown, revision)
	requireHealthyUnresolvedBook(t, f)
	_, err = f.database.Exec(`DROP TRIGGER investment_capital_return_effects_no_update`)
	require.NoError(t, err)
	_, err = f.database.Exec(`UPDATE investment_capital_return_effects SET basis_knowledge = 'known', reduction_value = '0', reduction_scale = 0, excess_value = allocated_value, excess_scale = allocated_scale`)
	require.NoError(t, err)
	foundation := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentFoundation)
	require.Equal(t, SelfCheckFailed, foundation.Status, "superseded unknown evidence cannot be relabeled known")
}

func TestUnknownNonSaleExportResolutionAndRestoreRetainOriginalNulls(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"write_off", "cash_in_lieu", "return_of_capital"} {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
			require.NoError(t, err)
			switch family {
			case "write_off":
				_, err = f.investmentService.WriteOff(ctx, backdatedWriteOff(f, "2026-07-01", 2))
			case "cash_in_lieu":
				var split InvestmentSplitResult
				split, err = f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 5, 4))
				require.NoError(t, err)
				_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
			case "return_of_capital":
				_, err = f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
			}
			require.NoError(t, err)
			for _, resolved := range []bool{false, true} {
				if resolved {
					resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
				}
				var out bytes.Buffer
				require.NoError(t, NewExportService(db.NewExportRepository(f.database)).WriteBundle(ctx, &out, ExportFilter{}))
				archive, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
				require.NoError(t, err)
				originalName, revisedName := "disposal-decisions.csv", "disposal-revisions.csv"
				amounts := []string{"disposed_basis_value", "disposed_basis_scale"}
				if family == "return_of_capital" {
					originalName, revisedName = "investment-capital-return-effects.csv", "investment-capital-return-revision-effects.csv"
					amounts = []string{"reduction_value", "reduction_scale", "excess_value", "excess_scale"}
				}
				seen := 0
				for _, entry := range archive.File {
					if entry.Name != originalName && entry.Name != revisedName {
						continue
					}
					reader, err := entry.Open()
					require.NoError(t, err)
					rows, err := csv.NewReader(reader).ReadAll()
					require.NoError(t, err)
					require.NoError(t, reader.Close())
					if entry.Name == revisedName && !resolved {
						require.Len(t, rows, 1)
						continue
					}
					require.Len(t, rows, 2)
					knowledge := "unknown"
					if entry.Name == revisedName {
						knowledge = "known"
					}
					require.Equal(t, knowledge, rows[1][slices.Index(rows[0], "basis_knowledge")])
					columns := amounts
					if family != "return_of_capital" && entry.Name == originalName {
						columns = []string{"disposed_basis"}
					}
					for _, column := range columns {
						if knowledge == "unknown" {
							require.Empty(t, rows[1][slices.Index(rows[0], column)])
						} else {
							require.NotEmpty(t, rows[1][slices.Index(rows[0], column)])
						}
					}
					seen++
				}
				want := 1
				if resolved {
					want = 2
				}
				require.Equal(t, want, seen)
				backupPath := filepath.Join(t.TempDir(), "backup.sqlite")
				_, err = db.OnlineBackupSQLiteDatabase(ctx, f.database, backupPath, db.OnlineBackupOptions{})
				require.NoError(t, err)
				restoredURL := db.SQLiteFileURL(filepath.Join(t.TempDir(), "restored.sqlite"))
				_, err = db.RestoreSQLiteDatabase(ctx, backupPath, restoredURL)
				require.NoError(t, err)
				restored, err := db.Open(ctx, restoredURL)
				require.NoError(t, err)
				run, err := selfCheckOver(t, restored).RunSelfCheck(ctx, "manual")
				require.NoError(t, err)
				require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentFoundation).Status)
				require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentReplay).Status)
				require.NoError(t, restored.Close())
			}
		})
	}
}

func TestCapitalReturnForwardMigrationPreservesPopulatedKnownHistory(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buy := buyOn(t, f, "2026-05-01", 2, 2000)
	_, err := f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1500))
	require.NoError(t, err)
	_, err = acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, buy, "2026-05-01", 2, 1000))
	require.NoError(t, err)
	require.Equal(t, 1, capitalReturnRevisionCount(t, f))
	before := buyReplacementPreviewSnapshot(t, f.database)
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 2)
	require.NoError(t, err, "known history can be represented by schema 2")
	require.NoError(t, db.Migrate(ctx, f.database))
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "forward upgrade preserves every original and revised figure")
	requireInvestmentSelfCheckPasses(t, f)
}

func TestCapitalReturnUnknownAmountPairsAndDowngradeRefusal(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	_, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
	require.NoError(t, err)
	_, err = f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	// Step below 0003 explicitly so later migrations do not change the target.
	_, err = provider.DownTo(ctx, 2)
	require.ErrorContains(t, err, "CHECK constraint failed", "unknown evidence cannot be discarded by downgrade")
	// Later migrations may have been removed before schema 3 refused.
	require.NoError(t, db.Migrate(ctx, f.database))
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.database.Exec(`DROP TRIGGER investment_capital_return_effects_no_update`)
	require.NoError(t, err)
	for _, assignment := range []string{
		"basis_knowledge = 'known'",
		"reduction_value = '0'", "reduction_scale = 0", "excess_value = '0'", "excess_scale = 0",
		"basis_knowledge = 'known', reduction_value = '0', reduction_scale = 0, excess_value = '0'",
		"basis_knowledge = 'known', reduction_value = '0', excess_value = '0', excess_scale = 0",
	} {
		_, err = f.database.Exec(`UPDATE investment_capital_return_effects SET ` + assignment)
		require.ErrorContains(t, err, "CHECK constraint failed", assignment)
		require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	}
}

func TestUnknownNonSaleCheckpointsAndLateActionRollback(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"write_off", "cash_in_lieu", "return_of_capital"} {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			_, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
			require.NoError(t, err)
			var actionID, checkpointID int64
			var preview func(context.Context, ReverseInvestmentSaleInput) (ReconciliationImpact, error)
			var reverse func(context.Context, ReverseInvestmentSaleInput) (Transaction, error)
			switch family {
			case "write_off":
				posted, err := f.investmentService.WriteOff(ctx, backdatedWriteOff(f, "2026-06-01", 2))
				require.NoError(t, err)
				actionID = posted.Transaction.ID
				checkpointID = reconcileHolding(t, f, "2026-07-01", 0)
				preview, reverse = f.investmentService.ReverseWriteOffReconciliationImpact, f.investmentService.ReverseWriteOff
			case "cash_in_lieu":
				split, err := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 5, 4))
				require.NoError(t, err)
				posted, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
				require.NoError(t, err)
				actionID = posted.Transaction.ID
				checkpointID = reconcileCash(t, f, "2026-07-01", 8)
				preview, reverse = f.investmentService.ReverseCashInLieuReconciliationImpact, f.investmentService.ReverseCashInLieu
			case "return_of_capital":
				posted, err := f.investmentService.CapitalReturn(ctx, capitalReturnInput(f, "2026-06-01", 1000))
				require.NoError(t, err)
				actionID = posted.Transaction.ID
				checkpointID = reconcileCash(t, f, "2026-07-01", 10)
				asCapital := func(input ReverseInvestmentSaleInput) ReverseInvestmentCapitalReturnInput {
					return ReverseInvestmentCapitalReturnInput{OwnerUserID: input.OwnerUserID, TransactionID: input.TransactionID, Reason: input.Reason,
						ReconciliationOverride: input.ReconciliationOverride, GainImpactAcknowledgement: input.GainImpactAcknowledgement}
				}
				preview = func(ctx context.Context, input ReverseInvestmentSaleInput) (ReconciliationImpact, error) {
					return f.investmentService.ReverseCapitalReturnReconciliationImpact(ctx, asCapital(input))
				}
				reverse = func(ctx context.Context, input ReverseInvestmentSaleInput) (Transaction, error) {
					return f.investmentService.ReverseCapitalReturn(ctx, asCapital(input))
				}
			}
			input := ReverseInvestmentSaleInput{OwnerUserID: f.ownerUserID, TransactionID: actionID, Reason: "action withdrawn"}
			before := buyReplacementPreviewSnapshot(t, f.database)
			impact, err := preview(ctx, input)
			require.NoError(t, err)
			require.Len(t, impact.AffectedCheckpoints, 1)
			require.Equal(t, checkpointID, impact.AffectedCheckpoints[0].CheckpointID)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			if impact.GainImpact != nil {
				input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
			}
			_, err = reverse(ctx, input)
			require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			input.ReconciliationOverride = true
			_, err = f.database.Exec(`CREATE TRIGGER refuse_unknown_action_checkpoint AFTER UPDATE ON reconciliation_checkpoints
				WHEN NEW.status = 'invalidated' BEGIN SELECT RAISE(ABORT, 'forced late checkpoint failure'); END`)
			require.NoError(t, err)
			_, err = reverse(ctx, input)
			require.ErrorContains(t, err, "forced late checkpoint failure")
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			_, err = f.database.Exec(`DROP TRIGGER refuse_unknown_action_checkpoint`)
			require.NoError(t, err)
			_, err = reverse(ctx, input)
			require.NoError(t, err)
			require.Empty(t, activeCheckpointIDs(t, f))
			requireHealthyUnresolvedBook(t, f)
		})
	}
}
