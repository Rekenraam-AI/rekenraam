package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

// #191: the operation-kind capability matrix. Every kind the investment
// writers accept (db.InvestmentOperationKinds) has a row, and each of its
// capabilities names the test that exercises it, the test that asserts its
// named refusal, or the issue that tracks the gap. Cells hold test functions,
// not names, so a renamed or deleted test fails to compile here.
//
// Columns:
//   - replay: the kind recorded behind a later depletion is admitted and
//     replays it, or is refused with the blocking operation named;
//   - replacement and reversal: the kind's correction commands;
//   - export: the bundle carries the operation's rows (checked contents);
//   - self-check: every investment self-check and the conservation oracle
//     pass with the kind recorded.
//
// Operation kinds and replay-intent families are separate axes: several kinds
// share an intent family, so the second table below covers each family.

type capabilityMode int

const (
	capabilityExercised capabilityMode = iota + 1
	capabilityRefused
	// capabilityLotFree is a replay cell for a kind with no lot effect:
	// TestEveryOperationKindExportsItsRowsAndPassesSelfCheck asserts the
	// operation writes no lot facts, lot effects or disposal decisions.
	capabilityLotFree
	capabilityGap
)

type capability struct {
	mode  capabilityMode
	test  func(*testing.T)
	issue int
}

func exercised(test func(*testing.T)) capability {
	return capability{mode: capabilityExercised, test: test}
}
func refused(test func(*testing.T)) capability {
	return capability{mode: capabilityRefused, test: test}
}
func lotFree() capability {
	return capability{mode: capabilityLotFree, test: TestEveryOperationKindExportsItsRowsAndPassesSelfCheck}
}
func gap(issue int) capability { return capability{mode: capabilityGap, issue: issue} }

type operationKindCapabilities struct {
	kind string
	// exportedIn are the bundle files, besides investment-operations.csv,
	// whose operation_id column must name the operation.
	exportedIn  []string
	replay      capability
	replacement capability
	reversal    capability
	export      capability
	selfCheck   capability
}

const (
	lotFactsFile          = "investment-lot-facts.csv"
	disposalDecisionsFile = "disposal-decisions.csv"
	transferFactsFile     = "investment-transfer-facts.csv"
)

var operationKindCapabilityMatrix []operationKindCapabilities

