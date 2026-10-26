package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

func stageAdditionalDividendClaim(t *testing.T, f *dividendEventFixture, suggestion InvestmentEventSuggestion, providerID string) int64 {
	t.Helper()
	var sourceID int64
	var normalized string
	require.NoError(t, f.database.QueryRow(`SELECT source_id,normalized_json FROM investment_provider_events WHERE id=?`, suggestion.ProviderEventID).Scan(&sourceID, &normalized))
	_, err := f.pricing.repository.StageProviderEvent(context.Background(), db.StageProviderEventParams{BookID: BookID, ActorUserID: 1, SourceID: sourceID,
		ProviderEventID: providerID, InstrumentID: f.instrument.ID, EventFamily: "dividend", EventDate: "2026-07-29", NormalizedJSON: normalized,
		RawJSON: `{"source":"additional"}`, ConfidenceBPS: 8000, ProposedJSON: suggestion.ProposedTransactionJSON, RecordedAt: "2026-08-20T00:00:00Z"})
	require.NoError(t, err)
	var id int64
	require.NoError(t, f.database.QueryRow(`SELECT s.id FROM investment_event_suggestions s JOIN investment_provider_events e ON e.id=s.provider_event_id WHERE e.provider_event_id=?`, providerID).Scan(&id))
	return id
}

func TestDividendSourceAttributionRepairPreservesEvidenceAndWholePaymentIdentity(t *testing.T) {
	t.Parallel()
	for _, generated := range []bool{false, true} {
		name := "linked"
		if generated {
			name = "generated"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := newDividendEventFixture(t)
			f.fetch(t)
			july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
			input := DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: NullableValue(f.instrument.CommodityID), CashAccountID: f.cashAccountID,
				CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2}
			var originalID int64
			if generated {
				accepted, err := f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "accept", july.ID, f.completion("2026-08-14", 1600))
				require.NoError(t, err)
				require.NotNil(t, accepted.GeneratedTransactionID)
				originalID = *accepted.GeneratedTransactionID
			} else {
				payment, err := f.investmentSvc.Dividend(ctx, input)
				require.NoError(t, err)
				originalID = payment.ID
				_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "link", july.ID, originalID)
				require.NoError(t, err)
			}
			var raw, normalized string
			require.NoError(t, f.database.QueryRow(`SELECT raw_json,normalized_json FROM investment_provider_events WHERE id=?`, july.ProviderEventID).Scan(&raw, &normalized))
			fixture := &investmentsTestFixture{database: f.database}
			balance := accountBalance(t, fixture, f.cashAccountID)
			other := seedTestSecurityCommodity(t, fixture, "DIFFERENTPAYER")
			current := originalID
			versions := []int64{originalID}
			for _, step := range []struct {
				patch  NullablePatch[int64]
				status string
			}{
				{NullableValue(other), SelfCheckWarning}, {NullablePatch[int64]{Set: true}, SelfCheckIncomplete}, {NullableValue(f.instrument.CommodityID), SelfCheckPassed},
			} {
				replacement := input
				replacement.CommodityID = step.patch
				result, err := f.investmentSvc.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: 1, TransactionID: current, Reason: "payer repair", Replacement: replacement})
				require.NoError(t, err)
				current = result.Replacement.ID
				versions = append(versions, result.Inverse.ID, current)
				chain, err := f.investmentSvc.CorrectionChain(ctx, 1, originalID)
				require.NoError(t, err)
				require.Equal(t, step.status, chain.EffectiveDividend.Attribution.Status)
				require.Equal(t, step.status == SelfCheckIncomplete, chain.EffectiveDividend.Attribution.Unavailable)
				if step.status == SelfCheckWarning {
					require.Equal(t, []string{db.AttributionProviderMismatch}, chain.EffectiveDividend.Attribution.Reasons)
				} else {
					require.Empty(t, chain.EffectiveDividend.Attribution.Reasons)
				}
				run := mustRunInvestmentSelfCheck(t, fixture)
				require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentFoundation).Status)
				require.Equal(t, step.status, resultFor(t, run, CheckInvestmentAttribution).Status)
				count := countTransactions(t, f.database)
				f.fetch(t)
				require.Equal(t, count, countTransactions(t, f.database))
				require.Equal(t, balance, accountBalance(t, fixture, f.cashAccountID))
				var nowRaw, nowNormalized string
				require.NoError(t, f.database.QueryRow(`SELECT raw_json,normalized_json FROM investment_provider_events WHERE id=?`, july.ProviderEventID).Scan(&nowRaw, &nowNormalized))
				require.Equal(t, raw, nowRaw)
				require.Equal(t, normalized, nowNormalized)
				retained := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
				if generated {
					require.Equal(t, "accepted", retained.Status)
					require.Equal(t, originalID, *retained.GeneratedTransactionID)
				} else {
					require.Equal(t, "linked", retained.Status)
					require.Equal(t, originalID, *retained.LinkedTransactionID)
				}
			}
			additional := stageAdditionalDividendClaim(t, f, july, "new-claim")
			before := buyReplacementPreviewSnapshot(t, f.database)
			for _, id := range versions {
				_, err := f.database.Exec(`UPDATE investment_event_suggestions SET linked_transaction_id=?,status='linked' WHERE id=?`, id, additional)
				require.ErrorContains(t, err, "cash payment already claimed")
				_, err = f.database.Exec(`UPDATE investment_event_suggestions SET generated_transaction_id=?,status='accepted' WHERE id=?`, id, additional)
				require.ErrorContains(t, err, "cash payment already claimed")
			}
			_, err := f.investmentSvc.LinkSuggestion(ctx, 1, 0, "duplicate", additional, current)
			var validation ValidationError
			require.ErrorAs(t, err, &validation)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			// Another actual payment remains linkable even with identical economics.
			unclaimed, err := f.investmentSvc.Dividend(ctx, input)
			require.NoError(t, err)
			_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "valid", additional, unclaimed.ID)
			require.NoError(t, err)
		})
	}
}

