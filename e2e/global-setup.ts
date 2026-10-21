import { execSync } from 'node:child_process';
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { repoRoot, workerStateRoot } from './playwright/support/fixtures';
import { timingDirectory } from './timing-path';

/**
 * Builds the single binary once for every worker and clears the previous
 * run's worker databases, so each self-managed run starts from empty books.
 * Set E2E_SKIP_BUILD=1 to reuse an existing dist/rekenraam while iterating.
 */
export default function globalSetup(): void {
  const start = performance.now();
  mkdirSync(timingDirectory, { recursive: true });
  const record = (build_ms: number) => writeFileSync(path.join(timingDirectory, 'global-setup.json'), JSON.stringify({
    duration_ms: performance.now() - start, build_ms,
    external_app: Boolean(process.env.E2E_BASE_URL), reused_binary: process.env.E2E_SKIP_BUILD === '1'
  }) + '\n');
  if (process.env.E2E_BASE_URL) { record(0); return; }
  rmSync(workerStateRoot, { recursive: true, force: true });
  if (process.env.E2E_SKIP_BUILD === '1') { record(0); return; }
  const buildStart = performance.now();
  try {
    execSync('pnpm build', {
      cwd: repoRoot,
      stdio: 'inherit',
      env: { ...process.env, GOCACHE: process.env.GOCACHE ?? '/tmp/rekenraam-go-build-cache' }
    });
  } finally {
    record(performance.now() - buildStart);
  }
}
