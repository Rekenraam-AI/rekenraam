-- Cursor continuations only cover the unvisited suffix. A posted transaction
-- must not coalesce against a running scan, a suffix, or a targeted retry.
-- A never-attempted full scan still safely coalesces new demand (ADR 0010).
-- +goose Up
DROP TRIGGER fx_work_after_posting_version_insert;
-- +goose StatementBegin
CREATE TRIGGER fx_work_after_posting_version_insert
AFTER INSERT ON posting_versions
WHEN EXISTS (SELECT 1 FROM transaction_versions tv WHERE tv.id = NEW.transaction_version_id AND tv.status = 'posted')
  AND EXISTS (SELECT 1 FROM commodities c WHERE c.id = NEW.commodity_id AND c.kind = 'currency')
  AND NEW.commodity_id != COALESCE(
    (SELECT base_commodity_id FROM pricing_policies WHERE book_id = NEW.book_id),
    (SELECT default_currency_commodity_id FROM books WHERE id = NEW.book_id),
    0
  )
  AND NOT EXISTS (
    SELECT 1 FROM background_work_items bw
    WHERE bw.book_id = NEW.book_id AND bw.kind = 'pricing.fx_coverage'
      AND bw.status = 'pending' AND bw.attempts = 0
      AND json_extract(bw.payload_json, '$.next') IS NULL
      AND json_extract(bw.payload_json, '$.retry') IS NULL
      AND json_extract(bw.payload_json, '$.currency_id') = NEW.commodity_id
      AND json_extract(bw.payload_json, '$.start_date') <= (SELECT entry_date FROM journal_entries WHERE id = NEW.journal_entry_id)
  )
BEGIN
  INSERT INTO background_work_items (book_id, kind, payload_json, available_at, created_at, updated_at)
  VALUES (
    NEW.book_id, 'pricing.fx_coverage',
    json_object('reason', 'transaction_entered', 'start_date', (SELECT entry_date FROM journal_entries WHERE id = NEW.journal_entry_id), 'currency_id', NEW.commodity_id),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  )
  ON CONFLICT(book_id, kind, payload_json) WHERE status IN ('pending', 'running') DO NOTHING;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER fx_work_after_posting_version_insert;
-- +goose StatementBegin
CREATE TRIGGER fx_work_after_posting_version_insert
AFTER INSERT ON posting_versions
WHEN EXISTS (SELECT 1 FROM transaction_versions tv WHERE tv.id = NEW.transaction_version_id AND tv.status = 'posted')
  AND EXISTS (SELECT 1 FROM commodities c WHERE c.id = NEW.commodity_id AND c.kind = 'currency')
  AND NEW.commodity_id != COALESCE(
    (SELECT base_commodity_id FROM pricing_policies WHERE book_id = NEW.book_id),
    (SELECT default_currency_commodity_id FROM books WHERE id = NEW.book_id),
    0
  )
  AND NOT EXISTS (
    SELECT 1 FROM background_work_items bw
    WHERE bw.book_id = NEW.book_id AND bw.kind = 'pricing.fx_coverage'
      AND bw.status IN ('pending', 'running')
      AND json_extract(bw.payload_json, '$.currency_id') = NEW.commodity_id
      AND json_extract(bw.payload_json, '$.start_date') <= (SELECT entry_date FROM journal_entries WHERE id = NEW.journal_entry_id)
  )
BEGIN
  INSERT INTO background_work_items (book_id, kind, payload_json, available_at, created_at, updated_at)
  VALUES (
    NEW.book_id, 'pricing.fx_coverage',
    json_object('reason', 'transaction_entered', 'start_date', (SELECT entry_date FROM journal_entries WHERE id = NEW.journal_entry_id), 'currency_id', NEW.commodity_id),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
  )
  ON CONFLICT(book_id, kind, payload_json) WHERE status IN ('pending', 'running') DO NOTHING;
END;
-- +goose StatementEnd
