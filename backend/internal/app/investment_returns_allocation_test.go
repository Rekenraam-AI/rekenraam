package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// R13 slice 4: allocation at the closing day. Base ledger (see
// newReturnsFixture): broker cash 100 and 10 TEST. Every weight is a literal
// from the plan's allocation cases, never derived from the code under test.

// classify gives TEST an instrument version: a stock quoted in EUR.
func (f *returnsFixture) classify(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	result, err := f.database.ExecContext(ctx, `INSERT INTO investment_instruments (book_id, commodity_id, created_at, created_by_user_id)
		VALUES (?, ?, '2026-01-01T00:00:00Z', 1)`, BookID, f.stockCommodityID)
	require.NoError(t, err)
	instrumentID, err := result.LastInsertId()
	require.NoError(t, err)
	_, err = f.database.ExecContext(ctx, `INSERT INTO investment_instrument_versions (instrument_id, version_seq, effective_from, recorded_at,
		changed_by_user_id, change_reason, status, instrument_type, display_name, quote_commodity_id, trading_commodity_id, quantity_scale, price_scale)
		VALUES (?, 1, '2026-01-01', '2026-01-01T00:00:00Z', 1, 'test fixture', 'active', 'stock', 'Test Security', ?, ?, 0, 2)`,
		instrumentID, f.eurCommodityID, f.eurCommodityID)
	require.NoError(t, err)
}

// underContainer files the holding under a brokerage container account.
func (f *returnsFixture) underContainer(t *testing.T) int64 {
	t.Helper()
	containerID := seedTestAccountWithClass(t, f.database, "active", false, "asset", "brokerage")
	_, err := f.database.ExecContext(context.Background(), `INSERT INTO account_versions (account_id, version_seq, effective_from, recorded_at,
			changed_by_user_id, change_reason, status, opened_on, closed_on, code, name, account_class, account_kind, parent_account_id, allows_postings)
		SELECT account_id, 2, effective_from, recorded_at, changed_by_user_id, 'test parent', status, opened_on, closed_on, code, name,
			account_class, account_kind, ?, allows_postings
		FROM account_versions WHERE account_id = ? AND version_seq = 1`, containerID, f.holdingAccountID)
	require.NoError(t, err)
	return containerID
}

func requireAllocationRow(t *testing.T, row ReturnsAllocationRow, side string, euros int64, weight string) {
	t.Helper()
	require.Equal(t, side, row.Side)
	requireReturnsAmount(t, euros, ReturnsAmount{Amount: row.Value})
	if weight == "" {
		require.Nil(t, row.Weight)
		return
	}
	require.NotNil(t, row.Weight)
	require.Equal(t, weight, *row.Weight)
}

// A1: fully priced, grouped by the outermost portfolio account.
func TestReturnsAllocationSplitsTheClosingValue(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.classify(t)
	containerID := f.underContainer(t)
	f.price(t, "2027-01-31", 11000)

	result := f.read(t, "2026-02-01", "2027-01-31")
	allocation := result.Allocation
	require.Equal(t, "2027-01-31", allocation.Date)
	require.Empty(t, allocation.Reason)
	require.Equal(t, AllocationBasisClosingValue, allocation.Basis)
	requireReturnsAmount(t, 1200, ReturnsAmount{Amount: allocation.Denominator})
	require.Equal(t, result.ClosingValue.Amount, allocation.Denominator, "the denominator is the closing value")
	require.Empty(t, allocation.WeightsReason)
	require.Empty(t, allocation.Unknown)

	require.Len(t, allocation.ByAccount, 2)
	require.Equal(t, containerID, allocation.ByAccount[0].AccountID)
	requireAllocationRow(t, allocation.ByAccount[0], AllocationSideLong, 1100, "0.916667")
	require.Equal(t, f.brokerCashID, allocation.ByAccount[1].AccountID)
	requireAllocationRow(t, allocation.ByAccount[1], AllocationSideLong, 100, "0.083333")

	require.Len(t, allocation.ByAssetType, 2)
	require.Equal(t, "stock", allocation.ByAssetType[0].AssetType)
	requireAllocationRow(t, allocation.ByAssetType[0], AllocationSideLong, 1100, "0.916667")
	require.Equal(t, AllocationAssetTypeCash, allocation.ByAssetType[1].AssetType)
	requireAllocationRow(t, allocation.ByAssetType[1], AllocationSideLong, 100, "0.083333")

	require.Len(t, allocation.ByCurrency, 1)
	require.Equal(t, f.eurCommodityID, allocation.ByCurrency[0].CurrencyID)
	requireAllocationRow(t, allocation.ByCurrency[0], AllocationSideLong, 1200, "1.000000")
}