// init builds the matrix: its cells name the test that reads it, which a
// package-level initializer cannot.
func init() {
	allKinds := exercised(TestEveryOperationKindExportsItsRowsAndPassesSelfCheck)
	operationKindCapabilityMatrix = []operationKindCapabilities{
		{kind: "buy", exportedIn: []string{lotFactsFile},
			replay:      exercised(TestBackdatedPurchaseReplaysLaterAverageCostSale),
			replacement: exercised(TestReplaceOldManualBuyReplaysDependentSameDaySale),
			reversal:    exercised(TestReverseManualBuyReplaysDependentLIFOSaleAndKeepsHistory),
			export:      exercised(TestInvestmentOperationExportAndImportEffectsUsePrimaryJournalLinks),
			selfCheck:   allKinds},
		{kind: "sell", exportedIn: []string{disposalDecisionsFile},
			replay:      exercised(TestBackdatedDisposalBehindLaterSaleUnderEveryMethod),
			replacement: exercised(TestReplaceOlderManualSaleReplaysDependentSaleAndRefusesImpossibleCorrection),
			reversal:    exercised(TestReverseManualSaleReplaysLaterSaleAndKeepsOriginalHistory),
			export:      allKinds,
			selfCheck:   allKinds},
		{kind: "write_off", exportedIn: []string{disposalDecisionsFile},
			replay:      exercised(TestBackdatedDisposalBehindLaterSaleUnderEveryMethod),
			replacement: exercised(TestReplacePartialWriteOffReplaysDependentSale),
			reversal:    exercised(TestReverseFullWriteOffRestoresPosition),
			export:      exercised(TestWriteOffCorrectionsExportAsOperations),
			selfCheck:   allKinds},
		{kind: "cash_in_lieu", exportedIn: []string{disposalDecisionsFile, "investment-cash-in-lieu-facts.csv"},
			replay:      exercised(TestCashInLieuBackdatedBehindALaterSaleRevisesIt),
			replacement: exercised(TestReplaceCashInLieuKeepsItsSplitAndPreviewMatchesCommit),
			reversal:    exercised(TestReverseCashInLieuRestoresFractionAndFreesSplit),
			export:      allKinds,
			selfCheck:   allKinds},
		{kind: "dividend",
			replay:      lotFree(),
			replacement: exercised(TestReplaceDividendCorrectsAmountAndWithholding),
			reversal:    exercised(TestReverseDividendRequiresReconciliationOverride),
			export:      exercised(TestDividendAndReinvestmentCorrectionsExportAsOperations),
			selfCheck:   allKinds},
		{kind: "reinvested_dividend", exportedIn: []string{lotFactsFile},
			replay:      exercised(TestReinvestmentGainImpactRequiresAcknowledgement),
			replacement: exercised(TestReplaceReinvestedDividendAmountAndQuantityReplaysDependentSale),
			reversal:    exercised(TestReverseReinvestedDividendReplaysDependentSale),
			export:      exercised(TestDividendAndReinvestmentCorrectionsExportAsOperations),
			selfCheck:   allKinds},
		{kind: "return_of_capital", exportedIn: []string{"investment-capital-return-facts.csv", "investment-capital-return-effects.csv"},
			replay:      exercised(TestBackdatedCapitalReturnRevisesLaterSaleUnderAcknowledgement),
			replacement: exercised(TestCapitalReturnReplacementMovesEffectiveAndPaymentDatesThenReverses),
			reversal:    exercised(TestReverseCapitalReturnRestoresBasisAndRevisesLaterSale),
			export:      exercised(TestCapitalReturnBundlePreservesOriginalReplacementAndQuantityRevision),
			selfCheck:   allKinds},
		// Journal-free admission shares the split kind; its operation-addressed
		// correction/settlement surface is tracked separately in #196.
		{kind: "split", exportedIn: []string{"investment-split-facts.csv"},
			replay:      exercised(TestBackdatedAcquisitionEntitledToJournalFreeSplitPostsAdjustment),
			replacement: exercised(TestSplitReplacementCorrectsRatioAndRevisesLaterGain),
			reversal:    exercised(TestSplitReversalPostsExactInverseAndRestoresUnsplitHolding),
			export:      exercised(TestJournalFreeSplitExportsAndSurvivesRestore),
			selfCheck:   exercised(TestJournalFreeSplitExemptionDoesNotHideMissingJournals)},
		{kind: "short_sale", exportedIn: []string{lotFactsFile},
			replay:      exercised(TestBackdatedShortOpeningRevisesLaterLIFOCoverWithAcknowledgement),
			replacement: exercised(TestReplaceShortSaleRevisesLaterCoverWithAcknowledgement),
			reversal:    exercised(TestReverseShortSaleNamesTheCoverThatNeedsIt),
			export:      exercised(TestShortOperationsExportTheirSide),
			selfCheck:   allKinds},
		{kind: "short_cover", exportedIn: []string{disposalDecisionsFile},
			replay:      exercised(TestBackdatedShortCoverRevisesLaterFIFOCover),
			replacement: exercised(TestReplaceShortCoverChangesMethodAmountAndDate),
			reversal:    exercised(TestReverseShortCoverRestoresOwedUnits),
			export:      exercised(TestShortOperationsExportTheirSide),
			selfCheck:   allKinds},
		{kind: "share_exchange", exportedIn: []string{transferFactsFile},
			replay:      exercised(TestShareExchangeBackdatedBehindAnOldSaleRevisesIt),
			replacement: exercised(TestShareExchangeReplacementChangesRatioAndInstrument),
			reversal:    exercised(TestShareExchangeReversalRestoresTheOldHoldingAndRetiresTheNewLots),
			export:      exercised(TestShareExchangeExportCarriesInstrumentsAndRatio),
			selfCheck:   allKinds},
		{kind: "spin_off", exportedIn: []string{transferFactsFile},
			replay:      exercised(TestSpinOffBackdatedBehindAParentSaleRevisesIt),
			replacement: exercised(TestSpinOffReplacementChangesFractionRatioAndInstrument),
			reversal:    exercised(TestSpinOffReversalRestoresParentBasisAndRetiresTheNewLots),
			export:      exercised(TestSpinOffExportCarriesTheBasisFraction),
			selfCheck:   allKinds},
		{kind: "cash_and_stock_merger", exportedIn: []string{transferFactsFile, disposalDecisionsFile},
			replay:      exercised(TestCashStockMergerBackdatedBehindAnOldHoldingSaleRevisesIt),
			replacement: exercised(TestCashStockMergerReplacementRestatesTheCashResultWithAcknowledgement),
			reversal:    exercised(TestCashStockMergerReversalReturnsOldLotsAndRemovesTheCashResult),
			export:      exercised(TestCashStockMergerExportCarriesElectionDatesAndDecision),
			selfCheck:   allKinds},
		{kind: "internal_transfer", exportedIn: []string{transferFactsFile, "investment-transfer-lot-links.csv"},
			replay:      exercised(TestBackdatedInternalTransferReplaysLaterSourceSale),
			replacement: exercised(TestReplaceInternalTransferQuantityReplaysDestinationSale),
			reversal:    exercised(TestReverseInternalTransferRestoresSourceAndRemovesDestinationLot),
			export:      exercised(TestTransferCorrectionsExportWithOriginalsAndCorrectionLinks),
			selfCheck:   allKinds},
		{kind: "external_transfer_in", exportedIn: []string{transferFactsFile, lotFactsFile},
			replay:      exercised(TestBackdatedTransferInFromOldBrokerReplaysCurrentBrokerSaleByOriginalDate),
			replacement: exercised(TestReplaceExternalTransferInCorrectsBasisAndAppendsBridge),
			reversal:    exercised(TestReverseExternalTransferInRetiresLotAndInvertsBridge),
			export:      exercised(TestTransferCorrectionsExportWithOriginalsAndCorrectionLinks),
			selfCheck:   allKinds},
		{kind: "external_transfer_out", exportedIn: []string{transferFactsFile, "investment-transfer-lot-links.csv"},
			replay:      exercised(TestBackdatedExternalTransferOutReplaysLaterSale),
			replacement: exercised(TestReplaceExternalTransferOutRebridgesCorrectedQuantityAtItsSlot),
			reversal:    exercised(TestReverseExternalTransferOutRestoresUnitsAndCancelsNetBridge),
			export:      allKinds,
			selfCheck:   allKinds},
		{kind: "basis_resolution", exportedIn: []string{"investment-basis-resolutions.csv"},
			replay:      exercised(TestResolvingUnknownInboundAfterPartialSalePostsFullBridgeAndResolvesBoth),
			replacement: exercised(TestReplacingResolutionRevisesPartialSaleAndKeepsEvidence),
			reversal:    exercised(TestReversingResolutionRestoresUnknownBasisAndUnresolvedSale),
			export:      exercised(TestKnownZeroResolutionExportsAndSurvivesRestore),
			selfCheck:   allKinds},
		{kind: "reversal",
			replay:      exercised(TestReverseManualSaleReplaysLaterSaleAndKeepsOriginalHistory),
			replacement: refused(TestReversalOperationIsNotItselfCorrectable),
			reversal:    refused(TestReversalOperationIsNotItselfCorrectable),
			export:      exercised(TestDividendAndReinvestmentCorrectionsExportAsOperations),
			selfCheck:   allKinds},
	}
}

