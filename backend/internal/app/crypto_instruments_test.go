package app

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
	"rekenraam/backend/migrations"
)

func TestCryptoInstrumentCreatesScale24CommodityAndPreservesLots(t *testing.T) {
	t.Parallel()
	f := newInvestTestFixture(t)
	ctx := context.Background()
	instrument, holding := cryptoTestHolding(t, f)
	var kind, effective string
	var scale int
	require.NoError(t, f.database.QueryRow(`SELECT c.kind,cv.max_quantity_scale,iiv.effective_from FROM commodities c JOIN current_commodity_versions cv ON cv.commodity_id=c.id JOIN investment_instruments ii ON ii.commodity_id=c.id JOIN current_investment_instrument_versions iiv ON iiv.instrument_id=ii.id WHERE c.id=?`, instrument.CommodityID).Scan(&kind, &scale, &effective))
	assert.Equal(t, "crypto", kind)
	assert.Equal(t, 24, scale)
	assert.Equal(t, db.CommodityGenesisDate, effective)
	input := InvestmentTradeInput{OwnerUserID: 1, TransactionDate: "2026-06-12", CommodityID: instrument.CommodityID, HoldingAccountID: holding.ID, CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, QuantityValue: exact.MustParse("1000000000000000000000001"), QuantityScale: 24, CashAmountValue: 10000, CashAmountScale: 2}
	_, err := f.investmentSvc.Buy(ctx, input)
	require.NoError(t, err)
	input.QuantityValue = exact.MustParse("1")
	input.CashAmountValue = 1
	_, err = f.investmentSvc.Sell(ctx, input)
	require.NoError(t, err)
	requireKnownBasisSelfCheckPasses(t, f.database)

	lots, err := f.investmentSvc.ListLots(ctx, holding.ID, instrument.CommodityID)
	require.NoError(t, err)
	require.Len(t, lots, 1)
	assert.Equal(t, exact.MustParse("1000000000000000000000000"), lots[0].RemainingQuantityValue)
	_, err = f.investmentSvc.CreateInstrument(ctx, InvestmentInstrumentInput{OwnerUserID: 1, OriginType: "browser_api", InstrumentType: "crypto", DisplayName: "Too precise", CommodityCode: "OVER", QuantityScale: 25})
	require.Error(t, err)
	_, err = f.investmentSvc.UpdateInstrument(ctx, InvestmentInstrumentInput{OwnerUserID: 1, OriginType: "browser_api", InstrumentID: instrument.ID, InstrumentType: "stock", DisplayName: instrument.DisplayName, QuantityScale: 12})
	require.Error(t, err)
}

