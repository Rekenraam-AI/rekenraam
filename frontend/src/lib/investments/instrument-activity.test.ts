import { describe, expect, it } from 'vitest';
import { instrumentAccountIDs, instrumentUnits } from './instrument-activity';

const posting = (account_id: number, commodity_id: number, quantity_value: string, overrides = {}) => ({
  account_id, commodity_id, quantity_value, quantity_scale: 4, account_class: 'asset', ...overrides
});

describe('instrumentUnits (#186)', () => {
  it('sums the instrument across holding postings only', () => {
    const buy = { journal_entries: [{ postings: [
      posting(1, 10, '50000'),
      posting(2, 10, '-50000', { account_system_role: 'investment_clearing' }),
      posting(3, 1, '-100000')
    ] }] };
    expect(instrumentUnits(buy, 10)).toEqual({ value: '50000', scale: 4 });
    expect(instrumentAccountIDs(buy, 10)).toEqual([1]);
  });

  it('nets a transfer between holdings and lists both accounts', () => {
    const transfer = { journal_entries: [{ postings: [posting(1, 10, '-20000')] }, { postings: [posting(4, 10, '20000')] }] };
    expect(instrumentUnits(transfer, 10)).toEqual({ value: '0', scale: 4 });
    expect(instrumentAccountIDs(transfer, 10)).toEqual([1, 4]);
  });

  it('is undefined when no holding carries the instrument', () => {
    expect(instrumentUnits({ journal_entries: [{ postings: [posting(3, 1, '500')] }] }, 10)).toBeUndefined();
    expect(instrumentUnits({}, 10)).toBeUndefined();
  });
});
