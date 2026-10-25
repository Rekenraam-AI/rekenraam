import { expect, test } from './support/owned-book';
import { apiJSON } from './support/api';
import { readyForLedger } from './support/ledger';
import { daysFromTodayISO } from './support/dates';
import { expectNoAccessibilityViolations } from './support/a11y';

// #410: net worth is trade-date, so a period whose trade has not settled yet
// counts the unpaid cash as unsettled and the screen says so.
test.use({ viewport: { width: 390, height: 844 } });
test('net worth discloses an unsettled trade on a phone', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const opened = daysFromTodayISO(-60);
  const traded = daysFromTodayISO(-5);
  const settled = daysFromTodayISO(-3);
  const cash = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: 'Settlement cash', account_class: 'asset', account_kind: 'brokerage_cash', default_commodity_id: currencyID,
    allows_postings: true, opened_on: opened, effective_from: opened
  });
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: 'UNSET', instrument_type: 'stock', display_name: 'Settlement review', symbol: 'UNSET',
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: opened
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: 'Settlement holding', opened_on: opened, effective_from: opened
  });
  await apiJSON(page, 'POST', '/api/v1/investments/buy', csrfToken, {
    transaction_date: traded, settlement_date: settled, commodity_id: instrument.commodity_id, holding_account_id: holding.id,
    cash_account_id: cash.id, quantity_value: '1', quantity_scale: 0, cash_amount_value: '10000', cash_amount_scale: 2,
    cash_commodity_id: currencyID, cost_basis_method: 'fifo'
  });
  const query = `view=net-worth&start_date=${traded}&end_date=${settled}&bucket=day&account_id=${cash.id}&reporting_currency_id=${currencyID}`;
  await page.goto(`/app/reports?${query}`);
  await expect(page.getByText('Some periods include trades that had not settled yet.', { exact: false })).toBeVisible();
  const rows = page.getByRole('table').locator('tbody tr');
  await expect(rows.filter({ hasText: 'Includes unsettled trades' })).toHaveCount(4);
  await expect(rows.filter({ hasText: 'Includes unsettled trades' }).first()).toContainText('-100.00');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Reports with unsettled trades');
});
