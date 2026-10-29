# ADR 0021: Entered Asset Values

## Status

Accepted (2026-10-29), owner decision on #454 (option A: dated values kept
beside the ledger, not balance-adjustment postings). Applies ADR 0012's
separation of canonical facts from valuation projections to assets that no
provider prices, and ADR 0017's incomplete-data rules to their values.

## Context

Net worth depends on values no provider supplies: a house, a vehicle, a pension
or another statement-based account. Today such an account is worth the sum of
its postings. A property account with no postings counts as zero, and a pension
last updated a year ago looks current; neither can be disclosed the way missing
or out-of-date prices are (owner decision 2026-10-29,
[Home and navigation design](../design/home-and-navigation.md#principles)).

One asset can carry several legitimate values on the same date. A Dutch house
has the owner's market estimate, an assessed WOZ value (valued as of 1 January
of the previous year, used for eigenwoningforfait or for box 3 on a second home)
and what was actually paid in. A car loses value every year without any cash
moving. The [accounts system design](../design/accounts-system-design.md) left
open whether value changes are "explicit transactions or later price
observations".

Two storage models were considered:

- **A. Dated values beside the ledger.** The ledger keeps what was paid in; a
  separate dated value says what the asset is worth.
- **B. Balance-adjustment postings.** Each new value posts the difference
  against a revaluation category, so the ledger balance is the value.

## Decision

1. **Entered values are dated observations per account, never postings.** The
   ledger keeps only real money movements: purchase, improvements,
   contributions, sale proceeds. Recording, revising or voiding a value never
   creates, edits or reverses a transaction. Spending, cashflow, budgets,
   reconciliation and the core ledger CSV/QIF export are unaffected.
2. **Each value has a basis.** `estimate` is the owner's view of what the asset
   is worth (the pension statement, a market estimate, a car's resale value).
   `assessed` is an official value for tax, carrying its valuation date
   (peildatum), the tax year it applies to and when it was recorded. Each
   account states which basis net worth uses (default `estimate`). A read never
   falls back from one basis to another; an owner who uses the WOZ as their
   estimate sets the account to use `assessed`, and the choice is shown.
3. **Value on a date rolls forward.** Value(d) = the latest value of the chosen
   basis on or before d, plus the account's postings after that value's date up
   to d. Without any value the account is worth its ledger balance, so existing
   accounts keep working. A renovation adds to the last estimate; pension
   contributions add to the last statement until the next one.
4. **Coverage follows the shared rules.** An account that needs a value is
   **missing** on a date when it has neither a value nor postings by then: net
   worth is unavailable, never counted as zero. It is **out of date** when its
   latest value (or, with none, its latest posting) is older than its update
   interval: the value is used and counted. Accounts of kind `property` and
   `vehicle`, and accounts the owner marks (for example a pension held as
   `other_asset` or `fund_holding`), need a value. The update interval is set
   per account; defaults are 3 months for statement-based accounts, 12 months
   for property and vehicles, or never.
5. **A sale or closure ends the value.** Closing the account, or recording its
   sale, sets its value to zero from that date; a later value is refused. The
   sale proceeds are posted in the ledger as usual. Without this, the last
   estimate minus the proceeds would survive the sale.
6. **The difference is an unrealized change, not income.** Value minus ledger
   balance is shown as an unrealized gain or loss (a house's appreciation, a
   car's depreciation), and a net worth report may split a period's change into
   money in and out versus value changes. No realized gain, revaluation or tax
   posting follows from it (ADR 0012).
7. **Depreciation is a schedule that produces estimates.** A vehicle may carry a
   schedule (straight-line over N years to a residual value). It produces
   `estimate` values on its dates, distinguishable from typed ones, and the
   owner can override any of them. It posts nothing.
8. **Values keep history.** Revisions and voids are versioned with actor, time
   and reason; nothing is hard-deleted. A read can ask for values as known at a
   time, as for prices (ADR 0008).

## Consequences

- An account shows two figures, labelled: its value (with date and basis) and
  what was paid in (the ledger balance). Reconciliation always works on the
  ledger balance.
- Spending does not include a car's loss of value. A "cost of ownership" view
  may later add value changes as a derived line; it never posts them.
- The core ledger CSV and QIF cannot carry entered values; the export screen
  says so. The full archive and backups include values, bases and intervals,
  and the ledger self-check covers them (no value after closure, no value
  before the account opened).
- Values in another currency follow the shared FX rules: a missing rate makes
  the converted value unavailable.
- Tax projections (box 3, eigenwoningforfait) read `assessed` values in a later
  report; this ADR adds no tax rule.
- Liabilities stay ledger-only. Loan or mortgage statements are reconciled
  against postings, not entered as values.
- An opt-in "post this change" action (a revaluation posting linked to an entered
  value, kept out of cashflow) remains possible later under ADR 0012 if a real
  need appears. It is not part of this decision.

## Implementation And Validation

Delivery is tracked in #454 in three slices: estimates with roll-forward,
update intervals, closure and coverage; then `assessed` values and the
per-account basis choice; then depreciation schedules. Net worth, Home (#437)
and the Review page (#453) read one shared valuation coverage.

Backend tests derive net worth by hand for missing, out-of-date, revised, voided
and rolled-forward values across a date range, including the boundary before the
first value and after closure; backup/restore and archive round-trips; and the
self-check rules above.
