-- +goose Up
-- #229: the payer is an operation fact shared by gross income and withholding.
-- Historical cash-only journals do not establish an instrument. Leave them
-- unassigned rather than guessing from descriptions or transaction metadata.
CREATE TABLE investment_dividend_facts (
  operation_id INTEGER PRIMARY KEY REFERENCES investment_operations(id),
  book_id INTEGER NOT NULL REFERENCES books(id),
  commodity_id INTEGER NOT NULL REFERENCES commodities(id),
  withholding_only INTEGER NOT NULL DEFAULT 0 CHECK (withholding_only IN (0, 1)),
  created_audit_event_id INTEGER NOT NULL REFERENCES audit_events(id)
);
CREATE INDEX investment_dividend_facts_commodity ON investment_dividend_facts(book_id, commodity_id, operation_id);

-- +goose StatementBegin
CREATE TRIGGER investment_dividend_facts_insert_guard BEFORE INSERT ON investment_dividend_facts BEGIN
  SELECT RAISE(ABORT, 'invalid dividend instrument fact') WHERE NOT EXISTS (
    SELECT 1 FROM investment_operations o
    JOIN commodities c ON c.id = NEW.commodity_id AND c.book_id = o.book_id
    WHERE o.id = NEW.operation_id AND o.book_id = NEW.book_id
      AND c.kind IN ('security', 'crypto')
      AND o.created_audit_event_id = NEW.created_audit_event_id
      AND (o.operation_kind = 'dividend' OR (o.operation_kind = 'reversal' AND EXISTS (
        SELECT 1 FROM investment_dividend_facts predecessor
        WHERE predecessor.operation_id = o.correction_of_operation_id
          AND predecessor.commodity_id = NEW.commodity_id
          AND predecessor.withholding_only = NEW.withholding_only
      )))
  );
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_dividend_facts_no_update BEFORE UPDATE ON investment_dividend_facts BEGIN
  SELECT RAISE(ABORT, 'dividend instrument facts are immutable');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_dividend_facts_no_delete BEFORE DELETE ON investment_dividend_facts BEGIN
  SELECT RAISE(ABORT, 'dividend instrument facts are immutable');
END;
-- +goose StatementEnd

-- +goose Down
-- A populated fact table cannot be downgraded without losing payer/tax evidence.
CREATE TEMP TABLE dividend_facts_downgrade_guard (empty INTEGER CHECK (empty = 1));
INSERT INTO dividend_facts_downgrade_guard SELECT NOT EXISTS (SELECT 1 FROM investment_dividend_facts);
DROP TABLE dividend_facts_downgrade_guard;
DROP TABLE investment_dividend_facts;
