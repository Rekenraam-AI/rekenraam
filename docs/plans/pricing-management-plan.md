# Price and FX management (#116)

Status: **shipped** through #200–#202 (2026-10-13). Retained as the bounded
operator contract; current consumer selection policy lives in
[investment reporting](investment-reporting-plan.md).

ADR 0008 governs immutable source observations; ADR 0010 governs existing FX
fetching. Delivery acceptance lives in #116 and #200–#202. Execution order
remains in [the roadmap](../roadmap.md).

## Read contract

The operator workspace composes current commodity/source choices and price-pair
coverage in one response. Pairs include configured instruments, current open
lot cost currencies, active account currencies relative to the configured FX
base, and existing series. A deliberate additional pair can be inspected and
entered even without a configured series. History pages sort by descending
observation ID (recording order), with an opaque keyset cursor bound to filters;
valuation and recorded dates remain separate, visible fields. Voided rows are
available as evidence. Stored price coefficients remain positive int64 at scales
0–18; browser DTOs and new manual requests use canonical decimal strings.
This is a deliberate pre-release wire change for `PriceObservationResponse`
(including currency-settings composition); legacy create requests remain
numeric, while new manual/correction requests are lossless strings. Existing
storage and exports are unchanged; no migration or database reset is needed.

Coverage uses an explicit valuation date, defaulting to owner-local today,
current recorded knowledge, and the configured pricing-policy age threshold.
Candidates rank by valuation date, manual/override first at equal dates, then
non-approximate, then recorded time and ID. Future and voided observations are
excluded. A stale candidate stays inspectable but is not called usable by this
diagnostic. Same-commodity pairs require no observation and are rejected as
manual entry. This diagnostic is not a new report engine: current holdings use
latest-price selection without this age gate; reporting FX uses its named
window (default seven days). #203 now supplies their shared compatible-quote
selection and explicit date/knowledge ceilings, with seven-day stale warnings
on retained holdings/settings values. The [R18 policy](investment-reporting-plan.md)
records the consumer differences and remaining reproducible-profile boundary.

Every observation exposes source, quote type and adjustment basis, valuation,
observation, publication and recording times, manual/derived/approximate flags,
trade-version linkage, derivation/metadata, and supersession/void evidence.
Original derivation JSON is also returned as text, so inspection never rounds
its nested numeric leg coefficients through a browser JSON Number. A
manual observation is an owner assertion, not provider verification. Correcting
a provider/trade quote produces explicitly manual evidence.

## Commands and review

Manual entry requires an exact positive price and base quantity, a valuation
date, and a reason. It always uses the seeded manual source, manual quote type,
raw adjustment basis, and optional plain-text reference/observation timestamp.
The coefficient range is checked before converting to the existing int64 model;
overflow refuses without writes. The legacy create route may not carry a
supersedes ID: correction has its own reviewed command.

An impact preview reads the exact bounded dependency closure used by voiding,
under one database transaction, and never writes audit, prices or work. Its
acknowledgement fingerprints the root and every active derived observation.
The command rechecks that closure and fingerprint inside its writer transaction;
a changed/retired root or newly derived observation requires a fresh review.
Legacy callers may void without an acknowledgement; the new UI always supplies
one. The existing depth-16 bound remains atomic: deeper closure refuses wholly.

Correction retires the root and active derived closure, then appends a manual
replacement for the root's same commodity pair and valuation date. Other dates
or pairs require explicit void plus separate manual entry. It preserves all
original values/source/trade identities, records immutable supersession linkage,
and uses one audit event for retirement and replacement. It does not regenerate
derived rates, reinterpret provider IDs or edit trade-implied ledger facts.
Voiding/correction changes market valuations only; journal postings, operational
basis and reconciliation checkpoints are untouched. Late failure rolls back
retirements, replacement, series and audit together.

## Browser behavior

A six-locale mobile/keyboard workspace provides loading, empty, error, success,
filter, paginated history, provenance detail and deliberate mutation review.
The review shows every affected observation and valuation-only consequences.
Cancel writes nothing; stale refusal preserves the form and clears its review.
Success invalidates pricing/currency/investment/report/forecast reads. Provider
configuration and refresh remain in existing currency settings; no new adapter,
network request on page reads, report snapshots or attachments are introduced.
