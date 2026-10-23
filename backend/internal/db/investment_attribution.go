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
	TransactionID int64
	PaymentDate   string
	Fact          PaymentInLieuFact
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
		f.holding_account_id, f.commodity_id, f.ex_date
		FROM effective_investment_operations o
		JOIN investment_payment_in_lieu_facts f ON f.operation_id = o.id AND f.book_id = o.book_id
		JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
		JOIN transaction_versions v ON v.id = link.transaction_version_id AND v.book_id = o.book_id
		WHERE o.book_id = ? AND o.operation_kind = 'payment_in_lieu' AND (? = 0 OR v.transaction_id = ?)
		AND o.created_audit_event_id = f.created_audit_event_id
		AND (f.ex_date IS NULL OR (length(f.ex_date) = 10
			AND f.ex_date GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]'
			AND date(f.ex_date, '+0 days') = f.ex_date))
		AND (f.holding_account_id IS NULL OR EXISTS (SELECT 1 FROM accounts a WHERE a.id = f.holding_account_id AND a.book_id = f.book_id))
		AND (f.commodity_id IS NULL OR EXISTS (SELECT 1 FROM commodities c WHERE c.id = f.commodity_id AND c.book_id = f.book_id AND c.kind IN ('security', 'crypto')))
		ORDER BY o.id`, bookID, transactionID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("read payment attribution: %w", err)
	}
	var payments []paymentInLieuAttributionRecord
	for rows.Next() {
		var record paymentInLieuAttributionRecord
		// Nullable reads prepare for #388's partial owner attribution. A missing
		// facts row is deliberately excluded, never disguised as unavailable:
		// investment_foundation independently names that structural failure.
		var holding, commodity sql.NullInt64
		var exDate sql.NullString
		if err := rows.Scan(&record.TransactionID, &record.PaymentDate, &holding, &commodity, &exDate); err != nil {
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