func TestDividendProviderConcurrentClaimsCommitOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	additional := stageAdditionalDividendClaim(t, f, july, "concurrent-claim")
	payment, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: NullableValue(f.instrument.CommodityID), CashAccountID: f.cashAccountID,
		CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2})
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []int64{july.ID, additional} {
		go func(id int64) {
			<-start
			_, err := f.investmentSvc.LinkSuggestion(ctx, 1, 0, "concurrent", id, payment.ID)
			results <- err
		}(id)
	}
	close(start)
	successes := 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else {
			var validation ValidationError
			require.ErrorAs(t, err, &validation)
		}
	}
	require.Equal(t, 1, successes)
	var count int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_event_suggestions WHERE linked_transaction_id=?`, payment.ID).Scan(&count))
	require.Equal(t, 1, count)
}

// A provider dividend event's date is its ex date. An owner correction to a
// different known ex date contradicts that retained claim and is disclosed as
// a warning, like a different payer; clearing the ex date is unknown, not a
// contradiction; restoring the claimed date passes. Cash never changes.
func TestDividendCorrectedExDateContradictingProviderEvidenceWarns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	input := DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: NullableValue(f.instrument.CommodityID),
		ExDate: NullableValue("2026-07-29"), CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID,
		IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2}
	payment, err := f.investmentSvc.Dividend(ctx, input)
	require.NoError(t, err)
	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "link", july.ID, payment.ID)
	require.NoError(t, err)
	fixture := &investmentsTestFixture{database: f.database}
	balance := accountBalance(t, fixture, f.cashAccountID)

	current := payment.ID
	for _, step := range []struct {
		exDate  NullablePatch[string]
		status  string
		reasons []string
	}{
		{NullableValue("2026-07-30"), SelfCheckWarning, []string{db.AttributionProviderMismatch}},
		{NullablePatch[string]{Set: true}, SelfCheckPassed, []string{}},
		{NullableValue("2026-07-29"), SelfCheckPassed, []string{}},
	} {
		replacement := input
		replacement.ExDate = step.exDate
		result, err := f.investmentSvc.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: 1, TransactionID: current,
			Reason: "ex date correction", Replacement: replacement})
		require.NoError(t, err)
		current = result.Replacement.ID
		chain, err := f.investmentSvc.CorrectionChain(ctx, 1, payment.ID)
		require.NoError(t, err)
		require.Equal(t, step.status, chain.EffectiveDividend.Attribution.Status, "ex date %+v", step.exDate)
		require.Equal(t, step.reasons, chain.EffectiveDividend.Attribution.Reasons, "ex date %+v", step.exDate)
		require.False(t, chain.EffectiveDividend.Attribution.Unavailable, "the payer is still known")
		run := mustRunInvestmentSelfCheck(t, fixture)
		require.Equal(t, step.status, resultFor(t, run, CheckInvestmentAttribution).Status)
		require.Equal(t, SelfCheckPassed, resultFor(t, run, CheckInvestmentFoundation).Status)
		require.Equal(t, balance, accountBalance(t, fixture, f.cashAccountID), "attribution repair never changes cash")
	}
}
