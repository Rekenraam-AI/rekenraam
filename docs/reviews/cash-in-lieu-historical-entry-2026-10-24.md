# Historical Cash-In-Lieu Entry (#394)

Delivered under ADR 0019 and the financial-dependency simplification plan.
Entry, preview and lot lookup share correction's original-parent policy for
splits, share exchanges, spin-offs and cash-and-stock mergers. They keep exact
original provenance and validate the actual dated disposal through the existing
writer. Historical parent standing is no longer an entry authorization gate.

## Independent accounting and financial controls

The retained reversed-split probe was first run as a failing regression:
3 units bought for €30, split 3:2, reverse the split, then dispose 0.5 for €8.
The old service refused preview solely because the parent had a successor.
`TestCashInLieuOfReversedSplitUsesActualDisposalReplay` now expects 2.5 units,
€25 remaining basis, €5 disposed basis, €3 gain and −€22 cash. It replaces
`TestCashInLieuOfReversedSplitIsRefused`; whole-unit, original date and actual
quantity/election refusals are preserved in separate named tests.
`TestCashInLieuRefusesSettlingAReversedExchange` is replaced by
`TestCashInLieuOfReversedExchangeRefusesMissingUnits`: its original fixture has
no independently acquired destination units, so preview and commit still refuse,
now specifically for insufficient units, with an unchanged durable snapshot.

`TestCashInLieuHistoricalParentEntryAllFamilies` independently checks both
historical standings in all four families. Replacement cases buy 3 for €36,
replace the 3:2 action by 5:2, then dispose 0.5 for €8: 7 units remain.
Disposed basis is €2.40 for split/exchange, €0.48 for a 20% spin-off and €1.80
for a merger carrying 75% of basis. Remaining basis plus disposed basis equals
the allocated original basis. Reversal cases use original units for a split,
or 2 independently acquired destination units costing €20 for the other
families. The half-unit basis is respectively €6 or €5. Explicit specific-lot
choices use dated available units, and preview allocations equal committed ones.
Original parent and payment date survive bundle export.

`TestCashInLieuHistoricalEntryKeepsFinancialRefusalsAndRollsBackLateFailure`
leaves exactly 0.5 units after each parent's replacement: an attempted 0.75,
whole, zero or negative quantity, invalid dates/elections and an unrelated
operation refuse without changing the complete durable snapshot. An injected
late fact-insert failure rolls back journals, lots, facts, observations, source,
background work and audit. `TestHistoricalCashEntryStorageKeepsOriginalParentIntegrity`
exercises late typed-parent/book/instrument guards through the disposal writer.

`TestHistoricalCashEntryBindsActualGainAndCheckpointImpact` requires the actual
checkpoint set and changed downstream-sale gain acknowledgement. Missing and
stale review write nothing; accepted review invalidates the actual checkpoint.
The later four-unit sale takes 2.5 units at €10 plus 1.5 at €24: €61 basis.

## History, API and browser controls

`TestHistoricalCashEntryMigrationChangesAdmissionWithoutRewritingHistory`
round-trips migrations 25/26 in all four families. Old admission refuses new
historical entry; upgrade permits it without rewriting any facts. Downgrade
restores admission safely while preserving historical facts readable by v25.
Immutable update/delete guards and the older misleading-downgrade refusal stay.

`TestHistoricalCashInLieuAPIEntryPreviewCommitAllParents` runs actual authenticated
HTTP lot lookup, preview and commit for every family/standing, independently
checks basis/gain, preserves the exact original reference and exposes selected
historical scope in the existing history response. Inverse journals have no
entry context. Existing current-parent, correction, source and export suites
remain part of the backend gate.

The cash-in-lieu browser suite exercises mobile entry after both replacement and
reversal in all four families in English, plus split history/transaction entry
in all six locales. It checks the translated historical notice, dated specific
lots, independent expected basis, preview-versus-commit allocations, original
identity, warning after reversal, phone overflow and accessibility. Existing
six-locale correction/lineage journeys remain.

## Validation

- The focused historical-parent service and real HTTP tests pass. The retained
  reversed-split regression was observed failing before the production change.
- `./scripts/test-backend.sh`: formatting, vet and the full race gate pass,
  including application (293.507 s) and database (226.215 s) packages under the
  unchanged five-minute package deadline.
- `pnpm build` produces the integrated application and passes the embed test.
- `./scripts/test-frontend.sh`: zero Svelte errors/warnings and 727 unit tests
  in 56 files pass; catalog parity and OpenAPI generation pass.
- 60 focused browser journeys pass across cash in lieu, splits, share exchanges,
  spin-offs, cash-and-stock mergers and unknown-basis non-sale actions, including
  the new historical-entry matrix and retained six-locale correction journeys.
- Admission density passes at 0/10/100 payments in both history shapes, with
  five samples per operation (26.785 s including construction).
- The sequential 250,000-posting gate passes: export 19.582 s / 433 MiB
  cumulative allocation / 24 MiB additional peak heap; self-check 13.106 s /
  137 MiB allocation / 39 MiB additional peak heap. The 60 s / 256 MiB
  per-consumer limits remain unchanged. Whole gate: 50.494 s.
- Structural audit: migration/ADR numbering, generated-client drift, duplicate
  paragraphs/workflows and work IDs pass. The two findings relative to the pre-delivery `HEAD`
  are the intentionally replaced reversed-split and reversed-exchange policy
  tests, dispositioned above. Local Markdown links and `git diff --check` pass.

[Full backend and resource results](cash-in-lieu-historical-entry-2026-10-24.txt)
retain the commands and measurements. Resource gates ran sequentially after the
race and browser suites completed.

No nullable parent, automatic sale reinterpretation, new provider behavior or
bundle schema change is introduced. GitHub CI has not been run by this local commit.
