# ADR 0012: Investment Ledger, Subledger, And Reporting Boundaries

## Status

Accepted

## Date

2026-08-29

## Context

An investment trade has facts that do not change with a reporting convention:
the instrument and quantity traded, cash paid or received, fees and withholding,
trade and settlement dates, account, provider identity, and source evidence.
Realized and unrealized gain are different. They depend on a cost-basis method,
period/date basis, jurisdiction or accounting purpose, prices, FX, and the time at
which those prices were known.

The existing implementation correctly posts buys and sells as ordinary balanced
multi-commodity transactions and keeps lots beside them. It also has one mutable
authoritative lot projection and a current-state gains endpoint. Treating that
projection as either the whole ledger or as disposable reporting state is unsafe:
specific-lot instructions and basis adjustments are durable elections, while one
mutable projection cannot represent several legitimate bases at once.

## Decision

Investment accounting has four explicit layers.

1. **Canonical journal.** Posted transactions record the economic event: exact
   security and cash quantities, fees, income, withholding and tax postings. Buy
   and sell settlement balances through the `commodity_trading` equity account.
   For an open position that account contains a raw per-commodity clearing/cash-
   flow residual, not a method-independent realized gain: part of the cash
   residual is still capital attributable to the remaining holding. A basis
   policy is required to split it. The journal does not change merely because a
   report selects FIFO, LIFO, average cost, or a valuation method. An external
   transfer-out bridge may record the carried value calculated under the
   committed operational basis election. If corrected economic history
   changes that value, append a guarded, dated adjustment; an alternative
   reporting profile alone never changes posted journal amounts.
2. **Investment subledger.** Immutable acquisition, disposal, transfer,
   corporate-action, basis-adjustment, and lot-election events record the
   relationships the journal alone cannot express. An investment mutation is one
   domain operation: its journal and subledger consequences commit, reverse, or
   correct atomically. Generic transaction mutation must not be allowed to strand
   one side.
3. **Basis and valuation projections.** Server-side read models derive positions,
   realized gain, unrealized gain, and comparisons under a named basis profile.
   A profile identifies its purpose/scope and exact rules, including cost-basis
   method, effective period/date basis, price knowledge cutoff, valuation source
   policy, FX method, staleness, reporting currency, and rounding. Projections do
   not mutate canonical facts. More than one projection may coexist.
4. **Optional accounting postings.** If a future workflow needs realized gain,
   unrealized revaluation, or a tax liability in formal statements, it creates an
   explicit, auditable transaction linked to the projection/run and policy that
   produced it. For realized gain this would be an explicit method-specific
   reclassification between `commodity_trading` equity and the chosen income or
   expense account, leaving the selected basis of the open position in clearing.
   This is not a second recognition of gain: for a fully closed position whose
   opening basis, net proceeds and charges have corresponding clearing legs in
   the same cost currency, the clearing residual nets to its operational gain
   or loss. An open position's residual has not yet been split. An external
   transfer with unknown carried basis, a fee expensed outside clearing, or an
   unmatched foreign-currency charge breaks that simple identity; a report
   must flag or explicitly adjust it before reclassification. A report must
   never silently manufacture ledger postings.

For operational gains, a trade charge enters acquisition basis or reduces
disposal proceeds only to the extent its cost-currency value enters
`commodity_trading`, whether as part of net settlement or an explicit
clearing leg. A separately expensed charge stays out of operational
basis/proceeds, so the combined after-expense result must include that
expense separately. Do not count one charge in both places. Cross-currency
charges require explicit value/conversion legs before they can enter the
same-currency clearing identity. Tax-specific treatment remains a separate
reporting-policy question.

Each recognized charge's operational treatment is an immutable election
with the selected policy and version recorded at commit. Resolve it from
the transaction, account, global book policy, then a named fallback; later
default changes do not reinterpret prior trades. Ordinary same-currency
commissions default to inclusion in clearing, preserving the existing
net-cash trade behavior.

Each posted long disposal snapshots signed cost-currency proceeds in its
decision and allocates them to consumed lots by exact quantity ratio, assigning
the rounding remainder to the final lot. The operational gains view reads that amount and allocated basis;
cash journal legs are not a proxy for proceeds when a trade also posts
separately expensed charges. A zero-proceeds write-off snapshots zero. A
net-only trade retains its gross-unknown source marker and net-derived price
as an approximate valuation quote. For observations dated alike, manual prices
and valuation overrides rank first, trusted provider and gross-derived prices
second, and approximate prices third. Recorded time and ID break equal ranks.
A fee paid separately has its own net cash component, cash account and payment
date. Its journal legs post on that date. A clearing-included fee still enters
the trade's operational basis or proceeds; a separately expensed fee does not.

The operational disposal decision remains durable even when alternative reports
are available. Every committed disposal snapshots its resolved method, the tier
that supplied it (transaction, account, global, or fallback), the applicable
policy/profile version, explicit allocations when used, and audit provenance.
Changing a default later does not reinterpret that decision.

The immutable event stream is authoritative. Current lot quantities, remaining
basis, and future per-profile basis states are rebuildable materialized
projections. They must conserve quantity and basis across sequential events and
must be checked against the current posted journal.

The first correction slice may keep one operational projection, but its schema and
events must not preclude projections keyed by basis profile. Multi-method and
multi-jurisdiction reporting is built only after the event and policy contracts are
specified; it is not implemented by repeatedly mutating the operational lots.

## Consequences

- Cost-basis choices do not alter the settlement postings for a trade, but an
  actual lot election is still a durable accounting fact.
- Broker-displayed basis may be retained as sourced evidence without automatically
  becoming the book's authoritative policy.
