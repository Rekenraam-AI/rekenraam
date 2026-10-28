# Financial Dependency Simplification

Owner direction: 2026-10-23, governed by
[ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md).
This is a design and delivery-boundary plan. #391's cross-family inventory and
contracts are complete at `4ad8373f772eb02b40126f5e9d746f0eb50c9e79`;
#388/#389/#392/#393/#394/#395 are delivered.
The earlier comparative baseline is `8549045a4484f01e6e9b28e4f0a5fd6aa8213985`.

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

## Completed Family Dispositions (2026-10-23)

The [source-pinned inventory](../reviews/financial-dependency-inventory-2026-10-23.md)
records entry/preview/correction/link consumers, authoritative versus explanatory
facts, falsifiable retained dependencies, named controls, independent figures,
consumer consequences and reproducible counterfactuals for every #391 family.
Its sampled probes do not claim all-path runtime acceptance. These decisions
complete the design review; open implementation issues own their own validation.

| Family | Accepted dependency and correction policy | Delivery disposition |
|---|---|---|
| Cash dividend, embedded/separate withholding | Exact cash/gross/tax and payment shape remain required; payer is optional/clearable, without historical share authorization. Preserve known tax independently of payer and unknown imported economics. | #393 delivers nullable facts, audited assign/clear/omit semantics and missing-payer CSV income. #395 owns actual date/account/currency correction, distinct from neutral attribution repair. |
| Dividend payment in lieu | Cash/expense/recovery is authoritative; holding/instrument/ex-date are optional audited explanation, with warning/unavailable support rather than write permission. | #388/#389 delivered; retain diagnostics/source controls. #395 owns remaining financial date/account/currency correction. |
| Fractional cash in lieu, all four parents | Real dated disposal/elections remain required. Original typed parent is immutable provenance; its current standing is advisory on both entry and correction. | #392 delivered correction replay. **#394 delivered** entry, dated lot lookup and insert admission for historical parents; actual disposal replay still refuses impossible quantities. Keep historical reference/type/book integrity, no nullable-parent redesign. |
| Reinvested dividend | Retain real income-funded acquisition and required position/date/currency facts. Their values can change through typed old/new-position replay; required presence does not justify permanently fixed values. | **Accept #395**, consistently across cash dividend/payment and reinvestment fixed fields; valid isolated field-change probes pass, dependent-sale date control refuses. Keep typed identity and actual source/election/checkpoint constraints. |
| Ordinary fees/interest and trade charges | Ordinary postings need no trade/loan link. Retain a trade charge's actual settlement, treatment, dated cash and clearing-component identity; recompute affected allocations. | Keep existing typed policy; no new issue or common related-payment mechanism. |
| Capital return | Receipt, dated entitlement cohort, exact per-lot reduction/excess and basis knowledge are real calculation inputs. Preserve explicit elections; replay supported changes. | Keep allocation lineage. No optional-lot reinterpretation or invented zero basis. |
| Splits/exchanges/spin-offs/cash-stock mergers | Ratios, source/destination quantity/basis, elected fractions/cohorts and root order remain actual effects. Replay supported changes; refuse lost units or owner elections that cannot be reproduced. | Keep actual typed allocations; blanket fraction-parent veto already removed by #392. |
| Transfers, inbound basis resolution and disposal/cover elections | Keep actual quantity/cost movement, chosen lots and bridge journals. Recompute supported basis changes; withdraw effective resolution before transfer reversal. Never silently choose another specific lot. | Keep current dependency and resolution-reversal workflow; a composed command has no demonstrated need here. |
| Provider/import evidence and correction identity | Raw evidence and settled owner decisions remain immutable; owner attribution can change. One payment identity spans its correction ancestry. Mismatch is visible without rewriting evidence or re-authorizing duplicate effects. | #393 delivers shared dividend/payment identity, candidate/storage guards and source/payer diagnostics. Original/inverse/descendant claims stay protected. No separate ticket per check. |
| Prices/derivations | Keep derivation legs and atomic bounded retirement when input is voided; recompute live valuation. Quotes do not authorize journals. | Keep current cascade controls; no cash-correction veto. |
| Saved reports | Keep snapshot/profile/input/output bytes and hashes; later corrections produce a different live report or a new saved run. | Keep immutable evidence; reject freezing the ledger because a saved report refers to it. |
| Reconciliation/order | Review actual changed boundary balances/sets and gain allocations, with stale-plan protection. Net compound effects per account/commodity/checkpoint; preserve neutral checkpoints and root order. | Keep current commitment controls; reject per-leg review of neutral attribution repair. |
| Mortgage/loan comparison | Compose owner-known cash, liability principal and interest as ordinary exact balanced postings. An origination/schedule is not required authorization. | No new loan engine, inferred amortization, special correction family or mandatory schedule. Positive/negative ordinary-payment probe retained. |

