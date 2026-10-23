# Financial Dependency Simplification

Owner direction: 2026-10-23, governed by
[ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md).
This is a design and delivery-boundary plan. The first four-case comparative
contract below is complete; the wider family inventory and runtime delivery
remain open. Runtime baseline: `8549045a4484f01e6e9b28e4f0a5fd6aa8213985`.

Delivery acceptance and progress live in
[#391](https://github.com/sergeyfarin/rekenraam/issues/391). The broader
product-complexity audit is [#390](https://github.com/sergeyfarin/rekenraam/issues/390);
#284 retains its remaining behavior-preserving maintainability work. Overall
work order remains in [the roadmap](../roadmap.md).

## Review Unit And Decision Test

Review a relationship and its user workflow, then trace the relevant code.
For each relationship, record its meaning and authoritative inputs; current
entry/edit/reverse/replace/link restrictions; UI, service, writer, storage,
report/export and diagnostic consumers; protected outcome; a counterfactual
with independent expected results; and a retain, recompute, reassign, diagnose
or remove disposition. Repeated checks may protect different admission paths.

For a retained hard dependency, name what becomes invalid if it is removed.
Balances, quantities, basis/elections and allocations are central; correct
report classification/coverage, reconciliation, immutable evidence and source
identity are also examined. A field used only for explanation is a candidate
for versioned attribution, not an automatic veto. Security and operational
protections are assessed against their own threat/recovery contracts.

## Family Coverage And Starting Hypotheses

The entries below guide the inventory; they are not completed dispositions.
The identified current mechanisms come from the baseline code and active
contracts. Keep or reject is a valid result for any unproven simplification.

| Family | Starting boundary and required examination |
|---|---|
| Payment in lieu of dividend | Cash/expense/recovery has no lot effect. Retire historical-short authorization in favor of repairable attribution and visible diagnostics, consistently across entry/replacement and subsequent position edits (#388/#389). |
| Fractional-share cash in lieu | Ordinary long disposal plus fixed `settled_operation_id`. Trace `requireNoCashInLieuSettlesTx` in split, share-exchange, spin-off and cash-and-stock-merger corrections. Distinguish real disposal/allocation inputs from provenance. |
| Dividend and embedded/separate withholding | Existing cash and owner-confirmed components remain; compare instrument/ex-date/holding attribution and correction with payment in lieu. Do not invent unknown gross or tax, or add lot dependencies for cash-only classification. |
| Reinvested dividends | Income and acquisition both have financial effects. Preserve the acquisition's lots/basis and atomic composition; compare its attribution repair with ordinary dividends. |
| Fees and interest | Compare cash/expense postings, recovery and optional instrument/period attribution. A capitalized trade fee is a calculation input, unlike an explanatory note. |
| Return of capital and basis actions | Entitled quantities and basis reductions/excess are calculations. Trace required inputs, known/unknown propagation and replay before relaxing any lineage. |
| Disposals, covers and transfers | Keep actual quantity, basis, elections, side and bridge/settlement conservation. Use as negative controls for the cash-only simplification; unchanged aggregate cash is insufficient. |
| Provider/import links and corrections | Separate a suggested association from source identity, dedupe and ordered effects. Repair cannot post twice, erase raw evidence or undo a settled owner decision. |
| Prices, derived observations and saved reports | Distinguish live derivation dependencies from immutable historical report inputs. Preserve reviewed consequences and saved bytes; determine whether an edit threatens a real commitment rather than extending financial blocking by analogy. |
| Reconciliation and order | Preserve genuine balance/boundary and reviewed-set commitments; compare metadata-only edits and existing command-netted behavior. A confirmation must protect a concrete consequence. |
| Mortgage/loan payment comparison | Current liability postings plus owner-supplied principal/interest split can express a payment. Distinguish the liability account from optional origination/schedule links. No loan feature or schedule engine is introduced here. |

## Contract Stages

These are prerequisites within a slice, not a second application work queue.

1. **Meaning and necessity.** Complete the family row with all actual consumers,
   a valid-correction example and an invalid counterexample. Reuse #284's 34
   rule groups and existing route/sub-view maps for navigation. Trace only
   affected consumers deeply; record limits rather than claiming every LOC
   reviewed. Absence of a relationship is also a disposition.
2. **Financial and repair contract.** State required versus optional inputs,
   contradictory versus unavailable attribution, diagnostic surface/severity
   and discovery, versioned repair, provider/manual behavior, report/export
   coverage and API/schema compatibility. An integrity failure stays separate
   from an otherwise valid cash record with unsupported explanation.
3. **Bounded implementation.** Reuse the existing writer and exact arithmetic.
   Specify the smallest change and its retained guarantees before coding.
   Original evidence survives; attribution repair preserves cash. A compound
   financial correction validates the complete result and atomically commits
   journals, subledger, audit/source effects and required reviewed impacts.
4. **Verification and handoff.** Record independent quantities/amounts and
   before/after journal, lot, allocation, source, audit and checkpoint outcomes.
   Preserve relevant negative controls and the reference book. Tests pinning
   a retired policy change alongside the decision; they do not establish that
   the old policy must remain. Update shipped documentation only on delivery.

The payment row can proceed without the loan, price or wider #390 rows. #389's
diagnostic can be developed against explicit fixtures before #388 relaxes
writer admission; visibility ships first or with that relaxation. This avoids
a circular dependency and a period of silently unsupported classification.

## Cash-In-Lieu Characterization

Cover every supported parent type and native reversal/replacement. A parent
correction that leaves the fraction disposal valid is the simplifying case;
a correction removing necessary units or invalidating a specific-lot election
is the retained-protection control. Also exercise basis-changing parent
corrections and same-day/correction-root order, including subsequent disposals.

Determine whether the settlement link supplies quantity, account/instrument,
date/order or basis inputs, or merely identifies the source explanation. A
historical FK may remain as original evidence while effective attribution is
reassigned/versioned. Do not weaken a constraint before its readers and
derived effects have a replacement contract. #391 files a bounded delivery
issue once these decisions and named tests are ready; it does not pre-approve
deleting all parent fences.

## Four-Case Contract (2026-10-23)

The [comparison report](../reviews/financial-dependency-comparison-2026-10-23.md)
retains source consumers, independent figures and a reproducible isolated
experiment. These decisions apply to the four compared cases; other family
rows above remain hypotheses. Existing runtime contracts change only when
their linked implementation ships.

| Case | Required financial meaning | Attribution and correction disposition | Delivery |
|---|---|---|---|
| Cash dividend | Exact gross, recorded withholding, cash/currency/date and posting accounts; gross is never inferred from net alone | Payer is optional, set/change/clear is audited, and holdings are not permanent authorization. Preserve the cash-only correction shape and existing coverage. | #393 closes the clear/unassigned gaps; no new holding/ex-date fields. |
| Embedded/separate withholding | Exact known tax and correct posting shape; a separate tax payment creates no gross income | Instrument is optional in both forms. Separate-withholding knowledge must survive without a payer-facts row carrying a non-null instrument. No historical lot or mandatory dividend-payment link. | #393 changes the facts representation with its migration. |
| Payment in lieu of dividend | Exact owed cash/expense, optional recorded recovery, payment date/currency/accounts; no lot effect | Instrument, holding and ex-date are optional owner attribution. Remove entry/replacement entitlement authorization and the shared writer's global before/after scan. Unsupported attribution is diagnostic; allow audited repairs preserving cash. | Delivered by #388/#389; other families remain separate. |
| Fractional-share cash in lieu | Actual fractional disposal quantity, account/instrument, basis/election, proceeds and disposal/payment dates | Keep immutable parent provenance; validate parent edits by actual replay. Resolve correction ancestry for current context. A retired parent alone is an attribution warning, while impossible disposals still refuse. | Delivered by #392 across all four parent families; original evidence and real disposal guards remain. |

### Required Facts And Repair Semantics

Cash-only commands always require owner-confirmed financial magnitudes and
valid cash/tax/income/expense accounts, currency and payment date. The payment
in lieu recovery retains its existing positive, same-currency, less-than-owed
contract. Supplied attribution IDs must name the proper object kind in the
same book; supplied dates must be calendar dates. Holding attribution is not
a security posting and does not authorize one. Missing attribution cannot
block an otherwise supported cash-only command or silently choose an account,
instrument, ex-date, gross amount or tax treatment.

For cash-only attribution fields, entry omission/null means unassigned;
replacement omission preserves the prior value, explicit null clears it and
a supplied value replaces it. Presence must survive HTTP decoding. A supplied
holding with no instrument is retained as partial attribution rather than
inventing a payer. A missing field means unavailable historical support; a
complete ex-date after the recorded payment is a chronology warning rather
than a reason to erase/refuse the known cash. Currency/payment-date/posting
account corrections remain financial operations governed by existing rules.

Use the current immutable facts and replacement/correction chain, extending
the typed facts with nullable attribution fields and their guards. Every named
payment keeps its own facts row; missing facts rows, cross-book references and
malformed stored values remain integrity failures. Preserve historical values
and repeated reversal facts. For separate withholding, retain payment shape
independently of payer presence. Migrations preserve existing known values;
new export manifests declare nullable attribution/shape representation and
leave unknown cells empty. No generic relationship table is needed.

One owner action previews and commits an attribution repair. Reuse the
audited inverse-plus-replacement mechanism with the same payment date/cash
account/currency. Equal financial legs net to zero; all affected checkpoint
balances and gains remain unchanged, so an attribution-only repair needs no
reconciliation invalidation. Show a correction, not a broker refund followed
by a new charge. A real amount/account/date change still follows normal
financial review. Reversal does not require supported attribution.

Owner attribution and source statements remain distinct. A repaired payment
does not post again on reimport. A provider event link keeps its original
evidence; when effective owner attribution no longer matches it, disclose the
disagreement without rewriting the event, reopening its decision or admitting
a second source identity. An explicit relink still validates book, effective
payment and once-per-payment identity atomically. Reports continue using
recorded amounts and existing scope/formulas; diagnostics do not reclassify
payment-in-lieu expenses as dividends or claim to verify the charged amount.

### Diagnostic Contract For #389

Reuse the existing Data settings self-check and transaction detail/correction
surface. Add a read-only `investment_attribution` check and one `warning`
severity, keeping integrity failure and unavailable coverage meanings intact.
No new review queue, persistent acknowledgement, worker or mandatory modal is
introduced. The check uses exact effective openings minus covers by cost
currency at the close before the ex-date, grouped/batched where necessary.
Ordinary writes never run this whole-book diagnostic.

| Evaluation | Result and visibility |
|---|---|
| Complete supported attribution | Passed; this establishes historical support, not correctness of the amount. |
| Context needed for the historical check is missing | Incomplete coverage with unavailable count/sample; known cash stays usable. |
| Complete context has no short at the close, or ex-date follows payment | Warning with stable reason (`no_short_at_entitlement_close` / `ex_date_after_payment`), named transaction and repair link. Never label it financial corruption or unknown data. |
| Effective owner attribution differs from linked/generated provider evidence | Warning `provider_attribution_mismatch`; preserve the source, decision and payment identity through correction ancestry. Missing context is still disclosed separately. |
| Broken facts/FKs, invalid quantities/elections or journal/lot disagreement | Existing integrity checks fail independently. |
| Evaluation cannot execute | Existing errored-run/read-error behavior; never substitute unsupported or zero. |

Keep `finding_count`/`sample` for integrity findings and
`unavailable_count`/`unavailable_sample` for missing coverage. Add distinct
`warning_count`/`warning_sample` fields and `warning_check_count` at run level;
each attribution warning sample is `{transaction_id, reason}`. Unavailable
samples for this check also identify transactions, not operation IDs.
Aggregation is failed > warning > incomplete > passed after successful
execution; warning and unavailable counts stay visible even alongside failure.
Saved runs retain their original results/timestamps. Changes require a
self-check storage migration, OpenAPI/generated types, translated status/copy
and Data settings rendering; add no universal severity/rule registry.

Warnings and missing-context rows link to
`/app/transactions?transaction_id=...` and the existing replacement form's
editable attribution. The effective payment terms read supplies the same
diagnostic on demand for detail/repair; it is advisory, never write permission.
After repair, refresh that detail and allow rerunning the self-check; its last
saved run is explicitly historical. Do not invent support for dividend
eligibility where holding/ex-date data is not recorded. The #392 parent-link
warning can use this same surface once its source-resolution contract ships.

### Implementation Boundaries

#388 and #389 now have their targeted contract; neither waits for every other
#391 family or #390. They may ship together, or #389's read/visibility first
against explicit fixtures. Missing attribution fields and contradictory
historical support are accepted only alongside this visibility. #392 and #393
are separate bounded changes with their own negative controls. The wider
family inventory remains in #391; this slice does not complete that issue.

### Read-Side Delivery (#389, 2026-10-23)

The first runtime slice implements the shared dated-short diagnostic, saved
warning outcomes (migration `0022`), API/generated types and translated Data
settings/detail visibility. It removes historical eligibility from the
foundation integrity check while retaining structural facts validation.
Effective covers and corrections are read through `EffectiveLotMovements`;
one dated sweep serves the book's payments. The reader uses no savepoint or
write admission and is not called by investment commands.

At this first diagnostic delivery, nullable entry, omission versus clearing,
audited attribution repair and removal of write blockers remained with #388.
The cash-attribution delivery below completes that boundary; unsupported
histories now use ordinary owner commands in integration fixtures.
At that diagnostic-first checkpoint, the broader #390/#391 inventory and
#392/#393 remained open. The subsequent delivery sections record runtime completion.

### Cash-Attribution Delivery (#388, 2026-10-23)

#388 now completes the payment row and #389's repair integration. Entry may
leave attribution unassigned; replacement preserves omission, clears explicit
null and accepts supplied values. The existing nullable-field primitive is
shared with recurring requests. Amount, payment date, cash/currency/expense
roles and recovery constraints remain required. Attribution references need
the correct same-book object kind, without historical posting eligibility.
No instrument or ex-date is inferred from a holding or today's date.

Migration `0023` preserves immutable originals/reversals while admitting SQL
NULL attribution. A shared structural read view serves integrity and advisory
readers; typed payment ancestry preserves source identity across corrections.
An attribution-only repair uses the existing inverse/replacement action with
one audit event, zero checkpoint-balance change and no gain/lot effect. It is
labelled a correction. Source disagreement is visible and never erases raw
evidence, reopens a decision or permits relinking another version of the same
payment. Bundle schema 16 exports unknown cells empty.

The local entitlement gate and shared writer's before/after enumeration/replay
are removed, rather than replaced with another whole-book write check.
SQLite's independent FK checks remain; a native authorizer test refuses all
attribution-context reads during ordinary investment writes/previews. The
existing six-locale entry/correction/detail and Data settings surfaces carry
the workflow, with no new queue or acknowledgement requirement.

The [delivery evidence](../reviews/payment-attribution-repair-2026-10-23.md)
records policy-test dispositions, retained financial controls and bounded
measurements. This does not complete #390/#391 or relax #392's real disposal
constraints. #393 still owns the dividend/separate-withholding nullable payer
representation and clear workflow.

### Cash-In-Lieu Parent Delivery (#392, 2026-10-23)

Split, share-exchange, spin-off and cash-and-stock-merger reversal/replacement
now use existing typed disposal replay, without `requireNoCashInLieuSettlesTx`.
Original `settled_operation_id` stays immutable and exportable; replacement
ancestry supplies current explanatory context. New entry requires a current
matching parent. Existing disposal correction/dated lot selection remains
usable after parent replacement or reversal and keeps the original reference.

Current parent missing after reversal is `cash_in_lieu_parent_unresolved`;
current holding/instrument/date mismatch is `cash_in_lieu_parent_mismatch`.
Both use the existing investment-attribution self-check/detail warning surface.
Historical malformed/missing/cross-book facts remain integrity failures.
No attribution scan is added to ordinary writes. No new workflow, generic
relationship table or dependency engine is introduced. Schema migration 24
updates the fact guard and typed read views without changing financial rows;
bundle schema 16 and all CSV columns remain compatible.

The independent €36 / 7.5 × 0.5 = €2.40 example, 0.3 < 0.5 refusal,
all-family replacement/reversal, specific-lot/later-sale, same-day root order,
gain/checkpoint binding, rollback, migration and owner-surface controls are
recorded in [delivery evidence](../reviews/cash-in-lieu-parent-replay-2026-10-23.md).
#393 and the remaining #391/#390 inventory stay open.

## Whole-Application Complexity Review

#390 maps all capability families from the feature ledger and existing entry
point inventories before sampling depth. The first pass is capped at six
workflows, twelve named rules and five actionable proposals, with reviewed,
sampled and deferred coverage explicit. Start with ordinary correction,
cash-only classification, corporate-action/fraction disposal, import repair,
reconciliation and planning. Expand along implicated consumers only.

For each proposal, show the user's task before and after, its protected
outcome, steps/confirmations/reversal chains, implementation/change cost and
the consequence of doing nothing. Consider deleting behavior or composing
existing primitives as well as extracting code. A clean implementation of an
unnecessary rule is still unnecessary; an unfamiliar financial case or
independent storage defense is not waste just because it is verbose.

Use the existing reports and hand-derived reference figures; add a disposable
probe only to decide a concrete hypothesis. Keep a concise set of retained
complexity and rejected simplifications. Do not build a universal rule engine,
require zero warnings, measure success by deleted LOC, or make an exhaustive
new audit a prerequisite to every local improvement.
