# Browser CI runtime delivery — 2026-10-21

Implementation for [#381](https://github.com/sergeyfarin/rekenraam/issues/381),
following the [initial investigation](ci-runtime-investigation-2026-10-21.md).
The controlled local full smoke command's median critical path falls from
**24m 41s to 15m 36s
(36.8%)**, with the same 376 cases and audit matrices.
This uses two CI runners with three workers each. Median combined runner
time changes by **+11.1%**. Hosted Actions
job startup, downloads and queueing remain unmeasured; this is local evidence,
not a claim that the published workflow has already achieved that result.

## Implementation

Browser runs retain source SHA, binary SHA256, worker/runner context, global
setup/build timings, selected cases, attempts, hooks, fixtures, named route/view
cells and API-operation timing totals. Incremental NDJSON survives interruption;
the final JSON summary records status and global errors. Timing artifacts omit
request/response bodies, assertion values and browser stdout. Success and failure
both upload timings; failure debug artifacts remain separate.

The route and sub-view matrices have independent projects and worker books.
Their original seed calls move from beforeAll into worker-scoped fixtures with
the same 180/240-second bounds. This lets Playwright schedule long configuration
tests independently instead of batching several onto one worker. Global
fullyParallel remains false. External-app runs remain serial.

Six repeated networkidle call sites now use request observation installed before
navigation, waiting for pending/chained requests, fonts and two rendered frames.
Explicit route identity, heading and control assertions remain. This is not a
prediction of future debounced work. Independent browser controls hold requests
and fonts, chain and abort requests, and reject missing observation. Reporter
controls exercise real pass/fail/skip and collection failures without an app.

CI selects disjoint app (364 cases) and audit (12 cases) groups through the same
smoke wrapper, on independent runners. Both build the integrated binary, run
stub/harness controls and retain three workers. Both run on every existing
event; fail-fast is disabled and artifacts have distinct names. Local smoke
defaults to all 376 cases. Release preflight keeps its weekly/manual frequency.
Native SQLite, FULL durability and backend race checks remain; no timeout increases.
Cache policy (#382), event overlap (#383) and broad oracle/fixture removal
(#284) are separate work.

## Controlled experiment

Source cohort: `afba0f41615ceddcc0b4f4012fdaa1a5f6ab7f98` plus the retained
candidate file hashes. The source SHA identifies the parent; uncommitted
implementation inputs are identified by hashes in the
[structured evidence](ci-browser-runtime-2026-10-21.json). The earlier hosted
377-case revision is a different cohort; its absolute runtime is not this
baseline.

Each job runs the actual `./scripts/test-e2e-smoke.sh`, including normal full
build, stub controls and harness controls. No E2E_SKIP_BUILD, external app or
live provider is used. Three immutable source workspaces share installed
dependencies and the same warmed Go cache. Only the Playwright configuration
and two sweep files differ; application source is identical. Ports, output,
worker databases and project books are isolated. SvelteKit normally embeds a
Date.now version: all three benchmark copies pin the same version string,
without changing the production frontend configuration. Every timed build
verifies byte-identical application SHA256
`41db39343012611de86689d1158f0ab9b979358a40f1ab634e648d9008112f3f`.

The baseline uses three workers on four pinned logical CPUs. Candidate app
uses those same four CPUs; audits run concurrently on another four CPUs with
three more workers. This models two independent runners, with eight CPUs/six
workers in aggregate. The shared ten-CPU host is unreserved; CPU/load context
is retained. This is a latency improvement with extra resources, not a claim
of 30% less compute. Timed wrapper wall includes build/setup but excludes
hosted checkout, dependency/browser installation and runner queueing.

| Attempt | Status | Critical-path wall | Combined runner minutes | Build per job (app / audits) |
| --- | --- | ---: | ---: | ---: |
| baseline-1 | pass | 24m 41s | 24.68 | 51.6s |
| candidate-1 | pass | 15m 36s | 27.44 | 60.3s / 60.5s |
| baseline-2 | FAIL (#384) | 27m 16s | 27.27 | 61.2s |
| candidate-2 | pass | 16m 18s | 28.60 | 71.6s / 70.4s |
| baseline-3 | pass | 25m 24s | 25.40 | 58.2s |
| candidate-3 | pass | 13m 56s | 24.78 | 54.9s / 54.3s |
| baseline-4 | pass | 23m 33s | 23.56 | 52.7s |

Successful baseline samples are 1, 3 and 4; candidate samples are 1, 2 and 3.
Baseline wall range: 23m 33s–25m 24s.
Candidate range: 13m 56s–16m 18s.
Successful sample totals: 73.64 baseline and
80.81 candidate runner minutes.
All formal attempts, including the failed baseline, consume
181.72 runner minutes. Nested operation/cell/test
durations overlap and must not be summed as critical-path wall time.

Baseline 2 selected/attempted all 376 cases; 375 passed. Its unchanged German
MFA enrollment fixture minted a previous-step TOTP immediately before a
30-second boundary. The activation request arrived 31ms after the boundary,
making the code two steps old. HMAC verification of the synthetic code and
the server's mfa_invalid refusal confirm the fixture race; the correct server
window is unchanged. The failure is preserved in the table and total cost,
and tracked separately in [#384](https://github.com/sergeyfarin/rekenraam/issues/384).
No fixture/auth fix was mixed into this comparison. The failed run is excluded
from the three-success performance cohort rather than inflating its baseline
with failure/restart overhead.

An earlier candidate with beforeAll grouping passed 376 cases (Playwright
20m 02s including 48.9s build) but lacked a captured outer-wrapper wall/exit;
it is exploratory, not a median sample.
An initial timestamp-unpinned comparison was stopped before browser cases
after the binary-identity check failed. Logs lost to the earlier reboot are
also excluded. Their dispositions remain in the evidence.

## Coverage and validation

Every successful baseline/candidate repetition has the same canonical selected
case inventory, with one passing attempt per case and no retries or skips.
App and audit selections have no overlap and their union is exactly the
376-case smoke suite across 79 files. Route cells retain 144 theme/viewport,
120 locale and 20 error combinations (284 total). Sub-views retain all
520 reachable combinations and 40 explicitly unreachable dispositions across
eight 70-row matrices. All 804 named executed cells and matrix results match.

Whole-file AST comparison, after explicitly normalizing the seed-fixture move,
parameter binding, named-step wrappers and readiness substitutions, confirms
the remaining sweep bodies are equivalent. The original 109 route/555 sub-view
call expressions and seed call order are retained; no financial, accessibility,
contrast, focus or route-identity assertion was removed.

Final `TZ=UTC ./scripts/test-e2e.sh` validation passes with a normal, unpinned
frontend build and three workers: **381/381 browser cases**, including all
376 smoke cases and five release-preflight cases, without retries or skips.
The 804 named cells and eight matrices match the comparison; all runtime
source hashes still match. The same wrapper also passes five provider-stub,
two reporter and four independent readiness controls. The integrated build
passes its 66 catalog cases and embedded-web check.

An earlier full-suite run crossed Europe/Amsterdam midnight and failed with
355 passes, 14 failures and 12 timeouts (20m 04s); all 381 cases were attempted.
Its result, complete diagnostics and worker database are retained locally;
sanitized evidence is included in the JSON. It exposed two independent,
unchanged date gaps. Newly created owner-October-22 accounts are absent from
the SQL current-version view, which still chooses versions using UTC's
October 21: [#385](https://github.com/sergeyfarin/rekenraam/issues/385) owns
the product read defect and sibling survey. Provider dividend fixtures
expect a local-calendar ex-date while the stub uses UTC: [#386](https://github.com/sergeyfarin/rekenraam/issues/386).
Other account/date-dependent failures are retained with their original
contexts; this performance change does not repair or certify those paths.
The complete rerun uses CI's UTC environment, with no runtime source,
assertion or timeout change. Deliberately opposite owner/browser-zone
controls remain selected. This is a scoped UTC validation, not a claim
that the known local-midnight product gap is fixed.

Scoped TypeScript checking, Bash syntax, workflow YAML/selection checks,
fixture/AST equivalence and diff whitespace checks pass. The CI change
preserves backend race and timeout policies. Hosted CI verification
remains pending until the local commit is pushed through the normal owner
workflow. #381 stays open for that follow-through; no push or pull request
is part of this delivery.

The UTC full-wrapper run takes 18m 23s. Both full-suite
validation attempts consume 38.45 runner minutes; all formal comparisons
and full validations total 220.18 runner minutes. Preliminary incomplete measurements remain
separate, as described above.
