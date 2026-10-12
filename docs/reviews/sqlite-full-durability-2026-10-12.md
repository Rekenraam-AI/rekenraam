# SQLite FULL durability and settings review — 2026-10-12

The owner selected FULL consistently after #195's completed native-driver
admission and authorized push. [ADR 0015](../adrs/0015-sqlite-commit-durability.md)
governs this follow-up. The original
[driver review](native-sqlite-driver-2026-10-12.md) remains historical
WAL/NORMAL evidence; its percentages do not describe the new FULL policy.

## Delivered policy

The application writer and migrations now use WAL/FULL. Startup requires
`PRAGMA synchronous=2`. Both the normalized DSN and the registered driver's
connection hook install FULL on each new physical connection, including direct
driver opens and replacements. Auxiliary backup, inspection and stopped-app
maintenance connections continue using FULL explicitly. Read-only connections
share the setting, though it adds no durability to reads. Ordinary tests use
the same policy as runtime; there is no faster test-only writer.

The new direct-open regression failed with OFF=0 before the fix. The expanded
writer replacement case failed with NORMAL=1 before the fix. Both pass with
FULL=2. Every read connection in the four-connection replacement case now also
checks FULL. Existing auxiliary FULL cases remain.

This changes connection policy only. The schema, migration checksum and SQL
seed are unchanged. No legacy databases/backups exist, as the owner confirmed.

## Previous conditions and settings

| Setting or condition | Review and disposition |
| --- | --- |
| Four nodes | No such deployment condition exists. This is one app process and one local SQLite file. |
| Four benchmark CPUs | Affinity 0–3 and GOMAXPROCS=4 make controlled runs comparable. They are not a production or development CPU cap. |
| Read pool max 4/idle 2 | Retain bounded concurrent exports, backup and self-check reads. It is independent of CPU count. All four connections and their replacements receive FULL. |
| One writer | Retain SQLite's single-writer architecture and application transaction serialization. Pool queueing uses Go contexts; SQLite's busy timeout does not limit that queue. |
| Busy timeout 5000 ms | Retain explicitly for runtime and auxiliary connections. Native's raw default is 5000; modernc's raw default was 0. Production already normalized the value, so this difference is absorbed. |
| Deferred BEGIN | Retain the admitted transaction behavior; do not introduce an incidental immediate/exclusive lock policy. |
| WAL/autocheckpoint 1000 pages | Retain. FULL synchronizes the WAL per commit; checkpoint size controls maintenance, not the durability window. |
| Foreign keys, CHECK enforcement, writable schema | Runtime retains FK on, CHECK enforcement on and writable_schema off, normalized against disabling DSN aliases. |
| Engine mutexes | Retain THREADSAFE=1 and full mutex protection. No concurrency protection is removed for speed. |
| Statement cache | Keep off. Cache/fixture reuse tuning is a separate measured change, not part of durability admission. |
| Recursive triggers, secure delete, isolation, locking and cache defaults | Raw old/new probes agree: recursive_triggers=0, secure_delete=0, read_uncommitted=0, locking_mode=normal, cache_size=-2000, temp_store=0, trusted_schema=1, defer_foreign_keys=0. No additional default drift was found. These are observations, not new schema requirements. mmap_size is inapplicable to the memory probe. |
| Package parallelism and timeout | Retain the shared `-race -p 1 -timeout=20m` gate and every regression. The wrapper uses available Go parallelism within a package. Historical translated-engine scaling is not evidence of optimal native scheduling. |
| CI integrated-build prerequisites | Retain the existing shared CI gates; all jobs must pass. The final push is checked against remote CI rather than treating local validation as its result. |
| Native driver/build coverage | v1.14.52 bundles SQLite 3.53.4. Runtime, tests, migrations, backup/restore, seed, development, integrated build and Docker all use this implementation with CGO/FTS5/netgo/osusergo. Production and project-test import graphs contain no modernc; Goose's upstream tests retain modernc checksums in go.sum. |
| Cross-compilation/static linking | Neither is required. Supported Linux artifacts retain an explicit libc baseline; the container supplies Debian trixie. No system libsqlite3 is linked. No desktop or Windows artifact is newly claimed. |
| Race instrumentation | Go application and driver-wrapper instrumentation stays intact. The native C engine remains outside the Go race detector. |

The current scope does not require another database, more writers, a legacy
upgrade workflow or schema regeneration. #189/#192 and fixture URI follow-up
#128 retain their separate acceptance; a native driver does not fix them.

