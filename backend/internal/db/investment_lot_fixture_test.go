package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// createPostedBuyLot preserves each test's exact opening amounts while using
// the shared posted writer for its operation, journal, audit and lot effects.
func createPostedBuyLot(t *testing.T, ctx context.Context, repo *InvestmentRepository, lot CreateInvestmentLotParams) (InvestmentLotRecord, error) {
	t.Helper()
	tradingID := lotFixtureAccount(t, repo.database, lot, "commodity_trading", "equity", "equity")
	cashID := lotFixtureAccount(t, repo.database, lot, "", "asset", "checking")
	postings := []PostingSpec{
		{LineKey: "holding", AccountID: lot.AccountID, CommodityID: lot.CommodityID, QuantityValue: lot.QuantityValue, QuantityScale: lot.QuantityScale, ReconciliationStatus: "uncleared", MetadataJSON: "{}"},
		{LineKey: "security-clearing", AccountID: tradingID, CommodityID: lot.CommodityID, QuantityValue: lot.QuantityValue.Negated(), QuantityScale: lot.QuantityScale, ReconciliationStatus: "uncleared", MetadataJSON: "{}"},
	}
	if lot.CostBasisValue != 0 {
		postings = append(postings,
			PostingSpec{LineKey: "cash", AccountID: cashID, CommodityID: lot.CostCommodityID, QuantityValue: exact.New(-lot.CostBasisValue), QuantityScale: lot.CostBasisScale, ReconciliationStatus: "uncleared", MetadataJSON: "{}"},
			PostingSpec{LineKey: "cash-clearing", AccountID: tradingID, CommodityID: lot.CostCommodityID, QuantityValue: exact.New(lot.CostBasisValue), QuantityScale: lot.CostBasisScale, ReconciliationStatus: "uncleared", MetadataJSON: "{}"})
	}
	_, record, err := repo.CreateTransactionAndLot(ctx, CreateTransactionParams{
		BookID: lot.BookID, ActorUserID: lot.CreatedByUserID, AuthSessionID: lot.AuthSessionID,
		RequestID: lot.RequestID, OriginType: lot.OriginType, Operation: "investment.buy",
		CreatedAt: lot.CreatedAt, ChangeReason: lot.ChangeReason,
		AccountRuleDependencies: accountRuleDependenciesFor(t, repo.database, lot.AccountID, tradingID, cashID),
		Spec: TransactionSpec{Status: "posted", TransactionKind: "investment", InvestmentOperationKind: "buy",
			TransactionDate: lot.OpenedOn, MetadataJSON: "{}", JournalEntries: []JournalEntrySpec{
				{EntryDate: lot.OpenedOn, EntryKind: "investment", MetadataJSON: "{}", Postings: postings},
			}},
		InvestmentComponents: []InvestmentComponentSpec{
			{Kind: "gross_consideration", CommodityID: lot.CostCommodityID, AmountValue: exact.New(lot.CostBasisValue).String(), AmountScale: lot.CostBasisScale, AmountDate: lot.OpenedOn},
			{Kind: "net_settlement", CommodityID: lot.CostCommodityID, AmountValue: exact.New(-lot.CostBasisValue).String(), AmountScale: lot.CostBasisScale, AmountDate: lot.OpenedOn, CashAccountID: cashID, PostingLineKey: "cash"},
		},
	}, lot)
	return record, err
}

func lotFixtureAccount(t *testing.T, database *sql.DB, lot CreateInvestmentLotParams, role, class, kind string) int64 {
	t.Helper()
	code := "lot-fixture-" + kind
	var id int64
	err := database.QueryRow(`SELECT a.id FROM accounts a JOIN account_versions v ON v.account_id = a.id
		WHERE a.book_id = ? AND (a.system_role = NULLIF(?, '') OR v.code = ?) LIMIT 1`, lot.BookID, role, code).Scan(&id)
	if err == nil {
		return id
	}
	require.ErrorIs(t, err, sql.ErrNoRows)
	result, err := database.Exec(`INSERT INTO accounts (book_id, system_role, created_at, created_by_user_id)
		VALUES (?, NULLIF(?, ''), ?, ?)`, lot.BookID, role, lot.CreatedAt, lot.CreatedByUserID)
	require.NoError(t, err)
	id, err = result.LastInsertId()
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO account_versions (account_id, version_seq, effective_from, recorded_at,
		changed_by_user_id, change_reason, status, opened_on, code, name, account_class, account_kind, allows_postings)
		VALUES (?, 1, '0001-01-01', ?, ?, 'lot fixture', 'active', '0001-01-01', ?, ?, ?, ?, 1)`,
		id, lot.CreatedAt, lot.CreatedByUserID, code, code, class, kind)
	require.NoError(t, err)
	return id
}
