# Financial Dependency Simplification

Owner direction: 2026-10-23, governed by
[ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md).
This is a design and delivery-boundary plan, not a completed audit or shipped
behavior. Baseline: `8549045a4484f01e6e9b28e4f0a5fd6aa8213985`.

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
