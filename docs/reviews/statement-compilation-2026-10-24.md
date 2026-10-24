# Statement compilation cost — 2026-10-24

[#404](https://github.com/sergeyfarin/rekenraam/issues/404) followed the
[#402 record](race-suite-cost-2026-10-24.md): statement compilation
(`sqlite3_prepare_v2`) was a third of the application race package's CPU, and
the DSN pins the native statement cache off (`_stmt_cache_size=0`). The owner's
order was Track A first (explicit reuse at hot production sites), then Track B
(the global native cache) only after its own review, preferring A if it
recovered most of B's gain.

**Result.** Track A is delivered: five loops now compile each statement once
per call instead of once per row. It is time-positive but small: −1.8% on the
single-package median and −2.2% leaf work, against −4.4% for the cache
comparator. A recovers about 40% of B's gain, not most of it. The owner
**declined Track B** on 2026-10-24: its remaining gain over A is about 2.7%
(≈6 s), and it would change the production DSN, rework six authorizer seams and
need a six-case long-lived-pool review. `_stmt_cache_size=0` and its pin
(`TestWithRequiredPragmasPreservesExistingQuery`) stay. Schema slimming in
[#405](https://github.com/sergeyfarin/rekenraam/issues/405) is the next lever,
as [ADR 0016](../adrs/0016-native-backend-test-resources.md) already orders.

## Source and conditions

| Item | Value |
|---|---|
| Base | `fa0f13bf871f5418fe33fdc821df152087067988` |
| Head | Base plus this change (Track A statement sets and two db tests) |
| Cache comparator | Base with `_stmt_cache_size=128`; measured only, never adopted |
| Host | 10 vCPU AMD EPYC 3251, 17 GiB, Linux 6.8.0-147, `GOMAXPROCS` unset — the #402 host |
| Toolchain | go1.27.3; SQLite 3.53.4 bundled by mattn/go-sqlite3 v1.14.52; `fts5,netgo,osusergo`, FULL, race |

Evidence: [statement-compilation-2026-10-24.json](fixtures/statement-compilation-2026-10-24.json),
summarised with [race-suite-cost-leaves.py](fixtures/race-suite-cost-leaves.py).
Variant trees were disposable `git worktree` checkouts.

## Which compilations are reusable

The #402 profile attributes compile time to call stacks, not to whether a
compilation repeats one done earlier. Two temporary probes, removed before
commit, answered that:

1. **Trace.** A non-race run of `./internal/app` at base with
   `SQLITE_TRACE_STMT` on every connection (`sqlite_trace` tag). With the cache
   off, every top-level statement execution is one compilation. Each was
   classified per connection: a repeat inside the same transaction, a repeat
   on that connection in a later transaction, or the first on that connection.
2. **Weight.** Each of the 2,708 distinct SQL texts was prepared five times on
   a migrated database and timed. An `INSERT` compiles every trigger on its
   table: the `posting_versions` insert costs 238 µs, a primary-key `SELECT`
   7–13 µs. 236 texts that use test-only tables (2,798 executions) took the mean.

| Compilation | Executions | Weighted compile time | Share | Who can avoid it |
|---|---:|---:|---:|---|
| Repeat in the same transaction | 243,419 | 12.0 s | 22% | Track A (explicit reuse) or a cache |
| Repeat on the connection, later transaction | 488,333 | 25.5 s | 47% | Only a connection-level cache |
| First on the connection | 245,945 | 16.6 s | 31% | Nothing; schema size sets the cost |

So explicit reuse can reach at most 22% of compile time — about 7% of package
CPU if every in-transaction repeat were captured. Most of what a cache saves is
cross-transaction, which explicit reuse cannot reach without a statement
lifetime longer than a transaction. That lifetime is Track B's review.

The in-transaction repeats are concentrated. Most come from loops within one
call, not from separate calls sharing a transaction: a version writes about
3.3 postings, and 16.4k of the 17.4k repeated `posting_versions` compilations
were later postings of the same version.

## Track A: what changed

`internal/db/statement_reuse.go` adds `statementSet`: lazily prepares each
distinct query once on the enclosing `*sql.Tx` and closes them when the owning
call returns. A query a call never runs is never compiled. The set also
satisfies the `ExecContext`/`QueryContext`/`QueryRowContext` seams that loop
helpers take, so helpers accept either the transaction or the set. Compilation
errors surface where they did before, including through `Row.Scan`.

| Site | Statements reused per call | Weighted share of compile time |
|---|---|---:|
| `insertTransactionVersion` journal entry / posting loop | journal entry insert, posting line select and insert, account day-sequence reads, posting insert, posting-tag replace | ≈10% |
| `SaveSelfCheckResults` | result upsert, once per check | 1.2% |
| `requireAccountRuleDependenciesTx` | latest account version per dependency | 0.6% |
| Checkpoint selection (`periodScopedCheckpointInvalidationRefs`, `netCheckpointInvalidationRefs`), inside a transaction only | active checkpoints per candidate | 0.7% |
| `persistInvestmentReplayProjectionTx` lot loop | lot-state upsert | 0.5% |

Together about 13% of compile time. Calls through the pool keep per-call
preparation, since a pool statement is not bound to one connection or
transaction. Nothing else changes: same SQL, same order, same error mapping.

Tests (both `internal/db`):

- `TestTransactionVersionCompilesItsPostingInsertOnce` counts compilations of
  the `posting_versions` insert with an authorizer, which fires only at
  compilation. A negative control prepares the insert twice to show it counts
  each one. Four postings compile once; without this change the same test sees
  four. The written postings keep their accounts and line order.
- `TestStatementSetReusesStatementsAndReportsCompileErrors` covers reuse, the
  three compile-error paths and re-preparation after `Close`.

The six authorizer seams #402 listed are untouched and pass at head: none of
them observes a reused site's statements.

## Matched timings

**Single package**, `go test -race -count=1 -timeout=10m -json ./internal/app`,
nine sequential runs (base, head, cache, head, cache, base, cache, base, head),
binaries prebuilt.

| Variant | Wall, s (runs) | Median | Leaf work median | Result |
|---|---|---:|---:|---|
| Base | 218.4, 224.6, 223.3 | **223.3** | 2,180.0 s | all pass |
| Head (Track A) | 215.1, 219.4, 231.1 | **219.4** (−1.8%) | 2,131.8 s (−2.2%) | all pass |
| Cache 128 (comparator) | 210.3, 214.6, 213.4 | **213.4** (−4.4%) | 2,083.6 s (−4.4%) | 2 tests fail every run, as in #402 |

All nine runs have case-name digest
`b12beb7b09973dddac9cc6c7b9108b179bfc52c59ced1b9ca6f37534379bc1db`
(2,320 leaves). The cache comparator's gain is about half of what #402
measured on its base (−8.3%). The head range overlaps the base range, as the
13–16% same-source spread #402 and slice 6 observed allows; the leaf-work
reduction is consistent with the trace estimate (13% of the 32% compile share,
diluted by race overhead).

**Full gate**, `GOFLAGS=-count=1 GO_PHASE_TIMINGS=1 scripts/test-backend.sh`,
interleaved head, base, base, head.

| Order | Variant | Race phase wall | app | api | db |
|---:|---|---:|---:|---:|---:|
| 1 | Head | 353.1 s | 255.5 s | 128.6 s | 206.1 s |
| 2 | Base | 387.4 s | 280.2 s | 150.6 s | 214.2 s |
| 3 | Base | 393.3 s | 276.6 s | 151.7 s | 214.6 s |
| 4 | Head | 391.6 s | 274.1 s | 150.5 s | 224.5 s |

Every run passed. Run 1 is the fastest of the four in every package, so part
of its margin is run variance, not the change.

## Against ADR 0016

| Threshold | Base | Head |
|---|---|---|
| Investigate: single-package median above 210 s | 223.3 s, fires | 219.4 s, **still fires** |
| Act: two uncached full-gate runs both above 270 s | 280.2 and 276.6 s, fires | 255.5 and 274.1 s, **does not fire**, with one run above |
| CI per-package median above 210 s | not measured here; ≈183 s at #402 | — |

The act threshold no longer fires at head, but one head run is above 270 s, so
the margin is thin and the next migration or test batch is likely to fire it
again. The investigate threshold still fires. Following ADR 0016's order, the
next remedy is schema-load and compilation reduction through #405's slimmed
baseline (or, by amendment with matched evidence, a deadline change).

## Track B: declined

Not adopted, and not evaluated further:

- **Gain.** −4.4% median against base here; about −2.7% beyond Track A.
- **Cost.** A production DSN policy change; six authorizer seams reworked with
  cache-on negative controls; a long-lived-pool review (schema changes on
  another connection, read-only pool and `query_only`, transaction boundaries,
  FTS5 including #403's refresh, failure and `SQLITE_BUSY` reset, pool
  replacement); size measurement including production memory; an ADR note.
- **Overlap.** #405 attacks the same compile cost (smaller trigger bodies) and
  the first-on-connection and schema-load buckets that no cache reaches.

`_stmt_cache_size=0` and `TestWithRequiredPragmasPreservesExistingQuery` stay.
Revisiting the cache needs a new issue with the review above and fresh matched
evidence.

## Reproduction

```sh
# single package, per variant worktree, sequentially
scripts/go-backend.sh test -race -count=1 -exec /bin/true ./internal/app
/usr/bin/time scripts/go-backend.sh test -race -count=1 -timeout=10m -json ./internal/app > run.json
python3 docs/reviews/fixtures/race-suite-cost-leaves.py run.json
# full gate, uncached
GOFLAGS=-count=1 GO_PHASE_TIMINGS=1 scripts/test-backend.sh
# focused tests
scripts/go-backend.sh test -count=1 -run 'TestTransactionVersionCompilesItsPostingInsertOnce|TestStatementSetReuses' ./internal/db
```

The trace probe needs the `sqlite_trace` build tag and a temporary
`SetTrace(TraceStmt)` call in the driver's connect hook; neither is committed.
