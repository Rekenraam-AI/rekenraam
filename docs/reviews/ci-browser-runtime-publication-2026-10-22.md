# Browser CI publication verification — 2026-10-22

The [published implementation](https://github.com/sergeyfarin/rekenraam/commit/b10e8f9cffdf10ca42ae9105a9f7806c5dc9ad30)
passes [CI](https://github.com/sergeyfarin/rekenraam/actions/runs/39723309289), [CodeQL](https://github.com/sergeyfarin/rekenraam/actions/runs/39723308030) and the
deliberate [Release Preflight](https://github.com/sergeyfarin/rekenraam/actions/runs/39723407300) run. The acceptance
evidence supports closing [#381](https://github.com/sergeyfarin/rekenraam/issues/381).
The [original controlled comparison](ci-browser-runtime-2026-10-21.md) and
its failed attempts remain unchanged historical evidence.

## Publication and source boundaries

Remote main had three newer commits: Go/frontend dependencies, editor-refetch
fixes and Svelte API adoption. The five local commits rebased cleanly onto
`77c7b440`; the original local implementation `bd20da27` is published as
`b10e8f9cffdf10ca42ae9105a9f7806c5dc9ad30`. Every measured CI/harness/sweep file hash still matches the original
experiment. The post-merge structural audit is clean, including generated
OpenAPI, document/workflow duplication and tests retained from both sides.

The published smoke selection is **378 cases**, versus the original local
376: all 376 remain, plus two `editor-refetch.spec.ts` cases. The split is
**366 app + 12 audit**, with no overlap. All **804 named matrix cells** match
the original inventory. The five release-preflight cases also pass at this
same SHA: **383 browser cases across the two workflows**, without retries,
skips or global errors. This is separate from the earlier single-wrapper
381-case validation; no new-cohort repetition is represented as an old one.

## Hosted results

| CI job | Result | Whole job wall |
| --- | --- | ---: |
| Backend Tests | success | 7m 01s |
| Frontend Check | success | 1m 16s |
| E2E Smoke (audits) | success | 8m 46s |
| E2E Smoke (app) | success | 9m 44s |
| Backend Coverage | success | 2m 48s |
| Integrated Build | success | 1m 50s |

The browser groups' observed critical path, from the first group start to
the last group finish, is **9m 45s**; their combined runner time
is **18m 30s**. The previously cited 377-case E2E job took
19m 56s. This is a **51.1% observed reduction**
across different source cohorts, dependencies and hosted processors; it is
one successful hosted observation, not a matched three-run median claim.

The controlled original 3+3 local comparison remains **36.8% lower median
full-wrapper critical path**, with **11.1% higher median combined runner
time**. The hosted observation does not replace that experiment or establish
a general compute saving. Queueing, host CPU differences and warm/cold build
conditions still matter. In this hosted run the app/preflight runners report
AMD EPYC 7763, while audits report Intel Xeon 6973P-C; each has four available
CPUs and three configured workers, with Node 24.22.0.

| Retained timing group | Passing cases | Build | Playwright incl. setup |
| --- | ---: | ---: | ---: |
| app | 366 | 119.3s | 8m 33s |
| audits | 12 | 82.8s | 7m 43s |
| release-preflight | 5 | 122.3s | 2m 11s |

Normal builds ran in all groups. The binary hashes differ between builds
because normal SvelteKit builds embed a timestamp version; the benchmark-only
version pin was never applied to production source. Every artifact identifies
the same published source SHA.

## Acceptance and artifact verification

Both successful CI jobs uploaded `browser-timings-app` and
`browser-timings-audits`; release preflight uploaded
`release-preflight-timings`. All three ZIP digests match GitHub's artifact
metadata. Downloaded JSON verifies the published SHA, status, worker/runner
context, build/global setup and per-attempt fixture/hook/view timings.
The [structured verification](ci-browser-runtime-publication-2026-10-22.json)
retains case/cell durations, source and artifact identities, job/step times
and the exact two additional tests, so evidence remains after the seven-day
hosted artifact retention period.

The local 3+3 experiment meets the 30% target. Hosted smoke verifies the
published source and successful artifact path, retaining production-CSP
contrast, route identity, shared-book and FX-isolation cases. Release
preflight verifies its five shared-harness journeys and timing upload.
Shared commands, assertion inventories, timeouts, native SQLite/FULL,
backend race checks and event frequency remain unchanged.

#382 still owns Go-cache work and #383 owns overlapping event triggers.
The existing TOTP fixture gap #384, confirmed owner-local current-read defect
#385 and provider fixture calendar gap #386 remain open. The failed
Europe/Amsterdam-midnight validation remains in the original evidence; these
UTC hosted passes do not repair or certify the known owner-date gap.
