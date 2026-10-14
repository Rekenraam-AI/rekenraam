# R18 reporting policy

ADR [0012](../adrs/0012-investment-ledger-subledger-and-reporting-boundaries.md)
governs this plan. The [roadmap](../roadmap.md#r18--reproducible-investment-basis-and-gains-projections)
owns sequencing; [#203](https://github.com/sergeyfarin/rekenraam/issues/203)
owns the bounded observation-selection delivery. This first policy slice does
not deliver alternative basis profiles, tax calculations or frozen historical
reports.

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
| `name` / `version` | Stable key; a rule change is a new version, never a silent reinterpretation. | `operational_v1` |
| `purpose` | What the figure is for. A label, not a compliance claim. | `operational` (later: `alternative_method`, `broker_comparison`, `tax_estimate`) |
| `basis_method` | How disposals consume basis. | `operational_elections`: each committed disposal's recorded method, tier and allocations (ADR 0012); no re-election. |
| `valuation_as_of` | Calendar date the open positions are replayed to and priced at. | Any date up to owner-local today; default today. |
| `positions_as_of` | Which events build the holding. | Every effective event dated on or before `valuation_as_of`, replayed in a rolled-back read (the #166 dated-holdings replay, both sides). |
| `knowledge_cutoff` | UTC instant limiting admitted **observations**. | Read start. |
| `ledger_knowledge` | Which book history the replay sees. | `current_effective`: today's effective history — a later correction, reversal or withdrawal restates an earlier valuation date. Not reproducible as known then; see *Reproducibility* below. |
| `quote_policy` | Observation selection. | `live_raw_v1` (above). |
| `staleness` | Age rule for the selected price. | Holdings rule: retain the latest compatible price on or before `valuation_as_of`; mark it `stale` past seven calendar days; never a value gate. |
| `fx_method` / `reporting_currency` | Conversion of results. | `none`: every figure stays in the position's cost currency. |
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

### Delivery slices

| Slice | Issue | Scope |
| --- | --- | --- |
| 1 | [#230](https://github.com/sergeyfarin/rekenraam/issues/230) | `operational_v1` descriptor and as-of valuation: positions replayed to `valuation_as_of`, priced under `live_raw_v1`, on the gains read and screen. |
| 2 | [#231](https://github.com/sergeyfarin/rekenraam/issues/231) | Period gains statement: opening/closing unrealized, realized and income per cost currency for a date range. |
| 3 | [#232](https://github.com/sergeyfarin/rekenraam/issues/232) | Alternative-method projection (FIFO/LIFO/average) beside the operational result, computed by rolled-back replay. |
| 4 | [#233](https://github.com/sergeyfarin/rekenraam/issues/233) | Reporting-currency conversion with a named FX method and coverage disclosure. |
| 5 | [#234](https://github.com/sergeyfarin/rekenraam/issues/234) | Reproducible runs: knowledge-time ledger reconstruction or immutable persisted run, decided by evidence. |
| 6 | [#235](https://github.com/sergeyfarin/rekenraam/issues/235) | Person-level pooled average-cost profile across accounts (s104/ACB/PMP style), labelled as a method, not tax advice. |

Broker-comparison reads (retained broker-reported basis beside the book's)
follow slice 3 once a provider supplies that evidence; they are not
scheduled.

## Validation evidence

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
