package app

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// #395: a cash dividend's date, cash account and currency are financial
// inputs the owner may correct. The inverse cancels the original on the
// original's own date and account; the replacement posts the corrected facts.
// Payment shape (dividend vs standalone withholding) stays fixed.

// accountCommodityBalance sums an account's current postings in one commodity, in cents.
func accountCommodityBalance(t *testing.T, f *investmentsTestFixture, accountID, commodityID int64) int64 {
	t.Helper()
	rows, err := f.database.Query(`SELECT pv.quantity_value, pv.quantity_scale FROM posting_versions pv
		JOIN current_transaction_versions current ON current.id = pv.transaction_version_id
		JOIN transactions tx ON tx.id = current.transaction_id
		WHERE pv.account_id = ? AND pv.commodity_id = ? AND tx.deleted_at IS NULL`, accountID, commodityID)
	require.NoError(t, err)
	defer rows.Close()
	total := exact.ScaledIntFromInt64(0, 2)
	for rows.Next() {
		var value exact.Coefficient
		var scale int
		require.NoError(t, rows.Scan(&value, &scale))
		total.AddScaled(exact.ScaledIntFromCoefficient(value, scale))
	}
	require.NoError(t, rows.Err())
	cents, err := total.TruncatedTo(2).Int64()
	require.NoError(t, err)
	return cents
}

// postingDates lists a transaction's current posting dates per account.
func postingDates(t *testing.T, f *investmentsTestFixture, transactionID int64) map[int64]string {
	t.Helper()
	rows, err := f.database.Query(`SELECT pv.account_id, je.entry_date FROM posting_versions pv
		JOIN journal_entries je ON je.id = pv.journal_entry_id
		JOIN current_transaction_versions current ON current.id = pv.transaction_version_id
		WHERE current.transaction_id = ?`, transactionID)
	require.NoError(t, err)
	defer rows.Close()
	dates := map[int64]string{}
	for rows.Next() {
		var accountID int64
		var date string
		require.NoError(t, rows.Scan(&accountID, &date))
		dates[accountID] = date
	}
	require.NoError(t, rows.Err())
	return dates
}

