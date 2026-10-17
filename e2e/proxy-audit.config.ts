import { defineConfig } from '@playwright/test';
import process from 'node:process';

/**
 * Browser evidence for the #268 reverse-proxy audit, run only by hand against
 * the disposable deployment in deploy/docker/proxy-audit (see its README).
 * It is not part of the main suite: it needs that deployment, a fresh volume
 * and the operator's setup token, and its phases are ordered around restarts.
 */
const host = 'rekenraam.test';
const port = process.env.PROXY_AUDIT_PORT ?? '8443';
const chromiumExecutable = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE;

export default defineConfig({
  testDir: './proxy-audit',
  workers: 1,
  fullyParallel: false,
  use: {
    baseURL: `https://${host}:${port}`,
    // Caddy's local CA is not in the browser's store; curl verifies the chain
    // against it explicitly (README). The browser still sees an https origin.
    ignoreHTTPSErrors: true,
    // Full Chromium, not the headless shell: the shell never restores a page
    // from the back/forward cache, so it cannot show what that cache keeps.
    channel: chromiumExecutable ? undefined : 'chromium',
    launchOptions: {
      executablePath: chromiumExecutable,
      // Playwright turns the back/forward cache off by default; the audit
      // needs the browser's real restore behavior.
      ignoreDefaultArgs: ['--disable-back-forward-cache'],
      args: [`--host-resolver-rules=MAP ${host} 127.0.0.1`]
    }
  }
});
