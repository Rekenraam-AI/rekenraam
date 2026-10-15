package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// GainsRunRecord is a frozen document, independent of later live projections.
type GainsRunRecord struct {
	ID              int64           `json:"id"`
	BookID          int64           `json:"-"`
	SchemaVersion   int             `json:"schema_version"`
	ReportType      string          `json:"report_type"`
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

const gainsRunColumns = `id, book_id, schema_version, report_type, captured_at, created_at,
 created_by_user_id, audit_event_id, snapshot_sha256, inputs_sha256, output_sha256, inputs_json, output_json`

func scanGainsRun(scan func(...any) error) (GainsRunRecord, error) {
	var r GainsRunRecord
	var input, output string
	err := scan(&r.ID, &r.BookID, &r.SchemaVersion, &r.ReportType, &r.CapturedAt, &r.CreatedAt,
		&r.CreatedByUserID, &r.AuditEventID, &r.SnapshotSHA256, &r.InputsSHA256, &r.OutputSHA256, &input, &output)
	r.InputsJSON, r.OutputJSON = json.RawMessage(input), json.RawMessage(output)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func GainsRunDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ValidGainsRunDocument checks stored bytes, not a recalculation against the
// current book. A fingerprint is an integrity check, not a signature.
func ValidGainsRunDocument(r GainsRunRecord) bool {
	var input struct {
		SnapshotSHA256 string          `json:"snapshot_sha256"`
		ReportType     string          `json:"report_type"`
		Profile        json.RawMessage `json:"profile"`
	}
	var output struct {
		Profile json.RawMessage `json:"profile"`
	}
	var profile struct {
		Name            string `json:"name"`
		KnowledgeCutoff string `json:"knowledge_cutoff"`
	}
	return (r.SchemaVersion == 1 || (r.SchemaVersion == 2 && r.ReportType == "statement")) && (r.ReportType == "gains" || r.ReportType == "statement") &&
		GainsRunDigest(r.InputsJSON) == r.InputsSHA256 && GainsRunDigest(r.OutputJSON) == r.OutputSHA256 &&
		json.Unmarshal(r.InputsJSON, &input) == nil && json.Unmarshal(r.OutputJSON, &output) == nil &&
		input.ReportType == r.ReportType && input.SnapshotSHA256 == r.SnapshotSHA256 &&
		json.Unmarshal(input.Profile, &profile) == nil && profile.Name == "operational_v1" &&
		profile.KnowledgeCutoff == r.CapturedAt && string(input.Profile) == string(output.Profile)
}

// GainsSnapshot copies one consistent SQLite state into a private disposable
// database. Dated replay can then use its existing rolled-back transactions
// without nesting in a live write transaction or observing concurrent changes.
// The digest identifies the full SQLite input image before any replay/open.
func (r *InvestmentRepository) GainsSnapshot(ctx context.Context) (*sql.DB, string, func(), error) {
	return r.gainsSnapshot(ctx, true)
}

// LiveGainsSnapshot uses the same consistent input capture as saved runs,
// without reading the entire image again to fingerprint an unsaved response.
func (r *InvestmentRepository) LiveGainsSnapshot(ctx context.Context) (*sql.DB, func(), error) {
	snapshot, _, cleanup, err := r.gainsSnapshot(ctx, false)
	return snapshot, cleanup, err
}

func (r *InvestmentRepository) gainsSnapshot(ctx context.Context, fingerprint bool) (*sql.DB, string, func(), error) {
	dir, err := os.MkdirTemp("", "rekenraam-gains-")
	if err != nil {
		return nil, "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "snapshot.sqlite")
	if _, err = OnlineBackupSQLiteDatabase(ctx, r.database, path, OnlineBackupOptions{Deadline: 30 * time.Second}); err != nil {
		cleanup()
		return nil, "", nil, err
	}
	var digest string
	if fingerprint {
		file, err := os.Open(path)
		if err != nil {
			cleanup()
			return nil, "", nil, err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if err = errors.Join(copyErr, closeErr); err != nil {
			cleanup()
			return nil, "", nil, err
		}
		digest = hex.EncodeToString(hash.Sum(nil))
	}
	snapshot, err := Open(ctx, sqliteFileURL(path))
	if err != nil {
		cleanup()
		return nil, "", nil, err
	}
	return snapshot, digest, func() { _ = snapshot.Close(); cleanup() }, nil
}

func (r *InvestmentRepository) CreateGainsRun(ctx context.Context, run GainsRunRecord, audit AuditEventParams) (GainsRunRecord, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return GainsRunRecord{}, err
	}
	defer rollbackTx(ctx, tx)
	audit.Operation = "investment.gains_run.create"
	run.AuditEventID, err = insertAuditEvent(ctx, tx, audit)
	if err != nil {
		return GainsRunRecord{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO investment_gains_runs
 (book_id, schema_version, report_type, captured_at, created_at, created_by_user_id, audit_event_id,
 snapshot_sha256, inputs_sha256, output_sha256, inputs_json, output_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.BookID, run.SchemaVersion, run.ReportType, run.CapturedAt, run.CreatedAt, run.CreatedByUserID, run.AuditEventID,
		run.SnapshotSHA256, run.InputsSHA256, run.OutputSHA256, string(run.InputsJSON), string(run.OutputJSON))
	if err != nil {
		return GainsRunRecord{}, fmt.Errorf("insert gains run: %w", err)
	}
	run.ID, err = result.LastInsertId()
	if err != nil {
		return GainsRunRecord{}, err
	}
	if err = tx.Commit(); err != nil {
		return GainsRunRecord{}, err
	}
	return run, nil
}

func (r *InvestmentRepository) GainsRun(ctx context.Context, bookID, id int64) (GainsRunRecord, error) {
	return scanGainsRun(r.database.QueryRowContext(ctx, `SELECT `+gainsRunColumns+` FROM investment_gains_runs WHERE book_id = ? AND id = ?`, bookID, id).Scan)
}

// ListGainsRuns is a descending ID cursor; output documents are read only on
// selection, keeping the list bounded even when a saved report is large.
func (r *InvestmentRepository) ListGainsRuns(ctx context.Context, bookID, before int64, limit int) ([]GainsRunRecord, error) {
	rows, err := r.database.QueryContext(ctx, `SELECT id, book_id, schema_version, report_type, captured_at, created_at,
 created_by_user_id, audit_event_id, snapshot_sha256, inputs_sha256, output_sha256, inputs_json, '{}'
 FROM investment_gains_runs WHERE book_id = ? AND (? = 0 OR id < ?) ORDER BY id DESC LIMIT ?`, bookID, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []GainsRunRecord{}
	for rows.Next() {
		run, err := scanGainsRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		run.OutputJSON = nil
		result = append(result, run)
	}
	return result, rows.Err()
}

func StreamGainsRuns(ctx context.Context, tx *sql.Tx, bookID int64, visit func(GainsRunRecord) error) error {
	rows, err := tx.QueryContext(ctx, `SELECT `+gainsRunColumns+` FROM investment_gains_runs WHERE book_id = ? ORDER BY id`, bookID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		run, err := scanGainsRun(rows.Scan)
		if err != nil {
			return err
		}
		if err := visit(run); err != nil {
			return err
		}
	}
	return rows.Err()
}