func TestReplaceDividendCorrectsDateCashAccountAndCurrency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	usd := seedTestCurrencyCommodity(t, f.database, "USD")
	brokerCash := seedTestAccount(t, f.database, "active", true)
	withholding := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	original, err := f.investmentService.Dividend(ctx, cashDividendInput(f, withholding, 10000, 1500))
	require.NoError(t, err)

	replacement := cashDividendInput(f, withholding, 11000, 1650)
	replacement.TransactionDate = "2026-02-20"
	replacement.CashAccountID = brokerCash
	replacement.CashCommodityID = usd
	input := ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID, TransactionID: original.ID,
		Reason: "paid earlier, in USD, into the broker account", Replacement: replacement}
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.ReplaceDividendReconciliationImpact(ctx, input)
	require.NoError(t, err)
	require.Empty(t, impact.AffectedCheckpoints)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database), "preview writes nothing")

	result, err := f.investmentService.ReplaceDividend(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "2026-03-01", result.Inverse.TransactionDate, "the inverse cancels the original on its own date")
	require.Equal(t, "2026-02-20", result.Replacement.TransactionDate)
	for accountID, date := range postingDates(t, f, result.Inverse.ID) {
		require.Equal(t, "2026-03-01", date, "inverse posting on account %d", accountID)
	}
	require.Equal(t, "2026-02-20", postingDates(t, f, result.Replacement.ID)[brokerCash])

	// Independent figures: the EUR original nets to zero everywhere; the USD
	// replacement carries exactly 110.00 gross, 16.50 tax and 93.50 cash.
	for _, account := range []int64{f.cashAccountID, f.incomeAccountID, withholding} {
		require.Zero(t, accountCommodityBalance(t, f, account, f.eurCommodityID), "EUR on account %d", account)
	}
	require.Equal(t, int64(9350), accountCommodityBalance(t, f, brokerCash, usd))
	require.Equal(t, int64(-11000), accountCommodityBalance(t, f, f.incomeAccountID, usd))
	require.Equal(t, int64(1650), accountCommodityBalance(t, f, withholding, usd))
	require.Zero(t, accountCommodityBalance(t, f, f.cashAccountID, usd))

	var gross, tax, grossDate string
	require.NoError(t, f.database.QueryRow(`SELECT
		(SELECT amount_value || '@' || amount_scale || ':' || commodity_id FROM investment_operation_components c WHERE c.operation_id = o.id AND c.component_kind = 'dividend_gross'),
		(SELECT amount_value || '@' || amount_scale || ':' || commodity_id FROM investment_operation_components c WHERE c.operation_id = o.id AND c.component_kind = 'withholding'),
		(SELECT amount_date FROM investment_operation_components c WHERE c.operation_id = o.id AND c.component_kind = 'dividend_gross')
		FROM investment_operations o WHERE o.correction_mode = 'replace'`).Scan(&gross, &tax, &grossDate))
	require.Equal(t, "11000@2:"+strconv.FormatInt(usd, 10), gross)
	require.Equal(t, "-1650@2:"+strconv.FormatInt(usd, 10), tax)
	require.Equal(t, "2026-02-20", grossDate)

	chain, err := f.investmentService.CorrectionChain(ctx, f.ownerUserID, original.ID)
	require.NoError(t, err)
	require.Equal(t, result.Replacement.ID, *chain.EffectiveTransactionID)
	require.NotNil(t, chain.EffectiveDividend)
	require.Equal(t, "2026-02-20", chain.EffectiveDividend.EventDate)
	require.Equal(t, brokerCash, chain.EffectiveDividend.CashAccountID)
	require.Equal(t, usd, chain.EffectiveDividend.CashCommodityID)
	require.Empty(t, chain.EffectiveDividend.SourceDifferences, "a manual dividend has no imported source claim")
	requireInvestmentSelfCheckPasses(t, f)

	// The corrected payment remains correctable on its new facts.
	_, err = f.investmentService.ReverseDividend(ctx, ReverseInvestmentDividendInput{
		OwnerUserID: f.ownerUserID, TransactionID: result.Replacement.ID, Reason: "not received"})
	require.NoError(t, err)
	require.Zero(t, accountCommodityBalance(t, f, brokerCash, usd))
	require.Zero(t, accountCommodityBalance(t, f, f.incomeAccountID, usd))
	requireInvestmentSelfCheckPasses(t, f)
}

func TestReplaceStandaloneWithholdingCorrectsDateAndCashAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newInvestmentsTestFixture(t)
	brokerCash := seedTestAccount(t, f.database, "active", true)
	tax := seedTestAccountWithClass(t, f.database, "active", true, "expense", "expense")
	input := DividendInput{OwnerUserID: f.ownerUserID, TransactionDate: "2026-03-01", WithholdingOnly: true,
		CashAccountID: f.cashAccountID, CashCommodityID: f.eurCommodityID, WithholdingAccountID: &tax,
		AmountValue: 1500, AmountScale: 2, Memo: "ACME withholding"}
	original, err := f.investmentService.Dividend(ctx, input)
	require.NoError(t, err)

	replacement := input
	replacement.TransactionDate = "2026-03-10"
	replacement.CashAccountID = brokerCash
	result, err := f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
		TransactionID: original.ID, Reason: "debited later from the broker account", Replacement: replacement})
	require.NoError(t, err)
	require.Equal(t, "2026-03-10", result.Replacement.TransactionDate)
	require.Zero(t, accountCommodityBalance(t, f, f.cashAccountID, f.eurCommodityID))
	require.Equal(t, int64(-1500), accountCommodityBalance(t, f, brokerCash, f.eurCommodityID))
	require.Equal(t, int64(1500), accountCommodityBalance(t, f, tax, f.eurCommodityID), "tax stays tax, with no income leg")
	require.Zero(t, accountCommodityBalance(t, f, f.incomeAccountID, f.eurCommodityID))

	// Shape stays fixed in both directions.
	shape := replacement
	shape.WithholdingOnly = false
	shape.IncomeAccountID = &f.incomeAccountID
	_, err = f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
		TransactionID: result.Replacement.ID, Reason: "turn into a dividend", Replacement: shape})
	require.Error(t, err)
	requireInvestmentSelfCheckPasses(t, f)
}

