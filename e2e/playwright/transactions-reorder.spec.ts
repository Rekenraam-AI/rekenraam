import { expect, test, type Page } from '@playwright/test';
import { apiJSON } from './support/api';
import { createCashAccount, readyForLedger } from './support/ledger';
import { todayISO } from './support/dates';

/**
 * #185 slice 2: same-day reordering is a mode, off by default. A move places
 * an entry beside its visible same-day neighbour everywhere both appear; an
 * account's "Only in this account" switch reorders that register alone, for a
 * statement that lists the day differently. Phone-sized screen.
 */

test.use({ viewport: { width: 390, height: 844 } });

type Seeded = Awaited<ReturnType<typeof seed>>;

async function seed(page: Page) {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const checking = await createCashAccount(page, csrfToken, `Reorder checking ${suffix}`, currencyID);
  const savings = await createCashAccount(page, csrfToken, `Reorder savings ${suffix}`, currencyID);
  const category = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/categories', csrfToken, {
    name: `Reorder groceries ${suffix}`, category_type: 'expense'
  });
  // Every account and the category are new, so their registers hold only
  // this test's entries; the main list is filtered to one of them.
  const day = todayISO();
  const posting = (accountID: number, value: string) => ({
    account_id: accountID, commodity_id: currencyID, quantity_value: value, quantity_scale: 2
  });
  const create = (description: string, postings: ReturnType<typeof posting>[], kind = 'ordinary') =>
    apiJSON<{ id: number }>(page, 'POST', '/api/v1/transactions', csrfToken, {
      transaction_date: day, transaction_kind: kind, status: 'posted', description,
      journal_entries: [{ entry_date: day, entry_kind: kind === 'transfer' ? 'transfer_leg' : 'ordinary', postings }]
    });
  return { suffix, day, checking, savings, category, create, posting };
}

/** An account's transactions on the seeded day, oldest first. */
async function registerOrder(page: Page, accountID: number): Promise<number[]> {
  const register = await apiJSON<{ entries: Array<{ transaction_id: number }> }>(
    page, 'GET', `/api/v1/accounts/${accountID}/register`);
  return register.entries.map((entry) => entry.transaction_id).reverse();
}

function row(page: Page, text: string) {
  return page.getByRole('row').filter({ hasText: text });
}

test('reordering on the main list applies in every account both entries share', async ({ page }) => {
  const s: Seeded = await seed(page);
  const coffee = await s.create(`Coffee ${s.suffix}`, [s.posting(s.checking.id, '-300'), s.posting(s.category.id, '300')]);
  const salary = await s.create(`Salary ${s.suffix}`, [s.posting(s.savings.id, '-5000'), s.posting(s.checking.id, '5000')], 'transfer');

  await page.goto(`/app/transactions?account_id=${s.checking.id}`);
  await expect(row(page, `Salary ${s.suffix}`)).toBeVisible();
  // Off by default: no arrows until the mode starts.
  await expect(page.getByRole('button', { name: 'Move earlier in the day' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Reorder' }).click();
  await expect(page.getByText('The order changes on every screen that shows both entries.')).toBeVisible();
  // The newest is on top; the coffee cannot move earlier than the start of the day.
  await expect(row(page, `Coffee ${s.suffix}`).getByRole('button', { name: 'Move earlier in the day' })).toBeDisabled();
  await row(page, `Salary ${s.suffix}`).getByRole('button', { name: 'Move earlier in the day' }).click();

  await expect.poll(() => registerOrder(page, s.checking.id)).toEqual([salary.id, coffee.id]);
  await page.getByRole('button', { name: 'Done' }).click();
  await expect(page.getByRole('button', { name: 'Move earlier in the day' })).toHaveCount(0);
});

test('only in this account reorders one register and leaves the other', async ({ page }) => {
  const s: Seeded = await seed(page);
  const first = await s.create(`Transfer one ${s.suffix}`, [s.posting(s.checking.id, '-1000'), s.posting(s.savings.id, '1000')], 'transfer');
  const second = await s.create(`Transfer two ${s.suffix}`, [s.posting(s.checking.id, '-2000'), s.posting(s.savings.id, '2000')], 'transfer');

  await page.goto(`/app/accounts/${s.savings.id}/register`);
  await page.getByRole('button', { name: 'Reorder' }).click();
  await page.getByLabel('Only in this account').check();
  await expect(page.getByText('Other screens keep their order.')).toBeVisible();
  await row(page, `Transfer two ${s.suffix}`).getByRole('button', { name: 'Move earlier in the day' }).click();

  await expect.poll(() => registerOrder(page, s.savings.id)).toEqual([second.id, first.id]);
  expect(await registerOrder(page, s.checking.id)).toEqual([first.id, second.id]);
});

test('reordering in a category applies in the account both entries share', async ({ page }) => {
  const s: Seeded = await seed(page);
  const bread = await s.create(`Bread ${s.suffix}`, [s.posting(s.checking.id, '-250'), s.posting(s.category.id, '250')]);
  const milk = await s.create(`Milk ${s.suffix}`, [s.posting(s.checking.id, '-120'), s.posting(s.category.id, '120')]);

  await page.goto(`/app/categories/${s.category.id}`);
  await page.getByRole('button', { name: 'Reorder' }).click();
  await expect(page.getByLabel('Only in this account')).toHaveCount(0);
  await row(page, `Bread ${s.suffix}`).getByRole('button', { name: 'Move later in the day' }).click();

  await expect.poll(() => registerOrder(page, s.category.id)).toEqual([milk.id, bread.id]);
  expect(await registerOrder(page, s.checking.id)).toEqual([milk.id, bread.id]);
});
