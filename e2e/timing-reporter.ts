import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { appendFileSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import type { FullConfig, FullResult, Reporter, Suite, TestCase, TestResult, TestStep } from '@playwright/test/reporter';
import { timingDirectory } from './timing-path';

/** Timings only: no request/response bodies, assertion values or browser logs. */
export default class TimingReporter implements Reporter {
  private config!: FullConfig;
  private suite!: Suite;
  private attempts: unknown[] = [];
  private globalErrors = 0;

  printsToStdio() { return false; }

  constructor() {
    mkdirSync(timingDirectory, { recursive: true });
    for (const name of ['summary.json', 'global-setup.json']) rmSync(path.join(timingDirectory, name), { force: true });
    writeFileSync(path.join(timingDirectory, 'attempts.ndjson'), '');
  }
  onError() { this.globalErrors++; }

  onBegin(config: FullConfig, suite: Suite) {
    this.config = config;
    this.suite = suite;
    mkdirSync(timingDirectory, { recursive: true });
    writeFileSync(path.join(timingDirectory, 'attempts.ndjson'), '');
  }

  onTestEnd(test: TestCase, result: TestResult) {
    const steps: unknown[] = [];
    const api: Record<string, { calls: number; duration_ms: number }> = {};
    function visit(step: TestStep, parent?: string) {
      if (['test.step', 'hook', 'fixture'].includes(step.category)) {
        steps.push({ title: step.title, parent, category: step.category,
          start: step.startTime.toISOString(), duration_ms: step.duration,
          failed: Boolean(step.error) });
      }
      if (step.category === 'pw:api') {
        // Discard locator arguments/values; retain only the operation name.
        const operation = step.title.split(/["']/)[0].trim();
        const entry = api[operation] ??= { calls: 0, duration_ms: 0 };
        entry.calls++;
        entry.duration_ms += step.duration;
      }
      for (const child of step.steps) visit(child, step.category === 'test.step' ? step.title : parent);
    }
    for (const step of result.steps) visit(step);
    const attempt = { id: test.id, file: path.relative(this.config.rootDir, test.location.file),
      title: test.titlePath().slice(1), project: test.parent.project()?.name,
      expected_status: test.expectedStatus, status: result.status, retry: result.retry,
      worker: result.workerIndex, parallel_index: result.parallelIndex,
      start: result.startTime.toISOString(), duration_ms: result.duration, steps, api };
    this.attempts.push(attempt);
    appendFileSync(path.join(timingDirectory, 'attempts.ndjson'), JSON.stringify(attempt) + '\n');
  }

  onEnd(result: FullResult) {
    mkdirSync(timingDirectory, { recursive: true });
    let setup: unknown = null;
    try { setup = JSON.parse(readFileSync(path.join(timingDirectory, 'global-setup.json'), 'utf8')); } catch { /* Interrupted setup. */ }
    let source: string | null = null;
    try { source = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: this.config.rootDir, encoding: 'utf8' }).trim(); } catch { /* Source archive. */ }
    let binary: string | null = null;
    try { binary = createHash('sha256').update(readFileSync(new URL('../dist/rekenraam', import.meta.url))).digest('hex'); } catch { /* External app. */ }
    writeFileSync(path.join(timingDirectory, 'summary.json'), JSON.stringify({
      schema: 1, source, binary_sha256: binary, status: result.status,
      start: result.startTime.toISOString(), duration_ms: result.duration,
      workers: this.config.workers, ci: Boolean(process.env.CI),
      runner: { platform: os.platform(), arch: os.arch(), node: process.version,
        cpus: os.cpus().length, available_parallelism: os.availableParallelism(),
        cpu_model: os.cpus()[0]?.model, memory_bytes: os.totalmem() },
      setup, global_errors: this.globalErrors, selected: (this.suite?.allTests() ?? []).map(test => ({ id: test.id,
        file: path.relative(this.config.rootDir, test.location.file),
        title: test.titlePath().slice(1), project: test.parent.project()?.name })),
      attempts: this.attempts
    }, null, 2) + '\n');
  }
}
