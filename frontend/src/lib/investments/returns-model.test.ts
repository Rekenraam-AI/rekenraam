import { describe, expect, it } from 'vitest';
import {
  fallbackReasonText, formatReturn, lastYear, parseReturnsSelection, presetRange, returnDirection, returnReasonText,
  returnsChartModel, returnsCoverage, returnsSelectionParams, unavailableAccountIDs, validRange, yearToDate
} from './returns-model';
import type { ReturnFigure, ReturnsSeriesPoint } from '#lib/api/investments.ts';
import type { Valuation } from '#lib/reports/reporting-currency.ts';

const today = '2026-10-24';
const params = (query: string) => new URLSearchParams(query);

describe('returns period presets', () => {
  it('year to date starts on 1 January', () => {
    expect(yearToDate(today)).toEqual({ from: '2026-01-01', to: today });
  });

  it('the last 12 months span 365 days ending today, the shortest annualized period', () => {
    expect(lastYear(today)).toEqual({ from: '2025-10-25', to: today });
    expect(lastYear('2027-01-31')).toEqual({ from: '2026-02-01', to: '2027-01-31' });
    expect(presetRange('last_year', today)).toEqual(lastYear(today));
  });

  it('a leap day steps back to the last day of February', () => {
    expect(lastYear('2028-02-29')).toEqual({ from: '2027-03-01', to: '2028-02-29' });
  });
});

describe('parseReturnsSelection', () => {
  it('reads a complete selection, deduplicating and sorting accounts', () => {
    expect(parseReturnsSelection(params('from=2026-02-01&to=2026-06-30&reporting_currency_id=3&account_id=9&account_id=4&account_id=9'), today))
      .toEqual({ from: '2026-02-01', to: '2026-06-30', reportingCurrencyID: 3, accountIDs: [4, 9] });
  });

  it('falls back to year to date for any range the server would refuse', () => {
    for (const query of ['', 'from=2026-02-30&to=2026-03-01', 'from=2026-05-01&to=2026-04-01', 'from=2026-01-01&to=2026-10-25', 'from=x&to=y']) {
      const selection = parseReturnsSelection(params(query), today);
      expect([selection.from, selection.to], query).toEqual(['2026-01-01', today]);
    }
  });

  it('ignores malformed currencies and accounts instead of sending them', () => {
    expect(parseReturnsSelection(params('reporting_currency_id=0&account_id=-1&account_id=a&account_id=2.5'), today))
      .toMatchObject({ reportingCurrencyID: null, accountIDs: [] });
  });

  it('round-trips through the URL parameters it writes', () => {
    const selection = { from: '2026-02-01', to: '2026-06-30', reportingCurrencyID: 3, accountIDs: [4, 9] };
    expect(parseReturnsSelection(returnsSelectionParams(selection), today)).toEqual(selection);
    expect(returnsSelectionParams({ ...selection, reportingCurrencyID: null, accountIDs: [] }).toString())
      .toBe('from=2026-02-01&to=2026-06-30');
  });
});

describe('validRange', () => {
  it('requires real dates in order, ending no later than today', () => {
    expect(validRange('2026-01-01', today, today)).toBe(true);
    expect(validRange(today, today, today)).toBe(true);
    expect(validRange('2026-02-29', '2026-03-01', today)).toBe(false);
    expect(validRange('2026-03-02', '2026-03-01', today)).toBe(false);
    expect(validRange('2026-01-01', '2026-10-25', today)).toBe(false);
  });
});

describe('return presentation', () => {
  it('shows a signed two-decimal percentage', () => {
    expect(formatReturn('0.090909', 'en')).toBe('+9.09%');
    expect(formatReturn('-0.909091', 'en')).toBe('-90.91%');
    expect(formatReturn('0.000000', 'en')).toBe('0.00%');
    expect(formatReturn('0.187954', 'de')).toBe('+18,80\u00a0%');
  });

  it('gives a direction cue that does not rely on colour', () => {
    expect(returnDirection('0.1')).toBe('up');
    expect(returnDirection('-0.1')).toBe('down');
    expect(returnDirection('0.000000')).toBe('flat');
  });
});

describe('reason text', () => {
  const date = (value: string) => `[${value}]`;

  it('names the day a dated reason applies to', () => {
    expect(returnReasonText('valuation_unavailable', '2026-05-01', date)).toContain('[2026-05-01]');
    expect(returnReasonText('non_positive_capital', '2026-07-30', date)).toContain('[2026-07-30]');
  });

  it('explains every documented reason and never shows a raw code', () => {
    const reasons = ['valuation_unavailable', 'unclassified_short', 'flow_unavailable', 'non_positive_capital', 'no_capital',
      'no_root', 'root_not_unique', 'numerical_limit', 'period_shorter_than_year', 'non_positive_growth'];
    const texts = reasons.map((reason) => returnReasonText(reason, null, date));
    expect(new Set(texts).size).toBe(reasons.length);
    for (const [index, text] of texts.entries()) expect(text).not.toContain(reasons[index]);
    expect(returnReasonText('something_new', null, date)).toBe(returnReasonText(undefined, null, date));
  });

  it('explains every attribution fallback', () => {
    const texts = ['unassigned_instrument', 'held_inside_and_outside', 'not_held'].map(fallbackReasonText);
    expect(new Set(texts).size).toBe(3);
  });
});

