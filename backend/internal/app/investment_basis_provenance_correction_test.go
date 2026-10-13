package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

func provenanceCorrectionInput(t *testing.T, f *investmentsTestFixture, transactionID, basis int64) CorrectTransferBasisResolutionInput {
	t.Helper()
	chain, err := f.investmentService.CorrectionChain(context.Background(), f.ownerUserID, transactionID)
	require.NoError(t, err)
	input := correctResolutionInput(f, transactionID, basis)
	input.ExpectedResolutionOperationID = chain.EffectiveBasisResolution.OperationID
	revision := chain.EffectiveBasisResolution.ProvenanceRevision
	input.ExpectedProvenanceRevision = &revision
	input.Reason = "correct the source reference, keeping the stated basis"
	return input
}

func requireProvenanceOnlyRowsChanged(t *testing.T, before, after map[string][][]any) {
	t.Helper()
	require.Len(t, after["audit_events"], len(before["audit_events"])+1)
	require.Len(t, after["investment_basis_resolution_provenance"], len(before["investment_basis_resolution_provenance"])+1)
	for table, rows := range before {
		if table != "audit_events" && table != "investment_basis_resolution_provenance" {
			require.Equal(t, rows, after[table], "financial rows and projections stay byte-for-byte unchanged: %s", table)
		}
	}
}

func TestSameBasisChangedEvidenceCorrectionPostsNoJournals(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
	require.NoError(t, err)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	input := correctResolutionInput(f, transfer.Transaction.ID, 8000)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
	require.NoError(t, err)
	input.ExpectedResolutionOperationID = chain.EffectiveBasisResolution.OperationID
	revision := 0
	input.ExpectedProvenanceRevision = &revision
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplaceTransferBasisResolutionImpact(ctx, input)
	require.NoError(t, err)
	require.Empty(t, impact.AffectedCheckpoints)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	result, err := f.investmentService.ReplaceTransferBasisResolution(ctx, input)
	require.NoError(t, err)
	require.Nil(t, result.Inverse)
	require.Nil(t, result.Replacement)
	requireProvenanceOnlyRowsChanged(t, before, buyReplacementPreviewSnapshot(t, f.database))
	requireHealthyUnresolvedBook(t, f)
}

func TestProvenanceCorrectionAdmitsExactScaledEqualityKnownZeroAndLegacy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		basis, replacement int64
		scale              int
		legacy             bool
	}{
		{"scaled_equality", 8000, 80, 0, false},
		{"journal_free_known_zero", 0, 0, 12, false},
		{"legacy_to_typed", 8000, 80000, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
			require.NoError(t, err)
			if tc.legacy {
				journal, params, err := f.investmentService.resolveTransferBasisWrite(ctx, resolveInput(f, transfer.Transaction.ID, tc.basis))
				require.NoError(t, err)
				params.SourceEvidenceJSON = `{"reference":"Synthetic legacy statement"}`
				_, err = f.investmentService.repository.ResolveTransferBasis(ctx, journal, params)
				require.NoError(t, err)
			} else {
				resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, tc.basis))
			}
			input := provenanceCorrectionInput(t, f, transfer.Transaction.ID, tc.replacement)
			input.BasisScale = tc.scale
			before := buyReplacementPreviewSnapshot(t, f.database)
			replaceResolutionAcknowledged(t, f, input)
			requireProvenanceOnlyRowsChanged(t, before, buyReplacementPreviewSnapshot(t, f.database))
			chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
			require.NoError(t, err)
			require.Equal(t, exact.New(tc.basis), chain.EffectiveBasisResolution.BasisValue, "even the stored amount representation is unchanged")
			require.Equal(t, 2, chain.EffectiveBasisResolution.BasisScale)
			require.Equal(t, 1, chain.EffectiveBasisResolution.ProvenanceRevision)
			require.JSONEq(t, input.SourceEvidenceJSON, chain.EffectiveBasisResolution.SourceEvidenceJSON)
			requireHealthyUnresolvedBook(t, f)
		})
	}
}

