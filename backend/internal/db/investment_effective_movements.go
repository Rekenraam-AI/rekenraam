package db

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"rekenraam/backend/internal/exact"
)

// Committed effective lot movements (#189). This file owns the per-lot
// movements a book's committed operational elections currently produce.
// effective_investment_lot_events only removes events that a correction or a
// replay revision superseded; it never adds the revised amounts back. Readers
// that fold lot history into current or dated state use EffectiveLotMovements
// instead of re-assembling the revision families themselves, which is how a
// revised cover was once forgotten (#188).
//
// Out of scope by design (docs/plans/investment-operation-refactor-plan.md,
// *Committed effective lot movements*): replay inputs, which keep original
// elections and recompute allocations; audits and exports of original and
// superseded evidence; per-subject effective terms; activity-date guards; and
// ADR 0012 reporting profiles.

// Movement sources: an original lot event, or the latest revision of one
// revision family standing in for the events it superseded.
const (
	MovementSourceEvent                  = "event"
	MovementSourceDisposalRevision       = "disposal_revision"
	MovementSourceSplitRevision          = "split_revision"
	MovementSourceTransferLinkRevision   = "transfer_link_revision"
	MovementSourcePooledTransferRevision = "pooled_transfer_revision"
	MovementSourceCapitalReturnRevision  = "capital_return_revision"
)

// EffectiveLotMovement is one signed change to one lot under the committed
// elections.
type EffectiveLotMovement struct {
	LotID           int64
	AccountID       int64
	CommodityID     int64
	CostCommodityID int64
	PositionSide    string
	// EventDate is the date of the event the movement stands for. A revised
	// movement takes its subject's date, which a revision never changes.
	EventDate string
	// Kind is the lot-event kind; a revised movement keeps the kind of the
	// events it replaces.
	Kind string
	// QuantityValue is a signed delta. Zero is legitimate for a merger's cash
	// disposal, a spin-off parent's basis reduction and a return of capital.
	QuantityValue exact.Coefficient
	QuantityScale int
	// BasisKnowledge is known or unknown; an unknown basis leaves
	// CostBasisValue and CostBasisScale unused, never zero.
	BasisKnowledge string
	CostBasisValue int64
	CostBasisScale int
	Source         string
	// OperationID is the operation the movement belongs to; 0 for a lot event
	// no operation links (a lot without operation provenance).
	OperationID int64
	// DecisionID is the disposal decision of a disposal movement, else 0.
	DecisionID    int64
	TransactionID sql.NullInt64
	// OrderOperationID is the correction root's same-day slot. EffectSeq is
	// the immutable event sequence, or the first effect of a revised subject.
	// RevisionEffectSeq orders that revision's children within the slot.
	// An unlinked original event has zero operation/order/effect fields.
	OrderOperationID  int64
	EffectSeq         int
	RevisionEffectSeq int
	// A link is identified by (TransferOperationID, LinkSeq), including
	// pooled source depletions. Both are zero for non-transfer movements.
	TransferOperationID int64
	LinkSeq             int
	// RevisionID identifies the exact row in Source's revision family;
	// RevisionSeq is its recorded version. Both are zero for original events.
	RevisionID  int64
	RevisionSeq int
	// OriginEventID is the immutable event anchoring the movement's slot.
	// A revision may move a different lot or number of lots than its anchor.
	// A split initially without effects anchors to its sourced fact instead (0).
	OriginEventID int64
}

// EffectiveMovementFilter narrows the read to what a reader folds, so damage
// in movements it never uses cannot fail it: a short reader does not validate
// long allocations, and realized gains reads disposals only.
type EffectiveMovementFilter struct {
	// PositionSide keeps one side's lots; empty keeps both.
	PositionSide string
	// DisposalsOnly keeps disposal movements: disposal events and revised
	// disposal allocations.
	DisposalsOnly bool
}