func cryptoTestHolding(t *testing.T, f *investTestFixture) (InvestmentInstrument, Account) {
	t.Helper()
	ctx := context.Background()
	instrument, err := f.investmentSvc.CreateInstrument(ctx, InvestmentInstrumentInput{OwnerUserID: 1, OriginType: "browser_api", InstrumentType: "crypto", DisplayName: "Bitcoin", CommodityCode: "CRYPTO-BTC", Symbol: "BTC", QuantityScale: 24, PriceScale: 12, QuoteCommodityID: &f.eurCommodityID, IdentifiersJSON: `{"coingecko_id":"bitcoin"}`})
	require.NoError(t, err)
	holding, err := f.investmentSvc.CreateHoldingAccount(ctx, HoldingAccountInput{OwnerUserID: 1, InstrumentID: instrument.ID, Name: "Bitcoin wallet", OpenedOn: "2020-01-01", EffectiveFrom: "2020-01-01"})
	require.NoError(t, err)
	return instrument, holding
}
func TestCryptoCSVStagesCommitsLotsAndDeduplicates(t *testing.T) {
	t.Parallel()
	f := newInvestTestFixture(t)
	ctx := context.Background()
	instrument, holding := cryptoTestHolding(t, f)
	cash, err := f.investmentSvc.accountService.CreateAccount(ctx, CreateAccountInput{OwnerUserID: 1, OriginType: "browser_api", Operation: "account.create", Name: "Crypto cash", AccountClass: "asset", AccountKind: "checking", DefaultCommodityID: &f.eurCommodityID, OpenedOn: "2020-01-01", EffectiveFrom: "2020-01-01"})
	require.NoError(t, err)
	f.cashAccountID = cash.ID
	input := StartImportInput{OwnerUserID: 1, Format: "crypto_csv", HoldingAccountID: holding.ID, CashAccountID: f.cashAccountID, Input: RawInput{Filename: "crypto.csv", Bytes: []byte("date,side,quantity,amount,external_ref,memo\n2026-06-13,SELL,0.000000000000000000000001,1.00,sell-1,test\n2026-06-12,BUY,1.000000000000000000000001,100.00,buy-1,test\n")}}
	batch, err := f.importService.StartImport(ctx, input)
	require.NoError(t, err)
	require.Len(t, batch.Rows, 2)
	var count int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&count))
	assert.Zero(t, count)
	preview, err := f.importService.PreviewCommit(ctx, PreviewCommitInput{OwnerUserID: 1, BatchID: batch.Batch.ID})
	require.NoError(t, err)
	assert.Equal(t, 2, preview.IncludableCount)
	result, err := f.importService.CommitImportBatch(ctx, CommitImportBatchInput{OwnerUserID: 1, BatchID: batch.Batch.ID})
	require.NoError(t, err)
	assert.Equal(t, 2, result.CommittedCount)
	lots, err := f.investmentSvc.repository.ListLots(ctx, BookID, holding.ID, instrument.CommodityID)
	require.NoError(t, err)
	require.Len(t, lots, 1)
	assert.Equal(t, exact.MustParse("1000000000000000000000000"), lots[0].RemainingQuantityValue)
	again, err := f.importService.StartImport(ctx, input)
	require.NoError(t, err)
	for _, row := range again.Rows {
		assert.Equal(t, "duplicate", row.DedupeStatus)
	}
	// No sell may escape as a generic currency-only transaction.
	input.Input.Bytes = []byte("date,side,quantity,amount,external_ref,memo\n2026-06-14,SELL,2,100,too-many,refuse\n")
	bad, err := f.importService.StartImport(ctx, input)
	require.NoError(t, err)
	var before int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM transactions`).Scan(&before))
	result, err = f.importService.CommitImportBatch(ctx, CommitImportBatchInput{OwnerUserID: 1, BatchID: bad.Batch.ID})
	require.NoError(t, err)
	assert.Zero(t, result.CommittedCount)
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM transactions`).Scan(&count))
	assert.Equal(t, before, count)
	// Inject failure at import identity creation, after the real buy writer
	// has built its journal and lot. They must all roll back together.
	input.Input.Bytes = []byte("date,side,quantity,amount,external_ref,memo\n2026-06-15,BUY,1,100,late-failure,refuse\n")
	late, err := f.importService.StartImport(ctx, input)
	require.NoError(t, err)
	_, err = f.database.Exec(`CREATE TRIGGER fail_crypto_identity BEFORE INSERT ON import_commit_identities BEGIN SELECT RAISE(ABORT,'injected identity failure'); END`)
	require.NoError(t, err)
	result, err = f.importService.CommitImportBatch(ctx, CommitImportBatchInput{OwnerUserID: 1, BatchID: late.Batch.ID})
	require.NoError(t, err)
	assert.Zero(t, result.CommittedCount)
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM transactions`).Scan(&count))
	assert.Equal(t, before, count)
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_lots`).Scan(&count))
	assert.Equal(t, 1, count)

}
func TestCryptoCSVRejectsUnsupportedRowsBeforeStaging(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, side, quantity, amount string }{{"staking", "STAKE", "1", "1"}, {"negative quantity", "BUY", "-1", "1"}, {"negative cash", "BUY", "1", "-1"}, {"quantity precision", "BUY", "0.0000000000000000000000001", "1"}, {"cash precision", "BUY", "1", "0.0000000000001"}, {"decimal comma", "BUY", "1,5", "1"}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&CryptoCSVAdapter{}).Parse(context.Background(), RawInput{Bytes: []byte(fmt.Sprintf("date,side,quantity,amount,external_ref,memo\n2026-06-12,%s,%s,%s,ref,memo\n", tc.side, tc.quantity, tc.amount))}, nil)
			require.Error(t, err)
		})
	}
}

