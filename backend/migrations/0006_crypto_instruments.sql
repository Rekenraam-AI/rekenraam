-- +goose Up
-- #117: rebuild only the leaf instrument-version table; preserve every ID and fact.
CREATE TEMP TABLE saved_instrument_versions AS SELECT * FROM investment_instrument_versions;
DROP VIEW current_investment_instrument_versions;
DROP TRIGGER investment_instrument_versions_no_update;
DROP TRIGGER investment_instrument_versions_no_delete;
DROP TABLE investment_instrument_versions;
CREATE TABLE IF NOT EXISTS investment_instrument_versions (
  id INTEGER PRIMARY KEY,
  instrument_id INTEGER NOT NULL REFERENCES investment_instruments(id) ON DELETE RESTRICT,
  version_seq INTEGER NOT NULL CHECK (version_seq > 0),
  effective_from TEXT NOT NULL CHECK (effective_from GLOB '????-??-??'),
  recorded_at TEXT NOT NULL,
  changed_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  change_reason TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active', 'archived')),
  instrument_type TEXT NOT NULL CHECK (instrument_type IN ('stock', 'etf', 'fund', 'bond', 'option', 'future', 'other', 'crypto')),
  display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0 AND display_name = trim(display_name)),
  symbol TEXT CHECK (symbol IS NULL OR (length(trim(symbol)) > 0 AND symbol = trim(symbol))),
  exchange_code TEXT,
  mic TEXT,
  issuer TEXT,
  country_code TEXT,
  quote_commodity_id INTEGER REFERENCES commodities(id) ON DELETE RESTRICT,
  trading_commodity_id INTEGER REFERENCES commodities(id) ON DELETE RESTRICT,
  quantity_scale INTEGER NOT NULL CHECK (quantity_scale BETWEEN 0 AND CASE WHEN instrument_type = 'crypto' THEN 24 ELSE 12 END),
  price_scale INTEGER NOT NULL CHECK (price_scale BETWEEN 0 AND 12),
  identifiers_json TEXT NOT NULL DEFAULT '{}',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  change_audit_event_id INTEGER REFERENCES audit_events(id) ON DELETE RESTRICT,
  UNIQUE (instrument_id, version_seq)
);

CREATE INDEX IF NOT EXISTS investment_instrument_versions_instrument_seq_idx
  ON investment_instrument_versions (instrument_id, version_seq DESC);

CREATE INDEX IF NOT EXISTS investment_instrument_versions_symbol_idx
  ON investment_instrument_versions (symbol COLLATE NOCASE, exchange_code, mic);

CREATE VIEW IF NOT EXISTS current_investment_instrument_versions AS
SELECT iiv.*
FROM investment_instrument_versions iiv
WHERE iiv.id = (
  SELECT current_iiv.id
  FROM investment_instrument_versions current_iiv
  WHERE current_iiv.instrument_id = iiv.instrument_id
    AND current_iiv.effective_from <= date('now')
  ORDER BY current_iiv.effective_from DESC, current_iiv.version_seq DESC
  LIMIT 1
);


INSERT INTO investment_instrument_versions SELECT * FROM saved_instrument_versions;
DROP TABLE saved_instrument_versions;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_instrument_versions_no_update
BEFORE UPDATE ON investment_instrument_versions
BEGIN
  SELECT RAISE(ABORT, 'investment_instrument_versions rows are append-only');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_instrument_versions_no_delete
BEFORE DELETE ON investment_instrument_versions
WHEN EXISTS (
  SELECT 1 FROM investment_instruments ii
  WHERE ii.id = OLD.instrument_id
    AND (
      EXISTS (SELECT 1 FROM investment_lots lot WHERE lot.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM posting_versions pv WHERE pv.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM import_connection_holdings ich WHERE ich.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM dividend_defaults dd WHERE dd.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM price_series ps WHERE ps.base_commodity_id = ii.commodity_id OR ps.quote_commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM instrument_source_links isl WHERE isl.instrument_id = ii.id)
      OR EXISTS (SELECT 1 FROM investment_provider_events ipe WHERE ipe.instrument_id = ii.id)
      OR EXISTS (SELECT 1 FROM investment_event_suggestions ies WHERE ies.instrument_id = ii.id)
      OR EXISTS (SELECT 1 FROM investment_automation_rules iar WHERE iar.instrument_id = ii.id)
    )
)
BEGIN
  SELECT RAISE(ABORT, 'investment_instrument_versions rows are append-only once referenced');
