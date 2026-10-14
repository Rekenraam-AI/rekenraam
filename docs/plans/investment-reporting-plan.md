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
