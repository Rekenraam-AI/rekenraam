import { expect, test, type Page } from './support/fixtures';
import { apiJSON } from './support/api';
import { readyForLedger } from './support/ledger';
import { todayISO } from './support/dates';
import { expectNoAccessibilityViolations } from './support/a11y';
import { readFileSync } from 'node:fs';

/**
 * #255: a short holder's payment in lieu of a dividend is recorded, corrected
 * and reversed on a phone-sized screen. It is charged to an expense account
 * and never changes the short position.
 */

test.use({ viewport: { width: 390, height: 844 } });

function daysFromTodayISO(days: number): string {
  const date = new Date();
  date.setDate(date.getDate() + days);
  return todayISO(date);
}

async function setup(page: Page) {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = `pil${Date.now()}`;
  const openedOn = daysFromTodayISO(-60);
  const cash = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: `PIL cash ${suffix}`, account_class: 'asset', account_kind: 'brokerage_cash',
    default_commodity_id: currencyID, allows_postings: true, opened_on: openedOn, effective_from: openedOn
  });
  const expense = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: `Borrow costs ${suffix}`, account_class: 'expense', account_kind: 'expense',
    default_commodity_id: currencyID, allows_postings: true, opened_on: openedOn, effective_from: openedOn
  });
  const name = `Lent Co ${suffix}`;
  const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: suffix.toUpperCase(), instrument_type: 'stock', display_name: name, symbol: suffix.toUpperCase(),
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 3, price_scale: 2, effective_from: openedOn
  });
  const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
    instrument_id: instrument.id, name: `Short holding ${suffix}`, opened_on: openedOn, effective_from: openedOn
  });
  await apiJSON(page, 'POST', '/api/v1/investments/short-sale', csrfToken, {
    transaction_date: daysFromTodayISO(-30), commodity_id: instrument.commodity_id, holding_account_id: holding.id,
    cash_account_id: cash.id, quantity_value: '10', quantity_scale: 0, cash_amount_value: '100000',
    cash_amount_scale: 2, cash_commodity_id: currencyID
  });
  return { csrfToken, currencyID, name, cashID: cash.id, expenseID: expense.id, holdingID: holding.id, commodityID: instrument.commodity_id };
}

async function expenseBalance(page: Page, accountID: number): Promise<string> {
  const ledger = await apiJSON<{ transactions: Array<{ status: string; journal_entries: Array<{ postings: Array<{ account_id: number; quantity_value: string; quantity_scale: number }> }> }> }>(
    page, 'GET', `/api/v1/transactions?account_id=${accountID}`);
  let cents = 0n;
  for (const transaction of ledger.transactions) {
    for (const entry of transaction.journal_entries) {
      for (const posting of entry.postings) {
        if (posting.account_id === accountID) cents += (BigInt(posting.quantity_value) * 100n) / 10n ** BigInt(posting.quantity_scale);
      }
    }
  }
  return cents.toString();
}

test('a payment in lieu is recorded, corrected and reversed on mobile', async ({ page }) => {
  const s = await setup(page);
  await page.goto('/app/investments');
  await page.getByRole('button', { name: 'Record payment in lieu' }).click();
  const form = page.getByRole('dialog', { name: 'Payment in lieu of dividend' });
  await form.getByLabel('Short holding').selectOption(String(s.holdingID));
  await expect(form.getByLabel('Instrument')).toHaveValue(String(s.commodityID));
  await form.getByLabel('Ex-dividend date').fill(daysFromTodayISO(-10));
  await form.getByLabel('Payment date').fill(daysFromTodayISO(-8));
  await form.getByLabel('Cash account').selectOption(String(s.cashID));
  await form.getByLabel(/Amount owed/).fill('5.00');
  await form.getByLabel('Expense account').selectOption(String(s.expenseID));
  await expectNoAccessibilityViolations(page, 'payment in lieu form');
  await form.getByRole('button', { name: 'Record payment' }).click();
  await expect(form).toBeHidden();
  await expect.poll(() => expenseBalance(page, s.expenseID)).toBe('500');
  // The short position is unchanged: still ten units owed.
  await expect(page.getByRole('button', { name: new RegExp(s.name) })).toContainText('-10');

  const ledger = await apiJSON<{ transactions: Array<{ id: number }> }>(page, 'GET', `/api/v1/transactions?account_id=${s.expenseID}`);
  await page.goto(`/app/transactions?transaction_id=${ledger.transactions[0].id}`);
  await page.getByRole('button', { name: 'Correct payment in lieu…' }).click();
  const correction = page.getByRole('dialog', { name: 'Correct payment in lieu' });
  await expect(correction.getByLabel(/Amount owed/)).toHaveValue('5.00');
  await correction.getByLabel('Reason for reversal').fill('broker charged 6.00');
  await correction.getByLabel(/Amount owed/).fill('6.00');
  await correction.getByRole('button', { name: 'Save correction' }).click();
  await expect(correction).toBeHidden();
  await expect.poll(() => expenseBalance(page, s.expenseID)).toBe('600');

  const chain = await apiJSON<{ effective_transaction_id: number }>(page, 'GET',
    `/api/v1/investments/transactions/${ledger.transactions[0].id}/correction-chain`);
  await page.goto(`/app/transactions?transaction_id=${chain.effective_transaction_id}`);
  await page.getByRole('button', { name: 'Reverse payment in lieu…' }).click();
  await page.getByLabel('Reason for reversal').fill('charged in error');
  await page.getByRole('button', { name: 'Review and reverse' }).click();
  await expect.poll(() => expenseBalance(page, s.expenseID)).toBe('0');
});

