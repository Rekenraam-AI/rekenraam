# R18 reporting policy

ADR [0012](../adrs/0012-investment-ledger-subledger-and-reporting-boundaries.md)
governs this plan. The [roadmap](../roadmap.md#r18--reproducible-investment-basis-and-gains-projections)
owns sequencing; [#203](https://github.com/sergeyfarin/rekenraam/issues/203)
owns the bounded observation-selection delivery. The delivered slices below
add operational and alternative-method basis profiles; tax calculations and
frozen historical reports remain outside them.

## Live observation selection: `live_raw_v1`

One SQL owner, `backend/internal/db/observation_selection.go`, supplies daily
and latest candidates to holdings/gains, reporting conversions, currency
settings, pricing diagnostics and constant-rate forecasts. It preserves all
original series, observations, corrections, withdrawals and journal evidence.
It never calls a provider or writes financial records.

Admission happens **before** ranking:

- Only currently non-voided observations recorded at or before the read's UTC
  knowledge cutoff participate. Current withdrawals apply even to a backdated
  valuation. This is a live observation admission boundary, **not** a historical
  reconstruction of what was active before a withdrawal.
- The observation date cannot exceed the consumer's valuation ceiling. For
  current values that ceiling is the server's saved owner-zone calendar date.
- Currency bases accept `manual`, `valuation_override`, `official_fixing`,
  `spot_mid`, and `trade_implied`, with `raw` or `not_applicable` basis.
- Security/other countable bases accept `manual`, `valuation_override`,
  `close`, `nav`, `spot_mid`, and `trade_implied`, with **raw** basis.
  The existing unadjusted `trade_implied` fill marker `not_applicable` is also
  admitted; it does not denote split/dividend-adjusted units.
- `bid`, `ask`, `adjusted_close`, and split/dividend/total-return adjusted
  series cannot price unadjusted ledger units. A manual flag cannot make an
  incompatible quote admissible. Original incompatible evidence stays in
  history/export.

The latest eligible valuation date wins. Within that date: manual flag or
valuation override first, other non-approximate observations second,
approximate observations third; later recording instant then larger observation
ID break equal ranks. A newer approximate date therefore beats an older manual
date. Source assignment controls fetching, not retroactive preference between
stored observations. There is no hidden provider-specific rank. A provider's
publication/observation timestamp is provenance, not the recorded-at boundary.

## Consumer contracts and intentional differences

| Consumer | Valuation and knowledge boundary | Age/coverage rule |
| --- | --- | --- |
| Current positions and unrealized gains | Owner-local today and UTC read start. Quantity/basis remain the current operational projection, irrespective of date filters on realized gains. | No age rejection: retain the latest compatible price, explicitly mark it stale after seven calendar days. Missing/future-only/incompatible/not-yet-known evidence leaves value/gain absent with a reason; quantity and cost currency/basis remain. |
| Currency settings latest rates | Owner-local today and UTC read start. | Retain old compatible rates with their date and the same seven-day stale warning. This is a management view, not a historical conversion guarantee. |
| Net worth series | Each bucket end, capped at owner-local today; observation knowledge limited to UTC read start. | Nearest on-or-before quote within requested age, default seven calendar days, inclusive. No eligible rate means no converted bucket total. Original commodity figures remain. |
| Spending and cashflow | Each posting entry date, with observation ceiling capped at owner-local today; UTC read start knowledge. | Same requested age window. Convert each posting with exact arithmetic then sum; incomplete converted groups/totals are absent. Original currency figures remain. |
| Constant-rate forecast | Owner-local forecast as-of date and UTC computation start. | Currency-only rates, fixed seven-day maximum age; one selected as-of rate stays constant across the future horizon. Missing/old rates omit conversion, retain exact source-currency series. This does not predict future FX. |
| Pricing workspace diagnostic | Explicitly displayed `as_of` date (owner-local today by default) and UTC read start. An explicitly future diagnostic date is allowed. | Configured `staleness_max_days`, independent of holdings' warning and reports' requested window. An old candidate remains inspectable with age/status; missing candidates disclose incompatible/future/knowledge reasons. History shows all original evidence. |

Age is a difference of calendar dates, not elapsed owner-zone hours. Seven days
is admissible; eight is too old for seven-day reports/forecasts. Holdings use
seven days as a **warning**, not a value gate. Pricing policy changes do not
silently reconfigure historical reporting or holdings.

Reporting responses identify `selection_policy`, `valuation_as_of`,
`knowledge_cutoff` and each selected observation/requested date, including
approximation. A `future_only` gap can also mean the report has a rate for a
later bucket but none at/before this posting. `no_observation_in_window` with a
nearest earlier date identifies old evidence; without one it identifies missing
evidence. Other reasons are `incompatible_quote_basis` and
`not_known_at_cutoff`. Holdings expose the same reasons plus `no_price` and
retain their existing `unrepresentable` arithmetic outcome. Screens disclose
selection and age policy and translate unavailable/stale reasons.

## Reproducible profile boundary

A future R18 profile must pin purpose, basis method/version, effective period,
valuation date, price knowledge boundary, quote/source policy, FX method,
staleness, reporting currency and rounding, and reconstruct retirements and
ledger/basis knowledge or persist an immutable run/snapshot. A timestamp on
this live selector alone cannot do that. No alternative profile may rewrite
operational elections, journal balances, tax liabilities or reconciliation.

## Basis and valuation profiles (#118)

A **profile** is a named, versioned, read-only rule set. Every projection
response names the profile it ran under and echoes each resolved field, so two
figures can be compared only when their descriptors match. Profiles never post,
never change lots, elections, prices or reconciliation, and never claim tax
compliance.

| Field | Meaning | Values defined so far |
| --- | --- | --- |
| `name` / `version` | Stable key; a rule change is a new version, never a silent reinterpretation. | `operational_v1`, `alternative_method_v1` |
| `purpose` | What the figure is for. A label, not a compliance claim. | `operational`, `alternative_method` (later: `broker_comparison`, `tax_estimate`) |
| `basis_method` | How disposals consume basis. | `operational_elections`: each committed disposal's recorded method, tier and allocations (ADR 0012); no re-election. |
| `valuation_as_of` | Calendar date the open positions are replayed to and priced at. | Any date up to owner-local today; default today. |
| `positions_as_of` | Which events build the holding. | Every effective event dated on or before `valuation_as_of`, replayed in a rolled-back read (the #166 dated-holdings replay, both sides). |
| `knowledge_cutoff` | UTC instant limiting admitted **observations**. | Read start. |
| `ledger_knowledge` | Which book history the replay sees. | `current_effective`: today's effective history — a later correction, reversal or withdrawal restates an earlier valuation date. Not reproducible as known then; see *Reproducibility* below. |
| `quote_policy` | Observation selection. | `live_raw_v1` (above). |
| `staleness` | Age rule for the selected price. | Holdings rule: retain the latest compatible price on or before `valuation_as_of`; mark it `stale` past seven calendar days; never a value gate. |
| `fx_method` / `reporting_currency_id` / `fx_max_age_days` | Conversion of results. | `none` in original figures; additive operational conversion under `valuation_date_all` or `historic_basis_valuation_close`, in the selected currency with an explicit inclusive age window (default seven days). |
| `rounding` | Arithmetic. | Exact; market value is one `exact.MulDivRound` half-up step at the deeper of quantity×price and basis scale. |

### Research recommendations

- **Which measure is authoritative where.** The portfolio dashboard shows the
  *position* view: market value, remaining operational basis and unrealized
  gain at a stated valuation date with the price date and stale marker beside
  it. Realized gain belongs to a *period* report (disposal-date range),
  alongside income and the change in unrealized gain between the period's
  start and end valuation dates. The dashboard never mixes a period's realized
  figure into a point-in-time total.
- **Jurisdiction or purpose without a tax engine.** Users choose a named
  profile, whose `purpose` and method are shown with every figure; a profile
  may carry a free-text jurisdiction note. Rekenraam computes the method the
  profile names and says which one it is — never "your taxable gain". Regimes
  whose pool spans accounts (UK s104, Canadian ACB, French PMP) are a separate
  profile method over the person's positions, not a re-election of the
  per-account operational lots.
- **Unrealized movement without flip-flop.** Unrealized gain is always
  presented *as of a valuation date with the price that produced it*, never as
  a bare "now" figure. Same-date selection is deterministic (`live_raw_v1`), so
  a refresh changes a figure only when a newer or higher-priority observation
  arrives, and the displayed price date says so. Period reports use the
  period-end date, so later refreshes cannot move a closed period's figure
  unless evidence for that date changes. Stale prices remain visible and
  marked rather than disappearing.
- **I-04 (formal postings).** Out of R18. ADR 0012 already forbids silent
  posting; an explicit linked reclassification workflow is not scheduled
  until a user need for formal gain statements is evidenced.

### Reproducibility

`operational_v1` is a *live* projection: rerunning it for the same
`valuation_as_of` returns the same figures until the book's effective history
or the admitted observations for that date change, and its descriptor says
which knowledge it used. Reproducing a figure *as it was known at an earlier
instant* needs either knowledge-time reconstruction of retirements and
revisions or a persisted immutable run; that choice is a later slice and no
current response claims it.

### Period statement (#231)

A period `from..to` reports, per currency and under the same profile:
opening unrealized gain valued at the day before `from`; realized gain of
disposals dated in the period; investment income as the gross income-account
credit of effective cash and reinvested dividends dated in the period
(withholding is separate; a reversed or replaced dividend is not effective;
return of capital reduces basis and is not income); closing unrealized gain at
`to`; their change; and the total of realized, income and change. Each
valuation states its position, unpriced, unresolved and stale counts and the
range of price dates used. Any figure resting on a missing price or unresolved
basis is null with its reason, and so is every figure built from it; a
currency with nothing held at a date has a known zero.

### Alternative-method comparison (#232)

`GET /api/v1/investments/gains?alternative_method=fifo|lifo|average_cost`
adds an `alternative` object beside the unchanged operational response. It
contains its own `profile`, `realized`, `unrealized` and `realized_totals`.
`alternative_method_v1` has purpose `alternative_method`; `basis_method` is
the selected method. Every disposal, including an operational specific-lot
election, uses that method (`specific_lot_fallback: selected_method`). This
is a comparison, never an operational re-election or a tax computation.

The optional inclusive `from`/`to` disposal period is echoed in the alternative
descriptor; omitted bounds are unbounded. It is independent of
`valuation_as_of`. Replay starts at the first effective event, rather than at
`from`, so earlier purchases and disposals determine the basis used inside the
period. Both long and named-short positions use their existing exact disposal
algorithms. Merger cash keeps its recorded disposal date and zero quantity,
independently of its cash posting's payable date. Unknown basis remains
unresolved, never a gain against the known subset.

A single merged stream orders effective events by date, correction-root slot
and effect. Ordinary holding transfers retain each recorded link's quantity
and destination identity/acquisition-date evidence, but deplete using the selected method
(`transfer_policy: selected_method_fixed_link_quantities`). Hypothetical
carried basis reaches a destination before it opens, including onward and
return transfers; no operational transfer revision or bridge is written.
Source-lot and pooled-lot destinations are both supported. Structural actions
and explicit capital-return entitlements retain their sourced lot relationships
(`structural_action_policy: recorded_entitlements`). If a hypothetical method
leaves those entitlements impossible, the comparison refuses with HTTP 409
`CONFLICT`, naming the dependent operation, and returns no partial projection.
It never silently changes the sourced action to fit a method.

The transaction is always rolled back, on success or failure. Temporary lot
state, method locks and generated replay effects never survive the read;
operational lots, elections, transfer links, journals, reconciliation, prices
and audit rows stay unchanged. The profile retains `current_effective` history,
`live_raw_v1` observations (using the operational read's same price knowledge
cutoff and valuation date), original cost currencies, the seven-day stale
warning and exact half-up valuation. As with `operational_v1`, later corrected
history or observation evidence can restate the result; immutable runs remain
#234. Cross-account person-level pooling remains #235. The period statement
endpoint continues to use `operational_v1`.

The Gains screen offers an optional method selector and compares realized
currency totals and each position's unrealized gain in adjacent columns. It
uses the same applied period and valuation controls as the operational figures,
shows the selected method and read-only policy, and retains loading, empty,
error and unavailable states in all six locales.

### Reporting-currency gains (#233)

Both gains endpoints accept `reporting_currency_id`, `fx_method` and optional
`fx_max_age_days` (1–365 calendar days, default seven). Currency and method
are required together; blank, duplicate, orphaned and invalid FX options are
rejected. Omitting all three preserves the original response shape. The
additive `reporting` object has its own operational profile, reporting currency,
FX method, window, valuation/period and rounding descriptor. Original-currency
figures and the alternative-method comparison remain available; this slice
converts the operational profile, not hypothetical alternative allocations.

- `valuation_date_all` converts each signed basis, proceeds, income and market
  component at the selected valuation date. The statement uses its opening
  date for opening unrealized and its closing date for closing unrealized,
  period proceeds/basis and income.
- `historic_basis_valuation_close` converts each operational lot's allocated
  disposed or remaining basis at its effective **original acquisition date**;
  proceeds (including the signed cost of a short cover) and gross income at
  their event dates; and signed market exposure at the valuation date. Long
  basis subtracts and short opening proceeds add. Operational pooling,
  subsequent basis adjustments and structural elections remain as recorded;
  this method converts the resulting per-lot amounts rather than creating a
  separate FX cost-basis pool. Transfers retain their effective carried original
  date (including the sourced latest-date convention of pooled transfers).
  An unknown original date leaves foreign basis and dependent gains unavailable;
  it never falls back to the transfer date. This is a named comparison rule,
  not a jurisdiction's tax or historical-FX accounting engine.

Rates use `live_raw_v1`, sharing the source projection's observation knowledge
cutoff. The nearest compatible observation on or before each requested date
must be within the inclusive window. Every signed foreign component is rounded
once, half-up on magnitude, to the reporting currency's standard scale, using
`exact.MulDivRound`; figures and totals sum those rounded components exactly.
Same-currency components are exact no-ops, retaining sub-cent precision without
requiring rates or acquisition dates. The descriptor names this rounding
`component_half_up_currency_scale`. Overflow of the supported exact coefficient
returns 422 `LEDGER_OVERFLOW` without changing original projections.

Each converted gain retains its component role, lot ID where applicable,
source currency and exact source/converted amount, requested date, observation
ID/date, exact rate and base quantity, approximation and derivation flags.
Missing, stale, incompatible, future-only or not-yet-known rates, unknown basis,
unknown acquisition dates and missing market values carry explicit reasons;
stale gaps also disclose the nearest prior date. A component gap makes its
figure null; a single incomplete figure makes the relevant aggregate null.
Known amounts in other currencies remain visible, but never form a partial
headline total. The statement combines converted opening, realized, income and
closing figures into exact unrealized change and period total, retaining
boundary/disposal/income evidence. It never converts an already summed
mixed-currency total or substitutes zero for missing coverage.

The six-locale Gains screen shares currency, method and maximum-age controls
with its period statement. Responsive reporting cards expose component/rate
provenance in keyboard-operable disclosures and show incomplete coverage beside
retained original figures. These remain live reads over current effective
history; historical reproduction remains #234 and person-level pooling #235.

### Delivery slices

| Slice | Issue | Scope |
| --- | --- | --- |
| 1 | [#230](https://github.com/sergeyfarin/rekenraam/issues/230) | `operational_v1` descriptor and as-of valuation: positions replayed to `valuation_as_of`, priced under `live_raw_v1`, on the gains read and screen. |
| 2 | [#231](https://github.com/sergeyfarin/rekenraam/issues/231) | Delivered 2026-10-14. Period gains statement: opening unrealized (day before the period), realized, gross dividend income and closing unrealized per currency for a date range; absent figures are null with reason and counts. |
| 3 | [#232](https://github.com/sergeyfarin/rekenraam/issues/232) | Delivered 2026-10-15. Alternative-method projection (FIFO/LIFO/average) beside the operational result, computed by rolled-back replay. |
| 4 | [#233](https://github.com/sergeyfarin/rekenraam/issues/233) | Delivered 2026-10-15. Operational gains and period statement in a reporting currency under two named FX methods, with per-component provenance and complete-only totals. |
| 5 | [#234](https://github.com/sergeyfarin/rekenraam/issues/234) | Reproducible runs: knowledge-time ledger reconstruction or immutable persisted run, decided by evidence. |
| 6 | [#235](https://github.com/sergeyfarin/rekenraam/issues/235) | Person-level pooled average-cost profile across accounts (s104/ACB/PMP style), labelled as a method, not tax advice. |

Broker-comparison reads (retained broker-reported basis beside the book's)
follow slice 3 once a provider supplies that evidence; they are not
scheduled.

## Validation evidence

- `TestGainsFX*` covers the two methods with independent expected amounts,
  signed short proceeds, dated period boundaries and income, mixed currencies,
  missing/stale/inclusively aged rates, exact same-currency sub-cent no-ops,
  unknown basis/acquisition dates, transfer origin evidence, validation and
  unchanged durable snapshots. `TestInvestmentGainsReportingCurrencyAPI`,
  `TestInvestmentGainsFXInvalidOptionsAPI` and
  `TestInvestmentGainsFXPrecisionOverflowIs422` verify the authenticated wire
  contract and original figures. `investments-gains-fx.spec.ts` exercises the
  shared mobile controls, coverage disclosure, method changes, original amounts,
  rate evidence, same-currency conversion and axe accessibility at 390px.

- `TestAlternativeGainsMethodsConserveQuantityAndBasisAcrossPartialSales`,
  `TestAlternativeGainsEqualOperationalWhenMethodsMatch`,
  `TestAlternativeGainsConserveBasisThroughTransfersAndDestinationSales` and
  `TestAlternativeGainsShortMethodsConserveOpeningProceeds` pin exact expected
  results and conservation under all three methods.
- `TestAlternativeGainsReadLeavesLotsElectionsJournalsAndAuditUnchanged` and
  `TestAlternativeGainsFailureRollsBackTemporaryProjection` snapshot durable
  state across repeated reads and injected late failures. The remaining
  `TestAlternativeGains*` cases cover independent period/valuation dates,
  corrected history/splits, unresolved basis, merger metadata and explicit
  refusal of impossible structural entitlements.
- `TestInvestmentGainsAlternativeMethodAPI` verifies both profiles, exact
  coefficients, totals and validation through authenticated HTTP.
  `TestAlternativeGainsOverflowIs422AndOperationalProjectionSurvives` verifies
  a hypothetical method exceeding supported precision returns 422 while the
  operational read remains usable. The 390px
  `investments-alternative-gains.spec.ts` journey covers the comparison
  control, expected values, unchanged operational basis and axe accessibility.

- `TestObservationSelectionThroughHoldingsAndLatestPrices`: future, adjusted,
  bid and raw admission through real holdings and settings price reads.
- `TestObservationSelectionPriorityKnowledgeAndAgeThroughConsumers`: named
  manual/provider, override, exact/approximate, date/time/ID ties, incompatible
  manual bases, future-only, knowledge cutoff and stale cases through real
  holdings, gains, latest prices and report conversion; original rows retained.
- `TestObservationSelectionAPIHoldingsAndReportingFX`: same policy through
  authenticated HTTP gains and spending FX, retaining original currency totals.
- `TestObservationSelectionOwnerLocalDateAndPreciseKnowledgeBoundary` pins
  opposite-midnight owner zones and refuses a quote recorded in the next second.
- `TestReportingCoverageRetainsEachSelectedObservationAndApproximation` pins
  distinct quote/date provenance instead of hiding later estimated uses.
- `TestForecastUsesSharedManualRawAndKnowledgePriority`,
  `TestForecastConstantFXStalenessAndTies` and related forecast suites retain
  the constant-as-of and complete-only conversion contract.
- Relevant 390px browser journeys cover displayed policy/reasons, gains,
  reporting conversion and #101's owner-local defaults. The backend race gate,
  frontend checks/tests and integrated build remain required.

An isolated mutation removing compatibility/date/knowledge admission and
same-day source priority makes the named service/API cases fail at runtime,
including manual/provider, exact/approximate, adjusted, future and knowledge
cases. These are observed assertion failures, not compilation failures.

Independent validation-tooling gap: [#207](https://github.com/sergeyfarin/rekenraam/issues/207)
tracks clean-checkout SvelteKit alias bootstrap before catalog checks.

Mobile validation also found [#208](https://github.com/sergeyfarin/rekenraam/issues/208):
the gains table scroll regions now have localized region names and keyboard
focus. The backend gate found [#209](https://github.com/sergeyfarin/rekenraam/issues/209):
the single test mutating the process-wide Trading 212 page limit is serial under
the existing reasoned opt-out convention; all fetch/continuation assertions and
race instrumentation remain.

The full report-screen axe check found [#210](https://github.com/sergeyfarin/rekenraam/issues/210):
the page-level Filters heading is now h2, keeping the localized filter label
and report-view headings at the correct page level.
