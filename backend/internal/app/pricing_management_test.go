package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriceHistoryTraversesMoreThanFiveHundredRowsWithBoundCursor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, service := newPricingRefreshTestService(t, []string{"USD", "EUR"}, nil)
	input := ManualPriceInput{OwnerUserID: 1, BaseCommodityID: 2, QuoteCommodityID: 1, ValuationDate: "2026-06-12", PriceValue: "123", PriceScale: 2, BaseQuantityValue: "1", Reason: "history fixture"}
	originals := map[int64]bool{}
	for range 503 {
		price, err := service.CreateManualPrice(ctx, input)
		require.NoError(t, err)
		originals[price.ID] = true
	}
	page, err := service.History(ctx, 2, 1, true, "", 50)
	require.NoError(t, err)
	require.Len(t, page.Prices, 50)
	require.NotEmpty(t, page.NextCursor)
	_, err = service.History(ctx, 1, 2, true, page.NextCursor, 50)
	require.ErrorAs(t, err, new(ValidationError))
	_, err = service.History(ctx, 2, 1, false, page.NextCursor, 50)
	require.ErrorAs(t, err, new(ValidationError))
	_, err = service.History(ctx, 2, 1, true, "bad cursor", 50)
	require.ErrorAs(t, err, new(ValidationError))
	_, err = service.History(ctx, 2, 1, true, "", 101)
	require.ErrorAs(t, err, new(ValidationError))
	// A newer insertion between pages cannot duplicate or hide an original row.
	_, err = service.CreateManualPrice(ctx, input)
	require.NoError(t, err)
	seen := map[int64]bool{}
	for {
		for _, price := range page.Prices {
			require.False(t, seen[price.ID])
			require.True(t, originals[price.ID])
			seen[price.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		page, err = service.History(ctx, 2, 1, true, page.NextCursor, 50)
		require.NoError(t, err)
	}
	assert.Len(t, seen, 503)
}

func TestPricingWorkspaceDefaultsToOwnerLocalDateAndDiscoversMissingFX(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, service := newPricingRefreshTestService(t, []string{"USD", "EUR"}, nil)
	_, err := database.Exec(`INSERT INTO user_preferences(user_id,time_zone,created_at,created_by_user_id,updated_at,updated_by_user_id) VALUES(1,'Europe/Amsterdam','2026-06-12T00:00:00Z',1,'2026-06-12T00:00:00Z',1) ON CONFLICT(user_id) DO UPDATE SET time_zone='Europe/Amsterdam'`)
	require.NoError(t, err)
	service.SetNowForTest(func() time.Time { return time.Date(2026, 6, 12, 22, 30, 0, 0, time.UTC) })
	workspace, err := service.Workspace(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "2026-06-13", workspace.AsOf)
	require.Len(t, workspace.Coverage, 1)
	assert.Equal(t, int64(2), workspace.Coverage[0].BaseCommodityID)
	assert.Equal(t, int64(1), workspace.Coverage[0].QuoteCommodityID)
	assert.Equal(t, "missing", workspace.Coverage[0].Status)
	old, err := service.CreateManualPrice(ctx, ManualPriceInput{OwnerUserID: 1, BaseCommodityID: 2, QuoteCommodityID: 1, ValuationDate: "0001-01-01", PriceValue: "1", BaseQuantityValue: "1", Reason: "historical assertion"})
	require.NoError(t, err)
	workspace, err = service.Workspace(ctx, "2026-06-13")
	require.NoError(t, err)
	assert.Equal(t, "stale", workspace.Coverage[0].Status)
	require.NotNil(t, workspace.Coverage[0].AgeDays)
	assert.Greater(t, *workspace.Coverage[0].AgeDays, 700000)
	assert.Equal(t, old.ID, workspace.Coverage[0].Observation.ID)
}
