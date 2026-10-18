-- +goose Up
-- #302: a CSV or QIF bank row's commit identity is scoped to the account it
-- posted to, so an identical row in two accounts' statements commits to each
-- (docs/plans/import-plan.md, "Dedupe identity"). dedupe_fingerprint stays
-- the unique commit key; for an account-scoped row it is derived from the
-- staged row's fingerprint and the account. source_fingerprint is the staged
-- row's own fingerprint, so staging can still flag a row already imported to
-- some account before the owner picks one. Every earlier identity was
-- book-global, so its source fingerprint is its key.
ALTER TABLE import_commit_identities ADD COLUMN source_fingerprint TEXT NOT NULL DEFAULT '';
UPDATE import_commit_identities SET source_fingerprint = dedupe_fingerprint;
CREATE INDEX import_commit_identities_source_idx ON import_commit_identities (book_id, source_fingerprint);

-- A committed staged row names the identity derived from its fingerprint.
-- +goose StatementBegin
DROP TRIGGER import_staged_rows_identity_same_book;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER import_staged_rows_identity_same_book
BEFORE UPDATE OF commit_status, committed_identity_id, committed_transaction_id ON import_staged_rows
BEGIN
  SELECT CASE WHEN NEW.commit_status = 'committed' AND NOT EXISTS (
    SELECT 1 FROM import_commit_identities i
    WHERE i.id = NEW.committed_identity_id AND i.book_id = NEW.book_id
      AND i.source_fingerprint = NEW.dedupe_fingerprint
  ) THEN RAISE(ABORT, 'staged row identity must match book and fingerprint') END;
  SELECT CASE WHEN NEW.commit_status = 'committed' AND NOT EXISTS (
    SELECT 1 FROM import_commit_identity_effects e WHERE e.identity_id = NEW.committed_identity_id
  ) THEN RAISE(ABORT, 'committed staged row requires identity effects') END;
  SELECT CASE WHEN NEW.commit_status = 'committed' AND NEW.committed_transaction_id IS NOT (
    SELECT e.transaction_id FROM import_commit_identity_effects e
    WHERE e.identity_id = NEW.committed_identity_id AND e.transaction_id IS NOT NULL
    ORDER BY e.effect_seq LIMIT 1
  ) THEN RAISE(ABORT, 'staged row transaction summary must match first effect') END;
END;
-- +goose StatementEnd

-- +goose Down
-- Downgrading would leave account-scoped identities that the older trigger
-- refuses, so it is refused once any exists.
CREATE TEMP TABLE import_identity_scope_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO import_identity_scope_downgrade_guard
SELECT NOT EXISTS (SELECT 1 FROM import_commit_identities WHERE source_fingerprint <> dedupe_fingerprint);
DROP TABLE import_identity_scope_downgrade_guard;

-- +goose StatementBegin
DROP TRIGGER import_staged_rows_identity_same_book;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER import_staged_rows_identity_same_book
BEFORE UPDATE OF commit_status, committed_identity_id, committed_transaction_id ON import_staged_rows
BEGIN
  SELECT CASE WHEN NEW.commit_status = 'committed' AND NOT EXISTS (
    SELECT 1 FROM import_commit_identities i
    WHERE i.id = NEW.committed_identity_id AND i.book_id = NEW.book_id
      AND i.dedupe_fingerprint = NEW.dedupe_fingerprint
  ) THEN RAISE(ABORT, 'staged row identity must match book and fingerprint') END;
  SELECT CASE WHEN NEW.commit_status = 'committed' AND NOT EXISTS (
    SELECT 1 FROM import_commit_identity_effects e WHERE e.identity_id = NEW.committed_identity_id
  ) THEN RAISE(ABORT, 'committed staged row requires identity effects') END;
  SELECT CASE WHEN NEW.commit_status = 'committed' AND NEW.committed_transaction_id IS NOT (
    SELECT e.transaction_id FROM import_commit_identity_effects e
    WHERE e.identity_id = NEW.committed_identity_id AND e.transaction_id IS NOT NULL
    ORDER BY e.effect_seq LIMIT 1
  ) THEN RAISE(ABORT, 'staged row transaction summary must match first effect') END;
END;
-- +goose StatementEnd

DROP INDEX import_commit_identities_source_idx;
ALTER TABLE import_commit_identities DROP COLUMN source_fingerprint;
