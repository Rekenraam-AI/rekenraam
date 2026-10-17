package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #262: the MFA settings share one per-session reauthentication budget, and an
// exhausted budget answers with the sign-in throttle's 429 envelope.
func TestMFASettingsReauthIsRateLimitedAcrossEndpoints(t *testing.T) {
	t.Parallel()

	handler, _ := newSetupTestHandler(t)
	sessionCookie, csrfToken := createOwnerSession(t, handler)
	enrollMFAForSession(t, handler, sessionCookie, csrfToken)

	wrong := `{"password":"not-the-password"}`
	for _, path := range []string{
		"/api/v1/auth/mfa/recovery-codes",
		"/api/v1/auth/mfa/recovery-codes",
		"/api/v1/auth/mfa/disable",
		"/api/v1/auth/mfa/disable",
	} {
		res := postJSONWithSession(t, handler, sessionCookie, csrfToken, path, wrong)
		require.Equal(t, http.StatusUnauthorized, res.Code, path)
	}
	exhausted := postJSONWithSession(t, handler, sessionCookie, csrfToken, "/api/v1/auth/mfa/totp/enroll", wrong)
	assertRateLimited(t, exhausted)

	// Correct input on any of the operations is refused while blocked, and
	// nothing changes.
	for _, path := range []string{
		"/api/v1/auth/mfa/disable",
		"/api/v1/auth/mfa/recovery-codes",
	} {
		res := postJSONWithSession(t, handler, sessionCookie, csrfToken, path, `{"password":"test-password"}`)
		assert.NotContains(t, res.Body.String(), "recovery_codes")
		assertRateLimited(t, res)
	}
	activate := postJSONWithSession(t, handler, sessionCookie, csrfToken, "/api/v1/auth/mfa/totp/activate", `{"code":"000000"}`)
	assertRateLimited(t, activate)

	status := readMFAStatus(t, handler, sessionCookie)
	assert.Equal(t, "active", status.Status)
	assert.Equal(t, 10, status.RecoveryCodesRemaining)
}

// The block belongs to the session that spent it: the owner signs in again and
// the new session has its own budget.
func TestMFASettingsReauthBlockDoesNotLockOutANewSession(t *testing.T) {
	t.Parallel()

	handler, _ := newSetupTestHandler(t)
	stolenCookie, stolenCSRF := createOwnerSession(t, handler)

	for range 5 {
		postJSONWithSession(t, handler, stolenCookie, stolenCSRF, "/api/v1/auth/mfa/totp/enroll", `{"password":"not-the-password"}`)
	}
	blocked := postJSONWithSession(t, handler, stolenCookie, stolenCSRF, "/api/v1/auth/mfa/totp/enroll", `{"password":"test-password"}`)
	assert.NotContains(t, blocked.Body.String(), "otpauth")
	assertRateLimited(t, blocked)

	loginRes := postLogin(t, handler, `{"username":"owner","password":"test-password"}`)
	require.Equal(t, http.StatusOK, loginRes.Code, "the settings budget is not the sign-in budget")
	ownerCookie := sessionCookieFrom(t, loginRes.Result().Cookies())
	sessionReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	sessionReq.AddCookie(ownerCookie)
	sessionRes := httptest.NewRecorder()
	handler.ServeHTTP(sessionRes, sessionReq)
	var session authSessionResponse
	require.NoError(t, json.NewDecoder(sessionRes.Body).Decode(&session))

	enrolled := postJSONWithSession(t, handler, ownerCookie, session.CSRFToken, "/api/v1/auth/mfa/totp/enroll", `{"password":"test-password"}`)
	assert.Equal(t, http.StatusCreated, enrolled.Code)
}

func assertRateLimited(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, res.Code)
	retryAfter, err := strconv.Atoi(res.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.Positive(t, retryAfter)
	assert.LessOrEqual(t, retryAfter, 15*60)
	var body errorResponse
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, "RATE_LIMITED", body.Error.Code)
}
