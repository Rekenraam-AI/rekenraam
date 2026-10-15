import { expect, test, type Page } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import process from 'node:process';
import { apiJSON } from './support/api';
import { readyForLedger, createCashAccount } from './support/ledger';
import { todayISO } from './support/dates';
import { expectNoAccessibilityViolations } from './support/a11y';
import { requireTrading212Stub, setTrading212Dividends } from './support/trading212';

requireTrading212Stub();
test.use({ viewport: { width: 390, height: 844 } });

async function dividendSource(page: Page) {
  const { csrfToken, currencyID } = await readyForLedger(page);
  const suffix = Date.now();
  const ticker = `DIV${suffix}`;
  const cash = await createCashAccount(page, csrfToken, `Dividend cash ${suffix}`, currencyID);
  const category = (kind: 'income' | 'expense') => apiJSON<{ id: number }>(page, 'POST', '/api/v1/accounts', csrfToken, {
    name: `Dividend ${kind} ${suffix}`, account_class: kind, account_kind: kind,
    default_commodity_id: currencyID, allows_postings: true, opened_on: todayISO(), effective_from: todayISO()
  });
  const income = await category('income');
  const tax = await category('expense');
  const instrument = await apiJSON<{ commodity_id: number }>(page, 'POST', '/api/v1/investments/instruments', csrfToken, {
    commodity_code: ticker, instrument_type: 'stock', display_name: `Dividend payer ${suffix}`, symbol: ticker,
    quote_commodity_id: currencyID, trading_commodity_id: currencyID, quantity_scale: 0, price_scale: 2, effective_from: todayISO()
  });
  const apiKey = `dividend-${suffix}`;
  await setTrading212Dividends(apiKey, { ticker, isin: '', currency: 'USD' }, [{ reference: `dividend-${suffix}`, amount: 85, date: todayISO() }]);
  const connectionName = `Dividend broker ${suffix}`;
  await apiJSON(page, 'POST', '/api/v1/import-connections', csrfToken, {
    source: 'trading212', display_name: connectionName, api_key: apiKey, cash_account_id: cash.id
  });
  const fetchRows = async (action: 'Import' | 'Re-fetch full history') => {
    await page.goto('/app/import');
    await page.getByRole('row').filter({ hasText: connectionName }).getByRole('button', { name: action, exact: true }).click();
    await expect(page.getByRole('button', { name: 'Commit to ledger' })).toBeVisible({ timeout: 15_000 });
  };
  const activity = () => apiJSON<{ transactions: Array<{ id: number }> }>(page, 'GET', `/api/v1/transactions?commodity_id=${instrument.commodity_id}`);
  await fetchRows('Import');
  return { csrfToken, currencyID, cash, income, tax, instrument, fetchRows, activity };
}

