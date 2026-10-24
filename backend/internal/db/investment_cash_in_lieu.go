package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Cash in lieu (slice 5, T-147) runs through the ordinary long disposal
// writer: the fraction is disposed under the holding's election with its
// cash as proceeds, the security legs on the disposal date and the cash on
// the payment date (the trade's settlement date). Its link to the operation
// it settles (a split, or since #181 a share exchange, cash-and-stock merger
// or spin-off) is written inside the same transaction, after the disposal,
// where the facts trigger validates provenance and the typed correction chain.

var ErrCashInLieuSplitUnavailable = errors.New("the operation this cash in lieu settles was not found or does not match the original holding")

// CashInLieuSettledRecord is the operation whose fraction a cash in lieu
// settles: the holding and instrument the fraction is in, and the date from
// which it can be disposed. For an exchange or spin-off they are its
// destination holding and new instrument.
type CashInLieuSettledRecord struct {
	OperationID      int64
	OperationKind    string
	EventDate        string
	AccountID        int64
	CommodityID      int64
	AlreadyCorrected bool
}

// CashInLieuSettledOperation reads the split, exchange, merger or spin-off
// posted as transactionID.
func (r *InvestmentRepository) CashInLieuSettledOperation(ctx context.Context, bookID, transactionID int64) (CashInLieuSettledRecord, error) {
	return r.cashInLieuSettledOperation(ctx, bookID, "v.transaction_id", transactionID)
}

func (r *InvestmentRepository) CashInLieuSettledOperationByID(ctx context.Context, bookID, operationID int64) (CashInLieuSettledRecord, error) {
	return r.cashInLieuSettledOperation(ctx, bookID, "o.id", operationID)
}

func (r *InvestmentRepository) cashInLieuSettledOperation(ctx context.Context, bookID int64, column string, id int64) (CashInLieuSettledRecord, error) {
	var record CashInLieuSettledRecord
	err := r.database.QueryRowContext(ctx, `SELECT o.id, o.operation_kind,
			COALESCE(s.effective_on, f.effective_on), COALESCE(s.account_id, f.destination_account_id),
			COALESCE(s.commodity_id, f.destination_commodity_id),
			EXISTS (SELECT 1 FROM investment_operations successor WHERE successor.correction_of_operation_id = o.id)
		FROM investment_operations o
		LEFT JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.role = 'primary'
		LEFT JOIN transaction_versions v ON v.id = link.transaction_version_id
		LEFT JOIN investment_split_facts s ON s.operation_id = o.id AND o.operation_kind = 'split'
		LEFT JOIN investment_transfer_facts f ON f.operation_id = o.id AND f.transfer_kind IN ('exchange', 'spin_off')
			AND o.operation_kind IN ('share_exchange', 'cash_and_stock_merger', 'spin_off')
		WHERE o.book_id = ? AND `+column+` = ? AND (s.operation_id IS NOT NULL OR f.operation_id IS NOT NULL)`,
		bookID, id).Scan(&record.OperationID, &record.OperationKind, &record.EventDate,
		&record.AccountID, &record.CommodityID, &record.AlreadyCorrected)
	if errors.Is(err, sql.ErrNoRows) {
		return CashInLieuSettledRecord{}, ErrNotFound
	}
	if err != nil {
		return CashInLieuSettledRecord{}, fmt.Errorf("read operation a cash in lieu settles: %w", err)
	}
	return record, nil
}

