import { defineConfig } from '@playwright/test';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  testDir: '.',
  testMatch: 'website.spec.ts',
  workers: 1,
  retries: 0,
  timeout: 60_000,
  outputDir: 'test-results/website',
  reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:1889', browserName: 'chromium', headless: true },
  webServer: {
    command: 'pnpm website:build && pnpm website:preview',
    cwd: fileURLToPath(new URL('../', import.meta.url)),
    url: 'http://127.0.0.1:1889',
    reuseExistingServer: false,
    timeout: 30_000
  }
});
