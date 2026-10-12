package app

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/exact"
)

// #189: committed effective lot movements have one owner. These helpers run
// after every shared self-check pass, so each revision scenario in the suite
// also proves the owner's contract.

func effectiveMovements(t *testing.T, f *investmentsTestFixture, filter db.EffectiveMovementFilter) []db.EffectiveLotMovement {
	t.Helper()
	movements, err := db.EffectiveLotMovements(context.Background(), f.database, BookID, filter)
	require.NoError(t, err)
	return movements
}

// requireEffectiveMovementsConsistent checks contract properties against
// immutable evidence, independently of replay:
//   - order anchors, journal/link provenance and exact revision identity;
//   - a revision never moves a date: every revised movement is dated like an
//     original event of the operation (or decision) it revises;
//   - the side and disposal filters select exactly the matching subset of the
//     unfiltered read.
func requireEffectiveMovementsConsistent(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	all := effectiveMovements(t, f, db.EffectiveMovementFilter{})
	var short, disposals []db.EffectiveLotMovement
	for _, movement := range all {
		if movement.OperationID > 0 {
			var sequence int
			var transactionID sql.NullInt64
			require.NoError(t, f.database.QueryRow(`SELECT x.effect_seq, e.transaction_id
				FROM investment_operation_lot_effects x JOIN investment_lot_events e ON e.id = x.lot_event_id
				WHERE x.operation_id = ? AND x.lot_event_id = ?`, movement.OperationID, movement.OriginEventID).
				Scan(&sequence, &transactionID))
			require.Positive(t, movement.OrderOperationID)
			require.Equal(t, sequence, movement.EffectSeq)
			require.Equal(t, transactionID, movement.TransactionID)
		}
		if movement.LinkSeq > 0 {
			var links int
			require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_transfer_lot_links
				WHERE operation_id = ? AND link_seq = ?`, movement.TransferOperationID, movement.LinkSeq).Scan(&links))
			require.Equal(t, 1, links)
		}
		if movement.Source == db.MovementSourceEvent {
			require.Zero(t, movement.RevisionID)
			require.Zero(t, movement.RevisionSeq)
			require.Zero(t, movement.RevisionEffectSeq)
		} else {
			// Verify exact revision provenance against durable evidence, not a
			// second read from the owner or its latest-revision views.
			table := map[string]string{
				db.MovementSourceDisposalRevision:       "investment_disposal_revisions",
				db.MovementSourceSplitRevision:          "investment_split_revisions",
				db.MovementSourceTransferLinkRevision:   "investment_transfer_link_revisions",
				db.MovementSourcePooledTransferRevision: "investment_transfer_link_revisions",
				db.MovementSourceCapitalReturnRevision:  "investment_capital_return_revisions",
			}[movement.Source]
			require.NotEmpty(t, table)
			column, subject := "operation_id", movement.OperationID
			if movement.Source == db.MovementSourceDisposalRevision {
				column, subject = "decision_id", movement.DecisionID
			}
			var sequence int
			var recordedSubject int64
			require.NoError(t, f.database.QueryRow(fmt.Sprintf(`SELECT revision_seq, %s FROM %s WHERE id = ?`, column, table),
				movement.RevisionID).Scan(&sequence, &recordedSubject))
			require.Equal(t, sequence, movement.RevisionSeq)
			require.Equal(t, subject, recordedSubject)
		}
		if movement.PositionSide == db.PositionSideShort {
			short = append(short, movement)
		}
		if movement.Kind == "disposal" {
			disposals = append(disposals, movement)
		}
		if movement.Source == db.MovementSourceEvent {
			continue
		}
		var originals int
		if movement.Source == db.MovementSourceDisposalRevision {
			require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_disposal_allocations a
				JOIN investment_lot_events e ON e.id = a.lot_event_id
				WHERE a.decision_id = ? AND e.event_date = ?`, movement.DecisionID, movement.EventDate).Scan(&originals))
		} else {
			require.NoError(t, f.database.QueryRow(`SELECT COUNT(*) FROM investment_operation_lot_effects x
				JOIN investment_lot_events e ON e.id = x.lot_event_id
				WHERE x.operation_id = ? AND e.event_date = ?`, movement.OperationID, movement.EventDate).Scan(&originals))
		}
		require.Positivef(t, originals, "%s movement of lot %d on %s has no original event on that date",
			movement.Source, movement.LotID, movement.EventDate)
	}
	require.Equal(t, short, effectiveMovements(t, f, db.EffectiveMovementFilter{PositionSide: db.PositionSideShort}))
	require.Equal(t, disposals, effectiveMovements(t, f, db.EffectiveMovementFilter{DisposalsOnly: true}))
}

