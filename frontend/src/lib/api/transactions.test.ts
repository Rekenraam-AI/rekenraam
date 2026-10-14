import { describe, expect, it } from 'vitest';
import { APIClientError } from './client';
import { isReconciliationOverrideRequired } from './transactions';

describe('isReconciliationOverrideRequired', () => {
  it('recognises the reconciliation-override refusal', () => {
    expect(isReconciliationOverrideRequired(new APIClientError({
      status: 409, code: 'CONFLICT', detail: 'reconciliation override is required'
    }))).toBe(true);
  });

  it('leaves other conflicts and errors alone', () => {
    expect(isReconciliationOverrideRequired(new APIClientError({ status: 409, code: 'CONFLICT', detail: 'stale version' }))).toBe(false);
    expect(isReconciliationOverrideRequired(new APIClientError({ status: 400, code: 'VALIDATION_FAILED', detail: 'reconciliation override' }))).toBe(false);
    expect(isReconciliationOverrideRequired(new Error('reconciliation override'))).toBe(false);
  });
});
