package app

import (
	"context"
	"testing"
	"time"

	"rekenraam/backend/internal/totp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The #262 reauthentication budget: password confirmations and activation
// codes on the MFA settings share one 5-in-15 budget per session.

func wrongPassword(sessionID int64) MFAPasswordInput {
	return MFAPasswordInput{UserID: 1, SessionID: sessionID, Password: "wrong-password", ClientIP: "198.51.100.4"}
}

func rightPassword(sessionID int64) MFAPasswordInput {
	return MFAPasswordInput{UserID: 1, SessionID: sessionID, Password: "correct-horse-battery", ClientIP: "198.51.100.4"}
}

func countAuthEvents(t *testing.T, service *AuthService, eventType string, reason string) int {
	t.Helper()
	events, err := service.AuthenticationEvents(context.Background(), 200)
	require.NoError(t, err)
	count := 0
	for _, event := range events.Events {
		if event.EventType == eventType && event.FailureReason == reason {
			count++
		}
	}
	return count
}

func TestReauthBudgetIsSharedAcrossMFASettingsOperations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	_, service := newMFATestService(t, now)
	enrollAndActivate(t, service, now)

	// Switching operation must not reset or bypass the budget.
	_, err := service.RegenerateRecoveryCodes(ctx, wrongPassword(1))
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.RegenerateRecoveryCodes(ctx, wrongPassword(1))
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	assert.ErrorIs(t, service.DisableMFA(ctx, wrongPassword(1)), ErrInvalidCredentials)
	assert.ErrorIs(t, service.DisableMFA(ctx, wrongPassword(1)), ErrInvalidCredentials)
	_, err = service.BeginTOTPEnrollment(ctx, wrongPassword(1))
	var rateLimited RateLimitError
	require.ErrorAs(t, err, &rateLimited, "the fifth wrong confirmation exhausts the shared budget")
	assert.Equal(t, loginThrottleWindow, rateLimited.RetryAfter)

	// The correct password is refused too while blocked, before any hash runs,
	// and nothing is mutated.
	failedBefore := countAuthEvents(t, service, authEventLoginFailed, authFailureInvalidCredentials)
	err = service.DisableMFA(ctx, rightPassword(1))
	require.ErrorAs(t, err, &rateLimited)
	assert.Equal(t, loginThrottleWindow, rateLimited.RetryAfter)
	_, err = service.RegenerateRecoveryCodes(ctx, rightPassword(1))
	assert.ErrorIs(t, err, ErrRateLimited)
	_, err = service.RegenerateRecoveryCodes(ctx, wrongPassword(1))
	assert.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, failedBefore, countAuthEvents(t, service, authEventLoginFailed, authFailureInvalidCredentials),
		"a blocked attempt must not reach password verification")
	assert.Equal(t, 3, countAuthEvents(t, service, authEventLoginBlocked, authFailureRateLimited))

	status, err := service.MFAStatusFor(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "active", status.Status)
	assert.Equal(t, mfaRecoveryCodeCount, status.RecoveryCodesRemaining)
}

func TestReauthBlockExpiresAfterTheWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	_, service := newMFATestService(t, now)
	enrollAndActivate(t, service, now)

	for range loginThrottleMaxFailures {
		_, _ = service.RegenerateRecoveryCodes(ctx, wrongPassword(1))
	}
	service.now = func() time.Time { return now.Add(loginThrottleWindow - time.Minute) }
	_, err := service.RegenerateRecoveryCodes(ctx, rightPassword(1))
	var rateLimited RateLimitError
	require.ErrorAs(t, err, &rateLimited)
	assert.Equal(t, time.Minute, rateLimited.RetryAfter, "Retry-After counts down to the block's end")

	service.now = func() time.Time { return now.Add(loginThrottleWindow + time.Second) }
	codes, err := service.RegenerateRecoveryCodes(ctx, rightPassword(1))
	require.NoError(t, err)
	assert.Len(t, codes, mfaRecoveryCodeCount)
}

