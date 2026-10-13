package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"rekenraam/backend/internal/exact"
)

// SplitActivityRecord composes source, standing and current journal delta for
// a split, including ones recorded before the first entitled lot existed.
type SplitActivityRecord struct {
	OperationID             int64
	TransactionID           int64
	CorrectionOfOperationID int64
	SuccessorOperationID    int64
	AccountID               int64
	CommodityID             int64
	AccountName             string
	InstrumentName          string
	EffectiveOn             string
	RatioNumerator          int64
	RatioDenominator        int64
	SourceEvidenceJSON      string
	Memo                    string
	Reason                  string
	CreatedAt               string
	Standing                string
	Settled                 bool
	JournalDeltaValue       exact.Coefficient
	JournalDeltaScale       int
}

// SplitActivity reads one bounded page in a single composed query. Journal
// quantities are exact strings summed in Go, never SQLite numeric aggregates.
func (r *InvestmentRepository) SplitActivity(ctx context.Context, bookID, beforeID int64, limit int, operationID int64) ([]SplitActivityRecord, error) {
	rows, err := r.database.QueryContext(ctx, `WITH page AS (
 SELECT o.id FROM investment_operations o
 WHERE o.book_id = ? AND o.operation_kind = 'split' AND (? = 0 OR o.id < ?) AND (? = 0 OR o.id = ?)
 ORDER BY o.id DESC LIMIT ?)
 SELECT o.id, COALESCE(v.transaction_id,0), COALESCE(o.correction_of_operation_id,0), COALESCE(successor.id,0),
 f.account_id, f.commodity_id, av.name, COALESCE(iv.display_name,c.code),
 f.effective_on,f.ratio_numerator,f.ratio_denominator,f.source_evidence_json,
 COALESCE(v.description,json_extract(a.metadata_json,'$.memo'),''), COALESCE(o.correction_reason,a.reason,''),o.created_at,
 CASE WHEN successor.correction_mode = 'reverse' THEN 'reversed' WHEN successor.id IS NOT NULL THEN 'superseded' ELSE 'effective' END,
 EXISTS(SELECT 1 FROM investment_cash_in_lieu_facts cash JOIN effective_investment_operations active ON active.id = cash.operation_id
 WHERE cash.settled_operation_id = o.id), pv.quantity_value,pv.quantity_scale
 FROM page JOIN investment_operations o ON o.id = page.id
 JOIN investment_split_facts f ON f.operation_id = o.id
 JOIN audit_events a ON a.id = o.created_audit_event_id
 JOIN current_account_versions av ON av.account_id = f.account_id
 JOIN commodities c ON c.id = f.commodity_id
 LEFT JOIN investment_instruments i ON i.commodity_id = f.commodity_id
 LEFT JOIN current_investment_instrument_versions iv ON iv.instrument_id = i.id
 LEFT JOIN investment_operations successor ON successor.correction_of_operation_id = o.id
 LEFT JOIN investment_operation_journal_links primary_link ON primary_link.operation_id = o.id AND primary_link.role = 'primary'
 LEFT JOIN transaction_versions v ON v.id = primary_link.transaction_version_id
 LEFT JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.role IN ('primary','split_adjustment')
 LEFT JOIN posting_versions pv ON pv.transaction_version_id = link.transaction_version_id AND pv.account_id = f.account_id AND pv.commodity_id = f.commodity_id
 ORDER BY o.id DESC,link.link_seq`, bookID, beforeID, beforeID, operationID, operationID, limit)
	if err != nil {
		return nil, fmt.Errorf("read split activity: %w", err)
	}
	defer rows.Close()
	records := make([]SplitActivityRecord, 0)
	var deltas []*exact.ScaledInt
	for rows.Next() {
		var record SplitActivityRecord
		var value sql.NullString
		var scale sql.NullInt64
		if err := rows.Scan(&record.OperationID, &record.TransactionID, &record.CorrectionOfOperationID, &record.SuccessorOperationID,
			&record.AccountID, &record.CommodityID, &record.AccountName, &record.InstrumentName, &record.EffectiveOn, &record.RatioNumerator,
			&record.RatioDenominator, &record.SourceEvidenceJSON, &record.Memo, &record.Reason, &record.CreatedAt, &record.Standing, &record.Settled, &value, &scale); err != nil {
			return nil, err
		}
		if len(records) == 0 || records[len(records)-1].OperationID != record.OperationID {
			records = append(records, record)
			deltas = append(deltas, exact.NewScaledInt())
		}
		if value.Valid {
			coefficient, err := exact.Parse(value.String)
			if err != nil {
				return nil, err
			}
			deltas[len(deltas)-1].AddCoefficient(coefficient, int(scale.Int64))
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	// Range-check complete per-operation totals, never a partial sum of
	// offsetting journals (for example, adjustments across cost currencies).
	for index, delta := range deltas {
		normalized := delta.Normalized()
		coefficient, err := normalized.Coefficient()
		if err != nil {
			return nil, err
		}
		records[index].JournalDeltaValue = coefficient
		records[index].JournalDeltaScale = normalized.Scale()
	}
	return records, nil
}
