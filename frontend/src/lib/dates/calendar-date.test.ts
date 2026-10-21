import { describe, expect, it } from 'vitest';
import { adjacentCalendarDate, isCalendarDate } from './calendar-date';

describe('isCalendarDate', () => {
  it.each(['2026-02-31', '2026-02-29', '2026-04-31', '2026-13-01', '2026-00-01'])(
    'rejects impossible calendar dates: %s',
    (date) => expect(isCalendarDate(date)).toBe(false)
  );

  it.each(['2026-02-28', '2024-02-29'])(
    'accepts ordinary and leap calendar dates: %s',
    (date) => expect(isCalendarDate(date)).toBe(true)
  );
});

describe('adjacentCalendarDate', () => {
  it('steps one calendar day across month, year and leap edges', () => {
    expect(adjacentCalendarDate('2026-06-01', 1)).toBe('2026-06-02');
    expect(adjacentCalendarDate('2026-03-01', -1)).toBe('2026-02-28');
    expect(adjacentCalendarDate('2028-03-01', -1)).toBe('2028-02-29');
    expect(adjacentCalendarDate('2026-12-31', 1)).toBe('2027-01-01');
    expect(adjacentCalendarDate('2027-01-01', -1)).toBe('2026-12-31');
    expect(adjacentCalendarDate('2026-04-30', 1)).toBe('2026-05-01');
  });
});
