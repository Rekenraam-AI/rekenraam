# Dividend income attribution coverage (#244)

Governed by product requirements and ADRs 0011–0013 and 0017. Builds on the
payer facts of #229, CSV mapping (#241) and imported dividend review/repair
(#242). Read-side only: no schema, export or ledger change.

`GET /api/v1/investments/income-coverage` is one composed read. It never writes.

- **Attributed income.** Per paying instrument and cash currency, exact sums of
  effective, posted named cash dividends (`dividend_gross`/`withholding`
  components) with an instrument fact: gross, withholding, net cash and counts.
  Separate withholding payments add tax but no gross. `income_state` is `none`
  (no recorded gross income) or `known`; `withholding_state` is `none`,
  `known_zero` or `known`. A named dividend's withholding is an owner-confirmed
  command fact, so no withholding component is a declared zero.
- **Unassigned investment cash.** Identifiable only through durable facts:
  (a) named dividends without a payer fact (gross/tax known, reason
  `payer_unassigned`); (b) committed Trading 212 dividend sources whose payment
  still posts only as ordinary cash, found through the import identity and the
  original staged row's provider kind (reasons `gross_unresolved`,
  `withholding_unresolved`, plus `payer_unassigned` or `payer_unlinked`). Memo
  text, ordinary income rows, other CSV rows and pending review rows are never
  considered. Cash is the exact posting on the import's settlement account in
  its original currency; it is not gross income and absent tax is not zero.
- **Source payer.** For (b), the adapter's own read-only lookup by typed source
  ISIN then ticker may name an existing instrument (`source_identified`). That is
  disclosed beside the instrument's figures as unclassified cash and never added
  to them; it is not a recorded payer. Raw source evidence is returned unchanged.
- **Scope.** `commodity_id` narrows instrument rows only; unassigned totals always
  describe the whole book, so an instrument view discloses excluded scope.
  Items page newest first with a `(date, transaction, currency)` cursor;
  `limit=0` returns the summary only. Totals are computed over all items.

Repair stays optional and reuses existing paths: imported cash goes to the
batch's dividend review (#242 same-payment repair); a payer-less named dividend
goes to its transaction detail, where native replacement may now assign a payer
(omission still keeps a recorded payer, and an assigned payer cannot be removed
by omission). Reports keep their complete-only rules; the R18 statement's
income remains named dividends only; in-period unclassified cash is disclosed
beside it since [#246](https://github.com/sergeyfarin/rekenraam/issues/246).

Validation: `TestDividendCoverageSeparatesKnownUnassignedAndCashOnlyIncome`,
`TestDividendCoverageCursorPagesUnassignedItemsWithoutGapsOrDuplicates`,
`TestIncomeCoverageAPIDisclosesCashOnlyImportAndFollowsRepair`, and the 390px
`income-coverage.spec.ts` journeys (named correction, imported disclosure).
