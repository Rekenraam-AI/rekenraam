import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { apiJSON } from './support/api';
import { readyForLedger } from './support/ledger';
import { todayISO, daysFromTodayISO } from './support/dates';

test.use({ viewport: { width: 390, height: 844 } });

// #230 (R18 slice 1): unrealized gains valued as of a chosen date replay the
// holding to that date and price it with the latest eligible price then.
test('gains value positions as of a chosen date and name the profile at 390px', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  // No "date" in the name: other specs on the shared book select by a Date label.
  const name = `As-of holding ${suffix}`;
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: `DATED${suffix}`, instrument_type: 'stock', display_name: name, symbol: `DV${suffix}`,
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2,
    effective_from: daysFromTodayISO(-40)
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name, opened_on: daysFromTodayISO(-40), effective_from: daysFromTodayISO(-40)
  });
  const transferIn = (date: string, quantity: string, basis: string) =>
    apiJSON(page, 'POST', '/api/v1/investments/transfers/external/in', csrfToken, {
      effective_on: date, holding_account_id: holding.id, commodity_id: instrument.commodity_id,
      quantity_value: quantity, quantity_scale: 0, carried_basis_value: basis, carried_basis_scale: 2,
      cost_commodity_id: currencyID, original_acquired_on: date, source_evidence: { statement: 'As-of valuation test' }
    });
  await transferIn(daysFromTodayISO(-30), '2', '10000');
  await transferIn(daysFromTodayISO(-5), '3', '15000');
  const price = (date: string, value: number) =>
    apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
      base_commodity_id: instrument.commodity_id, quote_commodity_id: currencyID, quote_type: 'close',
      adjustment_basis: 'raw', price_value: value, price_scale: 2, valuation_date: date, is_manual: false
    });
  const pastPriceDate = daysFromTodayISO(-20);
  await price(pastPriceDate, 3000);
  await price(daysFromTodayISO(-2), 5000);

  await page.goto('/app/investments');
  await page.getByRole('button', { name: 'Gains', exact: true }).click();
  const row = page.getByRole('row').filter({ hasText: name });
  await expect(row).toContainText('250.00'); // 5 × 50.00 today
  await expect(page.getByTestId('gains-profile')).toContainText(todayISO());

  const valuationDate = page.getByLabel('Valuation date');
  const pastDate = daysFromTodayISO(-15);
  await valuationDate.fill(pastDate);
  await page.getByRole('button', { name: 'Revalue', exact: true }).click();
  await expect(page.getByTestId('gains-profile')).toContainText(pastDate);
  await expect(page.getByTestId('gains-profile')).toContainText('not a tax calculation');
  await expect(row).toContainText('60.00'); // the 2 units held then × 30.00
  await expect(row).toContainText(`Price date ${pastPriceDate}`);

  await valuationDate.fill(daysFromTodayISO(1));
  await expect(page.getByText('Valuation date cannot be after today.')).toBeVisible();
  await expect(valuationDate).toHaveAttribute('aria-invalid', 'true');
  await expect(row).toContainText('60.00');

  expect((await new AxeBuilder({ page }).include('main').analyze()).violations).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});
