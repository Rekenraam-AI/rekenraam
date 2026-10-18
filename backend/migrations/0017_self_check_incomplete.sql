-- +goose Up
-- #313: a completed self-check can be incomplete — every evaluable integrity
-- check succeeded, but a named verification (supported unknown investment
-- basis) is unavailable. Unavailable scope is counted separately from
-- integrity findings, so a failed check can disclose both
-- (docs/plans/data-portability-plan.md, "Self-check outcomes").
--
-- SQLite cannot widen a CHECK in place. Both tables are rebuilt; results are
-- copied first and the old child is dropped before its parent, so the
-- parent's ON DELETE CASCADE has nothing to remove. Renaming the new parent
-- rewrites the new child's foreign key. Historical rows keep their recorded
-- verdicts: nothing is reinterpreted, and their unavailable counts are zero
-- because the run that wrote them did not measure that separately.
CREATE TABLE self_check_runs_v17 (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  trigger TEXT NOT NULL CHECK (trigger IN ('manual', 'scheduled')),
  status TEXT NOT NULL CHECK (status IN ('running', 'passed', 'incomplete', 'failed', 'errored')),
  failed_check_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_check_count >= 0),
  -- Checks whose verification is partly unavailable, whether or not they also
  -- failed. An incomplete run has none failed and at least one incomplete.
  incomplete_check_count INTEGER NOT NULL DEFAULT 0 CHECK (incomplete_check_count >= 0),
  error_summary TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  finished_at TEXT,
  created_at TEXT NOT NULL,
  CHECK (status <> 'incomplete' OR (failed_check_count = 0 AND incomplete_check_count > 0)),
  CHECK (status <> 'passed' OR (failed_check_count = 0 AND incomplete_check_count = 0))
);

INSERT INTO self_check_runs_v17 (id, book_id, trigger, status, failed_check_count, error_summary, started_at, finished_at, created_at)
SELECT id, book_id, trigger, status, failed_check_count, error_summary, started_at, finished_at, created_at
FROM self_check_runs;

CREATE TABLE self_check_results_v17 (
  id INTEGER PRIMARY KEY,
  run_id INTEGER NOT NULL REFERENCES self_check_runs_v17(id) ON DELETE CASCADE,
  check_id TEXT NOT NULL CHECK (length(trim(check_id)) > 0),
  status TEXT NOT NULL CHECK (status IN ('passed', 'incomplete', 'failed', 'not_applicable')),
  -- Integrity findings: things that are wrong.
  finding_count INTEGER NOT NULL DEFAULT 0 CHECK (finding_count >= 0),
  -- A capped sample of offending identifiers, for a human to go and look at.
  -- Diagnostic display only: never joined on, never counted from.
  sample_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(sample_json)),
  -- Unavailable coverage: things that could not be verified because the
  -- information is unknown, with its own capped sample.
  unavailable_count INTEGER NOT NULL DEFAULT 0 CHECK (unavailable_count >= 0),
  unavailable_sample_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(unavailable_sample_json)),
  summary TEXT NOT NULL DEFAULT '',
  UNIQUE (run_id, check_id),
  CHECK (status <> 'incomplete' OR (finding_count = 0 AND unavailable_count > 0)),
  CHECK (unavailable_count = 0 OR status IN ('incomplete', 'failed'))
);

INSERT INTO self_check_results_v17 (id, run_id, check_id, status, finding_count, sample_json, summary)
SELECT id, run_id, check_id, status, finding_count, sample_json, summary
FROM self_check_results;

DROP INDEX self_check_results_check_idx;
DROP TABLE self_check_results;
DROP INDEX self_check_runs_book_started_idx;
DROP TABLE self_check_runs;
ALTER TABLE self_check_runs_v17 RENAME TO self_check_runs;
ALTER TABLE self_check_results_v17 RENAME TO self_check_results;

CREATE INDEX self_check_runs_book_started_idx
  ON self_check_runs (book_id, started_at DESC, id DESC);
CREATE INDEX self_check_results_check_idx
  ON self_check_results (check_id, status);

-- +goose Down
-- The older schema cannot hold an incomplete verdict, and rewriting one into
-- passed or failed would restate history, so a downgrade is refused once any
-- exists.
CREATE TEMP TABLE self_check_incomplete_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO self_check_incomplete_downgrade_guard
SELECT NOT EXISTS (SELECT 1 FROM self_check_runs WHERE status = 'incomplete')
  AND NOT EXISTS (SELECT 1 FROM self_check_results WHERE status = 'incomplete' OR unavailable_count > 0);
DROP TABLE self_check_incomplete_downgrade_guard;

CREATE TABLE self_check_runs_v16 (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  trigger TEXT NOT NULL CHECK (trigger IN ('manual', 'scheduled')),
  status TEXT NOT NULL CHECK (status IN ('running', 'passed', 'failed', 'errored')),
  failed_check_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_check_count >= 0),
  error_summary TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  finished_at TEXT,
  created_at TEXT NOT NULL
);

INSERT INTO self_check_runs_v16 (id, book_id, trigger, status, failed_check_count, error_summary, started_at, finished_at, created_at)
SELECT id, book_id, trigger, status, failed_check_count, error_summary, started_at, finished_at, created_at
FROM self_check_runs;

CREATE TABLE self_check_results_v16 (
  id INTEGER PRIMARY KEY,
  run_id INTEGER NOT NULL REFERENCES self_check_runs_v16(id) ON DELETE CASCADE,
  check_id TEXT NOT NULL CHECK (length(trim(check_id)) > 0),
  status TEXT NOT NULL CHECK (status IN ('passed', 'failed', 'not_applicable')),
  finding_count INTEGER NOT NULL DEFAULT 0 CHECK (finding_count >= 0),
  sample_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(sample_json)),
  summary TEXT NOT NULL DEFAULT '',
  UNIQUE (run_id, check_id)
);

INSERT INTO self_check_results_v16 (id, run_id, check_id, status, finding_count, sample_json, summary)
SELECT id, run_id, check_id, status, finding_count, sample_json, summary
FROM self_check_results;

DROP INDEX self_check_results_check_idx;
DROP TABLE self_check_results;
DROP INDEX self_check_runs_book_started_idx;
DROP TABLE self_check_runs;
ALTER TABLE self_check_runs_v16 RENAME TO self_check_runs;
ALTER TABLE self_check_results_v16 RENAME TO self_check_results;

CREATE INDEX self_check_runs_book_started_idx
  ON self_check_runs (book_id, started_at DESC, id DESC);
CREATE INDEX self_check_results_check_idx
  ON self_check_results (check_id, status);