test('a holding that was not short refuses a payment in lieu with a translated reason', async ({ page }) => {
  const s = await setup(page);
  await page.goto('/app/investments');
  await page.getByRole('button', { name: 'Record payment in lieu' }).click();
  const form = page.getByRole('dialog', { name: 'Payment in lieu of dividend' });
  await form.getByLabel('Short holding').selectOption(String(s.holdingID));
  // Before the short sale: nothing was owed.
  await form.getByLabel('Ex-dividend date').fill(daysFromTodayISO(-40));
  await form.getByLabel('Payment date').fill(daysFromTodayISO(-38));
  await form.getByLabel('Cash account').selectOption(String(s.cashID));
  await form.getByLabel(/Amount owed/).fill('5.00');
  await form.getByLabel('Expense account').selectOption(String(s.expenseID));
  await form.getByRole('button', { name: 'Record payment' }).click();
  await expect(form.getByRole('alert')).toContainText('was not short this instrument');
});

// #389 read-side-first: Go integration tests construct unsupported historical
// attribution through the real writer. Here explicit saved-response fixtures
// exercise translated mixed severities and navigation until #388 admits the
// same history through ordinary browser commands.
for (const locale of ['en', 'es', 'fr', 'nl', 'de', 'ru']) {
  test(`attribution warnings and unavailable coverage stay visible in ${locale}`, async ({ page }) => {
    const s = await setup(page);
    const payment = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/payment-in-lieu', s.csrfToken, {
      holding_account_id: s.holdingID, commodity_id: s.commodityID,
      ex_date: daysFromTodayISO(-20), transaction_date: daysFromTodayISO(-18),
      cash_account_id: s.cashID, cash_commodity_id: s.currencyID, expense_account_id: s.expenseID,
      amount_value: '5000', amount_scale: 2
    });
    const copy = JSON.parse(readFileSync(new URL(`../../frontend/messages/settings/${locale}.json`, import.meta.url), 'utf8')) as Record<string, string>;
    const warning = { transaction_id: payment.id, reason: 'no_short_at_entitlement_close' };
    const failed = locale !== 'en';
    await page.route('**/api/v1/maintenance/self-check', async route => {
      await route.fulfill({ json: { has_run: true, read_only: true, latest_run: {
        id: 1, trigger: 'manual', status: failed ? 'failed' : 'warning',
        failed_check_count: failed ? 1 : 0, warning_check_count: 1, incomplete_check_count: 1,
        error_summary: '', started_at: '2026-10-23T00:00:00Z', finished_at: '2026-10-23T00:00:01Z',
        results: [{ check_id: 'investment_attribution', status: 'warning', finding_count: 0,
          sample: [], warning_count: 1, warning_sample: [warning], unavailable_count: 1,
          unavailable_sample: [payment.id], summary: '', explanation: '', next_step: '' },
          ...(failed ? [{ check_id: 'entry_balance', status: 'failed', finding_count: 1,
            sample: [payment.id], warning_count: 0, warning_sample: [], unavailable_count: 0,
            unavailable_sample: [], summary: 'Entry balance', explanation: '', next_step: '' }] : [])]
      } } });
    });
    await page.route(`**/api/v1/investments/transactions/${payment.id}/correction-chain`, async route => {
      const response = await route.fetch();
      const chain = await response.json();
      chain.effective_payment_in_lieu.attribution = { status: 'warning', reasons: [warning.reason], unavailable: false };
      await route.fulfill({ response, json: chain });
    });
    await page.evaluate(value => localStorage.setItem('PARAGLIDE_LOCALE', value), locale);
    await page.goto('/app/settings/data');
    const summary = page.getByTestId('selfcheck-summary');
    await expect(summary).toContainText(copy.settings_data_selfcheck_warning.replace('{count}', '1'));
    await expect(summary).toContainText(copy.settings_data_selfcheck_failed_unavailable.replace('{count}', '1'));
    await expect(page.getByTestId('selfcheck-warnings')).toHaveText(copy.settings_data_selfcheck_warnings.replace('{count}', '1'));
    await expect(page.getByTestId('selfcheck-unavailable')).toHaveText(copy.settings_data_selfcheck_unavailable.replace('{count}', '1'));
    const link = page.locator(`a[href="/app/transactions?transaction_id=${payment.id}"]`).first();
    await expect(link).toHaveAccessibleName(copy.investments_attribution_payment_link.replace('{id}', String(payment.id)));
    await expectNoAccessibilityViolations(page);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    await link.click();
    await expect(page.getByTestId('investment-attribution-diagnostic')).toContainText(copy.investments_attribution_no_short);
    await expect(page.getByTestId('investment-attribution-diagnostic')).toContainText(copy.investments_attribution_review);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  });
}
