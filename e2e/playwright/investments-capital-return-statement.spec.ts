import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { apiJSON } from './support/api';
import { ensureCurrency, readyForLedger } from './support/ledger';
import { daysFromTodayISO } from './support/dates';

test.use({ viewport: { width: 390, height: 844 } });

test('period statements disclose excluded capital-return excess and unknown coverage in six mobile locales and preserve saved disclosure', async ({ page }) => {
  const { csrfToken } = await readyForLedger(page);
  // A separate currency keeps this statement independent of other journeys.
  const currency = await ensureCurrency(page, csrfToken, 'DKK', 'Danish Krone');
  const suffix = Date.now();
  const openedOn = daysFromTodayISO(-60);
  const cash = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: `ROC statement cash ${suffix}`, account_class: 'asset', account_kind: 'brokerage_cash',
    default_commodity_id: currency, allows_postings: true, opened_on: openedOn, effective_from: openedOn
  });
  const position = async (code: string, unknown: boolean) => {
    const instrument = await apiJSON<{ id: number; commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
      commodity_code: `${code}${suffix}`, instrument_type: 'stock', display_name: `ROC statement ${code} ${suffix}`, symbol: `${code}${suffix}`,
      quote_commodity_id: currency, trading_commodity_id: currency, quantity_scale: 0, price_scale: 2, effective_from: openedOn
    });
    const holding = await apiJSON<{ id: number }>(page, 'POST', '/api/v1/investments/holding-accounts', csrfToken, {
      instrument_id: instrument.id, name: `ROC statement holding ${code} ${suffix}`, opened_on: openedOn, effective_from: openedOn
    });
    await apiJSON(page, 'POST', '/api/v1/investments/transfers/external/in', csrfToken, {
      effective_on: daysFromTodayISO(-40), holding_account_id: holding.id, commodity_id: instrument.commodity_id,
      quantity_value: '1', quantity_scale: 0, cost_commodity_id: currency, original_acquired_on: daysFromTodayISO(-40),
      ...(unknown ? { basis_knowledge: 'unknown' } : { carried_basis_value: '700', carried_basis_scale: 2 }),
      source_evidence: { statement: 'Capital-return period fixture' }
    });
    for (const offset of [-35, -2]) {
      await apiJSON(page, 'POST', '/api/v1/pricing/prices', csrfToken, {
        base_commodity_id: instrument.commodity_id, quote_commodity_id: currency, quote_type: 'manual', adjustment_basis: 'raw',
        price_value: 700, price_scale: 2, valuation_date: daysFromTodayISO(offset), is_manual: true
      });
    }
    return apiJSON<{ transaction: { id: number } }>(page, 'POST', '/api/v1/investments/return-of-capital', csrfToken, {
      effective_on: daysFromTodayISO(-10), payment_on: daysFromTodayISO(-5), holding_account_id: holding.id,
      commodity_id: instrument.commodity_id, cash_account_id: cash.id, currency_id: currency,
      amount_value: '1000', amount_scale: 2, source_evidence: { notice: `ROC-${code}-238` }
    });
  };
  const known = await position('RKNOWN', false);
  const locales = [
    ['en', 'Gains', 'excludes return-of-capital excess'],
    ['nl', 'Winsten', 'sluit het overschot bij kapitaalteruggave uit'],
    ['de', 'Gewinne', 'schließt Überschüsse aus Kapitalrückzahlungen aus'],
    ['fr', 'Plus-values', 'exclut les excédents de remboursement de capital'],
    ['es', 'Ganancias', 'Excluye el exceso de devolución de capital'],
    ['ru', 'Прибыль', 'не включает превышение возврата капитала']
  ] as const;
  const liveStatement = page.getByTestId('gains-statement').first();
  const row = liveStatement.getByTestId('gains-statement-row').filter({ has: page.getByRole('heading', { name: 'DKK', exact: true }) });
  const line = (name: string) => row.locator(`[data-line="${name}"]`);
  const visit = async (locale: string, tab: string) => {
    await page.evaluate((value) => localStorage.setItem('PARAGLIDE_LOCALE', value), locale);
    await page.goto('/app/investments');
    await page.getByRole('button', { name: tab, exact: true }).click();
    await liveStatement.locator('input[type="date"]').first().fill(daysFromTodayISO(-30));
    await liveStatement.locator('form').getByRole('button').click();
    await expect(row).toBeVisible();
  };
  try {
    for (const [locale, tab, scope] of locales) {
      await visit(locale, tab);
      await expect(line('total')).toContainText(scope);
      const decimal = new Intl.NumberFormat(locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      await expect(line('total')).toContainText(decimal.format(7));
      await expect(line('roc-receipt')).toContainText(decimal.format(10));
      await expect(line('roc-reduction')).toContainText(decimal.format(7));
      await expect(line('roc-excess')).toContainText(decimal.format(3));
      await liveStatement.getByTestId('statement-capital-return-evidence').locator('summary').first().click();
      await expect(liveStatement.getByTestId('statement-capital-return-evidence')).toContainText('ROC-RKNOWN-238');
      expect((await new AxeBuilder({ page }).include('[data-testid="gains-statement"]').analyze()).violations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    }
    await visit('en', 'Gains');
    const savedResponse = page.waitForResponse((res) => res.url().endsWith('/api/v1/investments/gains/runs') && res.request().method() === 'POST');
    await liveStatement.getByRole('button', { name: 'Save period statement', exact: true }).click();
    const saved = await (await savedResponse).json();
    expect(saved.schema_version).toBe(2);
    const frozen = page.getByTestId('saved-gains-run');
    await expect(frozen.getByTestId('gains-statement-row').filter({ has: page.getByRole('heading', { name: 'DKK', exact: true }) }).locator('[data-line="roc-excess"]')).toContainText('3.00');
    await apiJSON(page, 'POST', `/api/v1/investments/transactions/${known.transaction.id}/reverse-return-of-capital`, csrfToken, { reason: 'Notice withdrawn after saving' });
    await position('RUNKNOWN', true);
    for (const [locale, tab, scope] of locales) {
      await visit(locale, tab);
      await expect(line('total')).toContainText(scope);
      await expect(line('roc-receipt')).toContainText(new Intl.NumberFormat(locale, { minimumFractionDigits: 2 }).format(10));
      await expect(line('roc-excess')).not.toContainText(/0[.,]00/);
      await expect(line('roc-reduction')).not.toContainText(/0[.,]00/);
      if (locale === 'en') {
        await expect(line('roc-receipt')).toContainText('Receipts: 1; unresolved receipts: 1; unknown lot effects: 1.');
      }
      expect((await new AxeBuilder({ page }).include('[data-testid="gains-statement"]').analyze()).violations).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    }
    expect(await apiJSON(page, 'GET', `/api/v1/investments/gains/runs/${saved.id}`)).toEqual(saved);
  } finally {
    await page.evaluate(() => localStorage.setItem('PARAGLIDE_LOCALE', 'en'));
  }
});
