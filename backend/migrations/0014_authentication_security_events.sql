-- +goose Up
-- #288: account-security changes become attributed authentication events.
--
-- Where they live, deliberately: authentication_events, not audit_events.
-- audit_events is book-scoped ledger provenance whose rows financial tables
-- reference under ON DELETE RESTRICT, it has no client address, and nothing
-- reads it for sign-in history. authentication_events already carries user,
-- session, proxy-aware client IP and request id, and GET /api/v1/auth/events
-- already serves it, so a reviewer reads one timeline: the sign-in that
-- created a session next to the MFA change made from it.
--
-- New columns:
--   * origin says where the change came from. Only the recover-owner CLI
--     writes 'cli_recovery', and it writes nothing else.
--   * second_factor tells a recovery-code sign-in from a TOTP one. It is set
--     only on login_succeeded; '' there means no second factor was owed.
--     Rows written before this migration keep '' because the factor used was
--     never recorded.
--   * metadata_json carries non-secret context (which approved device, how
--     many sessions a recovery revoked). Never codes, secrets or tokens.
--
-- SQLite cannot widen a CHECK in place, so the table is rebuilt; nothing
-- references it.
CREATE TABLE authentication_events_v2 (
  id INTEGER PRIMARY KEY,
  occurred_at TEXT NOT NULL,
  event_type TEXT NOT NULL CHECK (
    event_type IN (
      'login_succeeded', 'login_failed', 'login_blocked', 'logout',
      'mfa_enrollment_started', 'mfa_activated', 'mfa_disabled',
      'mfa_recovery_codes_regenerated', 'trusted_device_revoked',
      'owner_access_recovered'
    )
  ),
  outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure')),
  username TEXT NOT NULL DEFAULT '',
  user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  auth_session_id INTEGER REFERENCES auth_sessions(id) ON DELETE SET NULL,
  client_ip TEXT NOT NULL DEFAULT '',
  failure_reason TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL DEFAULT '',
  origin TEXT NOT NULL DEFAULT 'browser_api' CHECK (origin IN ('browser_api', 'cli_recovery')),
  second_factor TEXT NOT NULL DEFAULT '' CHECK (second_factor IN ('', 'totp', 'recovery_code')),
  metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (
    json_valid(metadata_json) AND json_type(metadata_json) = 'object'
  ),
  CHECK (second_factor = '' OR event_type = 'login_succeeded'),
  CHECK ((origin = 'cli_recovery') = (event_type = 'owner_access_recovered'))
);

INSERT INTO authentication_events_v2 (
  id, occurred_at, event_type, outcome, username, user_id, auth_session_id,
  client_ip, failure_reason, request_id
)
SELECT id, occurred_at, event_type, outcome, username, user_id, auth_session_id,
  client_ip, failure_reason, request_id
FROM authentication_events;

DROP INDEX IF EXISTS authentication_events_client_ip_idx;
DROP INDEX IF EXISTS authentication_events_occurred_idx;
DROP TABLE authentication_events;
ALTER TABLE authentication_events_v2 RENAME TO authentication_events;

CREATE INDEX authentication_events_occurred_idx
  ON authentication_events (occurred_at DESC, id DESC);

CREATE INDEX authentication_events_client_ip_idx
  ON authentication_events (client_ip, occurred_at DESC)
  WHERE client_ip <> '';

-- +goose Down
-- Downgrading would drop account-security history or the factor a sign-in
-- used, so it is refused once any such row exists.
CREATE TEMP TABLE authentication_events_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO authentication_events_downgrade_guard SELECT NOT EXISTS (
  SELECT 1 FROM authentication_events
  WHERE event_type NOT IN ('login_succeeded', 'login_failed', 'login_blocked', 'logout')
     OR second_factor <> ''
);
DROP TABLE authentication_events_downgrade_guard;

CREATE TABLE authentication_events_v1 (
  id INTEGER PRIMARY KEY,
  occurred_at TEXT NOT NULL,
  event_type TEXT NOT NULL CHECK (
    event_type IN ('login_succeeded', 'login_failed', 'login_blocked', 'logout')
  ),
  outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure')),
  username TEXT NOT NULL DEFAULT '',
  user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  auth_session_id INTEGER REFERENCES auth_sessions(id) ON DELETE SET NULL,
  client_ip TEXT NOT NULL DEFAULT '',
  failure_reason TEXT NOT NULL DEFAULT '',
  request_id TEXT NOT NULL DEFAULT ''
);

INSERT INTO authentication_events_v1 (
  id, occurred_at, event_type, outcome, username, user_id, auth_session_id,
  client_ip, failure_reason, request_id
)
SELECT id, occurred_at, event_type, outcome, username, user_id, auth_session_id,
  client_ip, failure_reason, request_id
FROM authentication_events;

DROP INDEX IF EXISTS authentication_events_client_ip_idx;
DROP INDEX IF EXISTS authentication_events_occurred_idx;
DROP TABLE authentication_events;
ALTER TABLE authentication_events_v1 RENAME TO authentication_events;

CREATE INDEX authentication_events_occurred_idx
  ON authentication_events (occurred_at DESC, id DESC);

CREATE INDEX authentication_events_client_ip_idx
  ON authentication_events (client_ip, occurred_at DESC)
  WHERE client_ip <> '';
