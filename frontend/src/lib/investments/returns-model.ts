import type { ReadonlyURLSearchParams } from '$app/state';
import type { ReturnsSeriesPoint } from '#lib/api/investments.ts';
import { adjacentCalendarDate, isCalendarDate } from '#lib/dates/calendar-date.ts';
import { m } from '#lib/paraglide/messages.js';
import { staleObservationDates, type Valuation } from '#lib/reports/reporting-currency.ts';

// R13 returns screen (docs/plans/investment-returns-plan.md, slice 2). The
// selection lives in the URL like every report control, so a link reopens
// the same period, portfolio and currency.

export type ReturnsSelection = {
  from: string;
  to: string;
  /** Null until a currency is chosen; the screen then proposes one. */
  reportingCurrencyID: number | null;
  /** Empty selects the server's default investment accounts. */
  accountIDs: number[];
};

export type ReturnsPreset = 'ytd' | 'last_year';

/** Year to date: the gains statement's default period. */
export function yearToDate(today: string): { from: string; to: string } {
  return { from: `${today.slice(0, 4)}-01-01`, to: today };
}

/** The 365 or 366 days ending today: the shortest period with annual figures. */
export function lastYear(today: string): { from: string; to: string } {
  const year = String(Number(today.slice(0, 4)) - 1).padStart(4, '0');
  let anniversary = `${year}${today.slice(4)}`;
  // 29 February has no anniversary in a common year.
  if (!isCalendarDate(anniversary)) anniversary = `${year}-02-28`;
  return { from: adjacentCalendarDate(anniversary, 1), to: today };
}

export function presetRange(preset: ReturnsPreset, today: string): { from: string; to: string } {
  return preset === 'ytd' ? yearToDate(today) : lastYear(today);
}

/**
 * Reads the selection from a URL. Anything malformed or out of range falls back
 * to year to date rather than producing a request the server will refuse: a
 * stale or hand-edited link still opens a working screen.
 */
export function parseReturnsSelection(params: ReadonlyURLSearchParams, today: string): ReturnsSelection {
  const fallback = yearToDate(today);
  const from = params.get('from') ?? '';
  const to = params.get('to') ?? '';
  const range = validRange(from, to, today) ? { from, to } : fallback;

  const rawCurrency = params.get('reporting_currency_id');
  const currency = rawCurrency !== null && /^\d+$/.test(rawCurrency) ? Number(rawCurrency) : NaN;

  const accountIDs = [...new Set(params.getAll('account_id')
    .filter((raw) => /^\d+$/.test(raw))
    .map(Number)
    .filter((id) => Number.isSafeInteger(id) && id > 0))].sort((a, b) => a - b);

  return {
    ...range,
    reportingCurrencyID: Number.isSafeInteger(currency) && currency > 0 ? currency : null,
    accountIDs
  };
}

export function returnsSelectionParams(selection: ReturnsSelection): URLSearchParams {
  const params = new URLSearchParams({ from: selection.from, to: selection.to });
  if (selection.reportingCurrencyID !== null) params.set('reporting_currency_id', String(selection.reportingCurrencyID));
  for (const id of selection.accountIDs) params.append('account_id', String(id));
  return params;
}

/** A period the server accepts: real calendar dates, from ≤ to ≤ today. */
export function validRange(from: string, to: string, today: string): boolean {
  return isCalendarDate(from) && isCalendarDate(to) && from <= to && to <= today;
}

/** Why a figure is missing, in words. Unknown codes fall back to a generic
 * explanation instead of showing the code. */
export function returnReasonText(reason: string | null | undefined, date: string | null | undefined, formatDate: (date: string) => string): string {
  const day = date ? formatDate(date) : '';
  switch (reason) {
    case 'valuation_unavailable':
      return day ? m.returns_reason_valuation_dated({ date: day }) : m.returns_reason_valuation();
    case 'unclassified_short':
      return day ? m.returns_reason_unclassified_short_dated({ date: day }) : m.returns_reason_unclassified_short();
    case 'flow_unavailable':
      return m.returns_reason_flow();
    case 'non_positive_capital':
      return day ? m.returns_reason_non_positive_capital_dated({ date: day }) : m.returns_reason_non_positive_capital();
    case 'no_capital':
      return m.returns_reason_no_capital();
    case 'no_root':
      return m.returns_reason_no_root();
    case 'root_not_unique':
      return m.returns_reason_root_not_unique();
    case 'numerical_limit':
      return m.returns_reason_numerical_limit();
    case 'period_shorter_than_year':
      return m.returns_reason_period_shorter_than_year();
    case 'non_positive_growth':
      return m.returns_reason_non_positive_growth();
    default:
      return m.returns_reason_unknown();
  }
}

export function fallbackReasonText(reason: string): string {
  switch (reason) {
    case 'unassigned_instrument':
      return m.returns_fallback_unassigned_instrument();
    case 'held_inside_and_outside':
      return m.returns_fallback_held_inside_and_outside();
    case 'not_held':
      return m.returns_fallback_not_held();
    default:
      return m.returns_reason_unknown();
  }
}

