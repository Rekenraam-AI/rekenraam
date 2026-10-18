import { readFile } from 'node:fs/promises';
import { test, expect } from './support/fixtures';
import { apiJSON } from './support/api';
import { seedReferenceBook, type SeededReferenceBook, type ReferenceBook } from './support/reference-book';
import type { components } from '../../frontend/src/lib/api/schema';

type Gains = components['schemas']['InvestmentGainsResponse'];
type Method = ReferenceBook['register']['gains']['methods'][number];

test('[acceptance] reference gains match the register in live comparisons and the saved JSON download', async ({ page }) => {
  test.setTimeout(120_000);
  const seeded = await seedReferenceBook(page);
  const { book, ids } = seeded;
  const register = book.register.gains;
  await page.goto('/app/investments');
  await page.getByRole('button', { name: 'Gains', exact: true }).click();
  await page.getByLabel('From', { exact: true }).fill(book.register.period.start_date);
  await page.getByLabel('To', { exact: true }).fill(book.register.period.end_date);
  await page.getByRole('button', { name: 'Apply', exact: true }).click();
  await page.getByLabel('Valuation date', { exact: true }).fill(register.valuation_as_of);
  const revalued = page.waitForResponse((res) => new URL(res.url()).pathname === '/api/v1/investments/gains' && new URL(res.url()).searchParams.get('valuation_as_of') === register.valuation_as_of);
  await page.getByRole('button', { name: 'Revalue', exact: true }).click();
  const live = await (await revalued).json() as Gains;
  assertRegister(live, register.methods[0], seeded);
  const comparison = page.getByTestId('alternative-gains');
  for (const method of register.methods.slice(1)) {
    const response = page.waitForResponse((res) => new URL(res.url()).pathname === '/api/v1/investments/gains' && new URL(res.url()).searchParams.get('alternative_method') === method.method);
    await comparison.getByLabel('Compare basis method').selectOption(method.method);
    const result = await (await response).json() as Gains;
    expect(result.alternative).toBeDefined();
    assertRegister(result.alternative!, method, seeded);
    for (const position of method.unrealized) {
      const row = comparison.getByRole('row').filter({ hasText: `Account #${ids.get(position.account)}` });
      await expect(row).toHaveCount(1);
      const value = row.getByRole('cell').last();
      await expect(value).toHaveText(position.gain === null ? 'Unavailable: missing price or unknown basis' : `USD ${position.gain}`);
    }
  }
  // Saving explicitly computes an original-currency operational document, even
  // when the last displayed comparison is average cost. Its download must not
  // silently persist that comparison as the operational result.
  const savedResponse = page.waitForResponse((res) => new URL(res.url()).pathname === '/api/v1/investments/gains/runs' && res.request().method() === 'POST');
  await page.getByRole('button', { name: 'Save gains report', exact: true }).click();
  const run = await (await savedResponse).json();
  expect(run.output.profile.name).toBe('operational_v1');
  assertRegister(run.output, register.methods[0], seeded);
  const saved = page.getByTestId('saved-gains-run');
  await expect(saved).toContainText(`Saved report #${run.id}`);
  const downloadEvent = page.waitForEvent('download');
  await saved.getByRole('link', { name: 'Download saved report (JSON)' }).click();
  const bytes = await readFile((await (await downloadEvent).path())!, 'utf8');
  const serverBytes = await page.evaluate(async (id) => (await fetch(`/api/v1/investments/gains/runs/${id}?download=true`)).text(), run.id);
  expect(bytes).toBe(serverBytes);
  const downloaded = JSON.parse(bytes);
  expect(downloaded).toEqual(run);
  assertRegister(downloaded.output, register.methods[0], seeded);
  expect(await apiJSON(page, 'GET', `/api/v1/investments/gains/runs/${run.id}`)).toEqual(run);
});

function amount(value: string | null | undefined, scale: number | null | undefined, expected: string | null): void {
  if (expected === null) {
    expect(value == null).toBe(true);
    return;
  }
  expect(value).not.toBeNull();
  expect(scale).not.toBeNull();
  const places = expected.split('.')[1]?.length ?? 0;
  expect(BigInt(value!) * 10n ** BigInt(places)).toBe(BigInt(expected.replace('.', '')) * 10n ** BigInt(scale!));
}

function assertRegister(gains: Pick<Gains, 'realized' | 'unrealized' | 'realized_totals'>, method: Method, seeded: SeededReferenceBook): void {
  const { ids } = seeded;
  const accounts = new Set(method.unrealized.map((row) => ids.get(row.account)));
  const realized = gains.realized.filter((row) => accounts.has(row.account_id));
  const unrealized = gains.unrealized.filter((row) => accounts.has(row.account_id));
  expect(realized).toHaveLength(method.realized.length);
  expect(unrealized).toHaveLength(method.unrealized.length);
  for (const figure of method.realized) {
    const row = realized.find((row) => row.account_id === ids.get(figure.account) && row.disposal_date === figure.disposal_date)!;
    expect(row).toBeDefined();
    amount(row.realized_gain_value, row.realized_gain_scale, figure.gain);
    if (figure.basis !== undefined) amount(row.disposed_basis_value, row.disposed_basis_scale, figure.basis);
    if (figure.proceeds !== undefined) amount(row.proceeds_value, row.proceeds_scale, figure.proceeds);
  }
  for (const figure of method.unrealized) {
    const row = unrealized.find((row) => row.account_id === ids.get(figure.account))!;
    expect(row).toBeDefined();
    amount(row.unrealized_gain_value, row.unrealized_gain_scale, figure.gain);
  }
  // This worker may contain other specs' positions. Check the reference
  // cohort's total independently; the API test checks the book-wide total on
  // an otherwise empty book. No production money helper produces this oracle.
  const scale = Math.max(...realized.map((row) => row.realized_gain_scale!));
  const total = realized.reduce((sum, row) => sum + BigInt(row.realized_gain_value!) * 10n ** BigInt(scale - row.realized_gain_scale!), 0n);
  amount(total.toString(), scale, method.realized_total);
}
