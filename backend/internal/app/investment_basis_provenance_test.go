package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Both preview and commit must refuse incomplete declarations before any
// journal, audit, resolution or replay projection becomes durable.
func TestBasisResolutionRequiresSourceDeclarationWithoutWriting(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, evidence string }{
		{"missing", ""}, {"null", "null"}, {"empty", "{}"}, {"whitespace", "   "},
		{"untyped_reference", `{"reference":"statement 2020"}`},
		{"blank_reference", `{"kind":"statement","reference":"   "}`},
		{"null_reference", `{"kind":"provider","reference":null}`},
		{"unknown_kind", `{"kind":"other","reference":"statement"}`},
		{"owner_missing_rationale", `{"kind":"owner_assertion","known_on":"2026-10-13"}`},
		{"owner_blank_rationale", `{"kind":"owner_assertion","rationale":"  ","known_on":"2026-10-13"}`},
		{"owner_missing_date", `{"kind":"owner_assertion","rationale":"original purchase records"}`},
		{"owner_invalid_date", `{"kind":"owner_assertion","rationale":"original purchase records","known_on":"2026-02-30"}`},
		{"wrong_type", `{"kind":"statement","reference":5}`},
		{"control_character", `{"kind":"statement","reference":"statement\u0000"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
			require.NoError(t, err)
			input := resolveInput(f, transfer.Transaction.ID, 8000)
			input.SourceEvidenceJSON = tc.evidence
			before := buyReplacementPreviewSnapshot(t, f.database)
			for _, basis := range []int64{0, 8000} {
				input.BasisValue = basis
				_, err = f.investmentService.ResolveTransferBasisImpact(ctx, input)
				require.ErrorAs(t, err, new(ValidationError))
				_, err = f.investmentService.ResolveTransferBasis(ctx, input)
				require.ErrorAs(t, err, new(ValidationError))
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			}
			// Replacement must require a new declaration too; original evidence
			// remains immutable even if a caller attempts a corrective write.
			resolveAcknowledged(t, f, resolveInput(f, transfer.Transaction.ID, 8000))
			correction := correctResolutionInput(f, transfer.Transaction.ID, 6000)
			correction.SourceEvidenceJSON = tc.evidence
			before = buyReplacementPreviewSnapshot(t, f.database)
			_, err = f.investmentService.ReplaceTransferBasisResolutionImpact(ctx, correction)
			require.ErrorAs(t, err, new(ValidationError))
			_, err = f.investmentService.ReplaceTransferBasisResolution(ctx, correction)
			require.ErrorAs(t, err, new(ValidationError))
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
		})
	}
}

func TestBasisResolutionAcceptsDeclaredSourcesAndPreservesOriginalEvidence(t *testing.T) {
	t.Parallel()
	for _, evidence := range []string{
		`{"kind":"statement","reference":"Synthetic broker statement 2020","record_id":"synthetic-42"}`,
		`{"kind":"provider","reference":"Synthetic broker export row 42"}`,
		`{"kind":"owner_assertion","rationale":"Reconstructed from original purchase records.\nIncludes original fees.","known_on":"2026-10-13"}`,
	} {
		t.Run(evidence, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			seedExternalTransferEquity(t, f.database)
			transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
			require.NoError(t, err)
			input := resolveInput(f, transfer.Transaction.ID, 8000)
			input.SourceEvidenceJSON = evidence
			resolveAcknowledged(t, f, input)
			replaceResolutionAcknowledged(t, f, correctResolutionInput(f, transfer.Transaction.ID, 6000))
			chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
			require.NoError(t, err)
			require.Len(t, chain.BasisResolutions, 2)
			require.JSONEq(t, evidence, chain.BasisResolutions[0].SourceEvidenceJSON)
			require.Equal(t, "superseded", chain.BasisResolutions[0].Status)
			require.NotEqual(t, chain.BasisResolutions[0].Reason, evidence)
			_, err = f.database.Exec(`UPDATE investment_basis_resolutions SET source_evidence_json = '{}' WHERE operation_id = ?`, chain.BasisResolutions[0].OperationID)
			require.Error(t, err, "original evidence is immutable")
			rows := exportedBundle(t, f)["investment-basis-resolutions.csv"]
			require.Len(t, rows, 2)
			require.JSONEq(t, evidence, rows[0]["source_evidence_json"], "export preserves the superseded declaration and metadata")
			requireHealthyUnresolvedBook(t, f)
		})
	}
}

func TestLegacyBasisResolutionEvidenceRemainsReadableAndExported(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	seedExternalTransferEquity(t, f.database)
	transfer, err := f.investmentService.ExternalTransferIn(ctx, unknownTransferInInput(f, "2026-06-01", ""))
	require.NoError(t, err)
	journal, params, err := f.investmentService.resolveTransferBasisWrite(ctx, resolveInput(f, transfer.Transaction.ID, 8000))
	require.NoError(t, err)
	// Emulate a resolution already recorded before the declaration contract.
	legacy := `{"reference":"Synthetic legacy statement"}`
	params.SourceEvidenceJSON, journal.Spec.MetadataJSON = legacy, legacy
	_, err = f.investmentService.repository.ResolveTransferBasis(ctx, journal, params)
	require.NoError(t, err)
	requireHealthyUnresolvedBook(t, f)
	replaceResolutionAcknowledged(t, f, correctResolutionInput(f, transfer.Transaction.ID, 6000))
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, transfer.Transaction.ID)
	require.NoError(t, err)
	require.JSONEq(t, legacy, chain.BasisResolutions[0].SourceEvidenceJSON)
	rows := exportedBundle(t, f)["investment-basis-resolutions.csv"]
	require.JSONEq(t, legacy, rows[0]["source_evidence_json"])
	requireHealthyUnresolvedBook(t, f)
}
