package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"rekenraam/backend/internal/exact"
)

// CapitalReturnRead is one immutable whole-effect set. Effective is true only
// for the latest set of a currently effective operation, never a retired fact.
type CapitalReturnRead struct {
	OperationID, TransactionID, AccountID, CommodityID, CostCommodityID int64
	HoldingName, InstrumentName, CurrencyCode                           string
	CurrencyScale                                                       int
	EffectiveOn, PaymentOn                                              string
	AmountValue                                                         exact.Coefficient
	AmountScale                                                         int
	EntitlementRule, SourceEvidenceJSON                                 string
	RevisionID                                                          int64 // zero denotes the original set
	RevisionSeq                                                         int
	CausedByOperationID                                                 int64 // zero denotes the original operation
	AuditEventID                                                        int64
	CreatedAt                                                           string
	Effective                                                           bool
	Effects                                                             []CapitalReturnEffect
}

// ListCapitalReturnReads composes receipts and complete allocation sets in one
// snapshot. A gains read selects only effective operations/latest sets. A
// correction-history read restricts to the chain's operation IDs and keeps all
// sets, including originals whose lot identities a replay replaced.
func (r *InvestmentRepository) ListCapitalReturnReads(ctx context.Context, bookID int64, currentOnly bool, from, to string, operationIDs []int64) ([]CapitalReturnRead, error) {
	query := `WITH sets AS (
  SELECT o.id operation_id, 0 revision_id, 1 revision_seq, 0 caused_by_operation_id,
   o.created_audit_event_id audit_id, o.created_at
  FROM investment_operations o JOIN investment_capital_return_facts f ON f.operation_id = o.id WHERE o.book_id = ?
  UNION ALL
  SELECT operation_id, id, revision_seq, caused_by_operation_id, created_audit_event_id, created_at
  FROM investment_capital_return_revisions WHERE book_id = ?
 ), effects AS (
  SELECT operation_id, 0 revision_id, effect_seq, lot_id, entitled_quantity_value, entitled_quantity_scale,
   allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale, basis_knowledge
  FROM investment_capital_return_effects WHERE book_id = ?
  UNION ALL
  SELECT rev.operation_id, e.revision_id, e.effect_seq, e.lot_id, e.entitled_quantity_value, e.entitled_quantity_scale,
   e.allocated_value, e.allocated_scale, e.reduction_value, e.reduction_scale, e.excess_value, e.excess_scale, e.basis_knowledge
  FROM investment_capital_return_revision_effects e JOIN investment_capital_return_revisions rev ON rev.id = e.revision_id WHERE e.book_id = ?
 )
 SELECT f.operation_id, tv.transaction_id, f.account_id, f.commodity_id, f.cost_commodity_id,
  av.name, COALESCE(iv.display_name, security.code), c.code, cv.standard_scale, f.effective_on, f.payment_on, f.amount_value, f.amount_scale,
  f.entitlement_rule, f.source_evidence_json, s.revision_id, s.revision_seq, s.caused_by_operation_id, s.audit_id, s.created_at,
  (eo.id IS NOT NULL AND s.revision_id = COALESCE(latest.id, 0)),
  e.lot_id, e.entitled_quantity_value, e.entitled_quantity_scale, e.allocated_value, e.allocated_scale,
  e.reduction_value, e.reduction_scale, e.excess_value, e.excess_scale, e.basis_knowledge
 FROM sets s JOIN investment_capital_return_facts f ON f.operation_id = s.operation_id
 JOIN investment_operation_journal_links jl ON jl.operation_id = f.operation_id AND jl.role = 'primary'
 JOIN transaction_versions tv ON tv.id = jl.transaction_version_id
 JOIN ` + CurrentReferenceVersions(ctx, "account") + ` av ON av.account_id = f.account_id
 JOIN commodities security ON security.id = f.commodity_id
 LEFT JOIN investment_instruments i ON i.commodity_id = f.commodity_id
 LEFT JOIN ` + CurrentReferenceVersions(ctx, "investment_instrument") + ` iv ON iv.instrument_id = i.id
 JOIN commodities c ON c.id = f.cost_commodity_id
 JOIN ` + CurrentReferenceVersions(ctx, "commodity") + ` cv ON cv.commodity_id = c.id
 LEFT JOIN effective_investment_operations eo ON eo.id = f.operation_id
 LEFT JOIN latest_investment_capital_return_revisions latest ON latest.operation_id = f.operation_id
 LEFT JOIN effects e ON e.operation_id = s.operation_id AND e.revision_id = s.revision_id
 WHERE f.book_id = ?`
	args := []any{bookID, bookID, bookID, bookID, bookID}
	if currentOnly {
		query += ` AND eo.id IS NOT NULL AND s.revision_id = COALESCE(latest.id, 0)`
	}
	if from != "" {
		query += ` AND f.payment_on >= ?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND f.payment_on <= ?`
		args = append(args, to)
	}
	if operationIDs != nil {
		if len(operationIDs) == 0 {
			return []CapitalReturnRead{}, nil
		}
		query += ` AND f.operation_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(operationIDs)), ",") + `)`
		for _, id := range operationIDs {
			args = append(args, id)
		}
	}
	query += ` ORDER BY f.payment_on, f.operation_id, s.revision_seq, e.effect_seq`
	rows, err := r.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read return of capital allocations: %w", err)
	}
	defer rows.Close()
	out := make([]CapitalReturnRead, 0)
	for rows.Next() {
		var record CapitalReturnRead
		var lot, quantityScale, allocatedScale, reductionScale, excessScale sql.NullInt64
		var quantity, allocated, reduction, excess, knowledge sql.NullString
		if err := rows.Scan(&record.OperationID, &record.TransactionID, &record.AccountID, &record.CommodityID, &record.CostCommodityID,
			&record.HoldingName, &record.InstrumentName, &record.CurrencyCode, &record.CurrencyScale, &record.EffectiveOn, &record.PaymentOn,
			&record.AmountValue, &record.AmountScale, &record.EntitlementRule, &record.SourceEvidenceJSON,
			&record.RevisionID, &record.RevisionSeq, &record.CausedByOperationID, &record.AuditEventID, &record.CreatedAt, &record.Effective,
			&lot, &quantity, &quantityScale, &allocated, &allocatedScale, &reduction, &reductionScale, &excess, &excessScale, &knowledge); err != nil {
			return nil, fmt.Errorf("scan return of capital allocations: %w", err)
		}
		if len(out) == 0 || out[len(out)-1].OperationID != record.OperationID || out[len(out)-1].RevisionID != record.RevisionID {
			record.Effects = make([]CapitalReturnEffect, 0)
			out = append(out, record)
		}
		if lot.Valid {
			effect := CapitalReturnEffect{LotID: lot.Int64, EntitledQuantityValue: exact.Coefficient(quantity.String), EntitledQuantityScale: int(quantityScale.Int64),
				AllocatedValue: exact.Coefficient(allocated.String), AllocatedScale: int(allocatedScale.Int64), BasisKnowledge: knowledge.String}
			if effect.ReductionValue, effect.ReductionScale, err = capitalReturnAmount(reduction, reductionScale, knowledge.String); err != nil {
				return nil, err
			}
			if effect.ExcessValue, effect.ExcessScale, err = capitalReturnAmount(excess, excessScale, knowledge.String); err != nil {
				return nil, err
			}
			out[len(out)-1].Effects = append(out[len(out)-1].Effects, effect)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate return of capital allocations: %w", err)
	}
	return out, nil
}
