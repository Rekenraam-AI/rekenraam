package app

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

func TestJournalFreeSplitExportsAndSurvivesRestore(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	result, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	id := strconv.FormatInt(result.OperationID, 10)
	files := exportedBundle(t, f)
	require.Len(t, files["investment-operations.csv"], 1)
	require.Equal(t, id, files["investment-operations.csv"][0]["operation_id"])
	require.Empty(t, files["investment-operations.csv"][0]["transaction_id"])
	require.Len(t, files["investment-split-facts.csv"], 1)
	fact := files["investment-split-facts.csv"][0]
	require.Equal(t, id, fact["operation_id"])
	require.Equal(t, "2026-03-01", fact["effective_on"])
	require.Equal(t, "2", fact["ratio_numerator"])
	require.Equal(t, "1", fact["ratio_denominator"])
	require.JSONEq(t, `{"notice":"corporate action"}`, fact["source_evidence_json"])
	require.Empty(t, files["investment-operation-journal-links.csv"])
	require.Empty(t, files["investment-lot-effects.csv"])

	backupPath := filepath.Join(t.TempDir(), "backup.sqlite")
	_, err = db.OnlineBackupSQLiteDatabase(ctx, f.database, backupPath, db.OnlineBackupOptions{})
	require.NoError(t, err)
	restoredURL := "file:" + filepath.Join(t.TempDir(), "restored.sqlite")
	_, err = db.RestoreSQLiteDatabase(ctx, backupPath, restoredURL)
	require.NoError(t, err)
	restored, err := db.Open(ctx, restoredURL)
	require.NoError(t, err)
	defer restored.Close()
	f.database = restored
	accountRepository := db.NewAccountRepository(restored)
	f.accountService = NewAccountService(accountRepository, db.NewInstitutionRepository(restored), nil)
	f.transactionService = NewTransactionService(db.NewTransactionRepository(restored), db.NewPayeeRepository(restored), accountRepository, db.NewCommodityRepository(restored))
	f.investmentService = NewInvestmentService(db.NewInvestmentRepository(restored), f.accountService, f.transactionService, nil)
	requireSelfCheckPasses(t, f)
	// Restored source terms remain active even before the first lot exists.
	buyOn(t, f, "2026-02-01", 10, 10000)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(20, 0)))
	requireSelfCheckPasses(t, f)
	files = exportedBundle(t, f)
	require.Len(t, files["investment-split-revisions.csv"], 1)
	require.NotEmpty(t, files["investment-split-revisions.csv"][0]["adjustment_transaction_version_id"])
	for _, link := range files["investment-operation-journal-links.csv"] {
		if link["operation_id"] == id {
			require.Equal(t, "split_adjustment", link["role"])
		}
	}
}

func TestJournalFreeSplitEntitlementRefusalNamesSplitAndRollsBack(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 1, 3))
	require.NoError(t, err)
	before := buyReplacementPreviewSnapshot(t, f.database)
	input := backdatedBuy(f, "2026-02-01", 10, 10000)
	for _, preview := range []bool{true, false} {
		if preview {
			_, err = f.investmentService.TradeReconciliationImpact(ctx, InvestmentImpactBuy, input)
		} else {
			_, err = f.investmentService.Buy(ctx, input)
		}
		var dependency InvestmentBuyDependencyError
		require.ErrorAs(t, err, &dependency)
		require.Equal(t, split.OperationID, dependency.OperationID)
		require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	}
	requireSelfCheckPasses(t, f)
}

