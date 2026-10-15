package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"rekenraam/backend/internal/exact"
)

type alternativePositionKey struct {
	investmentReplayPositionKey
	side string
}

// AlternativeMethodGains reuses the disposal writer inside one transaction
// that is always rolled back. A merged stream carries alternative basis to
// destination lots before they open, including chains and transfers back.
// No operational decisions, link revisions or journals are persisted.
func (r *InvestmentRepository) AlternativeMethodGains(ctx context.Context, bookID int64, method string, boundary ObservationBoundary, period RealizedGainsParams) ([]RealizedGainRecord, []UnrealizedGainRecord, error) {
	personPooled := method == "person_pooled_average"
	if personPooled {
		method = "average_cost"
	}
	if method != "fifo" && method != "lifo" && method != "average_cost" || !isDisposalCalendarDate(boundary.AsOf) || boundary.RecordedAt == "" {
		return nil, nil, ErrInvalidDisposalParams
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin alternative gains read: %w", err)
	}
	defer rollbackTx(ctx, tx)
	rows, err := tx.QueryContext(ctx, `SELECT account_id, commodity_id, cost_commodity_id, position_side
		FROM investment_lots WHERE book_id = ? GROUP BY account_id, commodity_id, cost_commodity_id, position_side
		ORDER BY account_id, commodity_id, cost_commodity_id, position_side`, bookID)
	if err != nil {
		return nil, nil, err
	}
	var keys []alternativePositionKey
	for rows.Next() {
		var key alternativePositionKey
		if err := rows.Scan(&key.accountID, &key.commodityID, &key.costCommodityID, &key.side); err != nil {
			rows.Close()
			return nil, nil, err
		}
		keys = append(keys, key)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, nil, err
	}
	type event struct {
		key    alternativePositionKey
		intent InvestmentReplayIntent
	}
	var stream []event
	projections := map[alternativePositionKey]*InvestmentReplayProjection{}
	for _, key := range keys {
		intents, err := investmentReplayIntentsQuery(ctx, tx, bookID, key.accountID, key.commodityID, key.costCommodityID, key.side)
		if err != nil {
			return nil, nil, err
		}
		for _, intent := range intents {
			if period.To == "" || intent.EventDate <= period.To || intent.EventDate <= boundary.AsOf {
				stream = append(stream, event{key, intent})
			}
		}
		if err := resetInvestmentReplayPositionTx(ctx, tx, bookID, key.accountID, key.commodityID, key.costCommodityID, key.side); err != nil {
			return nil, nil, err
		}
		projections[key] = &InvestmentReplayProjection{PositionSide: key.side}
	}
	slices.SortStableFunc(stream, func(a, b event) int { return compareInvestmentReplayIntents(a.intent, b.intent) })
	carried := map[int64]InvestmentReplayTransferRevision{}
	realized := make([]RealizedGainRecord, 0)
	var positions []InvestmentPositionRecord
	for _, item := range stream {
		intent, key := item.intent, item.key
		intent.personPooledAverage = personPooled
		if positions == nil && intent.EventDate > boundary.AsOf {
			positions, err = alternativePositionsTx(ctx, tx, bookID, keys)
			if err != nil {
				return nil, nil, err
			}
		}
		if intent.Kind == "opening" {
			if basis, ok := carried[intent.LotID]; ok {
				intent.BasisKnowledge = normalizedBasisKnowledge(basis.BasisKnowledge)
				intent.AmountValue, intent.AmountScale = exact.New(basis.CostBasisValue), basis.CostBasisScale
				if intent.BasisKnowledge == InvestmentBasisUnknown {
					intent.AmountValue, intent.AmountScale = exact.New(0), 0
				}
			}
		}
		if intent.Kind == "disposal" {
			intent.CostBasisMethod, intent.SpecificLots = method, nil
		}
		projection := projections[key]
		beforeDisposals, beforeTransfers := len(projection.Disposals), len(projection.TransferRevisions)
		switch intent.Kind {
		case "transfer_out", "pooled_transfer_out", "pooled_lot_transfer_out":
			err = alternativeTransferTx(ctx, tx, bookID, key, method, intent, projection)
		default:
			err = applyInvestmentReplayIntentTx(ctx, tx, bookID, key.accountID, key.commodityID, key.costCommodityID, key.side, intent, projection)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("alternative %s operation %d: %w", method, intent.OperationID, err)
		}
		if personPooled {
			if err := rebalancePersonPoolTx(ctx, tx, bookID, key, intent); err != nil {
				return nil, nil, err
			}
		}
		for _, revision := range projection.TransferRevisions[beforeTransfers:] {
			var destination sql.NullInt64
			if err := tx.QueryRowContext(ctx, `SELECT destination_lot_id FROM investment_transfer_lot_links WHERE operation_id = ? AND link_seq = ?`, revision.OperationID, revision.LinkSeq).Scan(&destination); err != nil {
				return nil, nil, err
			}
			if destination.Valid {
				carried[destination.Int64] = revision
			}
		}
		for _, disposal := range projection.Disposals[beforeDisposals:] {
			// A merger exchange intent has no disposal quantity of its own.
			// Keep the cash decision's recorded date, transaction and zero units.
			disposalIntent := intent
			if intent.MergerDecisionID > 0 && disposal.DecisionID == intent.MergerDecisionID {
				if err := tx.QueryRowContext(ctx, `SELECT event_date, transaction_id, quantity_value, quantity_scale
					FROM investment_disposal_decisions WHERE book_id = ? AND id = ?`, bookID, disposal.DecisionID).
					Scan(&disposalIntent.EventDate, &disposalIntent.TransactionID, &disposalIntent.QuantityValue, &disposalIntent.QuantityScale); err != nil {
					return nil, nil, fmt.Errorf("read alternative merger cash decision: %w", err)
				}
			}
			if period.From != "" && disposalIntent.EventDate < period.From || period.To != "" && disposalIntent.EventDate > period.To {
				continue
			}
			record, err := alternativeRealizedGain(key, disposalIntent, disposal)
			if err != nil {
				return nil, nil, err
			}
			realized = append(realized, record)
		}
	}
	if positions == nil {
		positions, err = alternativePositionsTx(ctx, tx, bookID, keys)
		if err != nil {
			return nil, nil, err
		}
	}
	if err := priceDatedPositionsTx(ctx, tx, bookID, boundary, positions); err != nil {
		return nil, nil, err
	}
	// Match the operational read's newest-first disposal order.
	slices.Reverse(realized)
	unrealized, err := valuePositions(positions)
	return realized, unrealized, err
}