// A successful confirmation clears the session's failures, as a successful
// login clears the username budget, so an owner's occasional typo never adds
// up to a block.
func TestReauthSuccessClearsTheSessionBudget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	_, service := newMFATestService(t, now)
	enrollAndActivate(t, service, now)

	for range loginThrottleMaxFailures - 1 {
		_, err := service.RegenerateRecoveryCodes(ctx, wrongPassword(1))
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	_, err := service.RegenerateRecoveryCodes(ctx, rightPassword(1))
	require.NoError(t, err)
	for range loginThrottleMaxFailures - 1 {
		_, err := service.RegenerateRecoveryCodes(ctx, wrongPassword(1))
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
}

// An attacker holding a stolen session must not be able to lock the real
// owner out: the block belongs to that session, and neither the owner's other
// sessions nor sign-in are affected.
func TestReauthBlockIsConfinedToTheSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	_, service := newMFATestService(t, now)

	for range loginThrottleMaxFailures {
		_, _ = service.BeginTOTPEnrollment(ctx, wrongPassword(7))
	}
	_, err := service.BeginTOTPEnrollment(ctx, rightPassword(7))
	require.ErrorIs(t, err, ErrRateLimited)

	enrollment, err := service.BeginTOTPEnrollment(ctx, rightPassword(8))
	require.NoError(t, err, "another session keeps its own budget")
	require.NotEmpty(t, enrollment.Secret)

	result, err := service.Login(ctx, LoginInput{Username: "owner", Password: "correct-horse-battery", ClientIP: "198.51.100.4"})
	require.NoError(t, err, "settings reauthentication must not spend the sign-in budget")
	assert.NotEmpty(t, result.SessionToken)
}

func TestWrongActivationCodesSpendTheReauthBudget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	_, service := newMFATestService(t, now)

	enrollment, err := service.BeginTOTPEnrollment(ctx, rightPassword(1))
	require.NoError(t, err)

	// Two wrong passwords and three wrong codes share one budget.
	require.ErrorIs(t, service.DisableMFA(ctx, wrongPassword(1)), ErrInvalidCredentials)
	require.ErrorIs(t, service.DisableMFA(ctx, wrongPassword(1)), ErrInvalidCredentials)
	for range loginThrottleMaxFailures - 3 {
		_, err = service.ActivateTOTP(ctx, ActivateMFAInput{UserID: 1, SessionID: 1, Code: "000000"})
		require.ErrorIs(t, err, ErrMFACodeInvalid)
	}
	_, err = service.ActivateTOTP(ctx, ActivateMFAInput{UserID: 1, SessionID: 1, Code: "000000"})
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, loginThrottleMaxFailures-2, countAuthEvents(t, service, authEventLoginFailed, authFailureMFAInvalid))

	// A correct code while blocked is refused and the enrollment stays pending:
	// no recovery codes are minted.
	code, err := totp.Code(enrollment.Secret, totp.Step(now))
	require.NoError(t, err)
	codes, err := service.ActivateTOTP(ctx, ActivateMFAInput{UserID: 1, SessionID: 1, Code: code})
	require.ErrorIs(t, err, ErrRateLimited)
	assert.Empty(t, codes)
	status, err := service.MFAStatusFor(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "pending", status.Status)
	assert.Zero(t, status.RecoveryCodesRemaining)

	later := now.Add(loginThrottleWindow + time.Second)
	service.now = func() time.Time { return later }
	code, err = totp.Code(enrollment.Secret, totp.Step(later))
	require.NoError(t, err)
	codes, err = service.ActivateTOTP(ctx, ActivateMFAInput{UserID: 1, SessionID: 1, Code: code})
	require.NoError(t, err)
	assert.Len(t, codes, mfaRecoveryCodeCount)
}

// Recovery is independent of a blocked session: the documented way out is
// signing in again, and recovery codes still complete that sign-in.
func TestRecoveryCodeSignInIsIndependentOfABlockedSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	_, service := newMFATestService(t, now)

	enrollment, err := service.BeginTOTPEnrollment(ctx, rightPassword(1))
	require.NoError(t, err)
	activationCode, err := totp.Code(enrollment.Secret, totp.Step(now)-1)
	require.NoError(t, err)
	recoveryCodes, err := service.ActivateTOTP(ctx, ActivateMFAInput{UserID: 1, SessionID: 1, Code: activationCode})
	require.NoError(t, err)

	for range loginThrottleMaxFailures {
		_ = service.DisableMFA(ctx, wrongPassword(1))
	}
	require.ErrorIs(t, service.DisableMFA(ctx, rightPassword(1)), ErrRateLimited)

	challenge, err := service.Login(ctx, LoginInput{Username: "owner", Password: "correct-horse-battery", ClientIP: "198.51.100.4"})
	require.NoError(t, err)
	completed, err := service.CompleteLoginMFA(ctx, CompleteMFALoginInput{
		ChallengeToken: challenge.MFAChallengeToken, Code: recoveryCodes[0], ClientIP: "198.51.100.4",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, completed.SessionToken)
}

func TestReauthRequiresASession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	_, service := newMFATestService(t, now)

	_, err := service.BeginTOTPEnrollment(ctx, MFAPasswordInput{UserID: 1, Password: "correct-horse-battery"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidCredentials)
	_, err = service.ActivateTOTP(ctx, ActivateMFAInput{UserID: 1, Code: "000000"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrMFACodeInvalid)
}
