-- +goose Up
-- #392 / ADR 0019: financial replay validates the disposal; parent standing is attribution.
DROP TRIGGER investment_cash_in_lieu_facts_valid;
-- +goose StatementBegin
CREATE TRIGGER investment_cash_in_lieu_facts_valid
BEFORE INSERT ON investment_cash_in_lieu_facts
WHEN NOT EXISTS (
  SELECT 1 FROM investment_operations o
  JOIN investment_disposal_decisions d ON d.operation_id = o.id
  JOIN investment_operations settled ON settled.id = NEW.settled_operation_id AND settled.book_id = NEW.book_id
  WHERE o.id = NEW.operation_id AND o.book_id = NEW.book_id AND o.operation_kind = 'cash_in_lieu'
    AND settled.operation_kind IN ('split', 'share_exchange', 'cash_and_stock_merger', 'spin_off')
    AND o.created_audit_event_id = NEW.created_audit_event_id AND o.event_date <= NEW.payment_on
    AND length(NEW.payment_on) = 10 AND date(NEW.payment_on, '+0 days') IS NEW.payment_on
    AND (EXISTS (SELECT 1 FROM investment_split_facts s WHERE s.operation_id = settled.id
        AND d.account_id = s.account_id AND d.commodity_id = s.commodity_id AND d.event_date >= s.effective_on)
      OR EXISTS (SELECT 1 FROM investment_transfer_facts f WHERE f.operation_id = settled.id
        AND f.transfer_kind IN ('exchange', 'spin_off') AND d.account_id = f.destination_account_id
        AND d.commodity_id = f.destination_commodity_id AND d.event_date >= f.effective_on))
    -- New entry needs a current parent. A correction retains its exact
    -- original reference even after that parent has been corrected.
    AND (NOT EXISTS (SELECT 1 FROM investment_operations successor
      WHERE successor.correction_of_operation_id = settled.id)
      OR EXISTS (SELECT 1 FROM investment_cash_in_lieu_facts predecessor
        WHERE predecessor.operation_id = o.correction_of_operation_id
          AND predecessor.book_id = NEW.book_id AND predecessor.settled_operation_id = settled.id))
)
BEGIN SELECT RAISE(ABORT, 'investment cash in lieu fact is outside its disposal or settled operation'); END;
-- +goose StatementEnd

-- Follow only replacement ancestry within the supported parent families.
-- A reversal has no current explanatory parent. Original facts never change.
CREATE VIEW cash_in_lieu_parent_context AS
WITH RECURSIVE lineage(book_id, original_operation_id, operation_id) AS (
  SELECT book_id, id, id FROM investment_operations
    WHERE operation_kind IN ('split', 'share_exchange', 'cash_and_stock_merger', 'spin_off')
  UNION
  SELECT lineage.book_id, lineage.original_operation_id, successor.id
  FROM lineage JOIN investment_operations successor
    ON successor.correction_of_operation_id = lineage.operation_id AND successor.book_id = lineage.book_id
    AND successor.correction_mode = 'replace'
    AND successor.operation_kind IN ('split', 'share_exchange', 'cash_and_stock_merger', 'spin_off')
)
SELECT lineage.book_id, lineage.original_operation_id, current.id AS effective_operation_id,
  COALESCE(s.effective_on, t.effective_on) AS effective_on,
  COALESCE(s.account_id, t.destination_account_id) AS account_id,
  COALESCE(s.commodity_id, t.destination_commodity_id) AS commodity_id,
  v.transaction_id
FROM lineage JOIN effective_investment_operations current ON current.id = lineage.operation_id AND current.book_id = lineage.book_id
LEFT JOIN investment_split_facts s ON s.operation_id = current.id
LEFT JOIN investment_transfer_facts t ON t.operation_id = current.id AND t.transfer_kind IN ('exchange', 'spin_off')
LEFT JOIN investment_operation_journal_links l ON l.operation_id = current.id AND l.role = 'primary'
LEFT JOIN transaction_versions v ON v.id = l.transaction_version_id;

