-- +goose Up
CREATE TABLE investment_gains_runs (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id),
  schema_version INTEGER NOT NULL CHECK (schema_version = 1),
  report_type TEXT NOT NULL CHECK (report_type IN ('gains', 'statement')),
  captured_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  created_by_user_id INTEGER NOT NULL REFERENCES users(id),
  audit_event_id INTEGER NOT NULL REFERENCES audit_events(id),
  snapshot_sha256 TEXT NOT NULL CHECK (length(snapshot_sha256) = 64 AND snapshot_sha256 NOT GLOB '*[^0-9a-f]*' AND instr(snapshot_sha256, char(0)) = 0),
  inputs_sha256 TEXT NOT NULL CHECK (length(inputs_sha256) = 64 AND inputs_sha256 NOT GLOB '*[^0-9a-f]*' AND instr(inputs_sha256, char(0)) = 0),
  output_sha256 TEXT NOT NULL CHECK (length(output_sha256) = 64 AND output_sha256 NOT GLOB '*[^0-9a-f]*' AND instr(output_sha256, char(0)) = 0),
  inputs_json TEXT NOT NULL CHECK (json_valid(inputs_json) AND json_type(inputs_json) = 'object'),
  output_json TEXT NOT NULL CHECK (json_valid(output_json) AND json_type(output_json) = 'object')
);
CREATE INDEX investment_gains_runs_book ON investment_gains_runs(book_id, id DESC);
-- +goose StatementBegin
CREATE TRIGGER investment_gains_runs_no_update BEFORE UPDATE ON investment_gains_runs BEGIN
  SELECT RAISE(ABORT, 'investment gains runs are immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_gains_runs_no_delete BEFORE DELETE ON investment_gains_runs BEGIN
  SELECT RAISE(ABORT, 'investment gains runs are immutable');
END;
-- +goose StatementEnd

-- +goose Down
-- Refuse loss of saved reports; restore a verified backup to roll back populated runs.
-- +goose StatementBegin
CREATE TEMP TABLE gains_runs_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO gains_runs_downgrade_guard SELECT NOT EXISTS (SELECT 1 FROM investment_gains_runs);
DROP TABLE gains_runs_downgrade_guard;
-- +goose StatementEnd
DROP TABLE investment_gains_runs;