// CashInLieuFactWriter returns the post-write step that links a committed
// cash-in-lieu disposal to the operation it settles.
func CashInLieuFactWriter(ctx context.Context, bookID, settledOperationID int64, paymentOn string) func(*sql.Tx, int64) error {
	return func(tx *sql.Tx, transactionID int64) error {
		var operationID, auditEventID int64
		if err := tx.QueryRowContext(ctx, `SELECT o.id, o.created_audit_event_id FROM investment_operations o
			JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.role = 'primary'
			JOIN transaction_versions v ON v.id = link.transaction_version_id
			WHERE o.book_id = ? AND v.transaction_id = ? AND o.operation_kind = 'cash_in_lieu'`,
			bookID, transactionID).Scan(&operationID, &auditEventID); err != nil {
			return fmt.Errorf("read cash in lieu operation: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO investment_cash_in_lieu_facts
			(operation_id, book_id, settled_operation_id, payment_on, created_audit_event_id)
			VALUES (?, ?, ?, ?, ?)`, operationID, bookID, settledOperationID, paymentOn, auditEventID); err != nil {
			if strings.Contains(err.Error(), "investment cash in lieu fact is outside its disposal or settled operation") {
				return ErrCashInLieuSplitUnavailable
			}
			return fmt.Errorf("record cash in lieu fact: %w", err)
		}
		return nil
	}
}

// CashInLieuReplacementFactWriter links a replacement cash in lieu (T-150)
// to the operation its predecessor settled; the replacement writer hands it
// the new operation and audit event.
func CashInLieuReplacementFactWriter(ctx context.Context, bookID, settledOperationID int64, paymentOn string) func(*sql.Tx, int64, int64) error {
	return func(tx *sql.Tx, operationID, auditEventID int64) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO investment_cash_in_lieu_facts
			(operation_id, book_id, settled_operation_id, payment_on, created_audit_event_id)
			VALUES (?, ?, ?, ?, ?)`, operationID, bookID, settledOperationID, paymentOn, auditEventID); err != nil {
			if strings.Contains(err.Error(), "investment cash in lieu fact is outside its disposal or settled operation") {
				return ErrCashInLieuSplitUnavailable
			}
			return fmt.Errorf("record replacement cash in lieu fact: %w", err)
		}
		return nil
	}
}

// CashInLieuSettledIDs reads the immutable operation a disposal settles and
// its optional primary transaction. Operation identity survives either shape.
func (r *InvestmentRepository) CashInLieuSettledIDs(ctx context.Context, bookID, transactionID int64) (int64, int64, error) {
	var operationID, primaryID int64
	err := r.database.QueryRowContext(ctx, `SELECT f.settled_operation_id, COALESCE(sv.transaction_id, 0)
 FROM investment_cash_in_lieu_facts f
 JOIN investment_operation_journal_links link ON link.operation_id = f.operation_id AND link.role = 'primary'
 JOIN transaction_versions v ON v.id = link.transaction_version_id
 LEFT JOIN investment_operation_journal_links sl ON sl.operation_id = f.settled_operation_id AND sl.role = 'primary'
 LEFT JOIN transaction_versions sv ON sv.id = sl.transaction_version_id
 WHERE f.book_id = ? AND v.transaction_id = ?`, bookID, transactionID).Scan(&operationID, &primaryID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	if err != nil {
		return 0, 0, fmt.Errorf("read operation a cash in lieu settles: %w", err)
	}
	return operationID, primaryID, nil
}

// Cash-in-lieu previews include the immutable settlement fact and its transactional
// guard. No source acceptance runs, and every temporary row is rolled back.
func (r *InvestmentRepository) SimulateCashInLieu(ctx context.Context, journal CreateTransactionParams, disposal DisposeLotsParams, settledOperationID int64, paymentOn string) (SimulatedInvestmentWrite, []LotDisposalRecord, DisposalDecisionRecord, error) {
	transaction, allocations, decision, err := r.writeTransactionAndDisposeLots(ctx, journal, disposal, nil, CashInLieuFactWriter(ctx, journal.BookID, settledOperationID, paymentOn), true)
	if err != nil {
		return SimulatedInvestmentWrite{}, nil, DisposalDecisionRecord{}, err
	}
	for index := range allocations {
		allocations[index].EventID = 0
	}
	decision.ID, decision.TransactionID, decision.TransactionVersionID, decision.AuditEventID = 0, 0, 0, 0
	decision.Allocations = allocations
	return simulatedInvestmentWrite(transaction), allocations, decision, nil
}

func (r *InvestmentRepository) SimulateCashInLieuReplacement(ctx context.Context, expected SaleOperationRecord, inverse, replacement CreateTransactionParams, disposal DisposeLotsParams, settledOperationID int64, paymentOn string) (SimulatedInvestmentWrite, []LotDisposalRecord, DisposalDecisionRecord, error) {
	reversed, corrected, allocations, decision, err := r.replaceSale(ctx, expected, inverse, replacement, disposal, nil, CashInLieuReplacementFactWriter(ctx, inverse.BookID, settledOperationID, paymentOn), true)
	if err != nil {
		return SimulatedInvestmentWrite{}, nil, DisposalDecisionRecord{}, err
	}
	for index := range allocations {
		allocations[index].EventID = 0
	}
	decision.ID, decision.TransactionID, decision.TransactionVersionID, decision.AuditEventID = 0, 0, 0, 0
	decision.Allocations = allocations
	return simulatedInvestmentWrite(reversed, corrected), allocations, decision, nil
}

// CashInLieuAvailableLots returns dated pre-disposal quantities. A replacement
// removes its predecessor and keeps its original same-day root slot.
func (r *InvestmentRepository) CashInLieuAvailableLots(ctx context.Context, bookID int64, settled CashInLieuSettledRecord, currencyID int64, date string, replacingTransactionID int64) ([]InvestmentTradeCorrectionAvailableLot, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer rollbackTx(ctx, tx)
	intents, err := investmentReplayIntentsQuery(ctx, tx, bookID, settled.AccountID, settled.CommodityID, currencyID, "long")
	if err != nil {
		return nil, err
	}
	orderID := int64(0)
	var replacingID int64
	if replacingTransactionID > 0 {
		if err := tx.QueryRowContext(ctx, `SELECT f.operation_id FROM investment_cash_in_lieu_facts f
 JOIN investment_operation_journal_links l ON l.operation_id = f.operation_id AND l.role = 'primary'
 JOIN transaction_versions v ON v.id = l.transaction_version_id
 JOIN effective_investment_operations o ON o.id = f.operation_id
 WHERE f.book_id = ? AND v.transaction_id = ? AND f.settled_operation_id = ?`, bookID, replacingTransactionID, settled.OperationID).Scan(&replacingID); err != nil {
			return nil, ErrNotFound
		}
		orders, err := investmentReplayOrderOperationIDsQuery(ctx, tx, bookID)
		if err != nil {
			return nil, err
		}
		orderID = orders[replacingID]
	}
	before := make([]InvestmentReplayIntent, 0, len(intents))
	opened := make(map[int64]string)
	for _, intent := range intents {
		if intent.OperationID == replacingID || intent.EventDate > date || (orderID > 0 && intent.EventDate == date && intent.OrderOperationID >= orderID) {
			continue
		}
		before = append(before, intent)
		if intent.Kind == "opening" {
			opened[intent.LotID] = intent.EventDate
		}
	}
	projection, err := simulateInvestmentReplayTx(ctx, tx, bookID, settled.AccountID, settled.CommodityID, currencyID, before)
	if err != nil {
		return nil, err
	}
	lots := make([]InvestmentTradeCorrectionAvailableLot, 0, len(projection.Lots))
	for _, lot := range projection.Lots {
		if lot.RemainingQuantityValue.Sign() > 0 && opened[lot.LotID] != "" {
			lots = append(lots, InvestmentTradeCorrectionAvailableLot{LotID: lot.LotID, OpenedOn: opened[lot.LotID], QuantityValue: lot.RemainingQuantityValue.String(), QuantityScale: lot.RemainingQuantityScale})
		}
	}
	return lots, nil
}