func TestEffectiveMovementsOriginalRevisedAndCorrectedSameDayOrder(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	const date = "2026-06-01"
	destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	buy := buyOn(t, f, date, 3, 3000)
	transfer := chainTransfer(t, f, f.holdingAccountID, destination, *buy.LotID, date)
	saleInput := sellInput(f, date, 1)
	saleInput.HoldingAccountID = destination
	sale, err := f.investmentService.Sell(ctx, saleInput)
	require.NoError(t, err)
	laterBuy := buyOn(t, f, date, 1, 1000)
	buyID, transferID := transferOperationID(t, f, buy.Transaction.ID), transferOperationID(t, f, transfer.Transaction.ID)
	saleID, laterBuyID := transferOperationID(t, f, sale.Transaction.ID), transferOperationID(t, f, laterBuy.Transaction.ID)
	check := func(replacementID int64, revisionSeq int) {
		t.Helper()
		all := effectiveMovements(t, f, db.EffectiveMovementFilter{})
		require.Len(t, all, 5)
		var kinds []string
		var roots, operations []int64
		var sequences []int
		for _, m := range all {
			kinds = append(kinds, m.Kind)
			roots = append(roots, m.OrderOperationID)
			operations = append(operations, m.OperationID)
			sequences = append(sequences, m.EffectSeq)
			assert.Equal(t, date, m.EventDate)
		}
		assert.Equal(t, []string{"acquisition", "transfer_out", "transfer_in", "disposal", "acquisition"}, kinds)
		assert.Equal(t, []int64{buyID, transferID, transferID, saleID, laterBuyID}, roots)
		assert.Equal(t, []int64{replacementID, transferID, transferID, saleID, laterBuyID}, operations)
		assert.Equal(t, []int{1, 1, 2, 1, 1}, sequences)
		for _, m := range all[1:3] {
			assert.Equal(t, transferID, m.TransferOperationID)
			assert.Equal(t, 1, m.LinkSeq)
			assert.Equal(t, transfer.Transaction.ID, m.TransactionID.Int64)
			assert.Equal(t, revisionSeq, m.RevisionSeq)
		}
		assert.Positive(t, all[3].DecisionID)
		if revisionSeq > 0 {
			var linkRevision, disposalRevision int64
			require.NoError(t, f.database.QueryRow(`SELECT id FROM investment_transfer_link_revisions
				WHERE operation_id = ? AND link_seq = 1 AND revision_seq = ?`, transferID, revisionSeq).Scan(&linkRevision))
			require.NoError(t, f.database.QueryRow(`SELECT id FROM investment_disposal_revisions
				WHERE decision_id = ? AND revision_seq = ?`, all[3].DecisionID, revisionSeq).Scan(&disposalRevision))
			assert.Equal(t, linkRevision, all[1].RevisionID)
			assert.Equal(t, linkRevision, all[2].RevisionID)
			assert.Equal(t, disposalRevision, all[3].RevisionID)
			assert.Equal(t, 1, all[3].RevisionEffectSeq)
		}
		require.Equal(t, all, effectiveMovements(t, f, db.EffectiveMovementFilter{}), "repeat reads are deterministic")
		requireInvestmentSelfCheckPasses(t, f)
	}
	check(buyID, 0)
	first, err := acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, buy, date, 3, 6000))
	require.NoError(t, err)
	firstID := transferOperationID(t, f, first.Replacement.Transaction.ID)
	require.Greater(t, firstID, laterBuyID, "the replacement was recorded after every original subject")
	check(firstID, 2)
	second, err := acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, first.Replacement, date, 3, 9000))
	require.NoError(t, err)
	check(transferOperationID(t, f, second.Replacement.Transaction.ID), 3)
}

