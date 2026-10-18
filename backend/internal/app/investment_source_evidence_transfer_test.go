package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #312: transfer, transfer-in, capital-return and split corrections apply
// the #311 contract. A replacement that omits evidence keeps the recorded
// tokens, source_evidence_edits change only the keys they name, and the
// original's evidence stays as recorded.

// operationFactsEvidence reads the stored evidence of a transaction's
// operation from its facts table.
func operationFactsEvidence(t *testing.T, f *investmentsTestFixture, table string, transactionID int64) string {
	t.Helper()
	var evidence string
	require.NoError(t, f.database.QueryRow(`SELECT facts.source_evidence_json FROM `+table+` facts
		JOIN investment_operation_journal_links link ON link.operation_id = facts.operation_id AND link.role = 'primary'
		JOIN transaction_versions version ON version.id = link.transaction_version_id
		WHERE version.transaction_id = ?`, transactionID).Scan(&evidence))
	return evidence
}

func assertExactUneditedEvidence(t *testing.T, evidence string) {
	t.Helper()
	assert.Contains(t, evidence, `"revision":9007199254740993`)
	assert.Contains(t, evidence, `"ratio":0.123456789012345678901234567890`)
	assert.Contains(t, evidence, `"id":18446744073709551617`)
}

func TestInternalTransferReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	destinationID := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	buy := buyOn(t, f, "2026-05-01", 3, 3000)
	input := selectedLotMove(f, destinationID, *buy.LotID, "2026-06-01", 2)
	input.SourceEvidenceJSON = exactSourceEvidence
	transfer, err := f.investmentService.InternalTransfer(ctx, input)
	require.NoError(t, err)

	// The form omits evidence: the quantity changes, every token survives.
	replacement := selectedLotMove(f, destinationID, *buy.LotID, "2026-06-01", 3)
	replacement.SourceEvidenceJSON = ""
	replace := replaceTransferInput(f, transfer.Transaction.ID, replacement)
	_, err = f.investmentService.PreviewTransferReplacement(ctx, replace)
	require.NoError(t, err)
	result, err := acknowledgedReplaceTransfer(ctx, f.investmentService, replace)
	require.NoError(t, err)
	assert.Equal(t, exactSourceEvidence, transferFactsEvidence(t, f, result.Replacement.Transaction.ID))
	assert.Equal(t, exactSourceEvidence, transferFactsEvidence(t, f, transfer.Transaction.ID), "the original stays as recorded")

	// Edits apply to the successor's tokens; both forms at once are refused.
	edit := replaceTransferInput(f, result.Replacement.Transaction.ID, replacement)
	edit.SourceEvidenceEditsJSON = `{"reference":"amended notice","ex_date":null}`
	both := edit
	both.Replacement.SourceEvidenceJSON = `{"reference":"amended notice"}`
	_, err = f.investmentService.PreviewTransferReplacement(ctx, both)
	assert.ErrorAs(t, err, &ValidationError{})
	edited, err := acknowledgedReplaceTransfer(ctx, f.investmentService, edit)
	require.NoError(t, err)
	evidence := transferFactsEvidence(t, f, edited.Replacement.Transaction.ID)
	assertExactUneditedEvidence(t, evidence)
	assert.Contains(t, evidence, `"reference":"amended notice"`)
	assert.NotContains(t, evidence, `"ex_date"`)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestTransferOutReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newTransferOutFixture(t)
	ctx := context.Background()
	buy := buyOn(t, f, "2026-05-01", 4, 4000)
	input := transferOutOfLot(f, "2026-06-01", *buy.LotID, 2)
	input.SourceEvidenceJSON = exactSourceEvidence
	out, err := f.investmentService.ExternalTransferOut(ctx, input)
	require.NoError(t, err)

	replacement := transferOutOfLot(f, "2026-06-01", *buy.LotID, 3)
	replacement.SourceEvidenceJSON = ""
	replace := ReplaceInvestmentTransferOutInput{OwnerUserID: f.ownerUserID, TransactionID: out.Transaction.ID,
		Reason: "statement shows three shares", Replacement: replacement,
		SourceEvidenceEditsJSON: `{"reference":"amended notice"}`}
	_, err = f.investmentService.PreviewTransferOutReplacement(ctx, replace)
	require.NoError(t, err)
	result, err := f.investmentService.ReplaceTransferOut(ctx, replace)
	require.NoError(t, err)
	evidence := transferFactsEvidence(t, f, result.Replacement.Transaction.ID)
	assertExactUneditedEvidence(t, evidence)
	assert.Contains(t, evidence, `"ex_date":"2026-05-29"`)
	assert.Contains(t, evidence, `"reference":"amended notice"`)
	assert.Equal(t, exactSourceEvidence, transferFactsEvidence(t, f, out.Transaction.ID))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestTransferInReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newTransferOutFixture(t)
	ctx := context.Background()
	input := knownTransferInput(f)
	input.SourceEvidenceJSON = exactSourceEvidence
	in, err := f.investmentService.ExternalTransferIn(ctx, input)
	require.NoError(t, err)

	replacement := knownTransferInput(f)
	replacement.CarriedBasisValue, replacement.SourceEvidenceJSON = 9000, ""
	replace := replaceTransferInInput(f, in.Transaction.ID, replacement)
	_, err = f.investmentService.ReplaceTransferInReconciliationImpact(ctx, replace)
	require.NoError(t, err)
	result, err := acknowledgedReplaceTransferIn(ctx, f.investmentService, replace)
	require.NoError(t, err)
	assert.Equal(t, exactSourceEvidence, transferFactsEvidence(t, f, result.Replacement.Transaction.ID))

	edit := replaceTransferInInput(f, result.Replacement.Transaction.ID, replacement)
	edit.SourceEvidenceEditsJSON = `{"reference":null,"broker":"statement-43"}`
	edited, err := acknowledgedReplaceTransferIn(ctx, f.investmentService, edit)
	require.NoError(t, err)
	evidence := transferFactsEvidence(t, f, edited.Replacement.Transaction.ID)
	assertExactUneditedEvidence(t, evidence)
	assert.Contains(t, evidence, `"broker":"statement-43"`)
	assert.NotContains(t, evidence, `"reference"`)
	assert.Equal(t, exactSourceEvidence, transferFactsEvidence(t, f, in.Transaction.ID))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestCapitalReturnReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-05-01", 10, 10000)
	input := capitalReturnInput(f, "2026-06-01", 1000)
	input.SourceEvidenceJSON = exactSourceEvidence
	recorded, err := f.investmentService.CapitalReturn(ctx, input)
	require.NoError(t, err)

	// A changed amount with evidence omitted keeps every recorded token.
	replacement := capitalReturnInput(f, "2026-06-01", 1200)
	replacement.SourceEvidenceJSON = ""
	replace := ReplaceInvestmentCapitalReturnInput{OwnerUserID: f.ownerUserID, TransactionID: recorded.Transaction.ID,
		Reason: "issuer restated the amount", Replacement: replacement}
	_, err = f.investmentService.PreviewCapitalReturnReplacement(ctx, replace)
	require.NoError(t, err)
	result, err := f.investmentService.ReplaceCapitalReturn(ctx, replace)
	require.NoError(t, err)
	assert.Equal(t, exactSourceEvidence,
		operationFactsEvidence(t, f, "investment_capital_return_facts", result.Replacement.Transaction.ID))

	// Clearing the reference removes it; the rest stays exact.
	edit := ReplaceInvestmentCapitalReturnInput{OwnerUserID: f.ownerUserID, TransactionID: result.Replacement.Transaction.ID,
		Reason: "reference was wrong", Replacement: replacement, SourceEvidenceEditsJSON: `{"reference":null}`}
	edited, err := f.investmentService.ReplaceCapitalReturn(ctx, edit)
	require.NoError(t, err)
	evidence := operationFactsEvidence(t, f, "investment_capital_return_facts", edited.Replacement.Transaction.ID)
	assertExactUneditedEvidence(t, evidence)
	assert.NotContains(t, evidence, `"reference"`)
	assert.Equal(t, exactSourceEvidence,
		operationFactsEvidence(t, f, "investment_capital_return_facts", recorded.Transaction.ID))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestSplitReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-05-01", 10, 10000)
	input := splitInput(f, "2026-06-01", 2, 1)
	input.SourceEvidenceJSON = exactSourceEvidence
	split, err := f.investmentService.Split(ctx, input)
	require.NoError(t, err)

	// Unchanged terms are a no-op however the evidence arrives: omitted, as
	// an unchanged edit, or as the same object in another key order.
	reordered := `{"nested":{"id":18446744073709551617,"tags":["a",1e400]},"ex_date":"2026-05-29",` +
		`"ratio":0.123456789012345678901234567890,"reference":"issuer notice","revision":9007199254740993}`
	for name, same := range map[string]ReplaceInvestmentSplitInput{
		"omitted": replaceSplitInput(f, split.Transaction.ID, "2026-06-01", 2, 1),
		"same edit": func() ReplaceInvestmentSplitInput {
			i := replaceSplitInput(f, split.Transaction.ID, "2026-06-01", 2, 1)
			i.SourceEvidenceEditsJSON = `{"reference":"issuer notice"}`
			return i
		}(),
		"reordered": func() ReplaceInvestmentSplitInput {
			i := replaceSplitInput(f, split.Transaction.ID, "2026-06-01", 2, 1)
			i.SourceEvidenceJSON = reordered
			return i
		}(),
	} {
		_, err = f.investmentService.PreviewSplitReplacement(ctx, same)
		assert.ErrorAs(t, err, &ValidationError{}, name)
	}

	// A ratio change with evidence omitted keeps the recorded tokens, never {}.
	replace := replaceSplitInput(f, split.Transaction.ID, "2026-06-01", 3, 1)
	result, err := acknowledgedReplaceSplit(ctx, f.investmentService, replace)
	require.NoError(t, err)
	assert.Equal(t, exactSourceEvidence, operationFactsEvidence(t, f, "investment_split_facts", result.Replacement.ID))

	// A reference edit alone is a correction and keeps the other keys.
	edit := replaceSplitInput(f, result.Replacement.ID, "2026-06-01", 3, 1)
	edit.SourceEvidenceEditsJSON = `{"reference":"amended notice"}`
	edited, err := acknowledgedReplaceSplit(ctx, f.investmentService, edit)
	require.NoError(t, err)
	evidence := operationFactsEvidence(t, f, "investment_split_facts", edited.Replacement.ID)
	assertExactUneditedEvidence(t, evidence)
	assert.Contains(t, evidence, `"reference":"amended notice"`)
	assert.Equal(t, exactSourceEvidence, operationFactsEvidence(t, f, "investment_split_facts", split.Transaction.ID))
	requireInvestmentSelfCheckPasses(t, f)
}
