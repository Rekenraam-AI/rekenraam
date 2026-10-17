-- +goose Up
-- #111: a provider suggestion can be linked to an owner-recorded dividend
-- instead of posting a second one. Each posted or linked transaction admits at
-- most one provider event, so a later fetch can never claim it twice.
CREATE TEMP TABLE saved_investment_event_suggestions AS SELECT * FROM investment_event_suggestions;
DROP INDEX investment_event_suggestions_book_idx;
DROP TABLE investment_event_suggestions;
CREATE TABLE investment_event_suggestions (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  provider_event_id INTEGER NOT NULL REFERENCES investment_provider_events(id) ON DELETE RESTRICT,
  instrument_id INTEGER REFERENCES investment_instruments(id) ON DELETE RESTRICT,
  confidence_bps INTEGER NOT NULL CHECK (confidence_bps BETWEEN 0 AND 10000),
  status TEXT NOT NULL CHECK (status IN ('suggested', 'accepted', 'linked', 'ignored', 'auto_posted', 'failed', 'superseded')),
  proposed_transaction_json TEXT NOT NULL DEFAULT '{}',
  generated_transaction_id INTEGER REFERENCES transactions(id) ON DELETE RESTRICT,
  failure_reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  created_by_user_id INTEGER REFERENCES users(id) ON DELETE RESTRICT,
  updated_at TEXT NOT NULL,
  updated_by_user_id INTEGER REFERENCES users(id) ON DELETE RESTRICT,
  created_audit_event_id INTEGER REFERENCES audit_events(id) ON DELETE RESTRICT,
  updated_audit_event_id INTEGER REFERENCES audit_events(id) ON DELETE RESTRICT,
  linked_transaction_id INTEGER REFERENCES transactions(id) ON DELETE RESTRICT,
  CHECK ((status = 'linked') = (linked_transaction_id IS NOT NULL)),
  CHECK (linked_transaction_id IS NULL OR generated_transaction_id IS NULL)
);
INSERT INTO investment_event_suggestions (
  id, book_id, provider_event_id, instrument_id, confidence_bps, status, proposed_transaction_json,
  generated_transaction_id, failure_reason, created_at, created_by_user_id, updated_at, updated_by_user_id,
  created_audit_event_id, updated_audit_event_id)
SELECT id, book_id, provider_event_id, instrument_id, confidence_bps, status, proposed_transaction_json,
  generated_transaction_id, failure_reason, created_at, created_by_user_id, updated_at, updated_by_user_id,
  created_audit_event_id, updated_audit_event_id
FROM saved_investment_event_suggestions;
DROP TABLE saved_investment_event_suggestions;
CREATE INDEX investment_event_suggestions_book_idx
  ON investment_event_suggestions (book_id, status, created_at DESC, id DESC);
CREATE UNIQUE INDEX investment_event_suggestions_generated_idx
  ON investment_event_suggestions (generated_transaction_id) WHERE generated_transaction_id IS NOT NULL;
CREATE UNIQUE INDEX investment_event_suggestions_linked_idx
  ON investment_event_suggestions (linked_transaction_id) WHERE linked_transaction_id IS NOT NULL;
-- One suggestion per provider event: a re-fetch never queues a second review.
CREATE UNIQUE INDEX investment_event_suggestions_event_idx
  ON investment_event_suggestions (provider_event_id);

-- +goose Down
-- A link is owner evidence; refuse to drop it silently.
CREATE TEMP TABLE event_links_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO event_links_downgrade_guard SELECT NOT EXISTS (SELECT 1 FROM investment_event_suggestions WHERE status = 'linked');
DROP TABLE event_links_downgrade_guard;
CREATE TEMP TABLE saved_investment_event_suggestions AS SELECT * FROM investment_event_suggestions;
DROP INDEX investment_event_suggestions_event_idx;
DROP INDEX investment_event_suggestions_linked_idx;
DROP INDEX investment_event_suggestions_generated_idx;
DROP INDEX investment_event_suggestions_book_idx;
DROP TABLE investment_event_suggestions;
CREATE TABLE investment_event_suggestions (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  provider_event_id INTEGER NOT NULL REFERENCES investment_provider_events(id) ON DELETE RESTRICT,
  instrument_id INTEGER REFERENCES investment_instruments(id) ON DELETE RESTRICT,
  confidence_bps INTEGER NOT NULL CHECK (confidence_bps BETWEEN 0 AND 10000),
  status TEXT NOT NULL CHECK (status IN ('suggested', 'accepted', 'ignored', 'auto_posted', 'failed', 'superseded')),
  proposed_transaction_json TEXT NOT NULL DEFAULT '{}',
  generated_transaction_id INTEGER REFERENCES transactions(id) ON DELETE RESTRICT,
  failure_reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  created_by_user_id INTEGER REFERENCES users(id) ON DELETE RESTRICT,
  updated_at TEXT NOT NULL,
  updated_by_user_id INTEGER REFERENCES users(id) ON DELETE RESTRICT,
  created_audit_event_id INTEGER REFERENCES audit_events(id) ON DELETE RESTRICT,
  updated_audit_event_id INTEGER REFERENCES audit_events(id) ON DELETE RESTRICT
);
INSERT INTO investment_event_suggestions
SELECT id, book_id, provider_event_id, instrument_id, confidence_bps, status, proposed_transaction_json,
  generated_transaction_id, failure_reason, created_at, created_by_user_id, updated_at, updated_by_user_id,
  created_audit_event_id, updated_audit_event_id
FROM saved_investment_event_suggestions;
DROP TABLE saved_investment_event_suggestions;
CREATE INDEX investment_event_suggestions_book_idx
  ON investment_event_suggestions (book_id, status, created_at DESC, id DESC);