// EffectiveLotMovements reads the book's committed effective lot movements
// that pass the filter, ordered by date, correction-root slot and effect
// sequence, then revision child sequence and deterministic identity ties.
func EffectiveLotMovements(ctx context.Context, source queryer, bookID int64, filter EffectiveMovementFilter) ([]EffectiveLotMovement, error) {
	reads := []func(context.Context, queryer, int64, EffectiveMovementFilter) ([]EffectiveLotMovement, error){
		effectiveEventMovements, revisedDisposalMovements,
	}
	if !filter.DisposalsOnly {
		reads = append(reads, revisedSplitMovements, revisedTransferLinkMovements,
			revisedPooledTransferMovements, revisedCapitalReturnMovements)
	}
	var movements []EffectiveLotMovement
	for _, read := range reads {
		read, err := read(ctx, source, bookID, filter)
		if err != nil {
			return nil, err
		}
		movements = append(movements, read...)
	}
	if err := effectiveMovementProvenance(ctx, source, bookID, movements); err != nil {
		return nil, err
	}
	slices.SortFunc(movements, compareEffectiveLotMovements)
	return movements, nil
}

func compareEffectiveLotMovements(a, b EffectiveLotMovement) int {
	return cmp.Or(
		cmp.Compare(a.EventDate, b.EventDate),
		cmp.Compare(a.OrderOperationID, b.OrderOperationID),
		cmp.Compare(a.EffectSeq, b.EffectSeq),
		cmp.Compare(a.RevisionEffectSeq, b.RevisionEffectSeq),
		cmp.Compare(a.Source, b.Source),
		cmp.Compare(a.RevisionID, b.RevisionID),
		cmp.Compare(a.LinkSeq, b.LinkSeq),
		cmp.Compare(a.OriginEventID, b.OriginEventID),
		cmp.Compare(a.LotID, b.LotID),
		cmp.Compare(a.Kind, b.Kind),
	)
}