END;
-- +goose StatementEnd
INSERT INTO market_data_sources (code, name, kind, provider_key, status, metadata_json, created_at) VALUES
('coingecko', 'CoinGecko', 'provider', 'coingecko', 'active', '{"capabilities":["prices"],"requires_secret":true,"attribution_url":"https://www.coingecko.com/en/api"}', '0001-01-01T00:00:00Z'),
('twelve_data', 'Twelve Data', 'provider', 'twelve_data', 'active', '{"capabilities":["prices"],"requires_secret":true,"coverage":"operator_plan"}', '0001-01-01T00:00:00Z'),
('yahoo_finance', 'Yahoo Finance (unofficial)', 'provider', 'yahoo_finance', 'active', '{"capabilities":["prices"],"requires_secret":false,"unofficial":true}', '0001-01-01T00:00:00Z');

DROP TRIGGER investment_instruments_commodity_must_be_security;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_instruments_commodity_must_be_security
BEFORE INSERT ON investment_instruments
WHEN NOT EXISTS (
  SELECT 1
  FROM commodities c
  WHERE c.id = NEW.commodity_id
    AND c.book_id = NEW.book_id
    AND c.kind IN ('security','crypto')
)
BEGIN
  SELECT RAISE(ABORT, 'investment instrument commodity must be a security or crypto in the same book');
END;
-- +goose StatementEnd
DROP TRIGGER investment_lots_same_book;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_lots_same_book
BEFORE INSERT ON investment_lots
WHEN NOT EXISTS (
  SELECT 1
  FROM accounts a
  JOIN commodities security ON security.id = NEW.commodity_id
  JOIN commodities cost ON cost.id = NEW.cost_commodity_id
  WHERE a.id = NEW.account_id
    AND a.book_id = NEW.book_id
    AND security.book_id = NEW.book_id
    AND security.kind IN ('security','crypto')
    AND cost.book_id = NEW.book_id
)
BEGIN
  SELECT RAISE(ABORT, 'investment lot references must belong to one book');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_instrument_versions_commodity_kind
BEFORE INSERT ON investment_instrument_versions
WHEN NOT EXISTS (
 SELECT 1 FROM investment_instruments ii JOIN commodities c ON c.id=ii.commodity_id
 JOIN current_commodity_versions cv ON cv.commodity_id=c.id
 WHERE ii.id=NEW.instrument_id
 AND c.kind=CASE WHEN NEW.instrument_type='crypto' THEN 'crypto' ELSE 'security' END
 AND NEW.quantity_scale<=cv.max_quantity_scale
)
BEGIN SELECT RAISE(ABORT, 'instrument type and precision must match its commodity'); END;
-- +goose StatementEnd

-- +goose Down
-- Lossless on a stock-only database with unused quote sources. Crypto versions
-- or referenced quote sources refuse transactionally; restore a verified backup.
CREATE TEMP TABLE saved_instrument_versions AS SELECT * FROM investment_instrument_versions;
DROP VIEW current_investment_instrument_versions;
DROP TRIGGER investment_instrument_versions_commodity_kind;
DROP TRIGGER investment_instrument_versions_no_update;
DROP TRIGGER investment_instrument_versions_no_delete;
DROP TABLE investment_instrument_versions;
CREATE TABLE IF NOT EXISTS investment_instrument_versions (
  id INTEGER PRIMARY KEY,
  instrument_id INTEGER NOT NULL REFERENCES investment_instruments(id) ON DELETE RESTRICT,
  version_seq INTEGER NOT NULL CHECK (version_seq > 0),
  effective_from TEXT NOT NULL CHECK (effective_from GLOB '????-??-??'),
  recorded_at TEXT NOT NULL,
  changed_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  change_reason TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('active', 'archived')),
  instrument_type TEXT NOT NULL CHECK (instrument_type IN ('stock', 'etf', 'fund', 'bond', 'option', 'future', 'other')),
  display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0 AND display_name = trim(display_name)),
  symbol TEXT CHECK (symbol IS NULL OR (length(trim(symbol)) > 0 AND symbol = trim(symbol))),
  exchange_code TEXT,
  mic TEXT,
  issuer TEXT,
  country_code TEXT,
  quote_commodity_id INTEGER REFERENCES commodities(id) ON DELETE RESTRICT,
  trading_commodity_id INTEGER REFERENCES commodities(id) ON DELETE RESTRICT,
  quantity_scale INTEGER NOT NULL CHECK (quantity_scale BETWEEN 0 AND 12),
  price_scale INTEGER NOT NULL CHECK (price_scale BETWEEN 0 AND 12),
  identifiers_json TEXT NOT NULL DEFAULT '{}',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  change_audit_event_id INTEGER REFERENCES audit_events(id) ON DELETE RESTRICT,
  UNIQUE (instrument_id, version_seq)
);

