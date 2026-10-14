-- +goose Up
-- #224: a posting line is one posting in each transaction version. Line
-- identity carries reconciliation state, tags and per-account day order, so a
-- second posting on the same line in one version would silently share them.
CREATE UNIQUE INDEX IF NOT EXISTS posting_versions_line_once_per_version_idx
  ON posting_versions (transaction_version_id, posting_line_id);

-- +goose Down
DROP INDEX IF EXISTS posting_versions_line_once_per_version_idx;
