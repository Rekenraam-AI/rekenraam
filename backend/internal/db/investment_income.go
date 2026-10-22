package db

import (
	"context"
	"errors"
	"fmt"

	"rekenraam/backend/internal/exact"
)

// InvestmentIncomeRecord is one effective dividend's gross income in one
// currency: the negated sum of its income-class postings (#231).
type InvestmentIncomeRecord struct {
	OperationID int64
	CommodityID int64
	EventDate   string
	Value       exact.Coefficient
	Scale       int
}

// InvestmentIncome lists the gross income of every effective cash and
// reinvested dividend dated from..to inclusive. A reversed or replaced
// dividend is not effective, so only its replacement counts. Withholding
// posts to its own account and does not reduce the gross figure.
func (r *InvestmentRepository) InvestmentIncome(ctx context.Context, bookID int64, from, to string) ([]InvestmentIncomeRecord, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT o.id, o.event_date, pv.commodity_id, pv.quantity_value, pv.quantity_scale
		FROM effective_investment_operations o
		JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id
		JOIN transaction_versions linked ON linked.id = link.transaction_version_id
		JOIN current_transaction_versions tv ON tv.transaction_id = linked.transaction_id
		JOIN posting_versions pv ON pv.transaction_version_id = tv.id
		JOIN `+CurrentReferenceVersions(ctx, "account")+` av ON av.account_id = pv.account_id
		WHERE o.book_id = ? AND o.operation_kind IN ('dividend', 'reinvested_dividend')
			AND o.event_date BETWEEN ? AND ?
			AND tv.status = 'posted' AND av.account_class = 'income'
		ORDER BY o.event_date, o.id, pv.commodity_id, pv.id`, bookID, from, to)
	if err != nil {
		return nil, fmt.Errorf("read investment income: %w", err)
	}
	type key struct{ operation, commodity int64 }
	sums := map[key]*exact.ScaledInt{}
	var records []InvestmentIncomeRecord
	for rows.Next() {
		var record InvestmentIncomeRecord
		if err := rows.Scan(&record.OperationID, &record.EventDate, &record.CommodityID, &record.Value, &record.Scale); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan investment income: %w", err)
		}
		k := key{record.OperationID, record.CommodityID}
		if sums[k] == nil {
			sums[k] = exact.NewScaledInt()
			records = append(records, InvestmentIncomeRecord{OperationID: record.OperationID, CommodityID: record.CommodityID, EventDate: record.EventDate})
		}
		// Income is credited: a negative posting is positive income.
		sums[k].AddCoefficient(record.Value.Negated(), record.Scale)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate investment income: %w", err)
	}
	for i := range records {
		sum := sums[key{records[i].OperationID, records[i].CommodityID}]
		value, err := sum.Coefficient()
		if err != nil {
			return nil, fmt.Errorf("investment income %d: %w", records[i].OperationID, err)
		}
		records[i].Value, records[i].Scale = value, sum.Scale()
	}
	return records, nil
}
