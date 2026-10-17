import { expect, test } from './support/fixtures';
import { apiJSON } from './support/api';
import { readyForLedger } from './support/ledger';
import { todayISO } from './support/dates';
import { expectNoAccessibilityViolations } from './support/a11y';

test.use({ viewport: { width: 390, height: 844 } });
function ago(days: number) { const date = new Date(); date.setDate(date.getDate() - days); return todayISO(date); }

// #259: the open and closed lot status badges must clear colour contrast in
// both themes. A FIFO sell of the first of two lots leaves one of each.
test('the phone lot detail with an open and a closed lot is accessible in both themes', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = `lotb${Date.now()}`;
  const cash = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, { name: `Lot badge cash ${suffix}`, account_class: 'asset', account_kind: 'brokerage_cash', default_commodity_id: currencyID, allows_postings: true, opened_on: ago(60), effective_from: ago(60) });
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, { commodity_code: suffix.toUpperCase(), instrument_type: 'stock', display_name: `Lot badge ${suffix}`, symbol: suffix.toUpperCase(), quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 3, price_scale: 2, effective_from: ago(60) });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, { instrument_id: instrument.id, name: `Lot badge holding ${suffix}`, opened_on: ago(60), effective_from: ago(60) });
  const trade = (side: 'buy' | 'sell', days: number) => apiJSON(page, 'POST', `/api/v1/investments/${side}`, csrfToken, { transaction_date: ago(days), commodity_id: instrument.commodity_id, holding_account_id: holding.id, cash_account_id: cash.id, quantity_value: '1', quantity_scale: 0, cash_amount_value: '1000', cash_amount_scale: 2, cash_commodity_id: currencyID, cost_basis_method: 'fifo' });
  await trade('buy', 40);
  await trade('buy', 30);
  await trade('sell', 20);

  for (const colorScheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme });
    await page.goto('/app/investments');
    await page.getByRole('button').filter({ hasText: `Lot badge ${suffix}` }).click();
    const detail = page.getByRole('complementary', { name: 'Lot detail' });
    await expect(detail.getByText('Closed', { exact: true })).toBeVisible();
    await expect(detail.getByText('Open', { exact: true })).toBeVisible();
    await expectNoAccessibilityViolations(page, `${colorScheme} lot detail`);
  }
});
