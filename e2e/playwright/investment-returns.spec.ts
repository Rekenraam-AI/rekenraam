import { readFile } from 'node:fs/promises';
import { expect, test } from './support/owned-book';
import type { Page } from './support/fixtures';
import { apiJSON } from './support/api';
import { ensureCurrency, readyForLedger } from './support/ledger';
import { daysFromTodayISO, todayISO } from './support/dates';
import { expectNoAccessibilityViolations } from './support/a11y';

// R13 slices 2–9 (#119): the returns screen, its chart, allocation, saved runs
// and per-instrument panel over the returns_v1 read. The
// default scope is every investment account in the book, so each test owns
// its book and the figures are exact.
test.use({ viewport: { width: 390, height: 844 } });

function noHorizontalScroll(page: Page) {
  return page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
}

test('[acceptance] returns show an empty book, then exact figures and reasons on a phone', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const from = daysFromTodayISO(-45);
  const url = `/app/investments/returns?from=${from}&to=${todayISO()}&reporting_currency_id=${currencyID}`;

  await page.goto(url);
  await expect(page.getByText('No investment accounts', { exact: true })).toBeVisible();

  // Ten units bought for 1,000.00 from checking, outside the portfolio: a
  // contribution. Priced at 110.00 today: worth 1,100.00.
  const opened = daysFromTodayISO(-60);
  const checking = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Returns checking', account_class: 'asset', account_kind: 'checking',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: 'RETX', instrument_type: 'stock', display_name: 'Returns example', symbol: 'RETX',
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: opened
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: 'Returns holding', opened_on: opened, effective_from: opened
  });
  await apiJSON(page, 'POST', '/api/v1/investments/buy', csrfToken, {
    transaction_date: daysFromTodayISO(-40), commodity_id: instrument.commodity_id, holding_account_id: holding.id,
    cash_account_id: checking.id, quantity_value: '10', quantity_scale: 0, cash_amount_value: '100000', cash_amount_scale: 2,
    cash_commodity_id: currencyID, cost_basis_method: 'fifo'
  });
  await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
    base_commodity_id: instrument.commodity_id, quote_commodity_id: currencyID, quote_type: 'close',
    adjustment_basis: 'raw', price_value: 11000, price_scale: 2, valuation_date: todayISO(), is_manual: true
  });

  await page.goto(url);
  const summary = page.getByTestId('returns-summary');
  // 0/0 before the purchase is no capital, then 1,100 / 1,000.
  await expect(summary.getByTestId('returns-twr-period')).toHaveText('+10.00%');
  await expect(summary.getByTestId('returns-contributions')).toContainText('1,000.00');
  await expect(summary.getByTestId('returns-closing')).toContainText('1,100.00');
  await expect(summary.getByTestId('returns-gain')).toContainText('100.00');
  await expect(summary.getByTestId('returns-twr-annual')).toHaveText('—');
  await expect(summary.getByText('Shown only for periods of at least a year.').first()).toBeVisible();
  await expect(summary.getByText('Every investment account, with its sub-accounts.', { exact: true })).toBeVisible();
  await expect(summary.getByRole('paragraph').filter({ hasText: /^Returns holding$/ })).toBeVisible();

  // Slice 3: 46 days is a daily series of 47 points. 0.00 % before and after
  // the purchase while its fill price is within 7 days; the 32 days from -32
  // to -1 have no price, so the line breaks there; today's point is the
  // headline figure.
  const chart = summary.getByTestId('returns-chart');
  await expect(chart.getByRole('img')).toBeVisible();
  await expect(chart.getByTestId('returns-chart-gaps')).toHaveText('32 of 47 points have no value.');
  // Days -39 to -33 reuse the purchase's fill price: disclosed with the
  // chart, though no headline figure needs it.
  await expect(chart.getByTestId('returns-chart-stale')).toContainText('Some points use prices or rates from earlier days:');
  await chart.getByText('Table of points', { exact: true }).click();
  const points = chart.getByTestId('returns-chart-table').locator('tbody tr');
  await expect(points).toHaveCount(47);
  await expect(points.nth(13)).toContainText('0.00%');
  await expect(points.nth(14)).toContainText('Needs a price or exchange rate for');
  await expect(points.last()).toContainText('+10.00%');

  // Slice 4: the holding is the whole closing value.
  const allocation = summary.getByTestId('returns-allocation');
  const shares = allocation.getByTestId('returns-allocation-rows').locator('li');
  await expect(shares).toHaveCount(1);
  await expect(shares.first()).toContainText('Returns holding');
  await expect(shares.first()).toContainText('1,100.00');
  await expect(shares.first()).toContainText('100.00%');
  await allocation.getByRole('button', { name: 'Asset type', exact: true }).click();
  await expect(allocation.getByRole('button', { name: 'Asset type', exact: true })).toHaveAttribute('aria-pressed', 'true');
  await expect(shares.first()).toContainText('Stock');

  expect(await noHorizontalScroll(page)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Returns on a phone');

  // Ticking only checking leaves the holding out: the purchase is now money
  // leaving the portfolio, at the price paid.
  await page.getByRole('checkbox', { name: 'Returns checking', exact: true }).check();
  await page.getByRole('button', { name: 'Show returns', exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`account_id=${checking.id}`));
  await expect(summary.getByText('The accounts you ticked, with their sub-accounts.', { exact: true })).toBeVisible();
  await expect(summary.getByTestId('returns-withdrawals')).toContainText('1,000.00');
  await expect(summary.getByTestId('returns-closing')).toContainText('-1,000.00');
  await expect(summary.getByTestId('returns-twr-period')).toHaveText('—');
  // Checking alone is worth −1,000.00: a short row, and no shares of a
  // negative whole.
  await expect(allocation.getByText('Short', { exact: true })).toBeVisible();
  await expect(allocation.getByText("The portfolio's value is zero or negative, so shares cannot be shown.")).toBeVisible();
});

