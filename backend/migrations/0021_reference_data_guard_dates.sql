-- +goose Up
-- Reference-data guards select at the source version date, never SQLite UTC today.
DROP TRIGGER fx_work_after_account_version_insert;
-- +goose StatementBegin
CREATE TRIGGER fx_work_after_account_version_insert
AFTER INSERT ON account_versions
WHEN NEW.status = 'active'
  AND EXISTS (SELECT 1 FROM commodities c WHERE c.id = NEW.default_commodity_id AND c.kind = 'currency')
  AND NEW.default_commodity_id != COALESCE(
    (SELECT pp.base_commodity_id FROM accounts a JOIN pricing_policies pp ON pp.book_id = a.book_id WHERE a.id = NEW.account_id),
    (SELECT b.default_currency_commodity_id FROM accounts a JOIN books b ON b.id = a.book_id WHERE a.id = NEW.account_id),
    0
  )
  AND NOT EXISTS (
    SELECT 1 FROM account_versions prior
    WHERE prior.account_id = NEW.account_id AND prior.id != NEW.id
      AND prior.status = 'active' AND prior.default_commodity_id = NEW.default_commodity_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM account_versions other
    JOIN accounts other_account ON other_account.id = other.account_id
    JOIN accounts new_account ON new_account.id = NEW.account_id
    WHERE other_account.book_id = new_account.book_id
      AND other.id = (SELECT candidate.id FROM account_versions candidate
        WHERE candidate.account_id = other.account_id
          AND candidate.effective_from <= NEW.effective_from
        ORDER BY candidate.effective_from DESC, candidate.version_seq DESC LIMIT 1)
      AND other.account_id != NEW.account_id
      AND other.status = 'active' AND other.default_commodity_id = NEW.default_commodity_id
  )
BEGIN
  INSERT INTO background_work_items (book_id, kind, payload_json, available_at, created_at, updated_at)
  SELECT a.book_id, 'pricing.fx_coverage',
    json_object('reason', 'currency_activated', 'start_date', NEW.opened_on, 'currency_id', NEW.default_commodity_id),
    NEW.recorded_at, NEW.recorded_at, NEW.recorded_at
  FROM accounts a WHERE a.id = NEW.account_id
  ON CONFLICT(book_id, kind, payload_json) WHERE status IN ('pending', 'running') DO NOTHING;
END;
-- +goose StatementEnd

DROP TRIGGER investment_instrument_versions_commodity_kind;
-- +goose StatementBegin
CREATE TRIGGER investment_instrument_versions_commodity_kind
BEFORE INSERT ON investment_instrument_versions
WHEN NOT EXISTS (
 SELECT 1 FROM investment_instruments ii JOIN commodities c ON c.id=ii.commodity_id
 JOIN commodity_versions cv ON cv.commodity_id=c.id
 AND cv.id = (SELECT candidate.id FROM commodity_versions candidate
   WHERE candidate.commodity_id=c.id AND candidate.effective_from<=NEW.effective_from
   ORDER BY candidate.effective_from DESC, candidate.version_seq DESC LIMIT 1)
 WHERE ii.id=NEW.instrument_id
 AND c.kind=CASE WHEN NEW.instrument_type='crypto' THEN 'crypto' ELSE 'security' END
 AND NEW.quantity_scale<=cv.max_quantity_scale
)
BEGIN SELECT RAISE(ABORT, 'instrument type and precision must match its commodity'); END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER investment_instrument_versions_commodity_kind;
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

DROP TRIGGER fx_work_after_account_version_insert;
-- +goose StatementBegin
CREATE TRIGGER fx_work_after_account_version_insert
AFTER INSERT ON account_versions
WHEN NEW.status = 'active'
  AND EXISTS (SELECT 1 FROM commodities c WHERE c.id = NEW.default_commodity_id AND c.kind = 'currency')
  AND NEW.default_commodity_id != COALESCE(
    (SELECT pp.base_commodity_id FROM accounts a JOIN pricing_policies pp ON pp.book_id = a.book_id WHERE a.id = NEW.account_id),
    (SELECT b.default_currency_commodity_id FROM accounts a JOIN books b ON b.id = a.book_id WHERE a.id = NEW.account_id),
    0
  )
  AND NOT EXISTS (
    SELECT 1 FROM account_versions prior
    WHERE prior.account_id = NEW.account_id AND prior.id != NEW.id
      AND prior.status = 'active' AND prior.default_commodity_id = NEW.default_commodity_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM current_account_versions other
    JOIN accounts other_account ON other_account.id = other.account_id
    JOIN accounts new_account ON new_account.id = NEW.account_id
    WHERE other_account.book_id = new_account.book_id
      AND other.account_id != NEW.account_id
      AND other.status = 'active' AND other.default_commodity_id = NEW.default_commodity_id
  )
BEGIN
  INSERT INTO background_work_items (book_id, kind, payload_json, available_at, created_at, updated_at)
  SELECT a.book_id, 'pricing.fx_coverage',
    json_object('reason', 'currency_activated', 'start_date', NEW.opened_on, 'currency_id', NEW.default_commodity_id),
    NEW.recorded_at, NEW.recorded_at, NEW.recorded_at
  FROM accounts a WHERE a.id = NEW.account_id
  ON CONFLICT(book_id, kind, payload_json) WHERE status IN ('pending', 'running') DO NOTHING;
END;
-- +goose StatementEnd
