# ADR 0017: Provider-Independent Features And Incomplete Data

## Status

Accepted (2026-10-15), following the owner's direction that unavailable provider
data must not block app features. Refines ADR 0008's provider boundary and
ADR 0013's provider admission rules. Their ledger, subledger, exactness and
atomicity constraints remain in force.

## Context

Trading 212 is one input source. Its adapter coverage has sometimes been treated
as a completion gate for the wider app: #229's instrument-attribution work was
held open for a legacy import fallback, while #136, #145 and #176 wait for
provider-specific evidence. Manual correction, partial attribution and other
sources must remain useful even when a provider never supplies a field or event.

The app already has useful foundations: manual investment commands and native
corrections; CSV mapping; manual prices and FX; unknown basis with sourced
resolution; incomplete reporting with explicit reasons; and manual split linking
without a second financial posting. These are domain capabilities, independent
of an online connection.

## Decision

1. **Build domain features independently of providers.** Manual entry and
   correction, explicit file mapping and deterministic local fixtures can deliver
   a feature before any online adapter supports it. Missing API coverage,
   credentials, live samples or provider permission block only that adapter or
   automated mapping. A position, report or manual workflow must not require a
   Trading 212 connection or a successful refresh.
2. **Distinguish required financial facts from optional enrichment.** A command
   needs enough owner-confirmed facts to identify its accounts, commodities,
   dates and exact financial effects, and to preserve its domain invariants.
   Missing issuer metadata, an optional payer link or provider reference need
   not stop a valid entry. Unknown basis is admitted only through the existing
   unknown-basis commands. When required effects are unknown, keep the affected
   row in review and offer manual entry or linking; other valid work continues.
   A short sale, split ratio or execution cancellation is never inferred from an
   ambiguous fill name, order status or negative quantity.
3. **Allow incomplete records without inventing facts.** Known cash can remain
   usable while its investment classification, payer, gross/tax breakdown or
   basis is unresolved, through a workflow that explicitly supports that state.
   Keep known facts and missing fields distinct. Unknown is neither zero nor
   evidence of no withholding; net cash alone does not establish gross income.
   An explicit owner choice such as a CSV profile's "no withholding" mapping
   is a declaration, separate from absent provider data. Existing commands keep
   their admission requirements until a typed partial workflow is implemented;
   this decision does not authorize a generic posting that bypasses lot rules.
4. **Make limitations visible and repair optional.** Show which facts are
   unresolved and which balances, instrument allocations or derived figures are
   affected. Offer owner-supplied values, explicit file mapping, manual prices,
   or a link to a manually recorded operation where supported. The owner may
   leave optional attribution unresolved; repeated refresh or completion of
   enrichment must not become a condition for using the app. Missing local
   account mappings/defaults are app configuration gaps with a review path,
   not evidence that the provider lacks financial data.
5. **Keep reporting honest at the affected scope.** Preserve exact known cash,
   quantities and original-currency figures. Put unattributed amounts in an
   explicit unassigned group where they can be identified; do not allocate them
   to a guessed instrument or present an allocated subtotal as the whole book.
   A derived value requiring unknown gross, tax, basis, price or FX remains
   unavailable with its reason and coverage. Label known-only subtotals as such.
   Unrelated instruments and calculations remain usable. No zero substitution
   or new valuation/tax policy follows from this decision; ADR 0012 and each
   report's complete-only aggregate rules still govern.
6. **Preserve evidence and reconcile manual work with later imports.** Keep
   original provider values separate from owner assertions and corrections,
   including actor, reason and source identity. Repairs use versioned facts or
   named audited correction, never overwrite original evidence. Linking an
   already posted manual operation must not post it again. New posting or
   correction keeps the shared exactness, replay, reconciliation and atomic
   source-identity guards. Later refreshes must deduplicate and cannot erase an
   owner correction or silently replace it with a conflicting provider value.
7. **Track core delivery and provider automation separately.** Core acceptance
   covers usable manual/review workflows, known partial states, correction,
   disclosure, export and deterministic validation appropriate to the feature.
   Provider automation adds verified wire semantics, samples and adapter tests.
   A provider-only follow-up does not reopen delivered domain work or block a
   different feature. A real app defect, such as silently losing a payer it
   already knows, remains an app defect even when found through an adapter.

## Implementation And Validation

Reuse the current services, source evidence, field-specific knowledge/reason
contracts and correction/linking mechanisms. Do not introduce a universal
provider capability registry, generic completeness score or replacement ledger
as a prerequisite. Extend the relevant domain contract in a bounded slice when
its existing facts cannot express a partial state.

Core acceptance tests run without live provider access. Cover supported complete
input, missing optional attribution, a manually supplied value, unresolved
required effects without writes, and later reimport/linking without duplication
or loss of owner corrections. Add read/export checks for affected coverage and
use the same financial guards on manual and automated writes. A synthetic core
fixture proves domain behavior; it does not establish an unverified provider
mapping. Adapter fixtures retain that separate evidence gate.

## Consequences

- Provider availability and coverage become automation concerns rather than the
  feature roadmap's critical path. R13 can use manual/imported ledger and price
  evidence while provider comparison or benchmark coverage is unavailable.
- Partial attribution is a supported product outcome, not a financial integrity
  failure. Its disclosure and repair require real implementation; this ADR alone
  ships no new runtime workflow or schema.
- #242 remains a reproducible app review/repair defect: the adapter already
  resolves the payer, but a missing local income default causes its loss.
  Unsupported event automation (#136/#145/#176) remains evidence-gated without
  restricting the existing native correction, split or short-position features.
