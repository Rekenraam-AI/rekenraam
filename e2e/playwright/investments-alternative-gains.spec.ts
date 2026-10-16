import { test, expect } from './support/fixtures';
import AxeBuilder from '@axe-core/playwright';
import { apiJSON } from './support/api';
import { readyForLedger } from './support/ledger';
import { todayISO, daysFromTodayISO } from './support/dates';

test.use({ viewport: { width: 390, height: 844 } });

test('alternative basis methods compare with operational gains without changing them at 390px', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const name = `Alternative holding ${suffix}`;
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: `ALT${suffix}`, instrument_type: 'stock', display_name: name, symbol: `A${suffix}`,
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2,
    effective_from: daysFromTodayISO(-40)
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name, opened_on: daysFromTodayISO(-40), effective_from: daysFromTodayISO(-40)
  });
  for (const [days, basis] of [[-30, '4000'], [-20, '12000']] as const) {
    await apiJSON(page, 'POST', '/api/v1/investments/transfers/external/in', csrfToken, {
      effective_on: daysFromTodayISO(days), holding_account_id: holding.id, commodity_id: instrument.commodity_id,
      quantity_value: '4', quantity_scale: 0, carried_basis_value: basis, carried_basis_scale: 2,
      cost_commodity_id: currencyID, original_acquired_on: daysFromTodayISO(days), source_evidence: { statement: 'Alternative test' }
    });
  }
  // A write-off supplies a zero-proceeds disposal without needing a cash account.
  await apiJSON(page, 'POST', '/api/v1/investments/write-off', csrfToken, {
    transaction_date: daysFromTodayISO(-10), holding_account_id: holding.id, commodity_id: instrument.commodity_id,
    cost_commodity_id: currencyID, quantity_value: '2', quantity_scale: 0, cost_basis_method: 'fifo', reason: 'Alternative projection test'
  });
  await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
    base_commodity_id: instrument.commodity_id, quote_commodity_id: currencyID, quote_type: 'close',
    adjustment_basis: 'raw', price_value: 3000, price_scale: 2, valuation_date: todayISO(), is_manual: true
  });
  await page.goto('/app/investments');
  await page.getByRole('button', { name: 'Gains', exact: true }).click();
  const comparison = page.getByTestId('alternative-gains');
  const method = comparison.getByLabel('Compare basis method');
  const position = comparison.getByRole('row').filter({ hasText: name });
  await method.selectOption('lifo');
  await expect(comparison.getByRole('heading', { name: 'Comparison: LIFO' })).toBeVisible();
  await expect(position).toContainText('40.00'); // operational 180 - 140
  await expect(position).toContainText('80.00'); // LIFO 180 - 100
  await expect(comparison).toContainText('Specific-lot choices and transfer depletions use the selected method.');
  await expect(comparison).toContainText('not a tax calculation');
  await method.selectOption('average_cost');
  await expect(comparison.getByRole('heading', { name: 'Comparison: Average cost' })).toBeVisible();
  await expect(position).toContainText('60.00'); // average 180 - 120
  await expect(position).toContainText('40.00');
  expect((await new AxeBuilder({ page }).include('main').analyze()).violations).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await method.selectOption('');
  await expect(comparison.getByRole('table')).toHaveCount(0);
  const operational = await apiJSON<{ profile: { name: string }; unrealized: Array<{ account_id: number; remaining_cost_basis_value: string; remaining_cost_basis_scale: number }> }>(page, 'GET', '/api/v1/investments/gains', csrfToken);
  expect(operational.profile.name).toBe('operational_v1');
  const remaining = operational.unrealized.find((row) => row.account_id === holding.id)!;
  expect(BigInt(remaining.remaining_cost_basis_value)).toBe(140n * 10n ** BigInt(remaining.remaining_cost_basis_scale));
});
