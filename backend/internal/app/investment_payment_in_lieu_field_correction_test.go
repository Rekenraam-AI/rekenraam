package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// #395 slice 2: a payment in lieu's date, cash account and currency are
// financial inputs the owner may correct, as for a cash dividend (slice 1).
// The payment stays lot-free; its attribution keeps #388's patch semantics.

func TestReplacePaymentInLieuCorrectsDateCashAccountAndCurrency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	usd := seedTestCurrencyCommodity(t, f.database, "USD")
	brokerCash := seedTestAccount(t, f.database, "active", true)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	recovery := seedTestAccountWithClass(t, f.database, "active", true, "income", "income")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	lot := shortLotState(t, f)
	payment, err := f.investmentService.PaymentInLieu(ctx, withRecovery(paymentInLieuInput(f, expense, 5000), recovery, 900))
	require.NoError(t, err)

	replacement := withRecovery(paymentInLieuInput(f, expense, 5500), recovery, 500)
	replacement.TransactionDate = "2026-04-10"
	replacement.CashAccountID = brokerCash
	replacement.CashCommodityID = usd
	input := ReplaceInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: payment.ID,
		Reason: "charged a week later, in USD, by the broker", Replacement: replacement}
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplacePaymentInLieuReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Empty(t, impact.AffectedCheckpoints)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "preview writes nothing")

	result, err := f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "2026-04-03", result.Inverse.TransactionDate, "the inverse cancels the original on its own date")
	require.Equal(t, "2026-04-10", result.Replacement.TransactionDate)
	for accountID, date := range postingDates(t, f, result.Inverse.ID) {
		require.Equal(t, "2026-04-03", date, "inverse posting on account %d", accountID)
	}

	// Independent figures: the EUR payment nets to zero; the USD replacement
	// charges exactly 55.00, recovers 5.00 and pays 50.00 from the broker.
	require.Equal(t, int64(100000), accountCommodityBalance(t, f, f.cashAccountID, f.eurCommodityID), "only the short proceeds remain")
	require.Zero(t, accountCommodityBalance(t, f, expense, f.eurCommodityID))
	require.Zero(t, accountCommodityBalance(t, f, recovery, f.eurCommodityID))
	require.Equal(t, int64(-5000), accountCommodityBalance(t, f, brokerCash, usd))
	require.Equal(t, int64(5500), accountCommodityBalance(t, f, expense, usd))
	require.Equal(t, int64(-500), accountCommodityBalance(t, f, recovery, usd))
	require.Equal(t, lot, shortLotState(t, f), "a payment in lieu never touches the short lot")

	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, result.Replacement.ID, *chain.EffectiveTransactionID)
	terms := chain.EffectivePaymentInLieu
	require.NotNil(t, terms)
	require.Equal(t, "2026-04-10", terms.EventDate)
	require.Equal(t, brokerCash, terms.CashAccountID)
	require.Equal(t, usd, terms.CashCommodityID)
	require.Equal(t, "2026-04-01", terms.ExDate, "omitted attribution keeps the original's")
	requireInvestmentSelfCheckPasses(t, f)

	_, err = f.investmentService.ReversePaymentInLieu(ctx, ReverseInvestmentPaymentInLieuInput{
		OwnerUserID: f.ownerUserID, TransactionID: result.Replacement.ID, Reason: "charged in error"})
	require.NoError(t, err)
	require.Zero(t, accountCommodityBalance(t, f, brokerCash, usd))
	require.Zero(t, accountCommodityBalance(t, f, expense, usd))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestReplacePaymentInLieuDateAcrossReconciledBoundaryRequiresOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	shortSaleOn(t, f, "2026-03-02", 10, 100000)
	payment, err := f.investmentService.PaymentInLieu(ctx, paymentInLieuInput(f, expense, 5000))
	require.NoError(t, err)
	checkpointID := reconcileCash(t, f, "2026-04-30", 950)

	replacement := paymentInLieuInput(f, expense, 5000)
	replacement.TransactionDate = "2026-05-10"
	input := ReplaceInvestmentPaymentInLieuInput{OwnerUserID: f.ownerUserID, TransactionID: payment.ID,
		Reason: "debited after the statement", Replacement: replacement}
	impact, err := f.investmentService.ReplacePaymentInLieuReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Len(t, impact.AffectedCheckpoints, 1)
	require.Equal(t, checkpointID, impact.AffectedCheckpoints[0].CheckpointID)
	_, err = f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	require.Equal(t, []int64{checkpointID}, activeCheckpointIDs(t, f))
	require.Equal(t, int64(95000), accountCommodityBalance(t, f, f.cashAccountID, f.eurCommodityID), "refusal writes nothing")

	input.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{checkpointID}}
	_, err = f.investmentService.ReplacePaymentInLieu(ctx, input)
	require.NoError(t, err)
	require.Empty(t, activeCheckpointIDs(t, f))
	require.Equal(t, int64(95000), accountCommodityBalance(t, f, f.cashAccountID, f.eurCommodityID))
	requireInvestmentSelfCheckPasses(t, f)
}

