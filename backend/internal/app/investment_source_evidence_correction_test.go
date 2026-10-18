package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// A JSON number in source metadata is exact evidence. Adjacent integers
// above 2^53 must remain distinguishable through the real correction writer.
func TestCorporateActionCorrectionDistinguishesExactSourceNumbers(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"spin_off", "cash_and_stock_merger"} {
		t.Run(family, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			newID := seedTestSecurityCommodity(t, f, "EXACTSOURCE")
			buyOn(t, f, "2026-02-01", 10, 10000)
			const original = `{"notice":"issuer allocation","revision":9007199254740992}`
			const corrected = `{"revision":9007199254740993,"notice":"issuer allocation"}`
			var originalTransactionID, successorID int64
			switch family {
			case "spin_off":
				input := spinOffInput(f, newID, "2026-06-01", 1, 1, "25", 2)
				input.SourceEvidenceJSON = original
				posted, err := f.investmentService.SpinOff(ctx, input)
				require.NoError(t, err)
				originalTransactionID = posted.Transaction.ID
				replacement := replaceSpinOffInput(f, originalTransactionID, newID, "2026-06-01", 1, 1, "25", 2)
				replacement.SourceEvidenceJSON = corrected
				before := buyReplacementPreviewSnapshot(t, f.database)
				preview, err := f.investmentService.PreviewSpinOffReplacement(ctx, replacement)
				require.NoError(t, err, "a different exact source revision is a real evidence correction")
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				if preview.Impact.GainImpact != nil {
					replacement.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
				}
				result, err := f.investmentService.ReplaceSpinOff(ctx, replacement)
				require.NoError(t, err)
				successorID = result.Replacement.ID
				requireScaled(t, 7500, 2, lotRemainingBasis(t, f, posted.Plan.Links[0].SourceLotID), "parent basis")
			case "cash_and_stock_merger":
				input := cashStockMergerInput(f, newID, "2026-06-01", "2026-06-05", 1, 1, 3000, "25", 2)
				input.SourceEvidenceJSON = original
				posted, err := f.investmentService.CashStockMerger(ctx, input)
				require.NoError(t, err)
				originalTransactionID = posted.Transaction.ID
				replacement := replaceCashStockMergerInput(f, originalTransactionID, newID, "2026-06-01", "2026-06-05", 1, 1, 3000, "25", 2)
				replacement.SourceEvidenceJSON = corrected
				before := buyReplacementPreviewSnapshot(t, f.database)
				preview, err := f.investmentService.PreviewCashStockMergerReplacement(ctx, replacement)
				require.NoError(t, err, "a different exact source revision is a real evidence correction")
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				if preview.Impact.GainImpact != nil {
					replacement.GainImpactAcknowledgement = preview.Impact.GainImpact.Acknowledgement
				}
				result, err := f.investmentService.ReplaceCashStockMerger(ctx, replacement)
				require.NoError(t, err)
				successorID = result.Replacement.ID
				_, proceeds, basis, knowledge := mergerDecision(t, f, successorID)
				require.Equal(t, "known", knowledge)
				requireScaled(t, 3000, 2, proceeds, "cash remains 30.00")
				requireScaled(t, 2500, 2, basis, "elected basis remains 25.00")
			}
			for transactionID, evidence := range map[int64]string{originalTransactionID: original, successorID: corrected} {
				var saved string
				require.NoError(t, f.database.QueryRow(`SELECT f.source_evidence_json FROM investment_transfer_facts f
					JOIN investment_operation_journal_links j ON j.operation_id = f.operation_id AND j.role = 'primary'
					JOIN transaction_versions v ON v.id = j.transaction_version_id WHERE v.transaction_id = ?`, transactionID).Scan(&saved))
				require.Equal(t, evidence, saved, "original and corrected evidence retain every digit")
			}
			require.Equal(t, "10", positionQuantity(t, f, f.holdingAccountID, newID))
			requireInvestmentSelfCheckPasses(t, f)
		})
	}
}

func TestShareExchangeCorrectionRejectsUnchangedSourceEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	newID := seedTestSecurityCommodity(t, f, "EXCHANGESOURCE")
	buyOn(t, f, "2026-02-01", 10, 10000)
	input := shareExchangeInput(f, newID, "2026-06-01", 1, 1)
	input.SourceEvidenceJSON = `{"reference":"issuer notice","revision":9007199254740993}`
	posted, err := f.investmentService.ShareExchange(ctx, input)
	require.NoError(t, err)
	replacement := replaceShareExchangeInput(f, posted.Transaction.ID, newID, "2026-06-01", 1, 1)
	replacement.SourceEvidenceJSON = `{"revision":9007199254740993,"reference":"issuer notice"}`
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.PreviewShareExchangeReplacement(ctx, replacement)
	require.ErrorAs(t, err, &ValidationError{}, "reordering unchanged evidence is not a correction")
	_, err = f.investmentService.ReplaceShareExchange(ctx, replacement)
	require.ErrorAs(t, err, &ValidationError{})
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))

	// Omitting evidence while correcting the ratio retains the declaration.
	replacement.SourceEvidenceJSON = ""
	replacement.RatioNumerator = 2
	result, err := f.investmentService.ReplaceShareExchange(ctx, replacement)
	require.NoError(t, err)
	var saved string
	require.NoError(t, f.database.QueryRow(`SELECT source_evidence_json FROM investment_transfer_facts WHERE operation_id = ?`,
		transferOperationID(t, f, result.Replacement.ID)).Scan(&saved))
	require.Equal(t, input.SourceEvidenceJSON, saved)
	require.Equal(t, "20", positionQuantity(t, f, f.holdingAccountID, newID))
	requireInvestmentSelfCheckPasses(t, f)
}
