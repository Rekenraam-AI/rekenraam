package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Dividend attribution does not assert historical share entitlement. Only a
// retained provider claim supplies a comparison: unknown owner payer is
// unavailable support, a different known payer is a warning. Neither gates cash.
func dividendAttributions(ctx context.Context, source queryer, bookID, transactionID int64) ([]InvestmentAttributionDiagnostic, error) {
	rows, err := source.QueryContext(ctx, `SELECT DISTINCT v.transaction_id, f.commodity_id IS NULL,
		EXISTS (SELECT 1 FROM investment_cash_payment_identity previous
		 JOIN investment_event_suggestions claim ON claim.book_id=previous.book_id
		   AND (claim.generated_transaction_id=previous.transaction_id OR claim.linked_transaction_id=previous.transaction_id)
		 JOIN investment_provider_events event ON event.id=claim.provider_event_id AND event.book_id=claim.book_id
		 JOIN investment_instruments instrument ON instrument.id=event.instrument_id AND instrument.book_id=event.book_id
		 WHERE previous.book_id=identity.book_id AND previous.root_operation_id=identity.root_operation_id
		   AND f.commodity_id IS NOT NULL AND instrument.commodity_id<>f.commodity_id)
	 FROM effective_investment_operations o
	 JOIN valid_investment_dividend_facts f ON f.operation_id=o.id AND f.book_id=o.book_id
	 JOIN investment_operation_journal_links l ON l.operation_id=o.id AND l.book_id=o.book_id AND l.role='primary'
	 JOIN transaction_versions v ON v.id=l.transaction_version_id AND v.book_id=o.book_id
	 JOIN investment_cash_payment_identity identity ON identity.operation_id=o.id AND identity.book_id=o.book_id AND identity.transaction_id=v.transaction_id
	 WHERE o.book_id=? AND o.operation_kind='dividend' AND (?=0 OR v.transaction_id=?)
	 AND EXISTS (SELECT 1 FROM investment_cash_payment_identity previous
	   JOIN investment_event_suggestions claim ON claim.book_id=previous.book_id
	     AND (claim.generated_transaction_id=previous.transaction_id OR claim.linked_transaction_id=previous.transaction_id)
	   WHERE previous.book_id=identity.book_id AND previous.root_operation_id=identity.root_operation_id)
	 ORDER BY v.transaction_id`, bookID, transactionID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("read dividend source attribution: %w", err)
	}
	results := []InvestmentAttributionDiagnostic{}
	for rows.Next() {
		record := InvestmentAttributionDiagnostic{WarningReasons: []string{}}
		var mismatch bool
		if err := rows.Scan(&record.TransactionID, &record.Unavailable, &mismatch); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan dividend source attribution: %w", err)
		}
		if mismatch {
			record.WarningReasons = append(record.WarningReasons, AttributionProviderMismatch)
		}
		results = append(results, record)
	}
	return results, errors.Join(rows.Err(), rows.Close())
}

func (r *SelfCheckRepository) DividendAttributions(ctx context.Context, snapshot *sql.Tx, bookID int64) ([]InvestmentAttributionDiagnostic, error) {
	return dividendAttributions(ctx, snapshot, bookID, 0)
}

func (r *InvestmentRepository) DividendAttribution(ctx context.Context, bookID, transactionID int64) (InvestmentAttributionDiagnostic, error) {
	records, err := dividendAttributions(ctx, r.database, bookID, transactionID)
	if err != nil {
		return InvestmentAttributionDiagnostic{}, err
	}
	if len(records) == 0 {
		return InvestmentAttributionDiagnostic{TransactionID: transactionID, WarningReasons: []string{}}, nil
	}
	return records[0], nil
}
