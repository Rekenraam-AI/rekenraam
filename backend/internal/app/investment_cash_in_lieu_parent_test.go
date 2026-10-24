package app

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

func TestCashInLieuSurvivesSplitRatioCorrectionWithReviewedBasis(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-05-01", 3, 3600)
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 3, 2))
	require.NoError(t, err)
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, split.Transaction.ID))
	require.NoError(t, err)
	checkpoint := reconcileCash(t, f, "2026-07-01", -28)
	before := buyReplacementPreviewSnapshot(t, f.database)
	input := replaceSplitInput(f, split.Transaction.ID, "2026-06-01", 5, 2)
	preview, err := f.investmentService.PreviewSplitReplacement(ctx, input)
	require.NoError(t, err)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	require.NotNil(t, preview.Impact.GainImpact)
	_, err = f.investmentService.ReplaceSplit(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	_, err = f.investmentService.ReplaceSplit(ctx, input)
	require.NoError(t, err)
	quantity, basis := openPositionBasis(t, f, f.holdingAccountID)
	requireScaled(t, 7, 0, quantity, "3 * 2.5 - 0.5")
	requireScaled(t, 3360, 2, basis, "36 - 2.40")
	requireScaled(t, 240, 2, saleEffectiveBasis(t, f, cil.Transaction.ID), "36 / 7.5 * 0.5")
	require.Equal(t, int64(-2800), accountBalance(t, f, f.cashAccountID))
	require.Equal(t, []int64{checkpoint}, activeCheckpointIDs(t, f))
	requireSelfCheckPasses(t, f)
}

type cashParentCase struct {
	f                 *investmentsTestFixture
	kind              string
	parent, commodity int64
}

func newCashParentCase(t *testing.T, kind string, independentUnits bool) cashParentCase {
	t.Helper()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	buyOn(t, f, "2026-05-01", 3, 3600)
	c := cashParentCase{f: f, kind: kind, commodity: f.stockCommodityID}
	if kind != "split" {
		c.commodity = seedTestSecurityCommodity(t, f, "FRACTION")
	}
	if independentUnits && kind != "split" {
		input := InvestmentTradeInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-05-02", HoldingAccountID: f.holdingAccountID, CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, QuantityValue: exact.New(2), CashAmountValue: 2000, CashAmountScale: 2}
		input.CommodityID = c.commodity
		_, err := f.investmentService.Buy(ctx, input)
		require.NoError(t, err)
	}
	var err error
	switch kind {
	case "split":
		result, e := f.investmentService.Split(ctx, splitInput(f, "2026-06-01", 3, 2))
		c.parent, err = result.Transaction.ID, e
	case "share_exchange":
		result, e := f.investmentService.ShareExchange(ctx, shareExchangeInput(f, c.commodity, "2026-06-01", 3, 2))
		c.parent, err = result.Transaction.ID, e
	case "spin_off":
		result, e := f.investmentService.SpinOff(ctx, spinOffInput(f, c.commodity, "2026-06-01", 3, 2, "2", 1))
		c.parent, err = result.Transaction.ID, e
	case "cash_and_stock_merger":
		result, e := f.investmentService.CashStockMerger(ctx, cashStockMergerInput(f, c.commodity, "2026-06-01", "2026-06-05", 3, 2, 300, "25", 2))
		c.parent, err = result.Transaction.ID, e
	default:
		t.Fatal(kind)
	}
	require.NoError(t, err)
	return c
}

