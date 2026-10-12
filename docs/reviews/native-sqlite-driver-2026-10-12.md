# Native SQLite driver delivery review — 2026-10-12

#195 implements ADR 0014: one bundled native SQLite driver for runtime,
application/API tests, migrations, seed tooling, backup/restore and builds.
#164 owns the measured development-feedback outcome. The owner confirmed that
no legacy databases or backups exist; no installed-data migration or frozen
historical binary fixtures are added. The existing schema, checksum and SQL
seed remain unchanged. #189/#192 retain their independent original gaps.

## Engine and connection admission

`github.com/mattn/go-sqlite3` v1.14.52 replaces modernc v1.60.1. Both bundle
SQLite **3.53.4**, source ID
`2026-07-24 19:02:57 bf7c7f30031888f4e796e429ab3978879485813aaca6f641c7b33e4e09459bcc`.
There is no engine-version upgrade. Full compile options and build inputs are
recorded in the [evidence data](native-sqlite-driver-2026-10-12.json).

FTS5 and JSON are retained. The native module's default build omits optional
modernc extensions such as STAT4, column metadata, DBPAGE/DBSTAT, GEOPOLY,
RBU/session/snapshot and math/offset helpers; application queries do not use
those APIs. This is an admitted application feature set, not a promise of
identical arbitrary SQL-extension availability. Query plans can differ (STAT4
is absent). Both fingerprints advertise THREADSAFE=1. The WAL synchronous
compile default differs, but the writer explicitly installs NORMAL on every
physical connection, as before. A default audit also found that the native
driver installs NORMAL even on auxiliary connections, where modernc used
FULL. `sqliteAuxiliaryDSN` and the read-only DSN explicitly retain FULL for
backup, inspection and stopped-database maintenance; a named failing-then-
passing read-only regression and real auxiliary connections verify it.
Early native timing probes with the different auxiliary default were discarded;
the matched measurements use the corrected policy.

`sqliteDSN` preserves compatible URI options and normalizes the application's
reserved options, including aliases. Supported former `_pragma` options are
translated to required policy; unsupported options fail explicitly. A native
connection hook installs autocheckpoint=1000 on every new connection. The
writer remains one connection; the separate read pool remains max 4/idle 2,
read-only and query-only, with FULL synchronous mode retained. BEGIN remains deferred. Statement caching remains
explicitly **off**. Schema and fixture reuse are unchanged.

Named native cases cover replacement writer/read connections, refused writes,
deferred locking, cancellation followed by reuse of the same pinned connection, signed 38-digit
TEXT coefficients through Valuer/Scanner, NULL, BLOB and date/timestamp strings,
JSON/FTS5, URI punctuation and relative paths. Errors use native codes for
BUSY/LOCKED, unique and foreign-key classification; table-specific unique
identity still uses SQLite's column list after checking the extended code.
Busy errors handled by the shared API service-error boundary return retryable
503 RESOURCE_BUSY; stable domain conflict mappings remain covered by the suite.

## Backup and recovery

The destination starts native Backup while both raw-connection callbacks stay
active. Step=true means DONE. Metadata is read while the handle is valid;
Finish runs exactly once, including error and cancellation. A step without
progress waits 10 ms and observes the context instead of spinning. The
artifact is reserved with O_EXCL and mode 0600 before financial data is copied;
failed copies close connections and remove the file and its sidecars.

Real source-write contention proves BUSY cancellation, cleanup, a healthy
subsequent source write and a successful next copy. Scripted native outcomes
pin DONE/step-error/Finish-error/cancellation semantics. Existing WAL-content,
restore-safety, trial-balance and retained-key decryption cases pass. VACUUM
INTO remains the separate operator backup path.

## Build and validation

Supported Go commands source `scripts/native-sqlite-env.sh`, requiring CGO and
a C compiler/headers and enabling `fts5,netgo,osusergo`. Focused commands use
`scripts/go-backend.sh`; development, backend gates, seed generation, integrated
build, CI and govulncheck share the setup. CGO=0 and a missing compiler fail
early with actionable guidance. No system libsqlite3 is linked.

Repeated final-source build hashes are recorded in the evidence data. This is
a same-input reproducibility probe, not a published release hash.

Local builds use Go 1.27.2, GCC 13.3.0, binutils 2.42 and glibc 2.39 on
Linux/amd64. The final Debian trixie builder uses Go 1.27.2, GCC 14.2.0,
binutils 2.44 and glibc 2.41; the runtime is Debian trixie. Actual dependencies
are libm.so.6, libc.so.6 and the ELF loader. Standalone artifacts need a
compatible libc; this change makes no new OS/architecture release claim.

Two existing Docker blockers surfaced during required validation and are fixed:
Node 26 needs an explicit Corepack install (pinned 0.36.0), and the frontend
build needs the OpenAPI input copied into the build context. The runtime
explicitly installs ca-certificates with curl. The final-image smoke uses a
new disposable volume, checks production startup/API health, embedded deep-link
serving, uid 10001, directory 0700 and database/sidecar 0600, and verifies HTTPS
to the ECB provider (HTTP 200, certificate verification 0). Only its temporary
container/volume are removed afterward.

- Full final-source backend formatting, vet and Go race gate: pass.
- Full final-source ordinary coverage gate: pass, 79.6% merged coverage.
- Frontend shared checks/unit tests: pass.
- Integrated build and embedded-frontend test: pass; browser smoke: 124/124 pass.
- Final Debian image build and isolated-volume smoke: pass.
- Module verification: pass; production/test import graphs contain no modernc.
  Goose retains modernc in its own dependency/test graph, so Go module
  checksums for that upstream dependency remain; it is not an installed
  application driver or a second project test implementation.
