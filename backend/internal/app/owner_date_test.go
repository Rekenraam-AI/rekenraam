package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
)

func TestOwnerTodayUsesServerClockAndSavedZone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, instant, zone, date string }{
		{"east of UTC midnight", "2026-10-13T23:30:00Z", "Pacific/Kiritimati", "2026-10-14"},
		{"west of UTC midnight", "2026-10-14T00:30:00Z", "Pacific/Honolulu", "2026-10-13"},
		{"owner year ahead", "2026-12-31T23:30:00Z", "Europe/Amsterdam", "2027-01-01"},
		{"owner year behind", "2027-01-01T00:30:00Z", "America/Los_Angeles", "2026-12-31"},
		{"before DST jump", "2026-03-29T00:30:00Z", "Europe/Amsterdam", "2026-03-29"},
		{"after DST jump", "2026-03-29T01:30:00Z", "Europe/Amsterdam", "2026-03-29"},
		{"before DST repeat", "2026-10-25T00:30:00Z", "Europe/Amsterdam", "2026-10-25"},
		{"after DST repeat", "2026-10-25T01:30:00Z", "Europe/Amsterdam", "2026-10-25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newInvestmentsTestFixture(t)
			service := NewSettingsService(db.NewSettingsRepository(f.database))
			now, err := time.Parse(time.RFC3339, tc.instant)
			require.NoError(t, err)
			service.now = func() time.Time { return now }
			_, err = service.SavePreferences(context.Background(), SaveUserPreferencesInput{UserID: f.ownerUserID, TimeZone: tc.zone})
			require.NoError(t, err)
			today, err := service.OwnerToday(context.Background(), f.ownerUserID)
			require.NoError(t, err)
			assert.Equal(t, tc.date, today)
		})
	}
}

func TestOwnerTodayRefreshesZoneAndClockWithoutWriting(t *testing.T) {
	t.Parallel()
	f := newInvestmentsTestFixture(t)
	ctx := context.Background()
	service := NewSettingsService(db.NewSettingsRepository(f.database))
	now := time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	_, err := service.SavePreferences(ctx, SaveUserPreferencesInput{UserID: f.ownerUserID, TimeZone: "Europe/Amsterdam"})
	require.NoError(t, err)
	today, err := service.OwnerToday(ctx, f.ownerUserID)
	require.NoError(t, err)
	assert.Equal(t, "2027-01-01", today)
	_, err = service.SavePreferences(ctx, SaveUserPreferencesInput{UserID: f.ownerUserID, TimeZone: "Pacific/Honolulu"})
	require.NoError(t, err)
	today, err = service.OwnerToday(ctx, f.ownerUserID)
	require.NoError(t, err)
	assert.Equal(t, "2026-12-31", today)
	now = now.Add(24 * time.Hour)
	today, err = service.OwnerToday(ctx, f.ownerUserID)
	require.NoError(t, err)
	assert.Equal(t, "2027-01-01", today)
	var before int
	require.NoError(t, f.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events`).Scan(&before))
	_, err = service.OwnerToday(ctx, f.ownerUserID)
	require.NoError(t, err)
	var after int
	require.NoError(t, f.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events`).Scan(&after))
	assert.Equal(t, before, after)
	_, err = f.database.ExecContext(ctx, `UPDATE user_preferences SET time_zone = 'Not/AZone' WHERE user_id = ?`, f.ownerUserID)
	require.NoError(t, err)
	_, err = service.OwnerToday(ctx, f.ownerUserID)
	require.Error(t, err)
	// Match the existing Preferences contract when no row is stored.
	_, err = f.database.ExecContext(ctx, `DELETE FROM user_preferences WHERE user_id = ?`, f.ownerUserID)
	require.NoError(t, err)
	today, err = service.OwnerToday(ctx, f.ownerUserID)
	require.NoError(t, err)
	assert.Equal(t, "2027-01-01", today)
}
