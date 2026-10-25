package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ReturnsOperationAnchor is what R13 needs to know about the investment
// operation a transaction belongs to: which position its cash concerns
// (docs/plans/investment-returns-plan.md, "External flows"). Facts are read
// from the operation itself or, for a correction without facts of its own,
// from the nearest predecessor that has them.
type ReturnsOperationAnchor struct {
	TransactionID int64
	OperationID   int64
	OperationKind string
	// RootOperationID is the uncorrected original of the operation's
	// correction chain: an operation, its reversals and replacements share it.
	RootOperationID int64
	// SourceKind is the operation kind nearest in the chain that is not a
	// reversal, so a reversal of a transfer still reads as a transfer.
	SourceKind string
	// SubjectOperationID is the operation whose effect this journal carries:
	// its own, or for a reversal or the inverse inside a replacement, the
	// operation it undoes. A journal and its undoing share it, so they can be
	// cancelled as a pair while a replacement survives.
	SubjectOperationID int64
	// HoldingAccountID is the holding a capital-return or payment-in-lieu
	// fact names.
	HoldingAccountID sql.NullInt64
	// InstrumentFact is set when a dividend or payment-in-lieu fact exists;
	// InstrumentID is then the instrument it names, or null when unassigned.
	InstrumentFact bool
	InstrumentID   sql.NullInt64
	// ExDate is the operation's recorded ex date, or the payment-in-lieu
	// fact's, when there is one.
	ExDate sql.NullString
	// HoldingCommodityID is the instrument a capital-return fact names.
	HoldingCommodityID sql.NullInt64
	// Exchange is the share-exchange, cash-and-stock-merger or spin-off fact
	// of the operation or, for a reversal, of the operation it corrects.
	Exchange *ReturnsExchangeFact
}

// ReturnsExchangeFact names what an exchange-family operation gave up and
// received: the source instrument and holding, and the new instrument.
type ReturnsExchangeFact struct {
	SourceCommodityID      int64
	SourceAccountID        sql.NullInt64
	DestinationCommodityID int64
}

type returnsOperation struct {
	kind         string
	correctionOf sql.NullInt64
	holding      sql.NullInt64
	instrument   sql.NullInt64
	hasFact      bool
	exDate       sql.NullString
	// holdingCommodity is a capital-return fact's instrument.
	holdingCommodity sql.NullInt64
	exchange         *ReturnsExchangeFact
}

