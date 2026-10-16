import { execSync } from 'node:child_process';
import { rmSync } from 'node:fs';
import process from 'node:process';
import { repoRoot, workerStateRoot } from './playwright/support/fixtures';

/**
 * Builds the single binary once for every worker and clears the previous
 * run's worker databases, so each self-managed run starts from empty books.
 * Set E2E_SKIP_BUILD=1 to reuse an existing dist/rekenraam while iterating.
 */
export default function globalSetup(): void {
  if (process.env.E2E_BASE_URL) return;
  rmSync(workerStateRoot, { recursive: true, force: true });
  if (process.env.E2E_SKIP_BUILD === '1') return;
  execSync('pnpm build', {
    cwd: repoRoot,
    stdio: 'inherit',
    env: { ...process.env, GOCACHE: process.env.GOCACHE ?? '/tmp/rekenraam-go-build-cache' }
  });
}
