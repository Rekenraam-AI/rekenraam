package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ReturnsRunRecord is a frozen returns_v1 document (R13 slice 6),
// independent of later live reads. Fingerprints reuse GainsRunDigest.
type ReturnsRunRecord struct {
	ID              int64           `json:"id"`
	BookID          int64           `json:"-"`
	SchemaVersion   int             `json:"schema_version"`
	CapturedAt      string          `json:"captured_at"`
	CreatedAt       string          `json:"created_at"`
	CreatedByUserID int64           `json:"created_by_user_id"`
	AuditEventID    int64           `json:"audit_event_id"`
	SnapshotSHA256  string          `json:"snapshot_sha256"`
	InputsSHA256    string          `json:"inputs_sha256"`
	OutputSHA256    string          `json:"output_sha256"`
	InputsJSON      json.RawMessage `json:"inputs"`
	OutputJSON      json.RawMessage `json:"output,omitempty"`
}

// ReturnsRunDescriptor is the part of a saved returns output its inputs
// document repeats: what was read, over which accounts, and when.
type ReturnsRunDescriptor struct {
	Policy             string  `json:"policy"`
	From               string  `json:"from"`
	To                 string  `json:"to"`
	OpeningDate        string  `json:"opening_date"`
	Days               int     `json:"days"`
	AccountScope       string  `json:"account_scope"`
	ResolvedAccountIDs []int64 `json:"resolved_account_ids"`
	KnowledgeCutoff    string  `json:"knowledge_cutoff"`
}

const returnsRunColumns = `id, book_id, schema_version, captured_at, created_at,
 created_by_user_id, audit_event_id, snapshot_sha256, inputs_sha256, output_sha256, inputs_json, output_json`

func scanReturnsRun(scan func(...any) error) (ReturnsRunRecord, error) {
	var r ReturnsRunRecord
	var input, output string
	err := scan(&r.ID, &r.BookID, &r.SchemaVersion, &r.CapturedAt, &r.CreatedAt,
		&r.CreatedByUserID, &r.AuditEventID, &r.SnapshotSHA256, &r.InputsSHA256, &r.OutputSHA256, &input, &output)
	r.InputsJSON, r.OutputJSON = json.RawMessage(input), json.RawMessage(output)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// ValidReturnsRunDocument checks stored bytes, not a recalculation against
// the current book: both digests, the snapshot identity, and a descriptor
// that names returns_v1 at the run's own capture instant and agrees with
// the output it describes.
func ValidReturnsRunDocument(r ReturnsRunRecord) bool {
	var input struct {
		SnapshotSHA256 string               `json:"snapshot_sha256"`
		Descriptor     ReturnsRunDescriptor `json:"descriptor"`
	}
	var output ReturnsRunDescriptor
	if r.SchemaVersion != 1 || GainsRunDigest(r.InputsJSON) != r.InputsSHA256 || GainsRunDigest(r.OutputJSON) != r.OutputSHA256 ||
		json.Unmarshal(r.InputsJSON, &input) != nil || json.Unmarshal(r.OutputJSON, &output) != nil {
		return false
	}
	saved, err := json.Marshal(input.Descriptor)
	if err != nil {
		return false
	}
	described, err := json.Marshal(output)
	if err != nil {
		return false
	}
	return input.SnapshotSHA256 == r.SnapshotSHA256 && input.Descriptor.Policy == "returns_v1" &&
		input.Descriptor.KnowledgeCutoff == r.CapturedAt && bytes.Equal(saved, described)
}

func (r *InvestmentRepository) CreateReturnsRun(ctx context.Context, run ReturnsRunRecord, audit AuditEventParams) (ReturnsRunRecord, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return ReturnsRunRecord{}, err
	}
	defer rollbackTx(ctx, tx)
	audit.Operation = "investment.returns_run.create"
	run.AuditEventID, err = insertAuditEvent(ctx, tx, audit)
	if err != nil {
		return ReturnsRunRecord{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO investment_returns_runs
 (book_id, schema_version, captured_at, created_at, created_by_user_id, audit_event_id,
 snapshot_sha256, inputs_sha256, output_sha256, inputs_json, output_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.BookID, run.SchemaVersion, run.CapturedAt, run.CreatedAt, run.CreatedByUserID, run.AuditEventID,
		run.SnapshotSHA256, run.InputsSHA256, run.OutputSHA256, string(run.InputsJSON), string(run.OutputJSON))
	if err != nil {
		return ReturnsRunRecord{}, fmt.Errorf("insert returns run: %w", err)
	}
	run.ID, err = result.LastInsertId()
	if err != nil {
		return ReturnsRunRecord{}, err
	}
	if err = tx.Commit(); err != nil {
		return ReturnsRunRecord{}, err
	}
	return run, nil
}

func (r *InvestmentRepository) ReturnsRun(ctx context.Context, bookID, id int64) (ReturnsRunRecord, error) {
	return scanReturnsRun(r.database.QueryRowContext(ctx, `SELECT `+returnsRunColumns+` FROM investment_returns_runs WHERE book_id = ? AND id = ?`, bookID, id).Scan)
}

// ListReturnsRuns is a descending ID cursor; output documents are read only
// on selection, keeping the list bounded however large a saved read is.
func (r *InvestmentRepository) ListReturnsRuns(ctx context.Context, bookID, before int64, limit int) ([]ReturnsRunRecord, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT id, book_id, schema_version, captured_at, created_at,
 created_by_user_id, audit_event_id, snapshot_sha256, inputs_sha256, output_sha256, inputs_json, '{}'
 FROM investment_returns_runs WHERE book_id = ? AND (? = 0 OR id < ?) ORDER BY id DESC LIMIT ?`, bookID, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ReturnsRunRecord{}
	for rows.Next() {
		run, err := scanReturnsRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		run.OutputJSON = nil
		result = append(result, run)
	}
	return result, rows.Err()
}

func StreamReturnsRuns(ctx context.Context, tx *sql.Tx, bookID int64, visit func(ReturnsRunRecord) error) error {
	rows, err := tx.QueryContext(ctx, `SELECT `+returnsRunColumns+` FROM investment_returns_runs WHERE book_id = ? ORDER BY id`, bookID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		run, err := scanReturnsRun(rows.Scan)
		if err != nil {
			return err
		}
		if err := visit(run); err != nil {
			return err
		}
	}
	return rows.Err()
}