// ReturnsOperationAnchors returns one anchor per current posted transaction
// linked to an investment operation. It reads facts only; it never replays.
func (r *TransactionRepository) ReturnsOperationAnchors(ctx context.Context, bookID int64) ([]ReturnsOperationAnchor, error) {
	operations := map[int64]*returnsOperation{}
	rows, err := r.database.QueryContext(ctx, `
		SELECT o.id, o.operation_kind, o.correction_of_operation_id,
			(SELECT d.event_date FROM investment_operation_dates d
				WHERE d.operation_id = o.id AND d.date_role = 'ex' ORDER BY d.date_seq LIMIT 1)
		FROM investment_operations o WHERE o.book_id = ?`, bookID)
	if err != nil {
		return nil, fmt.Errorf("read returns operations: %w", err)
	}
	for rows.Next() {
		var id int64
		operation := &returnsOperation{}
		if err := rows.Scan(&id, &operation.kind, &operation.correctionOf, &operation.exDate); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan returns operation: %w", err)
		}
		operations[id] = operation
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate returns operations: %w", err)
	}

	facts := []struct {
		query string
		apply func(*returnsOperation, sql.NullInt64, sql.NullInt64, sql.NullString)
	}{
		{`SELECT operation_id, account_id, commodity_id, NULL FROM investment_capital_return_facts WHERE book_id = ?`,
			func(o *returnsOperation, holding, commodity sql.NullInt64, _ sql.NullString) {
				o.holding, o.holdingCommodity = holding, commodity
			}},
		{`SELECT operation_id, holding_account_id, commodity_id, ex_date FROM valid_investment_payment_in_lieu_facts WHERE book_id = ?`,
			func(o *returnsOperation, holding, instrument sql.NullInt64, exDate sql.NullString) {
				o.holding, o.instrument, o.hasFact = holding, instrument, true
				if exDate.Valid {
					o.exDate = exDate
				}
			}},
		{`SELECT operation_id, NULL, commodity_id, NULL FROM valid_investment_dividend_facts WHERE book_id = ?`,
			func(o *returnsOperation, _, instrument sql.NullInt64, _ sql.NullString) {
				o.instrument, o.hasFact = instrument, true
			}},
	}
	for _, fact := range facts {
		rows, err := r.database.QueryContext(ctx, fact.query, bookID)
		if err != nil {
			return nil, fmt.Errorf("read returns facts: %w", err)
		}
		for rows.Next() {
			var operationID int64
			var holding, instrument sql.NullInt64
			var exDate sql.NullString
			if err := rows.Scan(&operationID, &holding, &instrument, &exDate); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan returns fact: %w", err)
			}
			if operation := operations[operationID]; operation != nil {
				fact.apply(operation, holding, instrument, exDate)
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, fmt.Errorf("iterate returns facts: %w", err)
		}
	}

	rows, err = r.database.QueryContext(ctx, `
		SELECT operation_id, commodity_id, source_account_id, destination_commodity_id
		FROM investment_transfer_facts
		WHERE book_id = ? AND transfer_kind IN ('exchange', 'spin_off') AND destination_commodity_id IS NOT NULL`, bookID)
	if err != nil {
		return nil, fmt.Errorf("read returns exchange facts: %w", err)
	}
	for rows.Next() {
		var operationID int64
		var fact ReturnsExchangeFact
		if err := rows.Scan(&operationID, &fact.SourceCommodityID, &fact.SourceAccountID, &fact.DestinationCommodityID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan returns exchange fact: %w", err)
		}
		if operation := operations[operationID]; operation != nil {
			operation.exchange = &fact
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate returns exchange facts: %w", err)
	}

	rows, err = r.database.QueryContext(ctx, `
		SELECT tv.transaction_id, l.operation_id, l.role
		FROM investment_operation_journal_links l
		JOIN current_transaction_versions tv ON tv.id = l.transaction_version_id
		JOIN transactions t ON t.id = tv.transaction_id
		WHERE l.book_id = ? AND tv.status = 'posted' AND t.deleted_at IS NULL
		ORDER BY tv.transaction_id`, bookID)
	if err != nil {
		return nil, fmt.Errorf("read returns operation links: %w", err)
	}
	defer rows.Close()
	var anchors []ReturnsOperationAnchor
	for rows.Next() {
		var anchor ReturnsOperationAnchor
		var role string
		if err := rows.Scan(&anchor.TransactionID, &anchor.OperationID, &role); err != nil {
			return nil, fmt.Errorf("scan returns operation link: %w", err)
		}
		operation := operations[anchor.OperationID]
		if operation == nil {
			continue
		}
		anchor.OperationKind = operation.kind
		anchor.RootOperationID, anchor.SourceKind = anchor.OperationID, ""
		for id, current, depth := anchor.OperationID, operation, 0; current != nil && depth < 64; depth++ {
			if anchor.SourceKind == "" && current.kind != "reversal" {
				anchor.SourceKind = current.kind
			}
			anchor.RootOperationID = id
			if !current.correctionOf.Valid {
				break
			}
			id = current.correctionOf.Int64
			current = operations[id]
		}
		// The inverse journal inside a replacement undoes the operation it
		// corrects, so it is read with that operation's facts: a replacement
		// that changes the ex date or payer must not move the inverse with it.
		facts := operation
		anchor.SubjectOperationID = anchor.OperationID
		if role == "reversal" && operation.correctionOf.Valid && operations[operation.correctionOf.Int64] != nil {
			facts = operations[operation.correctionOf.Int64]
			anchor.SubjectOperationID = operation.correctionOf.Int64
		} else if operation.kind == "reversal" && operation.correctionOf.Valid {
			anchor.SubjectOperationID = operation.correctionOf.Int64
		}
		// A reversal or replacement may carry no facts of its own; its
		// predecessor's facts describe the same position.
		source := facts
		for depth := 0; source != nil && !source.hasFact && !source.holding.Valid && source.correctionOf.Valid && depth < 64; depth++ {
			source = operations[source.correctionOf.Int64]
		}
		if source != nil {
			anchor.HoldingAccountID, anchor.InstrumentFact, anchor.InstrumentID = source.holding, source.hasFact, source.instrument
			anchor.ExDate, anchor.HoldingCommodityID = source.exDate, source.holdingCommodity
		}
		exchange := facts
		for depth := 0; exchange != nil && exchange.exchange == nil && exchange.correctionOf.Valid && depth < 64; depth++ {
			exchange = operations[exchange.correctionOf.Int64]
		}
		if exchange != nil {
			anchor.Exchange = exchange.exchange
		}
		if facts.exDate.Valid {
			anchor.ExDate = facts.exDate
		}
		anchors = append(anchors, anchor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate returns operation links: %w", err)
	}
	return anchors, nil
}

// ReturnsCommodityCodes maps every commodity of the book to its code, so
// per-instrument rows have a stable, readable order.
func (r *TransactionRepository) ReturnsCommodityCodes(ctx context.Context, bookID int64) (map[int64]string, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT id, code FROM commodities WHERE book_id = ?`, bookID)
	if err != nil {
		return nil, fmt.Errorf("read returns commodity codes: %w", err)
	}
	defer rows.Close()
	codes := map[int64]string{}
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return nil, fmt.Errorf("scan returns commodity code: %w", err)
		}
		codes[id] = code
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate returns commodity codes: %w", err)
	}
	return codes, nil
}

// AccountBaseKinds maps every account kind code to its base kind.
func (r *TransactionRepository) AccountBaseKinds(ctx context.Context) (map[string]string, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT code, base_kind FROM account_kinds`)
	if err != nil {
		return nil, fmt.Errorf("read account base kinds: %w", err)
	}
	defer rows.Close()
	kinds := map[string]string{}
	for rows.Next() {
		var code, base string
		if err := rows.Scan(&code, &base); err != nil {
			return nil, fmt.Errorf("scan account base kind: %w", err)
		}
		kinds[code] = base
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate account base kinds: %w", err)
	}
	return kinds, nil
}

