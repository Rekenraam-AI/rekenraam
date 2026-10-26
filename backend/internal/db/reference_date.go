package db

import (
	"context"
	"strings"
)

type referenceDateKey struct{}

// WithReferenceDate pins current reference-data selection to the application's
// owner-local financial day for one operation. Explicit historical predicates
// do not use this value. It is context-local, never connection or process state.
func WithReferenceDate(ctx context.Context, date string) context.Context {
	return context.WithValue(ctx, referenceDateKey{}, date)
}

func ReferenceDate(ctx context.Context) string {
	date, _ := ctx.Value(referenceDateKey{}).(string)
	return date
}

// CurrentReferenceVersions supplies the same effective-date/version-sequence
// ordering as the former UTC views, with the application's date.
// Unwired repository callers retain UTC selection; application operations
// establish their reference date before opening a transaction or read snapshot.
func CurrentReferenceVersions(ctx context.Context, kind string) string {
	date := ReferenceDate(ctx)
	dateSQL := "date('now')"
	if date != "" {
		dateSQL = "'" + strings.ReplaceAll(date, "'", "''") + "'"
	}
	table, key := kind+"_versions", kind+"_id"
	if kind == "investment_instrument" {
		key = "instrument_id"
	}
	return `(SELECT reference_version.* FROM ` + table + ` reference_version
		WHERE reference_version.id = (
			SELECT candidate.id FROM ` + table + ` candidate
			WHERE candidate.` + key + ` = reference_version.` + key + `
			  AND candidate.effective_from <= ` + dateSQL + `
			ORDER BY candidate.effective_from DESC, candidate.version_seq DESC LIMIT 1))`
}
