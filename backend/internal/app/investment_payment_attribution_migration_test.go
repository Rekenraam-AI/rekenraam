package app

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/migrations"
)

func TestPaymentAttributionMigrationPreservesHistoryAndRefusesLossyDowngrade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 22)
	require.NoError(t, err)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	read := func() []string {
		var row [6]string
		require.NoError(t, f.database.QueryRow(`SELECT operation_id,book_id,holding_account_id,commodity_id,ex_date,created_audit_event_id FROM investment_payment_in_lieu_facts`).Scan(&row[0], &row[1], &row[2], &row[3], &row[4], &row[5]))
		return row[:]
	}
	original := read()
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	require.Equal(t, original, read())
	_, err = f.database.Exec(`UPDATE investment_payment_in_lieu_facts SET ex_date = '2026-03-01'`)
	require.ErrorContains(t, err, "immutable")
	_, err = f.database.Exec(`DELETE FROM investment_payment_in_lieu_facts`)
	require.ErrorContains(t, err, "immutable")
	_, err = provider.DownTo(ctx, 22)
	require.NoError(t, err)
	require.Equal(t, original, read())
	_, err = provider.Up(ctx)
	require.NoError(t, err)
	require.Equal(t, original, read())
	// A future ex-date is a warning the old schema could not represent.
	input := paymentInLieuInput(f, expense, 5000)
	input.ExDate = NullableValue("2026-05-01")
	_, err = f.investmentService.PaymentInLieu(ctx, input)
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 22)
	require.Error(t, err)
	var count int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_payment_in_lieu_facts`).Scan(&count))
	require.Equal(t, 2, count)
	// Nullable reversal facts retain the same row and audit ancestry.
	input.HoldingAccountID = NullablePatch[int64]{}
	input.CommodityID = NullablePatch[int64]{}
	input.ExDate = NullablePatch[string]{}
	unknown, err := f.investmentService.PaymentInLieu(ctx, input)
	require.NoError(t, err)
	_, err = f.investmentService.ReversePaymentInLieu(ctx, ReverseInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: unknown.ID, Reason: "duplicate"})
	require.NoError(t, err)
	_, err = provider.DownTo(ctx, 22)
	require.Error(t, err)
	old, err := f.investmentService.repository.PaymentInLieuOperationByTransactionID(ctx, BookID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, "2026-04-01", old.Fact.ExDate)
}
