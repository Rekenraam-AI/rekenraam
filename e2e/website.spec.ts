import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

const locales = ['en', 'nl', 'de', 'fr', 'es', 'ru'];

for (const locale of locales) {
  test(`${locale}: localized content, responsive layouts and accessible themes`, async ({ page }) => {
    const failures: string[] = [];
    const origins = new Set<string>();
    page.on('pageerror', error => failures.push(error.message));
    page.on('request', request => origins.add(new URL(request.url()).origin));
    await page.goto(locale === 'en' ? '/' : `/${locale}/`);
    await expect(page.locator('html')).toHaveAttribute('lang', locale);
    await expect(page.locator('h1')).toHaveCount(1);
    await expect(page.locator('.development strong')).not.toBeEmpty();
    await expect(page.locator('#features article')).toHaveCount(3);
    await expect(page.locator('a[href="https://retirement.rekenraam.com"]')).toBeVisible();
    const images = page.locator('.screenshot img');
    await expect(images).toHaveCount(3);
    for (const image of await images.all()) {
      await image.scrollIntoViewIfNeeded();
      await expect.poll(() => image.evaluate(element => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
    }
    for (const width of [320, 390, 768, 1280]) {
      await page.setViewportSize({ width, height: 900 });
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), `Overflow at ${width}px`).toBeTruthy();
    }
    for (const colorScheme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme });
      const audit = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
      expect(audit.violations, `${locale} ${colorScheme}`).toEqual([]);
    }
    await page.locator('[data-theme-toggle]').click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    expect(failures).toEqual([]);
    expect([...origins]).toEqual(['http://127.0.0.1:1889']);
  });
}

test('language switching and content work without JavaScript', async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, baseURL: 'http://127.0.0.1:1889' });
  try {
    const page = await context.newPage();
    await page.goto('/');
    for (const locale of ['ru', 'de', 'fr', 'es', 'nl', 'en']) {
      await page.locator('.language-picker summary').click();
      await page.locator(`.language-picker a[hreflang="${locale}"]`).click();
      await expect(page.locator('html')).toHaveAttribute('lang', locale);
      await expect(page.locator('#status')).toBeVisible();
      await expect(page.locator('[data-theme-toggle]')).toBeHidden();
    }
    const response = await page.goto('/missing-page/');
    expect(response?.status()).toBe(404);
    await expect(page.locator('h1')).toHaveText('Page not found');
    await page.locator('.not-found .button').click();
    await expect(page).toHaveURL('http://127.0.0.1:1889/');
  } finally {
    await context.close();
  }
});
