package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

// #311: corporate-action correction forms send only the evidence keys the
// owner edited, and the server applies them to the recorded tokens, so values
// a browser cannot represent exactly survive a correction unedited.

const exactSourceEvidence = `{"reference":"issuer notice","ex_date":"2026-05-29","revision":9007199254740993,` +
	`"ratio":0.123456789012345678901234567890,"nested":{"id":18446744073709551617,"tags":["a",1e400]}}`

func TestCorrectedSourceEvidenceAppliesEditsToRecordedTokens(t *testing.T) {
	t.Parallel()
	merged, err := correctedSourceEvidence(exactSourceEvidence, "", `{"reference":"amended notice","ex_date":null,"page":12}`)
	require.NoError(t, err)
	assert.Equal(t, `{"nested":{"id":18446744073709551617,"tags":["a",1e400]},"page":12,`+
		`"ratio":0.123456789012345678901234567890,"reference":"amended notice","revision":9007199254740993}`, merged)

	kept, err := correctedSourceEvidence(exactSourceEvidence, "", "")
	require.NoError(t, err)
	assert.Equal(t, exactSourceEvidence, kept, "no evidence and no edits keep the recorded evidence")
	whole, err := correctedSourceEvidence(exactSourceEvidence, `{"reference":"new"}`, "")
	require.NoError(t, err)
	assert.Equal(t, `{"reference":"new"}`, whole, "a whole source_evidence still replaces")
	fresh, err := correctedSourceEvidence("{}", "", `{"reference":"r","gone":null}`)
	require.NoError(t, err)
	assert.Equal(t, `{"reference":"r"}`, fresh)

	_, err = correctedSourceEvidence(exactSourceEvidence, `{"reference":"new"}`, `{"reference":"new"}`)
	assert.ErrorAs(t, err, &ValidationError{}, "both forms at once are refused")
	for _, invalid := range []string{`[]`, `"x"`, `null`, `{`} {
		_, err = correctedSourceEvidence(exactSourceEvidence, "", invalid)
		assert.ErrorAs(t, err, &ValidationError{}, invalid)
	}
}

func TestSpinOffReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "SPINCO")
	buyOn(t, f, "2026-02-01", 10, 10000)
	input := spinOffInput(f, newID, "2026-06-01", 1, 1, "1", 1)
	input.SourceEvidenceJSON = exactSourceEvidence
	spun, err := f.investmentService.SpinOff(ctx, input)
	require.NoError(t, err)

	// Unchanged edits with unchanged terms are still a no-op.
	same := replaceSpinOffInput(f, spun.Transaction.ID, newID, "2026-06-01", 1, 1, "1", 1)
	same.SourceEvidenceEditsJSON = `{"reference":"issuer notice"}`
	_, err = f.investmentService.PreviewSpinOffReplacement(ctx, same)
	assert.ErrorAs(t, err, &ValidationError{})

	// Only the allocation changes: every recorded token survives.
	replace := replaceSpinOffInput(f, spun.Transaction.ID, newID, "2026-06-01", 1, 1, "2", 1)
	_, err = f.investmentService.PreviewSpinOffReplacement(ctx, replace)
	require.NoError(t, err)
	result, err := f.investmentService.ReplaceSpinOff(ctx, replace)
	require.NoError(t, err)
	replacementEvidence := transferFactsEvidence(t, f, result.Replacement.ID)
	assert.Contains(t, replacementEvidence, `"revision":9007199254740993`)
	assert.Contains(t, replacementEvidence, `"ratio":0.123456789012345678901234567890`)
	assert.Contains(t, replacementEvidence, `"id":18446744073709551617`)
	assert.Equal(t, exactSourceEvidence, transferFactsEvidence(t, f, spun.Transaction.ID), "the original stays as recorded")

	// An edit and a removal apply to the successor's tokens alone.
	edit := replaceSpinOffInput(f, result.Replacement.ID, newID, "2026-06-01", 1, 1, "2", 1)
	edit.SourceEvidenceEditsJSON = `{"reference":"amended notice","ex_date":null}`
	edited, err := f.investmentService.ReplaceSpinOff(ctx, edit)
	require.NoError(t, err)
	assert.Equal(t, `{"nested":{"id":18446744073709551617,"tags":["a",1e400]},`+
		`"ratio":0.123456789012345678901234567890,"reference":"amended notice","revision":9007199254740993}`,
		transferFactsEvidence(t, f, edited.Replacement.ID))
	requireSpinOffSelfCheckPasses(t, f)

	// The export carries the original and every successor's tokens verbatim.
	snapshot, err := db.NewExportRepository(f.database).Snapshot(ctx)
	require.NoError(t, err)
	var out bytes.Buffer
	_, err = NewExportService(db.NewExportRepository(f.database)).writeInvestmentFoundationCSV(ctx, &out, snapshot,
		"transfer-facts", nil)
	require.NoError(t, err)
	require.NoError(t, snapshot.Rollback())
	reader := csv.NewReader(bytes.NewReader(out.Bytes()))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	require.NoError(t, err)
	var exported []string
	for _, row := range rows {
		exported = append(exported, row...)
	}
	assert.Contains(t, exported, exactSourceEvidence)
	assert.Contains(t, exported, replacementEvidence)
	assert.Contains(t, exported, transferFactsEvidence(t, f, edited.Replacement.ID))
}

func TestCashStockMergerReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 100, 100000)
	input := cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2)
	input.SourceEvidenceJSON = exactSourceEvidence
	merged, err := f.investmentService.CashStockMerger(ctx, input)
	require.NoError(t, err)

	same := replaceCashStockMergerInput(f, merged.Transaction.ID, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2)
	same.SourceEvidenceEditsJSON = `{"reference":"issuer notice"}`
	_, err = f.investmentService.PreviewCashStockMergerReplacement(ctx, same)
	assert.ErrorAs(t, err, &ValidationError{})

	replace := replaceCashStockMergerInput(f, merged.Transaction.ID, newID, "2026-06-01", "2026-06-05", 1, 2, 30000, "25", 2)
	replace.SourceEvidenceEditsJSON = `{"reference":"amended notice"}`
	result, err := f.investmentService.ReplaceCashStockMerger(ctx, replace)
	require.NoError(t, err)
	evidence := transferFactsEvidence(t, f, result.Replacement.ID)
	assert.Contains(t, evidence, `"revision":9007199254740993`)
	assert.Contains(t, evidence, `"ratio":0.123456789012345678901234567890`)
	assert.Contains(t, evidence, `"reference":"amended notice"`)
	assert.Equal(t, exactSourceEvidence, transferFactsEvidence(t, f, merged.Transaction.ID))
}

func TestShareExchangeReplacementPreservesExactUneditedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "NEWCO")
	buyOn(t, f, "2026-02-01", 10, 10000)
	input := shareExchangeInput(f, newID, "2026-06-01", 2, 1)
	input.SourceEvidenceJSON = exactSourceEvidence
	exchanged, err := f.investmentService.ShareExchange(ctx, input)
	require.NoError(t, err)

	// A reference edit alone is a correction and keeps the other keys.
	replace := replaceShareExchangeInput(f, exchanged.Transaction.ID, newID, "2026-06-01", 2, 1)
	replace.SourceEvidenceEditsJSON = `{"reference":"amended notice"}`
	result, err := f.investmentService.ReplaceShareExchange(ctx, replace)
	require.NoError(t, err)
	evidence := transferFactsEvidence(t, f, result.Replacement.ID)
	assert.Contains(t, evidence, `"revision":9007199254740993`)
	assert.Contains(t, evidence, `"ex_date":"2026-05-29"`)
	assert.Contains(t, evidence, `"reference":"amended notice"`)
}

// transferFactsEvidence reads the stored evidence text of a transaction's
// exchange or spin-off operation.
func transferFactsEvidence(t *testing.T, f *investmentsTestFixture, transactionID int64) string {
	t.Helper()
	var evidence string
	require.NoError(t, f.database.QueryRow(`SELECT facts.source_evidence_json FROM investment_transfer_facts facts
		JOIN investment_operation_journal_links link ON link.operation_id = facts.operation_id AND link.role = 'primary'
		JOIN transaction_versions version ON version.id = link.transaction_version_id
		WHERE version.transaction_id = ?`, transactionID).Scan(&evidence))
	return evidence
}
