# R13 investment returns contract (`returns_v1`)

Status: **contract accepted by the owner 2026-10-24 (four policy answers
recorded below); slice 1 (backend read) delivered 2026-10-24 and corrected
the same day after two reviews (`e30413a5`: seven findings, recorded inline
as "finding N"; `ae084c4d`: two, recorded under the expected cases). Slice 2
(returns screen) slice 3 (cumulative TTWROR series and chart) and slice 4
(allocation at the closing day), contracts below, delivered the same day;
slice 5 (benchmarks) delivered the same day; slice 6 pending.**
Governs [#119](https://github.com/sergeyfarin/rekenraam/issues/119). The
[roadmap](../roadmap.md) owns order; ADR 0012 (reporting boundaries) and ADR
0017 (provider independence, honest incomplete data) govern this plan.

R13 answers one question: *how did a selected set of investment accounts
perform over a period, independently of when money was added or removed
(time-weighted), and for the money the owner actually put in
(money-weighted)?* It is read-only. It never writes postings, operations,
observations or audit events, and it never invents a transfer to make a
figure work.

## Owner decisions (2026-10-24)

| Question | Decision |
|---|---|
| Income whose cash lands on the other side of the portfolio boundary | **Classify by the position the facts name, not by the cash account; fall back to the cash side only when no position can be named, and disclose the fallback.** See *Why not cash side alone* below. |
| Annualization | **Only for periods of at least 365 days.** Shorter periods show the period figure; the p.a. figure is unavailable with reason `period_shorter_than_year`. |
| Money-weighted roots | **Only a proven unique root** (Descartes' rule; see the IRR section). Otherwise unavailable with `root_not_unique`; no root is ever chosen silently. |
| First implementation | **Backend read + tests** (slice 1). Screen, chart, allocation, benchmark and saved runs are later slices. |

### Why not cash side alone (checked 2026-10-24)

The ledger alone would classify a dividend by where its cash lands: inside the
portfolio it is return, outside it is nothing. That is exact, but only
meaningful if cash and holdings are linked, and they are not:

- A dividend journal posts only the cash account and the income (and
  withholding) account (`app/investments.go` dividend writer). Its fact,
  `investment_dividend_facts.commodity_id`, names an **instrument**, nullable
  since #393; no holding account is recorded.
- Any non-system asset or liability can settle a trade, dividend, payment in
  lieu or capital return (`settlementRole` in `app/investments_roles.go`).
  Nothing requires a holding and its cash to share a brokerage container;
  `parent_account_id` on a holding is optional.
- A capital return posts cash against `commodity_trading` and touches no
  holding account; its fact (`investment_capital_return_facts.account_id`)
  names the holding. Payment-in-lieu facts name a nullable holding account
  and a nullable instrument.

So with cash side alone, an owner whose broker pays dividends into a
checking account would silently lose every dividend from the return, and
including that checking account would turn salary and groceries into
"performance". Classifying by the named position avoids both, without
writing anything: it is a read-time classification of facts that already
exist. Where the facts do not name one position (unassigned instrument, or
the instrument held both inside and outside the boundary), the cash side is
the only honest remaining rule, and the response lists every such entry.

**No transaction is created.** A dividend paid into checking stays exactly
one recorded journal (checking +20 against income −20), so reconciliation
against bank and broker statements is unchanged. Inside the return
calculation alone, that journal is read as portfolio income plus a same-day
withdrawal. Neither half is stored, and balances, net worth, cashflow,
reconciliation and exports keep reading the ledger as posted.

## Inputs

| Input | Rule |
|---|---|
| `from`, `to` | Inclusive calendar days, `from <= to <= owner-local today`. The **opening valuation** is at the end of `from − 1`; the **closing valuation** at the end of `to`. `days = to − (from − 1)`. |
| Reporting currency | Required, a `currency` commodity. Every value and flow is converted into it. There is no native multi-currency return: unlike commodities have no shared denominator. |
| Portfolio accounts | Optional account ids, each expanded to its subtree (as of `to`). Each must be a non-system asset or liability. Omitted → the **default investment scope**: every non-system asset/liability account whose kind's base kind is `investment_container`, `investment_cash`, `security_holding` or `digital_asset`, plus their descendants. The response echoes the resolved ids and which rule produced them. |
| `max_staleness_days` | Optional, 1–31, default 7: the on-or-before window for prices and FX (the R2 `observed_on_or_before` rule). |

Accounts are resolved once, as of `to`, for the whole period (the cashflow
report's reason applies: re-resolving mid-period would let an account join or
leave and break the flow identity).

## Valuation `V(d)`

`V(d)` is the portfolio's market value at the end of day `d`: every posted
ledger balance of every portfolio account through `d`, per commodity,
converted at `d` by the shared `RateTable` (`app/valuation.go`): a currency
directly, a security or crypto unit through its quote currency (#281 chain),
under `live_raw_v1` admission and the knowledge cutoff of the read. Negative
balances (named shorts, a margin liability) contribute negative values.

- It is **ledger based, not lot based**: return needs quantities and prices,
  never cost basis, so unknown basis does not affect any R13 figure.
- An **unclassified short** (a negative countable balance beyond named short
  lots, the net-worth rule) makes that valuation unavailable
  (`unclassified_short`).
- A commodity with no usable price/FX inside the window makes that valuation
  unavailable; the rate gap names the commodity and reason exactly as the
  existing `ValuationCoverage` does.

## External flows

A journal entry is examined only if it is dated within `[from, to]`. Its
postings are partitioned:

- **B**: postings on portfolio accounts.
- **P**: postings on income or expense accounts — performance, never a flow.
- **T**: postings on the `commodity_trading` system account — internal
  conversion legs, never a flow and never valued.
- **O**: every other posting (non-portfolio asset/liability/equity, and the
  remaining system roles such as `opening_balance`, `transfer_clearing`,
  `external_investment_transfer_equity`).

**Whether the entry belongs to the portfolio** is decided by its *position
anchor*:

1. Entries of an investment operation whose journals post to subledger-managed
   holding accounts: anchored to those holdings. All inside → in; all outside
   → out; both (an internal transfer or corporate action across the boundary)
   → *mixed*, handled by the plain rule below.
2. Otherwise, a capital-return or payment-in-lieu fact naming a holding
   account: anchored to it.
3. Otherwise, a dividend, withholding-only or payment-in-lieu fact naming an
   instrument: on the anchor date (the operation's `ex` date when recorded,
   else the entry date) look at the instrument's ledger balance in every
   subledger-managed holding. Held (non-zero) only inside → in; only
   outside → out; inside and outside → *ambiguous*; nowhere → *not held*.
4. No anchor (ordinary entries, unassigned instrument, ambiguous, not held):
   the **cash side** — the entry belongs iff it has a B posting.

The entry's external flow, in the reporting currency at the entry date:

| Entry | Flow `F` (positive = into the portfolio) |
|---|---|
| **Delivery**: belongs, and is an external transfer in/out (or its correction), or has an O posting on an equity-class account (never the T leg: `commodity_trading` is equity-class in production, but internal) | `F = Σ value(B) + Σ value(P)` |
| Belongs otherwise (anchored in, mixed, or cash side with a B posting) | `F = −Σ value(O)` |
| No, anchored out, but has B postings | `F = +Σ value(B)` (the cash entering is a contribution, not return) |
| No and no B posting | not examined |

**Why deliveries differ (review 2026-10-24, finding 1).** Equity-class
counterparts (`opening_balance`, `external_investment_transfer_equity`, owner
equity) record *book* value, not what crossed the boundary. An external
transfer-in posts units against `commodity_trading` and, only when basis is
known, the carried basis against the equity bridge; a transfer-out posts units
against `commodity_trading` alone. `−Σ value(O)` would therefore count a
security worth 100 with carried basis 1 as a contribution of 1 (and 99 of
gain), unknown basis as 0, and a transfer-out as a loss. A delivery is
instead recognised at the dated **market value** of what entered or left the
portfolio, `Σ value(B)`, less any performance posted in the same entry. R13
stays independent of cost basis. A security delivery that cannot be priced on
its date is `flow_unavailable`.

Consequences, each covered by an expected case:

- A deposit from checking into broker cash: `O = checking −1000` → `F = +1000`.
- A buy inside the portfolio: only B, P, T → `F = 0`. A buy paid from
  checking into a portfolio holding: `F = +cash paid` (the trade-versus-close
  difference is return, as it should be).
- An external transfer-in of one unit priced 100: `F = +100` whatever its
  carried basis, known or unknown. A transfer-out is a withdrawal at market.
- A dividend into broker cash: B and P only → `F = 0`; the income is return.
- A dividend for a portfolio holding paid into checking: anchored in,
  `O = checking +20` → `F = −20` — a same-day withdrawal, so the dividend
  is return and leaves.
- A dividend for an outside holding paid into broker cash: anchored out,
  `F = +20` — a contribution, not return.
- Fees and withholding are expense postings: returns are **net of fees and
  taxes**. A withholding booked to a receivable outside the portfolio is a
  withdrawal of that receivable.
- An unassigned dividend into broker cash counts as return under the cash
  side and is disclosed; one into checking is not part of the portfolio and
  is disclosed.

**Every** cash-side fallback for an investment operation is disclosed with its
transaction, date, reason (`unassigned_instrument`, `held_inside_and_outside`,
`not_held`) and whether it was counted — including `not_held` cash that landed
outside the portfolio (finding 7). A counted fallback marks the figures as
resting on a provisional attribution; the list is how a reader sees it.

**Correction cancellation before splitting (finding 3).** Flows are first
netted per *economic identity* and date: all entries of one investment
operation correction chain (the original, its reversals and replacements)
dated the same day form one net flow, and every other journal entry is its
own identity. Only then is each net flow split by sign into `In(d)` or
`Out(d)`. An erroneous payment and its same-day reversal therefore leave no
flow at all, and genuine, independent same-day flows keep the
inflow-at-start / outflow-at-end convention. A flow that cannot be converted
makes every figure depending on it unavailable.

## Time-weighted return (TTWROR)

Daily linking, Portfolio Performance's documented convention (inflows at the
start of the day, outflows at its end), adopted explicitly:

```
1 + r(d) = (V(d) + Out(d)) / (V(d−1) + In(d))
R_twr    = Π_{d=from..to} (1 + r(d)) − 1
```

- A factor whose denominator and numerator are both zero (no capital, nothing
  happening) is 1.
- A denominator `≤ 0` otherwise makes TTWROR unavailable:
  `non_positive_capital`, with the date.
- Arithmetic is exact (`big.Rat`) over **unrounded** values and flows
  (`RateTable.ConvertExact`: the same rate selection, coverage and gaps as
  `Convert`, without rounding to the reporting currency's scale). Only shown
  amounts and the final ratios are rounded, once.

**Evaluation days.** A run of days whose factors are all valid telescopes to
the ratio of its end values, so the calculator values only the days the
product and its validity checks actually need (finding 4: compressing over a
day whose denominator would be non-positive hides it). Prices are positive
(a trigger rejects `price_value <= 0`), so if every per-commodity portfolio
balance is non-negative and at least one is positive, `V > 0` with no price
needed. The evaluation days are:

1. the opening and closing days;
2. `d − 1` for every inflow day and `d` for every outflow day;
3. for every day `d` with a portfolio posting whose balances at the end of
   `d − 1` or `d` are not *safe* (some commodity negative, or all zero):
   `d − 1` and `d`;
4. every day of a run with constant but unsafe balances (a short, negative
   cash, a margin liability), because prices alone can turn the value's sign.

The proof is about exact values, which is why the chain must divide exact
values too (second review, finding 2): ten units at €0.00001 are worth
€0.0001, positive capital, though €0.00 at cent scale. Dividing rounded
values would make the result depend on whether that day happened to be an
evaluation day. All-zero balances value at exactly 0 without prices. Every other evaluation
day needs its own valuation; a gap is `valuation_unavailable` for TTWROR only.
This is exactly the daily product for every portfolio, and as cheap as the
original breakpoint set for long-only portfolios.

## Time-weighted series (slice 3)

The response's `time_weighted_series` is the cumulative TTWROR from the
opening boundary through the end of each point's day, read from **the same
exact chain** as the headline figure, never recomputed per point or from
rounded values.

- **Points.** The opening day (always `0` when the chain is defined), then
  every day for periods of at most 92 days (`day`), every seventh day counted
  from the opening day for at most 731 days (`week`; no locale week start),
  or every calendar month end otherwise (`month`), and always the closing
  day. The interval is echoed. A long period therefore needs at most a few
  hundred valuations.
- **Why extra points are safe.** A point that is not already an evaluation
  day lies in a run whose validity the evaluation days proved, so its value
  only splits a telescoping run: it changes no factor, cannot introduce
  non-positive capital, and the closing point equals `time_weighted` (or
  carries its reason) by construction. Every ledger test asserts this.
- **Coverage per point: one failure policy.** Each point applies the
  headline's policy as if it closed the period, in this order: an unknown
  flow (`flow_unavailable`), the opening valuation, the point's **own**
  valuation (dated at the point), the first other required valuation before
  it, then the chain's non-positive capital at or before it. The headline
  TTWROR *is* the closing point, so a missing closing valuation outranks an
  earlier gap or capital failure in both (review of `df4a75b1`, finding 1:
  the series had picked the first failure in date order and disagreed). A
  point-only price gap removes only that point. The series' `valuation`
  covers only point-only days, so a chart gap never reads as a gap behind the
  headline figures; its stale quotes are disclosed with the chart (finding 2:
  they had been dropped).
- **Consequence.** The policy is per point, not monotone: a point after a
  capital failure can report a later missing price of its own, and points
  after a required gap report that gap. Gain and IRR keep their own
  dependencies (the two boundary values and the flows).
- **No capital** (zero opening value and no flows) makes every point
  `no_capital`. Before the first inflow of a portfolio that starts empty,
  factors are 0/0 and the points read `0`, as the chain does.
- **Presentation.** The chart draws calendar time on x and breaks the line at
  every missing point; it never interpolates or fills a gap. A table of every
  point, with its reason in words, is the exact and accessible alternative.

## Allocation at the closing day (slice 4)

The response's `allocation` splits the closing value at `to`. It reads the
same image, scope and `RateTable` as `closing_value`, so with every balance
valued its denominator **is** the closing value (exact conversion is linear;
the tests assert equality). No separate request, date or scope exists.

- **Rows.** Each in-scope (account, commodity) ledger balance at the end of
  `to` is valued exactly at `to`. Its sign makes it **long** or **short**: a
  negative cash balance, a margin liability or a named short is short. A
  group sums its long balances and its short balances as *separate rows*;
  long and short are never netted inside one label. A single account's
  balance in one commodity is the ledger's own net and is not split.
- **Dimensions.**
  - *Account*: the outermost in-scope ancestor (the brokerage container a
    holding sits under), else the account itself.
  - *Asset type*: `cash` for a currency commodity; otherwise the instrument
    version's `instrument_type` effective on `to`; `unclassified` when no
    version is effective that day (never borrowed from a later version).
  - *Currency*: a currency is its own; otherwise the effective instrument
    version's quote currency, else none (`currency_id` null).
- **Weights.** `value / denominator`, exact, six places half away from zero;
  short weights are negative and each dimension's exact weights sum to 1
  (shown weights and values are each rounded once, so their sums may differ
  in the last place).
- **Unknown values.** A balance with no usable price or rate is listed in
  `unknown` by its units and is in no row or weight. `basis` is then
  `known_only` and the denominator is the sum of the known values; with
  everything valued it is `closing_value`.
- **Undefined shares.** A denominator `≤ 0` keeps every value and sets every
  weight null with `weights_reason: non_positive_value`.
- **Unclassified short** (the net-worth rule, as for `closing_value`) makes
  the whole allocation unavailable (`reason: unclassified_short`): its value
  is no better defined than the closing value's.
- **Coefficient range (review of `62f9201d`, finding 1).** Every reported
  amount, and every balance valued along the way, is written at the
  reporting (or posting) scale or, when that exceeds 38 digits, at a
  shallower scale that drops only trailing fractional zeros, so its value is
  unchanged. An amount that fits at no exact scale fails the whole read with
  `LEDGER_OVERFLOW` (422), as ledger balances do; it is never a null figure
  without a reason, and never relabelled a missing price. Offsetting 10^37
  balances therefore show as ±10^37 rows at scale 0.
- **Presentation.** One dimension at a time (account, asset type, currency),
  each row with value, share and a bar; a short row says "Short" in words.
  Shares carry no "+" sign, a short share a minus. Known-only basis,
  undefined shares and the unknown list are disclosed beside the rows. On a
  known-only basis a non-positive denominator says the *priced* holdings are
  worth zero or less and the total is unknown (finding 2): an unpriced
  portfolio's value is not zero.

## Benchmark (slice 5)

The response's `benchmark` (null unless requested) compares the portfolio
with **one owner-selected security or crypto commodity**
(`benchmark_commodity_id`), read as a hypothetical single unit held from the
opening boundary through `to`. Any commodity with observations qualifies,
held or not; manual observations are as valid as provider ones, and no live
provider is needed.

- **Value.** One unit is valued exactly in the reporting currency on each
  day by its own `RateTable` over the read's window and staleness rule, so a
  benchmark quoted in another currency includes the FX change, as the
  portfolio's figures do (#281 chain). Its gaps and stale quotes are reported
  in `benchmark.valuation` and never mark the portfolio's coverage incomplete.
- **Basis, declared, never inferred.** `benchmark_basis` is `price_return`
  (the default: raw observations under `live_raw_v1`, distributions
  excluded) or `total_return`: only observations recorded with
  `adjustment_basis = total_return` price the unit, under
  `live_total_return_v1` (`live_raw_v1` with that one change; currency legs
  stay raw). A raw price never stands in for a total-return one; a
  total-return request without such observations is unavailable, and the gap
  reads `incompatible_quote_basis`. `dividend_adjusted` and `split_adjusted`
  series are not accepted in v1.
- **Splits (review of `eeab1a2a`, finding 1).** Raw quotes are per current
  unit, so a price-return benchmark holds the units one unit becomes: from
  each effective recorded split of the instrument (any holding, inside or
  outside the portfolio; reversed and replaced splits excluded), its unit
  count is multiplied by the ratio on the split's effective date, exactly as
  a holding's ledger units change that day. A split is therefore neutral, as
  it is for the portfolio; reading raw quotes alone turned a 2:1 split into
  −50 %. The same split recorded in several holdings applies once; different
  ratios on one day are a conflict, and that day and every later point are
  `conflicting_splits` (dated the day) rather than a guessed ratio. The
  applied splits are listed in `benchmark.splits`. An instrument with an
  unrecorded split cannot be detected; the list shows what was applied. A
  total-return series is a continuous index level, never split-adjusted, and
  its list is empty. Other corporate actions (spin-offs, mergers, cash in
  lieu) are distributions and stay excluded from a price return.
- **Return over the same breakpoints.** A benchmark has no flows, so its
  daily chain telescopes exactly to `V(d) / V(open)` (prices are positive);
  requiring the portfolio's evaluation days would add failures that change no
  figure. `benchmark.points` are the portfolio series' dates; the headline
  `time_weighted` is the closing point and is annualized under the same
  365-day rule.
- **Failure policy.** A missing opening value makes every point
  `valuation_unavailable` dated the opening day; a missing point value removes
  that point alone, dated itself; then a split conflict at or before the point
  (the closing point is the headline).
- **No other figure changes.** Requesting a benchmark leaves every portfolio
  figure, series point, allocation and coverage byte-identical (tested).
  No excess-return figure is computed in v1: the reader compares the two
  returns side by side; an arithmetic or geometric difference is a separate
  choice.
- **Presentation.** The screen offers instruments as benchmarks, keeps the
  choice and basis in the URL (an id that is not an instrument is disclosed
  and never sent), shows the benchmark's period and annual figures with its
  basis in words and the splits it held through, and draws it on the chart's
  axes as a dashed line (a non-colour cue) with a legend and its own column
  in the table of points. A coverage gap is named by its missing leg (finding
  2): a price, a total-return price, or the quote currency's exchange rate
  when the gap carries `via_commodity_id`.
- **Recording total-return observations.** The general price API accepts any
  adjustment basis; the browser's manual price form records `raw` only, so a
  total-return series is entered through the API until that form offers the
  basis.

**Benchmark cases (slice 5).** S1 portfolio; benchmark IDX priced in EUR.

| Case | Observations | Benchmark |
|---|---|---|
| B1 price return | raw 200 (01-31), 220 (07-31), 210 (close); total_return 100 (01-31), 999 (02-14), 115 (close) | `0.050000`, p.a. `0.050000`; 02-07 `0` (7-day-old price); 02-14 `valuation_unavailable` on its own date (the total_return 999 is not read); 08-01 `0.100000`; portfolio `0.090909`, its coverage complete |
| B2 total return | B1 observations | `0.150000`; 02-14 `8.990000`; 08-01 unavailable (no raw price is borrowed); policy `live_total_return_v1` |
| B3 basis refused | raw only, 50 → 55 | total return: headline, p.a. and every point `valuation_unavailable` dated 01-31; gap `incompatible_quote_basis` |
| B4 FX | SPX 100 USD × 0.90 → 110 USD × 0.80 | 90 → 88 EUR: `−0.022222`; coverage names the USD leg |
| B5 closing gap | raw 200 (01-31), 210 (06-30) | full year: headline and p.a. unavailable dated the closing day; 02-07 `0` |
| B6 short period | B5, 02-01..06-30 | `0.050000`; p.a. `period_shorter_than_year` |
| B7 split | TEST itself; 2:1 split 07-01; raw 101 (06-27), 51 (07-01), 50 (close) | portfolio and benchmark `0.000000`; 06-27 `0.010000`; 07-04 `0.020000` = 2 × 51 / 100; splits `[2:1 on 07-01]` |
| B8 total return | B7 plus total_return 100 → 110 | `0.100000`, no split applied |
| B9 recorded twice | B7 split in two holdings | applied once, `0.000000` |
| B10 conflict | 2:1 and 3:1 on 07-01 in two holdings | 06-27 `0.010000`; 07-04, headline and p.a. `conflicting_splits` dated 07-01 |
| B11 reversed | B7 split reversed, period to 08-01, 102 on 08-01 | `0.020000`, no split applied |

**Review of `eeab1a2a` (2026-10-25).** Both findings accepted: (1) a
price-return benchmark read raw quotes for one fixed unit, so a recorded 2:1
split showed −50 % against a neutral portfolio — now split-adjusted from the
recorded splits (B7–B11); (2) a missing quote-currency rate was disclosed as a
missing price — now named as the exchange rate (browser case in
`investment-returns.spec.ts`). Independent coverage, strict total-return
selection, FX inclusion, shared chart dates and axes, URL persistence and
API-only total-return entry were reviewed and kept.

## Money-weighted return (IRR)

Investor-perspective flows with the same intraday convention, measured in
days `t` from the opening boundary, Actual/365:

```
−V(open) at t = 0
−In(d)   at t = (d − open) − 1     (start of day)
+Out(d)  at t = (d − open)         (end of day)
+V(to)   at t = days
Σ c_i (1 + irr)^(−t_i / 365) = 0
```

- Uniqueness is accepted **only** under Descartes' rule of signs: exactly one
  sign change in the time-ordered, same-offset-merged flows proves exactly one
  root with `irr > −1`. Norström's criterion is **not** used (finding 2): it
  proves a unique *non-negative* rate, and `−1800, +2270, −309, +10` has
  roots at −95 %, −90 % and +11.1111 %. Anything else is `root_not_unique`.
  Interleaved contributions and withdrawals are therefore unavailable in v1;
  that is the conservative cost of never choosing among roots.
- All flows of one sign (nothing invested or nothing returned): `no_root`.
- Solver (finding 6): solve in log-growth space over the period,
  `u = ln(1 + period MWR)`, so terms are `c_i · exp(−u · t_i / days)`.
  Bracket from `[−1, 1]`, doubling outward until the NPV changes sign; under
  Descartes the sign change must exist, so failing to bracket before
  `|u| > 700` is `numerical_limit`, never `no_root`. Then 200 deterministic
  bisection steps in `float64` over flows scaled by their largest magnitude.
  These are ratios, not money: floating evaluation is allowed here, rounded
  to six places, and never stored as a monetary value.
- The period money-weighted return is `exp(u) − 1`; the annual IRR is
  `exp(u · 365 / days) − 1`, shown only for `days ≥ 365`, and
  `numerical_limit` if it overflows.

## Presentation, annualization and day count

- Every return is a decimal fraction string, rounded half away from zero to
  six places (`0.090909` = 9.0909 %). The screen formats percentages.
- Annualized figures only when `days ≥ 365`: TTWROR p.a.
  `= (1 + R_twr)^(365/days) − 1`; IRR is already annual. Shorter periods
  return `null` with `period_shorter_than_year`. Leap days count as days
  (Actual/365, as Portfolio Performance does).
- Also returned, when available, in the reporting currency at its scale:
  opening value, closing value, contributions `ΣIn`, withdrawals `ΣOut`, and
  absolute gain `V(to) − V(open) − ΣIn + ΣOut`.

## Failure states and coverage

Each figure is independent: a failure removes only the figures that need the
missing input (ADR 0017).

| Reason | Removes |
|---|---|
| `valuation_unavailable` / `unclassified_short` at the opening or closing day (with date) | that value, absolute gain, TTWROR, IRR |
| `valuation_unavailable` / `unclassified_short` at any other evaluation day | TTWROR only (finding 5: gain and IRR need only the boundary values and the flows) |
| `flow_unavailable` (a flow could not be converted) | contributions/withdrawals, gain, TTWROR, IRR |
| `non_positive_capital` (with date) | TTWROR |
| `no_root`, `root_not_unique`, `numerical_limit` | IRR |
| `period_shorter_than_year` | annualized TTWROR/IRR only |
| `non_positive_growth` (period return ≤ −100 %) | annualized TTWROR only |
| `no_capital` (opening value zero and no flows) | TTWROR and IRR |

The response carries the shared `ValuationCoverage` (rate uses and gaps) and
the attribution fallback list. Stale (in-window, earlier-dated) prices are
used and reported as stale, never refused.

## Reproducibility

Slice 1 is a **live read**: one disposable consistent image of the database
(the gains reader's `LiveGainsSnapshot`), today's effective history and the
UTC knowledge instant of the read, echoed as `knowledge_cutoff`. A later
correction restates an earlier period, as every live report does. Saved,
immutable return runs (input fingerprint, frozen output bytes) are slice 6
and follow the R18 gains-run pattern; they are not implied by slice 1.

## Expected cases (independent, hand-derived)

Figures were computed outside the app with Python `fractions`/`decimal`
(scratch script, not code under test). All in EUR, one security ACME priced
in EUR, opening boundary 2026-01-31, period 2026-02-01..2027-01-31
(365 days). The portfolio is broker cash plus the ACME holding; *Checking*
is outside.

| Case | Ledger | Breakpoint values | TTWROR | IRR |
|---|---|---|---|---|
| S1 no flows | cash 100, 10 ACME; price 100 → 110 | 1100 → 1200 | `0.090909` | `0.090909` |
| S2 flows, dividend, fee inside | S1 + deposit 500 on 05-02 (t=90, start); dividend 20 into broker cash 07-31; fee 5 from broker cash 10-31; withdrawal 300 to Checking 11-01 (t=274, end); prices 105 (05-01), 120 (10-31), 125 (01-31) | 1100, 1150 (05-01), 1815 (10-31), 1515 (11-01), 1565 (close) | `0.187954` = 1150/1100 · 1815/1650 · 1565/1515 − 1 | `0.189483` |
| S3 dividend to Checking, attributed | S1 + dividend 20 for ACME paid into Checking 07-31 (t=181); price 105 on 07-31 | 1100 → 1150 (07-31) → 1200 | `0.109881` = 1170/1100 · 1200/1150 − 1 | `0.110074` |
| S4 unassigned dividend to Checking | S1 + unassigned dividend 20 into Checking | as S1 | `0.090909`, fallback disclosed (`unassigned_instrument`, not counted) | `0.090909` |
| S5 missing closing price | S1 without the closing price, beyond the window | — | unavailable `valuation_unavailable`, ACME gap | unavailable |
| S6 short period | S1 over 2026-02-01..2026-06-30 | — | period figure only, p.a. `period_shorter_than_year` | likewise |
| E1 solver | flows −1000, +2300, −1320 | — | — | `root_not_unique` |
| E2 solver | flows −1000 (t=0), +1100 (t=365) | — | — | `0.100000` |
| R1 delivery at market | S1 base, prices 100 on 07-30, 07-31, close; external transfer-in of 1 ACME on 07-31, carried basis 1.00 **or** unknown | 1100 (07-30), 1200 (close) | `0.000000` = 1100/1100 · 1200/1200 − 1; contribution 100, gain 0 | — |
| R2 solver, three roots | −1800, +2270 (t=365), −309 (t=730), +10 (t=1095) | — | — | `root_not_unique` (Norström would accept +11.1111 %) |
| R3 reversal neutrality | S1 base, prices 100 (07-30), 110 (07-31, close); dividend 20 for ACME into Checking on 07-31, reversed | as if neither existed | `0.090909`, no flows | `0.090909` |
| R4 hidden negative capital | broker cash only: 100 → −100 (fee 200 on 07-30) → 100 (income 200 on 07-31) | 100, −100 (07-30) | `non_positive_capital` on 2026-07-30 | — |
| R5 missing intermediate price | S1 base; deposit 500 on 05-02; no price within 7 days of 05-01; price 100 at close | open 1100, close 1600 | `valuation_unavailable` (2026-05-01) | gain 0, IRR `0.000000` |
| R6 one-day IRR | −100 (t=0), +110 (t=1) | — | — | period `0.100000`; annual not shown |
| R7 not-held disclosure | S1 base; all ACME sold 07-01; ACME dividend into Checking 07-31, no ex date | — | — | fallback `not_held`, not counted, disclosed |

**Series cases (slice 3).** S1 base (ACME 100 at the opening boundary):

| Case | Ledger | Points |
|---|---|---|
| C1 daily | period 02-01..02-05; prices 105 (02-02), 110 (02-04); deposit 500 into broker cash 02-03 | 01-31 `0`, 02-01 `0`, 02-02 `0.045455`, 02-03 `0.045455`, 02-04 `0.077135` = 1150/1100 · 1700/1650 − 1, 02-05 `0.077135` = headline |
| C2 weekly, local gap | full year; prices 105 (07-31), 110 (close) | 54 points; 02-07 `0` (7-day-old price); 02-14 and 07-25 `valuation_unavailable` on their own dates; 08-01 `0.045455`; close `0.090909`; headline coverage complete, series coverage names ACME |
| C4 chain stop | R4 ledger (broker cash only) | 07-25 `0`; 08-01 and close `non_positive_capital` dated 07-30 |
| C5 required gap | R5 ledger plus a price on 05-09 | 02-07 `0`; 05-02 own gap dated 05-02; 05-09 and close `valuation_unavailable` dated 05-01 |
| C6 closing gap first | R5 ledger, no closing price | headline and closing point `valuation_unavailable` dated the closing day, not 05-01 |
| C7 closing gap over capital | prices 100 on 07-29..07-31; fee 1,200 from broker cash 07-30, income 1,200 on 07-31; no closing price | 08-01 `non_positive_capital` dated 07-30; headline and closing point `valuation_unavailable` dated the closing day |
| C8 chart-only staleness | prices only at the boundaries (01-31, 02-05), period 02-01..02-05 | headline quotes all exact; series coverage marks 02-01..02-04 stale |

**Allocation cases (slice 4).** S1 base with TEST classified as a stock
quoted in EUR (unless stated), closing price 110:

| Case | Ledger | Allocation |
|---|---|---|
| A1 | holding under a brokerage container | denominator 1200 = closing value; account: container 1100 `0.916667`, broker cash 100 `0.083333`; asset type: stock `0.916667`, cash `0.083333`; currency: EUR long `1.000000` |
| A2 | fee 150 from broker cash 12-01 | denominator 1050; EUR long 1100 `1.047619` and EUR short −50 `−0.047619`, two rows; cash short `−0.047619` |
| A3 | no closing price | `known_only`, denominator 100, cash `1.000000`; unknown: holding, 10 TEST |
| A4 | fee 1,200 from broker cash | denominator 0; `non_positive_value`; rows 1100 long and −1100 short, no weights |
| A5 | TEST without an instrument version | asset type `unclassified` `0.916667`; currency none 1100 `0.916667` beside EUR 100 |
| A6 | −1 TEST posted to broker cash, no short lot | closing value and allocation `unclassified_short` |
| A7 | two extra broker-cash accounts at +10^37 and −10^37 EUR (scale 0) | denominator 1200; those rows exactly ±10^37, written at scale 0 |
| A8 | +10^37 TEST in broker cash, closing price 10.01 | value 1.001 × 10^38 fits no exact scale: the read fails `LEDGER_OVERFLOW` |

Slice 1 tests pin these literals; they never derive expected values from the
calculator. R1–R7 are the reproducers from the 2026-10-24 review of
`e30413a5`; they failed before the corrections recorded here.

**Second review of `ae084c4d` (2026-10-24).**

1. *Trading legs read as deliveries* — accepted. Delivery detection matched
   any equity-class O posting, and production seeds `commodity_trading` as
   equity, so an internal buy invented a contribution and a write-off became
   a withdrawal. Detection now excludes the T leg, and the shared test
   fixture seeds `commodity_trading` as equity like production (it used
   asset, which hid the defect). Cases: internal buy of 1 ACME for 100 at a
   110 close → contribution 0, gain 110, TTWROR `0.100000`; write-off of
   10 ACME → withdrawal 0, gain −1000, TTWROR `−0.909091`.
2. *Rounding to zero* — the defect accepted, the expected answer not. The
   review expected `non_positive_capital` for a holding valued
   1000 → €0.0001 → 1000. Every exact daily value there is positive, so the
   daily product is defined and is exactly 1 (`0.000000`); a zero
   denominator exists only after rounding, which is presentation, not
   capital. The real defect was the inconsistency: had that day been an
   evaluation day, rounded values would have divided by zero. The fix makes
   the chain exact rather than making the proof price every day of every
   portfolio. Cases: the telescoped sequence is `0.000000`; with a
   transfer-out of 1 unit on the €0.00001 day (forcing evaluation),
   (0.00009 + 0.00001) / 1000 · 900 / 0.00009 − 1 = `0.000000`, and a
   cent-rounded chain fails this case.

## Slices

1. **Backend live read** — `GET /api/v1/investments/returns`, OpenAPI,
   calculator and ledger/attribution reads, the cases above. *Delivered.*
2. **Returns screen** — period/account/currency selection, figures with
   reasons, fallback disclosure; loading/empty/error/success; mobile.
   *Delivered 2026-10-24* at `/app/investments/returns`: the selection lives
   in the URL; the reporting currency defaults to the book's default
   currency for new UI (shown and changeable, never a hidden base currency);
   presets are year to date and the last 12 months (the shortest period with
   annual figures); every unavailable figure shows its reason in words, and
   attribution fallbacks link to their transactions. A review of
   `f0b3b8ec` (four findings, all accepted) added: returns are invalidated by
   price corrections and are always refetched on a visit (`staleTime` 0,
   with an "updating" note), because the ledger writers that change them have
   no single invalidation hook; archived and closed accounts stay selectable,
   labelled with their status; a selected id that is not a selectable account
   is listed, removable and never sent, with a reset to every investment
   account; and missing and stale coverage are disclosed independently.
3. **Cumulative TTWROR series and chart** — points from the same chain,
   coverage per point (see *Time-weighted series*). *Delivered 2026-10-24*:
   `time_weighted_series` on the returns read, and a line chart with a table
   of points on the returns screen.
4. **Allocation at a date** — weights by account, instrument type and
   currency; denominator = closing portfolio value; unpriced holdings in an
   explicit unknown group and weights labelled known-only. Long and short
   are never netted inside a group label. *Delivered 2026-10-24* as
   `allocation` on the returns read at `to` (see *Allocation at the closing
   day*) and an allocation panel on the returns screen.
5. **Benchmarks** — an owner-selected price series (manual observations are
   valid); price-return unless the series is declared total-return;
   benchmark TTWROR over the same breakpoints; no live provider required.
   *Delivered 2026-10-24* as `benchmark` on the returns read (see
   *Benchmark*), a benchmark card and a dashed chart line on the screen.
6. **Saved return runs** — immutable output and input fingerprint, as R18.

Targets and rebalancing are **deferred** (no trade execution is implied,
per the product direction). Security-level (per-instrument) returns need an
instrument boundary with commodity-attributed income and are a separate
contract after slice 4.
