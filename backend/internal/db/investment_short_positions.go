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

// A cover revised by replay has no effective lot events of its own: the view
// drops the original disposal events and the latest revision's allocations
// stand in for them, dated to the decision (#188). Leaving them out would
// forget the cover and overstate the units owed.
func shortLotQuantityEvents(ctx context.Context, source queryer, bookID int64, through string) ([]ShortLotQuantityEvent, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT event_date, account_id, commodity_id, quantity_value, quantity_scale, revised FROM (
			SELECT e.event_date, l.account_id, l.commodity_id, e.quantity_value, e.quantity_scale,
				0 AS revised, e.id AS row_order
			FROM effective_investment_lot_events e
			JOIN investment_lots l ON l.id = e.lot_id
			WHERE l.book_id = ? AND l.position_side = 'short' AND (? = '' OR e.event_date <= ?)
			UNION ALL
			SELECT decision.event_date, l.account_id, l.commodity_id, allocation.quantity_value, allocation.quantity_scale,
				1, allocation.id
			FROM latest_investment_disposal_revisions revision
			JOIN investment_disposal_decisions decision ON decision.id = revision.decision_id
			JOIN effective_investment_operations operation ON operation.id = decision.operation_id
			JOIN investment_disposal_revision_allocations allocation ON allocation.revision_id = revision.id
			JOIN investment_lots l ON l.id = allocation.lot_id
			WHERE revision.book_id = ? AND l.position_side = 'short' AND (? = '' OR decision.event_date <= ?)
		)
		ORDER BY event_date, revised, row_order`, bookID, through, through, bookID, through, through)
	if err != nil {
		return nil, fmt.Errorf("read short lot quantity events: %w", err)
	}
	defer rows.Close()
	var events []ShortLotQuantityEvent
	for rows.Next() {
		var event ShortLotQuantityEvent
		var revised bool
		if err := rows.Scan(&event.EventDate, &event.AccountID, &event.CommodityID, &event.QuantityValue, &event.QuantityScale, &revised); err != nil {
			return nil, fmt.Errorf("scan short lot quantity event: %w", err)
		}
		if revised {
			// An allocation records covered units as a positive quantity.
			event.QuantityValue = event.QuantityValue.Negated()
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate short lot quantity events: %w", err)
	}
	return events, nil
}