func TestProvenanceCorrectionPreservesDownstreamSalesTransfersAndCheckpoints(t *testing.T) {
	t.Parallel()
	f := newTransferOutFixture(t)
	ctx := context.Background()
	destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-05-01", ""))
	require.NoError(t, err)
	_, err = acknowledgedSplit(ctx, f.investmentService, splitInput(f, "2026-05-10", 2, 1))
	require.NoError(t, err)
	_, err = f.investmentService.InternalTransfer(ctx, internalTransferFromLot(f, destination, *transfer.LotID, exact.New(2), 0))
	require.NoError(t, err)
	sale := sellInput(f, "2026-07-01", 2)
	sale.HoldingAccountID = destination
	_, err = f.investmentService.Sell(ctx, sale)
	require.NoError(t, err)
	_, err = f.investmentService.ExternalTransferOut(ctx, transferOutOfLot(f, "2026-08-01", *transfer.LotID, 1))
	require.NoError(t, err)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	checkpoint := reconcileAccount(t, f, f.holdingAccountID, f.stockCommodityID, "2026-08-31", 1)
	input := provenanceCorrectionInput(t, f, transfer.Transaction.ID, 8000)
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplaceTransferBasisResolutionImpact(ctx, input)
	require.NoError(t, err)
	require.Empty(t, impact.AffectedCheckpoints)
	require.Empty(t, impact.GainImpact.Changes)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, input)
	require.NoError(t, err, "no balance override or gain acknowledgement is needed")
	requireProvenanceOnlyRowsChanged(t, before, buyReplacementPreviewSnapshot(t, f.database))
	require.Equal(t, []int64{checkpoint}, activeCheckpointIDsFor(t, f, f.holdingAccountID, f.stockCommodityID))
	requireHealthyUnresolvedBook(t, f)
}

func TestProvenanceCorrectionRefusesNoOpsInvalidDeclarationsAndStaleWrites(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
	require.NoError(t, err)
	resolution := resolveInput(f, transfer.Transaction.ID, 8000)
	resolution.SourceEvidenceJSON = `{"kind":"statement","reference":"Synthetic \u0073tatement","record":{"n":9007199254740993}}`
	resolveAcknowledged(t, f, resolution)
	input := provenanceCorrectionInput(t, f, transfer.Transaction.ID, 8000)
	before := buyReplacementPreviewSnapshot(t, f.database)
	for _, evidence := range []string{
		resolution.SourceEvidenceJSON,
		`{ "record": {"n":9007199254740993}, "reference":"Synthetic statement", "kind":"statement" }`,
		`{}`, `{"kind":"statement","reference":""}`, `{"kind":"owner_assertion","rationale":"records"}`,
	} {
		invalid := input
		invalid.SourceEvidenceJSON = evidence
		for _, preview := range []bool{true, false} {
			if preview {
				_, err = f.investmentService.ReplaceTransferBasisResolutionImpact(ctx, invalid)
			} else {
				_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, invalid)
			}
			require.ErrorAs(t, err, new(ValidationError))
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
		}
	}
	missingReason := input
	missingReason.Reason = " "
	_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, missingReason)
	require.ErrorAs(t, err, new(ValidationError))
	missingExpected := input
	missingExpected.ExpectedProvenanceRevision = nil
	_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, missingExpected)
	require.Error(t, err)
	// Pin a repository write before a competing correction, exercising the
	// guard under the transaction lock rather than just service prevalidation.
	op, write, params, err := f.investmentService.prepareBasisResolutionReplacement(ctx, input)
	require.NoError(t, err)
	replaceResolutionAcknowledged(t, f, input)
	before = buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.repository.ReplaceTransferBasisResolution(ctx, op, write, params)
	require.ErrorIs(t, err, db.ErrInvestmentSaleChanged)
	_, err = f.investmentService.ReplaceTransferBasisResolutionImpact(ctx, input)
	require.ErrorIs(t, err, ErrInvestmentBasisResolutionChanged)
	_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, input)
	require.ErrorIs(t, err, ErrInvestmentBasisResolutionChanged)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	fresh := provenanceCorrectionInput(t, f, transfer.Transaction.ID, 8000)
	_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, fresh)
	require.ErrorAs(t, err, new(ValidationError), "repeating the now-effective declaration is a no-op")
	// A newer economic resolution also makes an old provenance command stale.
	replaceResolutionAcknowledged(t, f, correctResolutionInput(f, transfer.Transaction.ID, 6000))
	before = buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, fresh)
	require.ErrorIs(t, err, ErrInvestmentBasisResolutionChanged)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	requireHealthyUnresolvedBook(t, f)
}

