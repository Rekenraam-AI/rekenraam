package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

func TestSeparateWithholdingWithoutPayerPreservesShapeAndExactCoverage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	tax := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	_, err := f.investmentService.Dividend(ctx, cashDividendInput(f, tax, 10000, 1500))
	require.NoError(t, err)
	input := DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-03-02", WithholdingOnly: true,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, WithholdingAccountID: &tax,
		AmountValue: 200, AmountScale: 2}
	withholding, err := f.investmentService.Dividend(ctx, input)
	require.NoError(t, err)
	require.Equal(t, int64(8300), accountBalance(t, f, f.cashAccountID))
	require.Equal(t, int64(-10000), accountBalance(t, f, f.incomeAccountID))
	require.Equal(t, int64(1700), accountBalance(t, f, tax))
	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, withholding.ID)
	require.NoError(t, err)
	require.True(t, chain.EffectiveDividend.WithholdingOnly)
	require.Nil(t, chain.EffectiveDividend.CommodityID)
	coverage, err := f.investmentService.DividendCoverage(ctx, DividendCoverageInput{})
	require.NoError(t, err)
	require.Len(t, coverage.Unassigned, 2)
	for _, item := range coverage.Unassigned {
		if item.TransactionID == withholding.ID {
			require.Equal(t, CoverageNone, item.GrossState)
			require.Nil(t, item.Gross)
			require.Equal(t, CoverageKnown, item.WithholdingState)
			require.Equal(t, "200e-2", coverageAmountString(t, item.Withholding))
		}
	}
	requireInvestmentSelfCheckPasses(t, f)
}

func TestDividendPayerAssignClearAndOmissionPreserveReconciledAmounts(t *testing.T) {
	t.Parallel()
	for _, shape := range []string{"dividend", "embedded_tax", "separate_tax"} {
		t.Run(shape, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := newInvestmentsTestFixture(t)
			tax := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
			input := cashDividendInput(f, tax, 10000, 0)
			net, income, taxAmount := int64(10000), int64(-10000), int64(0)
			if shape == "embedded_tax" {
				input = cashDividendInput(f, tax, 10000, 1500)
				net, taxAmount = 8500, 1500
			} else if shape == "separate_tax" {
				input = DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-03-01", WithholdingOnly: true,
					CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, WithholdingAccountID: &tax, AmountValue: 200, AmountScale: 2}
				net, income, taxAmount = -200, 0, 200
			}
			input.CommodityID = NullableValue(f.stockCommodityID)
			original, err := f.investmentService.Dividend(ctx, input)
			require.NoError(t, err)
			checkpoint := reconcileCash(t, f, "2026-04-01", net/100)
			currentID := original.ID
			other := seedTestSecurityCommodity(t, f, "OTHERPAYER")
			for _, step := range []struct {
				name  string
				patch NullablePatch[int64]
				want  *int64
			}{
				{"omit", NullablePatch[int64]{}, &f.stockCommodityID},
				{"clear", NullablePatch[int64]{Set: true}, nil},
				{"assign", NullableValue(other), &other},
				{"omit_after_assign", NullablePatch[int64]{}, &other},
				{"clear_again", NullablePatch[int64]{Set: true}, nil},
			} {
				replacement := input
				replacement.CommodityID = step.patch
				command := ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID, TransactionID: currentID, Reason: step.name, Replacement: replacement}
				before := buyReplacementPreviewSnapshot(t, f.database)
				impact, err := f.investmentService.ReplaceDividendReconciliationImpact(ctx, command)
				require.NoError(t, err, step.name)
				require.Empty(t, impact.AffectedCheckpoints)
				require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
				result, err := f.investmentService.ReplaceDividend(ctx, command)
				require.NoError(t, err, step.name)
				currentID = result.Replacement.ID
				chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, original.ID)
				require.NoError(t, err)
				require.Equal(t, step.want, chain.EffectiveDividend.CommodityID)
				require.Equal(t, input.WithholdingOnly, chain.EffectiveDividend.WithholdingOnly)
				require.Equal(t, net, accountBalance(t, f, f.cashAccountID))
				require.Equal(t, income, accountBalance(t, f, f.incomeAccountID))
				require.Equal(t, taxAmount, accountBalance(t, f, tax))
				require.Equal(t, []int64{checkpoint}, activeCheckpointIDs(t, f))
				_, err = f.investmentService.ReplaceDividend(ctx, command)
				require.ErrorIs(t, err, ErrInvestmentDividendAlreadyCorrected)
			}
			var originalPayer int64
			require.NoError(t, f.database.QueryRow(`SELECT f.commodity_id FROM investment_dividend_facts f
				JOIN investment_operation_journal_links l ON l.operation_id=f.operation_id AND l.role='primary'
				WHERE l.transaction_version_id=?`, original.VersionID).Scan(&originalPayer))
			require.Equal(t, f.stockCommodityID, originalPayer)
			var lots int
			require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&lots))
			require.Zero(t, lots)
			requireInvestmentSelfCheckPasses(t, f)
			facts := exportedBundle(t, f)["investment-dividend-facts.csv"]
			require.NotEmpty(t, facts)
			var cleared bool
			for _, fact := range facts {
				if fact["commodity_id"] == "" {
					cleared = true
					if input.WithholdingOnly {
						require.Equal(t, "1", fact["withholding_only"])
					}
				}
			}
			require.True(t, cleared)
			// A real reversal still changes reconciled cash and requires review.
			reverse := ReverseInvestmentDividendInput{OwnerUserID: f.ownerUserID, TransactionID: currentID, Reason: "duplicate payment"}
			before := buyReplacementPreviewSnapshot(t, f.database)
			_, err = f.investmentService.ReverseDividend(ctx, reverse)
			require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			impact, err := f.investmentService.ReverseDividendReconciliationImpact(ctx, reverse)
			require.NoError(t, err)
			reverse.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{checkpoint}}
			require.NotEmpty(t, impact.AffectedCheckpoints)
			_, err = f.investmentService.ReverseDividend(ctx, reverse)
			require.NoError(t, err)
			require.Equal(t, int64(0), accountBalance(t, f, f.cashAccountID))
		})
	}
}

