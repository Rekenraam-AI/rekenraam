/**
 * Prints the next unspent TOTP code for the secret in PROXY_AUDIT_STATE, for
 * the curl probes in deploy/docker/proxy-audit/run.sh. Exits 2 when the replay
 * guard's window has no unspent step yet; the caller waits and retries.
 */
import { readFileSync, writeFileSync } from 'node:fs';
import process from 'node:process';
import { currentStep, totpCode } from '../playwright/support/totp.ts';

const path = process.env.PROXY_AUDIT_STATE ?? '';
const state = JSON.parse(readFileSync(path, 'utf8')) as { secret: string; lastStep: number };
const step = Math.max(currentStep(), state.lastStep + 1);
if (step > currentStep() + 1) process.exit(2);
state.lastStep = step;
writeFileSync(path, JSON.stringify(state), { mode: 0o600 });
console.log(totpCode(state.secret, step));
