import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { constants } from 'node:fs';
import { access, mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test as base, expect, referenceDateServerPath, repoRoot } from './fixtures';

// The fixed clock exists only in a Go test executable; the production binary
// keeps its real clock. Each case gets a real HTTP server and isolated SQLite
// file, serving the same built frontend with the normal API/auth middleware.
export const test = base.extend<{ referenceInstant: string; fixedReferenceURL: string }>({
  referenceInstant: ['2027-01-01T23:01:00Z', { option: true }],
  fixedReferenceURL: [async ({ referenceInstant }, use) => {
    await access(referenceDateServerPath, constants.X_OK);
    const directory = await mkdtemp(path.join(tmpdir(), 'rekenraam-reference-date-'));
    const addressFile = path.join(directory, 'address');
    const server = spawn(referenceDateServerPath, [
      '-test.run=^TestOwnerReferenceBrowserServer$', '-test.timeout=3m'
    ], {
      cwd: repoRoot,
      detached: true,
      env: {
        ...process.env,
        TMPDIR: directory,
        REKENRAAM_REFERENCE_BROWSER_ADDRESS_FILE: addressFile,
        REKENRAAM_REFERENCE_BROWSER_INSTANT: referenceInstant
      },
      stdio: ['ignore', 'pipe', 'pipe']
    });
    const closed = once(server, 'close');
    let output = '';
    server.stdout?.on('data', chunk => { output += chunk; });
    server.stderr?.on('data', chunk => { output += chunk; });
    try {
      let url = '';
      await expect.poll(async () => {
        if (server.exitCode !== null || server.signalCode !== null) throw new Error(`fixed-date server exited: ${output}`);
        try { url = await readFile(addressFile, 'utf8'); } catch { return false; }
        try { return (await fetch(`${url}/healthz`)).ok; } catch { return false; }
      }, { timeout: 45_000 }).toBe(true);
      await use(url);
    } catch (error) {
      throw new Error(`fixed-date server failed: ${output}`, { cause: error });
    } finally {
      if (server.pid && server.exitCode === null && server.signalCode === null) process.kill(-server.pid, 'SIGTERM');
      await closed;
      await rm(directory, { recursive: true, force: true });
    }
  }, { timeout: 60_000 }],
  baseURL: async ({ fixedReferenceURL }, use) => { await use(fixedReferenceURL); }
});
export { expect };
