package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/exact"
)

// A replacement operation links its own journal ('primary') and the inverse of
// the operation it corrects ('reversal'). Income belongs to the effective
// operation's own journal only: counting the linked inverse reported a
// corrected dividend as new minus old, and a currency correction as a negative
// figure in the old currency. The live period statement reads the same records.
func TestInvestmentIncomeCountsOnlyTheEffectivePrimaryJournal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("amount correction", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		original, err := f.investmentService.Dividend(ctx, cashDividendInput(f, 0, 10000, 0))
		require.NoError(t, err)
		_, err = f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
			TransactionID: original.ID, Reason: "statement shows 120.00", Replacement: cashDividendInput(f, 0, 12000, 0)})
		require.NoError(t, err)

		income, err := f.investmentService.repository.InvestmentIncome(ctx, BookID, "2026-01-01", "2026-12-31")
		require.NoError(t, err)
		require.Len(t, income, 1)
		require.Equal(t, f.eurCommodityID, income[0].CommodityID)
		requireScaled(t, 12000, 2, exact.ScaledIntFromCoefficient(income[0].Value, income[0].Scale), "the corrected gross, not 120.00 − 100.00")
		row := statementRowFor(t, gainsStatement(t, f, "2026-03-01", "2026-03-31"), f.eurCommodityID)
		require.Equal(t, 1, row.IncomeCount)
		require.Equal(t, StatementAmount{Value: 12000, Scale: 2}, row.Income)
	})

	t.Run("currency and date correction", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		usd := seedTestCurrencyCommodity(t, f.database, "USD")
		original, err := f.investmentService.Dividend(ctx, cashDividendInput(f, 0, 10000, 0))
		require.NoError(t, err)
		replacement := cashDividendInput(f, 0, 11000, 0)
		replacement.CashCommodityID, replacement.TransactionDate = usd, "2026-04-10"
		_, err = f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
			TransactionID: original.ID, Reason: "paid in USD in April", Replacement: replacement})
		require.NoError(t, err)

		income, err := f.investmentService.repository.InvestmentIncome(ctx, BookID, "2026-01-01", "2026-12-31")
		require.NoError(t, err)
		require.Len(t, income, 1, "no negative EUR row from the linked inverse")
		require.Equal(t, usd, income[0].CommodityID)
		require.Equal(t, "2026-04-10", income[0].EventDate)
		requireScaled(t, 11000, 2, exact.ScaledIntFromCoefficient(income[0].Value, income[0].Scale), "the corrected USD gross")
	})

	t.Run("reinvestment correction", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		original := reinvestOn(t, f, "2026-01-01", 10, 2000)
		_, err := acknowledgedReplaceReinvestedDividend(ctx, f.investmentService, replaceReinvestment(f, original.Transaction.ID,
			"restated", reinvestmentInput(f, "2026-01-01", 10, 4000)))
		require.NoError(t, err)
		income, err := f.investmentService.repository.InvestmentIncome(ctx, BookID, "2026-01-01", "2026-12-31")
		require.NoError(t, err)
		require.Len(t, income, 1)
		requireScaled(t, 4000, 2, exact.ScaledIntFromCoefficient(income[0].Value, income[0].Scale), "the corrected reinvested income")
	})

	t.Run("reversal", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		original, err := f.investmentService.Dividend(ctx, cashDividendInput(f, 0, 10000, 0))
		require.NoError(t, err)
		_, err = f.investmentService.ReverseDividend(ctx, ReverseInvestmentDividendInput{OwnerUserID: f.ownerUserID,
			TransactionID: original.ID, Reason: "not received"})
		require.NoError(t, err)
		income, err := f.investmentService.repository.InvestmentIncome(ctx, BookID, "2026-01-01", "2026-12-31")
		require.NoError(t, err)
		require.Empty(t, income)
	})
}

// The FX-converted statement reads the same income records, so a corrected
// dividend converts only its own gross at its own date — never the linked
// inverse of the original (#395 review regression).
func TestGainsFXStatementConvertsOnlyTheCorrectedDividendIncome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("amount and date correction", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		usd := gainsFXCurrency(t, f)
		original := dividendInputOn(f, "2026-03-10", 10000)
		payment, err := f.investmentService.Dividend(ctx, original)
		require.NoError(t, err)
		_, err = f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
			TransactionID: payment.ID, Reason: "paid 120.00 on the 15th", Replacement: dividendInputOn(f, "2026-03-15", 12000)})
		require.NoError(t, err)
		for date, rate := range map[string]int64{"2026-03-10": 90, "2026-03-15": 95, "2026-03-31": 120} {
			gainsFXRate(t, f, usd, date, rate)
		}
		statement := gainsStatement(t, f, "2026-03-01", "2026-03-31")
		for _, tc := range []struct {
			method string
			income int64
		}{{GainsFXHistoricBasis, 11400}, {GainsFXValuationDateAll, 14400}} {
			result, err := f.investmentService.ConvertGainsStatement(ctx, statement, GainsFXInput{CommodityID: usd, Method: tc.method})
			require.NoError(t, err)
			// 120.00 × 0.95 on the corrected date, or × 1.20 at the closing date.
			assertFXAmount(t, tc.income, 2, result.Income)
			require.Len(t, result.Income.Components, 1, tc.method)
		}
	})

	t.Run("currency correction", func(t *testing.T) {
		t.Parallel()
		f := newInvestmentsTestFixture(t)
		usd := gainsFXCurrency(t, f)
		payment, err := f.investmentService.Dividend(ctx, dividendInputOn(f, "2026-03-10", 10000))
		require.NoError(t, err)
		replacement := dividendInputOn(f, "2026-03-15", 11000)
		replacement.CashCommodityID = usd
		_, err = f.investmentService.ReplaceDividend(ctx, ReplaceInvestmentDividendInput{OwnerUserID: f.ownerUserID,
			TransactionID: payment.ID, Reason: "paid in USD", Replacement: replacement})
		require.NoError(t, err)
		statement := gainsStatement(t, f, "2026-03-01", "2026-03-31")
		result, err := f.investmentService.ConvertGainsStatement(ctx, statement, GainsFXInput{CommodityID: usd, Method: GainsFXHistoricBasis})
		require.NoError(t, err)
		// Native USD income needs no rate, and no EUR inverse component exists
		// to make the total unavailable or negative.
		assertFXAmount(t, 11000, 2, result.Income)
		require.Len(t, result.Income.Components, 1)
		require.Equal(t, usd, result.Income.Components[0].CommodityID)
	})
}

func dividendInputOn(f *investmentsTestFixture, date string, gross int64) DividendInput {
	input := cashDividendInput(f, 0, gross, 0)
	input.TransactionDate = date
	return input
}