func TestCryptoMigrationRefusesLossyDowngradeAtomically(t *testing.T) {
	t.Parallel()
	f := newInvestTestFixture(t)
	instrument, _ := cryptoTestHolding(t, f)
	provider, err := goose.NewProvider(goose.DialectSQLite3, f.database, migrations.FS)
	require.NoError(t, err)
	_, err = provider.DownTo(context.Background(), 5)
	require.Error(t, err)
	var scale int
	require.NoError(t, f.database.QueryRow(`SELECT quantity_scale FROM current_investment_instrument_versions WHERE instrument_id=?`, instrument.ID).Scan(&scale))
	assert.Equal(t, 24, scale)
	var count int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM market_data_sources WHERE code IN ('coingecko','twelve_data','yahoo_finance')`).Scan(&count))
	assert.Equal(t, 3, count)
}

func TestCryptoCSVBackdatedBuyDisclosesGainsAndHoldsUntilAcknowledged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestTestFixture(t)
	instrument, holding := cryptoTestHolding(t, f)
	cash, err := f.investmentSvc.accountService.CreateAccount(ctx, CreateAccountInput{OwnerUserID: 1, OriginType: "browser_api", Operation: "account.create", Name: "Crypto replay cash", AccountClass: "asset", AccountKind: "checking", DefaultCommodityID: &f.eurCommodityID, OpenedOn: "2020-01-01", EffectiveFrom: "2020-01-01"})
	require.NoError(t, err)
	trade := InvestmentTradeInput{OwnerUserID: 1, TransactionDate: "2026-06-12", CommodityID: instrument.CommodityID, HoldingAccountID: holding.ID, CashAccountID: cash.ID, CashCommodityID: f.eurCommodityID, QuantityValue: exact.New(10), CashAmountValue: 100000, CashAmountScale: 2}
	_, err = f.investmentSvc.Buy(ctx, trade)
	require.NoError(t, err)
	trade.TransactionDate = "2026-06-20"
	trade.QuantityValue = exact.New(1)
	trade.CashAmountValue = 20000
	trade.CostBasisMethod = "lifo"
	_, err = f.investmentSvc.Sell(ctx, trade)
	require.NoError(t, err)
	staged, err := f.importService.StartImport(ctx, StartImportInput{OwnerUserID: 1, Format: "crypto_csv", HoldingAccountID: holding.ID, CashAccountID: cash.ID, Input: RawInput{Filename: "late.csv", Bytes: []byte("date,side,quantity,amount,external_ref,memo\n2026-06-15,BUY,10,100.00,late-buy,Changes LIFO basis\n")}})
	require.NoError(t, err)
	preview, err := f.importService.PreviewCommit(ctx, PreviewCommitInput{OwnerUserID: 1, BatchID: staged.Batch.ID})
	require.NoError(t, err)
	require.Len(t, preview.GainImpacts, 1)
	require.NotEmpty(t, preview.GainImpacts[0].GainImpact.Changes)
	command := CommitImportBatchInput{OwnerUserID: 1, BatchID: staged.Batch.ID}
	held, err := f.importService.CommitImportBatch(ctx, command)
	require.NoError(t, err)
	assert.Zero(t, held.CommittedCount)
	assert.Equal(t, []int64{staged.Rows[0].ID}, held.GainReviewRowIDs)
	command.GainImpactAcknowledgements = map[int64]string{staged.Rows[0].ID: "stale"}
	held, err = f.importService.CommitImportBatch(ctx, command)
	require.NoError(t, err)
	assert.Zero(t, held.CommittedCount)
	assert.Equal(t, []int64{staged.Rows[0].ID}, held.GainReviewRowIDs)
	command.GainImpactAcknowledgements[staged.Rows[0].ID] = preview.GainImpacts[0].GainImpact.Acknowledgement
	committed, err := f.importService.CommitImportBatch(ctx, command)
	require.NoError(t, err)
	assert.Equal(t, 1, committed.CommittedCount)
	requireKnownBasisSelfCheckPasses(t, f.database)
}

func TestQuoteIdentifierEditPreservesExactUnrelatedMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestTestFixture(t)
	const extra = `{"unrelated":99999999999999999999999999999999999999,"nested":{"value":9007199254740993}}`
	instrument, err := f.investmentSvc.CreateInstrument(ctx, InvestmentInstrumentInput{OwnerUserID: 1, OriginType: "browser_api", CommodityCode: "EXACT-META", InstrumentType: "crypto", DisplayName: "Exact metadata", QuantityScale: 24, PriceScale: 12, IdentifiersJSON: extra, MetadataJSON: extra, QuoteIdentifiers: map[string]string{"coingecko_id": "bitcoin"}})
	require.NoError(t, err)
	updated, err := f.investmentSvc.UpdateInstrument(ctx, InvestmentInstrumentInput{OwnerUserID: 1, OriginType: "browser_api", InstrumentID: instrument.ID, InstrumentType: "crypto", DisplayName: instrument.DisplayName, QuantityScale: 24, PriceScale: 12, QuoteIdentifiers: map[string]string{"coingecko_id": "ethereum", "yahoo_symbol": ""}})
	require.NoError(t, err)
	assert.Equal(t, extra, updated.MetadataJSON)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(updated.IdentifiersJSON), &fields))
	assert.Equal(t, "99999999999999999999999999999999999999", string(fields["unrelated"]))
	assert.Equal(t, `{"value":9007199254740993}`, string(fields["nested"]))
	assert.Equal(t, `"ethereum"`, string(fields["coingecko_id"]))
	original, err := f.investmentSvc.repository.InstrumentByID(ctx, BookID, instrument.ID)
	require.NoError(t, err)
	assert.Equal(t, updated.IdentifiersJSON, original.IdentifiersJSON)
}
