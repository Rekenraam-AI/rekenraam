package app

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Declarations describe what the owner relied on; the app does not verify
// their truth. They are stored in the existing immutable evidence JSON.
func cleanBasisResolutionEvidence(value string) (string, error) {
	evidence, err := cleanSizedJSONObject(value, "source evidence", investmentJSONMaxBytes)
	if err != nil {
		return "", err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(evidence), &fields); err != nil {
		return "", ValidationError{Message: "source declaration fields must be text"}
	}
	// Struct decoding matches keys case-insensitively; read exact keys so
	// validation uses the same immutable fields as history and API consumers.
	text := func(key string) (string, error) {
		var value string
		if raw, present := fields[key]; present {
			if err := json.Unmarshal(raw, &value); err != nil {
				return "", ValidationError{Message: "source declaration fields must be text"}
			}
		}
		return value, nil
	}
	kind, err := text("kind")
	if err != nil {
		return "", err
	}
	switch kind {
	case "statement", "provider":
		reference, err := text("reference")
		if err != nil {
			return "", err
		}
		reference, err = cleanBasisSourceText(reference, "source reference", false)
		if err != nil {
			return "", err
		}
		if reference == "" {
			return "", ValidationError{Message: "source reference is required"}
		}
	case "owner_assertion":
		rationale, err := text("rationale")
		if err != nil {
			return "", err
		}
		rationale, err = cleanBasisSourceText(rationale, "source rationale", true)
		if err != nil {
			return "", err
		}
		if rationale == "" {
			return "", ValidationError{Message: "owner assertion rationale is required"}
		}
		knownOn, err := text("known_on")
		if err != nil {
			return "", err
		}
		if _, err := cleanRequiredDate(knownOn, "source knowledge date"); err != nil {
			return "", err
		}
	default:
		return "", ValidationError{Message: "source kind must be statement, provider or owner_assertion"}
	}
	// Preserve every supplied field in immutable evidence, including optional
	// provider metadata. Trimming is only for validating required text.
	return evidence, nil
}

func cleanBasisSourceText(value, field string, multiline bool) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > 500 {
		return "", ValidationError{Message: field + " must be at most 500 characters"}
	}
	// Keep the common control-character rules. The byte ceiling admits every
	// UTF-8 encoding of a text within this field's Unicode-character limit.
	if multiline {
		return cleanOptionalMultilineText(value, field, 500*utf8.UTFMax)
	}
	return cleanOptionalText(value, field, 500*utf8.UTFMax)
}
