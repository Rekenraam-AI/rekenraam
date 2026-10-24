package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #401 characterizes the correction preparation the split, share exchange,
// spin-off and cash-and-stock merger families share before the typed writer
// runs: admission messages, the typed lookup's not-found refusal, the
// already-corrected and imported-lineage fences. Every refusal reaches both
// preview and commit, for reversal and replacement, and writes nothing.

type correctionPreparationFamily struct {
	name             string
	noun             string
	notFound         error
	alreadyCorrected error
	imported         error
	// record posts the family's operation and returns its transaction id.
	record func(t *testing.T, f *investmentsTestFixture) int64
	// reverse and replace run the family's commands; replace carries a real
	// term change so only the preparation fences can refuse it.
	reverse func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason, acknowledgement string, preview bool) (string, error)
	replace func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason string, preview bool) error
}

func correctionPreparationFamilies() []correctionPreparationFamily {
	return []correctionPreparationFamily{
		{
			name: "split", noun: "split",
			notFound: ErrInvestmentSplitNotFound, alreadyCorrected: ErrInvestmentSplitAlreadyCorrected, imported: ErrInvestmentImportedSplit,
			record: func(t *testing.T, f *investmentsTestFixture) int64 {
				buyOn(t, f, "2026-02-01", 10, 10000)
				split, err := f.investmentService.Split(context.Background(), splitInput(f, "2026-03-01", 2, 1))
				require.NoError(t, err)
				return split.Transaction.ID
			},
			reverse: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason, acknowledgement string, preview bool) (string, error) {
				input := ReverseInvestmentSplitInput{OwnerUserID: owner, TransactionID: transactionID, Reason: reason, GainImpactAcknowledgement: acknowledgement}
				if preview {
					impact, err := f.investmentService.ReverseSplitReconciliationImpact(ctx, input)
					return gainAcknowledgement(impact), err
				}
				_, err := f.investmentService.ReverseSplit(ctx, input)
				return "", err
			},
			replace: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason string, preview bool) error {
				input := replaceSplitInput(f, transactionID, "2026-03-01", 3, 1)
				input.OwnerUserID, input.Reason = owner, reason
				if preview {
					_, err := f.investmentService.PreviewSplitReplacement(ctx, input)
					return err
				}
				_, err := f.investmentService.ReplaceSplit(ctx, input)
				return err
			},
		},
		{
			name: "share exchange", noun: "exchange",
			notFound: ErrShareExchangeNotFound, alreadyCorrected: ErrShareExchangeAlreadyCorrected, imported: ErrInvestmentImportedTransfer,
			record: func(t *testing.T, f *investmentsTestFixture) int64 {
				newID := seedTestSecurityCommodity(t, f, "NEWCO")
				buyOn(t, f, "2026-02-01", 10, 10000)
				exchanged, err := f.investmentService.ShareExchange(context.Background(), shareExchangeInput(f, newID, "2026-06-01", 2, 1))
				require.NoError(t, err)
				return exchanged.Transaction.ID
			},
			reverse: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason, acknowledgement string, preview bool) (string, error) {
				input := ReverseShareExchangeInput{OwnerUserID: owner, TransactionID: transactionID, Reason: reason, GainImpactAcknowledgement: acknowledgement}
				if preview {
					impact, err := f.investmentService.ReverseShareExchangeReconciliationImpact(ctx, input)
					return gainAcknowledgement(impact), err
				}
				_, err := f.investmentService.ReverseShareExchange(ctx, input)
				return "", err
			},
			replace: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason string, preview bool) error {
				input := replaceShareExchangeInput(f, transactionID, commodityIDBySymbol(f, "NEWCO"), "2026-06-01", 3, 1)
				input.OwnerUserID, input.Reason = owner, reason
				if preview {
					_, err := f.investmentService.PreviewShareExchangeReplacement(ctx, input)
					return err
				}
				_, err := f.investmentService.ReplaceShareExchange(ctx, input)
				return err
			},
		},
		{
			name: "spin-off", noun: "spin-off",
			notFound: ErrSpinOffNotFound, alreadyCorrected: ErrSpinOffAlreadyCorrected, imported: ErrInvestmentImportedTransfer,
			record: func(t *testing.T, f *investmentsTestFixture) int64 {
				newID := seedTestSecurityCommodity(t, f, "NEWCO")
				buyOn(t, f, "2026-02-01", 10, 10000)
				spun, err := f.investmentService.SpinOff(context.Background(), spinOffInput(f, newID, "2026-06-01", 1, 2, "2", 1))
				require.NoError(t, err)
				return spun.Transaction.ID
			},
			reverse: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason, acknowledgement string, preview bool) (string, error) {
				input := ReverseSpinOffInput{OwnerUserID: owner, TransactionID: transactionID, Reason: reason, GainImpactAcknowledgement: acknowledgement}
				if preview {
					impact, err := f.investmentService.ReverseSpinOffReconciliationImpact(ctx, input)
					return gainAcknowledgement(impact), err
				}
				_, err := f.investmentService.ReverseSpinOff(ctx, input)
				return "", err
			},
			replace: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason string, preview bool) error {
				input := replaceSpinOffInput(f, transactionID, commodityIDBySymbol(f, "NEWCO"), "2026-06-01", 1, 1, "2", 1)
				input.OwnerUserID, input.Reason = owner, reason
				if preview {
					_, err := f.investmentService.PreviewSpinOffReplacement(ctx, input)
					return err
				}
				_, err := f.investmentService.ReplaceSpinOff(ctx, input)
				return err
			},
		},
		{
			name: "cash-and-stock merger", noun: "merger",
			notFound: ErrCashStockMergerNotFound, alreadyCorrected: ErrCashStockMergerAlreadyCorrected, imported: ErrInvestmentImportedTransfer,
			record: func(t *testing.T, f *investmentsTestFixture) int64 {
				newID := seedTestSecurityCommodity(t, f, "NEWCO")
				buyOn(t, f, "2026-02-01", 100, 100000)
				return defaultCashStockMerger(t, f, newID).Transaction.ID
			},
			reverse: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason, acknowledgement string, preview bool) (string, error) {
				input := ReverseCashStockMergerInput{OwnerUserID: owner, TransactionID: transactionID, Reason: reason, GainImpactAcknowledgement: acknowledgement}
				if preview {
					impact, err := f.investmentService.ReverseCashStockMergerReconciliationImpact(ctx, input)
					return gainAcknowledgement(impact), err
				}
				_, err := f.investmentService.ReverseCashStockMerger(ctx, input)
				return "", err
			},
			replace: func(ctx context.Context, f *investmentsTestFixture, owner, transactionID int64, reason string, preview bool) error {
				input := replaceCashStockMergerInput(f, transactionID, commodityIDBySymbol(f, "NEWCO"), "2026-06-01", "2026-06-10", 1, 2, 30000, "25", 2)
				input.OwnerUserID, input.Reason = owner, reason
				if preview {
					_, err := f.investmentService.PreviewCashStockMergerReplacement(ctx, input)
					return err
				}
				_, err := f.investmentService.ReplaceCashStockMerger(ctx, input)
				return err
			},
		},
	}
}