// ReturnsInstrumentClass is how R13 allocation groups a commodity's units:
// the instrument version's type and quote currency effective on a date.
type ReturnsInstrumentClass struct {
	InstrumentType   string
	QuoteCommodityID sql.NullInt64
}

// ReturnsInstrumentClassesAsOf returns, per instrument commodity, the version
// effective on asOf. A commodity with no version effective that day is absent:
// its classification on that date is unknown, not borrowed from a later one.
func (r *TransactionRepository) ReturnsInstrumentClassesAsOf(ctx context.Context, bookID int64, asOf string) (map[int64]ReturnsInstrumentClass, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT i.commodity_id, v.instrument_type, v.quote_commodity_id
		FROM investment_instruments i
		JOIN investment_instrument_versions v ON v.id = (
			SELECT asof_v.id
			FROM investment_instrument_versions asof_v
			WHERE asof_v.instrument_id = i.id AND asof_v.effective_from <= ?
			ORDER BY asof_v.effective_from DESC, asof_v.version_seq DESC
			LIMIT 1
		)
		WHERE i.book_id = ?
	`, asOf, bookID)
	if err != nil {
		return nil, fmt.Errorf("read returns instrument classes: %w", err)
	}
	defer rows.Close()
	classes := map[int64]ReturnsInstrumentClass{}
	for rows.Next() {
		var commodityID int64
		var class ReturnsInstrumentClass
		if err := rows.Scan(&commodityID, &class.InstrumentType, &class.QuoteCommodityID); err != nil {
			return nil, fmt.Errorf("scan returns instrument class: %w", err)
		}
		classes[commodityID] = class
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate returns instrument classes: %w", err)
	}
	return classes, nil
}
