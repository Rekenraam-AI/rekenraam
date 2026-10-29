# Browser MFA enrollment fixtures (#384)

The #381 baseline failure submitted a previous-step TOTP code just after the
real server crossed a 30-second boundary. That code was correctly two steps
old at admission. This repair changes browser fixtures only, following the
backend fixture discipline from #376; it does not change production clocks,
the accepted ±1 window, replay protection, timeouts or journey assertions.

## Caller survey and repair

- `catalog-localization.spec.ts` signs in with a recovery code in Spanish,
  French, Dutch, German and Russian at 390px. It now activates with the current
  TOTP step, since it needs no unused step for the subsequent sign-in.
- `mfa.spec.ts` needs an unused TOTP step for its next sign-in. The shared
  `support/mfa-enrollment.ts` helper submits the previous step and observes the
  real activation response. Only a 401 with the exact
  `UNAUTHENTICATED` / `invalid authentication code` envelope and a newly
  observed advancing step permits a refresh. It permits at most three total
  submissions and returns the step that successfully activated MFA.
- The whole-tree survey found a third browser enrollment caller in
  `proxy-audit/proxy-deployment.spec.ts`, with the same previous-step pattern
  and a later TOTP sign-in. It now uses the same helper and records the
  successfully consumed step in its existing state file.

All callers retain their existing recovery-code, sign-in and cleanup
assertions. The main journey retains its single-use recovery-code refusal.
The helper submits through each caller's existing input and button, so locale
labels, browser cookies, CSRF and the actual UI/API flow remain exercised.

## Deterministic controls

`mfa-enrollment-fixture.spec.ts` drives the ordinary app binary and its real
SQLite/authentication services. Only the fixture's clock samples are
injectable; no server or browser clock is frozen and there are no sleeps.

| Control | Required evidence |
|---|---|
| Current step | One successful activation; ten recovery codes. |
| Previous step | One successful activation; returned consumed step matches the submitted step. |
| Boundary | The fixture's first sample is one real period stale; the server refuses the two-step-old code, then accepts the refreshed previous-step code. Exactly two submissions and two samples. |
| No advancement | The real server refuses the stale code; the unchanged fixture sample fails immediately. Exactly one submission. |
| Attempt limit | Three advancing but still stale samples produce three real invalid-code refusals and a hard failure, with no fourth submission. |
| CSRF refusal | A changed CSRF header produces a real 403; no new clock sample or submission is allowed. |
| Unrelated 401 | A real invalid-code refusal has only its response envelope changed by the test route; the helper fails on the different message before sampling again. |

Each control checks the stored MFA status and recovery-code count. Refusals
leave enrollment pending with no recovery codes; success leaves it active with
ten codes. The original owner session disables pending or active enrollment
after every control. The refusal samples would advance in the unrelated-error
controls if sampled again; a changed step alone never authorizes a retry.

## Validation

Local validation uses Chromium with three workers on the working tree based
on `b2ce48b93886aa1d17d3149d4bd8077a563d3aa1`.

- The first sandboxed attempt could not bind the provider-stub port (`EPERM`)
  before any case started. Its log is retained at
  `/tmp/rekenraam-384-focused-1.log`. Running with local-server access resolved
  this environment restriction.
- The initial focused selection passed **15/15** in 80.2 seconds including
  the integrated build, catalog parity, embedded-assets check and reference
  server compilation. It covers all five locale MFA journeys, the three main
  journey cases and all seven deterministic fixture controls.
- Three further repetitions passed **45/45** in 39.7 seconds with the built
  app reused, no failed attempts and no Playwright retries.
- Proxy-audit collection passes with the shared helper imported. The full
  Docker/proxy audit is not rerun for this fixture repair.
- The complete shared `scripts/test-e2e-smoke.sh` wrapper passed: **465/465**
  browser cases in 20.4 minutes, plus **12** stub tests, **8** Node harness
  controls and **4** browser harness controls. It reused the same validated
  app binary (`E2E_SKIP_BUILD=1`) with three workers. All 465 attempts passed
  without Playwright retries, skips or global errors, including the seven
  fixture controls and the original locale/main MFA journeys.

No hosted CI or full Docker proxy-deployment rerun is claimed; the proxy caller
uses the same locally exercised helper. Publication remains owner-controlled.

Logs are retained under `/tmp/rekenraam-384-*.log`. Per-run timing artifacts
are under ignored `e2e/timing-results/384-*`. The smoke wrapper uses its default
`e2e/test-results/` output; copy failure artifacts outside that directory before
any rerun, since Playwright clears it at startup.
