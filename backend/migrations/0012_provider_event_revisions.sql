-- +goose Up
-- #254: a provider can change an announcement it already published. The first
-- observation stays the event's evidence; each later distinct content for the
-- same source identity is appended here under its own audit event. A revision
-- is evidence only: it never touches the event's status, its suggestion, or
-- any posted or linked transaction.
CREATE TABLE investment_provider_event_revisions (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  provider_event_id INTEGER NOT NULL REFERENCES investment_provider_events(id) ON DELETE RESTRICT,
  content_key TEXT NOT NULL CHECK (content_key <> '' AND instr(content_key, char(0)) = 0),
  normalized_json TEXT NOT NULL CHECK (json_valid(normalized_json) AND json_type(normalized_json) = 'object'),
  raw_json TEXT NOT NULL CHECK (json_valid(raw_json)),
  observed_at TEXT NOT NULL,
  created_by_user_id INTEGER REFERENCES users(id) ON DELETE RESTRICT,
  created_audit_event_id INTEGER NOT NULL REFERENCES audit_events(id) ON DELETE RESTRICT
);
-- One row per distinct provider content: a repeated fetch of a revision is a no-op.
CREATE UNIQUE INDEX investment_provider_event_revisions_content_idx
  ON investment_provider_event_revisions (provider_event_id, content_key);
CREATE INDEX investment_provider_event_revisions_book_idx
  ON investment_provider_event_revisions (book_id, provider_event_id, id);
-- +goose StatementBegin
CREATE TRIGGER investment_provider_event_revisions_same_book BEFORE INSERT ON investment_provider_event_revisions
WHEN NOT EXISTS (SELECT 1 FROM investment_provider_events e WHERE e.id = NEW.provider_event_id AND e.book_id = NEW.book_id)
BEGIN
  SELECT RAISE(ABORT, 'provider event revision must belong to its event''s book');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_provider_event_revisions_no_update BEFORE UPDATE ON investment_provider_event_revisions BEGIN
  SELECT RAISE(ABORT, 'provider event revisions are immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_provider_event_revisions_no_delete BEFORE DELETE ON investment_provider_event_revisions BEGIN
  SELECT RAISE(ABORT, 'provider event revisions are immutable');
END;
-- +goose StatementEnd

-- +goose Down
-- Revision evidence is not reproducible from the provider; refuse to drop it.
CREATE TEMP TABLE event_revisions_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO event_revisions_downgrade_guard SELECT NOT EXISTS (SELECT 1 FROM investment_provider_event_revisions);
DROP TABLE event_revisions_downgrade_guard;
DROP TABLE investment_provider_event_revisions;