// A2: negative broker cash is a short row; the EUR group keeps its long
// holding and its short cash apart, weights summing to 1.
func TestReturnsAllocationNeverNetsLongAndShort(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.classify(t)
	f.post(t, "2026-12-01", f.feeID, f.brokerCashID, 15000)
	f.price(t, "2027-01-31", 11000)

	allocation := f.read(t, "2026-02-01", "2027-01-31").Allocation
	requireReturnsAmount(t, 1050, ReturnsAmount{Amount: allocation.Denominator})
	require.Len(t, allocation.ByCurrency, 2)
	require.Equal(t, f.eurCommodityID, allocation.ByCurrency[0].CurrencyID)
	requireAllocationRow(t, allocation.ByCurrency[0], AllocationSideLong, 1100, "1.047619")
	require.Equal(t, f.eurCommodityID, allocation.ByCurrency[1].CurrencyID)
	requireAllocationRow(t, allocation.ByCurrency[1], AllocationSideShort, -50, "-0.047619")
	require.Equal(t, AllocationAssetTypeCash, allocation.ByAssetType[1].AssetType)
	requireAllocationRow(t, allocation.ByAssetType[1], AllocationSideShort, -50, "-0.047619")
}

// A3: no closing price. TEST is listed by its units, outside every weight;
// weights are of the known value only.
func TestReturnsAllocationKeepsUnpricedHoldingsOut(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.classify(t)

	result := f.read(t, "2026-02-01", "2027-01-31")
	allocation := result.Allocation
	require.Equal(t, ReturnReasonValuationUnavailable, result.ClosingValue.Reason)
	require.Equal(t, AllocationBasisKnownOnly, allocation.Basis)
	requireReturnsAmount(t, 100, ReturnsAmount{Amount: allocation.Denominator})
	require.Len(t, allocation.Unknown, 1)
	require.Equal(t, f.holdingAccountID, allocation.Unknown[0].AccountID)
	require.Equal(t, f.stockCommodityID, allocation.Unknown[0].Quantity.CommodityID)
	require.Equal(t, "10", allocation.Unknown[0].Quantity.QuantityValue.String())
	require.Len(t, allocation.ByAssetType, 1)
	requireAllocationRow(t, allocation.ByAssetType[0], AllocationSideLong, 100, "1.000000")
}

// A4: a zero closing value has no shares; values stay, weights do not.
func TestReturnsAllocationWithoutPositiveValueHasNoWeights(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.classify(t)
	f.post(t, "2026-12-01", f.feeID, f.brokerCashID, 120000)
	f.price(t, "2027-01-31", 11000)

	allocation := f.read(t, "2026-02-01", "2027-01-31").Allocation
	requireReturnsAmount(t, 0, ReturnsAmount{Amount: allocation.Denominator})
	require.Equal(t, ReturnReasonNonPositiveValue, allocation.WeightsReason)
	requireAllocationRow(t, allocation.ByAccount[0], AllocationSideLong, 1100, "")
	requireAllocationRow(t, allocation.ByAccount[1], AllocationSideShort, -1100, "")
}

// A5: a security with no instrument version that day is unclassified, with
// no currency: its classification is unknown, not guessed.
func TestReturnsAllocationLeavesUnclassifiedSecuritiesUnguessed(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)

	allocation := f.read(t, "2026-02-01", "2027-01-31").Allocation
	require.Equal(t, AllocationAssetTypeUnclassified, allocation.ByAssetType[0].AssetType)
	requireAllocationRow(t, allocation.ByAssetType[0], AllocationSideLong, 1100, "0.916667")
	require.Len(t, allocation.ByCurrency, 2)
	require.Zero(t, allocation.ByCurrency[0].CurrencyID)
	requireAllocationRow(t, allocation.ByCurrency[0], AllocationSideLong, 1100, "0.916667")
	require.Equal(t, f.eurCommodityID, allocation.ByCurrency[1].CurrencyID)
}

