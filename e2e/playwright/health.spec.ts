import { expect, test } from './support/fixtures';

test('shows backend health status', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Rekenraam' })).toBeVisible();
  await expect(page.getByText('Backend status: ok')).toBeVisible();
});
