package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"rekenraam/backend/internal/exact"
)

// Cash-and-stock merger correction (#184), following the share exchange
// correction (#179) and ADR 0013 *Transfer Correction Refinement*. A reversal
// posts the exact inverse of the whole merger journal (the security legs and
// the cash leg), removes the merger and its cash decision from effective
// history and replays the old holding, whose lots return with their basis,
// then the new one, which loses the lots the merger opened. A replacement does
// the same and records corrected terms at the replaced merger's
// correction-root slot. The original journal, fact, links, realizations,
// decision, link and decision revisions stay immutable evidence; every effect
// commits under one audit event.

// ErrCashStockMergerOperationChanged means the merger or its journal moved
// after its correction was planned.
var ErrCashStockMergerOperationChanged = errors.New("cash-and-stock merger changed after its correction was prepared")

// CashStockMergerOperationRecord pins a posted merger, the holdings it moved
// and its recorded terms. A correction rechecks it inside the write.
type CashStockMergerOperationRecord struct {
	OperationID            int64
	TransactionID          int64
	TransactionVersionID   int64
	CurrentVersionID       int64
	EventDate              string
	PayableOn              string
	AccountID              int64
	DestinationAccountID   int64
	CommodityID            int64
	DestinationCommodityID int64
	RatioNumerator         int64
	RatioDenominator       int64
	CashAccountID          int64
	CashCommodityID        int64
	CashValue              exact.Coefficient
	CashScale              int
	CashBasisFractionValue exact.Coefficient
	CashBasisFractionScale int
	SourceEvidenceJSON     string
	AlreadyCorrected       bool
	ImportedLineage        bool
}

func (r *InvestmentRepository) CashStockMergerOperationByTransactionID(ctx context.Context, bookID, transactionID int64) (CashStockMergerOperationRecord, error) {
	return cashStockMergerOperationByTransactionIDQuery(ctx, r.database, bookID, transactionID)
}

// cashStockMergerOperationByTransactionIDQuery reads the merger's terms. The
// cash account is the first line of the journal's second (cash) entry, and the
// cash is its decision's proceeds.
func cashStockMergerOperationByTransactionIDQuery(ctx context.Context, reader transferOperationReader, bookID, transactionID int64) (CashStockMergerOperationRecord, error) {
	var record CashStockMergerOperationRecord
	var corrected int
	err := reader.QueryRowContext(ctx, `SELECT o.id, linked_version.transaction_id, link.transaction_version_id,
		current.id, f.effective_on, payable.event_date, f.source_account_id, f.destination_account_id, f.commodity_id,
		f.destination_commodity_id, f.ratio_numerator, f.ratio_denominator, cash.account_id,
		decision.cost_commodity_id, decision.proceeds_value, decision.proceeds_scale,
		f.cash_basis_fraction_value, f.cash_basis_fraction_scale, f.source_evidence_json,
		EXISTS(SELECT 1 FROM investment_operations successor WHERE successor.correction_of_operation_id = o.id)
		FROM investment_operations o
		JOIN investment_transfer_facts f ON f.operation_id = o.id AND f.book_id = o.book_id AND f.transfer_kind = 'exchange'
			AND f.cash_basis_fraction_value IS NOT NULL
		JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.book_id = o.book_id AND link.role = 'primary'
		JOIN transaction_versions linked_version ON linked_version.id = link.transaction_version_id
		JOIN current_transaction_versions current ON current.transaction_id = linked_version.transaction_id
		JOIN investment_operation_dates payable ON payable.operation_id = o.id AND payable.date_role = 'payable' AND payable.date_seq = 1
		JOIN investment_disposal_decisions decision ON decision.operation_id = o.id
		JOIN journal_entries cash_entry ON cash_entry.transaction_version_id = link.transaction_version_id AND cash_entry.entry_seq = 2
		JOIN posting_versions cash ON cash.journal_entry_id = cash_entry.id AND cash.line_seq = 1
		WHERE o.book_id = ? AND linked_version.transaction_id = ? AND o.operation_kind = 'cash_and_stock_merger'`,
		bookID, transactionID).Scan(&record.OperationID, &record.TransactionID, &record.TransactionVersionID,
		&record.CurrentVersionID, &record.EventDate, &record.PayableOn, &record.AccountID, &record.DestinationAccountID,
		&record.CommodityID, &record.DestinationCommodityID, &record.RatioNumerator, &record.RatioDenominator,
		&record.CashAccountID, &record.CashCommodityID, &record.CashValue, &record.CashScale,
		&record.CashBasisFractionValue, &record.CashBasisFractionScale, &record.SourceEvidenceJSON, &corrected)
	if errors.Is(err, sql.ErrNoRows) {
		return CashStockMergerOperationRecord{}, ErrNotFound
	}
	if err != nil {
		return CashStockMergerOperationRecord{}, fmt.Errorf("read cash-and-stock merger operation: %w", err)
	}
	record.AlreadyCorrected = corrected != 0
	if record.ImportedLineage, err = investmentOperationHasImportedLineageQuery(ctx, reader, bookID, record.OperationID); err != nil {
		return CashStockMergerOperationRecord{}, err
	}
	return record, nil
}