### Common Consumer Contract

Use the existing typed command lifecycle, shared writer and audited history.
Services plan; commit revalidates current versions/source identity, runs affected
replay and writes journal/subledger/observations/audit atomically. Preview uses
the same writer with rollback. Bind review only to actual gain/checkpoint
consequences; stale/late failure writes nothing. Attribution-only repair keeps
recorded financial effects and neutral checkpoints.

Cash explanation follows the nullable/omission contract below; supplied typed
identities must still exist in the book. Unknown/unassigned support is unavailable,
known contradictory explanation is a scoped warning, and broken conservation,
malformed facts or missing journals remain integrity failures. Reuse existing
detail/history/self-check and coverage discovery; no new queue or repeated
acknowledgement. #393 shows source/payer disagreement or unassigned attribution
without erasing the historical source claim. A dividend does not assert dated
entitlement, so no new holdings diagnostic is added.

Exact original-currency cash/gross/tax/basis and coverage survive repair and export.
Known unassigned tax is not zero; imported unknown gross/tax and unknown basis
remain unavailable in dependent totals. Preserve immutable original/source/parent
facts and correction ancestry in bundles; version schema meaning compatibly.
Saved reports retain their bytes while live reports recompute. Each runtime
follow-up owns forward migration/downgrade, API presence decoding, localized
mobile repair/discovery, actual preview/commit and failure controls where affected.

#394 replaces the original policy test `TestCashInLieuOfReversedSplitIsRefused`
with `TestCashInLieuOfReversedSplitUsesActualDisposalReplay` and all-four-parent
entry controls for both replacement and reversal. #395 adds
field-change controls without removing family/election safeguards. #393 turns
the audit's observed duplicate-provider-link characterization into a refusal
regression across replacement ancestry. The #391 design audit itself changed no production tests.
#379's historical evidence stays intact; #390 and #284 keep their separate scope.

## Family Coverage And Starting Hypotheses (Historical Baseline)

The entries below record the starting hypotheses at the earlier baseline.
The completed dispositions above supersede them; they are not a live checklist.

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
experiment. At that checkpoint these decisions applied to the four compared
cases; the completed inventory above records the subsequent family dispositions.
Existing runtime contracts change only when their linked implementation ships.

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

At the four-case checkpoint, #388/#389 had their targeted contract without
waiting for every other #391 family or #390; #392/#393 were separate bounded
changes. Missing attribution fields and contradictory historical support were
accepted only alongside visibility. The completed inventory above and runtime
delivery sections below record the subsequent dispositions and shipped scope.

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
measurements. At this checkpoint #390/#391 and the dividend/tax slice remained
open, and real disposal constraints were retained. The subsequent #392/#393
sections record their runtime delivery.

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
At this delivery checkpoint #393 and the remaining #391/#390 inventory stayed
open. The completed #391 inventory above records the subsequent design review.

## Dividend And Withholding Delivery (#393, 2026-10-23)