describe('returnsCoverage', () => {
  const base = {
    selection_policy: 'live_raw_v1', valuation_as_of: '2026-10-24', knowledge_cutoff: '2026-10-24T12:00:00Z',
    commodity_id: 1, scale: 2, method: 'observed_on_or_before', max_staleness_days: 7
  };
  const use = (commodity: number, date: string, stale: boolean) => ({
    observation_id: commodity, approximate: false, commodity_id: commodity, observation_date: date,
    requested_date: '2026-10-24', stale, derived: false
  });

  it('reports a gap and stale quotes together, never one instead of the other', () => {
    const valuation = { ...base, complete: false,
      rates_used: [use(7, '2026-10-22', true), use(8, '2026-10-24', false), use(9, '2026-10-22', true)],
      gaps: [{ commodity_id: 7, reason: 'no_observation_in_window' }] } as Valuation;
    expect(returnsCoverage(valuation)).toEqual({ missing: [7], staleDates: ['2026-10-22'] });
  });

  it('reports the stale FX leg when the instrument quote is current', () => {
    const valuation = { ...base, complete: true, gaps: [], rates_used: [{
      ...use(7, '2026-10-24', true), derived: true,
      via_commodity_id: 2, via_observation_id: 11, via_observation_date: '2026-10-22'
    }] } as Valuation;
    expect(returnsCoverage(valuation)).toEqual({ missing: [], staleDates: ['2026-10-22'] });
  });

  it('is empty without a valuation block or when everything is current', () => {
    expect(returnsCoverage(undefined)).toEqual({ missing: [], staleDates: [] });
    expect(returnsCoverage({ ...base, complete: true, rates_used: [use(8, '2026-10-24', false)], gaps: [] } as Valuation))
      .toEqual({ missing: [], staleDates: [] });
  });
});

describe('unavailableAccountIDs', () => {
  it('lists selected ids that are not selectable accounts, in selection order', () => {
    expect(unavailableAccountIDs([999999, 4, 12], [4, 9])).toEqual([999999, 12]);
    expect(unavailableAccountIDs([4], [4, 9])).toEqual([]);
  });
});

describe('returns chart model', () => {
  const point = (date: string, value: string | null, reason: ReturnFigure['reason'] = null): ReturnsSeriesPoint =>
    ({ date, time_weighted: { value, reason, date: null } });

  it('breaks the line at a missing point instead of interpolating', () => {
    const chart = returnsChartModel([
      point('2026-01-31', '0.000000'),
      point('2026-02-07', '0.050000'),
      point('2026-02-14', null, 'valuation_unavailable'),
      point('2026-02-21', '-0.050000'),
      point('2026-02-22', '0.100000')
    ]);
    expect(chart.segments.map((segment) => segment.map((p) => p.date))).toEqual([
      ['2026-01-31', '2026-02-07'], ['2026-02-21', '2026-02-22']
    ]);
    expect(chart.available).toBe(4);
    expect(chart.total).toBe(5);
    expect(chart.minimum).toBe(-0.05);
    expect(chart.maximum).toBe(0.1);
    expect(chart.zeroY).toBeCloseTo(2 / 3);
    // Calendar time: 7 of 22 days, and the 1-day closing interval is short.
    expect(chart.segments[0][1].x).toBeCloseTo(7 / 22);
    expect(chart.segments[1][1].x).toBe(1);
    expect(chart.segments[1][1].y).toBe(0);
  });

  it('keeps 0 % in range and centres a flat line', () => {
    const rising = returnsChartModel([point('2026-01-31', '0.000000'), point('2026-02-01', '0.020000')]);
    expect(rising.minimum).toBe(0);
    expect(rising.zeroY).toBe(1);
    const flat = returnsChartModel([point('2026-01-31', '0.000000'), point('2026-02-01', '0.000000')]);
    expect(flat.zeroY).toBe(0.5);
  });

  it('has no segments when no point has a value', () => {
    const chart = returnsChartModel([point('2026-01-31', null, 'no_capital'), point('2026-02-01', null, 'no_capital')]);
    expect(chart.segments).toEqual([]);
    expect(chart.available).toBe(0);
  });
});
