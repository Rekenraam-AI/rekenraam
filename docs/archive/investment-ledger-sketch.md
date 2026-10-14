# Investment ledger sketch (historical)

Status: superseded pre-implementation sketch, archived 2026-10-14 from
`docs/plans/transaction-ledger-core-plan.md`. The original section below is
preserved verbatim, including its older status explanation. Current behavior
is governed by [ADR 0012](../adrs/0012-investment-ledger-subledger-and-reporting-boundaries.md),
[ADR 0013](../adrs/0013-investment-operations-and-position-sides.md) and
[the operation contract](../plans/investment-operation-refactor-plan.md);
[the feature ledger](../implemented.md) records shipped capabilities.

## Investment Rule (superseded — historical note only)

This section is the pre-implementation sketch of lot accounting written before
investments shipped. **It is stale and no longer describes the real schema or
rules.** The shipped design lives in `docs/plans/investments-plan.md` and the actual
tables are `investment_lots`, `investment_lot_events` (event-sourced, not the
`investment_lot_assignments` sketch below), `investment_provider_events`,
`investment_event_suggestions`, `investment_automation_rules`,
`cost_basis_profiles`, and `dividend_defaults` in
`backend/migrations/0001_initial_schema.sql`. Realized gain/loss on sells is
computed by the app service from lot assignments, matching the original
intent; sells are fully supported (not blocked) as of R12/Trading 212 Slice 4b.
The paragraphs and schema sketch below are preserved for historical rationale
only — do not treat them as current design.

Investment buys can be represented as postings and can create lots.

Investment sells must not allow arbitrary user-entered realized gains once lot
accounting is in scope. The app service must compute realized gain/loss from
lot assignments and then generate the gain/loss postings.

Until lot assignment is implemented:

- allow simple buys that create lots if the lot table is part of the slice
- allow cash dividends
- block investment sells that require realized gain/loss computation, or mark
  them as unsupported in the API

Do not let users manually type realized gain postings for investment sells and
call them authoritative lot accounting.

Original future-tables sketch (superseded by the shipped schema above):

```text
investment_lots
- id
- book_id
- account_id
- commodity_id
- opened_by_posting_version_id
- opened_on
- status
- metadata_json

investment_lot_assignments
- id
- book_id
- lot_id
- posting_version_id
- quantity_value
- quantity_scale
- assignment_kind: acquire | dispose | transfer_in | transfer_out
- cost_value
- cost_scale
- cost_commodity_id
```
