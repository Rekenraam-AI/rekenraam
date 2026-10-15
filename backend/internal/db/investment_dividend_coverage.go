package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"rekenraam/backend/internal/exact"
)

// DividendCoverageOperation is one effective, posted cash dividend or separate
// withholding payment (#244). Its gross and withholding are owner-confirmed
// components of a named operation; Payer is absent when no instrument fact was
// recorded. Withholding is a positive magnitude.
type DividendCoverageOperation struct {
	OperationID     int64
	TransactionID   int64
	EventDate       string
	Payer           sql.NullInt64
	WithholdingOnly bool
	CashCommodityID int64
	CashCode        string
	Gross           *exact.ScaledInt
	Withholding     *exact.ScaledInt
}

// DividendCoverageOperations lists every effective posted named cash dividend.
// Reversed or replaced operations are not effective; only their replacement is.
func (r *InvestmentRepository) DividendCoverageOperations(ctx context.Context, bookID int64) ([]DividendCoverageOperation, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT o.id, tv.transaction_id, o.event_date, facts.commodity_id, COALESCE(facts.withholding_only, 0),
			c.component_kind, c.commodity_id, cash.code, c.amount_value, c.amount_scale
		FROM effective_investment_operations o
		JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
		JOIN transaction_versions linked ON linked.id = link.transaction_version_id
		JOIN transactions t ON t.id = linked.transaction_id
		JOIN current_transaction_versions tv ON tv.transaction_id = t.id
		JOIN investment_operation_components c ON c.operation_id = o.id AND c.component_kind IN ('dividend_gross', 'withholding')
		JOIN commodities cash ON cash.id = c.commodity_id
		LEFT JOIN investment_dividend_facts facts ON facts.operation_id = o.id
		WHERE o.book_id = ? AND o.operation_kind = 'dividend' AND tv.status = 'posted' AND t.deleted_at IS NULL
		ORDER BY o.event_date, o.id, c.component_seq`, bookID)
	if err != nil {
		return nil, fmt.Errorf("read dividend coverage operations: %w", err)
	}
	var result []DividendCoverageOperation
	for rows.Next() {
		var op DividendCoverageOperation
		var kind string
		var componentCommodity int64
		var code string
		var value exact.Coefficient
		var scale int
		if err := rows.Scan(&op.OperationID, &op.TransactionID, &op.EventDate, &op.Payer, &op.WithholdingOnly,
			&kind, &componentCommodity, &code, &value, &scale); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan dividend coverage operation: %w", err)
		}
		if len(result) == 0 || result[len(result)-1].OperationID != op.OperationID {
			op.CashCommodityID, op.CashCode = componentCommodity, code
			op.Gross, op.Withholding = exact.NewScaledInt(), exact.NewScaledInt()
			result = append(result, op)
		}
		current := &result[len(result)-1]
		if componentCommodity != current.CashCommodityID {
			rows.Close()
			return nil, fmt.Errorf("dividend operation %d mixes component currencies", current.OperationID)
		}
		if kind == "dividend_gross" {
			current.Gross.AddCoefficient(value, scale)
		} else {
			// Withholding components are stored as negative amounts.
			current.Withholding.AddCoefficient(value.Negated(), scale)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate dividend coverage operations: %w", err)
	}
	return result, nil
}

// ImportedDividendCash is a committed Trading 212 dividend source whose payment
// still posts only as ordinary cash (#242's repairable fallback). Only its cash
// on the import's settlement account is known; the source evidence stays raw.
type ImportedDividendCash struct {
	RowID           int64
	BatchID         int64
	TransactionID   int64
	TransactionDate string
	RawJSON         string
	NormalizedJSON  string
	CashCommodityID int64
	CashCode        string
	Cash            *exact.ScaledInt
}

// ImportedDividendCashFallbacks identifies fallbacks only through durable
// import provenance: a committed identity whose original staged row is a
// Trading 212 dividend. Ordinary income rows are never considered.
func (r *InvestmentRepository) ImportedDividendCashFallbacks(ctx context.Context, bookID int64) ([]ImportedDividendCash, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT s.id, s.batch_id, t.id, tv.transaction_date, s.raw_json, s.normalized_json,
			pv.commodity_id, cash.code, pv.quantity_value, pv.quantity_scale
		FROM import_commit_identities i
		JOIN import_staged_rows s ON s.id = (SELECT MIN(original.id) FROM import_staged_rows original
			WHERE original.committed_identity_id = i.id AND original.commit_status = 'committed')
		JOIN import_batches b ON b.id = s.batch_id AND b.book_id = i.book_id
		JOIN transactions t ON t.id = (SELECT e.transaction_id FROM import_commit_identity_effects e
			WHERE e.identity_id = i.id AND e.transaction_id IS NOT NULL ORDER BY e.effect_seq LIMIT 1)
		JOIN current_transaction_versions tv ON tv.transaction_id = t.id
		JOIN posting_versions pv ON pv.transaction_version_id = tv.id AND pv.account_id = i.account_id
		JOIN commodities cash ON cash.id = pv.commodity_id
		WHERE i.book_id = ? AND i.source_kind = 'trading212' AND b.source_kind = 'trading212'
			AND b.status NOT IN ('discarded', 'rolled_back')
			AND json_extract(s.raw_json, '$.kind') = 'trading212_dividend'
			AND tv.status = 'posted' AND tv.transaction_kind = 'ordinary' AND t.deleted_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM investment_operation_journal_links l
				JOIN transaction_versions lv ON lv.id = l.transaction_version_id WHERE lv.transaction_id = t.id)
		ORDER BY tv.transaction_date, t.id, pv.commodity_id, pv.id`, bookID)
	if err != nil {
		return nil, fmt.Errorf("read imported dividend cash: %w", err)
	}
	var result []ImportedDividendCash
	for rows.Next() {
		var item ImportedDividendCash
		var value exact.Coefficient
		var scale int
		if err := rows.Scan(&item.RowID, &item.BatchID, &item.TransactionID, &item.TransactionDate, &item.RawJSON,
			&item.NormalizedJSON, &item.CashCommodityID, &item.CashCode, &value, &scale); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan imported dividend cash: %w", err)
		}
		last := len(result) - 1
		if last < 0 || result[last].TransactionID != item.TransactionID || result[last].CashCommodityID != item.CashCommodityID {
			item.Cash = exact.NewScaledInt()
			result = append(result, item)
			last++
		}
		result[last].Cash.AddCoefficient(value, scale)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate imported dividend cash: %w", err)
	}
	return result, nil
}
