package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

func TestPaymentAttributionRepairPreservesProviderEvidenceAndPaymentIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.goShort(t, f.holding.ID)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	input := PaymentInLieuInput{OwnerUserID: 1, HoldingAccountID: NullableValue(f.holding.ID), CommodityID: NullableValue(f.instrument.CommodityID), ExDate: NullableValue("2026-07-29"),
		TransactionDate: "2026-08-14", CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, ExpenseAccountID: expense, AmountValue: 800, AmountScale: 2}
	payment, err := f.investmentSvc.PaymentInLieu(ctx, input)
	require.NoError(t, err)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "link", july.ID, payment.ID)
	require.NoError(t, err)
	var sourceID int64
	var raw, normalized string
	require.NoError(t, f.database.QueryRow(`SELECT source_id,raw_json,normalized_json FROM investment_provider_events WHERE id = ?`, july.ProviderEventID).Scan(&sourceID, &raw, &normalized))
	currentID := payment.ID
	for _, date := range []string{"2026-07-30", "", "2026-07-29"} {
		replacement := input
		replacement.ExDate = NullableValue(date)
		if date == "" {
			replacement.ExDate = NullablePatch[string]{Set: true}
		}
		result, err := f.investmentSvc.ReplacePaymentInLieu(ctx, ReplaceInvestmentPaymentInLieuInput{OwnerUserID: 1, TransactionID: currentID, Reason: "attribution correction", Replacement: replacement})
		require.NoError(t, err)
		currentID = result.Replacement.ID
		chain, err := f.investmentSvc.CorrectionChain(ctx, 1, payment.ID)
		require.NoError(t, err)
		if date != "2026-07-29" {
			require.Equal(t, date == "", chain.EffectivePaymentInLieu.Attribution.Unavailable)
			require.Equal(t, []string{db.AttributionProviderMismatch}, chain.EffectivePaymentInLieu.Attribution.Reasons)
		} else {
			require.Equal(t, SelfCheckPassed, chain.EffectivePaymentInLieu.Attribution.Status)
		}
		before := countTransactions(t, f.database)
		f.fetch(t)
		require.Equal(t, before, countTransactions(t, f.database), "refetch never posts repaired cash again")
		var newRaw, newNormalized string
		require.NoError(t, f.database.QueryRow(`SELECT raw_json,normalized_json FROM investment_provider_events WHERE id = ?`, july.ProviderEventID).Scan(&newRaw, &newNormalized))
		require.Equal(t, raw, newRaw)
		require.Equal(t, normalized, newNormalized)
		original := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
		require.Equal(t, "linked", original.Status)
		require.Equal(t, payment.ID, *original.LinkedTransactionID)
	}
	// A new event cannot claim another version of an already-linked payment.
	staged, err := f.pricing.repository.StageProviderEvent(ctx, db.StageProviderEventParams{BookID: BookID, ActorUserID: 1, SourceID: sourceID, ProviderEventID: "another-july-event",
		InstrumentID: f.instrument.ID, EventFamily: "dividend", EventDate: "2026-07-29", NormalizedJSON: normalized, RawJSON: `{"id":"another"}`,
		ConfidenceBPS: 8000, ProposedJSON: july.ProposedTransactionJSON, RecordedAt: "2026-08-20T00:00:00Z"})
	require.NoError(t, err)
	require.True(t, staged)
	var suggestionID int64
	require.NoError(t, f.database.QueryRow(`SELECT s.id FROM investment_event_suggestions s JOIN investment_provider_events e ON e.id = s.provider_event_id WHERE e.provider_event_id = 'another-july-event'`).Scan(&suggestionID))
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "duplicate", suggestionID, currentID)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "link refusal preserves cash, audit and evidence")
}
