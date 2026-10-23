package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const AttributionCashParentUnresolved = "cash_in_lieu_parent_unresolved"
const AttributionCashParentMismatch = "cash_in_lieu_parent_mismatch"

// A current explanation is advisory. Immutable original reference integrity
// and actual disposal conservation are checked independently.
func cashInLieuAttributions(ctx context.Context, source queryer, bookID, transactionID int64) ([]InvestmentAttributionDiagnostic, error) {
	rows, err := source.QueryContext(ctx, `SELECT v.transaction_id,
   CASE WHEN parent.effective_operation_id IS NULL THEN ?
     WHEN parent.account_id <> d.account_id OR parent.commodity_id <> d.commodity_id OR parent.effective_on > d.event_date THEN ? ELSE '' END
 FROM effective_investment_operations o
 JOIN valid_investment_cash_in_lieu_facts f ON f.operation_id = o.id AND f.book_id = o.book_id
 JOIN investment_disposal_decisions d ON d.operation_id = o.id
 JOIN investment_operation_journal_links l ON l.operation_id = o.id AND l.role = 'primary'
 JOIN transaction_versions v ON v.id = l.transaction_version_id
 LEFT JOIN cash_in_lieu_parent_context parent ON parent.book_id = f.book_id AND parent.original_operation_id = f.settled_operation_id
 WHERE o.book_id = ? AND o.operation_kind = 'cash_in_lieu' AND (? = 0 OR v.transaction_id = ?)
 ORDER BY o.id`, AttributionCashParentUnresolved, AttributionCashParentMismatch, bookID, transactionID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("read cash in lieu attribution: %w", err)
	}
	records := []InvestmentAttributionDiagnostic{}
	for rows.Next() {
		record := InvestmentAttributionDiagnostic{WarningReasons: []string{}}
		var reason string
		if err := rows.Scan(&record.TransactionID, &reason); err != nil {
			rows.Close()
			return nil, err
		}
		if reason != "" {
			record.WarningReasons = append(record.WarningReasons, reason)
		}
		records = append(records, record)
	}
	return records, errors.Join(rows.Err(), rows.Close())
}

func (r *SelfCheckRepository) CashInLieuAttributions(ctx context.Context, snapshot *sql.Tx, bookID int64) ([]InvestmentAttributionDiagnostic, error) {
	return cashInLieuAttributions(ctx, snapshot, bookID, 0)
}
func (r *InvestmentRepository) CashInLieuAttribution(ctx context.Context, bookID, transactionID int64) (InvestmentAttributionDiagnostic, error) {
	records, err := cashInLieuAttributions(ctx, r.database, bookID, transactionID)
	if err != nil {
		return InvestmentAttributionDiagnostic{}, err
	}
	if len(records) != 1 {
		return InvestmentAttributionDiagnostic{}, ErrNotFound
	}
	return records[0], nil
}
