import { expect, test, type Page } from '@playwright/test';
import { apiJSON } from './support/api';
import { todayISO } from './support/dates';
import { createCashAccount, expectSavedTransaction, readyForLedger } from './support/ledger';

/**
 * #185: the account register and a category's transactions share the main
 * screen's workspace, so a transaction can be entered where the user is
 * working, starting from that account or category, on a phone-sized screen.
 */

test.use({ viewport: { width: 390, height: 844 } });

async function addForm(page: Page) {
  await page.getByRole('button', { name: 'New transaction' }).click();
  await expect(page.getByRole('heading', { name: 'Add transaction' })).toBeVisible();
  return page.locator('form').filter({ has: page.getByRole('heading', { name: 'Add transaction' }) });
}

test('a transaction entered in an account register starts in that account', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const account = await createCashAccount(page, csrfToken, `Workspace checking ${suffix}`, currencyID);
  const description = `Register groceries ${suffix}`;

  await page.goto(`/app/accounts/${account.id}/register`);
  await expect(page.getByRole('heading', { name: account.name })).toBeVisible();
  const form = await addForm(page);
  await expect(form.getByLabel('Account').locator('option:checked')).toHaveText(account.name);
  await form.getByLabel('Date').fill(todayISO());
  await form.getByLabel('Description').fill(description);
  await form.getByLabel('Amount').fill('-12.40');
  await form.getByPlaceholder('Search categories').click();
  await form.locator('ul[role="listbox"] button[role="option"]').first().click();
  await form.getByRole('button', { name: 'Add transaction' }).click();
  await expectSavedTransaction(page, description);

  // The register lists it without a reload.
  await page.getByRole('button', { name: 'Close' }).first().click();
  await expect(page.getByText(description)).toBeVisible();
});

test('a transaction entered in a category starts in that category', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const account = await createCashAccount(page, csrfToken, `Workspace wallet ${suffix}`, currencyID);
  const category = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/categories', csrfToken, {
    name: `Workspace hobbies ${suffix}`, category_type: 'expense'
  });
  const description = `Category paints ${suffix}`;

  await page.goto(`/app/categories/${category.id}`);
  await expect(page.getByRole('heading', { name: `Workspace hobbies ${suffix}` })).toBeVisible();
  const form = await addForm(page);
  await expect(form.getByText(`Workspace hobbies ${suffix}`)).toBeVisible();
  await form.getByLabel('Date').fill(todayISO());
  await form.getByLabel('Description').fill(description);
  await form.getByLabel('Account').selectOption({ label: account.name });
  await form.getByLabel('Amount').fill('-30.00');
  await form.getByRole('button', { name: 'Add transaction' }).click();
  await expectSavedTransaction(page, description);

  const listed = await apiJSON<{ transactions: Array<{ description: string }> }>(
    page, 'GET', `/api/v1/transactions?category_id=${category.id}`);
  expect(listed.transactions.map((item) => item.description)).toContain(description);
});

test('a holding account points to investment entry instead of the generic editor', async ({ page }) => {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const instrument = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: `WS${suffix}`, instrument_type: 'stock', display_name: `Workspace Co ${suffix}`, symbol: `WS${suffix}`,
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: todayISO()
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: `Workspace holding ${suffix}`, opened_on: todayISO(), effective_from: todayISO()
  });

  await page.goto(`/app/accounts/${holding.id}/register`);
  await expect(page.getByRole('heading', { name: `Workspace holding ${suffix}` })).toBeVisible();
  await expect(page.getByRole('button', { name: 'New transaction' })).toHaveCount(0);
  await page.getByRole('link', { name: 'Record investment activity' }).click();
  await expect(page).toHaveURL(/\/app\/investments$/);
});
