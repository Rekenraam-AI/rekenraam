package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const changeReasonMaxBytes = 500

// cleanEffectiveFrom defaults and bounds a version date by the owner's day
// (SettingsService.OwnerTodayAt), never the server's UTC day.
func cleanEffectiveFrom(value string, today string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return today, nil
	}

	parsed, err := time.Parse(time.DateOnly, cleaned)
	if err != nil {
		return "", ValidationError{Message: "effective date must use YYYY-MM-DD"}
	}

	todayDate, _ := time.Parse(time.DateOnly, today)
	if parsed.After(todayDate) {
		return "", ValidationError{Message: "effective date must not be in the future"}
	}

	return cleaned, nil
}

func cleanChangeReason(value string, fallback string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		cleaned = fallback
	}
	if len(cleaned) > changeReasonMaxBytes {
		return "", ValidationError{Message: fmt.Sprintf("change reason must be at most %d bytes", changeReasonMaxBytes)}
	}
	for _, r := range cleaned {
		if unicode.IsControl(r) {
			return "", ValidationError{Message: "change reason must not contain control characters"}
		}
	}

	return cleaned, nil
}

func cleanSizedJSONObject(value string, field string, maxBytes int) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return "{}", nil
	}
	if !json.Valid([]byte(cleaned)) {
		return "", ValidationError{Message: fmt.Sprintf("%s must be valid JSON", field)}
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(cleaned)); err != nil {
		return "", ValidationError{Message: fmt.Sprintf("%s must be valid JSON", field)}
	}
	cleaned = compact.String()
	if cleaned[0] != '{' {
		return "", ValidationError{Message: fmt.Sprintf("%s must be a JSON object", field)}
	}
	if len(cleaned) > maxBytes {
		return "", ValidationError{Message: fmt.Sprintf("%s must be at most %d bytes", field, maxBytes)}
	}

	return cleaned, nil
}
