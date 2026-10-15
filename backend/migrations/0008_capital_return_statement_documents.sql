-- +goose Up
-- #238: widen only the saved-document version; preserve exact JSON and hashes.
CREATE TEMP TABLE saved_investment_gains_runs AS SELECT * FROM investment_gains_runs;
DROP TRIGGER investment_gains_runs_no_update;
DROP TRIGGER investment_gains_runs_no_delete;
DROP TABLE investment_gains_runs;
CREATE TABLE investment_gains_runs (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id),
  schema_version INTEGER NOT NULL CHECK (schema_version = 1 OR (schema_version = 2 AND report_type = 'statement')),
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
INSERT INTO investment_gains_runs SELECT * FROM saved_investment_gains_runs;
DROP TABLE saved_investment_gains_runs;
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
-- Schema 2 cannot be downgraded without losing its disclosure contract.
-- +goose StatementBegin
CREATE TEMP TABLE gains_runs_v2_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO gains_runs_v2_downgrade_guard SELECT NOT EXISTS (SELECT 1 FROM investment_gains_runs WHERE schema_version = 2);
DROP TABLE gains_runs_v2_downgrade_guard;
-- +goose StatementEnd
CREATE TEMP TABLE saved_investment_gains_runs AS SELECT * FROM investment_gains_runs;
DROP TRIGGER investment_gains_runs_no_update;
DROP TRIGGER investment_gains_runs_no_delete;
DROP TABLE investment_gains_runs;
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
INSERT INTO investment_gains_runs SELECT * FROM saved_investment_gains_runs;
DROP TABLE saved_investment_gains_runs;
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
