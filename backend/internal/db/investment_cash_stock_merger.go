package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"rekenraam/backend/internal/exact"
)

// Cash-and-stock merger (#181, operation plan "Compound corporate actions").
// It is a share exchange that also pays cash: every long lot of the old
// instrument open in the holding at the slot moves whole into a lot of the new
// instrument at the exact ratio, as for an exchange, after the cash recovers
// an elected share of its basis. For a lot of remaining basis b and the
// recorded fraction f, the realized basis a = b × f (truncated at the
// position's allocation scale) leaves through a quantity-0 disposal event,
// and the exchange's transfer_out carries b − a into the new lot, so basis is
// conserved per lot. The cash leg is one long disposal decision of quantity
// 0 whose proceeds are the cash, allocated across the lots by old units; its
// gain is cash − Σa.
//
// The journal is one transaction with two entries: the exchange's four
// instrument legs on the effective date and the cash (account +C,
// commodity_trading −C) on the payable date, never earlier.

var (
	// ErrCashStockMergerCurrency refuses a merger whose cash is in another
	// currency than an exchanged lot's cost basis: that would need explicit
	// conversion legs.
	ErrCashStockMergerCurrency = errors.New("a cash-and-stock merger's cash must be in every exchanged lot's cost currency")
	// ErrCashStockMergerLotChanged means replay no longer finds a merged lot
	// open with the units the merger took.
	ErrCashStockMergerLotChanged = errors.New("a merged lot no longer holds the units the merger took")
)

// CreateCashStockMergerParams are the sourced merger terms: the exchange,
// the actual cash and the basis election.
type CreateCashStockMergerParams struct {
	Exchange               CreateShareExchangeParams
	CashCommodityID        int64
	CashValue              int64
	CashScale              int
	PayableOn              string
	CashBasisFractionValue exact.Coefficient
	CashBasisFractionScale int
}

// CashStockMergerLink is one lot's move with its cash part: the exchange link
// carries b − a, RealizedBasis is a and Cash the lot's share of the cash.
type CashStockMergerLink struct {
	ShareExchangeLink
	RealizedBasisValue int64
	RealizedBasisScale int
	CashValue          int64
	CashScale          int
}

// CashStockMergerPlan is every link and the exchange's journal totals.
type CashStockMergerPlan struct {
	Links                    []CashStockMergerLink
	SourceQuantityValue      exact.Coefficient
	SourceQuantityScale      int
	DestinationQuantityValue exact.Coefficient
	DestinationQuantityScale int
}

// planCashStockMergerTx plans the exchange part and refuses cash in another
// currency than a lot's basis. A merger dated behind a later rewrite of the
// old holding, or a replacement, replays that holding with the merger as its
// subject at the slot, so it realizes and takes every lot open then (#184).
func planCashStockMergerTx(ctx context.Context, tx *sql.Tx, params CreateCashStockMergerParams, subject compoundActionSubject) (shareExchangeWritePlan, error) {
	if params.CashCommodityID <= 0 || params.CashValue <= 0 ||
		params.CashScale < 0 || params.CashScale > 12 || !isDisposalCalendarDate(params.PayableOn) ||
		params.PayableOn < params.Exchange.EffectiveOn || !validCashBasisFraction(params.CashBasisFractionValue, params.CashBasisFractionScale) {
		return shareExchangeWritePlan{}, fmt.Errorf("%w: cash-and-stock merger terms are incomplete", ErrInvalidDisposalParams)
	}
	plan, err := planShareExchangeAsTx(ctx, tx, params.Exchange, subject, InvestmentReplayIntent{
		OperationKind: "cash_and_stock_merger", BasisFractionValue: params.CashBasisFractionValue,
		BasisFractionScale: params.CashBasisFractionScale})
	if err != nil {
		return shareExchangeWritePlan{}, err
	}
	for _, link := range plan.Links {
		if link.CostCommodityID != params.CashCommodityID {
			return shareExchangeWritePlan{}, ErrCashStockMergerCurrency
		}
	}
	if plan.Replayed && len(plan.Realizations) != len(plan.Links) {
		return shareExchangeWritePlan{}, fmt.Errorf("%w: replay realized %d of %d merged lots", ErrInvalidDisposalParams,
			len(plan.Realizations), len(plan.Links))
	}
	return plan, nil
}

