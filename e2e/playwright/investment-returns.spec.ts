import { expect, test } from './support/owned-book';
import type { Page } from './support/fixtures';
import { apiJSON } from './support/api';
import { readyForLedger } from './support/ledger';
import { daysFromTodayISO, todayISO } from './support/dates';
import { expectNoAccessibilityViolations } from './support/a11y';

// R13 slices 2 and 3 (#119): the returns screen and its chart over the returns_v1 read. The
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
  await expect(summary.getByText('Returns holding', { exact: true })).toBeVisible();

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
