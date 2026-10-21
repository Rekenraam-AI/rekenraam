import { defineConfig } from '@playwright/test';

// Harness controls use synthetic pages; no app, book or provider is involved.
export default defineConfig({
  testDir: './harness',
  outputDir: './test-results/harness',
  workers: 1,
  reporter: 'list',
  use: {
    launchOptions: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE
      ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE } : undefined
  }
});
