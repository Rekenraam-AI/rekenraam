import { defineConfig } from '@playwright/test';
import process from 'node:process';

// Without E2E_BASE_URL every worker starts its own app and database
// (playwright/support/fixtures.ts), so workers run side by side. An external
// app has one book, so a run against it stays on one worker. E2E_WORKERS (or
// --workers) overrides the default; CI's 4-vCPU runner holds three Chromium +
// app pairs without starving them.
const externalApp = Boolean(process.env.E2E_BASE_URL);
const requestedWorkers = process.env.E2E_WORKERS ?? (process.env.CI ? '3' : '50%');
const workers = externalApp ? 1 : /^\d+$/.test(requestedWorkers) ? Number(requestedWorkers) : requestedWorkers;
const chromiumExecutable = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE;
// The Trading 212 provider stub (stubs/trading212.mjs), shared by every
// worker's app; see fixtures.ts for how the app is pointed at it.
const trading212StubPort = process.env.TRADING212_STUB_PORT ?? '16890';

export default defineConfig({
  testDir: './playwright',
  globalSetup: './global-setup.ts',
  workers,
  fullyParallel: false,
  // auth.spec.ts is the bootstrap journey: it needs a database with no owner
  // account, and every other spec creates one on its way in. Its own project
  // gets its own worker database (fixtures.ts), so it starts owner-less
  // whatever else runs. Against an external app there is one database, so
  // the dependency still orders bootstrap first there.
  projects: [
    { name: 'bootstrap', testMatch: /auth\.spec\.ts/ },
    {
      name: 'app',
      testIgnore: /auth\.spec\.ts/,
      dependencies: externalApp ? ['bootstrap'] : []
    }
  ],
  use: {
    // Sandboxes and CI images often ship a Chromium that is not the revision
    // this Playwright release pins, and cannot download the pinned one. Point
    // `PLAYWRIGHT_CHROMIUM_EXECUTABLE` at the browser that is present instead
    // of hand-patching the browser cache. Unset, Playwright uses its own.
    launchOptions: chromiumExecutable ? { executablePath: chromiumExecutable } : undefined
  },
  webServer: externalApp
    ? undefined
    : {
        command: `TRADING212_STUB_PORT=${trading212StubPort} node stubs/trading212.mjs`,
        url: `http://127.0.0.1:${trading212StubPort}/healthz`,
        timeout: 10_000,
        reuseExistingServer: false
      }
});
