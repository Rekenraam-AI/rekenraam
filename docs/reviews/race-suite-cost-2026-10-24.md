# Race-suite cost investigation — 2026-10-24

[#402](https://github.com/sergeyfarin/rekenraam/issues/402) asked whether the
application race package's growth comes from migration-chain tests, and whether
a coverage-preserving test optimization recovers it. It does not. Migration
replay is 4.5% of package CPU. The growth is the per-connection and
per-statement cost of a larger schema, paid by every test, plus new tests. No
test-fixture change recovers a measurable share without skipping real upgrade
paths, so none is made. One measured lever remains, and it is a production
setting: the native statement cache. The owner accepted the remedy sequence
below, which also tries explicit statement reuse first.

The investigation found a production bug,
[#403](https://github.com/sergeyfarin/rekenraam/issues/403). On a reused
read-only connection, the self-check's `sqlite_integrity` reported a false FTS5
checksum mismatch. It is fixed in the same change.

Governing documents: [ADR 0016](../adrs/0016-native-backend-test-resources.md)
(300-second package deadline, `-p 2`, 30% application headroom) and the
[slice 6 timing](maintainability-tests-2026-10-24.md#race-suite-cost-and-headroom)
whose attribution this corrects. Work order stays with the [roadmap](../roadmap.md).

## Source and conditions

| Item | Value |
|---|---|
| Base | `935ccdb7cfea729c6ad0fb133350f7fecc441ff8` |
| Head | Base plus this change: #403 fix, a self-check reader per test and database, and two regression tests |
| Host | 10 vCPU AMD EPYC 3251 (one thread per core), 17 GiB, Linux 6.8.0-147, `GOMAXPROCS` unset |
| Toolchain | go1.27.3; SQLite 3.53.4 bundled by mattn/go-sqlite3 v1.14.52 |
| Configuration | `scripts/native-sqlite-env.sh` (CGO, `fts5,netgo,osusergo`), FULL synchronous, race instrumentation. ADR 0016 resources unchanged |

Evidence: [race-suite-cost-2026-10-24.json](fixtures/race-suite-cost-2026-10-24.json),
summarised with [race-suite-cost-leaves.py](fixtures/race-suite-cost-leaves.py).
Each `-json` run was reduced to wall time, leaves, leaf work and failures. The
leaf definition matches slice 6.

## Matched timings

**Single package**, `go test -race -count=1 -timeout=10m -json ./internal/app`.
Nine sequential runs in three rotations
(base, head, cache, head, cache, base, cache, base, head). Test binaries were
prebuilt.

| Variant | Wall, s (runs) | Median | Leaves | Result |
|---|---|---:|---:|---|
| Base | 219.4, 232.2, 225.7 | **225.7** | 2,301 | all pass |
| Head | 239.0, 232.1, 221.4 | **232.1** | 2,301 | all pass |
| Statement cache 128 (experiment) | 206.9, 204.1, 212.2 | **206.9** | 2,301 | 3 failures, the same every run |

All nine runs have the same case-name digest,
`37d87a9432505f80db2444babdc5b88d8ce297ac83e45681f34aed13f6e00196`. No case was
added to, renamed in or removed from the application package. Head and base
differ by less than the 13–16% spread between same-source runs, which slice 6
also observed. The change is time-neutral.

**Full shared gate**, `GO_PHASE_TIMINGS=1 scripts/test-backend.sh` (race,
`-p 2`, five-minute deadline). Runs 3–5 add `GOFLAGS=-count=1`: the wrapper
does not bypass Go's test cache, and a first attempt at runs 3–4 was served
from it.

| Order | Variant | Race phase wall | app | api | db |
|---:|---|---:|---:|---:|---:|
| 1 | Base | 383.5 s | 277.7 s | 145.1 s | 216.5 s |
| 2 | Head | 386.1 s | 270.0 s | 158.7 s | 212.3 s |
| 3 | Head | 378.1 s | 278.2 s | 144.5 s | 212.7 s |
| 4 | Base | 397.2 s | 276.6 s | 159.2 s | 225.4 s |
| 5 | Statement cache (experiment) | 367.6 s | 246.7 s | 145.9 s | 209.4 s |

Under the full gate, the application package shares ten CPUs with a second
package. It takes about 50 s longer than it does alone. That is the #394
measurement class (293.5 s), not CI's per-package figure and not the
single-package runs.

## Where the time goes

A CPU profile of one base single-package run (1,376 CPU-seconds over 222.7 s)
attributes C time to the calling Go stack. Buckets are cumulative and overlap:

| Bucket | CPU s | Share |
|---|---:|---:|
| `sqlite3_prepare_v2`: statement compilation, spread across production writers and self-check queries | 449.8 | 32.7% |
| Driver connection open: its PRAGMAs force a full schema load | 267.6 | 19.4% |
| `SelfCheckService.RunSelfCheck` | 152.2 | 11.1% |
| `testdb.Open`: copy, open, ping | 140.4 | 10.2% |
| `db.OpenReadOnly`, 81.3 s of it from `selfCheckOver` | 85.3 | 6.2% |
| `sql.DB.Close`, including the WAL checkpoint | 63.9 | 4.7% |
| **All goose migrations, up and down, including SQL parsing** | **62.1** | **4.5%** |
| Online backup / restore / bundle export reads | 13.6 / 12.8 / 9.7 | ≤1% each |

Probes under race on the same host:

- Opening a migrated database costs 8.1 ms (read-only 7.3 ms). An empty
  database opens in 0.75 ms. The difference is parsing 507 schema objects
  (119 tables, 205 indexes, 162 triggers, 21 views; 216 KB of SQL). Every
  physical connection pays it.
- Open cost by schema version: 7.31 ms at 0001, 7.87 ms at 0013, 7.88 ms at 0020,
  **8.91 ms at 0026**. From 0020 to 0026, schema text grew 4.7% and open cost
  grew 13%; migrations 0021–0026 added larger trigger bodies.
- A fresh full migration takes about 1.1 s serially, about half of it 0001
  including parse. The full down chain takes about 0.4 s. A no-op `Migrate`
  takes 0.7 ms.

### Correcting the slice 6 attribution

Slice 6 selected its 74 "migration/restore/backup/bundle" leaves (81 at this
base) with a lexical name pattern. At this base the set holds 364 s of leaf
work. A third of it, 121 s, is six `TestInvestmentDiagnosticAuditResolutionLifecycleAndRestore`
cases that never migrate. They run a 14-check self-check and export a bundle
in each phase, and restore through `RestoreSQLiteDatabase`. Many other matches
contain "restores" as a verb. These tests are connection- and prepare-bound.
They grew with the chain because each migration enlarges the schema that every
connection loads and every write statement compiles against, not because they
replay migrations. The 13 → 14 check self-check slice 6 noted adds to the same
cost.

So the stable causes are **new tests** and **schema size**. Chain length matters
through schema size, and every test pays for it. Migration replay is a small,
real part.

## Migration path coverage

Fourteen application tests run goose. Together they use 62 CPU-seconds; the
largest is 9.2 s (0.7%). Each exercises a distinct path, and the
[map](fixtures/race-suite-cost-2026-10-24.json) records the targets:

- Fresh full chains: the bundle contract (`fresh`, `seeded`) and six restore
  tests that build source and live files from a fresh database.
- Populated deep downgrades and upgrades through later migrations:
  capital-return forward migration (0026 → 0002 → 0026); the capital-return
  refusal at 0003; gains runs (down to 0006, up, then refusal at 0007).
- Refusals at a target: crypto at 0006 and dividend-instrument facts at 0009.
- Near-head down/up cycles: payment attribution (0022), cash-in-lieu parent
  and historical entry (0023, 0025) and dividend attribution (0024).

The deep tests are the only populated runs of the intermediate down migrations
over real investment books. Starting them nearer to their subject migration
would skip those paths for at most about 2% of package CPU. The issue rules
that out without equivalent coverage, and no other test supplies it. The six
restore tests' fresh migrations total about 12 CPU-seconds and also exercise
the fresh-file path the runtime takes. They are kept. Goose re-parses migration
files per provider (14.5 s, 1.1%). Caching that would require replacing goose's
provider construction for a sub-noise gain, so it is not pursued.

## What was tried

**A self-check reader per test and database.** `selfCheckOver` opened a new
read-only pool for every run. The helper now keeps one per test and database,
like the server's process-lifetime reader, and closes it at that test's
cleanup. Because most tests run one self-check, it saves no measurable time,
as the tables above show. It is kept because it is the production shape. Its
first full run failed 21 top-level tests on a false
`sqlite_integrity: fts5: checksum mismatch for table "transaction_search"`.

**#403.** In SQLite 3.53.4, the FTS5 part of `PRAGMA integrity_check` uses a
connection's cached FTS5 structure. It skips the freshness check an ordinary
FTS5 read performs. Once a connection has run the pragma, a write from another
connection that changes the index makes that connection's next check report a
mismatch. A fresh connection reports `ok`, and so does FTS5's own
`'integrity-check'` command. The server keeps one reader for its lifetime, and
backups chain a self-check, so any later run after new searchable entries could
falsely fail.

`SelfCheckRepository.SQLiteIntegrity` now runs both pragmas in one read
transaction. It first reads one row from each FTS5 table, which refreshes the
cache. Real index damage still fails, with "malformed inverted index". The
tests that fail without the fix:

- `TestSQLiteIntegrityOnAReusedReaderAfterSearchWrites` (db, run 2). It keeps a
  canary on the bare pragma, so a fixed SQLite upgrade will flag the refresh
  for reconsideration.
- `TestRepeatedSelfCheckRunsPassAfterNewEntries` (api, run 2). It needs
  searchable text: a transaction without payee or description leaves the index
  unchanged.

No matching upstream report was found.

**Statement cache, measured only.** With `_stmt_cache_size=128` on every
connection, the single-package median is 8.3% lower (206.9 s vs 225.7 s). In
the full gate the application package is about 11% lower (246.7 s vs 270–278 s).
It is not adopted here, for three reasons:

1. It changes the production DSN policy that `TestWithRequiredPragmasPreservesExistingQuery`
   pins. The [driver plan](../plans/native-sqlite-driver-migration-plan.md)
   deliberately leaves it as a separately measured change.
2. Three test seams depend on per-call preparation. Response enrichment counts statement
   batches, its failure-injection test injects at compile time, and the
   throttle-rollover test has a credential barrier. All three use a SQLite
   authorizer, which fires only at compilation, so they fail with the cache on.
3. Cached statements across long-lived pools need their own correctness review.
   Schema changes, read-only pools and the integrity check above are cases where
   connection state has already mattered.

## Headroom against ADR 0016

ADR 0016 requires at most 210 s (30% headroom) for the application package
under its `-p 2` schedule. Its admission run was a four-CPU matched comparison.
Here:

| Measurement scope | Application time | Headroom under 300 s |
|---|---:|---:|
| CI per-package median, last six `main` runs before this change (slice 6) | ≈183 s | ≈39% |
| Single package, this host, median of three | 225.7 s | 24.8% |
| Full gate `-p 2`, this host, two uncached base runs | 276.6–277.7 s | 7.4–7.8% |
| Full gate with statement cache (one run, failing tests) | 246.7 s | 17.8% |

CI still clears 30%. The local full gate on a ten-CPU development host does
not, and is close enough to the deadline that ordinary noise can fail it, as
#394's 293.5 s did. The cost is CPU-bound (effective parallelism about 9.8 on
10 CPUs), so a faster or larger host scales it down directly.

## Monitoring threshold

The 200-second trigger slice 6 proposed did not say which measurement it
applied to. These replace it. They are adopted in the
[ADR 0016 amendment](../adrs/0016-native-backend-test-resources.md#amendment--race-suite-cost-thresholds-2026-10-24):

- **Investigate** when the median of three interleaved, uncached single-package
  runs of `./internal/app` (this document's command, reference ten-CPU host)
  exceeds **210 s**, or when CI's per-package median over the latest six
  successful `main` runs exceeds **210 s**.
- **Act before the next feature slice** when two uncached full-gate runs both
  exceed **270 s** (90% of the deadline) for any package.
- Single runs, and CI jobs without a matched base, are observations, not
  triggers.

At this base, the single-package and full-gate thresholds have both fired; the
CI one has not.

## Options considered

The cost has two levers: compilation and schema-load work per test, and the
amount of test work. Every option pulls one of these, or changes the
scheduling or budget.

| # | Option | For | Against |
|---:|---|---|---|
| 1 | Global native statement cache | One setting; measured 8–11%; also speeds production requests | Changes behavior on every connection; needs a correctness review and six authorizer seams reworked; a one-off gain while the schema grows |
| 2 | Explicit statement reuse at hot sites | No global semantics change; local and reviewable; authorizer seams mostly keep working; speeds production writes. Compilation under `insertTransactionVersion` and the investment journal writers alone is about 85–120 CPU-seconds | Gain not yet measured; needs care per site; diminishing returns |
| 3 | Accept the trend with explicit thresholds | No cost; CI keeps about 39% headroom | The local full gate is at about 92% of the deadline already, and every migration and test adds to it |
| 4 | Reduce schema load cost (fewer or smaller triggers and views) | Root cause; cheaper connections in production too; migrations can still be rewritten before v0.1.0 | The triggers are ledger invariants; high risk, slow; schema load is about 19% of CPU, so a large cut yields roughly 10% |
| 5 | Split the application test package | Better `-p 2` balance and per-package headroom | Total CPU unchanged, so it games the bound; the tests use unexported internals, making it a large refactor |
| 6 | Trim or cheapen test work (self-check runs, overlapping matrices) | Direct cut | Trades coverage for time; needs a per-test coverage map |
| 7 | Pooled, reset or in-memory fixtures (open/close is about 15%) | Direct cut | Breaks isolation or production equivalence; ADR 0016 forbids test-only database settings |
| 8 | Raise the deadline, use faster hardware, or run a CI-only full gate | No code | Hides the trend; ADR 0016 requires measured optimization first; this host is a low-clock EPYC 3251 |

## Owner disposition

**Accepted 2026-10-24.**

1. **Baseline now (option 3).** The thresholds above are adopted in the
   [ADR 0016 amendment](../adrs/0016-native-backend-test-resources.md#amendment--race-suite-cost-thresholds-2026-10-24).
   #402 and #403 close with this change.
2. **Reduce statement compilation cost, measured, in
   [#404](https://github.com/sergeyfarin/rekenraam/issues/404).**
   - Track A (option 2) comes first: explicit statement reuse at the hot
     production sites the profile names.
   - Track B (option 1) is the global native cache. It needs:
     - the six authorizer-based seams made independent of compilation, each
       with a cache-on negative control (three fail with the cache on, and
       three may pass vacuously);
     - a long-lived-pool correctness review;
     - a measured size;
     - an ADR note.
   - Prefer Track A if it recovers most of Track B's gain. Adopt Track B only
     after its review.
3. **Tripwire.** If the full-gate threshold still fires after #404, the next
   choices are option 4 (schema slimming, before v0.1.0) or option 8 (a
   deadline change), through an ADR 0016 amendment with matched evidence.
   Options 5–7 stay rejected.
4. **Before v0.1.0 regardless (added the same day).** Collapse the migrations
   into one slimmed baseline in
   [#405](https://github.com/sergeyfarin/rekenraam/issues/405). Collapsing
   alone recovers only migration replay (about 3–4% of CPU, mostly by retiring
   downgrade tests). Slimming targets the schema-load and compilation buckets.
   The tag makes migrations append-only, so this is the last cheap window.

Excluded throughout, by #402 and ADR 0016:

- lowering `-p` or test parallelism;
- faster test-only database settings;
- removing assertions or upgrade paths.

## Reproduction and validation

```sh
# single package, per variant tree, sequentially
scripts/go-backend.sh test -race -count=1 -exec /bin/true ./internal/app
/usr/bin/time scripts/go-backend.sh test -race -count=1 -timeout=10m -json ./internal/app > run.json
python3 docs/reviews/fixtures/race-suite-cost-leaves.py run.json
# full gate, uncached
GOFLAGS=-count=1 GO_PHASE_TIMINGS=1 scripts/test-backend.sh
# profile
scripts/go-backend.sh test -race -count=1 -json -cpuprofile=cpu.out -o app.test ./internal/app
go tool pprof -top -cum -focus='pressly/goose' app.test cpu.out
```

Variant trees were disposable `git worktree` checkouts of the base. Probes
(migration timing, connection opens, schema growth and FTS5 conditions) were
temporary test files, removed before commit.

**Passed:** every base and head run, single package and full gate, including
the uncached full gates at head (runs 2 and 3). Both #403 regression tests fail
without the fix and pass with it. **Failed, as expected:** the
statement-cache experiment, for the reasons listed.
