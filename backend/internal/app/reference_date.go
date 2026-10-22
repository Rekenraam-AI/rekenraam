package app

import (
	"context"
	"fmt"
	"time"

	"rekenraam/backend/internal/db"
)

// referenceContext resolves the owner day before repository transactions. Nested
// reads share that day without another preference query, including read-only
// snapshots. A scheduler must establish a new context for each operation.
func (s *SettingsService) referenceContext(ctx context.Context, now time.Time) (context.Context, error) {
	if db.ReferenceDate(ctx) != "" {
		return ctx, nil
	}
	zone := "UTC"
	if s != nil {
		var err error
		zone, err = s.repository.OwnerTimeZone(ctx)
		if err != nil {
			return nil, err
		}
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("load owner time zone: %w", err)
	}
	return db.WithReferenceDate(ctx, now.In(location).Format(time.DateOnly)), nil
}
