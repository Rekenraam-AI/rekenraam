# Capital-return reversal stall follow-up (#414)

Reviewed on 2026-10-29 against `ecc8c201` for
[#414](https://github.com/sergeyfarin/rekenraam/issues/414). The original 28-second stall remains
unreproduced and its cause unconfirmed. This change improves failure evidence;
it does not claim an application fix or issue closure.

## Original artifact and newer CI evidence

Retrieved `playwright-report-app` artifact `12656873900` from
[run 40335222303](https://github.com/sergeyfarin/rekenraam/actions/runs/40335222303).
Its only browser failure artifact is `error-context.md`: the timeout errors
and source, with no page snapshot, screenshot or trace. The worker log confirms
the last request was the transaction 108 correction-chain read at
14:02:08.732 UTC, followed by shutdown at 14:02:36.996 UTC. The reversal preview
was never requested. This confirms the earlier bound but cannot distinguish a
missing/disabled control, a renderer/frame stall, or a click that opened no
dialog. A quiet backend alone does not establish a main-thread stall.

The latest 50 CI workflow runs contained 40 non-cancelled runs newer than the
last investigation comment (2026-10-26 22:21:36 UTC). Of these, 37 retained app
timing artifacts; the three unavailable artifacts are identified in the
[retained summary](evidence/capital-return-stall-2026-10-29/ci-summary.json).

- 16,364 app attempts inspected; no Click or Fill average exceeded 2.5 seconds
  per call. This is an aggregate per-attempt measure, not a maximum individual
  action duration.
- All 259 capital-return read cases passed. Their longest whole attempt was
  9.018 seconds; the largest per-attempt mean click duration was 365.333 ms.
- [Latest published CI, run 40833141957](https://github.com/sergeyfarin/rekenraam/actions/runs/40833141957),
  passed on `14867c45`. The local #416 commit has not been pushed.

This finds no recurrence in the available newer evidence. Unavailable artifacts
are not counted as successful attempts. The returns-query and CI-setup fixes
described in the [#416 review](dividend-entry-contrast-2026-10-29.md) do not
establish a cause or fix for this reversal stall.

## Loaded-book reproduction

A temporary copy of the unchanged journey seeded 150 investment positions
through the real HTTP API in `beforeAll`, then ran the existing six locale
journeys ten times on the same worker book. Every position had its own
instrument and holding plus a posted buy, sharing one settlement account.
The book continued growing through each journey's corrections and receipts.
The original 30-second per-case budget and assertions remained in place;
only the synthetic seeding hook had a longer setup budget.

Unlike the earlier cloud experiment, this used Playwright 1.64.1's pinned
Chromium headless shell 1248, also used by CI. There was one worker and no CPU
throttling. This addresses the browser-revision difference; the
[earlier investigation](https://github.com/sergeyfarin/rekenraam/issues/414#issuecomment-6428719048)
already covered competing CPU load and multiple full suites.

All 61 cases passed in 8.4 minutes: 60 locale journeys and the existing
loading/error/empty-state case. The slowest whole attempt was 12.112 seconds;
the highest per-attempt mean click duration was 319 ms. The original stall did
not recur. Timing records remain under `/tmp/rekenraam-414-loaded-timings/`.

The app binary was reused from the validated #416 build, SHA-256
`b52be53e87fbbd83a771f62c91fde5e9f98bc50745f74d6b595f852e8f7ba0c4`.
There are no runtime or frontend changes in this follow-up.

A second loaded-book experiment restricted the validation process and its
children to four CPU cores using `os.sched_setaffinity`, then ran three workers.
Each worker seeded its own 150-position book and ran three repetitions of each
locale journey plus the loading/error/empty case. DOM/network snapshots were
enabled, as in the scoped diagnostic change. This matches CI's CPU count more
closely; it is not a claim of an identical hosted machine.

All 57 cases passed in 5.0 minutes: 54 locale journeys and three state cases.
The reporter confirmed `available_parallelism: 4` and three workers. The slowest
attempt was 17.342 seconds, and the highest per-attempt mean click duration was
588.833 ms. There was no 28-second click stall. The temporary source is
`/tmp/rekenraam-414-cpu-probe.spec.ts`, with timings in
`/tmp/rekenraam-414-cpu-timings/`. CPU affinity changed only these disposable
validation processes.

## Scoped diagnostic change

`investments-capital-return-reads.spec.ts` now records DOM snapshots and browser
network evidence in its failure-retained trace. Trace screenshots and sources
remain off. The global light trace remains unchanged for all other specs.
Only these seven cases pay the additional recording cost.

The suspected sequence also has five named steps: navigation to the transaction,
the reversal click, reason entry, submit, and dialog closure. The existing
timing reporter retains their durations on successful and failed attempts;
it no longer combines this click with unrelated clicks in the only available
aggregate. Names contain no IDs, entered values or financial content.

No timeout, retry, financial assertion, locale or interaction is changed.
The steps preserve the existing action sequence without additional waits.

A temporary diagnostic control disabled the actual reversal button immediately
before clicking it with a short action deadline. Its expected failure retained:

- 121 DOM snapshots;
- four correction-chain network records;
- five `element is not enabled` actionability log entries;
- zero trace screenshot frames;
- the failed `click the capital-return reversal action` timing step.

This verifies the artifact path, not a reproduction of the original cause.
The copied trace is `/tmp/rekenraam-414-capture-trace.zip`. The control and
temporary workload copies are removed from test discovery before committing;
their sources remain at `/tmp/rekenraam-414-{loaded,light,snapshot,capture}-probe.spec.ts`
for inspection alongside the local timing records.

## Validation and remaining work

The tracing cost comparison ran identical seven-case copies in light/snapshot/
snapshot/light order, one worker and fresh worker books each time, with the same
binary and browser. Only the snapshot setting differed. All 28 cases passed.

| Run | Global light trace | Scoped DOM/network snapshots |
|---|---|---|
| First pair | 34.026 s | 39.103 s |
| Second pair (reverse order) | 33.375 s | 38.630 s |

This adds about 5.2 seconds per seven-case file locally (15.3% including setup),
which is why snapshots are enabled only for this spec. This is not a hosted
CI cost measurement. Local timing directories are
`/tmp/rekenraam-414-{light,snapshot}-{1,2}-timings/`.

The CI app smoke group passed 446/446 cases in 9.0 minutes at three workers:

```sh
E2E_SKIP_BUILD=1 E2E_SUITE=app E2E_WORKERS=3 ./scripts/test-e2e-smoke.sh
```

This includes all seven capital-return read cases and all 30 added reversal
phase records. The wrapper's 12 provider-stub tests, 8 Node harness controls
and 4 browser readiness controls also passed. Timing records are under
`/tmp/rekenraam-414-app-smoke-timings/`.

The app binary is unchanged from the earlier same-session 458/458 complete
smoke run recorded in the [#416 review](dividend-entry-contrast-2026-10-29.md).
The focused comparison above exercises every added step. Raw local timing
files and temporary probe specs are not committed; the compact CI scan summary
is retained with this report. Local document links and the summary's per-run
totals were checked against their aggregate counts.

The issue stays open at P3. A future failure should be inspected in this order:
the named failed timing step, the trace's actionability log, its DOM snapshots
and browser network timeline, then the matching worker `app.log`. Distinguish
pre-click readiness, frame stability, event dispatch and dialog readiness before
choosing a UI or test fix. Hosted confirmation of this diagnostic change remains
pending publication.
