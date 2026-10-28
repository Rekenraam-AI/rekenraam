import { defineConfig } from '@playwright/test';

// Deliberate asset refresh only; this is not part of the ledger test suite.
export default defineConfig({
  testDir: '.',
  testMatch: 'website-screenshots.spec.ts',
  workers: 1,
  retries: 0,
  timeout: 120_000,
  outputDir: 'test-results/website-screenshots',
  reporter: 'list',
  use: { browserName: 'chromium', headless: true }
});
