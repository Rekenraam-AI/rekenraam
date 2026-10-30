package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

type postedCurrencyFixture struct {
	*investmentsTestFixture
	pricing *PricingService
	foreign int64
	other   int64
}

func newPostedCurrencyFixture(t *testing.T) postedCurrencyFixture {
	t.Helper()
	f := newInvestmentsTestFixture(t)
	foreign := seedTestCurrencyCommodity(t, f.database, "USD")
	_, err := f.database.Exec(`UPDATE books SET default_currency_commodity_id=? WHERE id=1`, f.eurCommodityID)
	require.NoError(t, err)
	pricing := NewPricingService(db.NewPricingRepository(f.database))
	now := func() time.Time { return time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC) }
	pricing.SetNowForTest(now)
	f.transactionService.SetNowForTest(now)
	f.transactionService.SetPricingRepository(db.NewPricingRepository(f.database))
	// Receivables/payables are the supported accounts without a fixed
	// commodity; their postings can hold several currencies.
	f.cashAccountID = seedTestAccountWithClass(t, f.database, "active", true, "asset", "receivable")
	return postedCurrencyFixture{f, pricing, foreign, seedTestAccountWithClass(t, f.database, "active", true, "equity", "equity")}
}

func (f postedCurrencyFixture) post(t *testing.T, date, status, value string, scale int) Transaction {
	t.Helper()
	quantity, err := exact.Parse(value)
	require.NoError(t, err)
	tx, err := f.transactionService.CreateTransaction(context.Background(), CreateTransactionInput{
		OwnerUserID: 1, OriginType: "internal", Spec: TransactionInput{TransactionDate: date, Status: status,
			JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
				{AccountID: f.cashAccountID, CommodityID: f.foreign, QuantityValue: quantity, QuantityScale: scale},
				{AccountID: f.other, CommodityID: f.foreign, QuantityValue: quantity.Negated(), QuantityScale: scale},
			}}}}})
	require.NoError(t, err)
	return tx
}

func (f postedCurrencyFixture) coverage(t *testing.T, date string) []PricingCoverage {
	t.Helper()
	workspace, err := f.pricing.Workspace(context.Background(), date)
	require.NoError(t, err)
	return workspace.Coverage
}

func TestPricingPostedCurrencyCoverageAgreesWithReportingAndAttention(t *testing.T) {
	t.Parallel()
	f := newPostedCurrencyFixture(t)
	f.post(t, "2026-06-12", "posted", "10000", 2)
	ctx := context.Background()
	report := func() NetWorthSeriesResult {
		result, err := f.transactionService.NetWorthSeries(ctx, NetWorthSeriesInput{
			StartDate: "2026-06-12", EndDate: "2026-06-12", Bucket: "day",
			Reporting: &ReportingCurrencyInput{CommodityID: f.eurCommodityID}})
		require.NoError(t, err)
		return result
	}
	attentionRates := func() int {
		result := NewAttentionService(attentionTestReader{}, f.pricing).Read(ctx)
		require.True(t, result.Complete)
		for _, source := range result.Sources {
			if source.ID == "rates" {
				require.NotNil(t, source.Count)
				return *source.Count
			}
		}
		t.Fatal("rates source missing")
		return -1
	}
	assert.Nil(t, report().Buckets[0].Converted, "real reporting consumer needs the foreign rate")
	coverage := f.coverage(t, "")
	require.Len(t, coverage, 1, "non-default posted currency must be discovered without an existing series")
	assert.Equal(t, f.foreign, coverage[0].BaseCommodityID)
	assert.Equal(t, f.eurCommodityID, coverage[0].QuoteCommodityID)
	assert.Equal(t, "missing", coverage[0].Status)
	assert.Equal(t, 1, attentionRates())
	price, err := f.pricing.CreateManualPrice(ctx, ManualPriceInput{OwnerUserID: 1,
		BaseCommodityID: f.foreign, QuoteCommodityID: f.eurCommodityID, ValuationDate: "2026-06-12",
		PriceValue: "125", PriceScale: 2, BaseQuantityValue: "1", Reason: "Synthetic FX assertion"})
	require.NoError(t, err)
	coverage = f.coverage(t, "")
	require.Len(t, coverage, 1, "posted exposure and series must deduplicate")
	assert.Equal(t, "usable", coverage[0].Status)
	assert.Equal(t, price.ID, coverage[0].Observation.ID)
	assert.Zero(t, attentionRates())
	converted := report().Buckets[0].Converted
	require.NotNil(t, converted)
	assert.Zero(t, exact.ScaledIntFromCoefficient(converted.QuantityValue, converted.QuantityScale).Cmp(exact.ScaledIntFromInt64(125, 0)))
}

