package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthSessionDisclosesOwnerDateOnlyWhenAuthenticated(t *testing.T) {
	t.Parallel()
	handler, database := newSetupTestHandler(t)
	cookie := bootstrapOwner(t, handler)
	for _, zone := range []string{"Pacific/Kiritimati", "Pacific/Honolulu", "Europe/Amsterdam"} {
		t.Run(zone, func(t *testing.T) {
			_, err := database.ExecContext(context.Background(), `UPDATE user_preferences SET time_zone = ? WHERE user_id = 1`, zone)
			require.NoError(t, err)
			location, err := time.LoadLocation(zone)
			require.NoError(t, err)
			before := time.Now().In(location).Format(time.DateOnly)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
			req.AddCookie(cookie)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			require.Equal(t, http.StatusOK, res.Code)
			var body map[string]any
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			after := time.Now().In(location).Format(time.DateOnly)
			assert.Contains(t, []string{before, after}, body["owner_today"])
			assert.Equal(t, true, body["authenticated"])
		})
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	var body map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.NotContains(t, body, "owner_today")
}