// A6: an unclassified short (negative units without short lots) makes the
// closing value unavailable, and the allocation with it.
func TestReturnsAllocationRefusesAnUnclassifiedShort(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 11000)
	_, err := f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api",
		Spec: TransactionInput{TransactionDate: "2026-12-01", JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.brokerCashID, CommodityID: f.stockCommodityID, QuantityValue: exact.New(-1), QuantityScale: 0},
			{AccountID: f.equityID, CommodityID: f.stockCommodityID, QuantityValue: exact.New(1), QuantityScale: 0},
		}}}},
	})
	require.NoError(t, err)

	result := f.read(t, "2026-02-01", "2027-01-31")
	require.Equal(t, ReturnReasonUnclassifiedShort, result.ClosingValue.Reason)
	require.Equal(t, ReturnReasonUnclassifiedShort, result.Allocation.Reason)
	require.Empty(t, result.Allocation.ByAccount)
	require.Nil(t, result.Allocation.Denominator)
}

// Review of 62f9201d, finding 1: offsetting 38-digit cash balances net to
// nothing, but each row is 10^37 EUR: written at scale 0, its exact value
// fits a coefficient, so it is never null.
func TestReturnsAllocationWritesLargeRowsAtAShallowerScale(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.classify(t)
	f.price(t, "2027-01-31", 11000)
	longID := seedTestAccountWithClass(t, f.database, "active", true, "asset", "brokerage_cash")
	shortID := seedTestAccountWithClass(t, f.database, "active", true, "asset", "brokerage_cash")
	quantity := exact.MustParse("10000000000000000000000000000000000000")
	_, err := f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api",
		Spec: TransactionInput{TransactionDate: "2026-12-01", JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: longID, CommodityID: f.eurCommodityID, QuantityValue: quantity, QuantityScale: 0},
			{AccountID: shortID, CommodityID: f.eurCommodityID, QuantityValue: quantity.Negated(), QuantityScale: 0},
		}}}},
	})
	require.NoError(t, err)

	result := f.read(t, "2026-02-01", "2027-01-31")
	requireReturnsAmount(t, 1200, result.ClosingValue)
	requireReturnsAmount(t, 1200, ReturnsAmount{Amount: result.Allocation.Denominator})
	expected := exact.ScaledIntFromCoefficient(quantity, 0)
	sides := map[string]bool{}
	for _, row := range result.Allocation.ByAccount {
		if row.AccountID != longID && row.AccountID != shortID {
			continue
		}
		require.NotNil(t, row.Value)
		value := exact.ScaledIntFromCoefficient(row.Value.QuantityValue, row.Value.QuantityScale)
		if row.Side == AllocationSideShort {
			value = value.Negated()
		}
		require.Zero(t, value.Cmp(expected), "%s %se-%d", row.Side, row.Value.QuantityValue, row.Value.QuantityScale)
		sides[row.Side] = true
	}
	require.Equal(t, map[string]bool{AllocationSideLong: true, AllocationSideShort: true}, sides)
}

// Review of 62f9201d, finding 1: an amount no coefficient can carry at any
// scale fails the read with LEDGER_OVERFLOW instead of reading as missing.
// 10^37 units at 10.01 are 1.001 × 10^38 EUR: 39 digits at scale 0.
func TestReturnsFailWithLedgerOverflowBeyondCoefficientRange(t *testing.T) {
	t.Parallel()
	f := newReturnsFixture(t)
	f.price(t, "2027-01-31", 1001)
	_, err := f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: f.ownerUserID, OriginType: "browser_api",
		Spec: TransactionInput{TransactionDate: "2026-12-01", JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.brokerCashID, CommodityID: f.stockCommodityID, QuantityValue: exact.MustParse("10000000000000000000000000000000000000"), QuantityScale: 0},
			{AccountID: f.equityID, CommodityID: f.stockCommodityID, QuantityValue: exact.MustParse("-10000000000000000000000000000000000000"), QuantityScale: 0},
		}}}},
	})
	require.NoError(t, err)

	_, err = f.investmentService.ReadReturns(context.Background(), ReturnsInput{From: "2026-02-01", To: "2027-01-31", ReportingCommodityID: f.eurCommodityID})
	var overflow LedgerOverflowError
	require.ErrorAs(t, err, &overflow)
}
