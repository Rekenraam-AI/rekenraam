package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

func TestCashInLieuBaselinePreservesOriginalParent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := newCashParentCase(t, "split", false)
	f := c.f
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
	require.NoError(t, err)
	read := func() string {
		var fact string
		require.NoError(t, f.database.QueryRow(`SELECT json_array(operation_id,book_id,settled_operation_id,payment_on,created_audit_event_id) FROM investment_cash_in_lieu_facts`).Scan(&fact))
		return fact
	}
	original := read()
	require.NoError(t, db.Migrate(ctx, f.database))
	require.Equal(t, original, read())
	_, impact, err := c.replace("2026-06-01", 5, 2, "", true)
	require.NoError(t, err)
	_, _, err = c.replace("2026-06-01", 5, 2, impact.GainImpact.Acknowledgement, false)
	require.NoError(t, err)
	require.Equal(t, original, read())
	_, err = f.database.Exec(`UPDATE investment_cash_in_lieu_facts SET payment_on='2026-06-06'`)
	require.ErrorContains(t, err, "immutable")
	_, err = f.database.Exec(`DELETE FROM investment_cash_in_lieu_facts`)
	require.ErrorContains(t, err, "immutable")
	require.Equal(t, original, read())
	source, err := f.investmentService.repository.TradeCorrectionContext(ctx, BookID, cil.Transaction.ID)
	require.NoError(t, err)
	require.NotEqual(t, source.SettledOperationID, source.EffectiveSettledOperationID)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestHistoricalCashEntryBaselinePreservesOriginalFacts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, true)
			f := c.f
			_, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.NoError(t, err)
			impact, err := c.reverse("", true)
			require.NoError(t, err)
			_, err = c.reverse(impact.GainImpact.Acknowledgement, false)
			require.NoError(t, err)
			before := buyReplacementPreviewSnapshot(t, f.database)
			require.NoError(t, db.Migrate(ctx, f.database))
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.NoError(t, err)
			entered := buyReplacementPreviewSnapshot(t, f.database)
			_, err = f.database.Exec(`UPDATE investment_cash_in_lieu_facts SET payment_on='2026-06-06'`)
			require.ErrorContains(t, err, "immutable")
			_, err = f.database.Exec(`DELETE FROM investment_cash_in_lieu_facts`)
			require.ErrorContains(t, err, "immutable")
			require.Equal(t, entered, buyReplacementPreviewSnapshot(t, f.database))
			run := mustRunInvestmentSelfCheck(t, f)
			attribution := resultFor(t, run, CheckInvestmentAttribution)
			require.Equal(t, SelfCheckWarning, attribution.Status)
			require.EqualValues(t, 2, attribution.WarningCount)
			require.EqualValues(t, 2, attribution.UnavailableCount)
			for _, result := range run.Results {
				if result.CheckID != CheckInvestmentAttribution && result.Status != SelfCheckNotApplicable {
					require.Equalf(t, SelfCheckPassed, result.Status, "%s: %s", result.CheckID, result.Summary)
				}
			}
			requireEffectiveMovementsConsistent(t, f)
			requireConservationOracle(t, f)
		})
	}
}
