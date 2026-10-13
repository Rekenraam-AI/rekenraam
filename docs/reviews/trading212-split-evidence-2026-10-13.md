# Trading 212 split evidence recheck — 2026-10-13

[#145](https://github.com/sergeyfarin/rekenraam/issues/145) remains blocked on
provider semantics and representative payloads. No unfinished native split
command is its prerequisite: manual splits, cash in lieu and source-row linking
already exist. This recheck advances evidence collection, not provider mapping.
The [slice 5 contract](../plans/investment-operation-slice-5-contract.md) governs
behavior; #145 owns the delivery checklist and the [roadmap](../roadmap.md)
owns sequencing.

## Official evidence checked

The current [official OpenAPI document](https://docs.trading212.com/_bundle/api.json?download=)
was downloaded without account credentials on 2026-10-13. Its version is `v0`;
the SHA-256 of the downloaded bytes is
`a272f70a713fa9f2f906e9b4be12136c9d5d63d7f9f48562e939aa35c4fc041d`.
The schema URL is mutable; this hash identifies the inspected revision.

- `HistoricalOrder` contains `order` and `fill`.
- `Fill` fields are `filledAt`, `id`, `price`, `quantity`, `tradingMethod`,
  `type` and `walletImpact`; `STOCK_SPLIT` remains a type enum member.
- `FillWalletImpact` contains `currency`, `fxRate`, `netValue`,
  `realisedProfitLoss` and `taxes`.
- Neither `Fill` nor its enclosing `Order` defines a split ratio,
  action entitlement, pre/post-action position or corporate-action identifier.
  `filledAt` is a date-time without a documented split-effective-date rule;
  `quantity` and `price` are numbers without split-specific interpretation.

Trading 212's [Invest/ISA split explanation](https://helpcentre.trading212.com/hc/en-us/articles/360007139378-What-is-stock-split-and-reverse-stock-split-Invest-ISA)
describes forward and reverse splits, including proportional fractional-share
adjustment in both directions. It also says pending orders are cancelled on a
split. That explains customer-facing behavior, not whether an API fill's
quantity is a delta, an entitlement or another amount. Pending-order cancellation
is not an executed-trade reversal and does not unblock #136.

Its [fractional-share article](https://helpcentre.trading212.com/hc/en-us/articles/9511997937437-Can-I-trade-with-fractional-shares)
also describes proportional split adjustment. Neither inspected help article
defines API cash-in-lieu representation or an account-specific rounding rule.
Do not assume every fractional entitlement is sold for cash.

The [practice-account article](https://helpcentre.trading212.com/hc/en-us/articles/360012626297-Are-corporate-actions-performed-on-practice-accounts)
says practice accounts reflect corporate-action price changes rather than
performing the actions. Demo responses therefore cannot establish live split
entitlement semantics. Synthetic test fixtures cannot establish them either.

## Missing evidence and how to collect it

Collect an actual live Invest/ISA history response for each relevant case,
together with the matching corporate-action notice and statement evidence.
Several cases may come from the same event. An absence of a cash-in-lieu row
needs provider explanation of when such a row is or is not produced; a
fabricated example cannot fill the gap.

| Case | Evidence needed beyond a `STOCK_SPLIT` type |
|---|---|
| Forward split | Announced ratio and effective date, eligible quantity before the event, entitlement after it, all related `{order, fill}` items |
| Reverse split | Same evidence, establishing quantity sign/direction and whether multiple rows represent one action |
| Fractional entitlement | Exact before/after quantities, provider precision and rounding, whether the fraction is retained, and any Pie/non-Pie grouping |
| Cash in lieu | Action-linked fraction removed, payment amount/currency/date, related history items and the provider's correlation rule, or documented conditions excluding this case |

Capture the provider JSON **before Rekenraam normalization**. The fetcher in
`backend/internal/onlinesource/trading212/fetcher.go` decodes a selected set of
fields into `OrderFill`; `backend/internal/app/import_trading212.go` then builds
the staged string map. A staged row's `raw_json` is that normalized map, not
the original HTTP response. Unknown provider fields and some wallet fields
are omitted. Consequently a staged export or bundle alone cannot prove that
the provider supplied no additional corporate-action evidence.

For an owner-operated capture, use the existing provider history read
permission and obtain `/api/v0/equity/history/orders` pages covering the event.
Follow the returned `nextPagePath` through the relevant interval; the
[official pagination reference](https://docs.trading212.com/api/section/pagination)
explains the continuation contract. Include related dividend/transaction
history only when the notice or provider response indicates the event is
represented there. A normal incremental refresh can stop before an older
action, so it is not proof the action has no API records. No authenticated
account request was made during this recheck.

Keep original captures private. In a reviewed fixture copy, remove credentials,
authorization headers, account identifiers, personal text and unrelated rows;
replace fill/order/reference IDs consistently while preserving their JSON
types and cross-row relationships. Keep the relevant instrument, event date,
decimal tokens, signs, nulls, missing fields and currencies unchanged. Record
each redaction explicitly. Do not round, shift dates, manufacture fields or
turn a synthetic fixture into a claimed representative capture.

Each evidence set needs a short provenance note:

- Capture date, live environment, account type/entity, endpoint and covered
  interval; no account number or credentials.
- Public action-notice URL or redacted provider notice, published ratio and
  effective-date/time-zone rule, and independently evidenced before/after
  holdings (not a ratio inferred from ledger arithmetic).
- Related row relationships and any statement cash payment; what was removed
  during redaction and which fields remain unexplained.
- Provider documentation or a support response defining those fields for
  this action family. Public schema examples of ordinary trades do not count.

## Questions for provider clarification

These are prepared questions, not a message sent to Trading 212:

1. For a `STOCK_SPLIT` historical fill, what exactly do `quantity`, its sign,
   `price`, `order.side` and `filledAt` mean? Is `filledAt` the financial
   effective time, processing time or something else, and in which time zone?
2. Where are the authoritative ratio, eligible entitlement and action identity
   available? Can one action generate several fills/orders across Pies or
   holdings, and how are they grouped without guessing?
3. For forward/reverse splits with fractions, when are fractions retained,
   rounded or settled in cash? Where are the removed quantity, proceeds,
   currency, date and parent-action link represented in API history?
4. Can these records be revised or reissued under the same/new fill ID, and
   what identifies a replacement versus a duplicate? Can the provider supply
   redacted live examples for the four cases above?

Until these questions have sourced answers, admission remains review-only.
Any later proposal must explicitly use verified ratio/date/entitlement
evidence, retain manual linking and commit source identity with the split
atomically under the existing gain/checkpoint guards. Unsupported or
ambiguous cases stay in review; quantity/price arithmetic is not a substitute.

## Existing safeguards validated

At repository commit `2d8ff86a`, the following existing cases passed with the
native SQLite wrapper and race detector:

- `TestFetchOrdersPreservesUnsupportedFillType`
- `TestFetchAdapterPreservesFillReviewEvidence`
- `TestImportUnsupportedFillCannotPost` (including `STOCK_SPLIT` and missing type)
- `TestUnsupportedSaleFillCannotUseCashFallback`
- `TestUnsupportedFillCannotCorrectAcceptedBuyOrSale`
- `TestTrading212SplitRowLinksToRecordedSplitWithoutDoublePosting`
- `TestTrading212SplitLinkRefusesOtherSecurityAndNonSplitRows`
- `TestTrading212SplitLinkRefusesBatchDiscardedAfterPreparation`
- `TestTrading212SplitRowLinksJournalFreeOperationAndKeepsDedupe`

These prove the current refusal and linking boundary, not verified provider
mapping. No real split payload or provider field clarification was available
for this recheck, so #145 remains open and blocked. No product behavior or
acceptance was relaxed.
