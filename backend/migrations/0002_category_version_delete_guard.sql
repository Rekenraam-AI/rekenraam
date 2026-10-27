-- +goose Up
-- #417: a used custom category's account version is refused at the database
-- like any other used account's. The baseline exempted every non-built-in
-- category from the usage checks, so SQL could delete posted history that the
-- category service already refuses to delete. Unused categories, built-in,
-- starter or custom, still delete through the shared unused-account branch.

-- +goose StatementBegin
DROP TRIGGER account_versions_no_delete;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER account_versions_no_delete
BEFORE DELETE ON account_versions
WHEN NOT (
  NOT EXISTS (
    SELECT 1
    FROM accounts a
    WHERE a.id = OLD.account_id
      AND a.system_role IS NOT NULL
  )
  AND NOT EXISTS (
    SELECT 1
    FROM posting_versions pv
    WHERE pv.account_id = OLD.account_id
  )
  AND NOT EXISTS (
    SELECT 1
    FROM account_versions child
    WHERE child.parent_account_id = OLD.account_id
  )
)
BEGIN
  SELECT RAISE(ABORT, 'account_versions rows are append-only');
END;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DROP TRIGGER account_versions_no_delete;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER account_versions_no_delete
BEFORE DELETE ON account_versions
WHEN NOT (
  (
    OLD.account_class IN ('income', 'expense')
    AND OLD.account_kind = OLD.account_class
    AND json_extract(OLD.metadata_json, '$.category.type') = OLD.account_class
    AND json_extract(OLD.metadata_json, '$.category.is_builtin') = 0
  )
  OR (
    NOT EXISTS (
      SELECT 1
      FROM accounts a
      WHERE a.id = OLD.account_id
        AND a.system_role IS NOT NULL
    )
    AND NOT EXISTS (
      SELECT 1
      FROM posting_versions pv
      WHERE pv.account_id = OLD.account_id
    )
    AND NOT EXISTS (
      SELECT 1
      FROM account_versions child
      WHERE child.parent_account_id = OLD.account_id
    )
  )
)
BEGIN
  SELECT RAISE(ABORT, 'account_versions rows are append-only');
END;
-- +goose StatementEnd