// #428: the screen must not gate a known reference failure behind a sibling
// reference query that is still pending, and Retry must recover once both can
// succeed. The route sweep fails every request at once, so it never exercises
// this order: here `currencies` is held open while `accounts` fails twice
// (the query client retries once).
test('a failed reference request outranks a pending sibling, and Retry recovers', async ({ page }) => {
  const { currencyID } = await readyForLedger(page);

  let failAccounts = true;
  let releaseCurrencies = () => {};
  const currenciesReleased = new Promise<void>((resolve) => {
    releaseCurrencies = resolve;
  });

  await page.route('**/api/v1/currencies**', async (route) => {
    const request = route.request();
    if (request.method() !== 'GET' || new URL(request.url()).pathname !== '/api/v1/currencies') return route.continue();
    await currenciesReleased;
    return route.continue();
  });
  await page.route('**/api/v1/accounts**', async (route) => {
    const request = route.request();
    if (request.method() !== 'GET' || !failAccounts) return route.continue();
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: 'reference probe failure' } })
    });
  });

  await page.goto(`/app/investments/returns?reporting_currency_id=${currencyID}`);

  // `currencies` is still pending when `accounts` has failed for good. The
  // error must show now; the pre-#428 branch order kept the spinner up.
  await expect(page.getByRole('alert').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText('Returns could not be loaded', { exact: true })).toBeVisible();
  await expect(page.getByText('Loading returns…', { exact: true })).toHaveCount(0);
  await expectNoAccessibilityViolations(page, 'returns reference error');

  // Retry re-runs all three reference queries; now they can succeed.
  failAccounts = false;
  releaseCurrencies();
  await page.getByRole('button', { name: 'Retry', exact: true }).click();

  await expect(page.getByLabel('From', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Show returns', exact: true })).toBeEnabled();
  await expect(page.getByText('Returns could not be loaded', { exact: true })).toHaveCount(0);
});