func validCashBasisFraction(value exact.Coefficient, scale int) bool {
	if scale < 0 || scale > 12 {
		return false
	}
	parsed, err := exact.Parse(string(value))
	if err != nil || parsed.Sign() < 0 {
		return false
	}
	return parsed.BigInt().Cmp(exact.Pow10(scale)) <= 0
}

// PlanCashStockMerger computes the exchange totals in a transaction that is
// always rolled back.
func (r *InvestmentRepository) PlanCashStockMerger(ctx context.Context, params CreateCashStockMergerParams, actorUserID int64) (ShareExchangePlan, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return ShareExchangePlan{}, fmt.Errorf("begin cash-and-stock merger plan: %w", err)
	}
	defer rollbackTx(ctx, tx)
	plan, err := planCashStockMergerTx(ctx, tx, params, compoundActionSubject{ActorUserID: actorUserID, CreatedAt: "1970-01-01T00:00:00Z"})
	if err != nil {
		return ShareExchangePlan{}, err
	}
	return plan.ShareExchangePlan, nil
}

// CreateCashStockMerger commits the journal, the exchange fact and election,
// the payable date, every realization, depletion, destination lot and link,
// and the cash decision under one audit event.
func (r *InvestmentRepository) CreateCashStockMerger(ctx context.Context, journal CreateTransactionParams, params CreateCashStockMergerParams) (TransactionRecord, CashStockMergerPlan, error) {
	return r.createCashStockMerger(ctx, journal, params, false)
}

// SimulateCashStockMerger runs the complete writer, then rolls back.
func (r *InvestmentRepository) SimulateCashStockMerger(ctx context.Context, journal CreateTransactionParams, params CreateCashStockMergerParams) (SimulatedInvestmentWrite, CashStockMergerPlan, error) {
	transaction, plan, err := r.createCashStockMerger(ctx, journal, params, true)
	if err != nil {
		return SimulatedInvestmentWrite{}, CashStockMergerPlan{}, err
	}
	for index := range plan.Links {
		plan.Links[index].DestinationLotID = 0
	}
	return simulatedInvestmentWrite(transaction), plan, nil
}

func (r *InvestmentRepository) createCashStockMerger(ctx context.Context, journal CreateTransactionParams, params CreateCashStockMergerParams, preview bool) (TransactionRecord, CashStockMergerPlan, error) {
	write := executeInvestmentWriteTx[CashStockMergerPlan]
	if preview {
		write = previewInvestmentWriteTx[CashStockMergerPlan]
	}
	return write(ctx, r.database, journal, func(tx *sql.Tx, transaction TransactionRecord, auditEventID int64) (CashStockMergerPlan, error) {
		return writeCashStockMergerTx(ctx, tx, params, transaction, journal, auditEventID)
	}, nil)
}

