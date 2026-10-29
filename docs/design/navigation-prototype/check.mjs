// Focused design-artifact checks using the repository's existing browser tools.
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { resolve } from 'node:path';

const require = createRequire(new URL('../../../e2e/package.json', import.meta.url));
const { chromium, expect } = require('@playwright/test');
const AxeBuilder = require('@axe-core/playwright').default;
const start = pathToFileURL(fileURLToPath(new URL('./index.html', import.meta.url))).href;
const screenshotDir = process.argv[2];
if (screenshotDir) await mkdir(screenshotDir, { recursive: true });
const browser = await chromium.launch();
const errors = [];
const remoteRequests = [];
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => { if (/^https?:/.test(request.url())) remoteRequests.push(request.url()); });
  const go = async path => { await page.goto(`${start}#${path}`); await expect(page.locator('main h1')).toBeVisible(); };
  await go('/app');

  // Exercise actual disclosures and links rather than only visiting fixture URLs.
  await page.getByRole('button', { name: 'Ledger', exact: true }).click();
  await page.locator('#nav-Ledger').getByRole('link', { name: 'Checking' }).click();
  await expect(page.locator('main h1')).toHaveText('Checking');
  await expect(page.getByRole('navigation', { name: 'Ledger pages' }).getByRole('link', { name: 'Accounts', exact: true })).toHaveAttribute('aria-current', 'true');
  await page.locator('[name="q"]').fill('March');
  await page.getByRole('button', { name: 'Apply filters' }).click();
  await expect(page).toHaveURL(/q=March/);
  await page.getByRole('button', { name: 'Checking', exact: true }).click();
  await page.locator('#search-input').fill('Savings');
  await page.locator('#search-input').press('Enter');
  await expect(page).toHaveURL(/accounts\/2\/register\?month=2026-03&q=March/);
  await expect(page.locator('main h1')).toHaveText('Savings');

  // Entry keeps the page's account and unsaved fields while cancel is resolved.
  await page.getByRole('button', { name: 'Add transaction', exact: true }).click();
  await expect(page.locator('#entry-account')).toHaveValue('2');
  await page.locator('[name="payee"]').fill('Unsaved sample');
  await page.keyboard.press('Escape');
  await expect(page.locator('#discard-warning')).toBeVisible();
  await expect(page.locator('[name="payee"]')).toHaveValue('Unsaved sample');
  await page.getByRole('button', { name: 'Keep editing' }).click();
  await expect(page.locator('[name="payee"]')).toBeFocused();
  await expect(page.locator('[name="payee"]')).toHaveValue('Unsaved sample');
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Discard sample entry' }).click();
  await expect(page.getByRole('button', { name: 'Add transaction', exact: true })).toBeFocused();
  await page.getByRole('link', { name: 'Reconcile this account' }).click();
  await page.getByRole('button', { name: 'Preview reconciliation' }).click();
  await expect(page.getByRole('heading', { name: 'Review checkpoint' })).toBeVisible();
  await page.getByRole('button', { name: 'Finish sample reconciliation' }).click();
  await expect(page.getByText('Sample reconciliation finished. No checkpoint was written.')).toBeVisible();

  await go('/app');
  const expandReview = async () => { if (await page.locator('#home-review').getAttribute('aria-expanded') !== 'true') await page.locator('#home-review').click(); };
  await expandReview();
  await page.getByRole('link', { name: '1 import batch to review', exact: false }).click();
  await expect(page.getByRole('heading', { name: 'ING CSV · October' })).toBeVisible();
  await page.locator('#import-category').selectOption({ label: 'Groceries' });
  await page.getByRole('button', { name: 'Review sample changes' }).click();
  await expect(page.getByRole('status')).toContainText('batch remains uncommitted');

  // Browser Back restores Home expansion and scroll after a real report drill-down.
  await go('/app');
  await expandReview();
  await page.getByRole('link', { name: 'Open spending report →' }).scrollIntoViewIfNeeded();
  const homeScroll = await page.evaluate(() => scrollY);
  await page.getByRole('link', { name: 'Open spending report →' }).click();
  await page.locator('main').getByRole('link', { name: 'Groceries', exact: true }).click();
  await expect(page.locator('main h1')).toHaveText('Groceries');
  await page.goBack();
  await expect(page.locator('main h1')).toHaveText('Spending');
  await page.goBack();
  await expect(page.locator('#home-review')).toHaveAttribute('aria-expanded', 'true');
  await expect.poll(() => page.evaluate(() => scrollY)).toBe(homeScroll);

  // Legacy links retain filters and switch subviews rather than leaving stale tabs.
  await go('/app/reports?view=cashflow&from=2026-03-01&account_id=2');
  await expect(page).toHaveURL(/reports\/cashflow\?from=2026-03-01&account_id=2/);
  await go('/app/investments?tab=splits&from=2026-03-01');
  await expect(page.getByRole('status').filter({ hasText: 'Selected view: splits' })).toBeVisible();
  await go('/app/import/dividends?batch_id=october');
  await expect(page.getByRole('heading', { name: 'Dividend review' })).toBeVisible();

  // Every menu destination has a real page and a reversible history entry.
  await go('/app');
  const destinations = await page.locator('header a').evaluateAll(links => [...new Set(links.map(link => link.hash))]);
  assert(destinations.length >= 25, 'Expected the complete approved menu inventory');
  for (const destination of destinations) {
    await go(destination.slice(1));
    await expect(page.locator('main h1')).not.toHaveText('Page not in this prototype');
  }
  await go('/app');
  await page.locator('#scenario').selectOption('failed');
  await expect(page.locator('#home-review')).toHaveText('Review status unavailable');
  await expect(page.locator('main')).not.toContainText('All clear');
  await page.locator('#scenario').selectOption('missing');
  await expect(page.locator('main h1')).toContainText('Net worth not available');
  await expect(page.locator('#home-networth')).toContainText('Unavailable');
  await page.locator('#scenario').selectOption('empty');
  await expect(page.getByRole('link', { name: 'Set up accounts' })).toBeVisible();
  await page.locator('#scenario').selectOption('everyday');

  // Old settings and trash links land on their new homes; brokerage shows positions, not holding accounts.
  await go('/app/settings/trash');
  await expect(page).toHaveURL(/transactions\?view=deleted/);
  await expect(page.getByRole('heading', { name: 'Deleted transactions' })).toBeVisible();
  await go('/app/settings/appearance');
  await expect(page.locator('main h1')).toHaveText('Preferences');
  await expect(page.getByRole('button', { name: 'Customize Home…' })).toBeVisible();
  await go('/app/accounts');
  await expect(page.getByRole('heading', { name: 'DEGIRO' })).toBeVisible();
  await expect(page.locator('main')).not.toContainText('holding');
  await go('/app');

  // Home's command bar is the shared search, inline: payee actions prefill the editor without posting.
  await page.locator('#home-cmd').fill('albert');
  await expect(page.locator('#home-cmd-list').getByRole('group', { name: 'Payee · Albert Heijn' })).toBeVisible();
  await page.locator('#home-cmd').press('Enter');
  await expect(page.locator('[name="payee"]')).toHaveValue('Albert Heijn');
  await expect(page.locator('[name="category"]')).toHaveValue('Groceries');
  await page.getByRole('button', { name: 'Cancel' }).click();
  await page.locator('#home-cmd').fill('');
  await page.locator('#home-cmd').fill('savings');
  await expect(page.locator('#home-cmd-list')).toContainText('Search all transactions for “savings”');
  await page.locator('#home-cmd').press('Escape');
  await expect(page.locator('#home-cmd-panel')).toBeHidden();

  // A widget is in exactly one place; removal returns it to More to explore, with Undo.
  await page.getByRole('button', { name: 'Options for Budgets · October' }).first().click();
  await page.getByRole('button', { name: 'Remove from My Home' }).click();
  await expect(page.locator('#home-budgets')).toHaveCount(0);
  await expect(page.locator('#explore-budgets')).toBeVisible();
  await page.getByRole('button', { name: 'Undo' }).click();
  await expect(page.locator('#home-budgets')).toBeVisible();
  await expect(page.locator('#explore-budgets')).toHaveCount(0);
  await page.getByRole('button', { name: 'Add Holdings to My Home' }).click();
  await expect(page.locator('#home-holdings')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Options for Holdings' })).toBeFocused();
  await page.getByRole('button', { name: 'Customize Home' }).click();
  await page.getByRole('button', { name: 'Reset Home' }).click();
  await expect(page.locator('#home-holdings')).toHaveCount(0);
  await page.getByRole('button', { name: 'Done' }).click();

  for (const width of [1440, 390, 320]) {
    await page.setViewportSize({ width, height: width === 1440 ? 1000 : 844 });
    for (const theme of ['light', 'dark']) {
      await page.evaluate(theme => { document.documentElement.dataset.theme = theme; }, theme);
      for (const path of ['/app', '/app/settings/preferences', '/app/accounts?view=institutions', '/app/accounts/1/register', '/app/reports/spending', '/app/import?batch_id=october', '/app/reconcile?account_id=1']) {
        await go(path);
        // A reload restores the prototype's chosen theme, so apply the test theme after go.
        await page.evaluate(theme => { document.documentElement.dataset.theme = theme; }, theme);
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `Page overflow: ${width} ${path}`);
        const audit = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
        assert.deepEqual(audit.violations.map(v => ({ id: v.id, nodes: v.nodes.map(n => n.target) })), [], `Accessibility: ${width} ${theme} ${path}`);
      }
    }
    await go('/app/accounts/1/register?month=2026-03');
    if (width < 900) {
      await page.getByRole('button', { name: 'All pages', exact: true }).click();
      await page.locator('#mobile-pages').getByText('Ledger', { exact: true }).click();
      await page.locator('#mobile-pages').getByRole('link', { name: 'Savings' }).click();
      await expect(page.locator('main h1')).toHaveText('Savings');
    }
    await page.locator('main h1 button').click();
    await expect(page.locator('#search-input')).toBeFocused();
    if (width < 600) assert.equal(await page.locator('#search-dialog').evaluate(d => d.getBoundingClientRect().width), width);
    const dialogAudit = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
    assert.deepEqual(dialogAudit.violations.map(v => v.id), [], `Open switcher: ${width}`);
    await page.keyboard.press('Escape');
    await expect(page.locator('main h1 button')).toBeFocused();
    await page.getByRole('button', { name: 'Hide amounts', exact: true }).click();
    await expect(page.locator('main .money').first()).toHaveText('••••••');
    await page.locator('main h1 button').click();
    await expect(page.getByRole('listbox')).not.toContainText('€');
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Hide amounts', exact: true }).click();
    if (screenshotDir && width !== 320) {
      await go('/app');
      await page.screenshot({ path: resolve(screenshotDir, `home-${width}.png`), fullPage: true });
      await go('/app/accounts/1/register?month=2026-03');
      await page.screenshot({ path: resolve(screenshotDir, `register-${width}.png`), fullPage: true });
    }
  }
  // Options under review in the prototype bar stay accessible and keep the page within the viewport.
  await page.setViewportSize({ width: 1440, height: 1000 });
  await go('/app/accounts/1/register');
  for (const [id, value] of [['#opt-sections', 'links'], ['#opt-caps', 'caps'], ['#opt-rail', 'on']]) await page.locator(id).selectOption(value);
  await expect(page.getByRole('navigation', { name: 'Accounts' }).getByRole('link', { name: /Checking/ })).toHaveAttribute('aria-current', 'page');
  await page.getByRole('navigation', { name: 'Sections' }).getByRole('link', { name: 'Investments' }).click();
  // A section link reopens that section's last-visited page.
  await expect(page.getByRole('navigation', { name: 'Investments pages' }).locator('[aria-current="page"]')).toHaveCount(1);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const path of ['/app/accounts/1/register', '/app/investments/income']) {
      await go(path);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `Option overflow: ${width} ${path}`);
      const audit = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
      assert.deepEqual(audit.violations.map(v => v.id), [], `Options accessibility: ${width} ${path}`);
    }
  }
  if (screenshotDir) { await page.setViewportSize({ width: 1440, height: 900 }); await go('/app/accounts/1/register'); await page.screenshot({ path: resolve(screenshotDir, 'options-register-1440.png') }); }
  for (const [id, value] of [['#opt-sections', 'menus'], ['#opt-caps', 'sentence'], ['#opt-rail', 'off']]) await page.locator(id).selectOption(value);
  assert.deepEqual(errors, [], 'Browser errors');
  assert.deepEqual(remoteRequests, [], 'Prototype must not contact a remote service');
  console.log(`Passed: ${destinations.length} destinations; entry, scoped switching, review, reconciliation, legacy links and Back; 42 page accessibility/reflow checks; Home command bar, payee prefill, widget moves with Undo and Customize; 3 open switchers; header options (links, capitals, account rail); masking; zero remote requests.`);
} finally {
  await browser.close();
}
