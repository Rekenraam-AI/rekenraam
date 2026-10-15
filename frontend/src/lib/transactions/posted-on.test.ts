import { describe, expect, it } from 'vitest';
import { editablePostings, hasPostingDay, otherPostingDays, postedOnForRequest } from './posted-on';

const cash = { id: 1, account_class: 'asset', account_system_role: null };
const savings = { id: 2, account_class: 'asset', account_system_role: null };
const groceries = { id: 3, account_class: 'expense', account_system_role: null };
const clearing = { id: 9, account_class: 'asset', account_system_role: 'transfer_clearing' };
const trading = { id: 8, account_class: 'equity', account_system_role: 'commodity_trading' };

describe('hasPostingDay', () => {
  it('is true only for a user asset or liability', () => {
    expect(hasPostingDay(cash)).toBe(true);
    expect(hasPostingDay({ account_class: 'liability' })).toBe(true);
    expect(hasPostingDay(groceries)).toBe(false);
    expect(hasPostingDay(clearing)).toBe(false);
    expect(hasPostingDay({ account_class: 'asset', system_role: 'transfer_clearing' })).toBe(false);
    expect(hasPostingDay(undefined)).toBe(false);
  });
});

describe('editablePostings', () => {
  it('drops the clearing bridge of a two-day transfer and keeps each leg on its day', () => {
    const rows = editablePostings(
      [
        { entry_date: '2026-06-01', postings: [cash, clearing] },
        { entry_date: '2026-06-03', postings: [savings, clearing] }
      ],
      '2026-06-01'
    );
    expect(rows.map((r) => [r.posting.id, r.postedOn])).toEqual([
      [1, ''],
      [2, '2026-06-03']
    ]);
  });

  it('keeps a delayed card posting on its posting day and the category following it', () => {
    const rows = editablePostings([{ entry_date: '2026-03-03', postings: [cash, groceries] }], '2026-03-01');
    expect(rows.map((r) => [r.posting.id, r.postedOn])).toEqual([
      [1, '2026-03-03'],
      [3, '']
    ]);
  });

  it('keeps a clearing posting in a single-entry journal, which is not a bridge', () => {
    const rows = editablePostings([{ entry_date: '2026-06-01', postings: [cash, clearing, trading] }], '2026-06-01');
    expect(rows.map((r) => r.posting.id)).toEqual([1, 9, 8]);
  });
});

describe('postedOnForRequest', () => {
  it('sends only a differing day on an account posting', () => {
    expect(postedOnForRequest('2026-06-03', '2026-06-01', cash)).toBe('2026-06-03');
    expect(postedOnForRequest('2026-06-01', '2026-06-01', cash)).toBeUndefined();
    expect(postedOnForRequest('', '2026-06-01', cash)).toBeUndefined();
    expect(postedOnForRequest('2026-06-03', '2026-06-01', groceries)).toBeUndefined();
  });
});

describe('otherPostingDays', () => {
  it('lists each differing posting day once, in order', () => {
    expect(
      otherPostingDays({
        transaction_date: '2026-06-01',
        journal_entries: [{ entry_date: '2026-06-04' }, { entry_date: '2026-06-01' }, { entry_date: '2026-06-02' }, { entry_date: '2026-06-04' }]
      })
    ).toEqual(['2026-06-02', '2026-06-04']);
    expect(otherPostingDays({ transaction_date: '2026-06-01', journal_entries: [{ entry_date: '2026-06-01' }] })).toEqual([]);
  });
});
