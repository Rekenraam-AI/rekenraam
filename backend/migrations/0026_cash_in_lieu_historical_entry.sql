-- +goose Up
-- #394 / ADR 0019: the original typed parent is provenance for entry too.
-- Dated replay still admits quantities, basis and elections atomically.
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
)
BEGIN SELECT RAISE(ABORT, 'investment cash in lieu fact is outside its disposal or settled operation'); END;
-- +goose StatementEnd

-- +goose Down
-- Migration 24 can read these facts but cannot enter them. No facts, views or
-- history meaning change; restore its admission policy without rewriting data.
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
