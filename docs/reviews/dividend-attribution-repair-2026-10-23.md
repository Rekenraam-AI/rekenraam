# Dividend And Withholding Attribution Repair — 2026-10-23

Delivery evidence for [#393](https://github.com/sergeyfarin/rekenraam/issues/393),
under [ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md) and
the [financial dependency plan](../plans/financial-dependency-simplification-plan.md).
Implementation base: `be38634874d9299072e70f06b7657996da02be52`.
Earlier [#391 inventory](financial-dependency-inventory-2026-10-23.md) remains
pinned historical evidence, including its intentionally failing desired
provider-identity contract at that source.

## Delivered Contract

Cash dividends, embedded withholding and standalone withholding all permit an
unassigned payer without holdings. Payment shape is a typed immutable fact,
independent of nullable payer. Gross income is never created for standalone tax,
known tax is never discarded or treated as unknown, and a missing payer does
not manufacture a position. Required financial accounts/currency/date/components
still pass native validation.

Entry omission/null is unassigned. Replacement omission preserves the preceding
payer, explicit null clears it, and a supplied eligible instrument assigns it.
The API and service reuse the existing nullable-field/patch types; native import
callers carry explicit values through the same command. Supplied zero, wrong
kind/book and invalid identities still refuse. Attribution-only repair appends
one audited inverse/replacement, keeps original facts and source evidence,
changes no lots/gains and leaves neutral cash checkpoints active. Actual amount
changes and reversal retain reconciliation review. Financial date/cash account/
currency remain fixed here; #395 owns those separate corrections.

CSV income profiles may omit the payer column or contain empty payer cells.
Exact known gross/tax/cash still determine native financial classification.
A supplied nonempty unknown source code remains reviewable rather than silently
ignored; supplied instruments still need valid active resolution. Raw rows,
source/commit identity and reimport dedupe survive both unassigned entry and
later payer clearing. Gross-versus-net declarations and source cash validity
remain necessary; no generic cash fallback or guessed economics is introduced.

## Storage, Identity And Visibility

Migration 25 makes the existing dividend fact's payer nullable and records shape
for every native cash dividend/tax operation. It preserves existing facts and
backfills only shape established by recorded gross/withholding components, never
payer from descriptions or amounts. Reversals retain exact nullable predecessor
facts. Immutable triggers, typed reference/audit validation and a structural
valid-facts view retain corruption detection. Populated downgrade refuses when
the former non-null format cannot retain explicit unassigned facts; no history
is erased to make downgrade succeed.

A shared cash-payment identity view follows immutable roots and journal links
for both dividend and payment-in-lieu families. Candidate discovery and commit
exclude a payment already generated or linked to another event across original,
inverse and replacement versions. Storage triggers guard both suggestion
columns atomically. They do not infer duplicate payments from matching amounts
or dates: another real unclaimed payment remains linkable. Ordinary cash writes
do not evaluate provider attribution for authorization.

For retained provider claims, a different known payer is a scoped warning;
clearing the payer makes current attribution unavailable with no invented
contradiction. Historical source payer/evidence and the original settled
suggestion remain unchanged. Manual unassigned dividends/tax appear in existing
income coverage without acquiring a source-agreement or historical-holdings
workflow. Detail and the existing self-check share the source diagnostic.
Missing/malformed shape facts, broken conservation or duplicate payment identity
remain integrity findings, separate from attribution uncertainty.

Legacy duplicate source claims are preserved across migration and exposed by
foundation self-check; migration does not choose which owner decision to erase.
This slice prevents new claims and does not add an automatic source-unlink or
reclassification command for already damaged books.

All six locales expose optional payer entry and assignment/clearing for every
cash shape. Existing provider history retains its original structured evidence
and links back to the original payment/correction chain. The evidence display
is shared between pending and settled suggestions. Cash/reinvestment entry
modals now bind their existing translated headings as accessible names; this
was exposed by the new mobile accessibility journey.

Bundle schema **17** retains the dividend CSV columns and emits a blank payer
for unassigned facts while retaining `withholding_only`. Other financial/source
CSV columns retain their meaning. The SQLite backup carries migration 25 and
restores through the normal integrity/migration path. Saved report evidence and
historical review artifacts are unchanged.

## Independent Figures And Named Controls

| Scenario | Independent result and controls |
|---|---|
| No lots; gross 100, embedded tax 15, standalone tax 2; both payers unassigned | Cash `100 - 15 - 2 = 83`, income -100, tax 17, zero acquisitions. Coverage keeps standalone gross absent and withholding known. `TestSeparateWithholdingWithoutPayerPreservesShapeAndExactCoverage`. |
| Assigned → omitted → cleared → assigned → omitted → cleared for each cash shape | Gross-only cash 100/income -100; embedded-tax cash 85/income -100/tax 15; standalone-tax cash -2/tax 2/no income. Every preview rolls back, each repair preserves the active cash checkpoint and original payer; real reversal still requires review. `TestDividendPayerAssignClearAndOmissionPreserveReconciledAmounts`. |
| Actual JSON entry omission/null and replacement value/omission/null in all shapes | Explicit null and omitted field have different effects after assignment, including repeated corrections. Shape, financial terms and native read models agree. `TestDividendAttributionHTTPDistinguishesOmissionNullAndValueForEveryCashShape`. |
| Late null-fact insertion failure, stale native correction, missing shape | No partial journal/facts/audit on refusal; missing facts are detected independently of balance. `TestDividendAttributionLateFailureAndMissingShapeAreAtomicAndDetectable` plus existing native correction/rollback controls. |
| Generated or manually linked provider dividend 16, multiple owner payer repairs | Cash unchanged, original raw/normalized evidence and settled event/transaction IDs unchanged. Known mismatch warns; clear is unavailable; original payer restores agreement. Direct second claims on original/inverse/descendants refuse; an unrelated unclaimed payment succeeds. `TestDividendSourceAttributionRepairPreservesEvidenceAndWholePaymentIdentity`. |
| Two concurrent provider claims for one payment | Exactly one claim commits. `TestDividendProviderConcurrentClaimsCommitOnce`. The original retained #391 duplicate-link characterization becomes `TestDividendProviderIdentityExcludesCorrectedPaymentDescendants`, a production refusal regression. |
| Old schema and old duplicate source claims | Existing source/journal/fact/audit rows survive upgrade unchanged; only unambiguous missing shape is backfilled. Duplicate old claims remain and self-check names the trust failure while balance/replay checks pass. `TestDividendAttributionMigrationPreservesHistoryBackfillsOnlyRecordedShapeAndRefusesLoss`, `TestDividendAttributionMigrationPreservesAndDiagnosesLegacyDuplicateSourceClaims`. |
| Frozen baseline book | Exactly one unassigned gross fact `[operation 4, payer null, shape 0, original audit 33]` is added from recorded components. All prior financial figures remain identical; fresh and upgraded schema match. `TestMigrateSeededBaselineMatchesFreshSchema`. |
| Native imported dividend corrected to 5.23, then payer explicitly cleared | Original source stays attached to the original operation; refetch remains duplicate, creates no payment, and the cleared operation remains reversible. `TestImportedDividendCorrectionPreservesCommittedIdentity`. |
| CSV missing payer column / blank cells, 100 gross less 15 tax plus 2 separate tax | Cash 83, income -100, tax 17; two nullable facts retain both shapes; preview/commit and reimport identity agree. Unknown nonempty source codes and inconsistent amounts still refuse. `TestCSVIncomeMissingPayerPreservesKnownAmountsAndImportIdentity`, existing CSV unresolved/economics controls. |

## Test Dispositions And Validation Boundaries

The two primary regressions and the CSV missing-payer controls were confirmed
failing before their fixes. No financial invariant test was removed.
`TestSeparateWithholdingRequiresInstrumentAndTaxAccount` is renamed to
`TestSeparateWithholdingRequiresTaxShapeAndValidSuppliedPayer`, replacing its
former missing-payer
refusal with a supplied-invalid-payer control; missing-payer acceptance is
covered explicitly. Fact-count and bundle-version assertions now describe
nullable shape facts/schema 17. The historical seed and schema inventory name
the new view/trigger objects rather than accepting arbitrary count drift.
A cash-parent migration test restores the head after a partially completed
multi-migration downgrade refusal before invoking current self-check readers.
The dividend correction-term equality test includes its new diagnostic field.

Early browser assertions assumed unnormalized coefficients and fixed displayed
trailing zeros; they were corrected to compare exact scaled values and the
existing formatter's output. Mobile tests exposed the real unnamed-dialog defect,
which was fixed in the shared dividend form/entry wiring. An overlapping integrated
build replaced embed assets while a backend gate compiled them; another overlapped
run hit the existing package deadline. Final validation uses a completed build
and stable assets, with the standard two-package concurrency and five-minute
deadline unchanged (application package 287.960 s, database package 221.591 s). No test timeout, resource
ceiling, financial guard or browser accessibility rule was relaxed.

- Full backend formatting/vet/race gate passes at the existing deadlines.
- Final frontend check has zero errors/warnings; all 727 unit tests pass.
- Integrated static frontend/Go binary build passes; 36 browser journeys pass,
  including three cash shapes in all six locales at 390 px with accessibility,
  real provider mismatch/unavailable repair and retained evidence, CSV mapping/
  missing-payer imports, coverage, native correction and imported repair.
- The 250k-posting gate passes: export **11.669 s / 20 MiB** additional peak Go
  heap; self-check **13.514 s / 39 MiB**. Builds and fixture construction are
  outside the operation measurements.
- Writer-density correctness controls pass at **0/10/100** in both unrelated and
  concentrated shapes, three samples per operation; build timing stays separate.
  At 100 concentrated payments the unrelated write median is 15.67 ms. These are
  bounded measurements, not a general latency guarantee.
- Generated OpenAPI drift, schema/ADR numbering, duplicate documentation/workflow
  and ID checks pass. The structural audit reports exactly the documented old
  policy-test name as missing; its replacement retains the financial controls
  and the rename is explicitly reviewed, rather than hidden by an alias or skip.
- Document links, named test references and `git diff --check` pass.

[Retained validation output](dividend-attribution-repair-2026-10-23.txt) records
commands, observed regressions and successful final gates. Race mode instruments
Go application/driver-wrapper code, not SQLite's C engine. Native-language copy
review remains separate. This delivery does not implement #394/#395 or complete
#390/#284; no new relationship engine, repair queue or loan subsystem is added.
