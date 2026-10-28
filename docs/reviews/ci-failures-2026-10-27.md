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

All reference requests had failed twice before the wait expired, so the
backend and retry path were not the problem. The page was stuck on
`referencesLoading`.

## Root cause

TanStack Query's observer only notifies the properties read before the query
settles. The returns screen gates its form on `accounts.isPending`,
`currencies.isPending` and `book.isPending`. If a fast response arrives before
the first render reads those flags, the settling update is dropped and the
spinner can stay up forever. The CI run's starved renderer widened that
window; the 10.1 s snapshot was a symptom, not the whole cause.

The first attempt put `referenceError` before `referencesLoading` so a known
error would outrank a sibling spinner. That made the route-sweep error case
pass, but the fork CI app suite then failed eight returns tests (and a local
three-worker run failed seven of ten): the error-first read left `isPending`
untracked, so the form stayed behind the spinner. The pre-fix screen passed
five repeated local runs.

## Changes

- `frontend/src/lib/investments/returns-screen.svelte` creates the accounts,
  currencies and book queries with `notifyOnChangeProps: 'all'`. The observer
  now delivers every result change, so the form cannot miss the settling
  update. A `showLoading` derived keeps the loading branch first (reading
  `isPending` before `referenceError`) while a known error still outranks it.
- `e2e/playwright/route-sweep.spec.ts` raises the bounded alert wait from 10 s
  to 20 s. Successful waits are ~1.4 s, so the happy path is unchanged; the
  longer bound absorbs a starved CI renderer. Twenty seconds also keeps a
  systematic failure across all 21 routes inside the 600 s test budget, so the
  collected failure messages still reach the assertion.

## Validation

- `pnpm --dir frontend run check`: 0 errors, 0 warnings.
- `pnpm --dir frontend test`: 61 files, 781 tests passed.
- `pnpm --dir e2e run test:harness`: 8 Node tests and 4 browser controls passed.
- Rebuilt with `pnpm build`.
- Focused `returns keep the form usable` case, 1 worker, ten runs: 10/10 passed.
- Full `investment-returns.spec.ts`, 3 workers: 10/10 passed.
- Focused route-sweep error test, 3 workers, `--repeat-each=3`: 3/3 passed.
  A recorded run puts `returns / error` at 1928 ms, matching
  `income coverage / error` at 1896 ms; the failing CI step was 10065 ms.
- Fork CI run [40580199872](https://github.com/Rekenraam-AI/rekenraam/actions/runs/40580199872)
  passed all six jobs on `0ec5fbf5`: Frontend Check, Backend Tests, Backend
  Coverage, Integrated Build, E2E Smoke (app) and E2E Smoke (audits). The app
  suite finished 442 tests with no failures, and `returns / error` took 2261 ms
  against the 20 s bound.
- The first attempt's fork run 40574145411 passed the audits job but failed the
  app suite, which is what exposed the observer race described above. The
  upstream PR is [#428](https://github.com/sergeyfarin/rekenraam/pull/428); its
  `pull_request` run stays `action_required` until a maintainer approves the
  first-time contributor, so the fork run is the CI evidence for this fix.
