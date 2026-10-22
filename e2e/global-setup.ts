import { execFileSync, execSync } from 'node:child_process';
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { referenceDateServerPath, repoRoot, workerStateRoot } from './playwright/support/fixtures';
import { timingDirectory } from './timing-path';

/**
 * Prepare executable artifacts before any browser fixture's startup deadline.
 * E2E_SKIP_BUILD reuses the app; the test-only reference server still compiles
 * against the current source and embedded frontend. Audit-only runs omit it.
 */
export default function globalSetup(): void {
  const start = performance.now();
  mkdirSync(timingDirectory, { recursive: true });
  let referenceServerBuildMS = 0;
  const buildStart = performance.now();
  try {
    if (!process.env.E2E_BASE_URL) {
      rmSync(workerStateRoot, { recursive: true, force: true });
      if (process.env.E2E_SKIP_BUILD !== '1') {
        execSync('pnpm build', { cwd: repoRoot, stdio: 'inherit', env: process.env });
      }
    }
    if (process.env.E2E_SUITE !== 'audits') {
      mkdirSync(workerStateRoot, { recursive: true });
      const referenceStart = performance.now();
      try {
        execFileSync(path.join(repoRoot, 'scripts/go-backend.sh'), [
          'test', '-c', '-o', referenceDateServerPath, './internal/api'
        ], { cwd: repoRoot, stdio: 'inherit', env: process.env });
      } finally {
        referenceServerBuildMS = performance.now() - referenceStart;
      }
    }
  } finally {
    writeFileSync(path.join(timingDirectory, 'global-setup.json'), JSON.stringify({
      duration_ms: performance.now() - start, build_ms: performance.now() - buildStart,
      reference_server_build_ms: referenceServerBuildMS,
      external_app: Boolean(process.env.E2E_BASE_URL), reused_binary: process.env.E2E_SKIP_BUILD === '1'
    }) + '\n');
  }
}
