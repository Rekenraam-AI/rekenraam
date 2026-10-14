-- +goose Up
-- #104: which observations a derived price was computed from used to be
-- recorded only inside price_observations.derivation_json, so following the
-- graph meant a json_each scan of the whole table per node. This table indexes
-- the same links, written in the transaction that inserts the derived
-- observation. derivation_json stays the human-readable record; this table is
-- what the void cascade and the self-check follow.
--
-- A source always exists before what is derived from it, so a link points to a
-- smaller id: the graph cannot contain a cycle.
CREATE TABLE price_observation_dependencies (
  book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE RESTRICT,
  observation_id INTEGER NOT NULL REFERENCES price_observations(id) ON DELETE RESTRICT,
  source_observation_id INTEGER NOT NULL REFERENCES price_observations(id) ON DELETE RESTRICT,
  PRIMARY KEY (observation_id, source_observation_id),
  CHECK (source_observation_id < observation_id)
);

-- "What is derived from this?" — the void cascade's question.
CREATE INDEX price_observation_dependencies_source_idx
  ON price_observation_dependencies (book_id, source_observation_id, observation_id);

-- +goose StatementBegin
CREATE TRIGGER price_observation_dependencies_valid
BEFORE INSERT ON price_observation_dependencies
WHEN NOT EXISTS (
  SELECT 1 FROM price_observations po
  WHERE po.id = NEW.observation_id AND po.book_id = NEW.book_id AND po.is_derived = 1
) OR NOT EXISTS (
  SELECT 1 FROM price_observations src
  WHERE src.id = NEW.source_observation_id AND src.book_id = NEW.book_id
)
BEGIN
  SELECT RAISE(ABORT, 'price dependency must link a derived observation to a source in the same book');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER price_observation_dependencies_no_update
BEFORE UPDATE ON price_observation_dependencies
BEGIN
  SELECT RAISE(ABORT, 'price dependencies are immutable');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER price_observation_dependencies_no_delete
BEFORE DELETE ON price_observation_dependencies
BEGIN
  SELECT RAISE(ABORT, 'price dependencies are immutable');
END;
-- +goose StatementEnd

-- Index the links existing derived observations already name. This is the
-- leg shape the old cascade followed: an integer observation_id in each
-- element of a $.legs array, on an is_derived observation. A leg naming no
-- earlier observation in the book was never followed and is not indexed.
-- derivation_json carries no JSON CHECK, so rows are filtered before any JSON
-- function reads them (CASE is evaluated lazily; a MATERIALIZED CTE is not
-- flattened into the join).
INSERT INTO price_observation_dependencies (book_id, observation_id, source_observation_id)
WITH derived AS MATERIALIZED (
  SELECT id, book_id, derivation_json
  FROM price_observations
  WHERE is_derived = 1
    AND CASE WHEN json_valid(derivation_json) THEN json_type(derivation_json, '$.legs') END = 'array'
), legs AS MATERIALIZED (
  SELECT d.book_id, d.id, json_extract(leg.value, '$.observation_id') AS source_id
  FROM derived d
  JOIN json_each(d.derivation_json, '$.legs') leg
  WHERE CASE WHEN leg.type = 'object' THEN json_type(leg.value, '$.observation_id') END = 'integer'
)
SELECT DISTINCT legs.book_id, legs.id, src.id
FROM legs
JOIN price_observations src
  ON src.id = legs.source_id AND src.book_id = legs.book_id AND src.id < legs.id;

-- +goose Down
DROP TABLE IF EXISTS price_observation_dependencies;