test('returns keep the form usable when a period is invalid', async ({ page }) => {
  const { currencyID } = await readyForLedger(page);
  await page.goto(`/app/investments/returns?reporting_currency_id=${currencyID}`);
  const from = page.getByLabel('From', { exact: true });
  await expect(from).toHaveValue(`${todayISO().slice(0, 4)}-01-01`);
  await from.fill(daysFromTodayISO(1));
  await expect(page.getByText('Choose a start on or before the end, and an end no later than today.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Show returns', exact: true })).toBeDisabled();
});

// Review of f0b3b8ec (adapted from its browser probes).

test('archived accounts stay selectable, labelled, for historical returns', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-400);
  const account = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Retired returns broker', account_class: 'asset', account_kind: 'brokerage_cash',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  await apiJSON(page, 'POST', `/api/v1/accounts/${account.id}/close`, csrfToken, { closed_on: todayISO() });
  await apiJSON(page, 'POST', `/api/v1/accounts/${account.id}/archive`, csrfToken, { effective_from: todayISO() });

  await page.goto(`/app/investments/returns?from=${daysFromTodayISO(-390)}&to=${daysFromTodayISO(-10)}&reporting_currency_id=${currencyID}&account_id=${account.id}`);
  const summary = page.getByTestId('returns-summary');
  await expect(summary).toBeVisible();
  await expect(page.getByRole('checkbox', { name: 'Retired returns broker (archived)', exact: true })).toBeChecked();
  await expect(summary.getByText('Retired returns broker (archived)', { exact: true })).toBeVisible();
});

test('an unavailable account in a link is shown, left out, and removable', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-60);
  const account = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Valid returns checking', account_class: 'asset', account_kind: 'checking',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  await page.goto(`/app/investments/returns?from=${daysFromTodayISO(-45)}&to=${todayISO()}&reporting_currency_id=${currencyID}&account_id=999999`);
  await expect(page.getByText('Returns could not be loaded', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Remove #999999', exact: true })).toBeVisible();

  // Ticking a valid account and applying drops the unavailable one.
  await page.getByRole('checkbox', { name: 'Valid returns checking', exact: true }).check();
  await page.getByRole('button', { name: 'Show returns', exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`account_id=${account.id}`));
  await expect(page).not.toHaveURL(/account_id=999999/);
  await expect(page.getByTestId('returns-summary')).toBeVisible();

  // The reset returns to every investment account.
  await page.getByRole('button', { name: 'Use every investment account', exact: true }).click();
  await page.getByRole('button', { name: 'Show returns', exact: true }).click();
  await expect(page).not.toHaveURL(/account_id=/);
});

test('a gap and stale quotes are disclosed together', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-60);
  const cash = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Gap checking', account_class: 'asset', account_kind: 'checking', default_commodity_id: currencyID,
    allows_postings: true, opened_on: opened, effective_from: opened
  });
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: 'RGAP', instrument_type: 'stock', display_name: 'Returns gap', symbol: 'RGAP',
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: opened
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: 'Gap holding', opened_on: opened, effective_from: opened
  });
  // The second purchase needs V the day before it, 21 days after the first
  // fill price: a gap for TTWROR only. Closing uses a two-day-old quote.
  for (const [offset, units, cents] of [[-40, '10', 100000], [-20, '1', 10000]] as const) {
    await apiJSON(page, 'POST', '/api/v1/investments/buy', csrfToken, {
      transaction_date: daysFromTodayISO(offset), commodity_id: instrument.commodity_id,
      holding_account_id: holding.id, cash_account_id: cash.id, quantity_value: units, quantity_scale: 0,
      cash_amount_value: cents, cash_amount_scale: 2, cash_commodity_id: currencyID, cost_basis_method: 'fifo'
    });
  }
  await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
    base_commodity_id: instrument.commodity_id, quote_commodity_id: currencyID, quote_type: 'close', adjustment_basis: 'raw',
    price_value: 11000, price_scale: 2, valuation_date: daysFromTodayISO(-2), is_manual: true
  });

  await page.goto(`/app/investments/returns?from=${daysFromTodayISO(-45)}&to=${todayISO()}&reporting_currency_id=${currencyID}`);
  const summary = page.getByTestId('returns-summary');
  await expect(summary.getByTestId('returns-closing')).toContainText('1,210.00');
  await expect(summary.getByTestId('returns-twr-period')).toHaveText('—');
  await expect(summary.getByText(/No usable price or exchange rate for: RGAP/)).toBeVisible();
  await expect(summary.getByText(/Some prices or rates come from earlier days/)).toBeVisible();
});

