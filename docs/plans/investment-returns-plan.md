# R13 investment returns contract (`returns_v1`)

Status: **contract accepted by the owner 2026-10-24 (four policy answers
recorded below); slice 1 (backend read) delivered 2026-10-24. Later slices pending.**
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
| Money-weighted roots | **Only a proven unique root.** Otherwise unavailable with `root_not_unique`; no root is ever chosen silently. |
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

| Belongs? | Flow `F` (positive = into the portfolio) |
|---|---|
| Yes (anchored in, mixed, or cash side with a B posting) | `F = −Σ value(O)` |
| No, anchored out, but has B postings | `F = +Σ value(B)` (the cash entering is a contribution, not return) |
| No and no B posting | not examined |

Consequences, each covered by an expected case:

- A deposit from checking into broker cash: `O = checking −1000` → `F = +1000`.
- A buy inside the portfolio: only B, P, T → `F = 0`. A buy paid from
  checking into a portfolio holding: `F = +cash paid` (the trade-versus-close
  difference is return, as it should be).
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

Every cash-side fallback for an investment operation is disclosed with its
transaction, date, reason (`unassigned_instrument`, `held_inside_and_outside`,
`not_held`) and whether it was counted. Per day, positive entry flows sum to
`In(d)` and the magnitudes of negative ones to `Out(d)`; an entry's flow is
netted before that split. A flow that cannot be converted makes every figure
depending on it unavailable.

## Time-weighted return (TTWROR)

Daily linking, Portfolio Performance's documented convention (inflows at the
start of the day, outflows at its end), adopted explicitly:

```
1 + r(d) = (V(d) + Out(d)) / (V(d−1) + In(d))
R_twr    = Π_{d=from..to} (1 + r(d)) − 1
```

Days with no flow telescope, so the calculator values only the *breakpoints*:
the opening day, the closing day, `d − 1` for every day with an inflow and
`d` for every day with an outflow. This is algebraically identical to the
daily product; it is not an approximation.

- A factor whose denominator and numerator are both zero (no capital, nothing
  happening) is 1.
- A denominator `≤ 0` otherwise makes TTWROR unavailable:
  `non_positive_capital`, with the date.
- Arithmetic is exact (`big.Rat`) over values already rounded to the reporting
  currency's scale by `RateTable`; only the final ratio is rounded.

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

- Uniqueness must be **proven** before solving: Descartes' rule (one sign
  change in the time-ordered flows) or Norström's criterion (the running sum
  starts non-zero and changes sign exactly once). Otherwise
  `root_not_unique`.
- All flows of one sign (nothing invested or nothing returned): `no_root`.
- Solver: deterministic bisection on `irr ∈ (−0.999999, 1e6)`, 200
  iterations in `float64` over flows scaled by their largest magnitude. If
  the bracket does not straddle zero: `no_root`. These are ratios, not
  money: floating evaluation is allowed here, rounded to six places, and
  never stored as a monetary value.
- The period money-weighted return is `(1 + irr)^(days/365) − 1`.

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
| `valuation_unavailable` (rate/price gap at a breakpoint, with date) | figures needing that `V(d)` (opening/closing values, absolute gain, TTWROR, IRR) |
| `flow_unavailable` (a flow could not be converted) | contributions/withdrawals, gain, TTWROR, IRR |
| `unclassified_short` (with date) | as `valuation_unavailable` |
| `non_positive_capital` (with date) | TTWROR |
| `no_root`, `root_not_unique` | IRR |
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

Slice 1 tests pin these literals; they never derive expected values from the
calculator.

## Slices

1. **Backend live read** — `GET /api/v1/investments/returns`, OpenAPI,
   calculator and ledger/attribution reads, the cases above. *Delivered.*
2. **Returns screen** — period/account/currency selection, figures with
   reasons, fallback disclosure; loading/empty/error/success; mobile.
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
