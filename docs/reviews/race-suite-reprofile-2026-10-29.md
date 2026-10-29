# Race-suite reprofile after #405 — 2026-10-29

[#419](https://github.com/sergeyfarin/rekenraam/issues/419) remeasures the
application race suite after the schema baseline consolidation. Both
[ADR 0016](../adrs/0016-native-backend-test-resources.md) thresholds now fire:
the isolated median is 226.3 s and both full gates time out in the application
package at 300 s. This investigation changes no production
code, database setting, assertion, test schedule or deadline.

## Sources and method

| Item | Value |
|---|---|
| Delivered #405 source | `f4ee790fad60b01efb2262dc626efdb412c97b3b` |
| Current source | `d524eecf4be639494597ea7b2f6fcc6b3969c7fc` |
| Source changes since #405 | Investment income/correction fixes and coverage; category-history deletion guard and coverage; restore admission checks |
| Host | Reference ten-vCPU AMD EPYC 3251 configuration, Linux 6.8.0-147; all ten CPUs available, GOMAXPROCS unset |
| Toolchain | Go 1.27.3, GCC 13.3.0, mattn/go-sqlite3 1.14.52 with bundled SQLite 3.53.4 |
| Native configuration | CGO, `fts5,netgo,osusergo`, FULL, statement cache 0, Go race instrumentation |
| Resource policy | Shared gate `-p 2 -timeout=5m`; isolated runs also retain the 300-second deadline; no CPU affinity or test-parallelism cap |

Both measured backend trees are unmodified at their named SHA. The #405 tree
is a disposable Git archive; the current tree has only investigation documents
and helpers added. All builds and test runs are sequential. A process spot check
found no competing build/test job. This is one host, not a controlled fleet or
CI performance measurement.

The [runner](fixtures/race-suite-reprofile.py) prebuilds both race binaries and
executes uncached isolated runs in the order **base, current, current, base,
base, current**. Reversing the middle pair reduces source-order bias. It then
runs the unchanged shared backend gate twice with `GOFLAGS=-count=1
GO_PHASE_TIMINGS=1`, followed by a complete current application CPU-profile run.
Profile execution is separate from the three-run median.

The default Go cache is read-only in this workspace's sandbox. A copy in `/tmp`
supplies a writable, initially warm compilation cache. An initial sandboxed
test attempt stopped immediately because `httptest` could not open a loopback
socket; accepted measurements all use the same environment with loopback
permitted. Neither setup failure is included in the timings.

## Isolated application execution

| Source | Package seconds, three runs | Median, s | Leaf cases |
|---|---|---:|---:|
| #405 | 182.103, 219.452, 223.548 | 219.452 | 2,453 |
| Current | 224.904, 226.265, 228.302 | 226.265 | 2,476 |

All six runs pass. Each source has a stable case-name digest across its runs.
The #405 digest is the exact published #405 digest,
`cc0ed1a7c397e7032bb2e7b250ac0502178942c5086e5b20a92f5705668b542f`.
Current adds 23 leaf cases and removes none. Both sources retain the same four
opt-in measurement skips. A leaf has no descendant test, including descendants
whose `t.Run` name contains slashes; parent durations include parallel-slot
queueing and are excluded from leaf work.

The current median is 3.1% above the fresh #405 median. The first base run is
unusually fast; the 182.1–223.5-second same-source spread exceeds the median
difference. This is not evidence of a precise production regression. Likewise,
the fresh #405 median being below its historical 229.6-second result does not
establish a new optimization: its source and case digest are unchanged. The
matched rotations and source inventories make these limits visible.

## Shared gates and phase timings

| Run | Application, s | API, s | Database, s | Race phase wall, s | Command wall, s | Result |
|---|---:|---:|---:|---:|---:|---|
| 1 | 300.194, timeout | 185.322 | 114.972 | 384.18 | 421.438 | Fail |
| 2 | 300.275, timeout | 187.444 | 121.536 | 376.12 | 384.209 | Fail |

The application durations are **censored at the deadline**, not completed
execution times. Every other package passes in both runs. Both timeouts show
active SQL work and parallel-slot waiters, not a demonstrated deadlock. There
is no named assertion or Go race-detector failure before the timeout. The
backup-pruning permission warning is the intentional refusal fixture
`TestPruningFailureStillLeavesASelfCheckBehind`.

| Phase | Run 1 wall / user / system seconds | Run 2 wall / user / system seconds |
|---|---|---|
| Modules | 0.01 / 0.01 / 0.02 | 0.02 / 0.02 / 0.01 |
| Vet | 6.43 / 34.58 / 2.47 | 0.63 / 1.16 / 0.41 |
| Race compile/vet | 30.34 / 67.83 / 5.96 | 6.88 / 13.15 / 3.58 |
| Race execution | 384.18 / 1558.52 / 907.52 | 376.12 / 1593.29 / 888.45 |

Formatting and vet pass. Compilation is separate from race execution and
cannot explain the per-package timeout. The first controller stopped on the
failed gate; it continued with gate 2 and the profile on the same unchanged
source, retaining all earlier runs. The retained runner now continues after
gate failures and returns failure after collecting the profile.

These runs satisfy the **act** trigger as well as investigate. The 30% headroom
objective is not restored. The historical #405 full-gate pair (248.5/262.5 s)
is context, not a same-day causal comparison: fresh #405 full gates were not
run. The 23 added cases do not by themselves prove the cause of today's loss
of headroom. No hosted timing or CI claim is made.

## CPU attribution

The complete current profiled run passes in **275.842 s**, with the same
2,476-leaf digest as every unprofiled current run. It is slower than the
unprofiled 226.265 s median and is excluded from that median. The profile
captures 1,530.40 CPU-seconds over 272.68 s of sampling; CPU-seconds and elapsed
seconds are different measures.

Disjoint sample buckets, counting each stack once:

| Bucket | CPU seconds | Share |
|---|---:|---:|
| Explicit SQLite statement preparation, outside driver open | 469.71 | 30.69% |
| Driver connection setup / PRAGMAs, including schema load | 272.30 | 17.79% |
| Remaining test execution and runtime, including other native calls | 492.50 | 32.18% |
| Unattributed `runtime._ExternalCode` | 295.89 | 19.33% |

Go's profile attributes C calls to their Go caller where a stack is available;
it does not resolve all native engine internals. Connection setup is not a
pure schema-parser measurement. Native no-argument execution can include
implicit preparation, so the explicit-prepare bucket does not capture every
compilation. The unattributed external samples cannot honestly be assigned to
schema loading, compilation or financial calculations. Race coverage of the C
engine remains outside the Go detector, as ADR 0014 states.

Overlapping ownership/caller views are useful for choosing work, but **must
not be added** to the disjoint table:

| Caller / activity | CPU seconds | Share |
|---|---:|---:|
| All Goose migration work, including parser goroutines | 12.53 | 0.82% |
| `testdb.Open`, including its connection open | 150.74 | 9.85% |
| `db.OpenReadOnly` | 64.34 | 4.20% |
| Native connection close, including checkpoint/cleanup | 65.73 | 4.29% |
| `SelfCheckService.RunSelfCheck` | 168.37 | 11.00% |
| Explicit preparation within that self-check | 127.87 | 8.36% |

Against [#402](race-suite-cost-2026-10-24.md), migration work falls from 4.5%
to 0.82%, while explicit preparation (then 32.7%) and connection setup (then
19.4%) remain large. These are different suite versions and profile runs,
not a controlled percentage-speedup experiment. The slow leaf cases still
include the deep transfer chain and diagnostic lifecycle/restore matrix;
their elapsed durations include work and contention, not just migration replay.
No test-body or oracle assertion is removed.

## Native schema probes

Separate disposable archives of both SHAs run the retained
[#405 probe](fixtures/schema-baseline-probe-2026-10-26_test.go.txt), under race,
with five batches of 100 open/close and prepare/close operations per stage.
Probe files never enter the measured application trees. The write probe only
prepares `INSERT INTO posting_versions DEFAULT VALUES`; it does not execute it.

| Source / schema | Objects | Stored SQL bytes | Open/close median, ms | Prepare/close median, ms |
|---|---:|---:|---:|---:|
| #405 empty | 0 | 0 | 1.065 | — |
| #405 head 1 | 489 | 212,067 | 10.864 | 0.342 |
| Current empty | 0 | 0 | 0.904 | — |
| Current head 2 | 489 | 211,776 | 10.848 | 0.363 |

Both migrated schemas have 118 tables, 191 indexes, 164 triggers and 16 views.
The #417 guard reduces stored SQL by 291 bytes; it does not add objects.
Migrated connection cost remains much larger than empty-file cost, supporting
the schema-load attribution without isolating every part of that difference.
There is no meaningful measured open-cost improvement between these two
sources. Sequential microprobe batches are not a production-latency benchmark.

## Ranked recommendation and disposition

1. **Take one bounded production read-query compilation experiment next.**
   Preparation beneath `investmentFoundationCheck` (42.27 CPU s),
   `EffectiveLotMovements` (56.49 s) and `investmentReplayIntentsQuery`
   (40.65 s) totals 139.41 s as a stack union, **9.11%** of sampled CPU.
   Inventory and time their actual SQL, then simplify equivalent queries or
   use call-scoped reuse only where repetition supports it. Halving this
   bucket would save 69.7 CPU s (**4.55%**), roughly 10 s at the isolated median
   **if** wall time scaled proportionally. This is a hypothesis and upper-bound
   exercise, not an observed gain or a promise of 30% headroom. The
   [follow-up #445](https://github.com/sergeyfarin/rekenraam/issues/445) has bounded
   scope, semantic controls, measurable admission and matched suite acceptance.
2. **Further trigger/view slimming needs a specific equivalence case first.**
   Connection setup is 17.79%; even halving all of it would only remove about
   8.9% of sampled CPU. Removing financial guards or replacing the independent
   self-check is not justified. #405 already delivered a modest measured cut;
   another broad schema rewrite has no demonstrated gain here.
3. **If bounded production optimization cannot clear the act threshold,
   return an explicit resource/deadline decision to the owner.** Neither the
   9.11% target bucket nor the previous cache gain promises the required margin.
   A deadline change recovers budget, not execution time, and requires an ADR
   amendment with matched evidence. No such decision is inferred here.

Migration replay is below 1% of CPU and is not the next lever. The global
statement cache remains declined under #404; its historical incremental gain
over explicit reuse is not fresh evidence for admission. Fixture pooling,
coverage cuts, package splits and lower parallelism remain excluded.

The owner authorized filing, tracking updates and the Git push on 2026-10-29.
The chosen remedy is filed as
[#445](https://github.com/sergeyfarin/rekenraam/issues/445), completing #419's
measurement and recommendation scope. The act rule remains in force before
the next feature slice. Publishing this evidence does not change test policy
or authorize a release.

## Reproduction and validation

The [JSON evidence](fixtures/race-suite-reprofile-2026-10-29.json) retains UTC
starts, commands, all package/phase timings and outcomes, case-name digests,
case differences, leaf work, profile tables/buckets, native probe batches and
source/binary/log/profile hashes. Raw logs, binaries and the 1.3 MiB profile
remain under `/tmp/rekenraam-419/accepted` in this workspace; temporary files
are not a durable artifact store. The checked-in tables and commands preserve
the result and its reproduction method without committing binaries.

```sh
# Extract the two named SHAs into BASE and CURRENT; use no competing jobs.
# Select a writable build cache for every command (warm compilation is fine).
export GOCACHE=/tmp/rekenraam-419-cache
python3 docs/reviews/fixtures/race-suite-reprofile.py BASE CURRENT OUTPUT
# The runner exits nonzero if either full gate fails, after collecting profile.
python3 docs/reviews/fixtures/race-suite-reprofile-results.py OUTPUT \
  f4ee790fad60b01efb2262dc626efdb412c97b3b \
  d524eecf4be639494597ea7b2f6fcc6b3969c7fc HOST_JSON RESULT_JSON
# Optional final argument: directory containing probe-{base,head}.json and
# probe-runs.json. Copy the retained #405 probe into separate source archives;
# SCHEMA_PROBE_VERSIONS=0,1 (base) / 0,2 (current), SCHEMA_PROBE_OUTPUT=...;
# scripts/go-backend.sh test -race -count=1 -timeout=5m \
#   -run '^TestSchemaBaseline405Probe$' ./internal/db
```

Passed: six complete unprofiled application race runs, the complete profiled
application run, both native probes, formatting/vet and all non-application
packages in both full gates. Failed: both shared gates at the unchanged
application timeout. Document/helper checks are recorded with this delivery;
Python syntax, slash-name leaf accounting, disjoint CPU-bucket accounting,
controller continuation after failed gates, evidence consistency, local Markdown
links and `git diff --check` pass. No production code change requires a frontend
or integrated-build rerun.