func TestDividendAttributionLateFailureAndMissingShapeAreAtomicAndDetectable(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	input := cashDividendInput(f, 0, 10000, 0)
	input.CommodityID = NullableValue(f.stockCommodityID)
	original, err := f.investmentService.Dividend(ctx, input)
	require.NoError(t, err)
	_, err = f.database.Exec(`CREATE TRIGGER refuse_dividend_repair BEFORE INSERT ON investment_dividend_facts
		WHEN NEW.commodity_id IS NULL BEGIN SELECT RAISE(ABORT,'late dividend repair failure'); END`)
	require.NoError(t, err)
	input.CommodityID = NullablePatch[int64]{Set: true}
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
		TransactionID: original.ID, Reason: "clear payer", Replacement: input})
	require.ErrorContains(t, err, "late dividend repair failure")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.database.Exec(`DROP TRIGGER investment_dividend_facts_no_delete`)
	require.NoError(t, err)
	_, err = f.database.Exec(`DELETE FROM investment_dividend_facts`)
	require.NoError(t, err)
	check := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentFoundation)
	require.Equal(t, SelfCheckFailed, check.Status)
	require.Contains(t, check.Summary, "payment-shape facts")
	// Do not silently turn a supplied invalid commodity into unknown attribution.
	input.CommodityID = NullableValue(int64(0))
	_, err = f.investmentService.Dividend(ctx, input)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
}

func TestDividendProviderIdentityExcludesCorrectedPaymentDescendants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	input := DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: NullableValue(f.instrument.CommodityID),
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2}
	payment, err := f.investmentSvc.Dividend(ctx, input)
	require.NoError(t, err)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "link", july.ID, payment.ID)
	require.NoError(t, err)
	repaired, err := f.investmentSvc.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: 1,
		TransactionID: payment.ID, Reason: "same payment corrected", Replacement: input})
	require.NoError(t, err)
	var sourceID int64
	var normalized string
	require.NoError(t, f.database.QueryRow(`SELECT source_id,normalized_json FROM investment_provider_events WHERE id=?`, july.ProviderEventID).Scan(&sourceID, &normalized))
	_, err = f.pricing.repository.StageProviderEvent(ctx, db.StageProviderEventParams{BookID: BookID, ActorUserID: 1,
		SourceID: sourceID, ProviderEventID: "another-claim", InstrumentID: f.instrument.ID, EventFamily: "dividend",
		EventDate: "2026-07-29", NormalizedJSON: normalized, RawJSON: `{"id":"another"}`, ConfidenceBPS: 8000,
		ProposedJSON: july.ProposedTransactionJSON, RecordedAt: "2026-08-20T00:00:00Z"})
	require.NoError(t, err)
	var suggestionID int64
	require.NoError(t, f.database.QueryRow(`SELECT s.id FROM investment_event_suggestions s JOIN investment_provider_events e ON e.id=s.provider_event_id WHERE e.provider_event_id='another-claim'`).Scan(&suggestionID))
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "duplicate", suggestionID, repaired.Replacement.ID)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}
