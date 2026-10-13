import { describe, expect, it } from 'vitest';
import { sessionOwnerToday } from './owner-date.ts';
import { defaultReportRange, repairReportRange } from '#lib/reports/report-range.ts';

describe('authenticated owner date contract', () => {
  it('uses the server date verbatim across midnight and year boundaries', () => {
    for (const today of ['2026-12-31', '2027-01-01', '2020-02-29']) {
      expect(sessionOwnerToday({ authenticated: true, owner_today: today })).toBe(today);
      expect(defaultReportRange(today)).toEqual({ startDate: `${today.slice(0, 4)}-01-01`, endDate: today, bucket: 'month' });
    }
  });
  it('refuses missing, unauthenticated and malformed dates without a browser fallback', () => {
    expect(() => sessionOwnerToday({ authenticated: false, owner_today: '2026-01-01' })).toThrow();
    for (const today of [undefined, '', '2026-02-29', '2026-99-01', '2026-01-00', '2026-01-01T00:00:00Z']) {
      expect(() => sessionOwnerToday({ authenticated: true, owner_today: today })).toThrow();
    }
  });
  it('repairs missing report ranges with the owner year while preserving explicit ranges', () => {
    const params = new URLSearchParams('view=spending&account_id=12');
    const repaired = repairReportRange(params, '2027-01-01');
    expect(repaired?.get('start_date')).toBe('2027-01-01');
    expect(repaired?.get('end_date')).toBe('2027-01-01');
    expect(repaired?.get('account_id')).toBe('12');
    const explicit = new URLSearchParams('start_date=2020-01-01&end_date=2020-02-29&bucket=day');
    expect(repairReportRange(explicit, '2027-01-01')).toBeNull();
  });
});
