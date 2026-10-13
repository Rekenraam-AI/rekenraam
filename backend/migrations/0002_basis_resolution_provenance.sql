-- +goose Up
CREATE TABLE investment_basis_resolution_provenance (
  id INTEGER PRIMARY KEY,
  book_id INTEGER NOT NULL REFERENCES books(id),
  resolution_operation_id INTEGER NOT NULL REFERENCES investment_basis_resolutions(operation_id),
  revision_seq INTEGER NOT NULL CHECK (revision_seq > 0),
  source_evidence_json TEXT NOT NULL CHECK (json_valid(source_evidence_json) AND json_type(source_evidence_json) = 'object'),
  created_audit_event_id INTEGER NOT NULL UNIQUE REFERENCES audit_events(id),
  UNIQUE (resolution_operation_id, revision_seq)
);

-- +goose StatementBegin
CREATE TRIGGER investment_basis_resolution_provenance_valid
BEFORE INSERT ON investment_basis_resolution_provenance
WHEN NEW.revision_seq <> COALESCE((SELECT MAX(p.revision_seq) FROM investment_basis_resolution_provenance p
  WHERE p.resolution_operation_id = NEW.resolution_operation_id), 0) + 1
OR NOT EXISTS (SELECT 1 FROM investment_basis_resolutions r
  JOIN effective_investment_operations o ON o.id = r.operation_id
  JOIN audit_events a ON a.id = NEW.created_audit_event_id
  WHERE r.operation_id = NEW.resolution_operation_id AND r.book_id = NEW.book_id
    AND a.book_id = NEW.book_id AND a.operation = 'investment.basis_resolution.provenance'
    AND length(trim(a.reason)) > 0 AND a.actor_user_id IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT, 'invalid basis resolution provenance lineage or audit');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER investment_basis_resolution_provenance_no_update
BEFORE UPDATE ON investment_basis_resolution_provenance
BEGIN
  SELECT RAISE(ABORT, 'basis resolution provenance is immutable');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER investment_basis_resolution_provenance_no_delete
BEFORE DELETE ON investment_basis_resolution_provenance
BEGIN
  SELECT RAISE(ABORT, 'basis resolution provenance is immutable');
END;
-- +goose StatementEnd

CREATE VIEW latest_investment_basis_resolution_provenance AS
SELECT p.* FROM investment_basis_resolution_provenance p
WHERE NOT EXISTS (SELECT 1 FROM investment_basis_resolution_provenance newer
  WHERE newer.resolution_operation_id = p.resolution_operation_id AND newer.revision_seq > p.revision_seq);

-- +goose Down
DROP VIEW latest_investment_basis_resolution_provenance;
DROP TRIGGER investment_basis_resolution_provenance_no_delete;
DROP TRIGGER investment_basis_resolution_provenance_no_update;
DROP TRIGGER investment_basis_resolution_provenance_valid;
DROP TABLE investment_basis_resolution_provenance;
