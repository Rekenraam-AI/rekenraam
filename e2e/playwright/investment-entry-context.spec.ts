import { expect, test, type Page } from '@playwright/test';
import { apiJSON } from './support/api';
import { todayISO } from './support/dates';
import { createCashAccount, readyForLedger } from './support/ledger';
import { expectNoAccessibilityViolations } from './support/a11y';

/**
 * #186: investment activity is entered where the user is working. A holding
 * account's register and an instrument's view offer the investment record
 * forms with the holding and instrument already chosen, on a phone-sized
 * screen, and neither offers the generic editor that would write an
 * investment-linked posting.
 */

test.use({ viewport: { width: 390, height: 844 } });

async function setup(page: Page) {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const name = `Context Co ${suffix}`;
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: `CX${suffix}`, instrument_type: 'stock', display_name: name, symbol: `CX${suffix}`,
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: todayISO()
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: `Context holding ${suffix}`, opened_on: todayISO(), effective_from: todayISO()
  });
  const cash = await createCashAccount(page, csrfToken, `Context cash ${suffix}`, currencyID);
  return { csrfToken, currencyID, suffix, name, instrument, holding, cash };
}

async function openActivity(page: Page, menuLabel: string, activityLabel: string) {
  await page.getByRole('button', { name: menuLabel }).click();
  await page.getByRole('button', { name: activityLabel, exact: true }).click();
}

test('a holding register records a pre-filled buy and opens it in the investment detail', async ({ page }) => {
  const { suffix, name, instrument, holding, cash } = await setup(page);
  const memo = `Context buy ${suffix}`;

  await page.goto(`/app/accounts/${holding.id}/register`);
  await expect(page.getByRole('heading', { name: `Context holding ${suffix}` })).toBeVisible();
  await expect(page.getByRole('button', { name: 'New transaction' })).toHaveCount(0);

  await openActivity(page, 'Record investment activity', 'Record buy');
  const dialog = page.getByRole('dialog');
  await expect(dialog.locator('#buy-instrument')).toHaveValue(name);
  await expect(dialog.locator('#buy-holding-account')).toHaveValue(String(holding.id));
  await dialog.locator('#buy-quantity').fill('5');
  await dialog.locator('#buy-cash-account').selectOption(String(cash.id));
  await dialog.locator('#buy-cash-amount').fill('100.00');
  await dialog.locator('#buy-memo').fill(memo);
  await dialog.getByRole('button', { name: 'Record buy' }).click();
  await expect(dialog).toBeHidden();

  const listed = await apiJSON<{ transactions: Array<{ description: string }> }>(
    page, 'GET', `/api/v1/transactions?commodity_id=${instrument.commodity_id}`);
  expect(listed.transactions.map((item) => item.description)).toEqual([memo]);

  // The register lists it without a reload, and the row opens the investment
  // detail with its correction and reversal history, not the generic editor.
  await page.getByText(memo).first().click();
  await expect(page.getByRole('heading', { name: 'Edit transaction' })).toHaveCount(0);
  await expect(page.locator('#investment-correction-heading')).toBeVisible();
});

test('an instrument view lists its activity and pre-fills the instrument and its only holding', async ({ page }) => {
  const { csrfToken, currencyID, suffix, name, instrument, holding, cash } = await setup(page);
  const memo = `Context seed buy ${suffix}`;
  await apiJSON(page, 'POST', '/api/v1/investments/buy', csrfToken, {
    transaction_date: todayISO(), commodity_id: instrument.commodity_id, holding_account_id: holding.id,
    cash_account_id: cash.id, quantity_value: '3', quantity_scale: 0,
    cash_amount_value: '6000', cash_amount_scale: 2, cash_commodity_id: currencyID, memo
  });

  await page.goto('/app/investments');
  await page.getByRole('tab', { name: 'Instruments' }).or(page.getByRole('button', { name: 'Instruments', exact: true })).first().click();
  await page.getByRole('link', { name: `View activity for ${name}` }).click();
  await expect(page).toHaveURL(new RegExp(`/app/investments/instruments/${instrument.commodity_id}$`));
  await expect(page.getByRole('heading', { name: new RegExp(name) })).toBeVisible();
  await expect(page.getByText(memo)).toBeVisible();
  await expect(page.getByText('+3')).toBeVisible();
  await expect(page.getByRole('button', { name: 'New transaction' })).toHaveCount(0);

  await page.getByRole('button', { name: 'Record investment activity' }).click();
  await expectNoAccessibilityViolations(page, 'instrument view investment entry');
  await page.getByRole('button', { name: 'Record sell', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.locator('#sell-instrument')).toHaveValue(name);
  await expect(dialog.locator('#sell-holding-account')).toHaveValue(String(holding.id));
});

test('investment entry in context is translated in every locale', async ({ page }) => {
  const { holding, instrument } = await setup(page);
  const locales: Array<[string, string, string]> = [
    ['en', 'Record investment activity', 'Back to investments'],
    ['de', 'Wertpapiervorgang erfassen', 'Zurück zur Geldanlage'],
    ['es', 'Registrar operación de inversión', 'Volver a inversiones'],
    ['fr', 'Enregistrer une opération de placement', 'Retour aux investissements'],
    ['nl', 'Beleggingsactiviteit vastleggen', 'Terug naar beleggingen'],
    ['ru', 'Записать инвестиционную операцию', 'Назад к инвестициям']
  ];
  try {
    for (const [locale, menu, back] of locales) {
      await page.evaluate((value) => localStorage.setItem('PARAGLIDE_LOCALE', value), locale);
      await page.goto(`/app/accounts/${holding.id}/register`);
      await expect(page.getByRole('button', { name: menu })).toBeVisible();
      await page.goto(`/app/investments/instruments/${instrument.commodity_id}`);
      await expect(page.getByRole('button', { name: menu })).toBeVisible();
      await expect(page.getByRole('link', { name: back })).toBeVisible();
    }
  } finally {
    await page.evaluate(() => localStorage.setItem('PARAGLIDE_LOCALE', 'en'));
  }
});
