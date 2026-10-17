-- +goose Up
-- #292: closed_on is the last day an account is open
-- (docs/design/accounts-system-design.md). A close writes a version with
-- status 'closed' that takes effect on closed_on, so requiring status 'active'
-- refused the closing day itself. A closed version now accepts postings dated
-- on or before its closed_on, which (because the version starts on closed_on)
-- is exactly the closing day. db.PostingAccountRule.AcceptsPostingsOn applies
-- the same rule before a write reaches this trigger.

-- +goose StatementBegin
DROP TRIGGER IF EXISTS posting_versions_account_version_valid;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posting_versions_account_version_valid
BEFORE INSERT ON posting_versions
WHEN NOT EXISTS (
  SELECT 1
  FROM journal_entries je
  JOIN account_versions av ON av.account_id = NEW.account_id
  WHERE je.id = NEW.journal_entry_id
    AND av.id = (
      SELECT asof_av.id FROM account_versions asof_av
      WHERE asof_av.account_id = NEW.account_id AND asof_av.effective_from <= je.entry_date
      ORDER BY asof_av.effective_from DESC, asof_av.version_seq DESC LIMIT 1
    )
    AND (
      av.status = 'active'
      OR (av.status = 'closed' AND av.closed_on IS NOT NULL AND je.entry_date <= av.closed_on)
    )
    AND av.allows_postings = 1
    AND (av.opened_on IS NULL OR je.entry_date >= av.opened_on)
    AND (av.closed_on IS NULL OR je.entry_date <= av.closed_on)
    AND (av.default_commodity_id IS NULL OR av.default_commodity_id = NEW.commodity_id)
    AND (av.quantity_scale_override IS NULL OR NEW.quantity_scale <= av.quantity_scale_override)
)
BEGIN
  SELECT RAISE(ABORT, 'posting account is not eligible on entry date');
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS posting_versions_account_version_valid;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER posting_versions_account_version_valid
BEFORE INSERT ON posting_versions
WHEN NOT EXISTS (
  SELECT 1
  FROM journal_entries je
  JOIN account_versions av ON av.account_id = NEW.account_id
  WHERE je.id = NEW.journal_entry_id
    AND av.id = (
      SELECT asof_av.id FROM account_versions asof_av
      WHERE asof_av.account_id = NEW.account_id AND asof_av.effective_from <= je.entry_date
      ORDER BY asof_av.effective_from DESC, asof_av.version_seq DESC LIMIT 1
    )
    AND av.status = 'active' AND av.allows_postings = 1
    AND (av.opened_on IS NULL OR je.entry_date >= av.opened_on)
    AND (av.closed_on IS NULL OR je.entry_date <= av.closed_on)
    AND (av.default_commodity_id IS NULL OR av.default_commodity_id = NEW.commodity_id)
    AND (av.quantity_scale_override IS NULL OR NEW.quantity_scale <= av.quantity_scale_override)
)
BEGIN
  SELECT RAISE(ABORT, 'posting account is not eligible on entry date');
END;
-- +goose StatementEnd