func (c cashParentCase) replace(date string, numerator, denominator int64, ack string, preview bool) (int64, ReconciliationImpact, error) {
	ctx := context.Background()
	s := c.f.investmentService
	switch c.kind {
	case "split":
		input := replaceSplitInput(c.f, c.parent, date, numerator, denominator)
		input.GainImpactAcknowledgement = ack
		if preview {
			p, e := s.PreviewSplitReplacement(ctx, input)
			return 0, p.Impact, e
		}
		r, e := s.ReplaceSplit(ctx, input)
		return r.Replacement.ID, ReconciliationImpact{}, e
	case "share_exchange":
		input := replaceShareExchangeInput(c.f, c.parent, c.commodity, date, numerator, denominator)
		input.GainImpactAcknowledgement = ack
		if preview {
			p, e := s.PreviewShareExchangeReplacement(ctx, input)
			return 0, p.Impact, e
		}
		r, e := s.ReplaceShareExchange(ctx, input)
		return r.Replacement.ID, ReconciliationImpact{}, e
	case "spin_off":
		input := replaceSpinOffInput(c.f, c.parent, c.commodity, date, numerator, denominator, "2", 1)
		input.GainImpactAcknowledgement = ack
		if preview {
			p, e := s.PreviewSpinOffReplacement(ctx, input)
			return 0, p.Impact, e
		}
		r, e := s.ReplaceSpinOff(ctx, input)
		return r.Replacement.ID, ReconciliationImpact{}, e
	default:
		input := replaceCashStockMergerInput(c.f, c.parent, c.commodity, date, "2026-06-05", numerator, denominator, 300, "25", 2)
		input.GainImpactAcknowledgement = ack
		if preview {
			p, e := s.PreviewCashStockMergerReplacement(ctx, input)
			return 0, p.Impact, e
		}
		r, e := s.ReplaceCashStockMerger(ctx, input)
		return r.Replacement.ID, ReconciliationImpact{}, e
	}
}

func (c cashParentCase) reverse(ack string, preview bool) (ReconciliationImpact, error) {
	ctx := context.Background()
	s := c.f.investmentService
	switch c.kind {
	case "split":
		input := ReverseInvestmentSplitInput{OwnerUserID: c.f.ownerUserID, TransactionID: c.parent, Reason: "Correct parent", GainImpactAcknowledgement: ack}
		if preview {
			return s.ReverseSplitReconciliationImpact(ctx, input)
		}
		_, e := s.ReverseSplit(ctx, input)
		return ReconciliationImpact{}, e
	case "share_exchange":
		input := reverseShareExchangeInput(c.f, c.parent)
		input.GainImpactAcknowledgement = ack
		if preview {
			return s.ReverseShareExchangeReconciliationImpact(ctx, input)
		}
		_, e := s.ReverseShareExchange(ctx, input)
		return ReconciliationImpact{}, e
	case "spin_off":
		input := reverseSpinOffInput(c.f, c.parent)
		input.GainImpactAcknowledgement = ack
		if preview {
			return s.ReverseSpinOffReconciliationImpact(ctx, input)
		}
		_, e := s.ReverseSpinOff(ctx, input)
		return ReconciliationImpact{}, e
	default:
		input := reverseCashStockMergerInput(c.f, c.parent)
		input.GainImpactAcknowledgement = ack
		if preview {
			return s.ReverseCashStockMergerReconciliationImpact(ctx, input)
		}
		_, e := s.ReverseCashStockMerger(ctx, input)
		return ReconciliationImpact{}, e
	}
}