## Validation and performance evidence

Fresh controlled ordinary/race runs retain the same 2,834-case historical
cohort, tags, four-CPU affinity, `-p 1`, `-count=1` and 20-minute package bound.
`TestNative*` is omitted only from the historical timing cohort; native cases
run in the focused and full shared correctness gates. The FULL code includes
the strengthened assertions in existing cases. Timing JSON records case-name
digests, package execution and command wall time separately.

All six uncached timing suites pass. The case count and name digest match the
original driver comparison exactly:
`85f28b35c41dce481495a6a6337c2fabfa7a52fb1825c8311c3e6731367bc629`.

| FULL measurement, seconds | Median | Range |
| --- | --- | --- |
| Full ordinary command wall | 143.97 | 142.14–167.87 |
| Full Go race command wall | 256.51 | 254.95–303.42 |
| Application ordinary execution | 90.937 | 90.149–92.333 |
| Application race execution | 113.309 | 109.964–114.740 |
| API race execution | 52.965 | 52.688–54.582 |
| Database race execution | 50.004 | 49.447–53.643 |

The slowest application race run retains **90.4%** headroom under the unchanged
1200-second bound, exceeding #164's required 30%. The historical native NORMAL
medians were 115.53 seconds ordinary wall and 225.34 seconds race wall. Keep
those as references with their original ranges and host-load qualifications;
the FULL results are a fresh durability-policy baseline. Compared with the
original modernc/NORMAL full-race reference of 1660.18 seconds, current
development feedback remains substantially shorter with stronger durability.

The 260-hop case executes in 7.68–8.11 seconds ordinary and 11.73–12.29 seconds
under race within the parallel cohort. The operation-kind export/self-check
case takes 44.94–77.35 seconds under race. Real-ledger generic-cash and investment
import commit regressions pass in all six runs. Individual case elapsed times
include concurrent-suite resource waits; they are not interactive latency
measurements. The [JSON evidence](sqlite-full-durability-2026-10-12.json)
records every run and selected cases.

The full shared `./scripts/test-backend.sh` gate passes formatting, vet and
ordinary Go race checks, including every native case, in **219.93 seconds**
with normal available host parallelism. Application execution is 103.588 s,
API 50.352 s and database 41.317 s. Unchanged pure-Go packages can use the
shared gate's usual result cache; the six controlled suites above use count=1.
These observations give no reason to cap development at four CPUs, and do not
establish an optimal schedule for every host.

The integrated `pnpm build` passes, including embedded frontend serving, and
the browser smoke suite passes **124/124** journeys (5.6 minutes). Module
verification, native-only production/project-test import graphs, active local
documentation links and shell syntax checks pass.

The final Debian image builds and its isolated-volume production smoke passes:
API health, embedded deep links, uid 10001, directory/file modes 0700/0600,
libc 2.41 compatibility and verified HTTPS to the ECB provider (HTTP 200,
certificate verification result 0). Its binary links libm/libc and the ELF
loader, with no system libsqlite3. Healthy production startup also verifies the
new writer's required FULL state before accepting requests. The disposable
container and volume are removed after the check.

Remote CI is pending at commit time; the delivery issues record the push and
its GitHub Actions results.

The standalone commit probe measures 500 transactions per mode on private
temporary WAL databases, repeated three times with alternating mode order.
Each transaction inserts one TEXT-valued entry, two balanced postings and one
audit row with foreign keys enabled. The upstream native driver permits the
diagnostic NORMAL comparison; the shipped app's registered driver requires
FULL. Preparation, writes and commit are all included. This isolates a storage
cost, not a complete application save/import workflow or a production latency
guarantee. On this shared VM, FULL's median transaction time spans 4.72–10.07 ms
and p95 spans 11.22–12.65 ms; NORMAL medians span 0.073–0.079 ms. Batching can
amortize this cost when the workflow's atomicity permits it.
Its [source](sqlite-full-commit-probe.go) is retained for reproducibility; run
`./scripts/go-backend.sh run ../docs/reviews/sqlite-full-commit-probe.go` from
the repository root. The wrapper enters `backend/` before invoking Go.

The CPU and storage are shared. The commit probe and narrow validation ran
alongside part of the first controlled pair on other CPUs; they can still
compete for storage. The historical NORMAL measurements were taken earlier
under different host load. FULL results establish current correctness and
headroom; differences from those historical medians do not isolate fsync cost.
Keep the first run and all ranges rather than deleting a slower observation.
