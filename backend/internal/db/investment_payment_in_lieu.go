package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ValidatePaymentInLieuAttribution validates references, not dated posting
// eligibility: archived/future-opened holdings and instruments still identify
// real objects. The insertion trigger repeats these storage constraints at
// commit. Cash posting roles and dates are validated by their normal owners.
func (r *InvestmentRepository) ValidatePaymentInLieuAttribution(ctx context.Context, bookID int64, fact PaymentInLieuFact) error {
	var valid bool
	err := r.database.QueryRowContext(ctx, `SELECT
		(? = 0 OR EXISTS (SELECT 1 FROM accounts a JOIN account_versions av ON av.account_id = a.id
			JOIN account_kinds k ON k.code = av.account_kind AND k.account_class = av.account_class
			WHERE a.book_id = ? AND a.id = ? AND a.system_role IS NULL AND av.account_class = 'asset' AND k.base_kind = 'security_holding'
			AND av.version_seq = (SELECT MAX(version_seq) FROM account_versions WHERE account_id = a.id)))
		AND (? = 0 OR EXISTS (SELECT 1 FROM commodities c WHERE c.book_id = ? AND c.id = ? AND c.kind IN ('security', 'crypto')))`,
		fact.HoldingAccountID, bookID, fact.HoldingAccountID, fact.CommodityID, bookID, fact.CommodityID).Scan(&valid)
	if err != nil {
		return fmt.Errorf("read payment attribution references: %w", err)
	}
	if !valid {
		return fmt.Errorf("%w: payment attribution must identify a holding and instrument in this book", ErrInvalidDisposalParams)
	}
	return nil
}

func validPaymentInLieuParams(params CreateTransactionParams) bool {
	return params.BookID > 0 && params.ActorUserID > 0 && params.PaymentInLieu != nil &&
		params.Spec.TransactionKind == "investment" && params.Spec.Status == "posted"
}

// CreatePaymentInLieu posts a cash charge through the investment writer.
// Optional attribution never authorizes the cash. postWrite (suggestion acceptance)
// runs last in the same transaction.
func (r *InvestmentRepository) CreatePaymentInLieu(ctx context.Context, params CreateTransactionParams, postWrite func(*sql.Tx, int64) error) (TransactionRecord, error) {
	if !validPaymentInLieuParams(params) || params.Spec.InvestmentOperationKind != "payment_in_lieu" {
		return TransactionRecord{}, fmt.Errorf("%w: payment in lieu is incomplete", ErrInvalidDisposalParams)
	}
	transaction, _, err := executeInvestmentWriteWithGuardTx(ctx, r.database, params, nil, noPaymentInLieuEffect, postWrite)
	return transaction, err
}

// PreviewPaymentInLieu runs the same writer and rolls it back.
func (r *InvestmentRepository) PreviewPaymentInLieu(ctx context.Context, params CreateTransactionParams) (SimulatedInvestmentWrite, error) {
	if !validPaymentInLieuParams(params) || params.Spec.InvestmentOperationKind != "payment_in_lieu" {
		return SimulatedInvestmentWrite{}, fmt.Errorf("%w: payment in lieu is incomplete", ErrInvalidDisposalParams)
	}
	transaction, _, err := previewInvestmentWriteWithGuardTx(ctx, r.database, params, nil, noPaymentInLieuEffect)
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
		current.id, o.event_date, COALESCE(facts.holding_account_id, 0), COALESCE(facts.commodity_id, 0), COALESCE(facts.ex_date, ''),
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
// repeats the original's attribution facts, including unassigned fields.
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
// audit event; the replacement may repair its attribution.
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
		!validPaymentInLieuParams(replacementParams) ||
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
			return checkPaymentInLieuForCorrectionTx(ctx, tx, inverseParams.BookID, expected)
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
