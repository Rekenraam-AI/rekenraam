# R13 investment returns contract (`returns_v1`)

Status: **contract accepted by the owner 2026-10-24 (four policy answers
recorded below); slice 1 (backend read) delivered 2026-10-24 and corrected
the same day after two reviews (`e30413a5`: seven findings, recorded inline
as "finding N"; `ae084c4d`: two, recorded under the expected cases). Slice 2
(returns screen) delivered the same day; slices 3–6 pending.**
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
3. **Cumulative TTWROR series and chart** — daily or bucketed points derived
   from the same chain (requires valuations at each point, so coverage is per
   point).
4. **Allocation at a date** — weights by account, instrument type and
   currency; denominator = closing portfolio value; unpriced holdings in an
   explicit unknown group and weights labelled known-only. Long and short
   are never netted inside a group label.
5. **Benchmarks** — an owner-selected price series (manual observations are
   valid); price-return unless the series is declared total-return;
   benchmark TTWROR over the same breakpoints; no live provider required.
6. **Saved return runs** — immutable output and input fingerprint, as R18.

Targets and rebalancing are **deferred** (no trade execution is implied,
per the product direction). Security-level (per-instrument) returns need an
instrument boundary with commodity-attributed income and are a separate
contract after slice 4.