Native dividend, embedded-tax and standalone-tax entry accept no payer. The
existing nullable-field/patch boundary distinguishes entry omission/null and
replacement omission/preserve, null/clear and value/assign. Shape is independent
of payer in immutable facts; known standalone tax stays tax without gross income
or lots. Neutral repairs use the existing atomic inverse/replacement writer,
retain original/source facts and preserve cash checkpoints. Date/cash account/
currency correction is #395's separate financial-input scope (cash dividends
delivered; see below).

Migration 25 preserves prior facts and backfills only recorded unambiguous shape,
never payer. Nullable exact reversals, valid-facts self-check and explicit
populated downgrade refusal preserve meaning. Bundle schema 17 retains CSV
columns, exports unassigned payer empty and retains withholding-only shape.

The shared cash-payment identity view and candidate/commit/storage guards cover
original, inverse and replacement journals for both dividend and payment in
lieu. A second provider event cannot claim another version of one payment;
another real unclaimed payment remains linkable. Existing legacy duplicate
claims are preserved and reported as integrity findings rather than erased.
Source-linked dividend repair shows a different known payer, or a known ex
date other than the provider event's (its ex date; added 2026-10-26 with
`TestDividendCorrectedExDateContradictingProviderEvidenceWarns`), as a warning,
and cleared payer as unavailable support without inventing a contradiction; a
cleared ex date is likewise unknown, not a contradiction. Manual
unassigned payer is disclosed in income coverage, without a new source-agreement
or holdings workflow. Existing detail/self-check and provider history expose
the diagnosis, original evidence and original payment/correction chain.

CSV profiles may omit the instrument column or contain blank payer cells; exact
known gross/tax/cash still use the same native writer. A nonempty unknown source
code remains reviewable rather than silently ignored. Raw evidence, commit
identity and dedupe survive unassigned entry and later native payer clearing.
This refines #241's missing-payer requirement without guessing economics or
introducing automatic instruments or generic cash fallback.

Six-locale entry/correction and mobile accessibility use existing forms. The
shared cash/reinvestment headings now name their modal entry dialogs. Named
controls, independent EUR 100/15/2 figures, schema/history/source preservation,
API presence, concurrency, rollback and validation are in the
[delivery evidence](../reviews/dividend-attribution-repair-2026-10-23.md).

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