func alternativePositionsTx(ctx context.Context, tx *sql.Tx, bookID int64, keys []alternativePositionKey) ([]InvestmentPositionRecord, error) {
	positions := make([]InvestmentPositionRecord, 0, len(keys))
	for _, key := range keys {
		projection := InvestmentReplayProjection{}
		if err := finishInvestmentReplayPositionTx(ctx, tx, bookID, key.accountID, key.commodityID, key.costCommodityID, key.side, &projection); err != nil {
			return nil, err
		}
		quantity, basis := exact.NewScaledInt(), exact.NewScaledInt()
		knowledge := InvestmentBasisKnown
		for _, lot := range projection.Lots {
			if lot.RemainingQuantityValue.Sign() <= 0 {
				continue
			}
			quantity.AddCoefficient(lot.RemainingQuantityValue, lot.RemainingQuantityScale)
			if lot.BasisKnowledge == InvestmentBasisUnknown {
				knowledge = InvestmentBasisUnknown
			} else {
				basis.AddInt64(lot.RemainingCostBasisValue, lot.RemainingCostBasisScale)
			}
		}
		if quantity.Sign() == 0 {
			continue
		}
		q, err := quantity.Coefficient()
		if err != nil {
			return nil, err
		}
		var b int64
		if knowledge == InvestmentBasisKnown {
			b, err = basis.Int64()
			if err != nil {
				return nil, err
			}
		}
		positions = append(positions, InvestmentPositionRecord{AccountID: key.accountID, CommodityID: key.commodityID, CostCommodityID: key.costCommodityID,
			PositionSide: key.side, QuantityValue: q, QuantityScale: quantity.Scale(), BasisKnowledge: knowledge,
			RemainingCostBasisValue: b, RemainingCostBasisScale: basis.Scale()})
	}
	return positions, nil
}