// checkCashStockMergerForCorrectionTx applies the shared correction fences:
// not already corrected, an imported lineage still names its committed
// source, no effective cash in lieu settles it, and neither the merger nor its
// journal moved since it was planned.
func checkCashStockMergerForCorrectionTx(ctx context.Context, tx *sql.Tx, bookID int64, expected CashStockMergerOperationRecord) error {
	current, err := cashStockMergerOperationByTransactionIDQuery(ctx, tx, bookID, expected.TransactionID)
	if err != nil {
		return err
	}
	if current.AlreadyCorrected {
		return ErrInvestmentOperationAlreadyCorrected
	}
	if current.ImportedLineage {
		linked, err := investmentOperationHasCommittedSourceQuery(ctx, tx, bookID, current.OperationID)
		if err != nil {
			return err
		}
		if !linked {
			return ErrInvestmentImportedCorrection
		}
	}
	if current != expected {
		return ErrCashStockMergerOperationChanged
	}
	if err := checkInvestmentSourceJournalTx(ctx, tx, bookID, current.TransactionID,
		current.EventDate, current.TransactionVersionID, current.CurrentVersionID); err != nil {
		if errors.Is(err, ErrInvestmentSaleChanged) {
			return ErrCashStockMergerOperationChanged
		}
		return err
	}
	return nil
}

// holdings lists the old holding, then the new one.
func (e CashStockMergerOperationRecord) holdings() [][2]int64 {
	return [][2]int64{{e.AccountID, e.CommodityID}, {e.DestinationAccountID, e.DestinationCommodityID}}
}

// ReverseCashStockMerger posts the inverse journal and removes the merger and
// its cash decision from effective history, replaying both holdings under one
// audit event.
func (r *InvestmentRepository) ReverseCashStockMerger(ctx context.Context, params CreateTransactionParams, expected CashStockMergerOperationRecord) (TransactionRecord, error) {
	return r.reverseCashStockMerger(ctx, params, expected, false)
}

// PreviewCashStockMergerReversal runs the reversal writer, both replays, gain
// comparison and checkpoint invalidation, then rolls back.
func (r *InvestmentRepository) PreviewCashStockMergerReversal(ctx context.Context, params CreateTransactionParams, expected CashStockMergerOperationRecord) (SimulatedInvestmentWrite, error) {
	transaction, err := r.reverseCashStockMerger(ctx, params, expected, true)
	if err != nil {
		return SimulatedInvestmentWrite{}, err
	}
	return simulatedInvestmentWrite(transaction), nil
}