// effectiveMovementProvenance attaches immutable subject slots in two bulk
// metadata reads. It reads no amounts, so unrelated damaged allocations do
// not widen a filtered reader's failure scope. Replay and committed reads
// share the correction ancestry calculation, never the replay allocations.
func effectiveMovementProvenance(ctx context.Context, source queryer, bookID int64, movements []EffectiveLotMovement) error {
	if len(movements) == 0 {
		return nil
	}
	roots, err := investmentReplayOrderOperationIDsQuery(ctx, source, bookID)
	if err != nil {
		return err
	}
	type effect struct {
		eventID, operationID, costCommodityID, decisionID, transferOperationID int64
		seq, linkSeq                                                           int
		kind                                                                   string
		transactionID                                                          sql.NullInt64
	}
	type subject struct {
		operationID int64
		kind        string
		id          int64 // Decision, split cost currency, link sequence, or 0 for capital return.
	}
	rows, err := source.QueryContext(ctx, `
		SELECT e.id, x.operation_id, l.cost_commodity_id, x.effect_seq, e.event_kind, e.transaction_id,
			COALESCE(a.decision_id, 0), COALESCE(link.operation_id, resolution.transfer_operation_id, 0),
			COALESCE(link.link_seq, resolution.link_seq, 0)
		FROM investment_operation_lot_effects x
		JOIN investment_operations o ON o.id = x.operation_id
		JOIN investment_lot_events e ON e.id = x.lot_event_id
		JOIN investment_lots l ON l.id = e.lot_id
		LEFT JOIN investment_disposal_allocations a ON a.lot_event_id = e.id
		LEFT JOIN investment_transfer_lot_links link ON link.operation_id = x.operation_id
			AND (link.cost_commodity_id IS NULL OR link.cost_commodity_id = l.cost_commodity_id)
			AND ((e.event_kind IN ('transfer_out', 'basis_reduction')
				AND (link.source_lot_id = e.lot_id OR link.source_lot_id IS NULL))
				OR (e.event_kind IN ('transfer_in', 'acquisition') AND link.destination_lot_id = e.lot_id))
		LEFT JOIN investment_basis_resolutions resolution ON resolution.operation_id = x.operation_id
			AND resolution.lot_id = e.lot_id
		WHERE o.book_id = ?`, bookID)
	if err != nil {
		return fmt.Errorf("read effective movement provenance: %w", err)
	}
	byEvent := make(map[int64]effect)
	anchors := make(map[subject]effect)
	for rows.Next() {
		var e effect
		if err := rows.Scan(&e.eventID, &e.operationID, &e.costCommodityID, &e.seq, &e.kind,
			&e.transactionID, &e.decisionID, &e.transferOperationID, &e.linkSeq); err != nil {
			rows.Close()
			return fmt.Errorf("scan effective movement provenance: %w", err)
		}
		byEvent[e.eventID] = e
		key := subject{operationID: e.operationID, kind: e.kind, id: int64(e.linkSeq)}
		switch e.kind {
		case "disposal":
			key.id = e.decisionID
		case "split_adjustment":
			key.id = e.costCommodityID
		}
		if previous, ok := anchors[key]; !ok || e.seq < previous.seq {
			anchors[key] = e
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("read effective movement provenance: %w", err)
	}
	for i := range movements {
		m := &movements[i]
		if m.OperationID == 0 {
			continue // Preserve unlinked evidence without fabricating an owner.
		}
		m.OrderOperationID = roots[m.OperationID]
		var anchor effect
		if m.Source == MovementSourceEvent {
			anchor = byEvent[m.OriginEventID]
			m.TransferOperationID, m.LinkSeq = anchor.transferOperationID, anchor.linkSeq
		} else {
			key := subject{operationID: m.OperationID, kind: m.Kind, id: int64(m.LinkSeq)}
			switch m.Source {
			case MovementSourceDisposalRevision:
				key.id = m.DecisionID
			case MovementSourceSplitRevision:
				key.id = m.CostCommodityID
			}
			anchor = anchors[key]
			m.OriginEventID, m.TransactionID = anchor.eventID, anchor.transactionID
			m.TransferOperationID = anchor.transferOperationID
		}
		// Split revisions are read through investment_split_facts. A split
		// can initially have no effects, or no effects in this cost currency;
		// its sourced fact anchors the same slot as replay's split intent.
		if m.Source == MovementSourceSplitRevision && anchor.eventID == 0 && m.OrderOperationID > 0 {
			m.EffectSeq = 1
			continue
		}
		if m.OrderOperationID <= 0 || anchor.eventID == 0 {
			return fmt.Errorf("effective %s movement of operation %d has no immutable order anchor", m.Source, m.OperationID)
		}
		m.EffectSeq = anchor.seq
	}
	return nil
}

func effectiveEventMovements(ctx context.Context, source queryer, bookID int64, filter EffectiveMovementFilter) ([]EffectiveLotMovement, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT e.id, e.lot_id, l.account_id, l.commodity_id, l.cost_commodity_id, l.position_side,
			e.event_date, e.event_kind, e.quantity_value, e.quantity_scale,
			e.cost_basis_value, e.cost_basis_scale, e.basis_knowledge, e.transaction_id,
			COALESCE((SELECT x.operation_id FROM investment_operation_lot_effects x WHERE x.lot_event_id = e.id), 0),
			COALESCE((SELECT a.decision_id FROM investment_disposal_allocations a WHERE a.lot_event_id = e.id), 0)
		FROM effective_investment_lot_events e
		JOIN investment_lots l ON l.id = e.lot_id
		WHERE e.book_id = ? AND (? = '' OR l.position_side = ?) AND (? = 0 OR e.event_kind = 'disposal')
		ORDER BY e.lot_id, e.event_date, e.id`, bookID, filter.PositionSide, filter.PositionSide, filter.DisposalsOnly)
	if err != nil {
		return nil, fmt.Errorf("read effective lot events: %w", err)
	}
	var movements []EffectiveLotMovement
	for rows.Next() {
		movement := EffectiveLotMovement{Source: MovementSourceEvent}
		var value, scale sql.NullInt64
		if err := rows.Scan(&movement.OriginEventID, &movement.LotID, &movement.AccountID, &movement.CommodityID,
			&movement.CostCommodityID, &movement.PositionSide, &movement.EventDate, &movement.Kind,
			&movement.QuantityValue, &movement.QuantityScale, &value, &scale, &movement.BasisKnowledge,
			&movement.TransactionID, &movement.OperationID, &movement.DecisionID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan effective lot event: %w", err)
		}
		movement.CostBasisValue, movement.CostBasisScale, err = projectedBasis(value, scale, movement.BasisKnowledge)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("read effective lot event %d basis: %w", movement.OriginEventID, err)
		}
		movements = append(movements, movement)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("read effective lot events: %w", err)
	}
	return movements, nil
}

// revisedDisposalMovements takes each revised disposal's latest allocation set
// out of its lots on the decision date.
func revisedDisposalMovements(ctx context.Context, source queryer, bookID int64, filter EffectiveMovementFilter) ([]EffectiveLotMovement, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT allocation.lot_id, l.account_id, l.commodity_id, l.cost_commodity_id, l.position_side,
			decision.event_date, allocation.quantity_value, allocation.quantity_scale,
			allocation.cost_basis_value, allocation.cost_basis_scale, allocation.basis_knowledge,
			-- A cash-and-stock merger's cash disposes no units (#181).
			operation.operation_kind = 'cash_and_stock_merger',
			decision.operation_id, decision.id, decision.transaction_id,
			revision.id, revision.revision_seq, allocation.allocation_seq
		FROM latest_investment_disposal_revisions revision
		JOIN investment_disposal_decisions decision ON decision.id = revision.decision_id
		JOIN effective_investment_operations operation ON operation.id = decision.operation_id
		JOIN investment_disposal_revision_allocations allocation ON allocation.revision_id = revision.id
		JOIN investment_lots l ON l.id = allocation.lot_id
		WHERE revision.book_id = ? AND (? = '' OR l.position_side = ?)
		ORDER BY revision.decision_id, allocation.allocation_seq`, bookID, filter.PositionSide, filter.PositionSide)
	if err != nil {
		return nil, fmt.Errorf("read revised disposal allocations: %w", err)
	}
	var movements []EffectiveLotMovement
	for rows.Next() {
		movement := EffectiveLotMovement{Source: MovementSourceDisposalRevision, Kind: "disposal"}
		var quantity exact.Coefficient
		var value, scale sql.NullInt64
		var basisOnly bool
		if err := rows.Scan(&movement.LotID, &movement.AccountID, &movement.CommodityID, &movement.CostCommodityID,
			&movement.PositionSide, &movement.EventDate, &quantity, &movement.QuantityScale, &value, &scale,
			&movement.BasisKnowledge, &basisOnly, &movement.OperationID, &movement.DecisionID,
			&movement.TransactionID,
			&movement.RevisionID, &movement.RevisionSeq, &movement.RevisionEffectSeq); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan revised disposal allocation: %w", err)
		}
		basis, basisScale, err := projectedBasis(value, scale, movement.BasisKnowledge)
		if err != nil || basis < 0 || quantity.Sign() < 0 || (quantity.Sign() == 0) != basisOnly {
			rows.Close()
			return nil, fmt.Errorf("invalid revised disposal allocation for lot %d", movement.LotID)
		}
		movement.QuantityValue = quantity.Negated()
		movement.CostBasisValue, movement.CostBasisScale = -basis, basisScale
		movements = append(movements, movement)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("read revised disposal allocations: %w", err)
	}
	return movements, nil
}

