# CI failures from e94b1c83 — 2026-10-18

Investigated at local `0863a4a5`. The five completed failed CI runs from
`e94b1c83` through remote `c11fb917` all failed only **E2E Smoke**; backend
tests, backend coverage, frontend checks and the integrated build passed.
The failures have two separate causes.

| Commit | CI run | Failure |
| --- | --- | --- |
| `e94b1c83` | [39071268097](https://github.com/sergeyfarin/rekenraam/actions/runs/39071268097) | Mobile account journey: ambiguous Name label |
| `a03b01ee` | [39104089621](https://github.com/sergeyfarin/rekenraam/actions/runs/39104089621) | Same ambiguous Name label |
| `ff91380f` | [39105372631](https://github.com/sergeyfarin/rekenraam/actions/runs/39105372631) | Legacy dividend fixture: staged row identity trigger |
| `aeed59cd` | [39105972031](https://github.com/sergeyfarin/rekenraam/actions/runs/39105972031) | Same legacy dividend fixture failure |
| `c11fb917` | [39113178777](https://github.com/sergeyfarin/rekenraam/actions/runs/39113178777) | Same ambiguous Name label |

## Root causes and disposition

The mobile account journey was introduced in `92362f89` for #269. Its partial,
case-insensitive `getByLabel('Name')` matches both the Name input and the Parent
select whenever a parent option contains "name". The select lives inside a
wrapping label whose text includes its options. Books persist between specs
within each worker, so prior account names determine whether the selector is
ambiguous. This explains passing runs between failures; `e94b1c83` did not
modify the journey or account editor.

The fix uses an exact Name label on both create and edit. The existing journey
now creates a `Name collision` account itself, so the triggering state is always
present even when the spec runs alone. With that fixture and the original
locator, the focused journey fails with the identical two-element strict-mode
error. No retry, timeout or worker-count change is needed.

The independent dividend failure came from migration 0016 in `0de4842b` (#302):
the legacy fixture inserted an identity without the new `source_fingerprint`,
so the staged-row trigger rejected it. Existing commit `5fc69223` supplies the
fingerprint. The following [CI run at 6e1af462](https://github.com/sergeyfarin/rekenraam/actions/runs/39111026913)
passed. No additional migration or production import change is required.

The three-repeat focused check exposed another selector ambiguity in
`income-coverage.spec.ts`: repeated specs can reuse a worker's book, leaving
two unresolved imported payments of 12.34. Selecting the card by amount and
kind matches both. The card now also matches the fixture's unique instrument
ticker, so assertions and the repair link refer to the payment created by
that test. This failure was observed locally during repetition, not in the
five CI logs above.

## Validation

- Rebuilt the integrated binary with `pnpm build`.
- Reproduced the account selector failure before changing either locator.
- Focused account, dividend-review and income-coverage specs passed all 36
  executions with three workers and `--repeat-each=3`:
  `E2E_SKIP_BUILD=1 pnpm --dir e2e exec playwright test reference-data-management.spec.ts import-dividend-review.spec.ts income-coverage.spec.ts --project=app --workers=3 --repeat-each=3`.
- Complete CI smoke gate: all **313 passed** in 5.4 minutes with
  `CI=1 E2E_SKIP_BUILD=1 ./scripts/test-e2e-smoke.sh`, reusing the binary
  rebuilt above. No retries were added. Browser checks needed local socket
  access for the disposable apps and provider stub.
- `git diff --check` passed. Remote CI requires a subsequent push; this change
  does not push or publish. The separate release-preflight suite is outside
  the smoke gate and was not run for these selector fixes.