The [2026-10-28 final disposition](../reviews/product-complexity-disposition-2026-10-28.md)
completes #390's bounded first pass: six workflows/twelve rules, retained
accounting/source/reconciliation/planning protections and delivered ADR 0019
relaxations. Two small browser correctness findings (#433/#434) are separately
fixed; no additional financial-policy change or dependency framework is adopted.
Unreviewed families and owner/release evidence remain explicitly outside that
audit's acceptance.

### Financial Field Correction Delivery (#395)

**Slice 1, cash dividends and standalone withholding (2026-10-26).** The
service and writer no longer pin a replacement's date, cash account and
currency. The inverse remains the exact original journal on the original's date
and accounts; the replacement is ordinary dividend entry under the same audit
event, so the shared writer reviews each reconciled boundary either journal
changes. A move inside one reconciled period nets to zero there and keeps the
checkpoint. The currency follows the selected cash account, as on entry, and an
account without a default commodity keeps the recorded one. Payment shape
(dividend versus standalone tax) remains fixed, and payer/ex date keep #393's
nullable patch semantics.

Source mismatch is disclosed, not vetoed. For imported lineage, the correction
chain compares the effective payment's cash facts exactly (scale-aware) with
the root operation the import produced: the immutable record of the claim,
without reparsing each importer's raw format. It reports `date`,
`cash_account`, `currency`, `amount` and `withholding`; payer and ex date stay
attribution with #393's diagnostics. Restoring the imported facts clears the
disclosure. Raw source rows and commit identity are unchanged.

The retired policy assertion in `TestDividendCorrectionsStayInTheirFamily`
now pins payment shape instead. Named cases:
`TestReplaceDividendCorrectsDateCashAccountAndCurrency` (EUR 100/15 to USD
110/16.50 on a new account and earlier date, original netting to zero),
`TestReplaceStandaloneWithholdingCorrectsDateAndCashAccount`,
`TestReplaceDividendDateAcrossReconciledBoundaryRequiresOverride` (out of,
into and within a reconciled period) and the source-difference assertions in
`TestImportedDividendCorrectionPreservesCommittedIdentity`. Browser: a 390 px
correction to another date and SEK cash account.

**Slice 2, payment in lieu (2026-10-26).** The same contract: the service and
writer no longer pin the replacement's date, cash account and currency; the
inverse stays on the original's date and accounts; the currency follows the
cash account. The payment stays lot-free, and #388's attribution patch
semantics and diagnostics are unchanged. Unlike an imported dividend, no
source claims a payment in lieu's cash facts: a provider event supplies only
instrument and ex date (already diagnosed as `provider_attribution_mismatch`),
and the owner supplies date, account and amount when completing it. So no
cash-fact disclosure is added; the control is that a provider-linked payment
keeps its one payment identity, link, raw evidence and refetch dedupe across a
date/account correction. Named cases:
`TestReplacePaymentInLieuCorrectsDateCashAccountAndCurrency` (EUR 50 owed /
9 recovered to USD 55 / 5 on a new account and date, short lot unchanged),
`TestReplacePaymentInLieuDateAcrossReconciledBoundaryRequiresOverride` and
`TestPaymentInLieuFieldCorrectionKeepsProviderLinkAndDedupe`. Browser: a 390 px
correction to another date and NOK cash account.

**Slice 3, reinvested dividend (2026-10-26).** Only the service check pinned
date, holding, instrument and cost currency; the acquisition writer already
replays typed old/new positions for buys (T-116), so the reinvestment uses the
same lifecycle unchanged. The replacement stays an income-funded
`reinvested_dividend` opening, never a cash purchase or a generic dividend.
Later FIFO, average-cost and specific-lot disposals and transfers replay or
refuse with the dependency named; changed gains need the exact preview
acknowledgement; the original implied price is retired and the replacement's
sits on its corrected date; a checkpoint the move crosses needs the reviewed
override. The retired policy assertion in
`TestReinvestmentAndBuyCorrectionsStayInTheirFamilies` now pins the family
fence (a reinvestment is never replaced through the buy command). Named cases
and browser coverage are listed in the feature ledger. Imported reinvestments
remain outside native correction, as before.

**Disposition.** #395's acceptance is met across all four families and the
issue is closed. Saved report bytes and source evidence were not touched by
any slice; their existing immutability tests remain the control.

### Historical Cash-In-Lieu Entry Delivery (#394, 2026-10-24)

Entry, preview and dated specific-lot lookup now accept an explicitly selected
historical split, exchange, spin-off or cash-and-stock merger. They keep the
original operation reference and original book/type/holding/instrument/date
integrity. Actual disposal replay enforces fractional quantities, available units,
basis, elections and chronological order, with unchanged atomic gain/checkpoint
review. Existing parent ancestry remains advisory; missing current context is
also explicitly unavailable, never evidence that the action supports the disposal.

Migration 26 removes only migration 24's insert standing gate. Facts, history
views and bundle schema/columns do not change. Downgrade to 25 safely restores
the former admission rule because those readers already understand historical
references; migration 24's earlier loss-of-meaning downgrade guard remains.
The composed correction-chain response exposes the selected original action's
scope for entry, including after replacement/reversal, while inverse journals
remain ineligible. Existing transaction/history and split-activity entry paths
show the original parent and a six-locale historical-context notice. No nullable
parent, automatic sale reinterpretation, picker framework or repair queue is added.

[Delivery evidence](../reviews/cash-in-lieu-historical-entry-2026-10-24.md)
records independent accounting, controls and the retired-policy test disposition.
