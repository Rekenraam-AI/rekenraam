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
}

type returnsOperation struct {
	kind         string
	correctionOf sql.NullInt64
	holding      sql.NullInt64
	instrument   sql.NullInt64
	hasFact      bool
	exDate       sql.NullString
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
		{`SELECT operation_id, account_id, NULL, NULL FROM investment_capital_return_facts WHERE book_id = ?`,
			func(o *returnsOperation, holding, _ sql.NullInt64, _ sql.NullString) { o.holding = holding }},
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
		SELECT tv.transaction_id, l.operation_id
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
		if err := rows.Scan(&anchor.TransactionID, &anchor.OperationID); err != nil {
			return nil, fmt.Errorf("scan returns operation link: %w", err)
		}
		operation := operations[anchor.OperationID]
		if operation == nil {
			continue
		}
		anchor.OperationKind = operation.kind
		// A reversal or replacement may carry no facts of its own; its
		// predecessor's facts describe the same position.
		source := operation
		for depth := 0; source != nil && !source.hasFact && !source.holding.Valid && source.correctionOf.Valid && depth < 64; depth++ {
			source = operations[source.correctionOf.Int64]
		}
		if source != nil {
			anchor.HoldingAccountID, anchor.InstrumentFact, anchor.InstrumentID = source.holding, source.hasFact, source.instrument
			anchor.ExDate = source.exDate
		}
		if operation.exDate.Valid {
			anchor.ExDate = operation.exDate
		}
		anchors = append(anchors, anchor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate returns operation links: %w", err)
	}
	return anchors, nil
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
