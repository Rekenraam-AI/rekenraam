# CI failures from 8b64a5a9 — 2026-10-27

`8b64a5a9` changed only `docs/reviews/focus-first-ui-review-2026-10-27.md`,
but [run 40559643992](https://github.com/sergeyfarin/rekenraam/actions/runs/40559643992)
failed **E2E Smoke (audits)**. Backend tests, coverage, frontend check and the
integrated build passed. The failing case was
`route sweep (#278) › every data route shows an accessible error state when its
data fails`, step `returns / error`.

## Trace evidence

The retained trace (`playwright-report-audits`, #414) shows:

- `page.goto('/app/investments/returns')` completed in 44 ms at 131542 ms.
- The alert wait started at 131592 ms with a 10 s budget.
- `accounts?include_archived=true`, `currencies`, `books/current` and
  `investments/instruments` were fulfilled with 500 at 131695 ms and retried
  once at 132702–132704 ms. No requests followed.
- The page still showed the reference-loading branch at 141606 ms:
  `… Back to investments Loading returns…`. The failure snapshot's
  `locator('main').innerText()` itself took 10.1 s.
- The next route (`pricing`) navigated at 141607 ms and its alert appeared in
  1.34 s. The other audit worker was running sub-view axe steps every ~0.8 s
  throughout; `summary.json` records 3 workers on 4 vCPUs.

All reference requests had failed twice before the wait expired, so this was
not a backend or retry-timing failure. The 10.1 s snapshot call points to a
starved renderer/worker rather than an endless spinner. Local repetition did
not reproduce it (12 focused runs with 3 workers; two full route-sweep runs).
A local probe at 50× CPU throttling still rendered the alert in ~5 s.

## Changes

- `frontend/src/lib/investments/returns-screen.svelte` now checks
  `referenceError` before `referencesLoading`. A known failed reference query
  no longer stays hidden behind the spinner while another reference query is
  still settling. A temporary mixed-state probe (accounts failed immediately,
  the other reference requests delayed 8 s) showed the alert at 1.4 s.
- `e2e/playwright/route-sweep.spec.ts` raises the bounded alert wait from 10 s
  to 20 s. Successful waits are ~1.4 s, so the happy path is unchanged; the
  longer bound only absorbs a starved CI renderer. Twenty seconds also keeps a
  systematic failure across all 21 routes inside the 600 s test budget, so the
  collected failure messages still reach the assertion.

## Validation

- `pnpm --dir frontend run check`: 0 errors, 0 warnings.
- `pnpm --dir frontend test`: 61 files, 781 tests passed.
- `pnpm --dir e2e run test:harness`: 8 Node tests and 4 browser controls passed.
- Rebuilt with `pnpm build`.
- Focused route-sweep error test, 3 workers, `--repeat-each=3`: 3/3 passed.
  A recorded run puts `returns / error` at 1928 ms, matching
  `income coverage / error` at 1896 ms; the failing CI step was 10065 ms.
- Remote CI still needs the fix commit; this review does not push.