// Moving a payment across a reconciled boundary changes that boundary's
// balance in either direction, so each needs the reviewed override. A move
// that stays inside the reconciled period nets to zero there and keeps it.
func TestReplaceDividendDateAcrossReconciledBoundaryRequiresOverride(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("out of the reconciled period", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		original, err := f.investmentService.Dividend(ctx, cashDividendInput(f, 0, 10000, 0))
		require.NoError(t, err)
		checkpointID := reconcileCash(t, f, "2026-04-01", 100)
		replacement := cashDividendInput(f, 0, 10000, 0)
		replacement.TransactionDate = "2026-05-01"
		input := ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID, TransactionID: original.ID,
			Reason: "paid after the statement", Replacement: replacement}
		impact, err := f.investmentService.ReplaceDividendReconciliationImpact(ctx, input)
		require.NoError(t, err)
		require.Len(t, impact.AffectedCheckpoints, 1)
		require.Equal(t, checkpointID, impact.AffectedCheckpoints[0].CheckpointID)
		_, err = f.investmentService.ReplaceDividend(ctx, input)
		require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
		require.Equal(t, []int64{checkpointID}, activeCheckpointIDs(t, f))
		require.Equal(t, int64(10000), accountCommodityBalance(t, f, f.cashAccountID, f.eurCommodityID), "refusal writes nothing")

		input.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{checkpointID}}
		result, err := f.investmentService.ReplaceDividend(ctx, input)
		require.NoError(t, err)
		require.Equal(t, "2026-05-01", result.Replacement.TransactionDate)
		require.Empty(t, activeCheckpointIDs(t, f))
		requireInvestmentSelfCheckPasses(t, f)
	})

	t.Run("into the reconciled period", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		_, err := f.investmentService.Dividend(ctx, cashDividendInput(f, 0, 10000, 0))
		require.NoError(t, err)
		checkpointID := reconcileCash(t, f, "2026-04-01", 100)
		later := cashDividendInput(f, 0, 5000, 0)
		later.TransactionDate = "2026-05-01"
		original, err := f.investmentService.Dividend(ctx, later)
		require.NoError(t, err)
		replacement := later
		replacement.TransactionDate = "2026-03-15"
		input := ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID, TransactionID: original.ID,
			Reason: "statement shows it in March", Replacement: replacement}
		impact, err := f.investmentService.ReplaceDividendReconciliationImpact(ctx, input)
		require.NoError(t, err)
		require.Len(t, impact.AffectedCheckpoints, 1)
		require.Equal(t, checkpointID, impact.AffectedCheckpoints[0].CheckpointID)
		_, err = f.investmentService.ReplaceDividend(ctx, input)
		require.ErrorIs(t, err, ErrReconciliationOverrideRequired)

		input.ReconciliationOverride = CheckpointOverride{Allowed: true, Reviewed: []int64{checkpointID}}
		_, err = f.investmentService.ReplaceDividend(ctx, input)
		require.NoError(t, err)
		require.Empty(t, activeCheckpointIDs(t, f))
		require.Equal(t, int64(15000), accountCommodityBalance(t, f, f.cashAccountID, f.eurCommodityID))
		requireInvestmentSelfCheckPasses(t, f)
	})

	t.Run("within the reconciled period", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		original, err := f.investmentService.Dividend(ctx, cashDividendInput(f, 0, 10000, 0))
		require.NoError(t, err)
		checkpointID := reconcileCash(t, f, "2026-04-01", 100)
		replacement := cashDividendInput(f, 0, 10000, 0)
		replacement.TransactionDate = "2026-03-20"
		input := ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID, TransactionID: original.ID,
			Reason: "typo in the day", Replacement: replacement}
		impact, err := f.investmentService.ReplaceDividendReconciliationImpact(ctx, input)
		require.NoError(t, err)
		require.Empty(t, impact.AffectedCheckpoints, "the reconciled balance at 2026-04-01 is unchanged")
		_, err = f.investmentService.ReplaceDividend(ctx, input)
		require.NoError(t, err)
		require.Equal(t, []int64{checkpointID}, activeCheckpointIDs(t, f))
		requireInvestmentSelfCheckPasses(t, f)
	})
}
