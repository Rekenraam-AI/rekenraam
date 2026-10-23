-- +goose Up
-- #388 / ADR 0019: instrument, holding and ex-date explain a cash charge;
-- they do not authorize it. Preserve all original facts and nullable reversal
-- equality; references retain same-book/object-kind guards, independently of
-- historical posting eligibility. No financial or source history is removed.
CREATE TABLE investment_payment_in_lieu_facts_v23 (
  operation_id INTEGER PRIMARY KEY REFERENCES investment_operations(id),
  book_id INTEGER NOT NULL REFERENCES books(id),
  holding_account_id INTEGER REFERENCES accounts(id),
  commodity_id INTEGER REFERENCES commodities(id),
  ex_date TEXT CHECK (ex_date IS NULL OR (ex_date GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]' AND length(ex_date) = 10)),
  created_audit_event_id INTEGER NOT NULL REFERENCES audit_events(id)
);
INSERT INTO investment_payment_in_lieu_facts_v23 (operation_id, book_id, holding_account_id, commodity_id, ex_date, created_audit_event_id) SELECT operation_id, book_id, holding_account_id, commodity_id, ex_date, created_audit_event_id FROM investment_payment_in_lieu_facts;
DROP TABLE investment_payment_in_lieu_facts;
ALTER TABLE investment_payment_in_lieu_facts_v23 RENAME TO investment_payment_in_lieu_facts;
CREATE INDEX investment_payment_in_lieu_facts_holding ON investment_payment_in_lieu_facts(book_id, holding_account_id, commodity_id, operation_id);
-- +goose StatementBegin
-- Exact reversal facts retain the original reference even if an unused
-- holding has since been reclassified; the equality guard still binds it.
CREATE TRIGGER investment_payment_in_lieu_facts_insert_guard BEFORE INSERT ON investment_payment_in_lieu_facts BEGIN
  SELECT RAISE(ABORT, 'invalid payment in lieu fact') WHERE NOT EXISTS (
    SELECT 1 FROM investment_operations o
    WHERE o.id = NEW.operation_id AND o.book_id = NEW.book_id
      AND (NEW.commodity_id IS NULL OR EXISTS (
        SELECT 1 FROM commodities c WHERE c.id = NEW.commodity_id AND c.book_id = o.book_id AND c.kind IN ('security', 'crypto')))
      AND (NEW.holding_account_id IS NULL OR EXISTS (
        SELECT 1 FROM accounts a JOIN account_versions av ON av.account_id = a.id
        JOIN account_kinds k ON k.code = av.account_kind AND k.account_class = av.account_class
        WHERE a.id = NEW.holding_account_id AND a.book_id = o.book_id
          AND (o.operation_kind = 'reversal' OR (a.system_role IS NULL
            AND av.account_class = 'asset' AND k.base_kind = 'security_holding'
            AND av.version_seq = (SELECT MAX(version_seq) FROM account_versions WHERE account_id = a.id)))))
      AND o.created_audit_event_id = NEW.created_audit_event_id
      AND (NEW.ex_date IS NULL OR date(NEW.ex_date, '+0 days') IS NEW.ex_date)
      AND (o.operation_kind = 'payment_in_lieu' OR (o.operation_kind = 'reversal' AND EXISTS (
        SELECT 1 FROM investment_payment_in_lieu_facts predecessor
        WHERE predecessor.operation_id = o.correction_of_operation_id
          AND predecessor.holding_account_id IS NEW.holding_account_id
          AND predecessor.commodity_id IS NEW.commodity_id
          AND predecessor.ex_date IS NEW.ex_date
      )))
  );
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_payment_in_lieu_facts_no_update BEFORE UPDATE ON investment_payment_in_lieu_facts BEGIN
  SELECT RAISE(ABORT, 'payment in lieu facts are immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_payment_in_lieu_facts_no_delete BEFORE DELETE ON investment_payment_in_lieu_facts BEGIN
  SELECT RAISE(ABORT, 'payment in lieu facts are immutable');
END;
-- +goose StatementEnd


-- Cash attribution corrections preserve the payment's source identity.
-- A typed read view follows only this family's immutable correction ancestry.
CREATE VIEW investment_payment_in_lieu_ancestry AS
WITH RECURSIVE ancestry(book_id, operation_id, ancestor_operation_id) AS (
  SELECT book_id, id, id FROM investment_operations WHERE operation_kind = 'payment_in_lieu'
  UNION
  SELECT ancestry.book_id, ancestry.operation_id, parent.correction_of_operation_id
  FROM ancestry JOIN investment_operations parent ON parent.id = ancestry.ancestor_operation_id AND parent.book_id = ancestry.book_id
  WHERE parent.correction_of_operation_id IS NOT NULL
)
SELECT book_id, operation_id, ancestor_operation_id FROM ancestry;

-- One read-side structural definition serves integrity and advisory readers.
-- Holding posting eligibility is intentionally absent; the reference itself
-- must exist in the book. New supplied roles are checked on insertion above.
CREATE VIEW valid_investment_payment_in_lieu_facts AS
SELECT f.* FROM investment_payment_in_lieu_facts f
JOIN investment_operations o ON o.id = f.operation_id AND o.book_id = f.book_id
WHERE o.created_audit_event_id = f.created_audit_event_id
  AND (f.ex_date IS NULL OR (length(f.ex_date) = 10
    AND f.ex_date GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]'
    AND date(f.ex_date, '+0 days') = f.ex_date))
  AND (f.holding_account_id IS NULL OR EXISTS (SELECT 1 FROM accounts a WHERE a.id = f.holding_account_id AND a.book_id = f.book_id))
  AND (f.commodity_id IS NULL OR EXISTS (SELECT 1 FROM commodities c WHERE c.id = f.commodity_id AND c.book_id = f.book_id AND c.kind IN ('security', 'crypto')))
  AND ((f.ex_date IS NULL AND NOT EXISTS (SELECT 1 FROM investment_operation_dates d WHERE d.operation_id = f.operation_id AND d.date_role = 'ex'))
    OR EXISTS (SELECT 1 FROM investment_operation_dates d WHERE d.operation_id = f.operation_id AND d.date_role = 'ex' AND d.event_date = f.ex_date));

-- +goose Down
-- The old representation cannot hold partial attribution or chronology
-- warnings. Refuse rather than erase fields or restate historical facts.
CREATE TEMP TABLE payment_attribution_downgrade_guard (ok INTEGER CHECK (ok = 1));
INSERT INTO payment_attribution_downgrade_guard
SELECT NOT EXISTS (SELECT 1 FROM investment_payment_in_lieu_facts f JOIN investment_operations o ON o.id = f.operation_id
  WHERE f.holding_account_id IS NULL OR f.commodity_id IS NULL OR f.ex_date IS NULL OR f.ex_date > o.event_date);
DROP TABLE payment_attribution_downgrade_guard;
DROP VIEW valid_investment_payment_in_lieu_facts;
DROP VIEW investment_payment_in_lieu_ancestry;
CREATE TABLE investment_payment_in_lieu_facts_v22 (
  operation_id INTEGER PRIMARY KEY REFERENCES investment_operations(id),
  book_id INTEGER NOT NULL REFERENCES books(id),
  holding_account_id INTEGER NOT NULL REFERENCES accounts(id),
  commodity_id INTEGER NOT NULL REFERENCES commodities(id),
  ex_date TEXT NOT NULL CHECK (ex_date GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]' AND length(ex_date) = 10),
  created_audit_event_id INTEGER NOT NULL REFERENCES audit_events(id)
);
INSERT INTO investment_payment_in_lieu_facts_v22 (operation_id, book_id, holding_account_id, commodity_id, ex_date, created_audit_event_id) SELECT operation_id, book_id, holding_account_id, commodity_id, ex_date, created_audit_event_id FROM investment_payment_in_lieu_facts;
DROP TABLE investment_payment_in_lieu_facts;
ALTER TABLE investment_payment_in_lieu_facts_v22 RENAME TO investment_payment_in_lieu_facts;
CREATE INDEX investment_payment_in_lieu_facts_holding ON investment_payment_in_lieu_facts(book_id, holding_account_id, commodity_id, operation_id);
-- +goose StatementBegin
CREATE TRIGGER investment_payment_in_lieu_facts_insert_guard BEFORE INSERT ON investment_payment_in_lieu_facts BEGIN
  SELECT RAISE(ABORT, 'invalid payment in lieu fact') WHERE NOT EXISTS (
    SELECT 1 FROM investment_operations o
    JOIN commodities c ON c.id = NEW.commodity_id AND c.book_id = o.book_id
    JOIN accounts a ON a.id = NEW.holding_account_id AND a.book_id = o.book_id
    WHERE o.id = NEW.operation_id AND o.book_id = NEW.book_id
      AND c.kind IN ('security', 'crypto')
      AND o.created_audit_event_id = NEW.created_audit_event_id
      AND NEW.ex_date <= o.event_date
      AND (o.operation_kind = 'payment_in_lieu' OR (o.operation_kind = 'reversal' AND EXISTS (
        SELECT 1 FROM investment_payment_in_lieu_facts predecessor
        WHERE predecessor.operation_id = o.correction_of_operation_id
          AND predecessor.holding_account_id = NEW.holding_account_id
          AND predecessor.commodity_id = NEW.commodity_id
          AND predecessor.ex_date = NEW.ex_date
      )))
  );
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_payment_in_lieu_facts_no_update BEFORE UPDATE ON investment_payment_in_lieu_facts BEGIN
  SELECT RAISE(ABORT, 'payment in lieu facts are immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_payment_in_lieu_facts_no_delete BEFORE DELETE ON investment_payment_in_lieu_facts BEGIN
  SELECT RAISE(ABORT, 'payment in lieu facts are immutable');
END;
-- +goose StatementEnd

