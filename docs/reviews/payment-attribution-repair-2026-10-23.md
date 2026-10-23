# Repairable payment attribution — 2026-10-23

Delivery: [#388](https://github.com/sergeyfarin/rekenraam/issues/388) and the
repair integration for [#389](https://github.com/sergeyfarin/rekenraam/issues/389),
under [ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md).
The [cross-family contract](../plans/financial-dependency-simplification-plan.md)
remains open under #391. This delivery covers payment in lieu of dividends;
it does not relax fractional-share cash-in-lieu allocations (#392),
dividend/withholding payer rules (#393), or genuine quantity/basis dependencies.

## Behavior and implementation

An owner can record a real cash charge without inventing a short, holding,
instrument or ex-date. Entry omission/null leaves attribution unknown.
Replacement omission preserves, explicit null clears and a supplied value
replaces; HTTP decoding preserves all three cases. A holding alone remains
partial attribution. Supplied references must identify same-book objects of
the correct kind, and supplied dates must be calendar dates. Archived or
future-opened references need not have been eligible financial posting targets.

The existing correction form edits these fields in six languages on mobile.
One native audited inverse plus replacement retains original facts and source
history. An attribution-only repair keeps identical financial legs, cash,
payment date, quantities, gains and checkpoint balances. It requires neither
a fictitious broker refund nor a new queue or warning acknowledgement. The
workspace uses its existing close-after-save behavior; reopening the original
or effective transaction reads current effective attribution. Saved self-check
runs remain historical until rerun.

The local entitlement gate and shared writer's book-wide before/after
entitlement replay are deleted, including their retired error codes/copy.
There is no replacement permission scan. The explicit read-only diagnostic
still evaluates historical support, separately from financial integrity and
technical errors. Unknown context is unavailable coverage, contradicted
history is a warning, and broken financial/structural facts remain failures.
Provider/owner differences add `provider_attribution_mismatch`, including
when context is cleared; unavailable coverage is still disclosed separately.

The existing nullable-field mechanism is shared with recurring updates.
Migration 23 makes only the attribution facts nullable, preserves all old
rows, and keeps immutable facts and null-safe reversal equality. Two typed
read views centralize structural facts and payment correction ancestry;
there is no generic dependency framework. Bundle schema 16 exports unknown
fields as empty cells. Downgrade to schema 22 refuses unrepresentable facts
instead of erasing or restating them.

Provider event payloads, proposals and decisions remain unchanged. Identity
follows immutable correction ancestry, so a corrected descendant cannot be
linked to a second event. Re-fetching the original event does not post again.
The fixture verifies this through assign/change/clear/restore cycles.

## Independent controls and policy-test disposition

`TestOrdinaryInvestmentWritesNeverReadPaymentAttributionContext` installs a
SQLite authorizer that denies reads of holding/instrument/ex-date payloads
while actual native short entry, replacement and reversal commands and their
previews execute with existing payments. All succeed with zero context reads;
a positive control proves the denial is active. SQLite's independent foreign
key checks may inspect fact identity columns; this is not a claim of zero
SQLite statements or zero financial/source reads.

Named repair controls cover zero checkpoint impact on reconciled cash,
unchanged lots/gains, original facts, same-audit inverse/replacement, exact
amount/recovery and posting-role refusals, invalid supplied references/dates,
late injected failure and full rollback. A real unused-holding account
reclassification also permits clearing attribution and standalone reversal:
new supplied IDs are validated, while exact inverse facts retain their original
references without imposing a new current-role requirement. HTTP tests exercise real omitted,
null and supplied JSON, nullable effective terms and preview rollback.
Migration tests cover preserved history, immutability and refused lossy
rollback. CSV tests assert genuinely empty cells, not invented zero IDs.

The structural audit intentionally reports these retired test names. Their
financial assertions remain, with the changed policy tested explicitly:

| Retired assertion/test | Current outcome/control |
| --- | --- |
| `TestPaymentInLieuRefusesAHoldingThatWasNotShortAtTheClose` | `TestPaymentInLieuAllowsUnsupportedAttributionButRetainsFinancialChecks`; exact cash/recovery and real posting-role refusals retained. |
| `TestPaymentInLieuEntitlementIsRecheckedInsideTheWrite` | `TestPaymentInLieuPreparedBeforeShortCorrectionKeepsCashAuthority`; preparation race commits real cash and reports unsupported attribution. |
| `TestReverseShortSaleNamesTheDependentPaymentInLieu` | `TestReverseShortSaleLeavesPaymentCashAndAttributionWarning`. |
| `TestReplaceShortSaleAfterTheExDateNamesTheDependentPaymentInLieu` | `TestReplaceShortSaleAfterExDateLeavesPaymentCashAndWarning`. |
| `TestBackdatedShortCoverBeforeTheExDateNamesTheDependentPaymentInLieu` | `TestBackdatedFullShortCoverLeavesPaymentEffectiveWithWarning`; insufficient-unit cover still refuses unchanged. |
| `TestReplaceShortCoverBeforeTheExDateNamesTheDependentPaymentInLieu` | `TestReplaceShortCoverBeforeExDateLeavesPaymentEffectiveWithWarning`. |
| `TestShortSaleReversalRechecksPaymentInLieuDependencyInsideTheWrite` | `TestShortSaleCorrectionPreparedBeforePaymentDoesNotNeedAttributionPermission`. |
| Browser “a holding that was not short refuses…” | Real accepted cash with named diagnostic; six-locale journeys additionally record unassigned cash, assign and clear attribution, rerun self-check and follow its named transaction link. |

#379's density fixture no longer corrupts facts to bypass entry. It records
unsupported attribution through ordinary commands, retains independent
constructed quantities/currencies and superseded controls, and measures
accepted short reversals instead of obsolete dependency refusals. It verifies
unchanged payment cash and durable rollback for every preview. Between reversal
samples it reopens the position outside timing, restoring the same financial
support before the next sample.

## Measurements

The manual runs are sequential, without concurrent race/browser/build work.
Fixture construction is outside timed operations. Writer figures are medians
of three runs; allocations are mean cumulative Go KiB per command. Historical
#379 figures used five runs (with the documented 10-payment exception and host
load caveats); they are a baseline, not a controlled benchmark speedup claim.
The full operation/read/build log is retained in
[payment-attribution-repair-2026-10-23.txt](payment-attribution-repair-2026-10-23.txt).

| Payments | Unrelated history: before → now | Concentrated history: before → now | New allocation: unrelated / concentrated |
| --- | --- | --- | --- |
| 0 | 18.6 ms → 13.15ms | 27.5 ms → 14.38ms | 121 / 121 KiB |
| 10 | 42.5 ms → 12.87ms | 67.7 ms → 13.37ms | 131 / 124 KiB |
| 100 | 604 ms → 15.67ms | 2.37 s → 15.87ms | 200 / 147 KiB |
| 300 | 2.61 s → 25.9ms | 21.1 s → 21.63ms | 334 / 226 KiB |

No new 1,000-payment figure is claimed: the agreed bounded matrix stops at
300 in both shapes. The authorizer control directly verifies removal of
attribution work independently of latency. Other financial/database work can
still grow with the affected history; this is not a constant-time claim.

The full eight-cell write matrix passes in 47.95 s, including fixture builds.
At 300 payments those builds take 10.762 s (unrelated) / 7.560 s
(concentrated); the historical concentrated build took about 73 minutes.
The six-cell diagnostic matrix passes in 7.56 s including construction.
At 100 payments, diagnostic reads take 4.905–5.147 ms / at most 398,792
allocated bytes (unrelated) and 4.791–6.464 ms / 288,160 bytes (concentrated).
These are explicit reads, not added write admission.

The 250,000-posting gate passes: bundle export 11.724 s, 433 MiB cumulative
allocation, 12 MiB additional peak heap; self-check 12.200 s, 137 MiB cumulative
allocation, 39 MiB additional peak heap. Both stay below the unchanged
60 s / 256 MiB peak-heap ceilings. The whole resource gate takes 38.19 s.

Commands:

```sh
REKENRAAM_PIL_DENSITIES=0,10,100,300 REKENRAAM_PIL_REPEATS=3 \
  ./scripts/measure-payment-in-lieu-density.sh
REKENRAAM_PIL_DENSITY=1 REKENRAAM_PIL_DENSITIES=0,10,100 \
  ./scripts/go-backend.sh test -count=1 -timeout=15m -v \
  -run '^TestPaymentInLieuAttributionCostByDensity$' ./internal/app
./scripts/measure-large-book.sh
```

## Validation

- Full `./scripts/test-backend.sh`: formatting, vet and race tests pass,
  including reference-book, financial, recovery/export and source controls.
- `./scripts/test-frontend.sh`: zero check errors/warnings; 56 files and
  727 unit tests pass. `pnpm build` produces the integrated binary.
- 26 browser journeys pass across payment entry/correction/reversal,
  six-locale diagnostics and assign/clear repair at 390 px with accessibility
  checks, Data settings and provider dividends.
- The final focused race run covers the reclassification edge, migration,
  nullable repair, provider identity, HTTP presence and zero-context-read
  controls. Migration head is 23; bundle schema is 16.
- The structural audit passes numbered-file, client generation, duplicate
  documentation/workflow and work-ID checks. Its only findings are the eight
  intentional retired test names dispositioned above. All 223 local links
  checked in changed Markdown documents resolve; `git diff --check` passes.

No live provider credentials or external wire semantics are claimed verified.
GitHub CI runs after push; local completion does not claim that later result.