-- Historical reference integrity is separate from current explanatory support.
CREATE VIEW valid_investment_cash_in_lieu_facts AS
SELECT f.* FROM investment_cash_in_lieu_facts f
JOIN investment_operations o ON o.id = f.operation_id AND o.book_id = f.book_id AND o.operation_kind = 'cash_in_lieu'
JOIN investment_disposal_decisions d ON d.operation_id = o.id AND d.book_id = o.book_id
JOIN investment_operations parent ON parent.id = f.settled_operation_id AND parent.book_id = f.book_id
WHERE parent.operation_kind IN ('split', 'share_exchange', 'cash_and_stock_merger', 'spin_off')
  AND o.created_audit_event_id = f.created_audit_event_id AND o.event_date <= f.payment_on
  AND length(f.payment_on) = 10 AND date(f.payment_on, '+0 days') = f.payment_on
  AND (EXISTS (SELECT 1 FROM investment_split_facts s WHERE s.operation_id = parent.id
      AND d.account_id = s.account_id AND d.commodity_id = s.commodity_id AND d.event_date >= s.effective_on)
    OR EXISTS (SELECT 1 FROM investment_transfer_facts t WHERE t.operation_id = parent.id
      AND t.transfer_kind IN ('exchange', 'spin_off') AND d.account_id = t.destination_account_id
      AND d.commodity_id = t.destination_commodity_id AND d.event_date >= t.effective_on));

-- +goose Down
-- The old runtime would label retained disposals corrupt. Refuse loss of meaning.
CREATE TEMP TABLE cash_parent_downgrade_guard (ok INTEGER CHECK (ok = 1));
INSERT INTO cash_parent_downgrade_guard SELECT NOT EXISTS (
 SELECT 1 FROM effective_investment_operations o JOIN investment_cash_in_lieu_facts f ON f.operation_id = o.id
 WHERE NOT EXISTS (SELECT 1 FROM effective_investment_operations parent WHERE parent.id = f.settled_operation_id));
DROP TABLE cash_parent_downgrade_guard;
DROP VIEW valid_investment_cash_in_lieu_facts;
DROP VIEW cash_in_lieu_parent_context;
DROP TRIGGER investment_cash_in_lieu_facts_valid;
-- +goose StatementBegin
CREATE TRIGGER investment_cash_in_lieu_facts_valid
BEFORE INSERT ON investment_cash_in_lieu_facts
WHEN NOT EXISTS (
  SELECT 1 FROM investment_operations o
  JOIN investment_disposal_decisions d ON d.operation_id = o.id
  JOIN investment_operations settled ON settled.id = NEW.settled_operation_id AND settled.book_id = NEW.book_id
  WHERE o.id = NEW.operation_id AND o.book_id = NEW.book_id AND o.operation_kind = 'cash_in_lieu'
    AND o.created_audit_event_id = NEW.created_audit_event_id AND o.event_date <= NEW.payment_on
    AND (EXISTS (SELECT 1 FROM investment_split_facts s WHERE s.operation_id = settled.id
        AND d.account_id = s.account_id AND d.commodity_id = s.commodity_id AND d.event_date >= s.effective_on)
      OR EXISTS (SELECT 1 FROM investment_transfer_facts f WHERE f.operation_id = settled.id
        AND f.transfer_kind IN ('exchange', 'spin_off') AND d.account_id = f.destination_account_id
        AND d.commodity_id = f.destination_commodity_id AND d.event_date >= f.effective_on))
    -- The operation it settles must still be effective (T-147 seam fence).
    AND NOT EXISTS (SELECT 1 FROM investment_operations successor
      WHERE successor.correction_of_operation_id = settled.id)
)
BEGIN SELECT RAISE(ABORT, 'investment cash in lieu fact is outside its disposal or settled operation'); END;
-- +goose StatementEnd
