package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
	"rekenraam/backend/internal/marketdata"
)

// dividendEventFixture holds 10 ASML (Twelve Data ASML/XAMS) bought on
// 2026-03-02 and points the dividend provider at a fake Twelve Data.
type dividendEventFixture struct {
	*investTestFixture
	pricing    *PricingService
	instrument InvestmentInstrument
	holding    Account
	body       atomic.Value
	calls      atomic.Int32
	logger     *slog.Logger
}

const dividendEventBody = `{"meta":{"symbol":"ASML","currency":"EUR","mic_code":"XAMS","exchange_timezone":"Europe/Amsterdam"},
"dividends":[{"ex_date":"2026-11-02","amount":9},{"ex_date":"2026-07-29","amount":1.6},{"ex_date":"2026-04-28","amount":"bad"},{"ex_date":"2026-02-02","amount":1.5}]}`

func newDividendEventFixture(t *testing.T) *dividendEventFixture {
	t.Helper()
	ctx := context.Background()
	f := &dividendEventFixture{investTestFixture: newInvestTestFixture(t), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	f.body.Store(dividendEventBody)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Uses the single write connection: it would hang if the fetch ran
		// inside a database transaction.
		var count int
		assert.NoError(t, f.database.QueryRowContext(r.Context(), `SELECT count(*) FROM transactions`).Scan(&count))
		f.calls.Add(1)
		assert.Equal(t, "/dividends", r.URL.Path)
		_, _ = fmt.Fprint(w, f.body.Load().(string))
	}))
	t.Cleanup(server.Close)
	f.pricing = NewPricingService(db.NewPricingRepository(f.database), marketdata.NewRegistry(marketdata.NewTwelveDataProvider(server.Client(), server.URL, "secret")))
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	f.pricing.SetNowForTest(func() time.Time { return now })
	instrument, err := f.investmentSvc.CreateInstrument(ctx, InvestmentInstrumentInput{OwnerUserID: 1, OriginType: "browser_api", ChangeReason: "fixture",
		InstrumentType: "stock", DisplayName: "ASML Holding", CommodityCode: "ASML", Symbol: "ASML", MIC: "XAMS", QuantityScale: 4, PriceScale: 4,
		QuoteCommodityID: &f.eurCommodityID, IdentifiersJSON: `{"twelve_data_symbol":"ASML"}`, EffectiveFrom: "0001-01-01"})
	require.NoError(t, err)
	f.instrument = instrument
	f.holding, err = f.investmentSvc.CreateHoldingAccount(ctx, HoldingAccountInput{OwnerUserID: 1, InstrumentID: instrument.ID, Name: "ASML", OpenedOn: "2020-01-01", EffectiveFrom: "2020-01-01"})
	require.NoError(t, err)
	_, err = f.investmentSvc.Buy(ctx, InvestmentTradeInput{OwnerUserID: 1, TransactionDate: "2026-03-02", CommodityID: instrument.CommodityID, HoldingAccountID: f.holding.ID,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, QuantityValue: exact.MustParse("10"), QuantityScale: 0, CashAmountValue: 600000, CashAmountScale: 2})
	require.NoError(t, err)
	var sourceID int64
	require.NoError(t, f.database.QueryRow(`SELECT id FROM market_data_sources WHERE code='twelve_data'`).Scan(&sourceID))
	_, err = f.pricing.SaveSourceAssignment(ctx, PricingSourceAssignmentInput{OwnerUserID: 1, CommodityID: instrument.CommodityID, QuoteCommodityID: f.eurCommodityID, SourceID: sourceID, EffectiveFrom: "2020-01-01"})
	require.NoError(t, err)
	return f
}

func (f *dividendEventFixture) fetch(t *testing.T) {
	t.Helper()
	queued, err := f.pricing.QueueDividendEventFetch(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, queued)
	f.pricing.runDueDividendEvents(context.Background(), f.logger, "worker")
}

