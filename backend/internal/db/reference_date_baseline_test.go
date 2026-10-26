package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReferenceDateBaselineGuardsUseSourceDates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, ownerID, currencyID := migratedInvestmentTestDatabase(t)
	_, err := database.Exec(`INSERT INTO commodities(book_id,code,kind,is_builtin,created_at,created_by_user_id)
		VALUES(1,'DATED','security',0,'2999-01-01T00:00:00Z',?)`, ownerID)
	require.NoError(t, err)
	var securityID int64
	require.NoError(t, database.QueryRow(`SELECT id FROM commodities WHERE code='DATED'`).Scan(&securityID))
	_, err = database.Exec(`INSERT INTO commodity_versions(commodity_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,symbol,display_symbol,name,standard_scale,max_quantity_scale)
		VALUES(?,1,'2999-01-01','2999-01-01T00:00:00Z',?,'fixture','active','DATED','DATED','Dated security',0,6)`, securityID, ownerID)
	require.NoError(t, err)
	params := CreateInvestmentInstrumentParams{
		BookID: 1, CreatedByUserID: ownerID, OriginType: "browser_api", Operation: "investment.instrument.create",
		CreatedAt: "2999-01-01T00:00:00Z", EffectiveFrom: "2999-01-01", ChangeReason: "source-date control",
		Spec: InvestmentInstrumentSpec{CommodityID: sql.NullInt64{Int64: securityID, Valid: true}, InstrumentType: "stock", DisplayName: "Dated instrument", QuantityScale: 6, PriceScale: 4, IdentifiersJSON: "{}", MetadataJSON: "{}"},
	}
	dated := WithReferenceDate(ctx, "2999-01-01")
	instrument, err := NewInvestmentRepository(database).CreateInstrument(dated, params)
	require.NoError(t, err)
	assert.Equal(t, securityID, instrument.CommodityID)

	// A later precision change cannot make an earlier source version invalid.
	_, err = database.Exec(`INSERT INTO commodity_versions(commodity_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,symbol,display_symbol,name,standard_scale,max_quantity_scale)
		VALUES(?,2,'2999-01-02','2999-01-02T00:00:00Z',?,'fixture','active','DATED','DATED','Later precision',0,2)`, securityID, ownerID)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO investment_instrument_versions(instrument_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,instrument_type,display_name,quantity_scale,price_scale)
		VALUES(?,2,'2999-01-01','2999-01-01T01:00:00Z',?,'fixture','active','stock','Same source day',6,4)`, instrument.ID, ownerID)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO investment_instrument_versions(instrument_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,instrument_type,display_name,quantity_scale,price_scale)
		VALUES(?,3,'2999-01-02','2999-01-02T01:00:00Z',?,'fixture','active','stock','Later source day',6,4)`, instrument.ID, ownerID)
	require.ErrorContains(t, err, "instrument type and precision must match its commodity")

	createAccount := func(name, date string) {
		t.Helper()
		_, err := NewAccountRepository(database).CreateAccount(WithReferenceDate(ctx, date), CreateAccountParams{
			BookID: 1, CreatedByUserID: ownerID, OriginType: "browser_api", Operation: "account.create", CreatedAt: date + "T00:00:00Z", EffectiveFrom: date,
			Spec: AccountSpec{Name: name, AccountClass: "asset", AccountKind: "checking", DefaultCommodityID: sql.NullInt64{Int64: currencyID, Valid: true}, AllowsPostings: true, OpenedOn: date, MetadataJSON: "{}"},
		})
		require.NoError(t, err)
	}
	createAccount("First dated account", "2999-01-01")
	var queued int
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM background_work_items WHERE kind='pricing.fx_coverage'`).Scan(&queued))
	require.Positive(t, queued)
	_, err = database.Exec(`UPDATE background_work_items SET status='completed' WHERE kind='pricing.fx_coverage'`)
	require.NoError(t, err)
	createAccount("Second dated account", "2999-01-01")
	var after int
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM background_work_items WHERE kind='pricing.fx_coverage'`).Scan(&after))
	assert.Equal(t, queued, after, "same source-day currency activation is already covered")
	createAccount("Earlier account", "2998-12-31")
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM background_work_items WHERE kind='pricing.fx_coverage'`).Scan(&after))
	assert.Equal(t, queued+1, after, "future account versions cannot suppress earlier coverage")

	accounts, versions := countRows(t, database, "accounts"), countRows(t, database, "account_versions")
	require.NoError(t, Migrate(ctx, database))
	require.NoError(t, Migrate(ctx, database))
	assert.Equal(t, accounts, countRows(t, database, "accounts"))
	assert.Equal(t, versions, countRows(t, database, "account_versions"))
	assert.Equal(t, 0, countRows(t, database, "posting_versions"))
}
