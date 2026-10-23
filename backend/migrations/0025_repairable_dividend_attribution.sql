-- +goose Up
-- #393 / ADR 0019: payment shape is financial; payer is nullable attribution.
CREATE TABLE investment_dividend_facts_v25 (
  operation_id INTEGER PRIMARY KEY REFERENCES investment_operations(id),
  book_id INTEGER NOT NULL REFERENCES books(id),
  commodity_id INTEGER REFERENCES commodities(id),
  withholding_only INTEGER NOT NULL DEFAULT 0 CHECK (withholding_only IN (0, 1)),
  created_audit_event_id INTEGER NOT NULL REFERENCES audit_events(id)
);
INSERT INTO investment_dividend_facts_v25 SELECT * FROM investment_dividend_facts;
-- Only recorded components establish a historical payment shape. Never infer payer.
INSERT INTO investment_dividend_facts_v25
SELECT o.id, o.book_id, NULL,
  NOT EXISTS (SELECT 1 FROM investment_operation_components c WHERE c.operation_id=o.id AND c.component_kind='dividend_gross'),
  o.created_audit_event_id
FROM investment_operations o WHERE o.operation_kind='dividend'
  AND NOT EXISTS (SELECT 1 FROM investment_dividend_facts_v25 f WHERE f.operation_id=o.id)
  AND EXISTS (SELECT 1 FROM investment_operation_components c WHERE c.operation_id=o.id AND c.component_kind IN ('dividend_gross','withholding'));