- Period and unrealized-gain questions belong to reproducible read models with an
  explicit `as_of` and knowledge cutoff, not to current-price UI arithmetic.
- Investment edit, void, delete, restore, correction, transfer, and corporate-action
  workflows need subledger-aware lifecycle behavior.
- The existing average-cost projection must be corrected before it is trusted: the
  sum of disposed and remaining basis must equal the pre-disposal pool after every
  sale, including a sequence of partial sales.
- The gains endpoint remains a current operational view until the later projection
  contract ships; it must not be described as tax reporting or a reproducible
  historical report.
- ADR 0008 remains in force for instruments, generic prices, provider trust, and
  ordinary ledger posting. This ADR refines its ledger/lot/reporting boundary.

## Live observation policy refinement — 2026-10-14 (#203)

The first R18 policy slice names `live_raw_v1` in the
[reporting plan](../plans/investment-reporting-plan.md). Compatible unadjusted
quotes are selected before date/source/approximation ranking by one shared
owner. Current values stop at owner-local today; every live read has a UTC
recorded-at admission boundary. Manual/override, exact and approximate priority
applies only within the same valuation date and never admits an adjusted quote.

Consumer age semantics remain explicit: holdings retain old prices with a
seven-day stale warning; reports require their requested dated window;
constant-rate forecasts require seven-day as-of coverage; pricing diagnostics
use their configured threshold. Missing or unusable prices never replace the
original commodity figures with zero. Current retirements still apply: a
recorded-at cutoff alone does not freeze ledger, basis or withdrawal history,
and these live reports carry no tax or reproducible historical-report claim.

## Profile contract refinement — 2026-10-14 (#118)

The [reporting plan](../plans/investment-reporting-plan.md#basis-and-valuation-profiles-118)
fixes the profile descriptor every projection echoes (name/version, purpose,
basis method, valuation date, knowledge cutoff, ledger knowledge, quote policy,
staleness, FX method, rounding). The first profile, `operational_v1` (#230),
replays positions to a valuation date under today's effective history and
labels that `ledger_knowledge: current_effective`; it does not claim
reproduction as known at an earlier instant (#234). The dashboard shows
point-in-time unrealized figures with their price date; realized gain belongs
to period reports. Purpose and jurisdiction are user labels on a named
method, never a compliance claim. Formal gain/revaluation postings (layer 4,
I-04) are not part of R18 until a user need is evidenced.

## Immutable report-run refinement — 2026-10-15 (#234)

Layer 3 now supports explicitly saved immutable operational gains and period
statements. The [reporting plan](../plans/investment-reporting-plan.md#immutable-operational-runs-234)
defines saved-document schema 1: one consistent disposable SQLite input image,
full `operational_v1` descriptor, resolved request, exact outputs, SHA-256 input
and output fingerprints, capture time and actor/audit attribution. Saving is an
explicit audited command; retrieval returns the preserved document without
replay. Current live profiles retain their effective-history semantics. Saved
runs do not post, re-elect lots or alter reconciliation, and they cannot recover
an earlier unsaved live response. Alternative-method and FX runs remain outside
saved schema 1.

Knowledge-time reconstruction was rejected for this bounded slice: observation
withdrawals and operational replay currently evaluate current effective facts
and latest revisions. A coordinated temporal contract across these readers and
reference versions would be substantially larger than a price timestamp filter.
The existing native backup API provides an isolated snapshot on which existing
Go readers can run unchanged, without racing later writes or introducing a
parallel financial engine. The private snapshot is discarded; only its physical
fingerprint and the finished document are retained. Snapshot hashes identify the
input image, not a portable logical export, and hashes do not provide signatures.

A saved run is durable investment evidence, with immutable UPDATE/DELETE guards
and a single atomic run/audit insertion. Forward migration 0007 adds the rows
without changing the baseline. Backup/restore preserves them; bundle schema 12
exports all saved documents and fingerprints even under ledger filters. The
investment-foundation self-check checks document integrity, never restates old
figures from current history. Future document schemas/readers must preserve old
outputs rather than recompute or overwrite them. This refines ADR 0011's bundle
contract without changing the canonical ledger CSV or QIF semantics.

## Consistent live composition refinement — 2026-10-15 (#237)

One live gains or period-statement response uses one disposable SQLite input
image across operational, alternative/person-pooled, historical-basis, income,
boundary and reporting-FX readers, including reference metadata and observation
retirements. It reuses #234's native capture without hashing or persisting a run.
Existing replay owners need rolled-back writes; a read-only transaction cannot
host them unchanged. Copying avoids holding the live writer across replay, at
the explicit cost of full-database I/O and temporary disk per request. Only
capture occupies the live pool connection. The image is private and removed on
completion or failure. The [reporting plan](../plans/investment-reporting-plan.md#consistent-live-input-state-237)
and [cost review](../reviews/live-gains-consistency-2026-10-15.md) record the
boundary, measurements and validation.

This is consistency within a response, not retained reproducibility or temporal
reconstruction: later requests evaluate later effective history. The descriptor
remains `current_effective`, and only an explicit saved run preserves its output.

## Capital-return disclosure refinement — 2026-10-15 (#238)

Layer 3's period result retains its realized + gross dividends + unrealized-change
formula with an explicit limited-scope label. Return-of-capital receipts,
associated basis reductions and excluded excess are separate payment-cohort
disclosures; unknown reduction/excess is never zero. The
[reporting policy](../plans/investment-reporting-plan.md#return-of-capital-period-disclosure-238)
defines effective/payment timing, independent complete-only FX figures and new
saved-statement schema 2. Existing schema-1 bytes and meaning are preserved.
This is reporting disclosure, with no layer-4 posting or tax recognition policy.
