package api

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/app"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/testdb"
	"rekenraam/backend/internal/totp"
)

func cookiePolicyHandler(t *testing.T, lifetime time.Duration) (http.Handler, *sql.DB) {
	t.Helper()
	database, _ := testdb.Open(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auth := app.NewAuthServiceWithSessionLifetime(db.NewAuthRepository(database), logger, lifetime)
	auth.SetSecretKey([]byte("0123456789abcdef0123456789abcdef"))
	return NewHandler(logger, http.NotFoundHandler(), Services{
		Setup:    app.NewSetupService(db.NewSetupRepository(database)),
		Auth:     auth,
		Settings: app.NewSettingsService(db.NewSettingsRepository(database)),
	}, HandlerOptions{}), database
}

func TestLoginCookieLifetimeMatchesPersistedPolicy(t *testing.T) {
	t.Parallel()
	for _, lifetime := range []time.Duration{2 * time.Hour, app.SessionLifetime, 60 * 24 * time.Hour} {
		for _, withMFA := range []bool{false, true} {
			name := lifetime.String()
			if withMFA {
				name += "/mfa"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				handler, database := cookiePolicyHandler(t, lifetime)
				setupCookie := bootstrapOwner(t, handler)
				assert.Equal(t, int(app.SessionLifetime.Seconds()), setupCookie.MaxAge, "setup retains its documented default")
				secret := ""
				if withMFA {
					secret = enrollMFAForSession(t, handler, setupCookie, authSessionCSRFToken(t, handler, setupCookie))
				}
				res := postLogin(t, handler, `{"username":"owner","password":"test-password"}`)
				require.Equal(t, http.StatusOK, res.Code)
				if withMFA {
					assert.Nil(t, findCookie(res.Result().Cookies(), sessionCookieName))
					assert.Nil(t, findCookie(res.Result().Cookies(), trustedDeviceCookieName))
					challenge := findCookie(res.Result().Cookies(), mfaChallengeCookieName)
					require.NotNil(t, challenge)
					code, err := totp.Code(secret, totp.Step(time.Now().UTC()))
					require.NoError(t, err)
					res = postLoginMFA(t, handler, challenge, `{"code":"`+code+`"}`)
					require.Equal(t, http.StatusOK, res.Code)
				}
				cookie := sessionCookieFrom(t, res.Result().Cookies())
				assert.Equal(t, int(lifetime.Seconds()), cookie.MaxAge)
				var created, expires string
				require.NoError(t, database.QueryRowContext(context.Background(), `SELECT created_at, expires_at FROM auth_sessions ORDER BY id DESC LIMIT 1`).Scan(&created, &expires))
				start, err := time.Parse(time.RFC3339, created)
				require.NoError(t, err)
				end, err := time.Parse(time.RFC3339, expires)
				require.NoError(t, err)
				assert.Equal(t, end.Sub(start), time.Duration(cookie.MaxAge)*time.Second)
				assert.True(t, cookie.HttpOnly)
				assert.True(t, cookie.Secure)
				assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
				assert.Equal(t, "/", cookie.Path)
				assert.Empty(t, cookie.Domain)
			})
		}
	}
}

func TestSuccessfulLoginRenewsExistingApprovedDeviceCookie(t *testing.T) {
	t.Parallel()
	handler, database := cookiePolicyHandler(t, app.SessionLifetime)
	setupCookie := bootstrapOwner(t, handler)
	// Use the authenticated endpoint to mint one device, then reuse its cookie.
	first := postLogin(t, handler, `{"username":"owner","password":"test-password"}`)
	require.Equal(t, http.StatusOK, first.Code)
	device := findCookie(first.Result().Cookies(), trustedDeviceCookieName)
	require.NotNil(t, device)
	var before int
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM login_trusted_devices`).Scan(&before))
	for _, withMFA := range []bool{false, true} {
		secret := ""
		if withMFA {
			secret = enrollMFAForSession(t, handler, setupCookie, authSessionCSRFToken(t, handler, setupCookie))
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"owner","password":"test-password"}`))
		req.Header.Set("Content-Type", "application/json")
		setSameOrigin(req)
		req.AddCookie(device)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code)
		if withMFA {
			assert.Nil(t, findCookie(res.Result().Cookies(), trustedDeviceCookieName))
			code, err := totp.Code(secret, totp.Step(time.Now().UTC()))
			require.NoError(t, err)
			challenge := findCookie(res.Result().Cookies(), mfaChallengeCookieName)
			require.NotNil(t, challenge)
			req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/mfa", strings.NewReader(`{"code":"`+code+`"}`))
			req.Header.Set("Content-Type", "application/json")
			setSameOrigin(req)
			req.AddCookie(challenge)
			req.AddCookie(device)
			res = httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			require.Equal(t, http.StatusOK, res.Code)
		}
		renewed := findCookie(res.Result().Cookies(), trustedDeviceCookieName)
		require.NotNil(t, renewed, "sliding database approval also needs a refreshed browser lifetime")
		assert.Equal(t, device.Value, renewed.Value)
		assert.Equal(t, int(app.TrustedDeviceLifetime.Seconds()), renewed.MaxAge)
		var after int
		require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM login_trusted_devices`).Scan(&after))
		assert.Equal(t, before, after, "renewal must retain the existing device identity")
	}
}