// replayIntentCapabilities is the second axis: each family the replay engine
// applies (the cases of applyInvestmentReplayIntentTx) and a test that
// replays it behind a later depletion.
var replayIntentCapabilities = map[string]func(*testing.T){
	"opening":                 TestBackdatedPurchaseReplaysLaterAverageCostSale,
	"disposal":                TestBackdatedDisposalBehindLaterSaleUnderEveryMethod,
	"transfer_out":            TestBackdatedInternalTransferReplaysLaterSourceSale,
	"pooled_transfer_out":     TestBackdatedPooledInternalTransferReplaysLaterAverageSale,
	"pooled_lot_transfer_out": TestBackdatedBuyRevisesPooledLotTransferAndDestinationFIFOSale,
	"exchange_out":            TestShareExchangeBackdatedBehindAnOldSaleRevisesIt,
	"spin_off":                TestSpinOffBackdatedBehindAParentSaleRevisesIt,
	"split":                   TestBackdatedSplitReplaysLaterSaleBehindGainAcknowledgement,
	"capital_return":          TestBackdatedCapitalReturnRevisesLaterSaleUnderAcknowledgement,
}

// testName is the top-level test a cell names; closures and helpers are not
// tests the runner executes.
func testName(t *testing.T, test func(*testing.T)) string {
	t.Helper()
	require.NotNil(t, test)
	name := runtime.FuncForPC(reflect.ValueOf(test).Pointer()).Name()
	name, found := strings.CutPrefix(name, "rekenraam/backend/internal/app.")
	require.True(t, found, "%s is not in the app package", name)
	require.True(t, strings.HasPrefix(name, "Test") && !strings.Contains(name, "."), "%s is not a top-level test", name)
	return name
}

