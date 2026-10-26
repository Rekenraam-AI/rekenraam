# ADR 0019: Financial Effects And Repairable Attribution

## Status

Accepted direction, 2026-10-23, at the owner's request. The necessity test and
delivery boundary below are adopted; individual relationship contracts are
recorded in [the dependency plan](../plans/financial-dependency-simplification-plan.md)
and [#391](https://github.com/sergeyfarin/rekenraam/issues/391) before implementation.
#388/#389 now deliver optional payment-in-lieu attribution, audited repair
and read-only warning visibility. Payment entitlement no longer gates entry,
replacement or later position corrections; the global payment replay is gone.
#392 now validates all four cash-in-lieu parent families through actual
disposal replay, retaining original provenance and advisory current context.

The first [four-case comparative contract](../plans/financial-dependency-simplification-plan.md#four-case-contract-2026-10-23)
now specifies nullable cash attribution, explicit clearing, audited net-zero
repair and warning versus unavailable/integrity outcomes. Its
[source-pinned experiment](../reviews/financial-dependency-comparison-2026-10-23.md)
supports relaxing the permanent cash-only and blanket parent fences while
retaining real disposal admission. #388/#389 implement their targeted contract;
#392 implements cash-in-lieu parent corrections; #393 delivers nullable,
clearable cash-dividend/withholding payer facts, tax-shape preservation, shared
cash-payment source identity and read-only source mismatch visibility. CSV
income without a payer uses the same typed writer; supplied unknown source
codes still require review. The completed
[family inventory](../reviews/financial-dependency-inventory-2026-10-23.md)
and [plan dispositions](../plans/financial-dependency-simplification-plan.md#completed-family-dispositions-2026-10-23)
cover every #391 family. #394 accepts historical-parent fractional-disposal
entry through actual replay; #395 accepts typed cash/reinvestment field
corrections through the existing writer and acquisition replay. #393 also owns
dividend source identity across correction ancestry and visible source/payer
mismatch, now delivered with migration 25 and bundle schema 17. #394/#395 are
accepted contracts; #394 now ships historical-parent entry/preview/lot selection
with migration 26 and localized existing-history discovery. #395 shipped on
2026-10-26: cash dividend, standalone withholding and payment-in-lieu date/cash
account/currency corrections through the existing writer, and reinvestment
date/holding/instrument/cost-currency corrections through acquisition replay.
Other typed allocation, source,
derivation, snapshot and reconciliation dependencies are retained with concrete
counterexamples; the review adds no relationship engine or loan subsystem.

Refines ADR 0013's cash/payment-in-lieu dependency policy and extends ADR 0017's
separation of financial effects, owner attribution and provider evidence. ADR
0012's journal/subledger atomicity and reporting boundaries remain in force.

## Context

Similar cash events have acquired different correction restrictions. A payment
in lieu has no lot effect but permanently constrains short-position correction;
fractional-share cash in lieu is a disposal with an additional corporate-action
fence. Dividend and withholding attribution is more permissive. Optimizing each
restriction independently can preserve unnecessary complexity. The owner asks:

> If removing a relationship leaves the recorded balances, quantities and
> allocations valid, why must that relationship block a correction?

## Decision

1. Require a concrete accounting reason for each retained hard financial
   dependency. Name the protected outcome and a counterexample that breaks it.
   Also examine basis/elections, report classification and coverage, source
   identity/dedupe, immutable evidence and reconciliation commitments. Unchanged
   totals alone do not prove those guarantees survive. Security, storage and
   recovery protections need their own reasons; this financial test does not
   authorize their removal.
2. Separate recorded financial effects, calculated allocations and explanatory
   attribution. Actual disposal/cover inputs, basis lineage and settlement
   allocations can require strong relationships. An association that merely
   explains a cash event should be auditable and repairable, with a scoped
   diagnostic when unsupported or unavailable. A historical source reference
   can remain immutable without permanently authorizing other records.
3. Adopt cash-only treatment for payment-in-lieu attribution: historical short
   eligibility should inform entry and diagnostics rather than permanently
   block position corrections. Apply this consistently to entry, replacement
   and later edits. Required attribution fields, repair/versioning, diagnostic
   surface and reporting consequences must be specified before retiring the
   existing gates; visible diagnostics ship before or with that change.
4. Review cash in lieu across splits, exchanges, spin-offs and mergers. Preserve
   the actual fraction disposal, exact proceeds, quantity/basis conservation,
   elections and ordering. Retain/refine a parent link only for a demonstrated
   calculation or commitment; test a still-valid disposal after parent
   correction as well as one made impossible by it. This ADR does not declare
   every corporate-action link optional.
5. Preserve original evidence and version owner repairs. Attribution-only repair
   preserves recorded cash; a real financial correction commits its journal,
   subledger and required derived effects atomically with normal reconciliation
   and gain review. Never invent a refund, cash amount, tax split, basis or
   missing provider fact to repair a relationship. Known contradictions,
   unavailable attribution and integrity damage have distinct meanings.
6. Prefer existing postings, typed investment effects and shared lifecycle
   machinery. Entry helpers may compose them without creating a separate
   correction framework per financial label. Review dividends, withholding,
   reinvestment, fees/interest, basis actions, transfers and source links under
   the same test. Loan payments are a comparison using liability postings;
   this decision adds no loan engine, universal dependency registry or event
   platform.

## Consequences And Delivery

[#390](https://github.com/sergeyfarin/rekenraam/issues/390) reviews product-rule
and workflow necessity separately from #284's behavior-preserving refactors.
It reuses existing capability/rule maps and bounded end-to-end scenarios;
neither LOC nor duplicate-check counts prove unnecessary behavior.

#391 records per-family dispositions and implementation boundaries. #388 is
reframed from optimizing the permanent payment blocker to delivering cash-only
attribution; #389 defines its accurate non-blocking diagnostic. Cash-in-lieu
implementation is scoped after its concrete dependency analysis. Each slice
needs only its own completed contract, not the entire cross-app audit.

The accepted direction can change APIs, repair UX, tests and schema meaning;
those changes must be explicit in their delivery slice. Historical #257/#379
results remain evidence of the then-current contract. Work order belongs only
to [the roadmap](../roadmap.md); this decision adds no blanket R13 or release gate.
