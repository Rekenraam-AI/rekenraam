import type { ReadonlyURLSearchParams } from '$app/state';
import { adjacentCalendarDate, isCalendarDate } from '#lib/dates/calendar-date.ts';
import { m } from '#lib/paraglide/messages.js';

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