func gainAcknowledgement(impact ReconciliationImpact) string {
	if impact.GainImpact == nil {
		return ""
	}
	return impact.GainImpact.Acknowledgement
}

func commodityIDBySymbol(f *investmentsTestFixture, symbol string) int64 {
	var id int64
	if err := f.database.QueryRow(`SELECT id FROM commodities WHERE code = ?`, symbol).Scan(&id); err != nil {
		panic(err)
	}
	return id
}

// eachPreparationCommand runs reversal and replacement, each previewed and
// committed, and requires the same refusal and no durable write from all four.
func eachPreparationCommand(t *testing.T, f *investmentsTestFixture, family correctionPreparationFamily,
	owner, transactionID int64, reason string, check func(t *testing.T, err error)) {
	t.Helper()
	ctx := context.Background()
	before := buyReplacementPreviewSnapshot(t, f.database)
	for _, preview := range []bool{true, false} {
		_, err := family.reverse(ctx, f, owner, transactionID, reason, "", preview)
		check(t, err)
		check(t, family.replace(ctx, f, owner, transactionID, reason, preview))
	}
	assert.Equal(t, before, buyReplacementPreviewSnapshot(t, f.database))
}

func requireValidationMessage(message string) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		var validation ValidationError
		require.ErrorAs(t, err, &validation)
		assert.Equal(t, message, validation.Message)
	}
}

func requireErrorIs(target error) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()
		require.ErrorIs(t, err, target)
	}
}

func TestCorporateActionCorrectionPreparationAdmission(t *testing.T) {
	t.Parallel()
	for _, family := range correctionPreparationFamilies() {
		t.Run(family.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			transactionID := family.record(t, f)
			idRequired := requireValidationMessage("owner and " + family.noun + " id are required")
			eachPreparationCommand(t, f, family, 0, transactionID, "wrong terms", idRequired)
			eachPreparationCommand(t, f, family, f.ownerUserID, 0, "wrong terms", idRequired)
			eachPreparationCommand(t, f, family, f.ownerUserID, transactionID, " \t",
				requireValidationMessage(family.noun+" correction reason is required"))
			eachPreparationCommand(t, f, family, f.ownerUserID, transactionID+1_000_000, "wrong terms", requireErrorIs(family.notFound))
		})
	}
}

