# Payment-in-lieu admission density audit — 2026-10-23

Scope: [#379](https://github.com/sergeyfarin/rekenraam/issues/379), from the
[#284 invariant ownership report](maintainability-invariants-2026-10-21.md)
(I10, follow-up 4), after [#272](https://github.com/sergeyfarin/rekenraam/issues/272)
and [#280](https://github.com/sergeyfarin/rekenraam/issues/280). Contract:
[investment-operation plan → Short positions (#103)](../plans/investment-operation-refactor-plan.md#short-positions-103),
ADR 0013, [#255](https://github.com/sergeyfarin/rekenraam/issues/255) and
[#257](https://github.com/sergeyfarin/rekenraam/issues/257).
Source: `41732da8f4e0d9d5684f9c701eb387d0319f3821` plus the measurement test
committed with this report; no production code changed. Raw figures:
[payment-in-lieu-density-2026-10-23.txt](payment-in-lieu-density-2026-10-23.txt).

## Verdict

The admission checks are correct, and their cost is material. Every
investment command enumerates the book's effective payments in lieu and replays
each payment's short position twice: once before its effects and once after.
This includes previews and commands unrelated to any short. Cost therefore
grows with book-wide payment density, not with what the command touches. The
command holds the single writer connection throughout.

| Effective payments | Unrelated short sale, unrelated history | Same, concentrated history |
| --- | --- | --- |
| 0 (control) | 18.6 ms, 0.1 MiB | 27.5 ms, 0.1 MiB |
| 10 | 42.5 ms, 0.6 MiB ¹ | 67.7 ms, 0.7 MiB |
| 100 | 604 ms, 9.0 MiB | 2.37 s, 63 MiB |
| 300 | 2.61 s, 47 MiB | 21.1 s, 613 MiB |
| 1,000 | 17.1 s, 494 MiB | not measured (cap, below) |

Median of five runs; allocation is the mean cumulative Go allocation per
command. ¹ The matrix log line for this cell was lost to a log-file clobber
(see *Limits*); the figure is the earlier solo run on the same source (three
runs). Every operation measured at a density costs within run-to-run variation
of the same figure: refused and accepted short corrections, their previews,
and the unrelated command and its preview (table in the raw file). The checks,
not the command, set the cost.

**Disposition: file one bounded optimization issue**,
[#388](https://github.com/sergeyfarin/rekenraam/issues/388). Retain the
invariant, both checkpoints and the rollback boundary. A read-only prototype
of the dated short-quantity check cut the 100-payment costs 9–19× with
identical verdicts (*Optimization evidence*). The prototype is not merged.

## What runs per command

`runInvestmentJournalsWithGuardTx` calls `entitledPaymentsInLieuTx` before the
guard and journals. It reads every effective payment in lieu and calls
`checkPaymentInLieuEntitlementTx` for each one. After the effects,
`requirePaymentsInLieuStillEntitledTx` re-checks every payment that was
entitled before. Each check lists the short lots' cost currencies opened by the
close before the ex-date. For each currency it calls `datedHoldingTx`, which:

1. reads the position's replay intents (openings, then covers) through
   `effective_investment_operations` and `effective_investment_transfer_links`;
2. opens a savepoint, resets the projected position, applies each dated intent,
   finishes the projection and rolls the savepoint back.

Replays per command = 2 × entitled payments × cost currencies iterated (the
first currency with owed units stops the loop). Statement counts were not
instrumented. At 100 unrelated payments a CPU profile attributes 58% of the
check to the intent read and 42% to the write-and-rollback simulation, with
69% of all CPU in cgo (SQLite). Each query alone is sub-millisecond in the
`sqlite3` shell, so the cost is the number of statements, not one bad plan.

Observed amplification:

- **Unrelated history** (one single-intent short per payment): per-payment cost
  rises with book size. It is 5.9 ms at 100, 8.6 ms at 300 and 17 ms at 1,000
  payments; allocation rises from 90 KiB to 0.5 MiB per payment. So each check
  is not constant-cost even when its position is tiny. The book-size-dependent
  part is not attributed here; #388 starts by profiling it.
- **Concentrated history** (every payment owed on one instrument that gains a
  sale per payment): each check replays a prefix of that history. The command
  therefore grows roughly with payments × history: about 9× from 100 to 300
  payments.
- **Fixture builds** show the same mechanism. Each payment recorded runs the
  admission over the payments before it, so building a book is quadratic
  (unrelated) or cubic (concentrated) in density. The last payment writes took
  0.5 s at 100 unrelated payments, 16.8 s at 1,000 unrelated and 20.6 s at 300
  concentrated. The 1,000-unrelated book took 3 h 12 min to build; the
  300-concentrated book took 1 h 13 min.
- **Writer occupancy**: a trivial read queued on the single connection while
  the command runs waits nearly the command's whole wall time (`writer_wait_max`
  in the raw file). The database pool has one connection, so other requests and
  background work wait behind it.

## Correctness controls

All held at every density in both shapes (`--- PASS` for each subtest in the
raw file):

- **Independent entitlement.** The builder records, by construction, which
  effective payments are owed: one short per instrument and no cover. A
  dual-currency instrument is covered fully in EUR before the ex-date and still
  short in USD, so the check iterates past the first currency. Before and
  after all measured commands, the durable set of effective payments equals
  the constructed set. The self-check's orphan-payment row names exactly the
  damaged control.
- **Accepted writes keep payments entitled.** Five chained replacements of the
  protected short (new proceeds, same date) commit. The payment owed on it
  remains effective, and its re-check passes inside each write.
- **Refusal leaves rows unchanged.** Reversing the protected short is refused
  on preview and commit, five times each. The error names the dependent payment
  (`PaymentInLieuDependencyError`, `INVESTMENT_PAYMENT_IN_LIEU_DEPENDENCY`).
  Journals, lots, decisions, operations, links, audit events, import/source
  rows and payment-in-lieu facts are byte-identical afterwards.
- **An already unentitled payment does not block unrelated work.** The damaged
  control (facts pointing at a never-short account) stays effective through
  every accepted write, and is excluded by the before-check.
- **A superseded payment is ignored.** The reversed control is not effective and
  is never checked.
- **Previews change nothing.** Every replacement and unrelated short-sale
  preview leaves the same durable snapshot.
- **Named tests.** Run under `-race` on the final source:
  `TestPaymentInLieuEntitlementIsRecheckedInsideTheWrite`,
  `TestShortSaleReversalRechecksPaymentInLieuDependencyInsideTheWrite`,
  `TestUnentitledPaymentInLieuDoesNotBlockUnrelatedCommands`, the four
  `*NamesTheDependentPaymentInLieu` correction/cover tests and the rest of the
  `PaymentInLieu` family all pass.

The shared preview snapshot (`buyReplacementPreviewSnapshot`) did not include
`investment_payment_in_lieu_facts`. The existing #257 "nothing written"
assertions therefore could not see a payment-fact change. It is added in this
change.

## Optimization evidence (prototype, not merged)

A short side has only openings and covers: transfers, splits and basis actions
select long lots (#173). The owed units at the close before the ex-date are
therefore the effective short openings, minus the effective short covers dated
on or before that close, in each cost currency. The prototype computed this
with one read-only query per check and exact Go arithmetic, with no savepoint
or replay. It was run alongside the current replay check on every call:

- all investment, short, cover, self-check and reference-book app tests: no
  mismatch;
- the density fixtures at 0, 10 and 30 payments in both shapes: no mismatch.

Timed back to back with the current source on a quiet host (load ≈ 1.3), at 100
payments:

| Shape, operation | Current | Prototype |
| --- | --- | --- |
| Unrelated, unrelated short sale | 508 ms, 9.0 MiB | 57 ms, 0.6 MiB |
| Unrelated, refused short reversal | 488 ms | 52 ms |
| Concentrated, unrelated short sale | 2.26 s, 63 MiB | 118 ms, 2.0 MiB |
| Concentrated, accepted short replacement | 2.48 s | 115 ms |
| Build, 100 unrelated / concentrated | 55 s / 2 min 45 s | 10 s / 11 s |

The residual is still linear in book-wide payments: about 0.4 ms (unrelated) or
1 ms (concentrated) per payment. Batching the before/after scans into
grouped reads, or limiting the after-check to short positions the command
changed, are further options. #388 records them with their risks: a second
dated-short computation must stay equivalent to replay, and a scope must
cover every position a command can change.

## Limits and unmeasured

- **Host load.** Both shapes ran concurrently on a 10-vCPU AMD EPYC 3251
  without `-race`, from 22:42 to 02:20. An unrelated release `cargo test` loaded
  the host (load average up to 8) until about 23:10. Absolute times at 0–100
  payments are inflated by up to about 40% against the quiet sequential rerun
  (for example 604 ms vs 508 ms at 100 unrelated). Compare each density with
  its own run's control. The amplification is one to three orders of magnitude
  above that variation.
- **Lost log cell.** A first matrix attempt was interrupted. Its stale process
  wrote a final line into the restarted run's log, overwriting the
  `unrelated/10` unrelated-short-sale lines. The table uses the earlier solo run
  for that cell.
- **Caps.** 1,000 concentrated payments was not built: the 300 build took
  73 min and scales cubically. 1,000 unrelated was built once (3 h 12 min). No
  real owner book was available; whether one person reaches these densities is
  not established here.
- **Not measured.** Statement counts, SQLite page cache and I/O, `-race`, HTTP
  latency, other investment command kinds (each runs the same admission), long
  positions and transfers, concurrent writer contention beyond the queued-read
  probe, and the book-size-dependent part of the per-check cost.
- **Distinct from #280.** #280's representative reads and the 250,000-posting
  cash-ledger export/self-check (`scripts/measure-large-book.sh`) do not
  exercise this path. These figures do not revise them.

## Self-check observation

The self-check backstop row ("effective payment in lieu without a short lot
opened by its ex-date") accepts any short lot opened before the ex-date, even
one fully covered by the entitlement close. The writer admission refuses that
state, so it arises only from damage or a writer defect. The backstop is weaker
than the rule it guards. Filed separately as
[#389](https://github.com/sergeyfarin/rekenraam/issues/389).

## Reproduction

```bash
./scripts/measure-payment-in-lieu-density.sh
```

```bash
REKENRAAM_PIL_SHAPES=unrelated REKENRAAM_PIL_DENSITIES=0,10,100 REKENRAAM_PIL_REPEATS=5 ./scripts/measure-payment-in-lieu-density.sh
```

The test (`backend/internal/app/payment_in_lieu_density_test.go`) skips
unless `REKENRAAM_PIL_DENSITY=1`. It is not on the push gate or a CI
schedule. Fixture construction is outside every measured operation, and each
operation reports wall time, cumulative allocation and the longest queued-read
wait.