test('[acceptance] imported dividend waits for owner facts and records its known payer without defaults on a phone', async ({ page }) => {
  const f = await dividendSource(page);
  await page.getByRole('button', { name: 'Commit to ledger' }).click();
  await expect(page.getByText('1 dividend payments remain in review.')).toBeVisible();
  expect((await f.activity()).transactions).toHaveLength(0);
  await page.getByRole('link', { name: 'Review dividends and repair attribution', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Pending dividend', exact: true })).toBeVisible();
  await expect(page.getByLabel('Dividend amount', { exact: true })).toHaveValue('');
  await expect(page.getByLabel('Withholding amount', { exact: true })).toHaveValue('');
  await expect(page.getByRole('combobox', { name: 'Instrument', exact: true })).toHaveValue(String(f.instrument.commodity_id));
  await page.getByRole('combobox', { name: 'Income account', exact: true }).selectOption(String(f.income.id));
  await page.getByRole('combobox', { name: 'Withholding account', exact: true }).selectOption(String(f.tax.id));
  await page.getByLabel('Dividend amount', { exact: true }).fill('100.00');
  await page.getByLabel('Withholding amount', { exact: true }).fill('15.00');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Imported dividend review on a phone');
  await page.getByRole('button', { name: 'Record reviewed dividend', exact: true }).click();
  await expect(page.getByText('Dividend recorded.', { exact: true })).toBeVisible();
  expect((await f.activity()).transactions).toHaveLength(1);
  await f.fetchRows('Re-fetch full history');
  await page.getByRole('button', { name: 'Commit to ledger' }).click();
  await expect(page.getByText('0 transactions committed')).toBeVisible();
  expect((await f.activity()).transactions).toHaveLength(1);
});

test('[acceptance] historical cash dividend repair keeps the original payment and owner note on a phone', async ({ page }) => {
  test.skip(Boolean(process.env.E2E_BASE_URL), 'historical source-link fixture needs the disposable local e2e database');
  const f = await dividendSource(page);
  const reviews = await apiJSON<{ rows: Array<{ batch_id: number; row_id: number; state: string; resolution: { account_id?: number } }> }>(page, 'GET', '/api/v1/import-dividends');
  const row = reviews.rows.find(r => r.state === 'pending' && r.resolution.account_id === f.cash.id)!;
  expect(row).toBeTruthy();
  const legacy = await apiJSON<{ id: number; version_id: number }>(page, 'POST', '/api/v1/transactions', f.csrfToken, {
    transaction_date: todayISO(), description: 'Historical cash dividend', note_markdown: 'Keep the owner note',
    journal_entries: [{ entry_date: todayISO(), entry_kind: 'ordinary', postings: [
      { account_id: f.cash.id, commodity_id: f.currencyID, quantity_value: '8500', quantity_scale: 2 },
      { account_id: f.income.id, commodity_id: f.currencyID, quantity_value: '-8500', quantity_scale: 2 }
    ] }]
  });
  // Frozen pre-#242 source linkage: the payment itself uses the real API.
  // Only the disposable e2e database gets an old ordinary-cash import marker;
  // no production endpoint is added to manufacture legacy financial records.
  const databasePath = fileURLToPath(new URL('../../backend/var/e2e.sqlite', import.meta.url));
  execFileSync('python3', ['-c', `
import sqlite3, sys
path, row_id, transaction_id, account_id = sys.argv[1:]
with sqlite3.connect(path, timeout=10) as db:
    db.execute('PRAGMA foreign_keys=ON')
    db.execute('PRAGMA synchronous=FULL')
    row = db.execute("SELECT s.book_id,s.dedupe_fingerprint,s.batch_id FROM import_staged_rows s JOIN import_batches b ON b.id=s.batch_id WHERE s.id=? AND s.commit_status='pending' AND b.source_kind='trading212' AND json_extract(s.raw_json,'$.kind')='trading212_dividend'", (row_id,)).fetchone()
    assert row is not None
    assert db.execute("SELECT transaction_kind FROM current_transaction_versions WHERE transaction_id=?", (transaction_id,)).fetchone()[0] == 'ordinary'
    identity = db.execute("INSERT INTO import_commit_identities(book_id,dedupe_fingerprint,source_kind,account_id,created_at) VALUES (?,?,'trading212',?,datetime('now'))", (row[0], row[1], account_id)).lastrowid
    db.execute('INSERT INTO import_commit_identity_effects(identity_id,effect_seq,transaction_id) VALUES (?,1,?)', (identity, transaction_id))
    db.execute("UPDATE import_staged_rows SET commit_status='committed',committed_identity_id=?,committed_transaction_id=? WHERE id=?", (identity, transaction_id, row_id))
    db.execute("UPDATE import_batches SET status='committed' WHERE id=?", (row[2],))
`, databasePath, String(row.row_id), String(legacy.id), String(f.cash.id)]);
  await page.goto(`/app/import/dividends?batch_id=${row.batch_id}`);
  await expect(page.getByRole('heading', { name: 'Repair previously imported cash payment', exact: true })).toBeVisible();
  await expect(page.getByRole('combobox', { name: 'Cash account', exact: true })).toBeDisabled();
  await page.getByRole('button', { name: 'Declare source cash as gross with no withholding', exact: true }).click();
  await expect(page.getByLabel('Dividend amount', { exact: true })).toHaveValue('85');
  await expect(page.getByLabel('Withholding amount', { exact: true })).toHaveValue('0');
  await page.getByRole('combobox', { name: 'Income account', exact: true }).selectOption(String(f.income.id));
  await page.getByLabel('Reason for repair', { exact: true }).fill('Confirm payer from statement');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await expectNoAccessibilityViolations(page, 'Historical dividend repair on a phone');
  await page.getByRole('button', { name: 'Repair dividend attribution', exact: true }).click();
  await expect(page.getByText('Dividend attribution repaired; cash payment preserved.', { exact: true })).toBeVisible();
  expect((await f.activity()).transactions.map(t => t.id)).toEqual([legacy.id]);
  const repaired = await apiJSON<{ id: number; version_id: number; note_markdown: string }>(page, 'GET', `/api/v1/transactions/${legacy.id}`);
  expect(repaired.version_id).not.toBe(legacy.version_id);
  expect(repaired.note_markdown).toBe('Keep the owner note');
});
