import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Outside Playwright's outputDir: the runner clears test-results after global
// setup, which would otherwise discard the build timing before the reporter.
export const timingDirectory = path.resolve(process.env.E2E_TIMING_DIR ??
  fileURLToPath(new URL('./timing-results', import.meta.url)));