func (f *dividendEventFixture) suggestions(t *testing.T) []InvestmentEventSuggestion {
	t.Helper()
	suggestions, err := f.investmentSvc.ListEventSuggestions(context.Background())
	require.NoError(t, err)
	return suggestions
}

func suggestionForExDate(t *testing.T, suggestions []InvestmentEventSuggestion, exDate string) InvestmentEventSuggestion {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.EventDate == exDate {
			return suggestion
		}
	}
	require.Failf(t, "missing suggestion", "no suggestion for ex-date %s", exDate)
	return InvestmentEventSuggestion{}
}

func (f *dividendEventFixture) completion(date string, gross int64) *SuggestionCompletion {
	return &SuggestionCompletion{TransactionDate: date, CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, AmountValue: gross, AmountScale: 2, IncomeAccountID: &f.categoryAccountID}
}

func TestDividendProducerStagesHeldExDatesAsIncompleteEvidence(t *testing.T) {
	t.Parallel()
	f := newDividendEventFixture(t)
	f.fetch(t)

	// 2026-02-02 predates the holding and 2026-11-02 has not gone ex yet.
	suggestions := f.suggestions(t)
	require.Len(t, suggestions, 2)
	july := suggestionForExDate(t, suggestions, "2026-07-29")
	assert.Equal(t, "suggested", july.Status)
	assert.Equal(t, "dividend", july.EventFamily)
	assert.Equal(t, "twelve_data", july.SourceCode)
	assert.Equal(t, "new", july.EventStatus)
	assert.Equal(t, 7000, july.ConfidenceBPS)
	var evidence providerDividendEvidence
	require.NoError(t, json.Unmarshal([]byte(july.EventEvidenceJSON), &evidence))
	assert.Equal(t, "2026-07-28", evidence.EntitlementDate)
	assert.Equal(t, &providerDecimal{Value: "16", Scale: 1}, evidence.AmountPerShare)
	assert.Equal(t, &providerDecimal{Value: "10", Scale: 0}, evidence.EntitledUnits)
	assert.Equal(t, &providerDecimal{Value: "16", Scale: 0}, evidence.EstimatedGross)
	assert.Equal(t, "EUR", evidence.ListingCurrency)
	require.Len(t, evidence.Holdings, 1)
	assert.Equal(t, f.holding.ID, evidence.Holdings[0].AccountID)
	assert.ElementsMatch(t, []string{dividendMissingPaymentDate, dividendMissingCashAccount, dividendMissingReceivedAmount, dividendMissingCurrency}, evidence.Missing)
	assert.JSONEq(t, `{"kind":"dividend_income","complete":false,"commodity_id":`+fmt.Sprint(f.instrument.CommodityID)+
		`,"external_ref_hint":"twelve_data:dividend:ASML:XAMS:2026-07-29","missing":["payment_date","cash_account","received_amount","dividend_currency"]}`, july.ProposedTransactionJSON)

	april := suggestionForExDate(t, suggestions, "2026-04-28")
	var aprilEvidence providerDividendEvidence
	require.NoError(t, json.Unmarshal([]byte(april.EventEvidenceJSON), &aprilEvidence))
	assert.Nil(t, aprilEvidence.AmountPerShare, "an unreadable amount is kept as missing, never guessed")
	assert.Nil(t, aprilEvidence.EstimatedGross)
	assert.Contains(t, aprilEvidence.Missing, dividendMissingAmountPerShare)
	assert.Equal(t, 3000, april.ConfidenceBPS)

	var raw string
	require.NoError(t, f.database.QueryRow(`SELECT raw_json FROM investment_provider_events WHERE provider_event_id = 'twelve_data:dividend:ASML:XAMS:2026-07-29'`).Scan(&raw))
	assert.JSONEq(t, `{"meta":{"symbol":"ASML","currency":"EUR","mic_code":"XAMS"},"dividend":{"ex_date":"2026-07-29","amount":1.6},"adjust":false}`, raw)
	var dividends int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_operations WHERE operation_kind = 'dividend'`).Scan(&dividends))
	assert.Zero(t, dividends, "staging never posts")
}

func TestDividendProducerRefetchKeepsOwnerDecisionsAndNeverDuplicates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	suggestions := f.suggestions(t)
	july := suggestionForExDate(t, suggestions, "2026-07-29")
	april := suggestionForExDate(t, suggestions, "2026-04-28")
	accepted, err := f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, f.completion("2026-08-14", 1360))
	require.NoError(t, err)
	_, err = f.investmentSvc.IgnoreSuggestion(ctx, 1, 0, "req", april.ID)
	require.NoError(t, err)

	// A revised provider amount for the same ex-date does not replace the
	// first observation or reopen the owner's decisions.
	f.body.Store(`{"meta":{"symbol":"ASML","currency":"EUR","mic_code":"XAMS"},"dividends":[{"ex_date":"2026-07-29","amount":1.7},{"ex_date":"2026-04-28","amount":1.84}]}`)
	f.fetch(t)
	f.fetch(t)
	after := f.suggestions(t)
	require.Len(t, after, 2)
	july = suggestionForExDate(t, after, "2026-07-29")
	assert.Equal(t, "accepted", july.Status)
	assert.Equal(t, "matched", july.EventStatus)
	assert.Equal(t, accepted.GeneratedTransactionID, july.GeneratedTransactionID)
	assert.Contains(t, july.EventEvidenceJSON, `"value":"16"`)
	april = suggestionForExDate(t, after, "2026-04-28")
	assert.Equal(t, "ignored", april.Status)
	assert.Equal(t, "ignored", april.EventStatus)
	var events, dividends int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_provider_events`).Scan(&events))
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_operations WHERE operation_kind = 'dividend'`).Scan(&dividends))
	assert.Equal(t, 2, events)
	assert.Equal(t, 1, dividends)
	assert.Equal(t, int32(3), f.calls.Load())
}

func TestIncompleteProviderDividendNeedsOwnerCompletionAndStaysPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	before := countTransactions(t, f.database)

	_, err := f.investmentSvc.AcceptSuggestion(ctx, 1, 0, "req", july.ID)
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Contains(t, validation.Message, "incomplete")

	missingCash := f.completion("2026-08-14", 1360)
	missingCash.CashAccountID = 0
	_, err = f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, missingCash)
	require.ErrorAs(t, err, &validation)

	// The dividend writer's own refusal (an income account that is not income)
	// is returned to the owner instead of failing the suggestion.
	wrongIncome := f.completion("2026-08-14", 1360)
	wrongIncome.IncomeAccountID = &f.cashAccountID
	_, err = f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, wrongIncome)
	require.Error(t, err)

	assert.Equal(t, before, countTransactions(t, f.database))
	july = suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	assert.Equal(t, "suggested", july.Status)
	assert.Equal(t, "new", july.EventStatus)
}

func TestCompletedProviderDividendCommitsThroughNativeCommandAtomically(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	completion := f.completion("2026-08-14", 1600)
	withholding, scale := int64(240), 2
	completion.WithholdingValue, completion.WithholdingScale, completion.WithholdingAccountID = &withholding, &scale, &f.categoryAccountID
	completion.WithholdingAccountID = nil
	_, err := f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, completion)
	require.Error(t, err, "withholding without an account is refused by the native command")

	completion.WithholdingValue, completion.WithholdingScale = nil, nil
	accepted, err := f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req-accept", july.ID, completion)
	require.NoError(t, err)
	assert.Equal(t, "accepted", accepted.Status)
	assert.Equal(t, "matched", accepted.EventStatus)
	require.NotNil(t, accepted.GeneratedTransactionID)

	var payer int64
	var auditOperation, eventDate string
	require.NoError(t, f.database.QueryRow(`
		SELECT facts.commodity_id, o.event_date, a.operation
		FROM investment_operations o
		JOIN investment_dividend_facts facts ON facts.operation_id = o.id
		JOIN audit_events a ON a.id = o.created_audit_event_id
		WHERE o.operation_kind = 'dividend'`).Scan(&payer, &eventDate, &auditOperation))
	assert.Equal(t, f.instrument.CommodityID, payer, "the payer fact comes from the event instrument")
	assert.Equal(t, "2026-08-14", eventDate, "the owner's payment date, not the ex-date, is the transaction date")
	assert.Equal(t, "investment.event_suggestion.accept", auditOperation)

	_, err = f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, completion)
	require.ErrorIs(t, err, ErrInvestmentSuggestionNotPending)
	check, err := selfCheckOver(t, f.database).RunSelfCheck(ctx, "manual")
	require.NoError(t, err)
	for _, id := range []string{CheckInvestmentFoundation, CheckEntryBalance} {
		assert.Equal(t, SelfCheckPassed, resultFor(t, check, id).Status, id)
	}
}

func TestProviderDividendAcceptLateFailureRollsBackPosting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	before := countTransactions(t, f.database)
	// The event-status write is the last statement of the accept transaction.
	_, err := f.database.ExecContext(ctx, `CREATE TEMP TRIGGER refuse_event_match BEFORE UPDATE ON investment_provider_events
		BEGIN SELECT RAISE(ABORT, 'late failure'); END`)
	require.NoError(t, err)
	_, err = f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, f.completion("2026-08-14", 1600))
	require.Error(t, err)
	_, err = f.database.ExecContext(ctx, `DROP TRIGGER temp.refuse_event_match`)
	require.NoError(t, err)

	assert.Equal(t, before, countTransactions(t, f.database))
	july = suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	assert.Equal(t, "suggested", july.Status)
	assert.Nil(t, july.GeneratedTransactionID)
	accepted, err := f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, f.completion("2026-08-14", 1600))
	require.NoError(t, err, "the rolled-back accept leaves the suggestion reviewable")
	assert.Equal(t, "accepted", accepted.Status)
}

func TestProviderDividendLinksOwnerRecordedDividendOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	// The owner recorded the July payment manually before any fetch.
	manual, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: &f.instrument.CommodityID,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2})
	require.NoError(t, err)
	// A dividend recorded before the ex-date is not offered for it.
	early, err := f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: 1, TransactionDate: "2026-07-01", CommodityID: &f.instrument.CommodityID,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 100, AmountScale: 2})
	require.NoError(t, err)
	f.fetch(t)
	suggestions := f.suggestions(t)
	july := suggestionForExDate(t, suggestions, "2026-07-29")
	require.Len(t, july.LinkCandidates, 1)
	assert.Equal(t, manual.ID, july.LinkCandidates[0].TransactionID)
	assert.Equal(t, exact.MustParse("16"), july.LinkCandidates[0].GrossValue)
	assert.Equal(t, 0, july.LinkCandidates[0].GrossScale)
	april := suggestionForExDate(t, suggestions, "2026-04-28")
	require.Len(t, april.LinkCandidates, 1, "only the July 1 payment falls in April's 90-day window")
	assert.Equal(t, early.ID, april.LinkCandidates[0].TransactionID)

	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "req", july.ID, early.ID)
	var validation ValidationError
	require.ErrorAs(t, err, &validation, "a payment before the ex-date is not this dividend")
	before := countTransactions(t, f.database)
	linked, err := f.investmentSvc.LinkSuggestion(ctx, 1, 0, "req", july.ID, manual.ID)
	require.NoError(t, err)
	assert.Equal(t, "linked", linked.Status)
	assert.Equal(t, "matched", linked.EventStatus)
	assert.Equal(t, &manual.ID, linked.LinkedTransactionID)
	assert.Equal(t, before, countTransactions(t, f.database), "linking posts nothing")

	// The schema refuses a second provider event for one payment, whatever
	// the service checks.
	_, err = f.database.Exec(`UPDATE investment_event_suggestions SET linked_transaction_id = ?, status = 'linked' WHERE id = ?`, manual.ID, april.ID)
	require.Error(t, err)
	linkedEarly, err := f.investmentSvc.LinkSuggestion(ctx, 1, 0, "req", april.ID, early.ID)
	require.NoError(t, err)
	assert.Equal(t, "linked", linkedEarly.Status)
	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "req", july.ID, manual.ID)
	require.ErrorIs(t, err, ErrInvestmentSuggestionNotPending)

	f.fetch(t)
	july = suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	assert.Equal(t, "linked", july.Status, "a later fetch does not reopen a linked event")
}

func TestShortPositionProviderDividendIsInformationalOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	// Close the long and sell short before the July ex-date.
	_, err := f.investmentSvc.Sell(ctx, InvestmentTradeInput{OwnerUserID: 1, TransactionDate: "2026-06-01", CommodityID: f.instrument.CommodityID, HoldingAccountID: f.holding.ID,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, QuantityValue: exact.MustParse("10"), CashAmountValue: 600000, CashAmountScale: 2})
	require.NoError(t, err)
	_, err = f.investmentSvc.ShortSale(ctx, InvestmentTradeInput{OwnerUserID: 1, TransactionDate: "2026-06-02", CommodityID: f.instrument.CommodityID, HoldingAccountID: f.holding.ID,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, QuantityValue: exact.MustParse("5"), CashAmountValue: 300000, CashAmountScale: 2})
	require.NoError(t, err)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	assert.JSONEq(t, `{"kind":"informational","reason":"short_position"}`, july.ProposedTransactionJSON)
	assert.Contains(t, july.EventEvidenceJSON, `"unsupported":"short_position"`)
	_, err = f.investmentSvc.AcceptSuggestionWithCompletion(ctx, 1, 0, "req", july.ID, f.completion("2026-08-14", 800))
	var validation ValidationError
	require.ErrorAs(t, err, &validation)
	july = suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	assert.Equal(t, "suggested", july.Status, "an informational row stays reviewable and can be ignored")
	ignored, err := f.investmentSvc.IgnoreSuggestion(ctx, 1, 0, "req", july.ID)
	require.NoError(t, err)
	assert.Equal(t, "ignored", ignored.Status)
}

func TestDividendProducerFailureNeverBlocksManualEntry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.body.Store(`{"code":403,"message":"plan","status":"error"}`)
	f.fetch(t)
	failed, err := f.pricing.FailedBackgroundWork(ctx)
	require.NoError(t, err)
	require.Len(t, failed, 1, "a missing plan entitlement is terminal and visible")
	assert.Equal(t, dividendEventWorkKind, failed[0].Kind)
	assert.Empty(t, f.suggestions(t))
	_, err = f.investmentSvc.Dividend(ctx, DividendInput{OwnerUserID: 1, TransactionDate: "2026-08-14", CommodityID: &f.instrument.CommodityID,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, IncomeAccountID: &f.categoryAccountID, AmountValue: 1600, AmountScale: 2})
	require.NoError(t, err, "manual dividend entry is unaffected by provider coverage")

	f.body.Store(dividendEventBody)
	_, err = f.pricing.RetryBackgroundWork(ctx, failed[0].ID)
	require.NoError(t, err)
	f.pricing.runDueDividendEvents(ctx, f.logger, "worker")
	assert.Len(t, f.suggestions(t), 2)
}

func countTransactions(t *testing.T, database interface {
	QueryRow(string, ...any) *sql.Row
}) int {
	t.Helper()
	var count int
	require.NoError(t, database.QueryRow(`SELECT count(*) FROM transactions`).Scan(&count))
	return count
}