func TestCashInLieuAllParentsReplaceByActualReplayAndRetainProvenance(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, false)
			f := c.f
			ctx := context.Background()
			cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.NoError(t, err)
			before := buyReplacementPreviewSnapshot(t, f.database)
			_, impact, err := c.replace("2026-06-01", 5, 2, "", true)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			require.NotNil(t, impact.GainImpact)
			_, _, err = c.replace("2026-06-01", 5, 2, "", false)
			require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			_, _, err = c.replace("2026-06-01", 5, 2, "stale", false)
			require.ErrorIs(t, err, ErrGainImpactAcknowledgementStale)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			_, err = f.database.Exec(`CREATE TRIGGER reject_parent_replay BEFORE INSERT ON investment_disposal_revisions BEGIN SELECT RAISE(ABORT,'forced parent replay rollback'); END`)
			require.NoError(t, err)
			_, _, err = c.replace("2026-06-01", 5, 2, impact.GainImpact.Acknowledgement, false)
			require.ErrorContains(t, err, "forced parent replay rollback")
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			_, err = f.database.Exec(`DROP TRIGGER reject_parent_replay`)
			require.NoError(t, err)
			replacement, _, err := c.replace("2026-06-01", 5, 2, impact.GainImpact.Acknowledgement, false)
			require.NoError(t, err)
			quantity, basis := openCommodityPosition(t, f, f.holdingAccountID, c.commodity)
			requireScaled(t, 7, 0, quantity, "same-day replacement retains parent root before the fraction")
			expected := map[string]int64{"split": 240, "share_exchange": 240, "spin_off": 48, "cash_and_stock_merger": 180}[kind]
			requireScaled(t, expected, 2, saleEffectiveBasis(t, f, cil.Transaction.ID), "independently derived disposed basis")
			total := map[string]int64{"split": 3600, "share_exchange": 3600, "spin_off": 720, "cash_and_stock_merger": 2700}[kind]
			requireScaled(t, total-expected, 2, basis, "basis is conserved")
			context, err := f.investmentService.repository.TradeCorrectionContext(ctx, BookID, cil.Transaction.ID)
			require.NoError(t, err)
			require.Equal(t, transferOperationID(t, f, c.parent), context.SettledOperationID)
			require.Equal(t, transferOperationID(t, f, replacement), context.EffectiveSettledOperationID)
			if kind == "split" {
				rows, err := f.investmentService.repository.SplitActivity(ctx, BookID, 0, 20, context.EffectiveSettledOperationID)
				require.NoError(t, err)
				require.Len(t, rows, 1)
				require.True(t, rows[0].Settled, "current split activity resolves original settlement ancestry")
			}
			facts := exportedBundle(t, f)["investment-cash-in-lieu-facts.csv"]
			require.Len(t, facts, 1)
			require.Equal(t, strconv.FormatInt(context.SettledOperationID, 10), facts[0]["settled_operation_id"])
			require.Equal(t, "2026-06-05", facts[0]["payment_on"])
			lots, err := f.investmentService.CashInLieuAvailableLots(ctx, f.ownerUserID, c.parent, f.eurCommodityID, "2026-06-01", cil.Transaction.ID)
			require.NoError(t, err)
			require.NotEmpty(t, lots)
			chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, cil.Transaction.ID)
			require.NoError(t, err)
			require.Equal(t, SelfCheckPassed, chain.CashInLieuAttribution.Status)
			// Correction after parent replacement keeps the historical source reference.
			repair := cashInLieuInput(f, c.parent)
			repair.ProceedsValue = 900
			corrected, impact, err := f.investmentService.PreviewCashInLieuReplacement(ctx, ReplaceCashInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: cil.Transaction.ID, Reason: "Correct proceeds", Replacement: repair})
			require.NoError(t, err)
			require.NotEmpty(t, corrected.Allocations)
			input := ReplaceCashInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: cil.Transaction.ID, Reason: "Correct proceeds", Replacement: repair}
			if impact.GainImpact != nil {
				input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
			}
			_, err = f.investmentService.ReplaceCashInLieu(ctx, input)
			require.NoError(t, err)
			requireSelfCheckPasses(t, f)
		})
	}
}