func TestProvenanceCorrectionLateFailureRollsBackAuditAndEvidence(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
	require.NoError(t, err)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	input := provenanceCorrectionInput(t, f, transfer.Transaction.ID, 8000)
	_, err = f.database.Exec(`CREATE TRIGGER probe_provenance_failure BEFORE INSERT ON investment_basis_resolution_provenance
		BEGIN SELECT RAISE(ABORT, 'forced failure after audit insertion'); END`)
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceTransferBasisResolutionImpact(ctx, input)
	require.ErrorContains(t, err, "forced failure")
	_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, input)
	require.ErrorContains(t, err, "forced failure")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func TestProvenanceCorrectionImmutableHistoryExportsAndSurvivesRestore(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
	require.NoError(t, err)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 0))
	first := provenanceCorrectionInput(t, f, transfer.Transaction.ID, 0)
	first.RequestID = "provenance-request-1"
	replaceResolutionAcknowledged(t, f, first)
	second := provenanceCorrectionInput(t, f, transfer.Transaction.ID, 0)
	second.SourceEvidenceJSON = `{"kind":"owner_assertion","rationale":"original purchase records","known_on":"2026-10-13","record_id":"42"}`
	second.Reason = "owner reconstructed the source"
	replaceResolutionAcknowledged(t, f, second)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
	require.NoError(t, err)
	require.Len(t, chain.BasisResolutions, 1)
	history := chain.BasisResolutions[0]
	require.Len(t, history.ProvenanceCorrections, 2)
	require.JSONEq(t, resolveInput(f, transfer.Transaction.ID, 0).SourceEvidenceJSON, history.SourceEvidenceJSON)
	require.JSONEq(t, first.SourceEvidenceJSON, history.ProvenanceCorrections[0].SourceEvidenceJSON)
	require.JSONEq(t, second.SourceEvidenceJSON, chain.EffectiveBasisResolution.SourceEvidenceJSON)
	require.Equal(t, f.ownerUserID, history.ProvenanceCorrections[0].ActorUserID)
	require.Equal(t, "provenance-request-1", history.ProvenanceCorrections[0].RequestID)
	require.Equal(t, first.Reason, history.ProvenanceCorrections[0].Reason)
	_, err = f.database.Exec(`UPDATE investment_basis_resolution_provenance SET source_evidence_json = '{}'`)
	require.Error(t, err)
	_, err = f.database.Exec(`DELETE FROM investment_basis_resolution_provenance`)
	require.Error(t, err)
	rows := exportedBundle(t, f)
	require.Len(t, rows["investment-basis-resolutions.csv"], 1)
	provenance := rows["investment-basis-resolution-provenance.csv"]
	require.Len(t, provenance, 2)
	require.JSONEq(t, first.SourceEvidenceJSON, provenance[0]["source_evidence_json"])
	require.Equal(t, "provenance-request-1", provenance[0]["request_id"])
	require.Equal(t, "2", provenance[1]["revision_seq"])
	require.JSONEq(t, second.SourceEvidenceJSON, provenance[1]["source_evidence_json"])
	backup := filepath.Join(t.TempDir(), "backup.sqlite")
	_, err = db.OnlineBackupSQLiteDatabase(ctx, f.database, backup, db.OnlineBackupOptions{})
	require.NoError(t, err)
	restoredURL := "file:" + filepath.Join(t.TempDir(), "restored.sqlite")
	_, err = db.RestoreSQLiteDatabase(ctx, backup, restoredURL)
	require.NoError(t, err)
	restored, err := db.Open(ctx, restoredURL)
	require.NoError(t, err)
	defer restored.Close()
	var count int
	require.NoError(t, restored.QueryRow(`SELECT COUNT(*) FROM investment_basis_resolution_provenance`).Scan(&count))
	require.Equal(t, 2, count)
	run, err := selfCheckOver(t, restored).RunSelfCheck(ctx, "manual")
	require.NoError(t, err)
	require.Equal(t, SelfCheckPassed, run.Status)
	// Withdrawal retires only economic knowledge; its complete evidence
	// revision history remains inspectable and healthy.
	reverseResolutionAcknowledged(t, f, correctResolutionInput(f, transfer.Transaction.ID, 0))
	chain, err = f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
	require.NoError(t, err)
	require.Equal(t, "reversed", chain.BasisResolutions[0].Status)
	require.Len(t, chain.BasisResolutions[0].ProvenanceCorrections, 2)
	requireHealthyUnresolvedBook(t, f)
}

