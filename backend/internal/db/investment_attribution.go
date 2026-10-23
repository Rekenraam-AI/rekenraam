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

const (
	AttributionNoShort            = "no_short_at_entitlement_close"
	AttributionExDateAfterPayment = "ex_date_after_payment"
	AttributionProviderMismatch   = "provider_attribution_mismatch"
)

// PaymentInLieuAttributionDiagnostic describes historical support for the
// owner's attribution. It does not verify the charged amount or authorize a
// write. Structural damage belongs to the foundation/ledger checks.
type PaymentInLieuAttributionDiagnostic struct {
	TransactionID  int64
	Unavailable    bool
	WarningReasons []string
}

type paymentInLieuAttributionRecord struct {
	TransactionID  int64
	PaymentDate    string
	Fact           PaymentInLieuFact
	SourceMismatch bool
}

func (r *SelfCheckRepository) PaymentInLieuAttributions(ctx context.Context, snapshot *sql.Tx, bookID int64) ([]PaymentInLieuAttributionDiagnostic, error) {
	return paymentInLieuAttributions(ctx, snapshot, bookID, 0)
}

// PaymentInLieuAttribution uses the same evaluator as the whole-book check,
// against a single read snapshot. Ordinary investment commands never call it.
func (r *InvestmentRepository) PaymentInLieuAttribution(ctx context.Context, bookID, transactionID int64) (PaymentInLieuAttributionDiagnostic, error) {
	tx, err := r.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return PaymentInLieuAttributionDiagnostic{}, err
	}
	defer rollbackTx(ctx, tx)
	results, err := paymentInLieuAttributions(ctx, tx, bookID, transactionID)
	if err != nil {
		return PaymentInLieuAttributionDiagnostic{}, err
	}
	if len(results) != 1 {
		return PaymentInLieuAttributionDiagnostic{}, ErrNotFound
	}
	return results[0], nil
}

func paymentInLieuAttributions(ctx context.Context, source queryer, bookID, transactionID int64) ([]PaymentInLieuAttributionDiagnostic, error) {
	rows, err := source.QueryContext(ctx, `SELECT v.transaction_id, o.event_date,
		f.holding_account_id, f.commodity_id, f.ex_date,
		EXISTS (SELECT 1 FROM investment_payment_in_lieu_ancestry ancestry
			JOIN investment_operation_journal_links source_link ON source_link.operation_id = ancestry.ancestor_operation_id AND source_link.role = 'primary'
			JOIN transaction_versions source_version ON source_version.id = source_link.transaction_version_id
			JOIN investment_event_suggestions suggestion ON suggestion.book_id = ancestry.book_id
				AND (suggestion.generated_transaction_id = source_version.transaction_id OR suggestion.linked_transaction_id = source_version.transaction_id)
			JOIN investment_provider_events event ON event.id = suggestion.provider_event_id AND event.book_id = suggestion.book_id
			WHERE ancestry.book_id = o.book_id AND ancestry.operation_id = o.id
			AND suggestion.status IN ('accepted', 'auto_posted', 'linked')
			AND (f.commodity_id IS NOT json_extract(suggestion.proposed_transaction_json, '$.commodity_id')
				OR f.ex_date IS NOT event.event_date
				OR NOT EXISTS (SELECT 1 FROM json_each(suggestion.proposed_transaction_json, '$.holding_account_ids') WHERE value = f.holding_account_id)))
		FROM effective_investment_operations o
		JOIN valid_investment_payment_in_lieu_facts f ON f.operation_id = o.id AND f.book_id = o.book_id
		JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
		JOIN transaction_versions v ON v.id = link.transaction_version_id AND v.book_id = o.book_id
		WHERE o.book_id = ? AND o.operation_kind = 'payment_in_lieu' AND (? = 0 OR v.transaction_id = ?)
		ORDER BY o.id`, bookID, transactionID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("read payment attribution: %w", err)
	}
	var payments []paymentInLieuAttributionRecord
	for rows.Next() {
		var record paymentInLieuAttributionRecord
		// Nullable reads preserve #388's partial owner attribution. A missing
		// facts row is deliberately excluded, never disguised as unavailable:
		// investment_foundation independently names that structural failure.
		var holding, commodity sql.NullInt64
		var exDate sql.NullString
		if err := rows.Scan(&record.TransactionID, &record.PaymentDate, &holding, &commodity, &exDate, &record.SourceMismatch); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan payment attribution: %w", err)
		}
		record.Fact = PaymentInLieuFact{HoldingAccountID: holding.Int64, CommodityID: commodity.Int64, ExDate: exDate.String}
		payments = append(payments, record)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	if len(payments) == 0 {
		return []PaymentInLieuAttributionDiagnostic{}, nil
	}
	movements, err := EffectiveLotMovements(ctx, source, bookID, EffectiveMovementFilter{PositionSide: PositionSideShort})
	if err != nil {
		return nil, fmt.Errorf("read attributed short quantities: %w", err)
	}
	return evaluatePaymentInLieuAttributions(payments, movements), nil
}

// Inputs have passed the structural SQL predicates above.
// Sweep one effective dated stream once for all payments. Keep currencies
// separate: a positive group establishes support even if another is zero or
// negative. No SQL coefficient sums, current-state shortcuts or savepoint
// replay; the reader works on a query-only SQLite connection.
func evaluatePaymentInLieuAttributions(payments []paymentInLieuAttributionRecord, movements []EffectiveLotMovement) []PaymentInLieuAttributionDiagnostic {
	order := make([]int, len(payments))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return cmp.Compare(payments[a].Fact.ExDate, payments[b].Fact.ExDate) })
	type position struct{ account, commodity int64 }
	quantities := map[position]map[int64]*exact.ScaledInt{}
	results := make([]PaymentInLieuAttributionDiagnostic, len(payments))
	next := 0
	for _, i := range order {
		payment := payments[i]
		fact := payment.Fact
		result := PaymentInLieuAttributionDiagnostic{TransactionID: payment.TransactionID, WarningReasons: []string{}}
		if payment.SourceMismatch {
			result.WarningReasons = append(result.WarningReasons, AttributionProviderMismatch)
		}
		if fact.ExDate != "" && fact.ExDate > payment.PaymentDate {
			result.WarningReasons = append(result.WarningReasons, AttributionExDateAfterPayment)
		}
		if fact.HoldingAccountID == 0 || fact.CommodityID == 0 || fact.ExDate == "" {
			result.Unavailable = true
			results[i] = result
			continue
		}
		for next < len(movements) && movements[next].EventDate < fact.ExDate {
			movement := movements[next]
			key := position{movement.AccountID, movement.CommodityID}
			if quantities[key] == nil {
				quantities[key] = map[int64]*exact.ScaledInt{}
			}
			if quantities[key][movement.CostCommodityID] == nil {
				quantities[key][movement.CostCommodityID] = exact.NewScaledInt()
			}
			quantities[key][movement.CostCommodityID].AddCoefficient(movement.QuantityValue, movement.QuantityScale)
			next++
		}
		supported := false
		for _, quantity := range quantities[position{fact.HoldingAccountID, fact.CommodityID}] {
			if quantity.Sign() > 0 {
				supported = true
				break
			}
		}
		if !supported {
			result.WarningReasons = append(result.WarningReasons, AttributionNoShort)
		}
		results[i] = result
	}
	return results
}
