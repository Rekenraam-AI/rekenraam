package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// A provider split row (Trading 212 STOCK_SPLIT) carries no verified ratio or
// entitlement, so it never posts. The user records the split manually and
// links the staged row to it: the row's dedupe identity then names the
// existing split operation, so neither a retry nor a re-fetch can post it
// again (T-122).

// ErrSplitLinkUnavailable means the staged row or split can no longer be
// linked: the row was committed, its batch discarded, or it is not a provider
// split, or the split is not effective, belongs to another security, or is
// already linked.
var ErrSplitLinkUnavailable = errors.New("staged split row cannot be linked to this split")

// SplitLinkCandidate is an effective recorded split of a security, with the
// staged-row identity already linked to it, if any.
type SplitLinkCandidate struct {
	OperationID      int64
	TransactionID    int64
	HoldingAccountID int64
	EffectiveOn      string
	RatioNumerator   int64
	RatioDenominator int64
	Linked           bool
}

// ListSplitLinkCandidates returns the effective splits of one security, newest
// first, marking those already linked to a committed source row.
func (r *ImportRepository) ListSplitLinkCandidates(ctx context.Context, bookID, commodityID int64) ([]SplitLinkCandidate, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT f.operation_id, COALESCE(v.transaction_id, 0), f.account_id, f.effective_on,
			f.ratio_numerator, f.ratio_denominator,
			EXISTS (SELECT 1 FROM import_commit_identity_effects e WHERE e.operation_id = f.operation_id)
		FROM investment_split_facts f
		JOIN effective_investment_operations o ON o.id = f.operation_id
		LEFT JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.role = 'primary'
		LEFT JOIN transaction_versions v ON v.id = link.transaction_version_id
		WHERE f.book_id = ? AND f.commodity_id = ?
		ORDER BY f.effective_on DESC, f.operation_id DESC`, bookID, commodityID)
	if err != nil {
		return nil, fmt.Errorf("read split link candidates: %w", err)
	}
	defer rows.Close()
	var candidates []SplitLinkCandidate
	for rows.Next() {
		var candidate SplitLinkCandidate
		if err := rows.Scan(&candidate.OperationID, &candidate.TransactionID, &candidate.HoldingAccountID,
			&candidate.EffectiveOn, &candidate.RatioNumerator, &candidate.RatioDenominator, &candidate.Linked); err != nil {
			return nil, fmt.Errorf("scan split link candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate split link candidates: %w", err)
	}
	return candidates, nil
}

// LinkStagedRowToSplitParams names the staged provider row and the recorded
// split it evidences. CommodityID is the security the row resolved to.
type LinkStagedRowToSplitParams struct {
	BookID            int64
	BatchID           int64
	RowID             int64
	DedupeFingerprint string
	SourceKind        string
	OperationID       int64
	CommodityID       int64
	ActorUserID       int64
	AuthSessionID     int64
	RequestID         string
	Now               string
}

// LinkStagedRowToSplit admits the row's dedupe identity with the split
// operation as its only effect and marks the row committed, under one audit
// event. No journal or lot row is written, so nothing can post twice.
func (r *ImportRepository) LinkStagedRowToSplit(ctx context.Context, params LinkStagedRowToSplitParams) (int64, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin split link: %w", err)
	}
	defer rollbackTx(ctx, tx)
	row := stagedRowLink{BookID: params.BookID, BatchID: params.BatchID, RowID: params.RowID,
		DedupeFingerprint: params.DedupeFingerprint, SourceKind: params.SourceKind}
	if err := checkLinkableStagedRowTx(ctx, tx, row); err != nil {
		if errors.Is(err, errStagedRowNotLinkable) {
			return 0, ErrSplitLinkUnavailable
		}
		return 0, err
	}
	var accountID, transactionID int64
	err = tx.QueryRowContext(ctx, `
		SELECT f.account_id, COALESCE(v.transaction_id, 0) FROM investment_split_facts f
		JOIN effective_investment_operations o ON o.id = f.operation_id
		LEFT JOIN investment_operation_journal_links link ON link.operation_id = o.id AND link.role = 'primary'
		LEFT JOIN transaction_versions v ON v.id = link.transaction_version_id
		WHERE f.operation_id = ? AND f.book_id = ? AND f.commodity_id = ?
			AND NOT EXISTS (SELECT 1 FROM import_commit_identity_effects e WHERE e.operation_id = o.id)`,
		params.OperationID, params.BookID, params.CommodityID).Scan(&accountID, &transactionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrSplitLinkUnavailable
	}
	if err != nil {
		return 0, fmt.Errorf("read split to link: %w", err)
	}
	identityID, err := r.admitStagedRowAsOperationTx(ctx, tx, row, operationLinkAdmission{
		OperationID: params.OperationID, TransactionID: transactionID, AccountID: accountID,
		AuditOperation: "import.row.link_split", ActorUserID: params.ActorUserID,
		AuthSessionID: params.AuthSessionID, RequestID: params.RequestID, Now: params.Now,
	})
	if errors.Is(err, errStagedRowNotLinkable) {
		return 0, ErrSplitLinkUnavailable
	}
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit split link: %w", err)
	}
	return identityID, nil
}

// errStagedRowNotLinkable is the shared refusal of the owner-asserted
// operation links; each family restates it under its own sentinel.
var errStagedRowNotLinkable = errors.New("staged row cannot be linked")

// stagedRowLink names a staged provider row as the caller prepared it.
type stagedRowLink struct {
	BookID            int64
	BatchID           int64
	RowID             int64
	DedupeFingerprint string
	SourceKind        string
}

// checkLinkableStagedRowTx re-reads the row and its batch under the link's
// write transaction, not trusting the caller's preparation: a discard or a
// commit between the two must not admit source evidence (T-139).
func checkLinkableStagedRowTx(ctx context.Context, tx *sql.Tx, row stagedRowLink) error {
	var batchID int64
	var commitStatus, fingerprint, batchStatus, sourceKind string
	err := tx.QueryRowContext(ctx, `SELECT r.batch_id, r.commit_status, r.dedupe_fingerprint, b.status, b.source_kind
		FROM import_staged_rows r JOIN import_batches b ON b.id = r.batch_id AND b.book_id = r.book_id
		WHERE r.id = ? AND r.book_id = ?`, row.RowID, row.BookID).
		Scan(&batchID, &commitStatus, &fingerprint, &batchStatus, &sourceKind)
	if errors.Is(err, sql.ErrNoRows) {
		return errStagedRowNotLinkable
	}
	if err != nil {
		return fmt.Errorf("read staged row to link: %w", err)
	}
	if batchID != row.BatchID || commitStatus == "committed" || fingerprint != row.DedupeFingerprint ||
		batchStatus == "discarded" || batchStatus == "rolled_back" || sourceKind != row.SourceKind {
		return errStagedRowNotLinkable
	}
	return nil
}

type operationLinkAdmission struct {
	OperationID    int64
	TransactionID  int64
	AccountID      int64
	AuditOperation string
	ActorUserID    int64
	AuthSessionID  int64
	RequestID      string
	Now            string
}

// admitStagedRowAsOperationTx records one audit event, admits the row's
// dedupe identity with the existing operation as its only effect and marks
// the row committed. It writes no journal, lot or disposal. An identity or
// operation already owned by another source row refuses the whole link.
func (r *ImportRepository) admitStagedRowAsOperationTx(ctx context.Context, tx *sql.Tx, row stagedRowLink, admission operationLinkAdmission) (int64, error) {
	if _, err := insertAuditEvent(ctx, tx, AuditEventParams{
		BookID: row.BookID, ActorUserID: admission.ActorUserID, AuthSessionID: admission.AuthSessionID,
		OccurredAt: admission.Now, RequestID: admission.RequestID, OriginType: "import",
		Operation: admission.AuditOperation,
		MetadataJSON: fmt.Sprintf(`{"batch_id":%d,"row_id":%d,"operation_id":%d}`,
			row.BatchID, row.RowID, admission.OperationID),
	}); err != nil {
		return 0, err
	}
	transactionID := admission.TransactionID
	identityID, err := r.CreateCommitIdentityWithEffects(ctx, tx, CreateImportCommitIdentityParams{
		BookID: row.BookID, DedupeFingerprint: row.DedupeFingerprint,
		CommittedTransactionID: transactionID, SourceKind: row.SourceKind,
		AccountID: admission.AccountID, CreatedAt: admission.Now,
	}, []CreateImportCommitEffectParams{{
		OperationID:   sql.NullInt64{Int64: admission.OperationID, Valid: true},
		TransactionID: sql.NullInt64{Int64: transactionID, Valid: transactionID > 0},
	}})
	// The unique effect index is the last word on a concurrent link of the
	// same operation from another row.
	if errors.Is(err, ErrCommitIdentityConflict) || IsSQLiteUniqueConstraint(err) {
		return 0, errStagedRowNotLinkable
	}
	if err != nil {
		return 0, err
	}
	if err := commitImportStagedRowExec(ctx, tx, CommitImportStagedRowParams{
		RowID: row.RowID, CommitStatus: "committed",
		CommittedIdentityID:    sql.NullInt64{Int64: identityID, Valid: true},
		CommittedTransactionID: sql.NullInt64{Int64: transactionID, Valid: transactionID > 0},
	}); err != nil {
		if errors.Is(err, ErrImportStagedRowAlreadyCommitted) {
			return 0, errStagedRowNotLinkable
		}
		return 0, err
	}
	return identityID, nil
}