CREATE INDEX IF NOT EXISTS investment_instrument_versions_instrument_seq_idx
  ON investment_instrument_versions (instrument_id, version_seq DESC);

CREATE INDEX IF NOT EXISTS investment_instrument_versions_symbol_idx
  ON investment_instrument_versions (symbol COLLATE NOCASE, exchange_code, mic);

CREATE VIEW IF NOT EXISTS current_investment_instrument_versions AS
SELECT iiv.*
FROM investment_instrument_versions iiv
WHERE iiv.id = (
  SELECT current_iiv.id
  FROM investment_instrument_versions current_iiv
  WHERE current_iiv.instrument_id = iiv.instrument_id
    AND current_iiv.effective_from <= date('now')
  ORDER BY current_iiv.effective_from DESC, current_iiv.version_seq DESC
  LIMIT 1
);


INSERT INTO investment_instrument_versions SELECT * FROM saved_instrument_versions;
DROP TABLE saved_instrument_versions;
DROP TRIGGER investment_instruments_commodity_must_be_security;
DROP TRIGGER investment_lots_same_book;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_instrument_versions_no_update
BEFORE UPDATE ON investment_instrument_versions
BEGIN
  SELECT RAISE(ABORT, 'investment_instrument_versions rows are append-only');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_instrument_versions_no_delete
BEFORE DELETE ON investment_instrument_versions
WHEN EXISTS (
  SELECT 1 FROM investment_instruments ii
  WHERE ii.id = OLD.instrument_id
    AND (
      EXISTS (SELECT 1 FROM investment_lots lot WHERE lot.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM posting_versions pv WHERE pv.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM import_connection_holdings ich WHERE ich.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM dividend_defaults dd WHERE dd.commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM price_series ps WHERE ps.base_commodity_id = ii.commodity_id OR ps.quote_commodity_id = ii.commodity_id)
      OR EXISTS (SELECT 1 FROM instrument_source_links isl WHERE isl.instrument_id = ii.id)
      OR EXISTS (SELECT 1 FROM investment_provider_events ipe WHERE ipe.instrument_id = ii.id)
      OR EXISTS (SELECT 1 FROM investment_event_suggestions ies WHERE ies.instrument_id = ii.id)
      OR EXISTS (SELECT 1 FROM investment_automation_rules iar WHERE iar.instrument_id = ii.id)
    )
)
BEGIN
  SELECT RAISE(ABORT, 'investment_instrument_versions rows are append-only once referenced');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_instruments_commodity_must_be_security
BEFORE INSERT ON investment_instruments
WHEN NOT EXISTS (
  SELECT 1
  FROM commodities c
  WHERE c.id = NEW.commodity_id
    AND c.book_id = NEW.book_id
    AND c.kind = 'security'
)
BEGIN
  SELECT RAISE(ABORT, 'investment instrument commodity must be a security in the same book');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_lots_same_book
BEFORE INSERT ON investment_lots
WHEN NOT EXISTS (
  SELECT 1
  FROM accounts a
  JOIN commodities security ON security.id = NEW.commodity_id
  JOIN commodities cost ON cost.id = NEW.cost_commodity_id
  WHERE a.id = NEW.account_id
    AND a.book_id = NEW.book_id
    AND security.book_id = NEW.book_id
    AND security.kind = 'security'
    AND cost.book_id = NEW.book_id
)
BEGIN
  SELECT RAISE(ABORT, 'investment lot references must belong to one book');
END;
-- +goose StatementEnd
DELETE FROM market_data_sources WHERE code IN ('coingecko','twelve_data','yahoo_finance');