// revisedSplitMovements applies a replayed split's latest effects per cost
// currency on the split's effective date. A split moves no basis.
func revisedSplitMovements(ctx context.Context, source queryer, bookID int64, filter EffectiveMovementFilter) ([]EffectiveLotMovement, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT effect.lot_id, l.account_id, l.commodity_id, l.cost_commodity_id, l.position_side,
			fact.effective_on, effect.quantity_delta_value, effect.quantity_delta_scale, revision.operation_id,
			revision.id, revision.revision_seq, effect.effect_seq
		FROM latest_investment_split_revisions revision
		JOIN effective_investment_operations operation ON operation.id = revision.operation_id
		JOIN investment_split_facts fact ON fact.operation_id = revision.operation_id
		JOIN investment_split_revision_effects effect ON effect.revision_id = revision.id
		JOIN investment_lots l ON l.id = effect.lot_id
		WHERE revision.book_id = ? AND (? = '' OR l.position_side = ?)
		ORDER BY revision.id, effect.effect_seq`, bookID, filter.PositionSide, filter.PositionSide)
	if err != nil {
		return nil, fmt.Errorf("read revised split effects: %w", err)
	}
	var movements []EffectiveLotMovement
	for rows.Next() {
		movement := EffectiveLotMovement{Source: MovementSourceSplitRevision, Kind: "split_adjustment",
			BasisKnowledge: InvestmentBasisKnown}
		if err := rows.Scan(&movement.LotID, &movement.AccountID, &movement.CommodityID, &movement.CostCommodityID,
			&movement.PositionSide, &movement.EventDate, &movement.QuantityValue, &movement.QuantityScale,
			&movement.OperationID, &movement.RevisionID, &movement.RevisionSeq, &movement.RevisionEffectSeq); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan revised split effect: %w", err)
		}
		movements = append(movements, movement)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("read revised split effects: %w", err)
	}
	return movements, nil
}

// revisedTransferLinkMovements moves a revised lot-to-lot link's units at its
// latest carried basis on the transfer date: out of its effective source lot
// (the corrected successor of the original acquisition, when replaced) and
// into the destination (T-132). An outbound link (T-143) has no destination.
// A share exchange's destination receives its own lot's units (#177); a
// spin-off parent keeps its units and gives up only basis (#180).
func revisedTransferLinkMovements(ctx context.Context, source queryer, bookID int64, filter EffectiveMovementFilter) ([]EffectiveLotMovement, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT revision.source_lot_id, src.account_id, src.commodity_id, src.cost_commodity_id, src.position_side,
			COALESCE(link.destination_lot_id, 0), COALESCE(destination.account_id, 0),
			COALESCE(destination.commodity_id, 0), COALESCE(destination.cost_commodity_id, 0),
			COALESCE(destination.position_side, ''),
			link.quantity_value, link.quantity_scale, revision.carried_basis_value, revision.carried_basis_scale,
			revision.basis_knowledge,
			COALESCE(destination.quantity_value, link.quantity_value),
			COALESCE(destination.quantity_scale, link.quantity_scale),
			fact.transfer_kind = 'spin_off', fact.effective_on, revision.operation_id,
			revision.link_seq, revision.id, revision.revision_seq
		FROM latest_investment_transfer_link_revisions revision
		JOIN effective_investment_operations operation ON operation.id = revision.operation_id
		JOIN investment_transfer_facts fact ON fact.operation_id = revision.operation_id
		JOIN investment_transfer_lot_links link ON link.operation_id = revision.operation_id
			AND link.link_seq = revision.link_seq
		JOIN investment_lots src ON src.id = revision.source_lot_id
		LEFT JOIN investment_lots destination ON destination.id = link.destination_lot_id
		WHERE revision.book_id = ? AND (? = '' OR src.position_side = ?)
		ORDER BY revision.operation_id, revision.link_seq`, bookID, filter.PositionSide, filter.PositionSide)
	if err != nil {
		return nil, fmt.Errorf("read revised transfer links: %w", err)
	}
	var movements []EffectiveLotMovement
	for rows.Next() {
		out := EffectiveLotMovement{Source: MovementSourceTransferLinkRevision, Kind: "transfer_out"}
		in := EffectiveLotMovement{Source: MovementSourceTransferLinkRevision, Kind: "transfer_in"}
		var quantity, inQuantity exact.Coefficient
		var scale, inScale int
		var basis sql.NullString
		var basisScale sql.NullInt64
		var knowledge, date string
		var spinOff bool
		var operationID int64
		if err := rows.Scan(&out.LotID, &out.AccountID, &out.CommodityID, &out.CostCommodityID, &out.PositionSide,
			&in.LotID, &in.AccountID, &in.CommodityID, &in.CostCommodityID, &in.PositionSide,
			&quantity, &scale, &basis, &basisScale, &knowledge, &inQuantity, &inScale,
			&spinOff, &date, &operationID, &out.LinkSeq, &out.RevisionID, &out.RevisionSeq); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan revised transfer link: %w", err)
		}
		value, valueScale, err := revisionBasis(basis, basisScale, knowledge)
		if err != nil || value < 0 || quantity.Sign() <= 0 || inQuantity.Sign() <= 0 {
			rows.Close()
			return nil, fmt.Errorf("invalid revised transfer link for lot %d", in.LotID)
		}
		in.LinkSeq, in.RevisionID, in.RevisionSeq = out.LinkSeq, out.RevisionID, out.RevisionSeq
		for _, movement := range []*EffectiveLotMovement{&out, &in} {
			movement.EventDate, movement.OperationID, movement.BasisKnowledge = date, operationID, knowledge
			movement.CostBasisScale = valueScale
		}
		in.QuantityValue, in.QuantityScale, in.CostBasisValue = inQuantity, inScale, value
		out.QuantityValue, out.QuantityScale, out.CostBasisValue = quantity.Negated(), scale, -value
		if spinOff {
			out.Kind, out.QuantityValue, out.QuantityScale = "basis_reduction", "0", 0
		}
		movements = append(movements, out)
		if in.LotID > 0 {
			movements = append(movements, in)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("read revised transfer links: %w", err)
	}
	return movements, nil
}

