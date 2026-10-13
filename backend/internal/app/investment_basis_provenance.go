package app

import "encoding/json"

// Declarations describe what the owner relied on; the app does not verify
// their truth. They are stored in the existing immutable evidence JSON.
func cleanBasisResolutionEvidence(value string) (string, error) {
	evidence, err := cleanSizedJSONObject(value, "source evidence", investmentJSONMaxBytes)
	if err != nil {
		return "", err
	}
	var declaration struct {
		Kind      string `json:"kind"`
		Reference string `json:"reference"`
		Rationale string `json:"rationale"`
		KnownOn   string `json:"known_on"`
	}
	if err := json.Unmarshal([]byte(evidence), &declaration); err != nil {
		return "", ValidationError{Message: "source declaration fields must be text"}
	}
	switch declaration.Kind {
	case "statement", "provider":
		reference, err := cleanOptionalText(declaration.Reference, "source reference", 500)
		if err != nil {
			return "", err
		}
		if reference == "" {
			return "", ValidationError{Message: "source reference is required"}
		}
	case "owner_assertion":
		rationale, err := cleanOptionalMultilineText(declaration.Rationale, "source rationale", 500)
		if err != nil {
			return "", err
		}
		if rationale == "" {
			return "", ValidationError{Message: "owner assertion rationale is required"}
		}
		if _, err := cleanRequiredDate(declaration.KnownOn, "source knowledge date"); err != nil {
			return "", err
		}
	default:
		return "", ValidationError{Message: "source kind must be statement, provider or owner_assertion"}
	}
	// Preserve every supplied field in immutable evidence, including optional
	// provider metadata. Trimming is only for validating required text.
	return evidence, nil
}
