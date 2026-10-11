package db

import (
	"context"
	"database/sql"
	"fmt"

	"rekenraam/backend/internal/exact"
)

// ShortLotQuantityEvent is one dated change to the borrowed units a holding
// owes: a short opening adds quantity, a cover removes it (#173). Folding
// them by date gives the negative balance a named short explains.
type ShortLotQuantityEvent struct {
	EventDate     string
	AccountID     int64
	CommodityID   int64
	QuantityValue exact.Coefficient
	QuantityScale int
}

// ShortLotQuantityEventsThrough reads every effective short-lot event dated
// on or before through (all of them when through is empty), in date order.
func (r *TransactionRepository) ShortLotQuantityEventsThrough(ctx context.Context, bookID int64, through string) ([]ShortLotQuantityEvent, error) {
	return shortLotQuantityEvents(ctx, r.database, bookID, through)
}

// SelfCheckShortLotQuantityEvents reads the same events inside the self-check
// snapshot.
func (r *SelfCheckRepository) SelfCheckShortLotQuantityEvents(ctx context.Context, transaction *sql.Tx, bookID int64) ([]ShortLotQuantityEvent, error) {
	return shortLotQuantityEvents(ctx, transaction, bookID, "")
}

// The units a short owes move with its committed effective movements, so a
// cover that replay revised still counts on its own date (#188, #189).
func shortLotQuantityEvents(ctx context.Context, source queryer, bookID int64, through string) ([]ShortLotQuantityEvent, error) {
	movements, err := EffectiveLotMovements(ctx, source, bookID, EffectiveMovementFilter{PositionSide: PositionSideShort})
	if err != nil {
		return nil, fmt.Errorf("read short lot quantity events: %w", err)
	}
	var events []ShortLotQuantityEvent
	for _, movement := range movements {
		if movement.PositionSide != PositionSideShort || (through != "" && movement.EventDate > through) {
			continue
		}
		events = append(events, ShortLotQuantityEvent{EventDate: movement.EventDate, AccountID: movement.AccountID,
			CommodityID: movement.CommodityID, QuantityValue: movement.QuantityValue, QuantityScale: movement.QuantityScale})
	}
	return events, nil
}