// Review of 62f9201d, finding 2: a portfolio whose only holding has no
// usable closing price has an unknown value, not a zero one.
test('allocation of an unpriced portfolio says its value is unknown', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-60);
  const checking = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Unpriced checking', account_class: 'asset', account_kind: 'checking', default_commodity_id: currencyID,
    allows_postings: true, opened_on: opened, effective_from: opened
  });
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: 'RUNP', instrument_type: 'stock', display_name: 'Returns unpriced', symbol: 'RUNP',
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: opened
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: 'Unpriced holding', opened_on: opened, effective_from: opened
  });
  // The fill price is 40 days old today: outside the 7-day window.
  await apiJSON(page, 'POST', '/api/v1/investments/buy', csrfToken, {
    transaction_date: daysFromTodayISO(-40), commodity_id: instrument.commodity_id, holding_account_id: holding.id,
    cash_account_id: checking.id, quantity_value: '10', quantity_scale: 0, cash_amount_value: '100000', cash_amount_scale: 2,
    cash_commodity_id: currencyID, cost_basis_method: 'fifo'
  });

  await page.goto(`/app/investments/returns?from=${daysFromTodayISO(-45)}&to=${todayISO()}&reporting_currency_id=${currencyID}`);
  const allocation = page.getByTestId('returns-allocation');
  await expect(allocation.getByTestId('returns-allocation-non-positive')).toHaveText(
    "The priced holdings are worth zero or less, so shares cannot be shown. The portfolio's total value is unknown.");
  await expect(allocation.getByText("The portfolio's value is zero or negative, so shares cannot be shown.")).toHaveCount(0);
  await expect(allocation.getByTestId('returns-allocation-unknown')).toContainText('Unpriced holding');
});

