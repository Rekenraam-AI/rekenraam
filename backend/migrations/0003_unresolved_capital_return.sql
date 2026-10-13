-- +goose Up
-- #169: retain receipt and entitlement, distinguish unknown reduction/excess.
-- Rebuild only leaf tables; existing known evidence is copied unchanged.
CREATE TEMP TABLE saved_investment_capital_return_effects AS SELECT * FROM investment_capital_return_effects;
DROP TRIGGER investment_capital_return_effects_valid;
DROP TRIGGER investment_capital_return_effects_no_update;
DROP TRIGGER investment_capital_return_effects_no_delete;
DROP TABLE investment_capital_return_effects;
CREATE TABLE IF NOT EXISTS investment_capital_return_effects (
  operation_id INTEGER NOT NULL REFERENCES investment_capital_return_facts(operation_id) ON DELETE RESTRICT,
  effect_seq INTEGER NOT NULL CHECK (effect_seq > 0),
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  lot_id INTEGER NOT NULL REFERENCES investment_lots(id) ON DELETE RESTRICT,
  lot_event_id INTEGER NOT NULL UNIQUE REFERENCES investment_lot_events(id) ON DELETE RESTRICT,
  entitled_quantity_value TEXT NOT NULL CHECK (instr(entitled_quantity_value, char(0)) = 0 AND (length(entitled_quantity_value) BETWEEN 1 AND 38 AND entitled_quantity_value NOT GLOB '*[^0-9]*' AND entitled_quantity_value NOT GLOB '0*')),
  entitled_quantity_scale INTEGER NOT NULL CHECK (entitled_quantity_scale BETWEEN 0 AND 24),
  allocated_value TEXT NOT NULL CHECK (instr(allocated_value, char(0)) = 0 AND (allocated_value = '0' OR (length(allocated_value) BETWEEN 1 AND 38 AND allocated_value NOT GLOB '*[^0-9]*' AND allocated_value NOT GLOB '0*'))),
  allocated_scale INTEGER NOT NULL CHECK (allocated_scale BETWEEN 0 AND 12),
  reduction_value TEXT CHECK (instr(reduction_value, char(0)) = 0 AND (reduction_value = '0' OR (length(reduction_value) BETWEEN 1 AND 38 AND reduction_value NOT GLOB '*[^0-9]*' AND reduction_value NOT GLOB '0*'))),
  reduction_scale INTEGER CHECK (reduction_scale BETWEEN 0 AND 12),
  excess_value TEXT CHECK (instr(excess_value, char(0)) = 0 AND (excess_value = '0' OR (length(excess_value) BETWEEN 1 AND 38 AND excess_value NOT GLOB '*[^0-9]*' AND excess_value NOT GLOB '0*'))),
  excess_scale INTEGER CHECK (excess_scale BETWEEN 0 AND 12),
  basis_knowledge TEXT NOT NULL DEFAULT 'known' CHECK (basis_knowledge IN ('known', 'unknown')),
  CHECK ((basis_knowledge = 'known' AND reduction_value IS NOT NULL AND reduction_scale IS NOT NULL
      AND excess_value IS NOT NULL AND excess_scale IS NOT NULL)
    OR (basis_knowledge = 'unknown' AND reduction_value IS NULL AND reduction_scale IS NULL
      AND excess_value IS NULL AND excess_scale IS NULL)),
  PRIMARY KEY (operation_id, effect_seq),
  UNIQUE (operation_id, lot_id)
);
INSERT INTO investment_capital_return_effects (operation_id, effect_seq, book_id, lot_id, lot_event_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale) SELECT operation_id, effect_seq, book_id, lot_id, lot_event_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale FROM saved_investment_capital_return_effects;
DROP TABLE saved_investment_capital_return_effects;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_effects_valid
BEFORE INSERT ON investment_capital_return_effects
WHEN NOT EXISTS (
  SELECT 1 FROM investment_capital_return_facts f
  JOIN investment_lots l ON l.id = NEW.lot_id
  JOIN investment_lot_events e ON e.id = NEW.lot_event_id
  WHERE f.operation_id = NEW.operation_id AND f.book_id = NEW.book_id
    AND l.book_id = f.book_id AND l.account_id = f.account_id AND l.commodity_id = f.commodity_id
    AND l.cost_commodity_id = f.cost_commodity_id AND l.position_side = 'long' AND l.opened_on <= f.effective_on
    AND e.basis_knowledge = NEW.basis_knowledge
    AND e.lot_id = l.id AND e.event_kind = 'basis_reduction' AND e.event_date = f.effective_on
)
BEGIN SELECT RAISE(ABORT, 'investment capital return effect is outside its operation or lot'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_effects_no_update
BEFORE UPDATE ON investment_capital_return_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return effects are immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_effects_no_delete
BEFORE DELETE ON investment_capital_return_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return effects are immutable'); END;
-- +goose StatementEnd
CREATE TEMP TABLE saved_investment_capital_return_revision_effects AS SELECT * FROM investment_capital_return_revision_effects;
DROP TRIGGER investment_capital_return_revision_effects_valid;
DROP TRIGGER investment_capital_return_revision_effects_no_update;
DROP TRIGGER investment_capital_return_revision_effects_no_delete;
DROP TABLE investment_capital_return_revision_effects;
CREATE TABLE IF NOT EXISTS investment_capital_return_revision_effects (
  revision_id INTEGER NOT NULL REFERENCES investment_capital_return_revisions(id) ON DELETE RESTRICT,
  effect_seq INTEGER NOT NULL CHECK (effect_seq > 0),
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  lot_id INTEGER NOT NULL REFERENCES investment_lots(id) ON DELETE RESTRICT,
  entitled_quantity_value TEXT NOT NULL CHECK (instr(entitled_quantity_value, char(0)) = 0 AND (length(entitled_quantity_value) BETWEEN 1 AND 38 AND entitled_quantity_value NOT GLOB '*[^0-9]*' AND entitled_quantity_value NOT GLOB '0*')),
  entitled_quantity_scale INTEGER NOT NULL CHECK (entitled_quantity_scale BETWEEN 0 AND 24),
  allocated_value TEXT NOT NULL CHECK (instr(allocated_value, char(0)) = 0 AND (allocated_value = '0' OR (length(allocated_value) BETWEEN 1 AND 38 AND allocated_value NOT GLOB '*[^0-9]*' AND allocated_value NOT GLOB '0*'))),
  allocated_scale INTEGER NOT NULL CHECK (allocated_scale BETWEEN 0 AND 12),
  reduction_value TEXT CHECK (instr(reduction_value, char(0)) = 0 AND (reduction_value = '0' OR (length(reduction_value) BETWEEN 1 AND 38 AND reduction_value NOT GLOB '*[^0-9]*' AND reduction_value NOT GLOB '0*'))),
  reduction_scale INTEGER CHECK (reduction_scale BETWEEN 0 AND 12),
  excess_value TEXT CHECK (instr(excess_value, char(0)) = 0 AND (excess_value = '0' OR (length(excess_value) BETWEEN 1 AND 38 AND excess_value NOT GLOB '*[^0-9]*' AND excess_value NOT GLOB '0*'))),
  excess_scale INTEGER CHECK (excess_scale BETWEEN 0 AND 12),
  basis_knowledge TEXT NOT NULL DEFAULT 'known' CHECK (basis_knowledge IN ('known', 'unknown')),
  CHECK ((basis_knowledge = 'known' AND reduction_value IS NOT NULL AND reduction_scale IS NOT NULL
      AND excess_value IS NOT NULL AND excess_scale IS NOT NULL)
    OR (basis_knowledge = 'unknown' AND reduction_value IS NULL AND reduction_scale IS NULL
      AND excess_value IS NULL AND excess_scale IS NULL)),
  PRIMARY KEY (revision_id, effect_seq),
  UNIQUE (revision_id, lot_id)
);
INSERT INTO investment_capital_return_revision_effects (revision_id, effect_seq, book_id, lot_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale) SELECT revision_id, effect_seq, book_id, lot_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale FROM saved_investment_capital_return_revision_effects;
DROP TABLE saved_investment_capital_return_revision_effects;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_revision_effects_valid
BEFORE INSERT ON investment_capital_return_revision_effects
WHEN NOT EXISTS (
  SELECT 1 FROM investment_capital_return_revisions r
  JOIN investment_capital_return_facts f ON f.operation_id = r.operation_id
  JOIN investment_lots l ON l.id = NEW.lot_id
  WHERE r.id = NEW.revision_id AND r.book_id = NEW.book_id
    AND l.book_id = f.book_id AND l.account_id = f.account_id AND l.commodity_id = f.commodity_id
    AND l.cost_commodity_id = f.cost_commodity_id AND l.position_side = 'long' AND l.opened_on <= f.effective_on
)
BEGIN SELECT RAISE(ABORT, 'investment capital return revision effect is outside its operation or lot'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_revision_effects_no_update
BEFORE UPDATE ON investment_capital_return_revision_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return revision effects are immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_revision_effects_no_delete
BEFORE DELETE ON investment_capital_return_revision_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return revision effects are immutable'); END;
-- +goose StatementEnd

-- +goose Down
-- Refuse to discard unknown evidence on downgrade.
CREATE TEMP TABLE capital_return_downgrade_guard (knowledge TEXT CHECK (knowledge = 'known'));
INSERT INTO capital_return_downgrade_guard SELECT basis_knowledge FROM investment_capital_return_effects;
INSERT INTO capital_return_downgrade_guard SELECT basis_knowledge FROM investment_capital_return_revision_effects;
DROP TABLE capital_return_downgrade_guard;
CREATE TEMP TABLE saved_investment_capital_return_effects AS SELECT operation_id, effect_seq, book_id, lot_id, lot_event_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale FROM investment_capital_return_effects;
DROP TRIGGER investment_capital_return_effects_valid;
DROP TRIGGER investment_capital_return_effects_no_update;
DROP TRIGGER investment_capital_return_effects_no_delete;
DROP TABLE investment_capital_return_effects;
CREATE TABLE IF NOT EXISTS investment_capital_return_effects (
  operation_id INTEGER NOT NULL REFERENCES investment_capital_return_facts(operation_id) ON DELETE RESTRICT,
  effect_seq INTEGER NOT NULL CHECK (effect_seq > 0),
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  lot_id INTEGER NOT NULL REFERENCES investment_lots(id) ON DELETE RESTRICT,
  lot_event_id INTEGER NOT NULL UNIQUE REFERENCES investment_lot_events(id) ON DELETE RESTRICT,
  entitled_quantity_value TEXT NOT NULL CHECK (instr(entitled_quantity_value, char(0)) = 0 AND (length(entitled_quantity_value) BETWEEN 1 AND 38 AND entitled_quantity_value NOT GLOB '*[^0-9]*' AND entitled_quantity_value NOT GLOB '0*')),
  entitled_quantity_scale INTEGER NOT NULL CHECK (entitled_quantity_scale BETWEEN 0 AND 24),
  allocated_value TEXT NOT NULL CHECK (instr(allocated_value, char(0)) = 0 AND (allocated_value = '0' OR (length(allocated_value) BETWEEN 1 AND 38 AND allocated_value NOT GLOB '*[^0-9]*' AND allocated_value NOT GLOB '0*'))),
  allocated_scale INTEGER NOT NULL CHECK (allocated_scale BETWEEN 0 AND 12),
  reduction_value TEXT NOT NULL CHECK (instr(reduction_value, char(0)) = 0 AND (reduction_value = '0' OR (length(reduction_value) BETWEEN 1 AND 38 AND reduction_value NOT GLOB '*[^0-9]*' AND reduction_value NOT GLOB '0*'))),
  reduction_scale INTEGER NOT NULL CHECK (reduction_scale BETWEEN 0 AND 12),
  excess_value TEXT NOT NULL CHECK (instr(excess_value, char(0)) = 0 AND (excess_value = '0' OR (length(excess_value) BETWEEN 1 AND 38 AND excess_value NOT GLOB '*[^0-9]*' AND excess_value NOT GLOB '0*'))),
  excess_scale INTEGER NOT NULL CHECK (excess_scale BETWEEN 0 AND 12),
  PRIMARY KEY (operation_id, effect_seq),
  UNIQUE (operation_id, lot_id)
);
INSERT INTO investment_capital_return_effects (operation_id, effect_seq, book_id, lot_id, lot_event_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale) SELECT operation_id, effect_seq, book_id, lot_id, lot_event_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale FROM saved_investment_capital_return_effects;
DROP TABLE saved_investment_capital_return_effects;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_effects_valid
BEFORE INSERT ON investment_capital_return_effects
WHEN NOT EXISTS (
  SELECT 1 FROM investment_capital_return_facts f
  JOIN investment_lots l ON l.id = NEW.lot_id
  JOIN investment_lot_events e ON e.id = NEW.lot_event_id
  WHERE f.operation_id = NEW.operation_id AND f.book_id = NEW.book_id
    AND l.book_id = f.book_id AND l.account_id = f.account_id AND l.commodity_id = f.commodity_id
    AND l.cost_commodity_id = f.cost_commodity_id AND l.position_side = 'long' AND l.opened_on <= f.effective_on
    AND e.lot_id = l.id AND e.event_kind = 'basis_reduction' AND e.event_date = f.effective_on
)
BEGIN SELECT RAISE(ABORT, 'investment capital return effect is outside its operation or lot'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_effects_no_update
BEFORE UPDATE ON investment_capital_return_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return effects are immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_effects_no_delete
BEFORE DELETE ON investment_capital_return_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return effects are immutable'); END;
-- +goose StatementEnd
CREATE TEMP TABLE saved_investment_capital_return_revision_effects AS SELECT revision_id, effect_seq, book_id, lot_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale FROM investment_capital_return_revision_effects;
DROP TRIGGER investment_capital_return_revision_effects_valid;
DROP TRIGGER investment_capital_return_revision_effects_no_update;
DROP TRIGGER investment_capital_return_revision_effects_no_delete;
DROP TABLE investment_capital_return_revision_effects;
CREATE TABLE IF NOT EXISTS investment_capital_return_revision_effects (
  revision_id INTEGER NOT NULL REFERENCES investment_capital_return_revisions(id) ON DELETE RESTRICT,
  effect_seq INTEGER NOT NULL CHECK (effect_seq > 0),
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  lot_id INTEGER NOT NULL REFERENCES investment_lots(id) ON DELETE RESTRICT,
  entitled_quantity_value TEXT NOT NULL CHECK (instr(entitled_quantity_value, char(0)) = 0 AND (length(entitled_quantity_value) BETWEEN 1 AND 38 AND entitled_quantity_value NOT GLOB '*[^0-9]*' AND entitled_quantity_value NOT GLOB '0*')),
  entitled_quantity_scale INTEGER NOT NULL CHECK (entitled_quantity_scale BETWEEN 0 AND 24),
  allocated_value TEXT NOT NULL CHECK (instr(allocated_value, char(0)) = 0 AND (allocated_value = '0' OR (length(allocated_value) BETWEEN 1 AND 38 AND allocated_value NOT GLOB '*[^0-9]*' AND allocated_value NOT GLOB '0*'))),
  allocated_scale INTEGER NOT NULL CHECK (allocated_scale BETWEEN 0 AND 12),
  reduction_value TEXT NOT NULL CHECK (instr(reduction_value, char(0)) = 0 AND (reduction_value = '0' OR (length(reduction_value) BETWEEN 1 AND 38 AND reduction_value NOT GLOB '*[^0-9]*' AND reduction_value NOT GLOB '0*'))),
  reduction_scale INTEGER NOT NULL CHECK (reduction_scale BETWEEN 0 AND 12),
  excess_value TEXT NOT NULL CHECK (instr(excess_value, char(0)) = 0 AND (excess_value = '0' OR (length(excess_value) BETWEEN 1 AND 38 AND excess_value NOT GLOB '*[^0-9]*' AND excess_value NOT GLOB '0*'))),
  excess_scale INTEGER NOT NULL CHECK (excess_scale BETWEEN 0 AND 12),
  PRIMARY KEY (revision_id, effect_seq),
  UNIQUE (revision_id, lot_id)
);
INSERT INTO investment_capital_return_revision_effects (revision_id, effect_seq, book_id, lot_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale) SELECT revision_id, effect_seq, book_id, lot_id, entitled_quantity_value, entitled_quantity_scale, allocated_value, allocated_scale, reduction_value, reduction_scale, excess_value, excess_scale FROM saved_investment_capital_return_revision_effects;
DROP TABLE saved_investment_capital_return_revision_effects;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_revision_effects_valid
BEFORE INSERT ON investment_capital_return_revision_effects
WHEN NOT EXISTS (
  SELECT 1 FROM investment_capital_return_revisions r
  JOIN investment_capital_return_facts f ON f.operation_id = r.operation_id
  JOIN investment_lots l ON l.id = NEW.lot_id
  WHERE r.id = NEW.revision_id AND r.book_id = NEW.book_id
    AND l.book_id = f.book_id AND l.account_id = f.account_id AND l.commodity_id = f.commodity_id
    AND l.cost_commodity_id = f.cost_commodity_id AND l.position_side = 'long' AND l.opened_on <= f.effective_on
)
BEGIN SELECT RAISE(ABORT, 'investment capital return revision effect is outside its operation or lot'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_revision_effects_no_update
BEFORE UPDATE ON investment_capital_return_revision_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return revision effects are immutable'); END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS investment_capital_return_revision_effects_no_delete
BEFORE DELETE ON investment_capital_return_revision_effects
BEGIN SELECT RAISE(ABORT, 'investment capital return revision effects are immutable'); END;
-- +goose StatementEnd
