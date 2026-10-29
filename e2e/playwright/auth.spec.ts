import { expect, test } from './support/fixtures';
import { completeCurrencySetupIfNeeded, signOut } from './support/session';

test('bootstraps the owner account, signs out, and signs back in', async ({ page }) => {
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'Create the owner account' })).toBeVisible();

  await page.getByLabel('Username').fill('owner');
  await page.getByLabel('Password').fill('test-password');
  await page.getByRole('button', { name: 'Create owner' }).click();

  await completeCurrencySetupIfNeeded(page);

  await expect(page).toHaveURL(/\/app$/);
  await expect(page.getByRole('heading', { name: 'Home', exact: true })).toBeVisible();
  await signOut(page);

  await expect(page).toHaveURL('/');
  await expect(page.getByRole('heading', { name: 'Sign in to continue' })).toBeVisible();

  await page.getByLabel('Username').fill('owner');
  await page.getByLabel('Password').fill('test-password');
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page).toHaveURL(/\/app$/);
  await expect(page.getByRole('heading', { name: 'Home', exact: true })).toBeVisible();
});

test('shows translated feedback for wrong owner password', async ({ page }) => {
  await ensureOwnerExistsAndReachLogin(page);

  await page.getByLabel('Username').fill('owner');
  await page.getByLabel('Password').fill('wrong-password');
  await page.getByRole('button', { name: 'Sign in' }).click();

  await expect(page).toHaveURL('/');
  await expect(page.getByRole('heading', { name: 'Sign in to continue' })).toBeVisible();
  await expect(page.getByRole('alert')).toContainText("We couldn't complete that action.");
  await expect(page.getByRole('alert')).toContainText('Please sign in to continue.');
});

async function ensureOwnerExistsAndReachLogin(page: Parameters<typeof test>[0]['page']) {
  await page.goto('/');

  const freshHeading = page.getByRole('heading', { name: 'Create the owner account' });
  const loginHeading = page.getByRole('heading', { name: 'Sign in to continue' });
  const settingsMenu = page.getByRole('button', { name: 'Settings', exact: true });

  await expect(freshHeading.or(loginHeading).or(settingsMenu)).toBeVisible({ timeout: 5000 });

  if (await freshHeading.isVisible()) {
    await page.getByLabel('Username').fill('owner');
    await page.getByLabel('Password').fill('test-password');
    await page.getByRole('button', { name: 'Create owner' }).click();
    await completeCurrencySetupIfNeeded(page);
    await expect(page).toHaveURL(/\/app$/);
    await signOut(page);
  } else if (await settingsMenu.isVisible()) {
    await signOut(page);
  }

  await expect(page).toHaveURL('/');
  await expect(loginHeading).toBeVisible();
}