// Slice 5: a benchmark read over the same points, price return unless the
// owner declares the series total-return.
test('a benchmark compares over the same period, on its declared basis', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-60);
  await apiJSON(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Benchmark broker', account_class: 'asset', account_kind: 'brokerage_cash',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  const index = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: 'BIDX', instrument_type: 'other', display_name: 'Benchmark index', symbol: 'BIDX',
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: opened
  });
  // Raw 200 → 210 (+5 %); total return 100 → 115 (+15 %), at the opening
  // boundary (the day before the period) and today.
  for (const [basis, date, cents] of [
    ['raw', daysFromTodayISO(-46), 20000], ['raw', todayISO(), 21000],
    ['total_return', daysFromTodayISO(-46), 10000], ['total_return', todayISO(), 11500]
  ] as const) {
    await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
      base_commodity_id: index.commodity_id, quote_commodity_id: currencyID, quote_type: 'close',
      adjustment_basis: basis, price_value: cents, price_scale: 2, valuation_date: date, is_manual: true
    });
  }

  await page.goto(`/app/investments/returns?from=${daysFromTodayISO(-45)}&to=${todayISO()}&reporting_currency_id=${currencyID}`);
  const summary = page.getByTestId('returns-summary');
  await expect(summary).toBeVisible();
  await expect(summary.getByTestId('returns-benchmark')).toHaveCount(0);

  await page.getByRole('combobox', { name: 'Instrument', exact: true }).selectOption({ label: 'BIDX · Benchmark index' });
  await page.getByRole('button', { name: 'Show returns', exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`benchmark_commodity_id=${index.commodity_id}`));
  const benchmark = summary.getByTestId('returns-benchmark');
  await expect(benchmark.getByRole('heading', { name: 'Benchmark: BIDX' })).toBeVisible();
  await expect(benchmark.getByText(/^Price return \(distributions excluded\)/)).toBeVisible();
  await expect(benchmark.getByTestId('returns-benchmark-period')).toHaveText('+5.00%');
  await expect(benchmark.getByTestId('returns-benchmark-annual')).toHaveText('—');
  // Days −38 to −1 have no price within 7 days: those benchmark points are
  // missing and the line breaks there.
  const chart = summary.getByTestId('returns-chart');
  await expect(chart.getByTestId('returns-chart-legend')).toContainText('Benchmark (BIDX), dashed');
  await expect(chart.getByTestId('returns-chart-benchmark-gaps')).toHaveText(
    'The benchmark has no value at 38 of 47 points; its line breaks there.');
  await chart.getByText('Table of points', { exact: true }).click();
  await expect(chart.getByRole('columnheader', { name: 'Benchmark (BIDX)' })).toBeVisible();
  await expect(chart.getByTestId('returns-chart-table').locator('tbody tr').last()).toContainText('+5.00%');

  expect(await noHorizontalScroll(page)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Returns with a benchmark on a phone');

  await page.getByRole('combobox', { name: 'Series', exact: true }).selectOption('total_return');
  await page.getByRole('button', { name: 'Show returns', exact: true }).click();
  await expect(page).toHaveURL(/benchmark_basis=total_return/);
  await expect(benchmark.getByText(/^Total return \(only prices recorded as total return\)/)).toBeVisible();
  await expect(benchmark.getByTestId('returns-benchmark-period')).toHaveText('+15.00%');

  // Removing the benchmark drops it from the link and the screen.
  await page.getByRole('combobox', { name: 'Instrument', exact: true }).selectOption('');
  await page.getByRole('button', { name: 'Show returns', exact: true }).click();
  await expect(page).not.toHaveURL(/benchmark_/);
  await expect(summary.getByTestId('returns-benchmark')).toHaveCount(0);
});

// Review of eeab1a2a, finding 2: a benchmark priced in another currency whose
// exchange rate is missing says so, rather than blaming the price.
test('a benchmark names a missing exchange rate apart from a missing price', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const eur = await ensureCurrency(page, csrfToken, 'EUR', 'Euro');
  expect(eur).not.toBe(currencyID);
  const opened = daysFromTodayISO(-60);
  await apiJSON(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'FX benchmark broker', account_class: 'asset', account_kind: 'brokerage_cash',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  const index = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: 'FXIDX', instrument_type: 'other', display_name: 'FX benchmark', symbol: 'FXIDX',
    quote_commodity_id: eur, trading_commodity_id: eur, quantity_scale: 0, price_scale: 2, effective_from: opened
  });
  for (const date of [daysFromTodayISO(-2), todayISO()]) {
    await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
      base_commodity_id: index.commodity_id, quote_commodity_id: eur, quote_type: 'close',
      adjustment_basis: 'total_return', price_value: 10000, price_scale: 2, valuation_date: date, is_manual: true
    });
  }
  const params = new URLSearchParams({ from: daysFromTodayISO(-1), to: todayISO(), reporting_currency_id: String(currencyID),
    benchmark_commodity_id: String(index.commodity_id), benchmark_basis: 'total_return' });
  await page.goto(`/app/investments/returns?${params}`);
  const notice = page.getByTestId('returns-benchmark').getByRole('status');
  await expect(notice).toHaveText('FXIDX has prices in EUR, but EUR has no usable exchange rate on some days; those benchmark figures are left out.');
});

