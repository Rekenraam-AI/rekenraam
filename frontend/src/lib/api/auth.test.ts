import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import {
  activateMFATOTP,
  completeLoginMFA,
  disableMFA,
  enrollMFATOTP,
  regenerateMFARecoveryCodes
} from './auth';
import { getFormErrorState } from '#lib/form-errors.ts';
import { m } from '#lib/paraglide/messages.js';

afterEach(() => vi.restoreAllMocks());

describe('MFA settings client', () => {
  // #262: an exhausted reauthentication budget answers 429 on every settings
  // operation; the screen shows the shared rate-limit feedback, never secrets.
  it.each([
    ['enroll', () => enrollMFATOTP('password', 'test-csrf')],
    ['activate', () => activateMFATOTP('123456', 'test-csrf')],
    ['disable', () => disableMFA('password', 'test-csrf')],
    ['regenerate', () => regenerateMFARecoveryCodes('password', 'test-csrf')]
  ])('surfaces a rate-limited %s as RATE_LIMITED', async (_name, call) => {
    vi.spyOn(apiClient, 'POST').mockResolvedValue({
      error: { error: { code: 'RATE_LIMITED', message: 'too many attempts, try again later' } },
      response: new Response(null, { status: 429, headers: { 'Retry-After': '900' } })
    });
    const failure = await call().then(() => null, (error: unknown) => error);
    expect(failure).toMatchObject({ status: 429, code: 'RATE_LIMITED' });
    expect(getFormErrorState(failure)?.message).toBe(m.api_error_rate_limited());
  });
});

describe('MFA sign-in client', () => {
  // #290: when the server cannot open the authenticator secret, the code step
  // must point at the recovery codes rather than report a wrong code.
  it('surfaces an unverifiable authenticator as a recovery-code prompt', async () => {
    vi.spyOn(apiClient, 'POST').mockResolvedValue({
      error: {
        error: {
          code: 'MFA_AUTHENTICATOR_UNAVAILABLE',
          message: 'authenticator codes cannot be checked on this server, use a recovery code'
        }
      },
      response: new Response(null, { status: 401 })
    });
    const failure = await completeLoginMFA('123456').then(
      () => null,
      (error: unknown) => error
    );
    expect(failure).toMatchObject({ status: 401, code: 'MFA_AUTHENTICATOR_UNAVAILABLE' });
    const message = getFormErrorState(failure)?.message;
    expect(message).toBe(m.api_error_mfa_authenticator_unavailable());
    expect(message).not.toBe(m.api_error_unauthenticated());
  });
});
