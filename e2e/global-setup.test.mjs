import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';

const require = createRequire(import.meta.url);

function setupControl(options = {}) {
  const directory = mkdtempSync(path.join(os.tmpdir(), 'rekenraam-build-env-'));
  try {
    // Exercise actual setup and child-process boundaries with isolated paths
    // and build sentinels. The control executes the prepared server artifact.
    copyFileSync(new URL('./global-setup.ts', import.meta.url), path.join(directory, 'global-setup.ts'));
    mkdirSync(path.join(directory, 'playwright/support'), { recursive: true });
    mkdirSync(path.join(directory, 'bin'));
    mkdirSync(path.join(directory, 'scripts'));
    const binary = path.join(directory, 'workers/reference-date-server.test');
    writeFileSync(path.join(directory, 'playwright/support/fixtures.ts'), `
      export const repoRoot = ${JSON.stringify(directory)};
      export const workerStateRoot = ${JSON.stringify(path.join(directory, 'workers'))};
      export const referenceDateServerPath = ${JSON.stringify(binary)};
    `);
    writeFileSync(path.join(directory, 'timing-path.ts'), `export const timingDirectory = ${JSON.stringify(path.join(directory, 'timings'))};`);
    const sentinel = `#!${process.execPath}
      const fs = require('node:fs');
      const args = process.argv.slice(2);
      const kind = process.argv[1].endsWith('pnpm') ? 'app' : 'reference-server';
      fs.appendFileSync(process.env.BUILD_ENV_OUTPUT, JSON.stringify({
        kind, GOCACHE: process.env.GOCACHE ?? null, GOMODCACHE: process.env.GOMODCACHE ?? null,
        args, cwd: process.cwd()
      }) + '\\n');
      if (kind === 'reference-server') {
        if (process.env.FAIL_REFERENCE_BUILD) process.exit(23);
        fs.writeFileSync(args[args.indexOf('-o') + 1], '#!${process.execPath}\\nconsole.log("prepared reference server");\\n', { mode: 0o755 });
      }
    `;
    writeFileSync(path.join(directory, 'bin/pnpm'), sentinel, { mode: 0o755 });
    writeFileSync(path.join(directory, 'scripts/go-backend.sh'), sentinel, { mode: 0o755 });
    const verifyBinary = options.suite !== 'audits' && !options.failReferenceBuild;
    writeFileSync(path.join(directory, 'control.spec.ts'), `
      import { test, expect } from ${JSON.stringify(require.resolve('@playwright/test'))};
      import { execFileSync } from 'node:child_process';
      test('setup completed', () => {
        ${verifyBinary ? `expect(execFileSync(${JSON.stringify(binary)}, { encoding: 'utf8' }).trim()).toBe('prepared reference server');` : ''}
      });
    `);
    writeFileSync(path.join(directory, 'playwright.config.ts'), "export default { testDir: '.', globalSetup: './global-setup.ts', reporter: 'dot' };");
    const output = path.join(directory, 'children.ndjson');
    const env = { ...process.env, PATH: `${directory}/bin:${process.env.PATH}`, BUILD_ENV_OUTPUT: output };
    for (const key of ['NODE_TEST_CONTEXT', 'E2E_BASE_URL', 'E2E_SKIP_BUILD', 'E2E_SUITE', 'GOCACHE', 'GOMODCACHE', 'FAIL_REFERENCE_BUILD']) delete env[key];
    if (options.cache) { env.GOCACHE = options.cache; env.GOMODCACHE = `${options.cache}/modules`; }
    if (options.skipBuild) env.E2E_SKIP_BUILD = '1';
    if (options.externalApp) env.E2E_BASE_URL = 'http://127.0.0.1:16888';
    if (options.suite) env.E2E_SUITE = options.suite;
    if (options.failReferenceBuild) env.FAIL_REFERENCE_BUILD = '1';
    const run = spawnSync(process.execPath, [require.resolve('@playwright/test/cli'), 'test', '--config', path.join(directory, 'playwright.config.ts')], {
      cwd: directory, env, encoding: 'utf8', timeout: 30_000
    });
    assert.ok(existsSync(output), `setup produced no build records: ${run.error?.message ?? ''}\n${run.stderr}\n${run.stdout}`);
    const children = readFileSync(output, 'utf8').trim().split('\n').map(JSON.parse);
    const timing = JSON.parse(readFileSync(path.join(directory, 'timings/global-setup.json'), 'utf8'));
    return { run, children, directory, binary, timing };
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

for (const cache of [undefined, path.join(os.tmpdir(), 'restored Go cache')]) {
  test(`E2E prepares the app and test server before cases with ${cache ? 'restored' : 'default'} Go paths`, () => {
    const { run, children, directory, binary } = setupControl({ cache });
    assert.equal(run.status, 0, run.stderr || run.stdout);
    assert.deepEqual(children, [
      { kind: 'app', GOCACHE: cache ?? null, GOMODCACHE: cache ? `${cache}/modules` : null, args: ['build'], cwd: directory },
      { kind: 'reference-server', GOCACHE: cache ?? null, GOMODCACHE: cache ? `${cache}/modules` : null, args: ['test', '-c', '-o', binary, './internal/api'], cwd: directory }
    ]);
  });
}

test('reusing the app binary still prepares the reference server before cases', () => {
  const { run, children, timing } = setupControl({ skipBuild: true });
  assert.equal(run.status, 0, run.stderr || run.stdout);
  assert.deepEqual(children.map(child => child.kind), ['reference-server']);
  assert.equal(timing.reused_binary, true);
});

test('audit-only setup does not compile an unused reference server', () => {
  const { run, children } = setupControl({ suite: 'audits' });
  assert.equal(run.status, 0, run.stderr || run.stdout);
  assert.deepEqual(children.map(child => child.kind), ['app']);
});

test('external app setup still prepares the isolated reference server', () => {
  const { run, children, timing } = setupControl({ externalApp: true });
  assert.equal(run.status, 0, run.stderr || run.stdout);
  assert.deepEqual(children.map(child => child.kind), ['reference-server']);
  assert.equal(timing.external_app, true);
});

test('reference compilation failure stops setup and retains its build timing', () => {
  const { run, children, timing } = setupControl({ failReferenceBuild: true });
  assert.notEqual(run.status, 0);
  assert.deepEqual(children.map(child => child.kind), ['app', 'reference-server']);
  assert.ok(timing.build_ms >= 0);
  assert.ok(timing.reference_server_build_ms > 0);
  assert.ok(timing.build_ms >= timing.reference_server_build_ms);
  assert.ok(timing.duration_ms >= timing.build_ms);
});
