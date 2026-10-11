package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
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

// requireEffectiveMovementsConsistent checks two contract properties that do
// not depend on replay:
//   - a revision never moves a date: every revised movement is dated like an
//     original event of the operation (or decision) it revises;
//   - the side and disposal filters select exactly the matching subset of the
//     unfiltered read.
func requireEffectiveMovementsConsistent(t *testing.T, f *investmentsTestFixture) {
	t.Helper()
	all := effectiveMovements(t, f, db.EffectiveMovementFilter{})
	short, disposals := 0, 0
	for _, movement := range all {
		if movement.PositionSide == db.PositionSideShort {
			short++
		}
		if movement.Kind == "disposal" {
			disposals++
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
	require.Len(t, effectiveMovements(t, f, db.EffectiveMovementFilter{PositionSide: db.PositionSideShort}), short)
	require.Len(t, effectiveMovements(t, f, db.EffectiveMovementFilter{DisposalsOnly: true}), disposals)
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