- Throwaway Go-race canary with native SQLite linked: detects the intentional
  race and fails as expected; no deliberately racy test is retained.

A final test review strengthened cancellation/reuse by pinning one sql.Conn;
the updated assertion also passes under -race.

Go -race continues to instrument all application and driver-wrapper Go code;
it does not instrument the native SQLite C engine. No Go instrumentation
exclusions, assertion removal or timeout increase is used. Remote CI has not
run these local commits because they have not been pushed.

## Matched performance

All twelve ordinary/race runs pass with **the same 2,834 named cases**.
The sorted case-name SHA-256 is
`85f28b35c41dce481495a6a6337c2fabfa7a52fb1825c8311c3e6731367bc629`.
The evidence data records each run's wall/user/system time, peak RSS, package
execution time, passed/skipped counts, name digest and the implementation-file
hashes. Baseline source is 8ff1cb3a; the native snapshot overlays this cutover
on that workload. New native compatibility cases run in the normal gates and
are excluded only from this comparison using `-skip '^TestNative'`.

Both variants use Go 1.27.2, the same GCC/libc host, CPU affinity 0–3,
GOMAXPROCS=4, package parallelism 1, count=1 and the existing 20-minute bound.
Variant runs are sequential. This is a shared KVM workspace: independent build
and validation work used other CPUs during early runs, so timings vary and are
not an exclusive-host microbenchmark. The first native run also includes more
compilation. Medians/ranges are reported instead of selecting the fastest run;
full wall time includes compilation/checking, while package timings isolate
execution. Recheck actual CI once the commits are pushed.

Reproduction from the corresponding isolated backend checkouts:

```sh
# GOMAXPROCS=4; repeat each mode three times, with CPU affinity fixed.
taskset -c 0-3 go test -json -count=1 -p 1 -timeout=20m ./...
taskset -c 0-3 go test -json -race -count=1 -p 1 -timeout=20m ./...
# Native additionally sources scripts/native-sqlite-env.sh and uses
# -skip '^TestNative' for the unchanged-cohort comparison only.
```

| Measurement, seconds | modernc median (range) | Native median (range) |
| --- | --- | --- |
| Full ordinary wall | 125.41 (117.83–132.68) | 115.53 (106.48–177.64) |
| Full Go race wall | 1660.18 (1511.76–1788.68) | 225.34 (218.72–353.59) |
| Application race execution | 962.65 (891.13–1177.07) | 85.71 (83.55–118.23) |
| API race execution | 313.02 (291.16–428.84) | 53.94 (50.34–61.58) |
| Database race execution | 174.81 (162.54–229.97) | 49.75 (49.55–50.46) |

Median full race time falls **86.4%**, from 27.67 to 3.76 minutes. Median
application race execution falls 91.1%, from 962.65 to 85.71 seconds. Even the
slowest native application run (118.23 s) retains 90.1% headroom under 1200 s,
well beyond #164's required 30%; the median full-race reduction exceeds the
plan's 50% target. All regressions and the package deadline remain intact.

Ordinary full-suite wall time improves only **7.9%** (125.41 to 115.53 s), with
a wide first-run range. The large gain targets race overhead, not a promise of
a comparable production speedup. The strongest existing race cases are:

| Existing application case | modernc median s | Native median s |
| --- | --- | --- |
| `TestEveryOperationKindExportsItsRowsAndPassesSelfCheck` | 327.17 | 32.83 |
| `TestTransferChainDeeperThanFormerRoundLimitPropagatesInOnePass` | 210.53 | 12.76 |
| `TestRecurringDuePreservesMoreThanOnePage` | 33.24 | 3.59 |
| `TestReplaceOlderManualSaleReplaysEveryBasisMethod` | 21.18 | 1.68 |
| `TestReplaceOldBuyRecalculatesDependentSaleUnderEveryBasisMethod` | 20.59 | 1.78 |

These case times come from the parallel suite and include running alongside
other cases. A separate unchanged 260-hop probe, on CPUs 4–7/GOMAXPROCS=4,
executes in 7.93 → 5.48 s ordinarily and **197.59 → 9.63 s under race**.
Its command wall times are 8.99 → 20.06 s ordinarily and 200.18 → 39.97 s under
race; the native probes first compiled their new snapshot variants. Reported
execution and command time are deliberately separate. This focused probe has
one run per mode; it supports iteration guidance, not a three-run claim.

## Build cost and disposition

With identical embedded frontend inputs and separate empty Go caches, the
single cold-build probes take **51.09 s modernc / 96.91 s native**; warm
rebuilds take **1.01 / 2.55 s**. This exposes the C compilation tradeoff rather
than folding it into a runtime claim. The final cold/warm native artifacts and
two normal-cache repeats all hash to
`c93106488c03693cc30b5037ddbb801f1eaccdf35f505c22fed58a7305d189b9`.
These are same-input worktree probes, not a published release hash. Different
C compilers, headers, libc targets or frontend inputs remain different inputs.

#195's cutover and #164's original runtime/headroom acceptance are met locally.
No timeout increase, fixture rewrite/reuse expansion, statement-cache tuning,
process sharding, or reduced validation frequency contributes to the result.
Further ordinary-test/fixture/cache optimization may be measured separately;
it is not required to restore the original margin. PostgreSQL and desktop
implementation remain outside this delivery; SQLite's local-file shape remains.