// revisedPooledTransferMovements takes a revised pooled_lot transfer's latest
// depletions out of its source lots and its whole quantity into the one
// destination (T-135).
func revisedPooledTransferMovements(ctx context.Context, source queryer, bookID int64, filter EffectiveMovementFilter) ([]EffectiveLotMovement, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT depletion.source_lot_id, src.account_id, src.commodity_id, src.cost_commodity_id, src.position_side,
			depletion.quantity_value, depletion.quantity_scale, depletion.cost_basis_value, depletion.cost_basis_scale,
			depletion.basis_knowledge, 0, fact.effective_on, revision.operation_id,
			revision.link_seq, revision.id, revision.revision_seq, depletion.depletion_seq
		FROM latest_investment_transfer_link_revisions revision
		JOIN effective_investment_operations operation ON operation.id = revision.operation_id
		JOIN investment_transfer_facts fact ON fact.operation_id = revision.operation_id
		JOIN investment_transfer_link_revision_depletions depletion ON depletion.revision_id = revision.id
		JOIN investment_lots src ON src.id = depletion.source_lot_id
		WHERE revision.book_id = ? AND revision.source_lot_id IS NULL AND (? = '' OR src.position_side = ?)
		UNION ALL
		SELECT link.destination_lot_id, destination.account_id, destination.commodity_id, destination.cost_commodity_id,
			destination.position_side, link.quantity_value, link.quantity_scale,
			revision.carried_basis_value, revision.carried_basis_scale,
			revision.basis_knowledge, 1, fact.effective_on, revision.operation_id,
			revision.link_seq, revision.id, revision.revision_seq, 0
		FROM latest_investment_transfer_link_revisions revision
		JOIN effective_investment_operations operation ON operation.id = revision.operation_id
		JOIN investment_transfer_facts fact ON fact.operation_id = revision.operation_id
		JOIN investment_transfer_lot_links link ON link.operation_id = revision.operation_id
			AND link.link_seq = revision.link_seq
		JOIN investment_lots destination ON destination.id = link.destination_lot_id
		WHERE revision.book_id = ? AND revision.source_lot_id IS NULL AND (? = '' OR destination.position_side = ?)`,
		bookID, filter.PositionSide, filter.PositionSide, bookID, filter.PositionSide, filter.PositionSide)
	if err != nil {
		return nil, fmt.Errorf("read revised pooled transfers: %w", err)
	}
	var movements []EffectiveLotMovement
	for rows.Next() {
		movement := EffectiveLotMovement{Source: MovementSourcePooledTransferRevision}
		var quantity exact.Coefficient
		var basis sql.NullString
		var basisScale sql.NullInt64
		var incoming bool
		if err := rows.Scan(&movement.LotID, &movement.AccountID, &movement.CommodityID, &movement.CostCommodityID,
			&movement.PositionSide, &quantity, &movement.QuantityScale, &basis, &basisScale,
			&movement.BasisKnowledge, &incoming, &movement.EventDate, &movement.OperationID,
			&movement.LinkSeq, &movement.RevisionID, &movement.RevisionSeq, &movement.RevisionEffectSeq); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan revised pooled transfer: %w", err)
		}
		value, valueScale, err := revisionBasis(basis, basisScale, movement.BasisKnowledge)
		if err != nil || value < 0 || quantity.Sign() <= 0 {
			rows.Close()
			return nil, fmt.Errorf("invalid revised pooled transfer for lot %d", movement.LotID)
		}
		movement.CostBasisScale = valueScale
		movement.Kind, movement.QuantityValue, movement.CostBasisValue = "transfer_in", quantity, value
		if !incoming {
			movement.Kind, movement.QuantityValue, movement.CostBasisValue = "transfer_out", quantity.Negated(), -value
		}
		movements = append(movements, movement)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("read revised pooled transfers: %w", err)
	}
	return movements, nil
}

// revisedCapitalReturnMovements reduces each lot by a revised return of
// capital's latest reduction on its effective date (T-148).
func revisedCapitalReturnMovements(ctx context.Context, source queryer, bookID int64, filter EffectiveMovementFilter) ([]EffectiveLotMovement, error) {
	rows, err := source.QueryContext(ctx, `
		SELECT e.lot_id, l.account_id, l.commodity_id, l.cost_commodity_id, l.position_side,
			e.reduction_value, e.reduction_scale, e.basis_knowledge, fact.effective_on, revision.operation_id,
			revision.id, revision.revision_seq, e.effect_seq
		FROM latest_investment_capital_return_revisions revision
		JOIN effective_investment_operations operation ON operation.id = revision.operation_id
		JOIN investment_capital_return_facts fact ON fact.operation_id = revision.operation_id
		JOIN investment_capital_return_revision_effects e ON e.revision_id = revision.id
		JOIN investment_lots l ON l.id = e.lot_id
		WHERE revision.book_id = ? AND (? = '' OR l.position_side = ?)
		ORDER BY revision.id, e.effect_seq`, bookID, filter.PositionSide, filter.PositionSide)
	if err != nil {
		return nil, fmt.Errorf("read revised capital returns: %w", err)
	}
	var movements []EffectiveLotMovement
	for rows.Next() {
		movement := EffectiveLotMovement{Source: MovementSourceCapitalReturnRevision, Kind: "basis_reduction",
			QuantityValue: "0"}
		var reduction sql.NullString
		var scale sql.NullInt64
		if err := rows.Scan(&movement.LotID, &movement.AccountID, &movement.CommodityID, &movement.CostCommodityID,
			&movement.PositionSide, &reduction, &scale, &movement.BasisKnowledge, &movement.EventDate,
			&movement.OperationID, &movement.RevisionID, &movement.RevisionSeq, &movement.RevisionEffectSeq); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan revised capital return: %w", err)
		}
		value, valueScale, err := revisionBasis(reduction, scale, movement.BasisKnowledge)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("invalid revised capital return reduction for lot %d", movement.LotID)
		}
		movement.CostBasisValue, movement.CostBasisScale = -value, valueScale
		movements = append(movements, movement)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("read revised capital returns: %w", err)
	}
	return movements, nil
}

// revisionBasis reads a revision's TEXT basis tuple; unknown leaves the
// numeric fields unused.
func revisionBasis(value sql.NullString, scale sql.NullInt64, knowledge string) (int64, int, error) {
	if knowledge == InvestmentBasisUnknown && !value.Valid && !scale.Valid {
		return 0, 0, nil
	}
	if knowledge != InvestmentBasisKnown || !value.Valid || !scale.Valid {
		return 0, 0, fmt.Errorf("invalid revision basis knowledge/amount pair")
	}
	parsed, err := exact.Parse(value.String)
	if err != nil {
		return 0, 0, err
	}
	amount, err := exact.ScaledIntFromCoefficient(parsed, int(scale.Int64)).Int64()
	return amount, int(scale.Int64), err
}
