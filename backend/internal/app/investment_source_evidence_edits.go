package app

import (
	"encoding/json"
	"strings"
)

// Corporate-action corrections (#311) carry source evidence the browser
// cannot round-trip: JSON numbers beyond 2^53, high-precision decimals and
// any metadata the form does not show. A correction form therefore sends only
// the keys its owner edited as source_evidence_edits, and the server applies
// them to the recorded evidence's raw tokens. A key set to null is removed,
// any other value replaces that top-level key exactly, and every key the edits
// do not name keeps its recorded token, nested data included. A full
// source_evidence remains an explicit whole replacement; sending both is
// refused, and sending neither keeps the recorded evidence.

// correctedSourceEvidence resolves a replacement's evidence from the recorded
// evidence, an optional whole replacement and optional top-level edits.
func correctedSourceEvidence(recorded, replacement, edits string) (string, error) {
	replacement = strings.TrimSpace(replacement)
	edits = strings.TrimSpace(edits)
	switch {
	case replacement != "" && edits != "":
		return "", ValidationError{Message: "send either source evidence or source evidence edits, not both"}
	case replacement != "":
		return replacement, nil
	case edits == "":
		return recorded, nil
	}
	return applySourceEvidenceEdits(recorded, edits)
}

func applySourceEvidenceEdits(recorded, edits string) (string, error) {
	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(edits), &patch); err != nil || patch == nil {
		return "", ValidationError{Message: "source evidence edits must be a JSON object"}
	}
	evidence := make(map[string]json.RawMessage)
	if strings.TrimSpace(recorded) != "" {
		if err := json.Unmarshal([]byte(recorded), &evidence); err != nil {
			return "", ValidationError{Message: "recorded source evidence must be a JSON object"}
		}
		if evidence == nil {
			evidence = make(map[string]json.RawMessage)
		}
	}
	for key, value := range patch {
		if strings.TrimSpace(string(value)) == "null" {
			delete(evidence, key)
			continue
		}
		evidence[key] = value
	}
	// RawMessage values are written back as their own tokens, so no number
	// is reparsed; only top-level key order is normalized.
	merged, err := json.Marshal(evidence)
	if err != nil {
		return "", ValidationError{Message: "source evidence edits must be valid JSON"}
	}
	return string(merged), nil
}