func TestPricingPostedCurrencyDiscoveryScope(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"posted", "closed_nonzero", "closed_zero", "liability", "zero_mixed_scales", "tiny_exact_exposure", "wide_exact_exposure", "future", "draft", "deleted", "voided", "category_only"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newPostedCurrencyFixture(t)
			date, status := "2026-06-12", "posted"
			if name == "future" {
				date = "2026-06-13"
			}
			if name == "draft" {
				status = "draft"
			}
			if name == "liability" {
				f.cashAccountID = seedTestAccountWithClass(t, f.database, "active", true, "liability", "payable")
			}
			if name == "category_only" {
				f.cashAccountID = f.incomeAccountID
			}
			value, scale := "10000", 2
			if name == "wide_exact_exposure" {
				value, scale = "999999999999999999999999", 0
			}
			tx := f.post(t, date, status, value, scale)
			switch name {
			case "closed_nonzero", "closed_zero":
				if name == "closed_zero" {
					f.post(t, date, "posted", "-100000", 3)
				}
				_, err := f.database.Exec(`INSERT INTO account_versions(account_id,version_seq,effective_from,recorded_at,changed_by_user_id,change_reason,status,opened_on,closed_on,name,account_class,account_kind,allows_postings)
					VALUES(?,2,'2026-06-12','2026-06-12T12:00:00Z',1,'closed fixture','closed','2026-01-01','2026-06-12','Closed receivable','asset','receivable',1)`, f.cashAccountID)
				require.NoError(t, err)
			case "zero_mixed_scales":
				f.post(t, date, "posted", "-100000", 3)
			case "tiny_exact_exposure":
				f.post(t, date, "posted", "-99999999", 6)
			case "wide_exact_exposure":
				f.post(t, date, "posted", "-999999999999999999999998999999", 6)
			case "deleted":
				_, err := f.transactionService.SoftDeleteTransaction(context.Background(), TransactionLifecycleInput{OwnerUserID: 1, OriginType: "internal", TransactionID: tx.ID, ChangeReason: "scope fixture"})
				require.NoError(t, err)
			case "voided":
				_, err := f.transactionService.VoidTransaction(context.Background(), VoidTransactionInput{OwnerUserID: 1, OriginType: "internal", TransactionID: tx.ID, ChangeReason: "scope fixture"})
				require.NoError(t, err)
			}
			want := name == "posted" || name == "closed_nonzero" || name == "liability" || name == "tiny_exact_exposure" || name == "wide_exact_exposure"
			assert.Len(t, f.coverage(t, "2026-06-12"), map[bool]int{true: 1, false: 0}[want])
		})
	}
}