/**
 * A return fraction ("0.090909") as a locale percentage with two decimals and
 * an explicit sign. Returns are ratios, not money, so a Number is exact
 * enough for presentation; the server already rounded to six places.
 */
export function formatReturn(value: string, locale: string): string {
  return new Intl.NumberFormat(locale, {
    style: 'percent',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
    signDisplay: 'exceptZero'
  }).format(Number(value));
}

/** Direction of a return, for a non-colour cue beside the figure. */
export function returnDirection(value: string): 'up' | 'down' | 'flat' {
  const number = Number(value);
  if (number > 0) return 'up';
  if (number < 0) return 'down';
  return 'flat';
}

export type ReturnsCoverage = {
  /** Commodities with no usable price or rate: figures needing them are absent. */
  missing: number[];
  /** Earlier-dated observations behind figures that are shown. */
  staleDates: string[];
};

/**
 * What the valuation block discloses. Missing and stale coverage are
 * independent: a gap can remove the time-weighted return while the closing
 * value and money-weighted return still rest on an earlier-dated quote, and
 * the reader needs to know both.
 */
export function returnsCoverage(valuation: Valuation | null | undefined): ReturnsCoverage {
  if (!valuation) return { missing: [], staleDates: [] };
  return {
    missing: [...new Set(valuation.gaps.map((gap) => gap.commodity_id))].sort((a, b) => a - b),
    staleDates: staleObservationDates(valuation.rates_used)
  };
}

/** Selected ids that are not selectable portfolio accounts in this book: a
 * stale or hand-edited link. They are shown, removable, and never sent. */
export function unavailableAccountIDs(selected: number[], selectable: number[]): number[] {
  const known = new Set(selectable);
  return selected.filter((id) => !known.has(id));
}

export type ReturnsChartPoint = { date: string; value: number; x: number; y: number };

export type ReturnsChart = {
  /** Runs of consecutive points with a value. A missing point breaks the
   * line; nothing is interpolated across it. */
  segments: ReturnsChartPoint[][];
  /** The plotted range, always including 0 so the baseline is visible. */
  minimum: number;
  maximum: number;
  /** Vertical position of 0 %, as a fraction of the plot height from the top. */
  zeroY: number;
  available: number;
  total: number;
};

function dayNumber(date: string): number {
  return Date.UTC(Number(date.slice(0, 4)), Number(date.slice(5, 7)) - 1, Number(date.slice(8, 10))) / 86_400_000;
}

/**
 * Places the cumulative time-weighted series on a unit square. x is calendar
 * time from the opening day, so the shorter last interval is drawn shorter;
 * y runs top (maximum) to bottom (minimum). Returns are ratios, not money,
 * so Numbers are fine for drawing.
 */
export function returnsChartModel(points: ReturnsSeriesPoint[]): ReturnsChart {
  const values = points.flatMap((point) => point.time_weighted.value === null ? [] : [Number(point.time_weighted.value)]);
  const minimum = Math.min(0, ...values);
  const maximum = Math.max(0, ...values);
  const span = maximum - minimum;
  const y = (value: number) => span === 0 ? 0.5 : (maximum - value) / span;
  const first = points.length > 0 ? dayNumber(points[0].date) : 0;
  const width = points.length > 1 ? dayNumber(points[points.length - 1].date) - first : 0;
  const segments: ReturnsChartPoint[][] = [];
  let run: ReturnsChartPoint[] = [];
  for (const point of points) {
    if (point.time_weighted.value === null) {
      if (run.length > 0) segments.push(run);
      run = [];
      continue;
    }
    const value = Number(point.time_weighted.value);
    run.push({ date: point.date, value, x: width === 0 ? 0 : (dayNumber(point.date) - first) / width, y: y(value) });
  }
  if (run.length > 0) segments.push(run);
  return { segments, minimum, maximum, zeroY: y(0), available: values.length, total: points.length };
}

export type AllocationDimension = 'account' | 'asset_type' | 'currency';

/** Width of a weight's bar, as a percentage of the track: the magnitude,
 * capped at the full track (a long leg of a shorted portfolio can exceed
 * 100 %; the figure beside the bar says by how much). */
export function allocationBarPercent(weight: string | null): number {
  if (weight === null) return 0;
  return Math.min(100, Math.abs(Number(weight)) * 100);
}

/** Asset types as stable codes; instrument types reuse the instrument labels. */
export function assetTypeText(assetType: string | null): string {
  switch (assetType) {
    case 'cash': return m.returns_allocation_cash();
    case 'stock': return m.quotes_type_stock();
    case 'etf': return m.quotes_type_etf();
    case 'fund': return m.quotes_type_fund();
    case 'bond': return m.quotes_type_bond();
    case 'option': return m.quotes_type_option();
    case 'future': return m.quotes_type_future();
    case 'crypto': return m.quotes_type_crypto();
    case 'other': return m.quotes_type_other();
    default: return m.returns_allocation_unclassified();
  }
}

/** A share of the whole ("0.687500") as a percentage: no "+" on a long
 * share, a minus on a short one. */
export function formatWeight(value: string, locale: string): string {
  return new Intl.NumberFormat(locale, { style: 'percent', minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(Number(value));
}