func TestCorporateActionCorrectionPreparationRefusesAnAlreadyCorrectedOperation(t *testing.T) {
	t.Parallel()
	for _, family := range correctionPreparationFamilies() {
		t.Run(family.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			transactionID := family.record(t, f)
			acknowledgement, err := family.reverse(ctx, f, f.ownerUserID, transactionID, "wrong terms", "", true)
			require.NoError(t, err)
			_, err = family.reverse(ctx, f, f.ownerUserID, transactionID, "wrong terms", acknowledgement, false)
			require.NoError(t, err)
			eachPreparationCommand(t, f, family, f.ownerUserID, transactionID, "wrong terms", requireErrorIs(family.alreadyCorrected))
		})
	}
}

// An operation created by an import may be corrected only while its
// committed import source is still linked; the lineage marker here is the
// creating audit event's origin, as for an import without a committed row.
func TestCorporateActionCorrectionPreparationRefusesUnlinkedImportedLineage(t *testing.T) {
	t.Parallel()
	for _, family := range correctionPreparationFamilies() {
		t.Run(family.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			transactionID := family.record(t, f)
			result, err := f.database.Exec(`UPDATE audit_events SET origin_type = 'import'
 WHERE id = (SELECT created_audit_event_id FROM investment_operations
 WHERE id = (SELECT operation_id FROM investment_operation_journal_links link
 JOIN transaction_versions version ON version.id = link.transaction_version_id
 WHERE version.transaction_id = ? LIMIT 1))`, transactionID)
			require.NoError(t, err)
			changed, err := result.RowsAffected()
			require.NoError(t, err)
			require.EqualValues(t, 1, changed)
			eachPreparationCommand(t, f, family, f.ownerUserID, transactionID, "wrong terms", requireErrorIs(family.imported))
		})
	}
}

// The control: with every fence clear, the same commands are admitted, so
// the refusals above come from the preparation rather than the inputs.
func TestCorporateActionCorrectionPreparationAdmitsTheControlCommands(t *testing.T) {
	t.Parallel()
	for _, family := range correctionPreparationFamilies() {
		t.Run(family.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			ctx := context.Background()
			transactionID := family.record(t, f)
			_, err := family.reverse(ctx, f, f.ownerUserID, transactionID, "wrong terms", "", true)
			require.NoError(t, err)
			err = family.replace(ctx, f, f.ownerUserID, transactionID, "wrong terms", true)
			if errors.Is(err, family.notFound) || errors.Is(err, family.alreadyCorrected) || errors.Is(err, family.imported) {
				t.Fatalf("control replacement refused by a preparation fence: %v", err)
			}
			require.NoError(t, err)
		})
	}
}

// A target whose recorded version or date no longer matches its posted
// original refuses with the family's own changed error; the writer repeats
// this check in its transaction. A journal-free target is admitted only for
// the family that can record one.
func TestCorporateActionCorrectionPreparationRefusesAChangedOriginal(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	buyOn(t, f, "2026-02-01", 10, 10000)
	split, err := f.investmentService.Split(ctx, splitInput(f, "2026-03-01", 2, 1))
	require.NoError(t, err)
	operation, err := f.investmentService.repository.SplitOperationByTransactionID(ctx, BookID, split.Transaction.ID)
	require.NoError(t, err)
	target := correctionTarget{OperationID: operation.OperationID, TransactionID: operation.TransactionID,
		CurrentVersionID: operation.CurrentVersionID, EventDate: operation.EventDate}
	for _, family := range []corporateActionCorrection{splitCorrection, shareExchangeCorrection, spinOffCorrection, cashStockMergerCorrection} {
		original, err := f.investmentService.verifyCorrectionTarget(ctx, family, target)
		require.NoError(t, err, family.noun)
		assert.Equal(t, split.Transaction.ID, original.ID)

		stale := target
		stale.CurrentVersionID++
		_, err = f.investmentService.verifyCorrectionTarget(ctx, family, stale)
		require.ErrorIs(t, err, family.changed, family.noun)
		moved := target
		moved.EventDate = "2026-03-02"
		_, err = f.investmentService.verifyCorrectionTarget(ctx, family, moved)
		require.ErrorIs(t, err, family.changed, family.noun)

		journalFree := target
		journalFree.TransactionID = 0
		_, err = f.investmentService.verifyCorrectionTarget(ctx, family, journalFree)
		if family.journalFree {
			require.NoError(t, err, family.noun)
		} else {
			require.Error(t, err, family.noun)
		}
	}
	assert.ErrorIs(t, cashStockMergerCorrection.changed, ErrShareExchangeChanged, "the merger keeps its exchange's refusal")
}
