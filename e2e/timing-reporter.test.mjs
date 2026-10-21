import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const require = createRequire(import.meta.url);
const runner = require.resolve('@playwright/test/cli');
const reporter = fileURLToPath(new URL('./timing-reporter.ts', import.meta.url));

function runnerEnvironment(directory) {
  const env = { ...process.env, E2E_TIMING_DIR: path.join(directory, 'timings') };
  // The child is a Playwright process, not a Node test-runner worker.
  delete env.NODE_TEST_CONTEXT;
  return env;
}

test('timing reporter retains success, failure, skip and view steps without stdout or assertion payloads', () => {
  const directory = mkdtempSync(path.join(os.tmpdir(), 'rekenraam-timing-control-'));
  try {
    const library = require.resolve('@playwright/test');
    writeFileSync(path.join(directory, 'control.spec.ts'), `
      import { test, expect } from ${JSON.stringify(library)};
      test('positive', async () => {
        console.log('synthetic-stdout-must-not-be-retained');
        await test.step('view: positive control', async () => { expect(1).toBe(1); });
      });
      test('negative', async () => {
        await test.step('view: negative control', async () => { expect('synthetic-assertion-payload').toBe('different'); });
      });
      test.skip('unexecuted', () => {});
    `);
    writeFileSync(path.join(directory, 'playwright.config.ts'), `
      export default { testDir: '.', workers: 1, reporter: [[${JSON.stringify(reporter)}]],
        outputDir: ${JSON.stringify(path.join(directory, 'output'))} };
    `);
    const run = spawnSync(process.execPath, [runner, 'test', '--config', path.join(directory, 'playwright.config.ts')], {
      encoding: 'utf8', timeout: 30_000,
      env: runnerEnvironment(directory)
    });
    assert.equal(run.status, 1, run.stderr || run.stdout);
    const raw = readFileSync(path.join(directory, 'timings/summary.json'), 'utf8');
    const summary = JSON.parse(raw);
    assert.equal(summary.status, 'failed');
    assert.equal(summary.selected.length, 3);
    assert.deepEqual(summary.attempts.map(row => row.status).sort(), ['failed', 'passed', 'skipped']);
    assert.ok(summary.attempts.find(row => row.status === 'passed').steps.some(step => step.title === 'view: positive control'));
    assert.ok(summary.attempts.find(row => row.status === 'failed').steps.some(step => step.title === 'view: negative control' && step.failed));
    assert.ok(summary.duration_ms > 0);
    assert.equal(summary.workers, 1);
    assert.ok(summary.runner.available_parallelism > 0);
    assert.ok(!raw.includes('synthetic-stdout-must-not-be-retained'));
    assert.ok(!raw.includes('synthetic-assertion-payload'));
    const attempts = readFileSync(path.join(directory, 'timings/attempts.ndjson'), 'utf8').trim().split('\n').map(JSON.parse);
    assert.equal(attempts.length, 3);
    assert.deepEqual(attempts.map(row => row.status), summary.attempts.map(row => row.status));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test('timing reporter records collection failure without hiding the original error', () => {
  const directory = mkdtempSync(path.join(os.tmpdir(), 'rekenraam-timing-collection-'));
  try {
    mkdirSync(path.join(directory, 'timings'));
    writeFileSync(path.join(directory, 'timings/global-setup.json'), '{"stale":true}');
    writeFileSync(path.join(directory, 'timings/attempts.ndjson'), '{"stale":true}\n');
    writeFileSync(path.join(directory, 'control.spec.ts'), "throw new Error('synthetic collection failure');");
    writeFileSync(path.join(directory, 'playwright.config.ts'), `
      export default { testDir: '.', reporter: [[${JSON.stringify(reporter)}]],
        outputDir: ${JSON.stringify(path.join(directory, 'output'))} };
    `);
    const run = spawnSync(process.execPath, [runner, 'test', '--config', path.join(directory, 'playwright.config.ts')], {
      encoding: 'utf8', timeout: 30_000,
      env: runnerEnvironment(directory)
    });
    assert.equal(run.status, 1, run.stderr || run.stdout);
    const summary = JSON.parse(readFileSync(path.join(directory, 'timings/summary.json'), 'utf8'));
    assert.equal(summary.status, 'failed');
    assert.ok(summary.global_errors > 0);
    assert.deepEqual(summary.attempts, []);
    assert.equal(summary.setup, null);
    assert.equal(readFileSync(path.join(directory, 'timings/attempts.ndjson'), 'utf8'), '');
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