// Transfer links fix economic quantities and destination identities, while
// the selected method supplies their hypothetical depletion and carried basis.
func alternativeTransferTx(ctx context.Context, tx *sql.Tx, bookID int64, key alternativePositionKey, method string, intent InvestmentReplayIntent, projection *InvestmentReplayProjection) error {
	links := []InvestmentReplayTransferLink{{LinkSeq: intent.LinkSeq, QuantityValue: intent.QuantityValue, QuantityScale: intent.QuantityScale}}
	if intent.Kind == "pooled_transfer_out" {
		links = intent.PooledLinks
	}
	for _, link := range links {
		allocations, err := disposeLotsWithAuditTx(ctx, tx, DisposeLotsParams{personPooledAverage: intent.personPooledAverage, BookID: bookID, AccountID: key.accountID,
			CommodityID: key.commodityID, CostCommodityID: key.costCommodityID, PositionSide: key.side,
			TransactionID: intent.TransactionID, EventDate: intent.EventDate, EventKind: "transfer_out", MetadataJSON: "{}",
			QuantityValue: link.QuantityValue, QuantityScale: link.QuantityScale, CostBasisMethod: method, AdmitUnknownBasis: true,
			CreatedAt: intent.CreatedAt, ActorUserID: intent.CreatedByUserID}, intent.AuditEventID, true)
		if err != nil {
			return err
		}
		basis, knowledge := exact.NewScaledInt(), InvestmentBasisKnown
		for _, allocation := range allocations {
			if allocation.BasisKnowledge == InvestmentBasisUnknown {
				knowledge = InvestmentBasisUnknown
			} else {
				basis.AddInt64(allocation.CostBasisValue, allocation.CostBasisScale)
			}
		}
		var value int64
		if knowledge == InvestmentBasisKnown {
			value, err = basis.Int64()
			if err != nil {
				return err
			}
		}
		projection.TransferRevisions = append(projection.TransferRevisions, InvestmentReplayTransferRevision{OperationID: intent.OperationID, LinkSeq: link.LinkSeq,
			CostBasisValue: value, CostBasisScale: basis.Scale(), BasisKnowledge: knowledge})
	}
	return nil
}

func alternativeRealizedGain(key alternativePositionKey, intent InvestmentReplayIntent, disposal InvestmentReplayDisposal) (RealizedGainRecord, error) {
	basis, proceeds := exact.NewScaledInt(), exact.NewScaledInt()
	record := RealizedGainRecord{AccountID: key.accountID, CommodityID: key.commodityID, CostCommodityID: key.costCommodityID,
		PositionSide: key.side, DisposalDate: intent.EventDate, TransactionID: sql.NullInt64{Int64: intent.TransactionID, Valid: intent.TransactionID > 0},
		QuantityValue: intent.QuantityValue, QuantityScale: intent.QuantityScale, BasisKnowledge: InvestmentBasisKnown}
	for _, allocation := range disposal.Allocations {
		proceeds.AddInt64(allocation.ProceedsValue, allocation.ProceedsScale)
		if allocation.BasisKnowledge == InvestmentBasisUnknown {
			record.BasisKnowledge = InvestmentBasisUnknown
		} else {
			basis.AddInt64(allocation.CostBasisValue, allocation.CostBasisScale)
		}
	}
	var err error
	record.ProceedsValue, err = proceeds.Int64()
	if err != nil {
		return record, err
	}
	record.ProceedsScale = proceeds.Scale()
	if record.BasisKnowledge == InvestmentBasisUnknown {
		return record, nil
	}
	if key.side == PositionSideLong {
		basis = basis.Negated()
	}
	record.DisposedBasisValue, err = basis.Int64()
	if err != nil {
		return record, err
	}
	record.DisposedBasisScale = basis.Scale()
	proceeds.AddScaled(basis)
	record.RealizedGainValue, err = proceeds.Int64()
	record.RealizedGainScale = proceeds.Scale()
	return record, err
}

// Reuse the exact average-cost redistribution with a zero depletion: no lot
// event is generated. Account quantities stay in their recorded accounts while
// every surviving lot receives its share of the owner's pool, including after
// acquisitions and structural effects. All of this state is rolled back.
func rebalancePersonPoolTx(ctx context.Context, tx *sql.Tx, bookID int64, key alternativePositionKey, intent InvestmentReplayIntent) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM current_investment_lots
		WHERE book_id = ? AND commodity_id = ? AND cost_commodity_id = ? AND position_side = ?
		AND status = 'open' AND opened_on <= ?`, bookID, key.commodityID, key.costCommodityID, key.side, intent.EventDate).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	params := DisposeLotsParams{personPooledAverage: true, BookID: bookID, AccountID: key.accountID,
		CommodityID: key.commodityID, CostCommodityID: key.costCommodityID, PositionSide: key.side,
		EventDate: intent.EventDate, QuantityValue: exact.New(0), AdmitUnknownBasis: true,
		CreatedAt: intent.CreatedAt, ActorUserID: intent.CreatedByUserID}
	// Opening intents do not need attribution for ordinary activation. The
	// pooled redistribution also touches their neighbours, so retain the
	// opening's existing, same-book attribution for these temporary writes.
	if intent.Kind == "opening" {
		if err := tx.QueryRowContext(ctx, `SELECT created_at, created_by_user_id, created_audit_event_id
			FROM investment_lots WHERE book_id = ? AND id = ?`, bookID, intent.LotID).
			Scan(&params.CreatedAt, &params.ActorUserID, &intent.AuditEventID); err != nil {
			return err
		}
	}
	scale, err := positionBasisAllocationScaleTx(ctx, tx, params)
	if err != nil {
		return err
	}
	_, err = disposeAverageCostTx(ctx, tx, params, intent.AuditEventID, scale)
	return err
}