// R13 slice 6 (#119): a saved run keeps its figures after a backdated change
// that restates the live read, reopens from its own bytes and downloads them.
test('a saved returns run keeps its figures after a backdated buy on a phone', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-60);
  const checking = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Saved run checking', account_class: 'asset', account_kind: 'checking',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: 'RUNX', instrument_type: 'stock', display_name: 'Saved run example', symbol: 'RUNX',
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: opened
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: 'Saved run holding', opened_on: opened, effective_from: opened
  });
  const buy = (date: string, quantity: string, cash: string) => apiJSON(page, 'POST', '/api/v1/investments/buy', csrfToken, {
    transaction_date: date, commodity_id: instrument.commodity_id, holding_account_id: holding.id,
    cash_account_id: checking.id, quantity_value: quantity, quantity_scale: 0, cash_amount_value: cash, cash_amount_scale: 2,
    cash_commodity_id: currencyID, cost_basis_method: 'fifo'
  });
  await buy(daysFromTodayISO(-40), '10', '100000');
  await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
    base_commodity_id: instrument.commodity_id, quote_commodity_id: currencyID, quote_type: 'close',
    adjustment_basis: 'raw', price_value: 11000, price_scale: 2, valuation_date: todayISO(), is_manual: true
  });

  await page.goto(`/app/investments/returns?from=${daysFromTodayISO(-45)}&to=${todayISO()}&reporting_currency_id=${currencyID}`);
  const runs = page.getByTestId('returns-runs');
  await expect(runs.getByText('No saved runs yet.', { exact: true })).toBeVisible();
  await expect(page.getByTestId('returns-summary').getByTestId('returns-contributions')).toContainText('1,000.00');
  const saving = page.waitForResponse((res) => res.url().endsWith('/api/v1/investments/returns/runs') && res.request().method() === 'POST');
  await runs.getByRole('button', { name: 'Save this run', exact: true }).click();
  const run = await (await saving).json() as { id: number; captured_at: string };
  expect(run.id).toBeGreaterThan(0);
  const saved = page.getByTestId('saved-returns-run');
  await expect(saved.getByRole('heading', { name: new RegExp(`^Saved run #${run.id} · `) })).toBeVisible();
  await expect(runs.getByRole('button', { name: new RegExp(`^#${run.id} · saved `) })).toHaveAttribute('aria-pressed', 'true');

  // Five more units bought 20 days ago restate the live period only.
  await buy(daysFromTodayISO(-20), '5', '50000');
  await page.reload();
  await expect(page.getByTestId('returns-summary').getByTestId('returns-contributions')).toContainText('1,500.00');
  await runs.getByRole('button', { name: new RegExp(`^#${run.id} · saved `) }).click();
  const frozen = saved.getByTestId('returns-run-summary');
  await expect(frozen.getByTestId('returns-contributions')).toContainText('1,000.00');
  await expect(frozen.getByTestId('returns-closing')).toContainText('1,100.00');
  await expect(frozen.getByTestId('returns-twr-period')).toHaveText('+10.00%');
  await expect(frozen.getByText(/^Saved from the ledger as known at /)).toBeVisible();
  // A run saved without instruments has no instrument panel.
  await expect(saved.getByTestId('returns-instruments')).toHaveCount(0);

  const downloading = page.waitForEvent('download');
  await saved.getByRole('link', { name: 'Download saved run (JSON)', exact: true }).click();
  const download = await downloading;
  expect(download.suggestedFilename()).toBe(`rekenraam-returns-run-${run.id}.json`);
  expect(JSON.parse(await readFile((await download.path())!, 'utf8'))).toEqual(run);

  expect(await noHorizontalScroll(page)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Saved returns run on a phone');
});