func TestPricingPostedCurrencyUsesOwnerDayAndExplicitValuationDate(t *testing.T) {
	t.Parallel()
	for _, zone := range []string{"Europe/Amsterdam", "America/Los_Angeles"} {
		t.Run(zone, func(t *testing.T) {
			t.Parallel()
			f := newPostedCurrencyFixture(t)
			_, err := f.database.Exec(`INSERT INTO user_preferences(user_id,time_zone,created_at,created_by_user_id,updated_at,updated_by_user_id)
				VALUES(1,?,'2026-06-12T00:00:00Z',1,'2026-06-12T00:00:00Z',1)
				ON CONFLICT(user_id) DO UPDATE SET time_zone=excluded.time_zone`, zone)
			require.NoError(t, err)
			f.pricing.SetSettings(NewSettingsService(db.NewSettingsRepository(f.database)))
			f.pricing.SetNowForTest(func() time.Time { return time.Date(2026, 6, 12, 22, 30, 0, 0, time.UTC) })
			f.post(t, "2026-06-13", "posted", "100", 2)
			want := 0
			if zone == "Europe/Amsterdam" {
				want = 1
			}
			assert.Len(t, f.coverage(t, ""), want)
			assert.Empty(t, f.coverage(t, "2026-06-12"))
			assert.Len(t, f.coverage(t, "2026-06-13"), 1, "explicit future management date remains a diagnostic")
		})
	}
}

func TestPricingPostedCurrencyIgnoresSupersededPostingVersions(t *testing.T) {
	t.Parallel()
	f := newPostedCurrencyFixture(t)
	tx := f.post(t, "2026-06-12", "posted", "100", 2)
	require.Len(t, f.coverage(t, ""), 1)
	_, err := f.transactionService.UpdateTransaction(context.Background(), UpdateTransactionInput{
		OwnerUserID: 1, OriginType: "internal", TransactionID: tx.ID, ExpectedVersionID: tx.VersionID,
		Spec: TransactionInput{TransactionDate: "2026-06-12", Status: "posted", JournalEntries: []JournalEntryInput{{Postings: []PostingInput{
			{AccountID: f.cashAccountID, CommodityID: f.eurCommodityID, QuantityValue: exact.New(100), QuantityScale: 2},
			{AccountID: f.other, CommodityID: f.eurCommodityID, QuantityValue: exact.New(-100), QuantityScale: 2},
		}}}}})
	require.NoError(t, err)
	assert.Empty(t, f.coverage(t, ""), "retired foreign postings create no current obligation")
}

func TestPricingPostedCurrencyRecoveryRetainsObservationSelectionAndAge(t *testing.T) {
	t.Parallel()
	f := newPostedCurrencyFixture(t)
	f.post(t, "2026-06-12", "posted", "100", 2)
	input := ManualPriceInput{OwnerUserID: 1, BaseCommodityID: f.foreign, QuoteCommodityID: f.eurCommodityID,
		ValuationDate: "2026-01-01", PriceValue: "125", PriceScale: 2, BaseQuantityValue: "1", Reason: "Synthetic rate"}
	old, err := f.pricing.CreateManualPrice(context.Background(), input)
	require.NoError(t, err)
	input.ValuationDate = "2026-06-13"
	_, err = f.pricing.CreateManualPrice(context.Background(), input)
	require.NoError(t, err)
	coverage := f.coverage(t, "")
	require.Len(t, coverage, 1)
	assert.Equal(t, "stale", coverage[0].Status, "future quote cannot replace a stale candidate")
	assert.Equal(t, old.ID, coverage[0].Observation.ID)
	input.ValuationDate = "2026-06-12"
	valid, err := f.pricing.CreateManualPrice(context.Background(), input)
	require.NoError(t, err)
	coverage = f.coverage(t, "")
	require.Len(t, coverage, 1)
	assert.Equal(t, "usable", coverage[0].Status)
	assert.Equal(t, valid.ID, coverage[0].Observation.ID)
	_, err = f.pricing.VoidPrice(context.Background(), VoidPriceInput{OwnerUserID: 1, ObservationID: valid.ID, VoidReason: "Withdraw synthetic rate", ChangeReason: "Withdraw synthetic rate"})
	require.NoError(t, err)
	assert.Equal(t, "stale", f.coverage(t, "")[0].Status, "voiding restores the existing eligible stale candidate")
}