func TestEffectiveMovementsPooledRevisionExpandsDepletionsBeforeSameDayDestination(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	const date = "2026-06-01"
	destination := seedTestAccountWithClass(t, f.database, "active", true, "asset", "security_holding")
	setHoldingCostBasisMethod(t, f, "average_cost")
	first := buyOn(t, f, date, 4, 4000)
	second := buyOn(t, f, date, 2, 2000)
	transfer, err := f.investmentService.InternalTransfer(ctx, pooledTransferInput(f, destination, date, exact.New(3), 0))
	require.NoError(t, err)
	transferID := transferOperationID(t, f, transfer.Transaction.ID)
	originals := effectiveMovements(t, f, db.EffectiveMovementFilter{})
	require.Len(t, originals, 4)
	assert.Equal(t, "transfer_out", originals[2].Kind)
	assert.Equal(t, "transfer_in", originals[3].Kind)
	for _, m := range originals[2:] {
		assert.Equal(t, transferID, m.TransferOperationID)
		assert.Equal(t, 1, m.LinkSeq)
	}
	replacement, err := acknowledgedReplaceBuy(ctx, f.investmentService, replaceBuyPrice(f, first, date, 1, 1000))
	require.NoError(t, err)
	all := effectiveMovements(t, f, db.EffectiveMovementFilter{})
	require.Len(t, all, 5)
	for _, m := range all[2:] {
		assert.Equal(t, transferID, m.OrderOperationID)
		assert.Equal(t, transferID, m.TransferOperationID)
		assert.Equal(t, 1, m.LinkSeq)
		assert.Equal(t, 2, m.RevisionSeq)
		assert.Equal(t, db.MovementSourcePooledTransferRevision, m.Source)
	}
	// Equal acquisition dates break ties by lot ID: the unchanged lot is
	// depleted before the newly recorded replacement. Preserve the revision's
	// recorded child sequence rather than guessing from correction-root order.
	assert.Equal(t, *second.LotID, all[2].LotID)
	assert.Equal(t, *replacement.Replacement.LotID, all[3].LotID)
	assert.Equal(t, exact.New(-2), all[2].QuantityValue)
	assert.Equal(t, exact.New(-1), all[3].QuantityValue)
	assert.Equal(t, transfer.DestinationLotIDs[0], all[4].LotID)
	assert.Equal(t, []string{"transfer_out", "transfer_out", "transfer_in"}, []string{all[2].Kind, all[3].Kind, all[4].Kind})
	assert.Equal(t, []int{1, 1, 2}, []int{all[2].EffectSeq, all[3].EffectSeq, all[4].EffectSeq})
	assert.Equal(t, []int{1, 2, 0}, []int{all[2].RevisionEffectSeq, all[3].RevisionEffectSeq, all[4].RevisionEffectSeq})
	assert.Equal(t, all[2].RevisionID, all[4].RevisionID)
	requireInvestmentSelfCheckPasses(t, f)
}

// requireRevisedMovement asserts the owner yields a revised movement of the
// given source and kind whose quantity sign matches.
func requireRevisedMovement(t *testing.T, f *investmentsTestFixture, source, kind string, quantitySign int) db.EffectiveLotMovement {
	t.Helper()
	for _, movement := range effectiveMovements(t, f, db.EffectiveMovementFilter{}) {
		if movement.Source == source && movement.Kind == kind && movement.QuantityValue.Sign() == quantitySign {
			return movement
		}
	}
	t.Fatalf("no %s %s movement with quantity sign %d", source, kind, quantitySign)
	return db.EffectiveLotMovement{}
}
