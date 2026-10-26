import { describe, expect, it } from 'vitest';
import type { AccountResponse } from '#lib/api/accounts.ts';
import { isSettlementAccount } from './settlement-accounts.ts';

function account(overrides: Partial<AccountResponse>): AccountResponse {
  return {
    id: 1, account_class: 'asset', account_kind: 'brokerage_cash', status: 'active', allows_postings: true,
    system_role: null, ...overrides
  } as AccountResponse;
}

describe('isSettlementAccount', () => {
  it('offers assets and liabilities that accept postings', () => {
    expect(isSettlementAccount(account({}))).toBe(true);
    expect(isSettlementAccount(account({ account_class: 'liability', account_kind: 'credit_card' }))).toBe(true);
  });

  it('refuses what the backend settlement role refuses', () => {
    expect(isSettlementAccount(account({ account_class: 'income', account_kind: 'income' }))).toBe(false);
    expect(isSettlementAccount(account({ account_kind: 'security_holding' }))).toBe(false);
    expect(isSettlementAccount(account({ system_role: 'commodity_trading' } as Partial<AccountResponse>))).toBe(false);
    expect(isSettlementAccount(account({ allows_postings: false }))).toBe(false);
    expect(isSettlementAccount(account({ status: 'archived' }))).toBe(false);
  });

  it('retains a correction\'s recorded account even when archived', () => {
    expect(isSettlementAccount(account({ id: 7, status: 'archived' }), 7)).toBe(true);
    expect(isSettlementAccount(account({ id: 8, status: 'archived' }), 7)).toBe(false);
  });
});