test('[acceptance] returns by instrument agree live and saved, through corrected and cancelled income, on a phone', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-60);
  const checking = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Sleeve checking', account_class: 'asset', account_kind: 'checking',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  const income = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Sleeve income', account_class: 'income', account_kind: 'income',
    default_commodity_id: currencyID, allows_postings: true, opened_on: opened, effective_from: opened
  });
  const instruments = [];
  for (const symbol of ['SLVA', 'SLVB']) {
    const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
      commodity_code: symbol, instrument_type: 'stock', display_name: `Sleeve ${symbol}`, symbol,
      quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: opened
    });
    const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
      instrument_id: instrument.id, name: `Sleeve ${symbol} holding`, opened_on: opened, effective_from: opened
    });
    // Ten units for 1,000.00 from checking, outside the portfolio.
    await apiJSON(page, 'POST', '/api/v1/investments/buy', csrfToken, {
      transaction_date: daysFromTodayISO(-40), commodity_id: instrument.commodity_id, holding_account_id: holding.id,
      cash_account_id: checking.id, quantity_value: '10', quantity_scale: 0, cash_amount_value: '100000', cash_amount_scale: 2,
      cash_commodity_id: currencyID, cost_basis_method: 'fifo'
    });
    // 100.00 on every flow day; SLVA closes at 110.00, SLVB at 100.00.
    for (const [date, price] of [[daysFromTodayISO(-30), 10000], [daysFromTodayISO(-20), 10000], [todayISO(), symbol === 'SLVA' ? 11000 : 10000]] as const) {
      await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
        base_commodity_id: instrument.commodity_id, quote_commodity_id: currencyID, quote_type: 'close',
        adjustment_basis: 'raw', price_value: price, price_scale: 2, valuation_date: date, is_manual: true
      });
    }
    instruments.push(instrument);
  }
  const [first, second] = instruments;
  const dividend = (commodityID: number | undefined, exDate: string, date: string, amount: string) => ({
    transaction_date: date, ...(commodityID ? { commodity_id: commodityID, ex_date: exDate } : {}),
    cash_account_id: checking.id, cash_commodity_id: currencyID, income_account_id: income.id, amount_value: amount, amount_scale: 2
  });
  // Corrected: 20.00 recorded for SLVB, moved to SLVA.
  const corrected = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/dividend', csrfToken,
    dividend(second.commodity_id, daysFromTodayISO(-30), daysFromTodayISO(-25), '2000'));
  const replacement = await apiJSON<{ replacement_transaction: { id: number } }>(page, 'POST',
    `/api/v1/investments/transactions/${corrected.id}/replace-dividend`, csrfToken,
    { reason: 'Payer was SLVA', replacement: dividend(first.commodity_id, daysFromTodayISO(-30), daysFromTodayISO(-25), '2000') });
  // Cancelled: 50.00 for SLVA, reversed.
  const cancelled = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/dividend', csrfToken,
    dividend(first.commodity_id, daysFromTodayISO(-20), daysFromTodayISO(-15), '5000'));
  await apiJSON(page, 'POST', `/api/v1/investments/transactions/${cancelled.id}/reverse-dividend`, csrfToken, { reason: 'Paid in error' });
  // Unassigned: listed, carried by no instrument.
  await apiJSON(page, 'POST', '/api/v1/investments/dividend', csrfToken, dividend(undefined, '', daysFromTodayISO(-10), '700'));

  await page.goto(`/app/investments/returns?from=${daysFromTodayISO(-45)}&to=${todayISO()}&reporting_currency_id=${currencyID}`);
  // Anchored on a loaded result, so the absence is not vacuous.
  await expect(page.getByTestId('returns-summary').getByTestId('returns-contributions')).toContainText('2,000.00');
  await expect(page.getByTestId('returns-summary').getByTestId('returns-instruments')).toHaveCount(0);
  await page.getByTestId('returns-instruments-toggle').check();
  await page.getByRole('button', { name: 'Show returns', exact: true }).click();
  await expect(page).toHaveURL(/instruments=true/);

  const expectSleeves = async (panel: ReturnType<Page['getByTestId']>) => {
    const cards = panel.getByTestId('returns-instrument-card');
    await expect(cards).toHaveCount(2);
    const a = cards.filter({ hasText: 'SLVA' });
    // 1,020 / 1,000 on the ex date, then 1,100 / 1,000: +12.20 %.
    await expect(a.getByTestId('returns-instrument-twr')).toHaveText('+12.20%');
    await expect(a.getByTestId('returns-instrument-income')).toContainText('20.00');
    await expect(a.getByTestId('returns-instrument-withdrawals')).toContainText('20.00');
    await expect(a.getByTestId('returns-instrument-gain')).toContainText('120.00');
    const b = cards.filter({ hasText: 'SLVB' });
    await expect(b.getByTestId('returns-instrument-twr')).toHaveText('0.00%');
    await expect(b.getByTestId('returns-instrument-income')).toContainText('0.00');
    await expect(b.getByTestId('returns-instrument-gain')).toContainText('0.00');
    await expect(panel.getByTestId('returns-instruments-residual')).toContainText('0.00');
    await expect(panel.getByTestId('returns-instruments-not-attributed').getByRole('listitem')).toHaveCount(1);
    await expect(panel.getByTestId('returns-instruments-not-attributed')).toContainText('no instrument assigned');
    // Paid into checking, outside the portfolio: the portfolio's own fallback
    // says it is not counted, and the panel says so rather than assuming.
    await expect(panel.getByTestId('returns-instruments-not-attributed').getByRole('listitem')).toContainText('not counted');
    await expect(panel.getByTestId('returns-instruments-income-rule')).toBeVisible();
  };
  const live = page.getByTestId('returns-summary').getByTestId('returns-instruments');
  await expectSleeves(live);
  expect(await noHorizontalScroll(page)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Returns by instrument on a phone');

  const runs = page.getByTestId('returns-runs');
  const saving = page.waitForResponse((res) => res.url().endsWith('/api/v1/investments/returns/runs') && res.request().method() === 'POST');
  await runs.getByRole('button', { name: 'Save this run', exact: true }).click();
  const run = await (await saving).json() as { id: number; inputs: { request: { instruments?: boolean } } };
  expect(run.inputs.request.instruments).toBe(true);
  const saved = page.getByTestId('saved-returns-run').getByTestId('returns-instruments');
  await expectSleeves(saved);
  // Same figures, reasons and coverage notes, live and saved.
  expect(await saved.getByTestId('returns-instrument-card').allInnerTexts()).toEqual(await live.getByTestId('returns-instrument-card').allInnerTexts());
  await expect(runs.getByRole('button', { name: new RegExp(`^#${run.id} · saved `) })).toContainText('By instrument');

  // Reversing the replacement restates the live read only.
  await apiJSON(page, 'POST', `/api/v1/investments/transactions/${replacement.replacement_transaction.id}/reverse-dividend`, csrfToken, { reason: 'Not paid' });
  await page.reload();
  await expect(live.getByTestId('returns-instrument-card').filter({ hasText: 'SLVA' }).getByTestId('returns-instrument-twr')).toHaveText('+10.00%');
  await runs.getByRole('button', { name: new RegExp(`^#${run.id} · saved `) }).click();
  await expectSleeves(saved);

  // Wider screens show one table with the same rows.
  await page.setViewportSize({ width: 1280, height: 900 });
  const rows = saved.getByTestId('returns-instrument-row');
  await expect(rows).toHaveCount(2);
  await expect(rows.filter({ hasText: 'SLVA' }).getByTestId('returns-instrument-twr')).toHaveText('+12.20%');
  await expect(saved.getByTestId('returns-instrument-card').first()).toBeHidden();
  expect(await noHorizontalScroll(page)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Returns by instrument on a wide screen');
  // At tablet width the table can overflow its box, which a keyboard can scroll.
  await page.setViewportSize({ width: 768, height: 1024 });
  const scroller = saved.getByRole('region', { name: 'By instrument (scrolls sideways)', exact: true });
  await expect(scroller).toHaveAttribute('tabindex', '0');
  expect(await noHorizontalScroll(page)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Returns by instrument at tablet width');
});