func TestJournalFreeSplitAdjustmentUsesGainAndCheckpointGuards(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	_, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	buyOn(t, f, "2026-04-01", 10, 10000)
	sale, err := f.investmentService.Sell(ctx, sellInput(f, "2026-05-01", 5))
	require.NoError(t, err)
	reconcileHoldingThrough(t, f, "2026-06-01", 5)
	input := backdatedBuy(f, "2026-02-01", 10, 10000)
	before := buyReplacementPreviewSnapshot(t, f.database)
	impact, err := f.investmentService.TradeReconciliationImpact(ctx, InvestmentImpactBuy, input)
	require.NoError(t, err)
	require.Len(t, impact.AffectedCheckpoints, 1)
	require.Len(t, impact.GainImpact.Changes, 1)
	requireScaled(t, 2500, 2, impact.GainImpact.Changes[0].After.DisposedBasis, "the sale consumes split-adjusted units")
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	_, err = f.investmentService.Buy(ctx, input)
	require.ErrorIs(t, err, ErrReconciliationOverrideRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input.ReconciliationOverride = true
	_, err = f.investmentService.Buy(ctx, input)
	require.ErrorIs(t, err, ErrGainImpactAcknowledgementRequired)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	input.GainImpactAcknowledgement = impact.GainImpact.Acknowledgement
	_, err = f.investmentService.Buy(ctx, input)
	require.NoError(t, err)
	require.Zero(t, holdingQuantity(t, f).Cmp(scaled(25, 0)))
	require.Zero(t, effectiveDisposedBasis(t, f.database, sale.Transaction.ID).Cmp(scaled(2500, 2)))
	requireSelfCheckPasses(t, f)
}

func TestJournalFreeSplitStalePlanAndAccountAreRefused(t *testing.T) {
	t.Parallel()
	for _, changed := range []string{"entitlement", "account"} {
		t.Run(changed, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			header, split, _, err := f.investmentService.prepareSplitWrite(ctx, splitInput(f, "2026-03-01", 2, 1))
			require.NoError(t, err)
			require.True(t, header.AuditOnly)
			expected := error(ErrInvestmentSplitChanged)
			if changed == "entitlement" {
				buyOn(t, f, "2026-02-01", 10, 10000)
			} else {
				changeHoldingAccountKind(t, f, "other_asset")
				expected = db.ErrPostingAccountVersionStale
			}
			before := buyReplacementPreviewSnapshot(t, f.database)
			_, _, err = f.investmentService.repository.CreateSplit(ctx, header, split)
			require.ErrorIs(t, err, expected)
			require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
		})
	}
}

func TestJournalFreeSplitExemptionDoesNotHideMissingJournals(t *testing.T) {
	t.Parallel()
	for _, initialZero := range []bool{true, false} {
		t.Run(strconv.FormatBool(initialZero), func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			if !initialZero {
				buyOn(t, f, "2026-02-01", 10, 10000)
			}
			split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
			require.NoError(t, err)
			if initialZero {
				buyOn(t, f, "2026-02-01", 10, 10000)
			}
			requireSelfCheckPasses(t, f)
			_, err = f.database.Exec(`DROP TRIGGER investment_operation_journal_links_no_delete`)
			require.NoError(t, err)
			_, err = f.database.Exec(`DELETE FROM investment_operation_journal_links WHERE operation_id = ?`, split.OperationID)
			require.NoError(t, err)
			foundation := resultFor(t, mustRunInvestmentSelfCheck(t, f), CheckInvestmentFoundation)
			require.Equal(t, SelfCheckFailed, foundation.Status)
			require.Contains(t, foundation.Summary, "splits post a different journal delta")
		})
	}
}

// #196 owns operation-addressed correction. An adjustment must never be
// mistaken for a primary split journal by the existing transaction routes.
func TestJournalFreeSplitCannotCorrectAnAdjustmentAsItsPrimary(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	_, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	buyOn(t, f, "2026-02-01", 10, 10000)
	var adjustmentID int64
	require.NoError(t, f.database.QueryRow(`SELECT v.transaction_id FROM investment_operation_journal_links link
 JOIN transaction_versions v ON v.id = link.transaction_version_id WHERE link.role = 'split_adjustment'`).Scan(&adjustmentID))
	before := buyReplacementPreviewSnapshot(t, f.database)
	_, err = f.investmentService.ReverseSplit(ctx, ReverseInvestmentSplitInput{
		OwnerUserID: f.ownerUserID, TransactionID: adjustmentID, Reason: "wrong ratio"})
	require.ErrorIs(t, err, ErrInvestmentSplitNotFound)
	require.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
	requireSelfCheckPasses(t, f)
}