func TestSelfCheckDetectsProvenanceRevisionLineageDamage(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
	require.NoError(t, err)
	resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
	replaceResolutionAcknowledged(t, f, provenanceCorrectionInput(t, f, transfer.Transaction.ID, 8000))
	requireHealthyUnresolvedBook(t, f)
	_, err = f.database.Exec(`DROP TRIGGER investment_basis_resolution_provenance_no_update`)
	require.NoError(t, err)
	_, err = f.database.Exec(`UPDATE investment_basis_resolution_provenance SET revision_seq = 2`)
	require.NoError(t, err)
	foundation := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentFoundation)
	require.Equal(t, SelfCheckFailed, foundation.Status)
	require.Contains(t, foundation.Summary, "basis resolution provenance has broken lineage or audit attribution")
}

func TestProvenanceCorrectionPreservesExactAdditionalMetadataOnServer(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
	require.NoError(t, err)
	original := resolveInput(f, transfer.Transaction.ID, 8000)
	original.SourceEvidenceJSON = `{"kind":"statement","reference":"Synthetic old statement","record_id":"42","exact_metadata":9007199254740993,"nested":{"amount":1.00}}`
	resolveAcknowledged(t, f, original)
	input := provenanceCorrectionInput(t, f, transfer.Transaction.ID, 8000)
	input.SourceEvidenceJSON = `{"kind":"owner_assertion","rationale":"purchase records","known_on":"2026-10-13"}`
	input.PreserveSourceMetadata = true
	replaceResolutionAcknowledged(t, f, input)
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
	require.NoError(t, err)
	evidence := chain.EffectiveBasisResolution.SourceEvidenceJSON
	require.Contains(t, evidence, `"exact_metadata":9007199254740993`)
	require.Contains(t, evidence, `"amount":1.00`)
	require.Contains(t, evidence, `"record_id":"42"`)
	require.NotContains(t, evidence, `"reference"`, "source fields are explicitly replaced")
	// Economic replacements made through the same form keep exact metadata too.
	input = provenanceCorrectionInput(t, f, transfer.Transaction.ID, 6000)
	input.PreserveSourceMetadata = true
	replaceResolutionAcknowledged(t, f, input)
	chain, err = f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
	require.NoError(t, err)
	require.Contains(t, chain.EffectiveBasisResolution.SourceEvidenceJSON, `"exact_metadata":9007199254740993`)
	require.Len(t, chain.BasisResolutions[0].ProvenanceCorrections, 1)
	requireHealthyUnresolvedBook(t, f)
}
