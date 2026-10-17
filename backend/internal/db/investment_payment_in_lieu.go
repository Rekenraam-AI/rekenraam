package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrPaymentInLieuNotShort refuses a payment in lieu whose holding was not
// short the instrument at the close before the ex-date (#255).
var ErrPaymentInLieuNotShort = errors.New("holding was not short the instrument at the close before the ex-date")

// A payment in lieu of a dividend is owed by a short holder (#255, ADR 0013
// §3). It is a cash charge against an expense account with an optional
// recovery credited back; it opens and consumes no lot and replays nothing.
// The short holding and ex-date are immutable operation facts, and the
// entitlement is re-checked inside the write transaction, never trusted from
// a read before it.

// paymentInLieuEntitlementDate is the close before the ex-date: the holder
// at that close owes the payment, as the provider producer reads it.
func paymentInLieuEntitlementDate(exDate string) (string, error) {
	date, err := time.Parse(time.DateOnly, exDate)
	if err != nil || date.Format(time.DateOnly) != exDate {
		return "", fmt.Errorf("%w: payment in lieu ex-date is not a calendar date", ErrInvalidDisposalParams)
	}
	return date.AddDate(0, 0, -1).Format(time.DateOnly), nil
}

// checkPaymentInLieuEntitlementTx requires open short units of the
// instrument in the holding at the close before the ex-date.
func checkPaymentInLieuEntitlementTx(ctx context.Context, tx *sql.Tx, bookID int64, fact PaymentInLieuFact) error {
	asOf, err := paymentInLieuEntitlementDate(fact.ExDate)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT cost_commodity_id FROM investment_lots
		WHERE book_id = ? AND account_id = ? AND commodity_id = ? AND position_side = ? AND opened_on <= ?
		ORDER BY cost_commodity_id`, bookID, fact.HoldingAccountID, fact.CommodityID, PositionSideShort, asOf)
	if err != nil {
		return fmt.Errorf("read payment in lieu short positions: %w", err)
	}
	var costCommodities []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan payment in lieu short position: %w", err)
		}
		costCommodities = append(costCommodities, id)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("iterate payment in lieu short positions: %w", err)
	}
	for _, costCommodityID := range costCommodities {
		key := investmentReplayPositionKey{accountID: fact.HoldingAccountID, commodityID: fact.CommodityID, costCommodityID: costCommodityID}
		_, held, err := datedHoldingTx(ctx, tx, bookID, key, PositionSideShort, asOf)
		if err != nil {
			return err
		}
		if held {
			return nil
		}
	}
	return ErrPaymentInLieuNotShort
}

// ErrPaymentInLieuDependency refuses a command that would leave an
// effective payment in lieu without the short it was owed on (#257): a short
// sale reversed or replaced away, or a cover dated before the ex-date. The
// owner reverses the payment first.
var ErrPaymentInLieuDependency = errors.New("an effective payment in lieu depends on this short position")

type PaymentInLieuDependencyError struct {
	OperationID   int64
	TransactionID int64
}

func (e PaymentInLieuDependencyError) Error() string {
	return fmt.Sprintf("%s: payment in lieu operation %d transaction %d", ErrPaymentInLieuDependency, e.OperationID, e.TransactionID)
}

func (e PaymentInLieuDependencyError) Unwrap() error { return ErrPaymentInLieuDependency }

type paymentInLieuEntitlement struct {
	OperationID   int64
	TransactionID int64
	Fact          PaymentInLieuFact
}

// entitledPaymentsInLieuTx lists the effective payments in lieu whose short
// entitlement holds in tx. A payment already without one (a self-check
// finding) is left out, so it never blocks an unrelated command.
func entitledPaymentsInLieuTx(ctx context.Context, tx *sql.Tx, bookID int64) ([]paymentInLieuEntitlement, error) {
	rows, err := tx.QueryContext(ctx, `SELECT o.id, v.transaction_id, f.holding_account_id, f.commodity_id, f.ex_date
		FROM effective_investment_operations o
		JOIN investment_payment_in_lieu_facts f ON f.operation_id = o.id AND f.book_id = o.book_id
		JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
		JOIN transaction_versions v ON v.id = link.transaction_version_id
		WHERE o.book_id = ? AND o.operation_kind = 'payment_in_lieu'
		ORDER BY o.id`, bookID)
	if err != nil {
		return nil, fmt.Errorf("read effective payments in lieu: %w", err)
	}
	var payments []paymentInLieuEntitlement
	for rows.Next() {
		var payment paymentInLieuEntitlement
		if err := rows.Scan(&payment.OperationID, &payment.TransactionID, &payment.Fact.HoldingAccountID,
			&payment.Fact.CommodityID, &payment.Fact.ExDate); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan effective payment in lieu: %w", err)
		}
		payments = append(payments, payment)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("iterate effective payments in lieu: %w", err)
	}
	entitled := payments[:0]
	for _, payment := range payments {
		err := checkPaymentInLieuEntitlementTx(ctx, tx, bookID, payment.Fact)
		if errors.Is(err, ErrPaymentInLieuNotShort) {
			continue
		}
		if err != nil {
			return nil, err
		}
		entitled = append(entitled, payment)
	}
	return entitled, nil
}

// requirePaymentsInLieuStillEntitledTx re-checks, after a command's effects,
// every payment entitled before it that is still effective; the first that
// lost its short is named and the command rolls back.
func requirePaymentsInLieuStillEntitledTx(ctx context.Context, tx *sql.Tx, bookID int64, before []paymentInLieuEntitlement) error {
	for _, payment := range before {
		var effective bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM effective_investment_operations
			WHERE book_id = ? AND id = ?)`, bookID, payment.OperationID).Scan(&effective); err != nil {
			return fmt.Errorf("read payment in lieu effectiveness: %w", err)
		}
		if !effective {
			continue
		}
		err := checkPaymentInLieuEntitlementTx(ctx, tx, bookID, payment.Fact)
		if errors.Is(err, ErrPaymentInLieuNotShort) {
			return PaymentInLieuDependencyError{OperationID: payment.OperationID, TransactionID: payment.TransactionID}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func validPaymentInLieuParams(params CreateTransactionParams) bool {
	return params.BookID > 0 && params.ActorUserID > 0 && params.PaymentInLieu != nil &&
		params.PaymentInLieu.HoldingAccountID > 0 && params.PaymentInLieu.CommodityID > 0 &&
		params.PaymentInLieu.ExDate != "" && params.PaymentInLieu.ExDate <= params.Spec.TransactionDate &&
		params.Spec.TransactionKind == "investment" && params.Spec.Status == "posted"
}

// CreatePaymentInLieu posts a payment in lieu through the investment writer
// after re-checking the short entitlement. postWrite (suggestion acceptance)
// runs last in the same transaction.
func (r *InvestmentRepository) CreatePaymentInLieu(ctx context.Context, params CreateTransactionParams, postWrite func(*sql.Tx, int64) error) (TransactionRecord, error) {
	if !validPaymentInLieuParams(params) || params.Spec.InvestmentOperationKind != "payment_in_lieu" {
		return TransactionRecord{}, fmt.Errorf("%w: payment in lieu is incomplete", ErrInvalidDisposalParams)
	}
	transaction, _, err := executeInvestmentWriteWithGuardTx(ctx, r.database, params, func(tx *sql.Tx) error {
		return checkPaymentInLieuEntitlementTx(ctx, tx, params.BookID, *params.PaymentInLieu)
	}, noPaymentInLieuEffect, postWrite)
	return transaction, err
}

// PreviewPaymentInLieu runs the same writer and rolls it back.
func (r *InvestmentRepository) PreviewPaymentInLieu(ctx context.Context, params CreateTransactionParams) (SimulatedInvestmentWrite, error) {
	if !validPaymentInLieuParams(params) || params.Spec.InvestmentOperationKind != "payment_in_lieu" {
		return SimulatedInvestmentWrite{}, fmt.Errorf("%w: payment in lieu is incomplete", ErrInvalidDisposalParams)
	}
	transaction, _, err := previewInvestmentWriteWithGuardTx(ctx, r.database, params, func(tx *sql.Tx) error {
		return checkPaymentInLieuEntitlementTx(ctx, tx, params.BookID, *params.PaymentInLieu)
	}, noPaymentInLieuEffect)
	if err != nil {
		return SimulatedInvestmentWrite{}, err
	}
	return simulatedInvestmentWrite(transaction), nil
}

func noPaymentInLieuEffect(*sql.Tx, TransactionRecord, int64) (struct{}, error) {
	return struct{}{}, nil
}

// PaymentInLieuOperationRecord pins a posted payment in lieu for correction.
type PaymentInLieuOperationRecord struct {
	OperationID          int64
	TransactionID        int64
	TransactionVersionID int64
	CurrentVersionID     int64
	EventDate            string
	Fact                 PaymentInLieuFact
	AlreadyCorrected     bool
}

func (r *InvestmentRepository) PaymentInLieuOperationByTransactionID(ctx context.Context, bookID, transactionID int64) (PaymentInLieuOperationRecord, error) {
	return paymentInLieuOperationByTransactionIDQuery(ctx, r.database, bookID, transactionID)
}

func paymentInLieuOperationByTransactionIDQuery(ctx context.Context, reader saleOperationReader, bookID, transactionID int64) (PaymentInLieuOperationRecord, error) {
	var record PaymentInLieuOperationRecord
	var corrected int
	err := reader.QueryRowContext(ctx, `SELECT o.id, linked_version.transaction_id, link.transaction_version_id,
		current.id, o.event_date, facts.holding_account_id, facts.commodity_id, facts.ex_date,
		EXISTS(SELECT 1 FROM investment_operations successor WHERE successor.correction_of_operation_id = o.id)
		FROM investment_operations o
		JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
		JOIN transaction_versions linked_version ON linked_version.id = link.transaction_version_id
		JOIN current_transaction_versions current ON current.transaction_id = linked_version.transaction_id
		JOIN investment_payment_in_lieu_facts facts ON facts.operation_id = o.id
		WHERE o.book_id = ? AND linked_version.transaction_id = ? AND o.operation_kind = 'payment_in_lieu'`,
		bookID, transactionID).Scan(&record.OperationID, &record.TransactionID, &record.TransactionVersionID,
		&record.CurrentVersionID, &record.EventDate, &record.Fact.HoldingAccountID, &record.Fact.CommodityID,
		&record.Fact.ExDate, &corrected)
	if errors.Is(err, sql.ErrNoRows) {
		return PaymentInLieuOperationRecord{}, ErrNotFound
	}
	if err != nil {
		return PaymentInLieuOperationRecord{}, fmt.Errorf("read payment in lieu operation: %w", err)
	}
	record.AlreadyCorrected = corrected != 0
	return record, nil
}

func checkPaymentInLieuForCorrectionTx(ctx context.Context, tx *sql.Tx, bookID int64, expected PaymentInLieuOperationRecord) error {
	current, err := paymentInLieuOperationByTransactionIDQuery(ctx, tx, bookID, expected.TransactionID)
	if err != nil {
		return err
	}
	if current.AlreadyCorrected {
		return ErrInvestmentOperationAlreadyCorrected
	}
	if current != expected {
		return ErrInvestmentSaleChanged
	}
	return checkInvestmentSourceJournalTx(ctx, tx, bookID, current.TransactionID,
		current.EventDate, current.TransactionVersionID, current.CurrentVersionID)
}

// ReversePaymentInLieu posts the exact inverse as a reversal operation that
// repeats the original's entitlement facts. A reversal needs no entitlement.
func (r *InvestmentRepository) ReversePaymentInLieu(ctx context.Context, params CreateTransactionParams, expected PaymentInLieuOperationRecord) (TransactionRecord, error) {
	return r.reversePaymentInLieu(ctx, params, expected, false)
}

// PreviewPaymentInLieuReversal runs the reversal writer and rolls it back.
func (r *InvestmentRepository) PreviewPaymentInLieuReversal(ctx context.Context, params CreateTransactionParams, expected PaymentInLieuOperationRecord) (SimulatedInvestmentWrite, error) {
	transaction, err := r.reversePaymentInLieu(ctx, params, expected, true)
	if err != nil {
		return SimulatedInvestmentWrite{}, err
	}
	return simulatedInvestmentWrite(transaction), nil
}

func (r *InvestmentRepository) reversePaymentInLieu(ctx context.Context, params CreateTransactionParams, expected PaymentInLieuOperationRecord, preview bool) (TransactionRecord, error) {
	if !validPaymentInLieuParams(params) || expected.OperationID <= 0 ||
		params.Spec.InvestmentOperationKind != "reversal" || *params.PaymentInLieu != expected.Fact ||
		params.Spec.TransactionDate != expected.EventDate ||
		params.InvestmentCorrectionOfOperationID != expected.OperationID ||
		params.InvestmentCorrectionMode != "reverse" || params.InvestmentCorrectionReason == "" ||
		!params.CorrectionOfTransactionID.Valid || params.CorrectionOfTransactionID.Int64 != expected.TransactionID {
		return TransactionRecord{}, fmt.Errorf("%w: payment in lieu reversal is incomplete", ErrInvalidDisposalParams)
	}
	guard := func(tx *sql.Tx) error {
		return checkPaymentInLieuForCorrectionTx(ctx, tx, params.BookID, expected)
	}
	var transaction TransactionRecord
	var err error
	if preview {
		transaction, _, err = previewInvestmentWriteWithGuardTx(ctx, r.database, params, guard, noPaymentInLieuEffect)
	} else {
		transaction, _, err = executeInvestmentWriteWithGuardTx(ctx, r.database, params, guard, noPaymentInLieuEffect, nil)
	}
	return transaction, err
}

type PaymentInLieuReplacementRecord struct {
	Inverse     TransactionRecord
	Replacement TransactionRecord
}

// ReplacePaymentInLieu posts the inverse and the corrected payment under one
// audit event; the replacement re-checks the (unchanged) short entitlement.
func (r *InvestmentRepository) ReplacePaymentInLieu(ctx context.Context, expected PaymentInLieuOperationRecord,
	inverseParams, replacementParams CreateTransactionParams,
) (PaymentInLieuReplacementRecord, error) {
	return r.replacePaymentInLieu(ctx, expected, inverseParams, replacementParams, false)
}

// SimulatePaymentInLieuReplacement runs the replacement writer and rolls it back.
func (r *InvestmentRepository) SimulatePaymentInLieuReplacement(ctx context.Context, expected PaymentInLieuOperationRecord,
	inverseParams, replacementParams CreateTransactionParams,
) (SimulatedInvestmentWrite, error) {
	record, err := r.replacePaymentInLieu(ctx, expected, inverseParams, replacementParams, true)
	if err != nil {
		return SimulatedInvestmentWrite{}, err
	}
	return simulatedInvestmentWrite(record.Inverse, record.Replacement), nil
}

func (r *InvestmentRepository) replacePaymentInLieu(ctx context.Context, expected PaymentInLieuOperationRecord,
	inverseParams, replacementParams CreateTransactionParams, preview bool,
) (PaymentInLieuReplacementRecord, error) {
	if expected.OperationID <= 0 || inverseParams.BookID <= 0 || inverseParams.BookID != replacementParams.BookID ||
		inverseParams.ActorUserID != replacementParams.ActorUserID ||
		inverseParams.Spec.InvestmentOperationKind != "" || inverseParams.PaymentInLieu != nil ||
		inverseParams.Spec.TransactionKind != "investment" || inverseParams.Spec.Status != "posted" ||
		!validPaymentInLieuParams(replacementParams) || *replacementParams.PaymentInLieu != expected.Fact ||
		replacementParams.Spec.InvestmentOperationKind != "payment_in_lieu" ||
		inverseParams.Spec.TransactionDate != expected.EventDate ||
		replacementParams.Spec.TransactionDate != expected.EventDate ||
		replacementParams.InvestmentCorrectionOfOperationID != expected.OperationID ||
		replacementParams.InvestmentCorrectionMode != "replace" ||
		replacementParams.InvestmentCorrectionReason == "" ||
		!inverseParams.CorrectionOfTransactionID.Valid ||
		inverseParams.CorrectionOfTransactionID.Int64 != expected.TransactionID ||
		!replacementParams.CorrectionOfTransactionID.Valid ||
		replacementParams.CorrectionOfTransactionID.Int64 != expected.TransactionID {
		return PaymentInLieuReplacementRecord{}, fmt.Errorf("%w: payment in lieu replacement is incomplete", ErrInvalidDisposalParams)
	}
	write := executeInvestmentJournalsWithGuardTx[struct{}]
	if preview {
		write = previewInvestmentJournalsWithGuardTx[struct{}]
	}
	journals, _, err := write(ctx, r.database,
		[]CreateTransactionParams{inverseParams, replacementParams},
		func(tx *sql.Tx) error {
			if err := checkPaymentInLieuForCorrectionTx(ctx, tx, inverseParams.BookID, expected); err != nil {
				return err
			}
			return checkPaymentInLieuEntitlementTx(ctx, tx, replacementParams.BookID, expected.Fact)
		}, func(tx *sql.Tx, journals []TransactionRecord, _ int64) (struct{}, error) {
			inverse, replacement := journals[0], journals[1]
			operationID, err := investmentOperationIDTx(ctx, tx, replacementParams.BookID, replacement.ID)
			if err != nil {
				return struct{}{}, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO investment_operation_journal_links
		(book_id, operation_id, transaction_version_id, link_seq, role)
		VALUES (?, ?, ?, 2, 'reversal')`, replacementParams.BookID, operationID, inverse.VersionID); err != nil {
				return struct{}{}, fmt.Errorf("link payment in lieu replacement inverse: %w", err)
			}
			return struct{}{}, nil
		}, nil)
	if err != nil {
		return PaymentInLieuReplacementRecord{}, err
	}
	return PaymentInLieuReplacementRecord{Inverse: journals[0], Replacement: journals[1]}, nil
}