func (r *InvestmentRepository) reverseCashStockMerger(ctx context.Context, params CreateTransactionParams, expected CashStockMergerOperationRecord, preview bool) (TransactionRecord, error) {
	if params.BookID <= 0 || params.ActorUserID <= 0 || expected.OperationID <= 0 ||
		params.Spec.InvestmentOperationKind != "reversal" || params.Spec.TransactionKind != "investment" ||
		params.Spec.Status != "posted" || params.Spec.TransactionDate != expected.EventDate ||
		params.InvestmentCorrectionOfOperationID != expected.OperationID ||
		params.InvestmentCorrectionMode != "reverse" || params.InvestmentCorrectionReason == "" ||
		!params.CorrectionOfTransactionID.Valid || params.CorrectionOfTransactionID.Int64 != expected.TransactionID {
		return TransactionRecord{}, fmt.Errorf("%w: cash-and-stock merger reversal is incomplete", ErrInvalidDisposalParams)
	}
	guard := func(tx *sql.Tx) error {
		return checkCashStockMergerForCorrectionTx(ctx, tx, params.BookID, expected)
	}
	effect := func(tx *sql.Tx, transaction TransactionRecord, auditEventID int64) (struct{}, error) {
		operationID, err := investmentOperationIDTx(ctx, tx, params.BookID, transaction.ID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, replayHoldingsTx(ctx, tx, params.BookID, expected.holdings(),
			operationID, auditEventID, params.ActorUserID, params.CreatedAt)
	}
	var transaction TransactionRecord
	var err error
	if preview {
		transaction, _, err = previewInvestmentWriteWithGuardTx(ctx, r.database, params, guard, effect)
	} else {
		transaction, _, err = executeInvestmentWriteWithGuardTx(ctx, r.database, params, guard, effect, nil)
	}
	return transaction, err
}

// CashStockMergerReplacementRecord is the inverse and replacement journals
// and what the replacement merger moved and realized.
type CashStockMergerReplacementRecord struct {
	Inverse     TransactionRecord
	Replacement TransactionRecord
	Plan        CashStockMergerPlan
}

// ReplaceCashStockMerger reverses a merger and records corrected terms as its
// successor at the correction root's same-day slot, replaying every holding
// either merger moved under one audit event.
func (r *InvestmentRepository) ReplaceCashStockMerger(ctx context.Context, expected CashStockMergerOperationRecord,
	inverseParams, replacementParams CreateTransactionParams, merger CreateCashStockMergerParams,
) (CashStockMergerReplacementRecord, error) {
	return r.replaceCashStockMerger(ctx, expected, inverseParams, replacementParams, merger, false)
}

// SimulateCashStockMergerReplacement runs the replacement writer and every
// replay, then rolls back. Destination lot IDs are cleared because they were
// never durable.
func (r *InvestmentRepository) SimulateCashStockMergerReplacement(ctx context.Context, expected CashStockMergerOperationRecord,
	inverseParams, replacementParams CreateTransactionParams, merger CreateCashStockMergerParams,
) (SimulatedInvestmentWrite, CashStockMergerPlan, error) {
	record, err := r.replaceCashStockMerger(ctx, expected, inverseParams, replacementParams, merger, true)
	if err != nil {
		return SimulatedInvestmentWrite{}, CashStockMergerPlan{}, err
	}
	for index := range record.Plan.Links {
		record.Plan.Links[index].DestinationLotID = 0
	}
	return simulatedInvestmentWrite(record.Inverse, record.Replacement), record.Plan, nil
}

func (r *InvestmentRepository) replaceCashStockMerger(ctx context.Context, expected CashStockMergerOperationRecord,
	inverseParams, replacementParams CreateTransactionParams, merger CreateCashStockMergerParams, preview bool,
) (CashStockMergerReplacementRecord, error) {
	exchange := merger.Exchange
	if expected.OperationID <= 0 || inverseParams.BookID <= 0 ||
		inverseParams.BookID != replacementParams.BookID || inverseParams.BookID != exchange.BookID ||
		inverseParams.ActorUserID != replacementParams.ActorUserID ||
		inverseParams.Spec.InvestmentOperationKind != "" ||
		inverseParams.Spec.TransactionKind != "investment" || inverseParams.Spec.Status != "posted" ||
		inverseParams.Spec.TransactionDate != expected.EventDate ||
		replacementParams.Spec.InvestmentOperationKind != "cash_and_stock_merger" ||
		replacementParams.Spec.TransactionKind != "investment" || replacementParams.Spec.Status != "posted" ||
		replacementParams.Spec.TransactionDate != exchange.EffectiveOn ||
		exchange.AccountID != expected.AccountID || exchange.CommodityID != expected.CommodityID ||
		exchange.ReplacesOperationID != expected.OperationID ||
		replacementParams.InvestmentCorrectionOfOperationID != expected.OperationID ||
		replacementParams.InvestmentCorrectionMode != "replace" || replacementParams.InvestmentCorrectionReason == "" ||
		!inverseParams.CorrectionOfTransactionID.Valid || inverseParams.CorrectionOfTransactionID.Int64 != expected.TransactionID ||
		!replacementParams.CorrectionOfTransactionID.Valid || replacementParams.CorrectionOfTransactionID.Int64 != expected.TransactionID {
		return CashStockMergerReplacementRecord{}, fmt.Errorf("%w: cash-and-stock merger replacement is incomplete", ErrInvalidDisposalParams)
	}
	write := executeInvestmentJournalsWithGuardTx[CashStockMergerPlan]
	if preview {
		write = previewInvestmentJournalsWithGuardTx[CashStockMergerPlan]
	}
	journals, plan, err := write(ctx, r.database,
		[]CreateTransactionParams{inverseParams, replacementParams},
		func(tx *sql.Tx) error {
			return checkCashStockMergerForCorrectionTx(ctx, tx, inverseParams.BookID, expected)
		}, func(tx *sql.Tx, journals []TransactionRecord, auditEventID int64) (CashStockMergerPlan, error) {
			inverse, replacement := journals[0], journals[1]
			operationID, err := investmentOperationIDTx(ctx, tx, exchange.BookID, replacement.ID)
			if err != nil {
				return CashStockMergerPlan{}, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO investment_operation_journal_links
				(book_id, operation_id, transaction_version_id, link_seq, role)
				VALUES (?, ?, ?, 2, 'reversal')`, exchange.BookID, operationID, inverse.VersionID); err != nil {
				return CashStockMergerPlan{}, fmt.Errorf("link cash-and-stock merger replacement inverse: %w", err)
			}
			// The writer replays the old holding and the replacement's new
			// holding; a replaced new holding elsewhere loses its lots here.
			plan, err := writeCashStockMergerTx(ctx, tx, merger, replacement, replacementParams, auditEventID)
			if err != nil {
				return CashStockMergerPlan{}, err
			}
			replaced := [2]int64{expected.DestinationAccountID, expected.DestinationCommodityID}
			if replaced == [2]int64{exchange.DestinationAccountID, exchange.DestinationCommodityID} {
				return plan, nil
			}
			return plan, replayHoldingsTx(ctx, tx, exchange.BookID, [][2]int64{replaced},
				operationID, auditEventID, replacementParams.ActorUserID, replacementParams.CreatedAt)
		}, nil)
	if err != nil {
		return CashStockMergerReplacementRecord{}, err
	}
	return CashStockMergerReplacementRecord{Inverse: journals[0], Replacement: journals[1], Plan: plan}, nil
}

// CashStockMergerTerms are a committed merger's terms and effective links
// (#184): replay may since have revised a link's carried basis and the cash
// decision's realized basis per lot, which the latest revisions supply.
type CashStockMergerTerms struct {
	Exchange               ShareExchangeTerms
	PayableOn              string
	CashAccountID          int64
	CashCommodityID        int64
	CashValue              exact.Coefficient
	CashScale              int
	CashBasisFractionValue exact.Coefficient
	CashBasisFractionScale int
	Links                  []CashStockMergerLink
}

// CashStockMergerTermsByTransactionID reads a merger for the correction chain.
// The decision allocates the cash in link order, so allocation N is link N's
// realization.
func (r *InvestmentRepository) CashStockMergerTermsByTransactionID(ctx context.Context, bookID, transactionID int64) (CashStockMergerTerms, error) {
	record, err := cashStockMergerOperationByTransactionIDQuery(ctx, r.database, bookID, transactionID)
	if err != nil {
		return CashStockMergerTerms{}, err
	}
	exchange, err := r.ShareExchangeTermsByOperation(ctx, bookID, record.OperationID)
	if err != nil {
		return CashStockMergerTerms{}, err
	}
	terms := CashStockMergerTerms{Exchange: exchange, PayableOn: record.PayableOn, CashAccountID: record.CashAccountID,
		CashCommodityID: record.CashCommodityID, CashValue: record.CashValue, CashScale: record.CashScale,
		CashBasisFractionValue: record.CashBasisFractionValue, CashBasisFractionScale: record.CashBasisFractionScale}
	rows, err := r.database.QueryContext(ctx, `
		SELECT a.allocation_seq, a.cost_basis_value, a.cost_basis_scale, a.proceeds_value, a.proceeds_scale, a.basis_knowledge
		FROM investment_disposal_decisions d
		JOIN investment_disposal_allocations a ON a.decision_id = d.id
		WHERE d.operation_id = ? AND NOT EXISTS (SELECT 1 FROM latest_investment_disposal_revisions r WHERE r.decision_id = d.id)
		UNION ALL
		SELECT a.allocation_seq, a.cost_basis_value, a.cost_basis_scale, a.proceeds_value, a.proceeds_scale, a.basis_knowledge
		FROM investment_disposal_decisions d
		JOIN latest_investment_disposal_revisions r ON r.decision_id = d.id
		JOIN investment_disposal_revision_allocations a ON a.revision_id = r.id
		WHERE d.operation_id = ?
		ORDER BY 1`, record.OperationID, record.OperationID)
	if err != nil {
		return CashStockMergerTerms{}, fmt.Errorf("read cash-and-stock merger realizations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var seq, proceedsScale int
		var basis, basisScale sql.NullInt64
		var proceeds exact.Coefficient
		var knowledge string
		if err := rows.Scan(&seq, &basis, &basisScale, &proceeds, &proceedsScale, &knowledge); err != nil {
			return CashStockMergerTerms{}, fmt.Errorf("scan cash-and-stock merger realization: %w", err)
		}
		if seq != len(terms.Links)+1 || seq > len(exchange.Plan.Links) {
			return CashStockMergerTerms{}, fmt.Errorf("cash-and-stock merger %d realization %d has no link", record.OperationID, seq)
		}
		link := CashStockMergerLink{ShareExchangeLink: exchange.Plan.Links[seq-1], CashScale: proceedsScale}
		if link.CashValue, err = exact.ScaledIntFromCoefficient(proceeds, proceedsScale).Int64(); err != nil {
			return CashStockMergerTerms{}, ErrInvestmentBasisRange
		}
		if normalizedBasisKnowledge(knowledge) == InvestmentBasisKnown {
			if !basis.Valid || !basisScale.Valid {
				return CashStockMergerTerms{}, fmt.Errorf("cash-and-stock merger realization %d has a known basis without an amount", seq)
			}
			link.RealizedBasisValue, link.RealizedBasisScale = basis.Int64, int(basisScale.Int64)
		}
		terms.Links = append(terms.Links, link)
	}
	if err := rows.Err(); err != nil {
		return CashStockMergerTerms{}, fmt.Errorf("read cash-and-stock merger realizations: %w", err)
	}
	if len(terms.Links) != len(exchange.Plan.Links) {
		return CashStockMergerTerms{}, fmt.Errorf("cash-and-stock merger %d realizes %d of %d links", record.OperationID,
			len(terms.Links), len(exchange.Plan.Links))
	}
	return terms, nil
}
