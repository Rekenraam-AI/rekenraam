import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

const require = createRequire(import.meta.url);

test('E2E build inherits the restored Go paths and leaves Go defaults intact when unset', () => {
  const directory = mkdtempSync(path.join(os.tmpdir(), 'rekenraam-build-env-'));
  try {
    // Exercise the actual setup and its child-process boundary, with isolated
    // worker/timing paths and a pnpm sentinel instead of an expensive build.
    copyFileSync(new URL('./global-setup.ts', import.meta.url), path.join(directory, 'global-setup.ts'));
    mkdirSync(path.join(directory, 'playwright/support'), { recursive: true });
    mkdirSync(path.join(directory, 'bin'));
    writeFileSync(path.join(directory, 'playwright/support/fixtures.ts'), `
      export const repoRoot = ${JSON.stringify(directory)};
      export const workerStateRoot = ${JSON.stringify(path.join(directory, 'workers'))};
    `);
    writeFileSync(path.join(directory, 'timing-path.ts'), `export const timingDirectory = ${JSON.stringify(path.join(directory, 'timings'))};`);
    writeFileSync(path.join(directory, 'bin/pnpm'), `#!${process.execPath}
      require('node:fs').writeFileSync(process.env.BUILD_ENV_OUTPUT, JSON.stringify({
        GOCACHE: process.env.GOCACHE ?? null, GOMODCACHE: process.env.GOMODCACHE ?? null,
        args: process.argv.slice(2), cwd: process.cwd()
      }));
    `, { mode: 0o755 });
    writeFileSync(path.join(directory, 'control.spec.ts'), `import { test } from ${JSON.stringify(require.resolve('@playwright/test'))}; test('setup completed', () => {});`);
    writeFileSync(path.join(directory, 'playwright.config.ts'), "export default { testDir: '.', globalSetup: './global-setup.ts', reporter: 'dot' };");
    for (const cache of [undefined, path.join(directory, 'restored Go cache')]) {
      const output = path.join(directory, 'child.json');
      const env = { ...process.env, PATH: `${directory}/bin:${process.env.PATH}`, BUILD_ENV_OUTPUT: output };
      for (const key of ['NODE_TEST_CONTEXT', 'E2E_BASE_URL', 'E2E_SKIP_BUILD', 'GOCACHE', 'GOMODCACHE']) delete env[key];
      if (cache) { env.GOCACHE = cache; env.GOMODCACHE = `${directory}/modules`; }
      const run = spawnSync(process.execPath, [require.resolve('@playwright/test/cli'), 'test', '--config', path.join(directory, 'playwright.config.ts')], {
        cwd: directory, env, encoding: 'utf8', timeout: 30_000
      });
      assert.equal(run.status, 0, run.stderr || run.stdout);
      assert.deepEqual(JSON.parse(readFileSync(output, 'utf8')), {
        GOCACHE: cache ?? null, GOMODCACHE: cache ? `${directory}/modules` : null,
        args: ['build'], cwd: directory
      });
    }
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
