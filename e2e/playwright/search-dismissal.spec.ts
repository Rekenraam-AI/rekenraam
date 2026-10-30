import { expect, test } from './support/fixtures';
import { ensureBrowserSession } from './support/session';

test('Escape from a search control closes results and restores the input without reopening', async ({ page }) => {
  await ensureBrowserSession(page);
  await page.goto('/app');
  const input = page.getByRole('combobox', { name: 'Search', exact: true });
  await input.fill('Home');
  await page.getByRole('button', { name: 'All', exact: true }).focus();
  await page.keyboard.press('Escape');
  await expect(input).toHaveAttribute('aria-expanded', 'false');
  await expect(input).toBeFocused();
  await expect(page.getByRole('listbox', { name: 'Search results', exact: true })).toHaveCount(0);
});