func writeCashStockMergerTx(ctx context.Context, tx *sql.Tx, params CreateCashStockMergerParams, transaction TransactionRecord,
	journal CreateTransactionParams, auditEventID int64) (CashStockMergerPlan, error) {
	exchange := params.Exchange
	operationID, err := investmentOperationIDTx(ctx, tx, exchange.BookID, transaction.ID)
	if err != nil {
		return CashStockMergerPlan{}, err
	}
	planned, err := planCashStockMergerTx(ctx, tx, params, compoundActionSubject{OperationID: operationID,
		TransactionID: transaction.ID, AuditEventID: auditEventID, ActorUserID: journal.ActorUserID, CreatedAt: journal.CreatedAt})
	if err != nil {
		return CashStockMergerPlan{}, err
	}
	same := func(value exact.Coefficient, scale int, expectedValue exact.Coefficient, expectedScale int) bool {
		return exact.ScaledIntFromCoefficient(value, scale).Cmp(exact.ScaledIntFromCoefficient(expectedValue, expectedScale)) == 0
	}
	if !same(planned.SourceQuantityValue, planned.SourceQuantityScale, exchange.ExpectedSourceQuantityValue, exchange.ExpectedSourceQuantityScale) ||
		!same(planned.DestinationQuantityValue, planned.DestinationQuantityScale,
			exchange.ExpectedDestinationQuantityValue, exchange.ExpectedDestinationQuantityScale) {
		return CashStockMergerPlan{}, ErrShareExchangePositionChanged
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO investment_transfer_facts
		(operation_id, book_id, transfer_kind, effective_on, commodity_id, source_account_id,
		 destination_account_id, source_evidence_json, created_audit_event_id,
		 destination_commodity_id, ratio_numerator, ratio_denominator,
		 cash_basis_fraction_value, cash_basis_fraction_scale)
		VALUES (?, ?, 'exchange', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, operationID, exchange.BookID, exchange.EffectiveOn,
		exchange.CommodityID, exchange.AccountID, exchange.DestinationAccountID, exchange.SourceEvidenceJSON, auditEventID,
		exchange.DestinationCommodityID, exchange.RatioNumerator, exchange.RatioDenominator,
		params.CashBasisFractionValue, params.CashBasisFractionScale); err != nil {
		return CashStockMergerPlan{}, fmt.Errorf("record cash-and-stock merger fact: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO investment_operation_dates (operation_id, date_role, date_seq, event_date)
		VALUES (?, 'payable', 1, ?)`, operationID, params.PayableOn); err != nil {
		return CashStockMergerPlan{}, fmt.Errorf("record cash-and-stock merger payable date: %w", err)
	}
	depletionParams := shareExchangeDepletionParams(exchange, params.CashCommodityID, transaction.ID, journal)
	allocationScale, err := positionBasisAllocationScaleTx(ctx, tx, depletionParams)
	if err != nil {
		return CashStockMergerPlan{}, err
	}
	// The cash recovers its share of each lot's basis first: a quantity-0
	// disposal event per lot. Its decision links those events below. A
	// replayed plan records the realizations replay produced at the slot; the
	// replay that follows installs the projection (#184).
	realizations := make([]LotDisposalRecord, len(planned.Links))
	for index, link := range planned.Links {
		var realization LotDisposalRecord
		if planned.Replayed {
			realization = planned.Realizations[index]
		} else if realization, err = realizeCashStockMergerLotTx(ctx, tx, exchange.BookID, exchange.AccountID, exchange.CommodityID,
			params.CashCommodityID, link.SourceLotID, exchange.EffectiveOn, link.SourceQuantityValue, link.SourceQuantityScale,
			params.CashBasisFractionValue, params.CashBasisFractionScale, allocationScale, journal.CreatedAt,
			journal.ActorUserID, auditEventID); err != nil {
			return CashStockMergerPlan{}, err
		}
		knowledge := normalizedBasisKnowledge(realization.BasisKnowledge)
		result, err := tx.ExecContext(ctx, `INSERT INTO investment_lot_events (
			book_id, lot_id, event_kind, transaction_id, event_date, quantity_value, quantity_scale,
			cost_basis_value, cost_basis_scale, metadata_json,
			created_at, created_by_user_id, created_audit_event_id, basis_knowledge
		) VALUES (?, ?, 'disposal', ?, ?, '0', 0, ?, ?, ?, ?, ?, ?, ?)`,
			exchange.BookID, link.SourceLotID, transaction.ID, exchange.EffectiveOn,
			nullableBasisValue(-realization.CostBasisValue, knowledge), nullableBasisScale(realization.CostBasisScale, knowledge),
			exchange.SourceEvidenceJSON, journal.CreatedAt, journal.ActorUserID, auditEventID, knowledge)
		if err != nil {
			return CashStockMergerPlan{}, fmt.Errorf("record cash-and-stock merger realization: %w", err)
		}
		if realization.EventID, err = result.LastInsertId(); err != nil {
			return CashStockMergerPlan{}, fmt.Errorf("read cash-and-stock merger realization id: %w", err)
		}
		realizations[index] = realization
	}
	if err := allocateCashStockMergerCash(realizations, planned.Links, params.CashValue, params.CashScale); err != nil {
		return CashStockMergerPlan{}, err
	}
	// Then the exchange moves each lot's units with the basis left.
	plan := CashStockMergerPlan{SourceQuantityValue: planned.SourceQuantityValue, SourceQuantityScale: planned.SourceQuantityScale,
		DestinationQuantityValue: planned.DestinationQuantityValue, DestinationQuantityScale: planned.DestinationQuantityScale}
	depletions := make([]LotDisposalRecord, len(planned.Links))
	for index, link := range planned.Links {
		var depletion LotDisposalRecord
		if planned.Replayed {
			depletion = planned.Depletions[index]
			if depletion.EventID, err = insertShareExchangeDepletionTx(ctx, tx, exchange, transaction.ID, journal,
				auditEventID, depletion); err != nil {
				return CashStockMergerPlan{}, err
			}
		} else if depletion, err = disposeLotTx(ctx, tx, depletionParams, link.SourceLotID, link.SourceQuantityValue,
			link.SourceQuantityScale, auditEventID, allocationScale); err != nil {
			return CashStockMergerPlan{}, err
		}
		if err := linkLotEffectTx(ctx, tx, operationID, depletion.EventID); err != nil {
			return CashStockMergerPlan{}, err
		}
		depletions[index] = depletion
	}
	for index, planLink := range planned.Links {
		link, depletion := planLink, depletions[index]
		link.BasisKnowledge = normalizedBasisKnowledge(depletion.BasisKnowledge)
		link.CarriedBasisValue, link.CarriedBasisScale = depletion.CostBasisValue, depletion.CostBasisScale
		if link.BasisKnowledge == InvestmentBasisUnknown {
			link.CarriedBasisValue, link.CarriedBasisScale = 0, 0
		}
		destination, err := createLotWithAuditTx(ctx, tx, CreateInvestmentLotParams{
			BookID: exchange.BookID, AccountID: exchange.DestinationAccountID, CommodityID: exchange.DestinationCommodityID,
			OpenedOn: exchange.EffectiveOn, SourceTransactionID: transaction.ID,
			QuantityValue: link.DestinationQuantityValue, QuantityScale: link.DestinationQuantityScale,
			CostBasisValue: link.CarriedBasisValue, CostBasisScale: link.CarriedBasisScale, CostCommodityID: link.CostCommodityID,
			OpeningBasisKnowledge: link.BasisKnowledge, MetadataJSON: `{"source":"cash_and_stock_merger"}`,
			EventKind: "transfer_in", CreatedAt: journal.CreatedAt, CreatedByUserID: journal.ActorUserID,
		}, auditEventID, planned.DestinationBackdated)
		if err != nil {
			return CashStockMergerPlan{}, err
		}
		link.DestinationLotID = destination.ID
		if _, err := tx.ExecContext(ctx, `INSERT INTO investment_transfer_lot_links
			(operation_id, link_seq, source_lot_id, destination_lot_id, quantity_value, quantity_scale,
			 basis_knowledge, carried_basis_value, carried_basis_scale, cost_commodity_id,
			 original_date_knowledge, original_acquired_on, source_evidence_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)`,
			operationID, index+1, link.SourceLotID, destination.ID, link.SourceQuantityValue, link.SourceQuantityScale,
			link.BasisKnowledge, nullableBasisValue(link.CarriedBasisValue, link.BasisKnowledge),
			nullableBasisScale(link.CarriedBasisScale, link.BasisKnowledge), link.CostCommodityID,
			link.OriginalDateKnowledge, link.OriginalAcquiredOn, exchange.SourceEvidenceJSON); err != nil {
			return CashStockMergerPlan{}, fmt.Errorf("link cash-and-stock merger lots: %w", err)
		}
		realization := realizations[index]
		plan.Links = append(plan.Links, CashStockMergerLink{ShareExchangeLink: link,
			RealizedBasisValue: realization.CostBasisValue, RealizedBasisScale: realization.CostBasisScale,
			CashValue: realization.ProceedsValue, CashScale: realization.ProceedsScale})
	}
	// The cash decision: quantity 0, the cash as proceeds, one allocation per
	// lot. The action fixes every lot, so it records specific_lot at the
	// transaction tier and never touches the method lock.
	decisionParams := depletionParams
	decisionParams.EventKind, decisionParams.QuantityValue, decisionParams.QuantityScale = "disposal", exact.New(0), 0
	decisionParams.ProceedsValue, decisionParams.ProceedsScale = params.CashValue, params.CashScale
	decisionParams.CostBasisMethod = "specific_lot"
	decisionParams.DecisionSource = DisposalDecisionSource{ResolutionTier: "transaction"}
	if _, err := createDisposalDecisionTx(ctx, tx, transaction, decisionParams, realizations, auditEventID); err != nil {
		return CashStockMergerPlan{}, err
	}
	if !planned.Replayed {
		if err := keepPositionMethodFamilyTx(ctx, tx, depletionParams, auditEventID); err != nil {
			return CashStockMergerPlan{}, err
		}
	}
	// The old holding first, so a dependency the emptied holding breaks is
	// named before one the new lots change.
	var replayed [][2]int64
	if planned.Replayed {
		replayed = append(replayed, [2]int64{exchange.AccountID, exchange.CommodityID})
	}
	if planned.DestinationBackdated {
		replayed = append(replayed, [2]int64{exchange.DestinationAccountID, exchange.DestinationCommodityID})
	}
	if err := replayHoldingsTx(ctx, tx, exchange.BookID, replayed, operationID, auditEventID,
		journal.ActorUserID, journal.CreatedAt); err != nil {
		return CashStockMergerPlan{}, err
	}
	if err := requirePositionBasisRangeQueryTx(ctx, tx, exchange.BookID, exchange.AccountID,
		exchange.CommodityID, params.CashCommodityID, true, PositionSideLong); err != nil {
		return CashStockMergerPlan{}, err
	}
	return plan, nil
}

// realizeCashStockMergerHoldingTx is the subject merger's cash step in replay
// (#184): it realizes the elected share of every long lot of the position
// open at the slot, in the order takeShareExchangeHoldingTx then takes them.
func realizeCashStockMergerHoldingTx(ctx context.Context, tx *sql.Tx, bookID, accountID, commodityID, costCommodityID int64,
	intent InvestmentReplayIntent, allocationScale int) ([]LotDisposalRecord, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, remaining_quantity_value, remaining_quantity_scale
		FROM current_investment_lots
		WHERE book_id = ? AND account_id = ? AND commodity_id = ? AND cost_commodity_id = ?
			AND position_side = 'long' AND status = 'open' AND opened_on <= ?
		ORDER BY opened_on, id`, bookID, accountID, commodityID, costCommodityID, intent.EventDate)
	if err != nil {
		return nil, fmt.Errorf("read cash-and-stock merger holding: %w", err)
	}
	var lots []LotAllocation
	for rows.Next() {
		var lot LotAllocation
		if err := rows.Scan(&lot.LotID, &lot.QuantityValue, &lot.QuantityScale); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan cash-and-stock merger holding: %w", err)
		}
		if lot.QuantityValue.Sign() > 0 {
			lots = append(lots, lot)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("read cash-and-stock merger holding: %w", err)
	}
	realized := make([]LotDisposalRecord, 0, len(lots))
	for _, lot := range lots {
		realization, err := realizeCashStockMergerLotTx(ctx, tx, bookID, accountID, commodityID, costCommodityID, lot.LotID,
			intent.EventDate, lot.QuantityValue, lot.QuantityScale, intent.BasisFractionValue, intent.BasisFractionScale,
			allocationScale, intent.CreatedAt, intent.CreatedByUserID, intent.AuditEventID)
		if err != nil {
			return nil, err
		}
		realized = append(realized, realization)
	}
	return realized, nil
}

// realizeCashStockMergerLotTx is the merger's cash step for one lot, shared by
// the writer and replay: the lot must be open, long and on or before the slot
// with exactly the link's units. It recovers remaining basis × fraction
// (truncated at the allocation scale) from the projection and returns it as a
// quantity-0 disposal; an unknown lot stays unknown and recovers no amount.
func realizeCashStockMergerLotTx(ctx context.Context, tx *sql.Tx, bookID, accountID, commodityID, costCommodityID, lotID int64,
	effectiveOn string, quantityValue exact.Coefficient, quantityScale int, fractionValue exact.Coefficient,
	fractionScale, allocationScale int, at string, actorUserID, auditEventID int64) (LotDisposalRecord, error) {
	var heldValue exact.Coefficient
	var heldScale int
	var basisValue, basisScale sql.NullInt64
	var knowledge string
	err := tx.QueryRowContext(ctx, `SELECT remaining_quantity_value, remaining_quantity_scale,
			remaining_cost_basis_value, remaining_cost_basis_scale, basis_knowledge
		FROM current_investment_lots
		WHERE id = ? AND book_id = ? AND account_id = ? AND commodity_id = ? AND cost_commodity_id = ?
			AND position_side = 'long' AND status = 'open' AND opened_on <= ?`,
		lotID, bookID, accountID, commodityID, costCommodityID, effectiveOn).Scan(&heldValue, &heldScale,
		&basisValue, &basisScale, &knowledge)
	if errors.Is(err, sql.ErrNoRows) {
		return LotDisposalRecord{}, ErrCashStockMergerLotChanged
	}
	if err != nil {
		return LotDisposalRecord{}, fmt.Errorf("read cash-and-stock merger lot: %w", err)
	}
	if exact.ScaledIntFromCoefficient(heldValue, heldScale).Cmp(exact.ScaledIntFromCoefficient(quantityValue, quantityScale)) != 0 {
		return LotDisposalRecord{}, ErrCashStockMergerLotChanged
	}
	record := LotDisposalRecord{LotID: lotID, QuantityValue: exact.New(0), CostCommodityID: costCommodityID,
		BasisKnowledge: normalizedBasisKnowledge(knowledge)}
	if record.BasisKnowledge == InvestmentBasisUnknown {
		return record, nil
	}
	if !basisValue.Valid || !basisScale.Valid {
		return LotDisposalRecord{}, fmt.Errorf("%w: lot %d has a known basis without an amount", ErrInvalidDisposalParams, lotID)
	}
	realized, remaining, err := spinOffAllocation(basisValue.Int64, int(basisScale.Int64), fractionValue, fractionScale, allocationScale)
	if err != nil {
		return LotDisposalRecord{}, err
	}
	record.CostBasisValue, _ = realized.Int64()
	record.CostBasisScale = realized.Scale()
	remainingValue, _ := remaining.Int64()
	if _, err := tx.ExecContext(ctx, `UPDATE investment_lot_state
		SET remaining_cost_basis_value = ?, remaining_cost_basis_scale = ?,
			updated_at = ?, updated_by_user_id = ?, updated_audit_event_id = NULLIF(?, 0)
		WHERE lot_id = ? AND book_id = ?`, remainingValue, remaining.Scale(),
		at, actorUserID, auditEventID, lotID, bookID); err != nil {
		return LotDisposalRecord{}, fmt.Errorf("realize cash-and-stock merger lot %d basis: %w", lotID, err)
	}
	return record, nil
}

// allocateCashStockMergerCash shares the cash across the realizations by the
// old units each lot surrendered, truncating every share but the last, as
// sale proceeds are allocated.
func allocateCashStockMergerCash(realizations []LotDisposalRecord, links []ShareExchangeLink, value int64, scale int) error {
	weighted := make([]LotDisposalRecord, len(links))
	for index, link := range links {
		weighted[index] = LotDisposalRecord{LotID: link.SourceLotID, QuantityValue: link.SourceQuantityValue,
			QuantityScale: link.SourceQuantityScale, CostBasisValue: realizations[index].CostBasisValue,
			CostBasisScale: realizations[index].CostBasisScale}
	}
	if err := allocateDisposalProceeds(weighted, value, scale); err != nil {
		return err
	}
	for index := range realizations {
		realizations[index].ProceedsValue, realizations[index].ProceedsScale = weighted[index].ProceedsValue, weighted[index].ProceedsScale
	}
	return nil
}

// cashStockMergerReplayDisposal is a merger's cash decision as replay
// produced it: the realizations, with the fixed cash shared by link units
// exactly as the writer shared it.
func cashStockMergerReplayDisposal(intent InvestmentReplayIntent, realizations []LotDisposalRecord) (InvestmentReplayDisposal, error) {
	links := make([]ShareExchangeLink, len(intent.ExchangeLinks))
	for index, link := range intent.ExchangeLinks {
		links[index] = ShareExchangeLink{SourceLotID: link.LotID, SourceQuantityValue: link.QuantityValue,
			SourceQuantityScale: link.QuantityScale}
	}
	cash, err := exact.ScaledIntFromCoefficient(intent.MergerCashValue, intent.MergerCashScale).Int64()
	if err != nil {
		return InvestmentReplayDisposal{}, ErrInvestmentBasisRange
	}
	if err := allocateCashStockMergerCash(realizations, links, cash, intent.MergerCashScale); err != nil {
		return InvestmentReplayDisposal{}, err
	}
	disposal := InvestmentReplayDisposal{DecisionID: intent.MergerDecisionID}
	for _, realization := range realizations {
		disposal.Allocations = append(disposal.Allocations, InvestmentReplayAllocation{LotID: realization.LotID,
			QuantityValue: exact.New(0), CostBasisValue: realization.CostBasisValue, CostBasisScale: realization.CostBasisScale,
			ProceedsValue: realization.ProceedsValue, ProceedsScale: realization.ProceedsScale,
			BasisKnowledge: realization.BasisKnowledge})
	}
	return disposal, nil
}