func TestOperationKindCapabilityMatrixCoversEveryAcceptedKind(t *testing.T) {
	t.Parallel()
	kinds := make([]string, 0, len(operationKindCapabilityMatrix))
	for _, row := range operationKindCapabilityMatrix {
		require.NotContains(t, kinds, row.kind, "one row per operation kind")
		kinds = append(kinds, row.kind)
		cells := map[string]capability{"replay": row.replay, "replacement": row.replacement,
			"reversal": row.reversal, "export": row.export, "self-check": row.selfCheck}
		for column, cell := range cells {
			switch cell.mode {
			case capabilityExercised, capabilityRefused:
				testName(t, cell.test)
			case capabilityLotFree:
				require.Equal(t, "replay", column, "%s %s: only replay can be lot-free", row.kind, column)
				require.Empty(t, row.exportedIn, "%s: a lot-free kind exports no lot or disposal rows", row.kind)
			case capabilityGap:
				require.Positive(t, cell.issue, "%s %s: a gap names the issue that tracks it", row.kind, column)
			default:
				require.Failf(t, "empty cell", "%s %s has no test, refusal or issue", row.kind, column)
			}
		}
	}
	// A kind a writer accepts without a row fails here; so does a stale row.
	require.ElementsMatch(t, db.InvestmentOperationKinds, kinds)
}

func TestReplayIntentCapabilitiesCoverEveryReplayedFamily(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "../db/investment_replay_simulation.go", nil, 0)
	require.NoError(t, err)
	var families []string
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name.Name != "applyInvestmentReplayIntentTx" {
			return true
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			selection, ok := node.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			if tag, ok := selection.Tag.(*ast.SelectorExpr); !ok || tag.Sel.Name != "Kind" {
				return true
			}
			for _, statement := range selection.Body.List {
				for _, expression := range statement.(*ast.CaseClause).List {
					literal, ok := expression.(*ast.BasicLit)
					require.True(t, ok, "intent cases are string literals")
					family, err := strconv.Unquote(literal.Value)
					require.NoError(t, err)
					families = append(families, family)
				}
			}
			return false
		})
		return false
	})
	require.NotEmpty(t, families, "the replay intent switch was not found")
	covered := make([]string, 0, len(replayIntentCapabilities))
	for family, test := range replayIntentCapabilities {
		testName(t, test)
		covered = append(covered, family)
	}
	require.ElementsMatch(t, families, covered)
}
