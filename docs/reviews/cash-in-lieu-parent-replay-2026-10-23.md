# Cash-in-lieu parent replay — 2026-10-23

Delivery: [#392](https://github.com/sergeyfarin/rekenraam/issues/392), under
[ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md) and
[the family contract](../plans/financial-dependency-simplification-plan.md).
Starting source: `9e5b8a26cc4fda5f2022b7fe3d9d0b738cd6d1a4`.
The broader #391/#390 review and cash-dividend/withholding payer work (#393)
remain open. This is not a new corporate-action engine or provider adapter.

## Result and scope

A parent correction now succeeds when the existing fractional-share disposal
is financially valid. The four calls to `requireNoCashInLieuSettlesTx` and the
blanket fence itself are removed from split, share-exchange, spin-off and
cash-and-stock-merger correction. Native typed replay still validates actual
quantities, elected lots, basis, same-day/correction-root ordering and later
disposals. Real impossibility names the downstream operation and writes nothing.
Changes to realized results still require the exact gain acknowledgement;
reconciled balance changes still require the exact checkpoint review.

`settled_operation_id` remains immutable original provenance. A typed read view
follows only supported replacement ancestry to current explanatory context.
It never associates the disposal with an unrelated action. Reversal leaves no
current explanatory parent. A second typed view checks historical reference
integrity separately from that explanation. No parent or cash-in-lieu financial
rows are rewritten by the schema migration.

New cash-in-lieu entry still requires a current matching parent. Existing
cash-in-lieu replacement retains its original parent after that parent is
replaced/reversed; the trigger verifies inheritance through the existing typed
correction chain. Dated available lots retain the disposal's original root slot.
The form can correct proceeds or elections using those lots after parent repair.

The existing `investment_attribution` self-check/detail surface adds:

- `cash_in_lieu_parent_unresolved`: no current parent after reversal.
- `cash_in_lieu_parent_mismatch`: current parent holding/instrument/date no
  longer explains the recorded disposal.

Both are advisory warnings. Real missing/malformed/cross-book original facts
remain foundation failures. The reader uses a query-only connection and never
runs inside ordinary investment write admission. No new queue, worker, modal
or acknowledgement is introduced. Split activity enables native correction
rather than requiring a cash-in-lieu reversal first. Six locale catalogs cover
those warnings, the changed split hint and a current-parent link in the
existing cash-in-lieu correction form.

Migration 24 preserves old facts and immutability. Safe downgrade before parent
correction preserves exact rows; downgrade refuses when the old runtime would
misclassify a still-effective disposal as corrupt. Bundle schema 16 and all CSV
columns remain unchanged. The original parent ID/payment date and complete
operation correction ancestry remain exportable; the bundle README explains
how to interpret them after parent repair.

## Independent accounting and controls

The original failing test reproduced the unnecessary split veto before any
production change: 3 units bought for €36, a 3:2 split and a 0.5-unit disposal
for €8 could not preview a 5:2 replacement. The delivered command previews and
commits with exact gain review:

- New quantity before disposal: 3 × 5/2 = 7.5; remaining: 7.
- Disposed basis: €36 / 7.5 × 0.5 = €2.40; remaining basis: €33.60.
- Net cash: −€36 + €8 = −€28. An active cash checkpoint stays active.

The all-four-parent matrix independently expects these revised half-unit
bases: split/share exchange €2.40; spin-off €0.48 (20% of €36 carried);
merger €1.80 (75% of €36 carried). Remaining quantity is 7 in each destination.
With a later one-unit sale in the same day, its basis is twice the half-unit
basis; remaining quantity is 6 and both disposals plus remaining basis conserve
the original allocated basis. These expected figures do not come from replay.

Every family has named replacement, reversal and refusal cases in
`investment_cash_in_lieu_parent_test.go`:

- Valid reversals use original units (split) or independent native acquisitions
  of the destination security. The recorded cash-in-lieu survives and warns;
  its subsequent specific-lot selection and correction remain usable.
- 0.3 units cannot satisfy a 0.5 disposal after a 1:10 replacement. Preview and
  commit refuse with the same durable snapshot in all four families.
- Other units cannot substitute for an invalidated specifically elected
  corporate-action lot. A split reversal that strands a later whole-unit sale
  names that sale instead. Preview/commit refuse without writing.
- Stale/unacknowledged gain review and injected late revision failures preserve
  journals, lots, allocations, facts, audit/source and checkpoint rows. Real
  holding-checkpoint changes still require the reviewed set; an unrelated set
  refuses as stale. The accepted override invalidates the actual checkpoint.
- Parent date after disposal warns when original units still support the
  disposal. Multiple replacements resolve current ancestry without changing
  the original link; restoring compatible terms clears the advisory warning.
- Query-only reads succeed. Deliberately corrupted original parent facts fail
  foundation checks and are never disguised as attribution warnings.
- Migration round trips preserve exact original facts, immutable update/delete
  guards stay active, and a misleading downgrade refuses without data loss.
- Export controls retain the original parent identity and payment date after
  accepted replacements, across all four families.

## Retired-policy test disposition

The structural audit's missing-name findings are intentional policy changes:

| Former test/workflow | Delivered control |
| --- | --- |
| `TestSplitReversalRefusedWhileCashInLieuSettlesIt` | `TestSplitReversalKeepsValidCashInLieuAndRevisesItsBasis`: original units support the fraction; exact basis and cash are asserted. |
| `TestCorrectingExchangeSettledByCashInLieuNamesIt` | `TestExchangeSettledByCashInLieuReplaysReplacementAndRefusesImpossibleReversal`: real missing-unit refusal retained; valid ratio correction uses gain review. |
| `TestCashStockMergerCorrectionNamesASettlingCashInLieu` | `TestCashStockMergerSettledDisposalGuardsUnitsAndReviewsBasisCorrection`: real reversal dependency retained; basis correction previews and commits. |
| `TestCashInLieuAPICorrectReverseAndReleaseSplitFence` | `TestCashInLieuAPICorrectReverseAndPreviewValidParentReversal`: HTTP preview validates the disposal instead of parent standing. |
| `TestJournalFreeSplitCashInLieuSettlementAndCorrectionFence` | `TestJournalFreeSplitCashInLieuRetainsIdentityAndReviewsReversal`: retain operation-only parent identity; preview and require the actual changed-gain review. |
| Journal-free split API/mobile old conflict/disabled assertions | Valid reversal previews successfully; commit still requires reviewed gains, and the mobile action is enabled. |
| Mobile “releases its split” journey | Preview a valid parent reversal with the real gain review; retain disposal entry/correction/reversal. Six-locale lineage/warning/repair journeys additionally exercise the full new workflow. |

## Validation and limits

- Full `./scripts/test-backend.sh`: formatting, vet and all race tests pass,
  including the reference book, four-family controls, source/correction suites,
  migration history, self-check, reconciliation and export/recovery cases.
- `./scripts/test-frontend.sh`: zero Svelte errors/warnings, 727 unit tests in
  56 files pass; `pnpm build` produces the integrated application.
- 50 browser journeys pass across cash in lieu, split, share exchange,
  spin-off, cash-and-stock merger and payment in lieu. Six-locale 390 px
  journeys check current-parent links, enabled split actions, warning after
  reversal, dated specific lots, actual cash correction and self-check naming;
  accessibility checks pass on the new diagnostic surface.
- The bounded write-density regression passes at 0/10/100 in both history
  shapes, three samples per operation (15.07 s including construction).
  This checks that unrelated investment commands retain the prior simplified
  admission behavior; it is not a dense-corporate-action benchmark.
- The sequential 250,000-posting gate passes: export 11.677 s / 433 MiB
  cumulative allocation / 20 MiB additional peak heap; self-check 12.367 s /
  137 MiB allocation / 39 MiB additional peak heap. Both remain within the
  unchanged 60 s / 256 MiB peak-heap ceilings. Whole gate: 39.05 s.
- The structural audit passes migration/ADR numbering, generated API,
  duplicate paragraph/workflow and work-ID checks. Its only findings are
  the six retired names dispositioned above. Local Markdown links resolve,
  all named evidence tests resolve to current or explicitly retired tests,
  and `git diff --check` passes.

Resource commands and full figures are retained in
[cash-in-lieu-parent-replay-2026-10-23.txt](cash-in-lieu-parent-replay-2026-10-23.txt).

No verified live provider wire behavior, general metadata reassignment,
nullable parent identity or new tax treatment is claimed. Genuine allocation
and quantity lineage remain financial dependencies. GitHub CI runs after the
requested push; local results do not claim that later verdict.
