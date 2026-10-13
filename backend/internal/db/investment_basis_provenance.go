package db

import (
	"context"
	"database/sql"
	"fmt"

	"rekenraam/backend/internal/exact"
)

// Provenance revisions amend evidence, never economic operation lineage.
// Revision 1 follows the immutable resolution's original declaration.
type BasisResolutionProvenanceRecord struct {
	ID                    int64
	ResolutionOperationID int64
	RevisionSeq           int
	SourceEvidenceJSON    string
	AuditEventID          int64
	ActorUserID           int64
	RequestID             string
	CreatedAt             string
	Reason                string
}

func (r *InvestmentRepository) correctBasisResolutionProvenance(ctx context.Context, expected BasisResolutionOperationRecord,
	header CreateTransactionParams, params ResolveTransferBasisParams, preview bool,
) (BasisResolutionReplacementRecord, error) {
	if !header.AuditOnly || header.Operation != "investment.basis_resolution.provenance" ||
		header.BookID != params.BookID || header.ActorUserID <= 0 || header.ChangeReason == "" ||
		header.Spec.InvestmentOperationKind != "" || len(header.Spec.JournalEntries) != 0 ||
		params.TransferOperationID != expected.TransferOperationID ||
		exact.ScaledIntFromCoefficient(params.BasisValue, params.BasisScale).Cmp(
			exact.ScaledIntFromCoefficient(expected.BasisValue, expected.BasisScale)) != 0 {
		return BasisResolutionReplacementRecord{}, fmt.Errorf("%w: provenance correction cannot change financial facts", ErrInvalidDisposalParams)
	}
	guard := func(tx *sql.Tx) error {
		return checkBasisResolutionForCorrectionTx(ctx, tx, params.BookID, expected)
	}
	effect := func(tx *sql.Tx, _ TransactionRecord, auditID int64) (struct{}, error) {
		_, err := tx.ExecContext(ctx, `INSERT INTO investment_basis_resolution_provenance
			(book_id, resolution_operation_id, revision_seq, source_evidence_json, created_audit_event_id)
			VALUES (?, ?, ?, ?, ?)`, params.BookID, expected.OperationID, expected.ProvenanceRevision+1,
			params.SourceEvidenceJSON, auditID)
		if err != nil {
			return struct{}{}, fmt.Errorf("record basis resolution provenance: %w", err)
		}
		return struct{}{}, nil
	}
	var journal TransactionRecord
	var err error
	if preview {
		journal, _, err = previewInvestmentWriteWithGuardTx(ctx, r.database, header, guard, effect)
	} else {
		journal, _, err = executeInvestmentWriteWithGuardTx(ctx, r.database, header, guard, effect, nil)
	}
	// Its zero-ID record carries the preview's gain/checkpoint comparison.
	return BasisResolutionReplacementRecord{Replacement: journal}, err
}

// One query composes all evidence revisions for the transfer's history,
// including economically superseded and withdrawn resolutions.
func (r *InvestmentRepository) transferBasisProvenanceHistory(ctx context.Context, bookID, transferOperationID int64) (map[int64][]BasisResolutionProvenanceRecord, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT p.id, p.resolution_operation_id, p.revision_seq,
		p.source_evidence_json, a.id, a.actor_user_id, COALESCE(a.request_id, ''), a.occurred_at, a.reason
		FROM investment_basis_resolution_provenance p
		JOIN investment_basis_resolutions r ON r.operation_id = p.resolution_operation_id
		JOIN audit_events a ON a.id = p.created_audit_event_id
		WHERE p.book_id = ? AND r.transfer_operation_id = ?
		ORDER BY p.resolution_operation_id, p.revision_seq`, bookID, transferOperationID)
	if err != nil {
		return nil, fmt.Errorf("read basis provenance history: %w", err)
	}
	defer rows.Close()
	history := make(map[int64][]BasisResolutionProvenanceRecord)
	for rows.Next() {
		var record BasisResolutionProvenanceRecord
		if err := rows.Scan(&record.ID, &record.ResolutionOperationID, &record.RevisionSeq, &record.SourceEvidenceJSON,
			&record.AuditEventID, &record.ActorUserID, &record.RequestID, &record.CreatedAt, &record.Reason); err != nil {
			return nil, fmt.Errorf("scan basis provenance history: %w", err)
		}
		history[record.ResolutionOperationID] = append(history[record.ResolutionOperationID], record)
	}
	return history, rows.Err()
}
