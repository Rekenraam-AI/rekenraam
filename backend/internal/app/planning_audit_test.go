package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rekenraam/backend/internal/db"
)

// #276: recurring "today", the forecast as-of date and the default budget
// month must name the same owner-local calendar date, including at DST
// transitions and month ends. Expected dates are read off a wall clock.
func TestPlanningAuditOwnerLocalDateAgreesAcrossDSTAndMonthEnds(t *testing.T) {
	t.Parallel()
	f, recurring, input := recurringFixture(t)
	ctx := context.Background()
	budgets := NewBudgetService(db.NewBudgetRepository(f.database), recurring.settings)
	for _, test := range []struct {
		name, zone string
		at         time.Time
		date       string
	}{
		// 2026-10-25 01:00 UTC is the CEST→CET switch: 22:30 UTC the day before is 00:30 CEST.
		{"Amsterdam just after midnight before fall-back", "Europe/Amsterdam", time.Date(2026, 10, 24, 22, 30, 0, 0, time.UTC), "2026-10-25"},
		{"Amsterdam 23:30 on the 25-hour day", "Europe/Amsterdam", time.Date(2026, 10, 25, 22, 30, 0, 0, time.UTC), "2026-10-25"},
		{"Amsterdam after fall-back midnight", "Europe/Amsterdam", time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC), "2026-10-26"},
		{"Amsterdam month end becomes November", "Europe/Amsterdam", time.Date(2026, 10, 31, 23, 30, 0, 0, time.UTC), "2026-11-01"},
		{"Los Angeles is still the previous month", "America/Los_Angeles", time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC), "2026-10-31"},
		// Auckland springs forward 2026-09-27 02:00 NZST (14:00 UTC on the 26th).
		{"Auckland spring-forward day", "Pacific/Auckland", time.Date(2026, 9, 26, 14, 30, 0, 0, time.UTC), "2026-09-27"},
		{"Auckland 23:30 NZDT", "Pacific/Auckland", time.Date(2026, 9, 27, 10, 30, 0, 0, time.UTC), "2026-09-27"},
	} {
		_, err := recurring.settings.SavePreferences(ctx, SaveUserPreferencesInput{UserID: input.OwnerUserID, TimeZone: test.zone})
		require.NoError(t, err)
		recurring.now = func() time.Time { return test.at }
		budgets.SetNowForTest(func() time.Time { return test.at })
		today, err := recurring.localToday(ctx, input.OwnerUserID)
		require.NoError(t, err)
		assert.Equal(t, test.date, today, test.name)
		bounds, err := forecastDateBounds(test.at, test.zone, 1)
		require.NoError(t, err)
		assert.Equal(t, test.date, bounds.AsOf, test.name)
		month, err := budgets.CurrentMonth(ctx, input.OwnerUserID)
		require.NoError(t, err)
		assert.Equal(t, test.date[:8]+"01", month.PeriodStart, test.name)
	}
}