// A provider-linked payment keeps its single payment identity and evidence
// when its cash facts are corrected: the provider never claimed them, the
// owner's completion did, so there is nothing to disclose against.
func TestPaymentInLieuFieldCorrectionKeepsProviderLinkAndDedupe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newDividendEventFixture(t)
	f.goShort(t, f.holding.ID)
	expense := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	brokerCash := seedTestAccount(t, f.database, "active", true)
	input := PaymentInLieuInput{OwnerUserID: 1, HoldingAccountID: NullableValue(f.holding.ID), CommodityID: NullableValue(f.instrument.CommodityID),
		ExDate: NullableValue("2026-07-29"), TransactionDate: "2026-08-14", CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID,
		ExpenseAccountID: expense, AmountValue: 800, AmountScale: 2}
	payment, err := f.investmentSvc.PaymentInLieu(ctx, input)
	require.NoError(t, err)
	f.fetch(t)
	july := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	_, err = f.investmentSvc.LinkSuggestion(ctx, 1, 0, "link", july.ID, payment.ID)
	require.NoError(t, err)
	var raw string
	require.NoError(t, f.database.QueryRow(`SELECT raw_json FROM investment_provider_events WHERE id = ?`, july.ProviderEventID).Scan(&raw))

	replacement := input
	replacement.TransactionDate, replacement.CashAccountID = "2026-08-20", brokerCash
	result, err := f.investmentSvc.ReplacePaymentInLieu(ctx, ReplaceInvestmentPaymentInLieuInput{OwnerUserID: 1, TransactionID: payment.ID,
		Reason: "debited later from the broker account", Replacement: replacement})
	require.NoError(t, err)
	require.Equal(t, "2026-08-20", result.Replacement.TransactionDate)
	chain, err := f.investmentSvc.CorrectionChain(ctx, 1, payment.ID)
	require.NoError(t, err)
	require.Equal(t, SelfCheckPassed, chain.EffectivePaymentInLieu.Attribution.Status, "matching attribution stays supported")
	require.Empty(t, chain.EffectivePaymentInLieu.Attribution.Reasons)

	before := countTransactions(t, f.database)
	f.fetch(t)
	require.Equal(t, before, countTransactions(t, f.database), "a refetch never posts the corrected payment again")
	linked := suggestionForExDate(t, f.suggestions(t), "2026-07-29")
	require.Equal(t, "linked", linked.Status)
	require.Equal(t, payment.ID, *linked.LinkedTransactionID)
	var newRaw string
	require.NoError(t, f.database.QueryRow(`SELECT raw_json FROM investment_provider_events WHERE id = ?`, july.ProviderEventID).Scan(&newRaw))
	require.Equal(t, raw, newRaw)
	var integrity int
	require.NoError(t, f.database.QueryRow(`SELECT count(*) FROM investment_cash_payment_identity WHERE transaction_id = ?`, result.Replacement.ID).Scan(&integrity))
	require.Equal(t, 1, integrity, "the replacement joins the original's payment identity")
}
