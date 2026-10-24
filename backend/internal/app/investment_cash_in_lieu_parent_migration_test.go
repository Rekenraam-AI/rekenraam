package app

import (
	"context"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/migrations"
	"testing"
)

func TestCashInLieuParentMigrationPreservesHistoryAndRefusesMisleadingDowngrade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := newCashParentCase(t, "split", false)
	f := c.f
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 23)
	require.NoError(t, err)
	cil, err := f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
	require.NoError(t, err)
	read := func() string {
		var fact string
		require.NoError(t, f.database.QueryRow(`SELECT json_array(operation_id,book_id,settled_operation_id,payment_on,created_audit_event_id) FROM investment_cash_in_lieu_facts`).Scan(&fact))
		return fact
	}
	original := read()
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	require.Equal(t, original, read())
	_, err = provider.DownTo(ctx, 23)
	require.NoError(t, err)
	require.Equal(t, original, read())
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	_, impact, err := c.replace("2026-06-01", 5, 2, "", true)
	require.NoError(t, err)
	_, _, err = c.replace("2026-06-01", 5, 2, impact.GainImpact.Acknowledgement, false)
	require.NoError(t, err)
	require.Equal(t, original, read())
	_, err = f.database.Exec(`UPDATE investment_cash_in_lieu_facts SET payment_on='2026-06-06'`)
	require.ErrorContains(t, err, "immutable")
	_, err = f.database.Exec(`DELETE FROM investment_cash_in_lieu_facts`)
	require.ErrorContains(t, err, "immutable")
	_, err = provider.DownTo(ctx, 23)
	require.Error(t, err)
	// Goose may have reverted newer migrations before the retained-parent
	// guard refused this older target. Current readers require the head schema.
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	require.Equal(t, original, read())
	source, err := f.investmentService.repository.TradeCorrectionContext(ctx, BookID, cil.Transaction.ID)
	require.NoError(t, err)
	require.NotEqual(t, source.SettledOperationID, source.EffectiveSettledOperationID)
	requireInvestmentSelfCheckPasses(t, f)
}

func TestHistoricalCashEntryMigrationChangesAdmissionWithoutRewritingHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, kind := range []string{"split", "share_exchange", "spin_off", "cash_and_stock_merger"} {
		t.Run(kind, func(t *testing.T) {
			c := newCashParentCase(t, kind, true)
			f := c.f
			provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
			require.NoError(t, err)
			_, err = provider.DownTo(ctx, 25)
			require.NoError(t, err)
			// Record with a current parent using the old insert policy.
			_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.NoError(t, err)
			impact, err := c.reverse("", true)
			require.NoError(t, err)
			_, err = c.reverse(impact.GainImpact.Acknowledgement, false)
			require.NoError(t, err)
			before := buyReplacementPreviewSnapshot(t, f.database)
			// The old trigger rejects entry, even with valid original scope and units.
			_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.Error(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			_, err = provider.Up(ctx)
			require.NoError(t, err)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
			_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.NoError(t, err)
			entered := buyReplacementPreviewSnapshot(t, f.database)
			// Version 25 readers already understand retained historical parents.
			// Downgrading admission alone is safe and preserves every original fact.
			_, err = provider.DownTo(ctx, 25)
			require.NoError(t, err)
			require.Equal(t, entered, buyReplacementPreviewSnapshot(t, f.database))
			_, err = f.investmentService.CashInLieu(ctx, cashInLieuInput(f, c.parent))
			require.Error(t, err)
			require.Equal(t, entered, buyReplacementPreviewSnapshot(t, f.database))
			_, err = provider.Up(ctx)
			require.NoError(t, err)
			_, err = f.database.Exec(`UPDATE investment_cash_in_lieu_facts SET payment_on='2026-06-06'`)
			require.ErrorContains(t, err, "immutable")
			_, err = f.database.Exec(`DELETE FROM investment_cash_in_lieu_facts`)
			require.ErrorContains(t, err, "immutable")
		})
	}
}