INSERT INTO investment_dividend_facts_v25
SELECT o.id, o.book_id, predecessor.commodity_id, predecessor.withholding_only, o.created_audit_event_id
FROM investment_operations o JOIN investment_dividend_facts_v25 predecessor ON predecessor.operation_id=o.correction_of_operation_id AND predecessor.book_id=o.book_id
WHERE o.operation_kind='reversal' AND NOT EXISTS (SELECT 1 FROM investment_dividend_facts_v25 f WHERE f.operation_id=o.id);
DROP TABLE investment_dividend_facts;
ALTER TABLE investment_dividend_facts_v25 RENAME TO investment_dividend_facts;
CREATE INDEX investment_dividend_facts_commodity ON investment_dividend_facts(book_id, commodity_id, operation_id);
-- +goose StatementBegin
CREATE TRIGGER investment_dividend_facts_insert_guard BEFORE INSERT ON investment_dividend_facts BEGIN
  SELECT RAISE(ABORT, 'invalid dividend instrument fact') WHERE NOT EXISTS (
    SELECT 1 FROM investment_operations o WHERE o.id=NEW.operation_id AND o.book_id=NEW.book_id
      AND o.created_audit_event_id=NEW.created_audit_event_id
      AND (NEW.commodity_id IS NULL OR EXISTS (SELECT 1 FROM commodities c
        WHERE c.id=NEW.commodity_id AND c.book_id=o.book_id AND c.kind IN ('security','crypto')))
      AND (o.operation_kind='dividend' OR (o.operation_kind='reversal' AND EXISTS (
        SELECT 1 FROM investment_dividend_facts predecessor
        WHERE predecessor.operation_id=o.correction_of_operation_id AND predecessor.book_id=o.book_id
          AND predecessor.commodity_id IS NEW.commodity_id AND predecessor.withholding_only=NEW.withholding_only)))
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

CREATE VIEW valid_investment_dividend_facts AS
SELECT f.* FROM investment_dividend_facts f JOIN investment_operations o ON o.id=f.operation_id AND o.book_id=f.book_id
WHERE o.created_audit_event_id=f.created_audit_event_id
  AND (f.commodity_id IS NULL OR EXISTS (SELECT 1 FROM commodities c WHERE c.id=f.commodity_id AND c.book_id=f.book_id AND c.kind IN ('security','crypto')))
  AND ((o.operation_kind='dividend' AND
    ((f.withholding_only=0 AND EXISTS (SELECT 1 FROM investment_operation_components c WHERE c.operation_id=o.id AND c.component_kind='dividend_gross'))
    OR (f.withholding_only=1 AND EXISTS (SELECT 1 FROM investment_operation_components c WHERE c.operation_id=o.id AND c.component_kind='withholding')
      AND NOT EXISTS (SELECT 1 FROM investment_operation_components c WHERE c.operation_id=o.id AND c.component_kind='dividend_gross'))))
  OR (o.operation_kind='reversal' AND EXISTS (SELECT 1 FROM investment_dividend_facts predecessor
    WHERE predecessor.operation_id=o.correction_of_operation_id AND predecessor.book_id=o.book_id
      AND predecessor.commodity_id IS f.commodity_id AND predecessor.withholding_only=f.withholding_only)));

-- One purpose: identity of cash payments across corrections, including inverse
-- journals. Reuse it for both cash families' candidate and commit checks.
CREATE VIEW investment_cash_payment_identity AS
WITH RECURSIVE ancestry(book_id, operation_id, ancestor_operation_id) AS (
  SELECT book_id, operation_id, operation_id FROM investment_dividend_facts
  UNION SELECT book_id, operation_id, operation_id FROM investment_payment_in_lieu_facts
  UNION
  SELECT a.book_id, a.operation_id, parent.correction_of_operation_id
  FROM ancestry a JOIN investment_operations parent ON parent.id=a.ancestor_operation_id AND parent.book_id=a.book_id
  WHERE parent.correction_of_operation_id IS NOT NULL
)
SELECT DISTINCT a.book_id, a.operation_id, root.id AS root_operation_id, v.transaction_id
FROM ancestry a JOIN investment_operations root ON root.id=a.ancestor_operation_id AND root.book_id=a.book_id
JOIN investment_operation_journal_links l ON l.operation_id=a.operation_id AND l.book_id=a.book_id AND l.role IN ('primary','reversal')
JOIN transaction_versions v ON v.id=l.transaction_version_id AND v.book_id=a.book_id
WHERE root.correction_of_operation_id IS NULL;
-- Existing evidence is preserved even if it exposes an old duplicate claim;
-- self-check reports it. New claims are guarded atomically, across both columns.
-- +goose StatementBegin
CREATE TRIGGER investment_event_payment_identity_insert BEFORE INSERT ON investment_event_suggestions
WHEN NEW.generated_transaction_id IS NOT NULL OR NEW.linked_transaction_id IS NOT NULL BEGIN
  SELECT RAISE(ABORT, 'cash payment already claimed by a provider event') WHERE EXISTS (
    SELECT 1 FROM investment_cash_payment_identity target
    JOIN investment_cash_payment_identity previous ON previous.book_id=target.book_id AND previous.root_operation_id=target.root_operation_id
    JOIN investment_event_suggestions other ON other.book_id=previous.book_id
      AND (other.generated_transaction_id=previous.transaction_id OR other.linked_transaction_id=previous.transaction_id)
    WHERE target.book_id=NEW.book_id AND target.transaction_id IN (NEW.generated_transaction_id,NEW.linked_transaction_id));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER investment_event_payment_identity_update BEFORE UPDATE OF generated_transaction_id,linked_transaction_id ON investment_event_suggestions
WHEN NEW.generated_transaction_id IS NOT NULL OR NEW.linked_transaction_id IS NOT NULL BEGIN
  SELECT RAISE(ABORT, 'cash payment already claimed by a provider event') WHERE EXISTS (
    SELECT 1 FROM investment_cash_payment_identity target
    JOIN investment_cash_payment_identity previous ON previous.book_id=target.book_id AND previous.root_operation_id=target.root_operation_id
    JOIN investment_event_suggestions other ON other.book_id=previous.book_id AND other.id<>NEW.id
      AND (other.generated_transaction_id=previous.transaction_id OR other.linked_transaction_id=previous.transaction_id)
    WHERE target.book_id=NEW.book_id AND target.transaction_id IN (NEW.generated_transaction_id,NEW.linked_transaction_id));
END;
-- +goose StatementEnd

-- +goose Down
-- The old non-null representation cannot retain explicit unassigned facts.
CREATE TEMP TABLE dividend_attribution_downgrade_guard (ok INTEGER CHECK (ok=1));
INSERT INTO dividend_attribution_downgrade_guard SELECT NOT EXISTS (SELECT 1 FROM investment_dividend_facts WHERE commodity_id IS NULL);
DROP TABLE dividend_attribution_downgrade_guard;
DROP TRIGGER investment_event_payment_identity_insert;
DROP TRIGGER investment_event_payment_identity_update;
DROP VIEW investment_cash_payment_identity;
DROP VIEW valid_investment_dividend_facts;
CREATE TABLE investment_dividend_facts_v24 (
  operation_id INTEGER PRIMARY KEY REFERENCES investment_operations(id), book_id INTEGER NOT NULL REFERENCES books(id),
  commodity_id INTEGER NOT NULL REFERENCES commodities(id), withholding_only INTEGER NOT NULL DEFAULT 0 CHECK (withholding_only IN (0,1)),
  created_audit_event_id INTEGER NOT NULL REFERENCES audit_events(id)
);
INSERT INTO investment_dividend_facts_v24 SELECT * FROM investment_dividend_facts;
DROP TABLE investment_dividend_facts;
ALTER TABLE investment_dividend_facts_v24 RENAME TO investment_dividend_facts;
CREATE INDEX investment_dividend_facts_commodity ON investment_dividend_facts(book_id,commodity_id,operation_id);
-- +goose StatementBegin
CREATE TRIGGER investment_dividend_facts_insert_guard BEFORE INSERT ON investment_dividend_facts BEGIN
  SELECT RAISE(ABORT, 'invalid dividend instrument fact') WHERE NOT EXISTS (
    SELECT 1 FROM investment_operations o JOIN commodities c ON c.id=NEW.commodity_id AND c.book_id=o.book_id
    WHERE o.id=NEW.operation_id AND o.book_id=NEW.book_id AND c.kind IN ('security','crypto')
      AND o.created_audit_event_id=NEW.created_audit_event_id
      AND (o.operation_kind='dividend' OR (o.operation_kind='reversal' AND EXISTS (
        SELECT 1 FROM investment_dividend_facts predecessor WHERE predecessor.operation_id=o.correction_of_operation_id
          AND predecessor.commodity_id=NEW.commodity_id AND predecessor.withholding_only=NEW.withholding_only))));
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
