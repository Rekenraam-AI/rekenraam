# CI runtime investigation — 2026-10-21

The current workflow is waiting for **browser execution**. The latest sampled
[CI run](https://github.com/sergeyfarin/rekenraam/actions/runs/39616676666)
takes 19m 59s overall; E2E Smoke takes 19m 56s. The native SQLite migration
remains in place. Backend execution has also grown, but it does not explain
the present 18–22-minute workflow duration.

This is an investigation and a set of proposals, not a performance fix or a
change to validation frequency. No tests, assertions, durability settings,
timeouts or workflow triggers were changed. The roadmap continues to own
work order. No new GitHub issue was created as part of this report.

## Evidence and revision boundaries

Local source: `7a3521e0914c9235d4a47b71421751a563a6ef9b`, initially clean.
Latest sampled CI source: `77c7b4400cfc99ca6768139b2fb937edaa9bdea6`.
The latest remote workflow, Playwright config, global setup and both sweep
files were fetched at that exact SHA. Their blob identities and relevant
source lines are retained in the [structured evidence](ci-runtime-investigation-2026-10-21.json).

GitHub Actions logs were read for five successful E2E jobs and the native-era
baseline/current backend jobs. These are observations across different
revisions and hosted runners, not controlled benchmarks. Current dependency
updates are not isolated as a cause. In particular, slow E2E runs already
exist before today's Go 1.27.3 update.

The GitHub connector supplied issue state/comments, job inventories, logs and
remote source. `gh run list`/`gh run view` supplied the initial run/job timing
inventory; subsequent CLI network calls were unavailable. No CI rerun was
started. Local Playwright discovery, without building or executing journeys,
selects **376 tests in 79 files**. The newer remote revision selects 377;
these are explicitly different cohorts.

## Where the time goes

Latest run, all jobs successful and started within one second of each other:

| Job | Elapsed |
| --- | ---: |
| Frontend Check | 1m 25s |
| Integrated Build | 2m 22s |
| Backend Coverage | 3m 38s |
| Backend Tests | 7m 30s |
| E2E Smoke | **19m 56s** |

E2E's latest timing separates three costs:

- Job start to smoke command: **60.0s**, including checkout, tool/cache setup,
  dependency installation and Chromium installation. Chromium installation
  itself takes about 22s.
- Smoke command to `Running 377 tests`: **104.3s**, including stub tests,
  frontend generation/build, embedded-web test and native Go binary build.
- `Running 377 tests` to `377 passed`: **1,027.6s (17m 08s)**.

Playwright prints `377 passed (18.9m)` because its duration includes setup;
that is not 18.9 minutes of browser execution alone. About **86% of the E2E
job** occurs after browser-test execution starts. Fixing setup alone cannot
recover a short workflow.

| Successful run | Tests / workers | Smoke command to execution | Execution only | Playwright reported |
| --- | ---: | ---: | ---: | ---: |
| [Oct 12, after native resource update](https://github.com/sergeyfarin/rekenraam/actions/runs/38259548269) | 124 / 1 | 105.7s | 173.6s | 4.6m |
| [Oct 21, 05:24 UTC](https://github.com/sergeyfarin/rekenraam/actions/runs/39549389877) | 375 / 3 | 85.2s | 895.5s | 16.3m |
| [Oct 21, 11:33 UTC](https://github.com/sergeyfarin/rekenraam/actions/runs/39592836719) | 375 / 3 | 91.6s | 961.7s | 17.5m |
| [Oct 21, 13:01 UTC](https://github.com/sergeyfarin/rekenraam/actions/runs/39604328433) | 377 / 3 | 89.6s | 1,180.9s | 21.2m |
| [Oct 21, 14:25 UTC](https://github.com/sergeyfarin/rekenraam/actions/runs/39616676666) | 377 / 3 | 104.3s | 1,027.6s | 18.9m |

The count is roughly three times the post-cutover count despite increasing
workers from one to three. Counts alone understate the added workload:

- `route-sweep.spec.ts` has only four tests, but iterates 24 routes through
  themes, desktop/phone/zoom, five translated locales and error states.
- `subview-sweep.spec.ts` has eight configuration tests, each looping over
  all sub-views. The [existing delivery evidence](subview-shared-book-fixes-2026-10-19.md)
  records **65 reachable views per configuration**, or 520 reachable
  view/configuration combinations. It runs axe, fit and close checks, with
  rendered-text contrast checks where needed. Individual configuration tests
  have a 20-minute bound; route tests have a 10-minute bound.
- Both sweeps already opt into parallel mode. Simply adding `mode: 'parallel'`
  would not be a new optimization. Each configuration still performs a long
  sequence inside one test; seeding runs in `beforeAll` in participating workers.
- Repeated `networkidle` waits and full-page axe checks are concrete profiling
  candidates. They are not yet measured removable cost.

These sweeps were added after the SQLite cutover, in `1c4d46eb` and `55e7a70d`.
They are strong candidates for the long tail, **not individually timed causes**:
successful logs use dots and have no test names/durations. The workflow uploads
reports only on failure, and no explicit JSON/blob reporter is configured.
The investigation cannot attribute a percentage to a particular spec or view.

## Confirmed cache and duplicate-work findings

**E2E builds use a different Go cache.** `e2e/global-setup.ts:18` supplies
`GOCACHE=/tmp/rekenraam-go-build-cache` when the environment does not explicitly
set it. The workflow does not set it. Actions restores and later considers
saving `/home/runner/.cache/go-build`. Setup-go's logged `go env` value is
not an exported environment override for the child build. Consequently the
default E2E child build bypasses the restored Go build cache on hosted runners.
The measured 85–106s setup/build phase includes more than compilation, so
those entire intervals are not claimed as recoverable savings.

**Four Go jobs share one effectively frozen cache key.** Backend race,
coverage, integrated build and E2E use setup-go with the same `backend/go.sum`
key. Latest logs restore the same approximately 73 MB archive and each says
the primary-key hit prevents saving. The integrated/E2E jobs still download
testify and YAML despite that hit. The cache demonstrably lacks some inputs
and does not acquire newly built variants on these runs. Which job originally
created it is unverified. Race, coverage, ordinary build, feature tags and
`-trimpath` need compatible retained cache entries, not just a cache-hit label.

GitHub documents that [existing caches cannot be updated](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching).
Setup-go recommends [distinct keys or one writer for parallel builds](https://github.com/actions/setup-go/blob/main/docs/advanced-usage.md#parallel-builds).
Use that guidance to design a measured refresh strategy, rather than merely
renaming the current shared key once.

**Branch push and PR validation can overlap.** `ci.yml` triggers every branch
push and every pull request. Its concurrency key includes `github.ref`, so
branch and PR refs do not cancel one another. The initial inventory contains
two completed runs of SHA `11ba8354180208006b66553bec902270cbb2253d`:
[39602471378](https://github.com/sergeyfarin/rekenraam/actions/runs/39602471378)
and [39602477579](https://github.com/sergeyfarin/rekenraam/actions/runs/39602477579).
Event payloads were not retained, so this pair is evidence of repeated
validation of the same SHA, not proof of identical merge context or exact
trigger attribution. Trigger overlap is established by configuration. Removing
it saves runner work; it does not make a single browser suite faster.

**Integrated build and E2E each build the app.** This is real repeated work,
but the jobs are independent. Passing an artifact could remove compilation
from E2E while adding a dependency on the build job; measure the resulting
critical path before changing it. It cannot explain 17 minutes of browser
execution. Backend coverage likewise deliberately runs a separate non-race
suite; it is not disposable duplication of the race gate.

## Existing issues and duplication disposition

Issue state was checked on GitHub, including closed issues and comments.

| Work | Disposition |
| --- | --- |
| [#164](https://github.com/sergeyfarin/rekenraam/issues/164), [#195](https://github.com/sergeyfarin/rekenraam/issues/195) | Closed native migration/resource work. Their accepted cohort improved as recorded; current browser growth is independent follow-up, not evidence that migration delivery failed. |
| [#126](https://github.com/sergeyfarin/rekenraam/issues/126), [#140](https://github.com/sergeyfarin/rekenraam/issues/140) | Historical closed backend runtime work. Reuse their fixture/isolation lessons; do not duplicate the old modernc-specific optimization or restore its limits. |
| [#284](https://github.com/sergeyfarin/rekenraam/issues/284) | Open; explicitly owns remaining fixture/oracle/test-duplication review. Its completed clone scan excludes tests and E2E. Put assertion-equivalence and fixture-reuse analysis here rather than opening another broad duplication audit. |
| [#379](https://github.com/sergeyfarin/rekenraam/issues/379) | Open payment-dense production-command measurement. It does not own CI caching or browser scheduling, and no measured link makes it the cause here. |
| [#250](https://github.com/sergeyfarin/rekenraam/issues/250), [#251](https://github.com/sergeyfarin/rekenraam/issues/251), [#253](https://github.com/sergeyfarin/rekenraam/issues/253) | Closed locale-loop fixes. #253's comments explicitly identify it as a duplicate of #250, filed from a stale run. Do not recreate it; the newer route/sub-view loops are different work. |
| [#280](https://github.com/sergeyfarin/rekenraam/issues/280), [#344](https://github.com/sergeyfarin/rekenraam/issues/344), [#352](https://github.com/sergeyfarin/rekenraam/issues/352) | Closed quality/shared-book/FX-isolation work. Their correctness regressions remain necessary when changing worker scheduling or fixtures. |

No duplicate financial assertion has been proven removable by this investigation.
Route landing checks, opened sub-view checks, keyboard focus journeys and
independent financial oracles establish different facts even when setup looks
similar. A cache or scheduling fix need not remove any of them.

## Proposed bounded follow-ups

These are proposed scopes for review, not newly allocated local work IDs or
an alternative roadmap. Existing issue searches found no focused open owner
for browser runtime, Go cache policy or overlapping CI triggers.

| Proposal | Suggested urgency | Observable acceptance |
| --- | --- | --- |
| Retain browser timings and reduce measured sweep cost | P2 | Keep machine-readable durations on successful runs, including fixture/setup time and view-level steps. Record the complete selected case/matrix inventory. Compare matched runs before/after smaller independent sweep batches or ready-state waits, with identical checks/frequency and no higher timeout; demonstrate at least 30% median E2E-job reduction over three comparable runs. |
| Align and refresh Go caches by workload | P2 | E2E uses the restored cache directory; keys retain race/coverage/build variants and can refresh after source changes. Demonstrate one cold and two warm runs, retained cache saves/restores and improved compile/vet wall time. Keep native tags, FULL and Go race coverage. |
| Remove overlapping branch/PR CI execution | P3 | Record event/ref/SHA for a branch update with an open PR. Run each intended validation context once while retaining main pushes, PR checks and manual runs; preserve required-check names. Measure runner minutes independently of wall time. |

Profiling/splitting should precede runner expansion. A two-shard browser
experiment can then retain all cases on the same event, with isolated databases
and unique artifacts. [Playwright's sharding guide](https://playwright.dev/docs/test-sharding)
explains that granularity and file sizes affect balance. Do not enable global
`fullyParallel` without reviewing shared-book and `beforeAll` assumptions;
the current worker database is intentionally shared across specs within a run.
Extra shards also repeat build/fixture setup and consume more runner minutes.

Moving exhaustive matrices to scheduled-only validation would change detection
frequency, so it is a separate policy proposal with an explicit smoke inventory
and full-suite acceptance. It is not the first optimization recommended here.
Keeping all browser checks on every intended event is feasible to investigate
before making that tradeoff. Chromium caching is lower priority than measured
execution and Go-cache fixes; the observed install is only about 22s.

Backend application race execution also rose from **105.893s to 197.696s**
between the sampled native-era and current CI runs. Current command/API/DB
package execution is **59.237s / 126.961s / 123.787s**. The app retains **34.1%**
of the five-minute package bound, close to ADR 0016's 30% admission criterion.
This is reason to profile new fixture/consumer cost under #284 and measure
headroom repeatedly, not to raise the bound or blame a driver regression from
unmatched cohorts. Per-package runtime excludes compilation/vet and is not
the whole backend-job duration.

When workflow implementation starts, also reconcile the stale
`.github/instructions/ci.instructions.md` claims about three jobs, Node 22
and separately hosted E2E with the actual five-job, Node 24 workflow.
The active developer workflow and ADRs govern this investigation.

## Validation and limits

Local Playwright list discovery passes. Retained JSON parses; timestamp
arithmetic, source identities, local links and diff whitespace are checked.
No application change requires a new full backend/browser run for this report;
the cited remote runs already completed successfully. No performance saving,
per-spec timing attribution, production regression or removable assertion is
claimed without measurement. No workflow changes, push or publication occurred.