func TestCashInLieuAllParentsRefuseInsufficientUnitsWithoutWriting(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, false)
			f := c.f
			cil, err := f.investmentService.CashInLieu(context.Background(), cashInLieuInput(f, c.parent))
			require.NoError(t, err)
			before := buyReplacementPreviewSnapshot(t, f.database)
			for _, preview := range []bool{true, false} {
				_, _, err := c.replace("2026-06-01", 1, 10, "", preview)
				require.Error(t, err)
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "0.3 units cannot satisfy 0.5")
				require.Contains(t, err.Error(), "operation")
			}
			var stillEffective int
			require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM effective_investment_operations WHERE id=?`, transferOperationID(t, f, cil.Transaction.ID)).Scan(&stillEffective))
			require.Equal(t, 1, stillEffective)
		})
	}
}

func TestCashInLieuAllParentsReverseWhenOtherUnitsSupportDisposal(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, true)
			f := c.f
			ctx := context.Background()
			input := cashInLieuInput(f, c.parent)
			input.CostBasisMethod = "fifo"
			cil, err := f.investmentService.CashInLieu(ctx, input)
			require.NoError(t, err)
			before := buyReplacementPreviewSnapshot(t, f.database)
			impact, err := c.reverse("", true)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			ack := ""
			if impact.GainImpact != nil {
				ack = impact.GainImpact.Acknowledgement
			}
			_, err = c.reverse(ack, false)
			require.NoError(t, err)
			chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, cil.Transaction.ID)
			require.NoError(t, err)
			require.Equal(t, SelfCheckWarning, chain.CashInLieuAttribution.Status)
			require.Equal(t, []string{"cash_in_lieu_parent_unresolved"}, chain.CashInLieuAttribution.Reasons)
			run := mustRunInvestmentSelfCheck(t, f)
			require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentFoundation).Status)
			require.Equal(t, SelfCheckWarning, resultFor(t, run, CheckInvestmentAttribution).Status)
			// Dated lots and ordinary correction are still available with a retired parent.
			lots, err := f.investmentService.CashInLieuAvailableLots(ctx, f.ownerUserID, c.parent, f.eurCommodityID, "2026-06-01", cil.Transaction.ID)
			require.NoError(t, err)
			require.NotEmpty(t, lots)
			input.ProceedsValue = 900
			repair := ReplaceCashInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: cil.Transaction.ID, Reason: "Correct cash", Replacement: input}
			_, review, err := f.investmentService.PreviewCashInLieuReplacement(ctx, repair)
			require.NoError(t, err)
			if review.GainImpact != nil {
				repair.GainImpactAcknowledgement = review.GainImpact.Acknowledgement
			}
			_, err = f.investmentService.ReplaceCashInLieu(ctx, repair)
			require.NoError(t, err)
		})
	}
}

func TestCashInLieuParentReversalRetainsSpecificLotAndLaterSaleDependencies(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, true)
			f := c.f
			ctx := context.Background()
			input := cashInLieuInput(f, c.parent)
			var blocked int64
			if kind != "split" {
				var lotID int64
				require.NoError(t, f.database.QueryRow(`SELECT id FROM investment_lots WHERE operation_id=? AND commodity_id=?`, transferOperationID(t, f, c.parent), c.commodity).Scan(&lotID))
				input.CostBasisMethod = "specific_lot"
				input.LotAllocations = []InvestmentLotAllocationInput{{LotID: lotID, QuantityValue: exact.New(5), QuantityScale: 1}}
			}
			cil, err := f.investmentService.CashInLieu(ctx, input)
			require.NoError(t, err)
			blocked = transferOperationID(t, f, cil.Transaction.ID)
			if kind == "split" {
				sold, err := f.investmentService.Sell(ctx, sellInput(f, "2026-06-10", 4))
				require.NoError(t, err)
				blocked = transferOperationID(t, f, sold.Transaction.ID)
			}
			before := buyReplacementPreviewSnapshot(t, f.database)
			for _, preview := range []bool{true, false} {
				_, err := c.reverse("", preview)
				if kind == "split" {
					var dependency InvestmentSplitDependencyError
					require.ErrorAs(t, err, &dependency)
					require.Equal(t, blocked, dependency.OperationID)
				} else {
					var dependency InvestmentTransferDependencyError
					require.ErrorAs(t, err, &dependency)
					require.Equal(t, blocked, dependency.OperationID)
				}
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "other lots do not authorize replacing an elected lot")
			}
		})
	}
}

func TestCashInLieuParentDateAndAncestryAreAdvisoryWhileCorruptionStillFails(t *testing.T) {
	t.Parallel()
	c := newCashParentCase(t, "split", false)
	f := c.f
	ctx := context.Background()
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
	require.NoError(t, err)
	// Three original units support the fraction even if the split is later.
	replacement, impact, err := c.replace("2026-06-02", 3, 2, "", true)
	require.NoError(t, err)
	_ = replacement
	ack := ""
	if impact.GainImpact != nil {
		ack = impact.GainImpact.Acknowledgement
	}
	replacement, _, err = c.replace("2026-06-02", 3, 2, ack, false)
	require.NoError(t, err)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, cil.Transaction.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"cash_in_lieu_parent_mismatch"}, chain.CashInLieuAttribution.Reasons)
	c.parent = replacement
	_, impact, err = c.replace("2026-06-01", 5, 2, "", true)
	require.NoError(t, err)
	ack = ""
	if impact.GainImpact != nil {
		ack = impact.GainImpact.Acknowledgement
	}
	replacement, _, err = c.replace("2026-06-01", 5, 2, ack, false)
	require.NoError(t, err)
	source, err := f.investmentService.repository.TradeCorrectionContext(ctx, BookID, cil.Transaction.ID)
	require.NoError(t, err)
	require.Equal(t, transferOperationID(t, f, replacement), source.EffectiveSettledOperationID)
	requireSelfCheckPasses(t, f)
	// Query-only diagnostics never require simulation writes.
	_, err = f.database.Exec(`PRAGMA query_only=ON`)
	require.NoError(t, err)
	diagnostic, err := f.investmentService.repository.CashInLieuAttribution(ctx, BookID, cil.Transaction.ID)
	require.NoError(t, err)
	require.Empty(t, diagnostic.WarningReasons)
	_, err = f.database.Exec(`PRAGMA query_only=OFF`)
	require.NoError(t, err)
	_, err = f.database.Exec(`DROP TRIGGER investment_cash_in_lieu_facts_no_update`)
	require.NoError(t, err)
	_, err = f.database.Exec(`UPDATE investment_cash_in_lieu_facts SET settled_operation_id=operation_id`)
	require.NoError(t, err)
	run := mustRunInvestmentSelfCheck(t, f)
	require.Equal(t, SelfCheckFailed, resultFor(t, run, CheckInvestmentFoundation).Status)
	require.Zero(t, resultFor(t, run, CheckInvestmentAttribution).WarningCount, "broken facts are not disguised as attribution warnings")
}

func TestCashInLieuParentReplayStillBindsReconciledHoldingReview(t *testing.T) {
	t.Parallel()
	c := newCashParentCase(t, "split", false)
	f := c.f
	ctx := context.Background()
	_, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
	require.NoError(t, err)
	reconcileInstrument(t, f, c.commodity, "2026-06-05", 4)
	input := replaceSplitInput(f, c.parent, "2026-06-01", 5, 2)
	preview, err := f.investmentService.PreviewSplitReplacement(ctx, input)
	require.NoError(t, err)
	require.Len(t, preview.Impact.AffectedCheckpoints, 1)
	input.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceSplit(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{987654}}
	_, err = f.investmentService.ReplaceSplit(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationImpactStale)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{preview.Impact.AffectedCheckpoints[0].CheckpointID}}
	_, err = f.investmentService.ReplaceSplit(ctx, input)
	require.NoError(t, err)
	require.Empty(t, activeCheckpointIDs(t, f))
	requireSelfCheckPasses(t, f)
}

func TestCashInLieuAllParentCorrectionsReplayLaterDisposalsInRootOrder(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, false)
			f := c.f
			ctx := context.Background()
			cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.NoError(t, err)
			input := sellInput(f, "2026-06-01", 1)
			input.CommodityID = c.commodity
			input.CostBasisMethod = "fifo"
			sold, err := f.investmentService.Sell(ctx, input)
			require.NoError(t, err)
			before := buyReplacementPreviewSnapshot(t, f.database)
			_, impact, err := c.replace("2026-06-01", 5, 2, "", true)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			require.GreaterOrEqual(t, len(impact.GainImpact.Changes), 2)
			_, _, err = c.replace("2026-06-01", 5, 2, impact.GainImpact.Acknowledgement, false)
			require.NoError(t, err)
			halfBasis := map[string]int64{"split": 240, "share_exchange": 240, "spin_off": 48, "cash_and_stock_merger": 180}[kind]
			total := map[string]int64{"split": 3600, "share_exchange": 3600, "spin_off": 720, "cash_and_stock_merger": 2700}[kind]
			requireScaled(t, halfBasis, 2, saleEffectiveBasis(t, f, cil.Transaction.ID), "fraction keeps its earlier same-day slot")
			requireScaled(t, 2*halfBasis, 2, saleEffectiveBasis(t, f, sold.Transaction.ID), "later whole unit uses corrected basis")
			quantity, basis := openCommodityPosition(t, f, f.holdingAccountID, c.commodity)
			requireScaled(t, 6, 0, quantity, "7.5 less 0.5 less 1")
			requireScaled(t, total-3*halfBasis, 2, basis, "remaining plus both disposals conserve basis")
			requireSelfCheckPasses(t, f)
		})
	}
}

func TestCashInLieuCurrentParentInstrumentMayDifferWhenActualUnitsRemainValid(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, true)
			f := c.f
			ctx := context.Background()
			originalCommodity := c.commodity
			input := cashInLieuInput(f, c.parent)
			input.CostBasisMethod = "fifo"
			cil, err := f.investmentService.CashInLieu(ctx, input)
			require.NoError(t, err)
			c.commodity = seedTestSecurityCommodity(t, f, "REVISED_DESTINATION")
			_, impact, err := c.replace("2026-06-01", 5, 2, "", true)
			require.NoError(t, err)
			ack := ""
			if impact.GainImpact != nil {
				ack = impact.GainImpact.Acknowledgement
			}
			_, _, err = c.replace("2026-06-01", 5, 2, ack, false)
			require.NoError(t, err)
			chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, cil.Transaction.ID)
			require.NoError(t, err)
			require.Equal(t, []string{"cash_in_lieu_parent_mismatch"}, chain.CashInLieuAttribution.Reasons)
			quantity, basis := openCommodityPosition(t, f, f.holdingAccountID, originalCommodity)
			requireScaled(t, 15, 1, quantity, "independent 2 units less the original half-unit disposal")
			requireScaled(t, 1500, 2, basis, "independent basis 20 less disposed 5")
			run := mustRunInvestmentSelfCheck(t, f)
			require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentFoundation).Status)
			require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentReplay).Status)
		})
	}
}

// #394: entry and correction use the same actual dated disposal, regardless
// of the historical parent's standing. Expected quantities and basis below
// are hand-derived, rather than taken from the replay implementation.
func TestCashInLieuHistoricalParentEntryAllFamilies(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		for _, reversed := range []bool{false, true} {
			name := kind + "/replaced"
			if reversed {
				name = kind + "/reversed"
			}
			t.Run(name, func(t *testing.T) {
				c := newCashParentCase(t, kind, reversed)
				f, ctx := c.f, context.Background()
				if reversed {
					impact, err := c.reverse("", true)
					require.NoError(t, err)
					ack := ""
					if impact.GainImpact != nil {
						ack = impact.GainImpact.Acknowledgement
					}
					_, err = c.reverse(ack, false)
					require.NoError(t, err)
				} else {
					_, _, err := c.replace("2026-06-01", 5, 2, "", false)
					require.NoError(t, err)
				}
				before := buyReplacementPreviewSnapshot(t, f.database)
				// Both public identity shapes must read the actual historical scope.
				parentID := transferOperationID(t, f, c.parent)
				lots, err := f.investmentService.CashInLieuAvailableLotsForOperation(ctx, f.ownerUserID, parentID, f.eurCommodityID, "2026-06-01", 0)
				require.NoError(t, err)
				require.Len(t, lots, 1)
				byJournal, err := f.investmentService.CashInLieuAvailableLots(ctx, f.ownerUserID, c.parent, f.eurCommodityID, "2026-06-01", 0)
				require.NoError(t, err)
				require.Equal(t, lots, byJournal)
				input := cashInLieuInput(f, 0)
				input.SettledOperationID, input.CostBasisMethod = parentID, "specific_lot"
				input.LotAllocations = []InvestmentLotAllocationInput{{LotID: lots[0].LotID, QuantityValue: exact.New(5), QuantityScale: 1}}
				preview, _, err := f.investmentService.PreviewCashInLieu(ctx, input)
				require.NoError(t, err)
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "lots and preview leave no durable changes")
				result, err := f.investmentService.CashInLieu(ctx, input)
				require.NoError(t, err)
				require.Equal(t, preview.Allocations, result.Allocations)
				disposed := map[string]int64{"split": 240, "share_exchange": 240, "spin_off": 48, "cash_and_stock_merger": 180}[kind]
				totalBasis := map[string]int64{"split": 3600, "share_exchange": 3600, "spin_off": 720, "cash_and_stock_merger": 2700}[kind]
				remainingUnits, scale := int64(7), 0
				expectedCash := int64(-2800)
				if kind == "cash_and_stock_merger" {
					expectedCash += 300
				}
				if reversed {
					disposed, totalBasis, remainingUnits, scale = 600, 3600, 25, 1
					if kind != "split" {
						disposed, totalBasis, remainingUnits = 500, 2000, 15
						expectedCash -= 2000
					}
					if kind == "cash_and_stock_merger" {
						expectedCash -= 300
					}
				}
				quantity, basis := openCommodityPosition(t, f, f.holdingAccountID, c.commodity)
				requireScaled(t, remainingUnits, scale, quantity, "actual remaining units")
				requireScaled(t, totalBasis-disposed, 2, basis, "basis conservation")
				requireScaled(t, disposed, 2, saleEffectiveBasis(t, f, result.Transaction.ID), "actual disposal basis")
				requireScaled(t, 800-disposed, 2, exact.ScaledIntFromCoefficient(preview.RealizedGainValue, preview.RealizedGainScale), "proceeds less disposed basis")
				require.Equal(t, expectedCash, accountBalance(t, f, f.cashAccountID))
				source, err := f.investmentService.repository.TradeCorrectionContext(ctx, BookID, result.Transaction.ID)
				require.NoError(t, err)
				require.Equal(t, parentID, source.SettledOperationID)
				chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, result.Transaction.ID)
				require.NoError(t, err)
				if reversed {
					require.Zero(t, source.EffectiveSettledOperationID)
					require.True(t, chain.CashInLieuAttribution.Unavailable)
					require.Equal(t, []string{"cash_in_lieu_parent_unresolved"}, chain.CashInLieuAttribution.Reasons)
				} else {
					require.NotEqual(t, parentID, source.EffectiveSettledOperationID)
					require.Equal(t, SelfCheckPassed, chain.CashInLieuAttribution.Status)
				}
				facts := exportedBundle(t, f)["investment-cash-in-lieu-facts.csv"]
				require.Len(t, facts, 1)
				require.Equal(t, strconv.FormatInt(parentID, 10), facts[0]["settled_operation_id"])
				require.Equal(t, "2026-06-05", facts[0]["payment_on"])
				run := mustRunInvestmentSelfCheck(t, f)
				require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentFoundation).Status)
			})
		}
	}
}

func TestCashInLieuHistoricalEntryKeepsFinancialRefusalsAndRollsBackLateFailure(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, false)
			f, ctx := c.f, context.Background()
			_, _, err := c.replace("2026-06-01", 1, 6, "", false)
			require.NoError(t, err) // Exactly 0.5 units remain, without a disposal.
			input := cashInLieuInput(f, c.parent)
			before := buyReplacementPreviewSnapshot(t, f.database)
			for _, invalid := range []struct {
				name string
				edit func(*CashInLieuInput)
			}{
				{"insufficient 0.75", func(i *CashInLieuInput) { i.QuantityValue, i.QuantityScale = exact.New(75), 2 }},
				{"whole unit", func(i *CashInLieuInput) { i.QuantityValue, i.QuantityScale = exact.New(1), 0 }},
				{"zero", func(i *CashInLieuInput) { i.QuantityValue = exact.New(0) }},
				{"negative", func(i *CashInLieuInput) { i.QuantityValue = exact.New(-5) }},
				{"original date", func(i *CashInLieuInput) { i.DisposalOn = "2026-05-31" }},
				{"payment date", func(i *CashInLieuInput) { i.PaymentOn = "2026-05-31" }},
				{"invalid election", func(i *CashInLieuInput) {
					i.CostBasisMethod = "specific_lot"
					i.LotAllocations = []InvestmentLotAllocationInput{{LotID: 999999, QuantityValue: exact.New(5), QuantityScale: 1}}
				}},
				{"wrong type", func(i *CashInLieuInput) { i.SettledTransactionID = 1 }},
			} {
				t.Run(invalid.name, func(t *testing.T) {
					bad := input
					invalid.edit(&bad)
					_, _, err := f.investmentService.PreviewCashInLieu(ctx, bad)
					require.Error(t, err)
					_, err = f.investmentService.CashInLieu(ctx, bad)
					require.Error(t, err)
					require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				})
			}
			_, err = f.database.Exec(`CREATE TRIGGER reject_historical_cash_fact BEFORE INSERT ON investment_cash_in_lieu_facts BEGIN SELECT RAISE(ABORT,'late historical entry failure'); END`)
			require.NoError(t, err)
			_, err = f.investmentService.CashInLieu(ctx, input)
			require.ErrorContains(t, err, "late historical entry failure")
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "journal, disposal, lots, observation, source and audit roll back")
			_, err = f.database.Exec(`DROP TRIGGER reject_historical_cash_fact`)
			require.NoError(t, err)
			_, err = f.investmentService.CashInLieu(ctx, input)
			require.NoError(t, err)
			requireInvestmentSelfCheckPasses(t, f)
		})
	}
}

func TestHistoricalCashEntryBindsActualGainAndCheckpointImpact(t *testing.T) {
	t.Parallel()
	f, split := cashInLieuSetup(t)
	ctx := context.Background()
	buyOn(t, f, "2026-05-02", 3, 7200)
	_, err := acknowledgedReverseSplit(ctx, f.investmentService, ReverseInvestmentSplitInput{OwnerUserID: f.ownerUserID, TransactionID: split.Transaction.ID, Reason: "Withdraw parent"})
	require.NoError(t, err)
	sold, err := f.investmentService.Sell(ctx, sellInput(f, "2026-06-10", 4))
	require.NoError(t, err)
	reconcileInstrument(t, f, f.stockCommodityID, "2026-06-10", 2)
	input := cashInLieuInput(f, split.Transaction.ID)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, impact, err := f.investmentService.PreviewCashInLieu(ctx, input)
	require.NoError(t, err)
	require.Len(t, impact.AffectedCheckpoints, 1)
	require.NotNil(t, impact.GainImpact)
	require.Len(t, impact.GainImpact.Changes, 1)
	require.Equal(t, sold.Transaction.ID, impact.GainImpact.Changes[0].TransactionID)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.investmentService.CashInLieu(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	input.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{987654}}
	_, err = f.investmentService.CashInLieu(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationImpactStale)
	input.ReconciliationOverride.Reviewed = []int64{impact.AffectedCheckpoints[0].CheckpointID}
	_, err = f.investmentService.CashInLieu(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	input.QuantityValue, input.QuantityScale = exact.New(25), 2
	_, err = f.investmentService.CashInLieu(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementStale)
	input.QuantityValue, input.QuantityScale = exact.New(5), 1
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "every refused impact leaves all durable rows unchanged")
	_, err = f.investmentService.CashInLieu(ctx, input)
	require.NoError(t, err)
	require.Empty(t, activeCheckpointIDs(t, f))
	requireScaled(t, 6100, 2, saleEffectiveBasis(t, f, sold.Transaction.ID), "2.5 units at 10 and 1.5 units at 24")
}

func TestHistoricalCashEntryStorageKeepsOriginalParentIntegrity(t *testing.T) {
	t.Parallel()
	c := newCashParentCase(t, "share_exchange", true)
	f, ctx := c.f, context.Background()
	_, err := c.reverse("", false)
	require.NoError(t, err)
	input := cashInLieuInput(f, c.parent)
	trade, parentID, payment, err := f.investmentService.cashInLieuTrade(ctx, input)
	require.NoError(t, err)
	other := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	acquisition := tradeOn(f, other, "2026-05-02", 2, 2000)
	acquisition.CommodityID = c.commodity
	_, err = f.investmentService.Buy(ctx, acquisition)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	for _, invalid := range []struct {
		name            string
		bookID, parent  int64
		wrongInstrument bool
		wrongHolding    bool
	}{
		{"missing parent", BookID, 999999, false, false},
		{"wrong book", 2, parentID, false, false},
		{"wrong type", BookID, transferOperationID(t, f, 1), false, false},
		{"wrong instrument", BookID, parentID, true, false},
		{"wrong holding", BookID, parentID, false, true},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			command := trade
			if invalid.wrongInstrument {
				command.CommodityID = f.stockCommodityID
			}
			if invalid.wrongHolding {
				command.HoldingAccountID = other
			}
			_, err := f.investmentService.sell(ctx, command, db.CashInLieuFactWriter(ctx, invalid.bookID, invalid.parent, payment))
			if invalid.bookID != BookID {
				require.Error(t, err)
			} else {
				require.ErrorIs(t, err, db.ErrCashInLieuSplitUnavailable)
			}
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "late original-scope guard rolls back the whole disposal")
		})
	}
}
