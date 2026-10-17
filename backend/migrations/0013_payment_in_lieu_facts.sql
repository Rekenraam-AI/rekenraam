-- +goose Up
-- #255: a payment in lieu of a dividend is owed by a short holder. The
-- holding, instrument and ex-date are operation facts: the journal only
-- touches cash, the expense account and an optional recovery account, so
-- nothing else in the ledger says which short position the charge was for.
-- A reversal carries its predecessor's facts unchanged.
CREATE TABLE investment_payment_in_lieu_facts (
  operation_id INTEGER PRIMARY KEY REFERENCES investment_operations(id),
  book_id INTEGER NOT NULL REFERENCES books(id),
  holding_account_id INTEGER NOT NULL REFERENCES accounts(id),
  commodity_id INTEGER NOT NULL REFERENCES commodities(id),
  ex_date TEXT NOT NULL CHECK (ex_date GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]' AND length(ex_date) = 10),
  created_audit_event_id INTEGER NOT NULL REFERENCES audit_events(id)
);
CREATE INDEX investment_payment_in_lieu_facts_holding
  ON investment_payment_in_lieu_facts(book_id, holding_account_id, commodity_id, operation_id);

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

-- +goose Down
-- A populated fact table cannot be downgraded without losing which short
-- position each charge was owed for.
CREATE TEMP TABLE payment_in_lieu_facts_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO payment_in_lieu_facts_downgrade_guard SELECT NOT EXISTS (SELECT 1 FROM investment_payment_in_lieu_facts);
DROP TABLE payment_in_lieu_facts_downgrade_guard;
DROP TABLE investment_payment_in_lieu_facts;
