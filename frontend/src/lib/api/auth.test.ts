import { afterEach, describe, expect, it, vi } from 'vitest';
import { apiClient } from './client';
import { activateMFATOTP, disableMFA, enrollMFATOTP, regenerateMFARecoveryCodes } from './auth';
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
