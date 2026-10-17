# Competitor comparison

Reviewed 2026-10-17 against the vendors' own documentation and the current
[Rekenraam feature ledger](implemented.md). This is a decision aid, not a claim
that an unlisted feature is absent. It compares workflows relevant to a
single-user, self-hosted, multi-currency finance app. Prices, popularity,
security quality, and features behind particular paid tiers were not assessed.
The earlier broad survey remains in
[the July research record](reviews/competitive-analysis-2026-07.md).

Local status was reconciled at `f35cc723`; see the
[app-state review and proposed audits](reviews/app-state-and-audit-plan-2026-10-17.md).
Official pages for the products below were rechecked on 2026-10-17. Firefly III
was available through indexed official documentation; direct retrieval failed,
so current release/tier details are unverified. Provider signup, feed availability
and licensing are separate checks. Documentation supports a workflow comparison,
not measured usability, equivalent financial semantics or market demand.

## Verified comparison

| Product | Documented strengths relevant here | Boundary or implication for Rekenraam |
|---|---|---|
| **Rekenraam** | Exact multi-commodity ledger, reconciliation, CSV/QIF export and import, budgets, recurring drafts, per-currency forecasts, investment lots/gains and Trading 212 import. | Native correction, transfers, structural actions, historical entry, unknown-basis resolution/recovery, named shorts, price management and crypto quotes ship. R18 adds dated gains, period statements, alternative/person-pooled methods, operational FX and immutable operational runs. Live-read consistency, capital-return disclosure and unclassified imported dividend cash disclosure ship (#237/#238/#246); reviewed Twelve Data dividend suggestions and named short payment in lieu also ship; tax rules and TWR/IRR remain outside R18. See [implemented](implemented.md) and [roadmap](roadmap.md). |
| **GnuCash** | Its [investment guide](https://www.gnucash.org/docs/v5/C/gnucash-guide/chapter_invest.html) documents lots, dividends, return of capital, splits and mergers, and a price database. | Sets the accounting and transaction-type bar for R16. The guide is for a desktop application; this comparison does not equate its lot methods or tax results with Rekenraam's. |
| **Quicken Classic** | Its [investment action list](https://info.quicken.com/win/tell-me-about-the-investment-transaction-list-s-ac) names transfers, return of capital, splits, short sales and covers; its [placeholder guide](https://www.quicken.com/support/resolving-placeholders-and-usd0-00-cost-basis-in-quicken-for-mac/) explains missing-basis recovery. | A migration target needs basis-preserving transfers, correction and explicit unknown-basis handling before it can promise comparable investment history. Quicken's [short-cover help](https://info.quicken.com/win/how-do-i-cover-a-short-sale) reinforces naming shorts separately from ordinary sells. |
| **Portfolio Performance** | Its [security menu](https://help.portfolio-performance.info/en/reference/view/securities/context-menu/) documents security transfers and a split wizard; its [performance manual](https://help.portfolio-performance.info/en/reference/view/reports/performance/dashboard/) documents TTWROR and IRR. | R16's split/transfer and historical-entry/recovery workflows now ship; R18 adds dated basis/gains and saved operational documents, while R13 returns remain open. Its default split path retroactively adjusts earlier transactions and quotes; Rekenraam's immutable-source design calls for a dated operation and separate adjusted views. |
| **Ghostfolio** | The project's [README](https://github.com/ghostfolio/ghostfolio/blob/main/README.md) documents self-hosting, activity import/export, multi-account holdings, portfolio charts, ROAI periods and a mobile-first PWA. | Portfolio analytics and mobile presentation are relevant R13 benchmarks. Its documented activity/API scope does not establish equivalent ledger or basis semantics, so no absence claim is made here. |
| **Actual Budget** | Its [rules](https://actualbudget.org/docs/budgeting/rules/) can transform imports, its [schedules](https://actualbudget.org/docs/schedules/) support automatic or reviewed entry, and its [import guide](https://actualbudget.org/docs/transactions/importing/) lists CSV/QIF/OFX/QFX/CAMT. Its [multi-currency guide](https://actualbudget.org/docs/budgeting/multi-currency/) says native support is still absent and describes an experimental workaround. | Rekenraam's exact multi-currency model is a meaningful difference. Actual's broader import formats and rules are a later usability benchmark; its documented schedules reinforce a clear review-before-post workflow. |
| **Firefly III** | Its [rules](https://docs.firefly-iii.org/how-to/firefly-iii/features/rules/), [data importer](https://docs.firefly-iii.org/how-to/data-importer/import/csv/) and [budgets](https://docs.firefly-iii.org/how-to/firefly-iii/finances/budgets/) cover mature routine transaction management. | Keep R6 import depth and rules on the later list. This review did not verify comparable security-lot accounting, so it makes no claim about its presence or absence. |
| **PocketSmith** | Its [multi-currency guide](https://pocketsmith.helpkit.so/multi-currency/6a6X8SseDBfyRznV6vJTS3/multi-currency-an-overview/6a6X8SseDAA5o85oTGHWtJ) and [investment-account guide](https://learn.pocketsmith.com/net-worth-assets--debts/6a6X8SseDAohQfjgQVP5mj/managing-investment-accounts/6a6X8SseDCNBiAKmvJS9uG) document cross-currency planning and alternative ways to represent investments. | Rekenraam's R10 forecast is already a differentiated, exact per-currency base. No lot-level parity or pricing claim is inferred from these guides. |
| **Monarch Money** | Its [manual holdings guide](https://help.monarch.com/hc/en-us/articles/10032888165140-Manual-Investment-Holdings) documents priced holdings and separates quantity tracking from manually entered balances. Its [CSV guide](https://help.monarch.com/hc/en-us/articles/4409682789908-Importing-Transactions-Manually) now documents overlap choices and transaction-ID matching. Its [manual transaction guide](https://help.monarch.com/hc/en-us/articles/360058441811-Creating-Manual-Transactions) says it lacks reconciliation. | Shows that a polished general finance app can have a different investment and trust boundary. Rekenraam should emphasize auditable import, reconciliation and investment history in migration examples without asserting feature-wide superiority. |

## Priority decisions from this review

1. **R16's agreed delivery bar is complete (2026-10-13).** #146 adds
   journal-free sourced splits and entitlement replay to the shipped long,
   short and compound-action workflows. This gains zero-delta split recording;
   inspecting/correcting/settling initially journal-free splits by operation
   identity also ships in #196, including mobile history and import dedupe. The competing
   investment workflows treat corrections, transfers, basis actions and splits
   as ordinary operations. Rekenraam's bounded #114 and #166–#168 gates, named
   shorts and compound actions are delivered; provider/tax-policy completeness
   remains separate.
2. **R11 and R17 now supply pricing operations and instrument entry (2026-10-14).**
   Users can inspect/correct observations and coverage, create crypto holdings
   with lots, import their trades through CSV, and opt into a shared quote
   registry with durable refresh and visible failures. This gains priced crypto
   entry parity; exchange/DeFi/staking/NFT coverage remains deliberately absent.
   Provider keys, EU market/display entitlements and unofficial Yahoo availability
   remain operator boundaries, not product coverage promises. See the
   [provider evidence and contract](plans/quote-providers-crypto-plan.md).
3. **R18's six gains-projection slices are delivered (2026-10-15).** Named
   quote/staleness rules, dated positions and period results, method comparisons,
   operational FX and immutable operational runs gain explicit valuation and
   basis inspection. Saved alternative/FX runs, broker-basis comparison and
   jurisdiction tax rules remain outside that delivery. #237/#238 are delivered;
   R13's TTWROR/IRR and allocation views remain the deliberate gap against the existing Portfolio Performance/Ghostfolio benchmark. Specify
   equations and external-flow classification before claiming returns parity.
4. **Treat locale input and owner-local dates as trust work.** Locale amount
   parsing (#100) and owner-local date defaults (#101) are delivered. Native
   terminology review (#102) remains before calling the five drafted
   translations complete or using them in a migration demo.
5. **Keep R6 and broader feeds later.** Actual and Firefly III document
   richer import automation, but Rekenraam already has usable CSV/QIF entry.
   No official source in this review justifies moving broader integrations
   ahead of validating the delivered core and specifying R13 returns.

These priorities are reflected in the [roadmap](roadmap.md). Recheck source
pages before making market-wide uniqueness or release-parity claims.

## Consequences of the October recheck

R13 remains a useful gap, with different metrics requiring different contracts:
Portfolio Performance documents [TTWROR](https://help.portfolio-performance.info/en/concepts/performance/time-weighted/),
while Ghostfolio's linked README calls its period measure ROAI. These are not
interchangeable labels for the same output. Adopt a named formula and test
external flows before comparing percentages. Allocation targets, denominators
and missing-value coverage also need a contract; a gains table is not an
allocation or return report.

The July survey and September strategy review remain dated research. Their
claims of category leadership, uniqueness, demand, prices or willingness to
pay are not established by this workflow recheck. The September recommendation
to validate migration and daily use before broadening scope remains a proposal
worth testing through the household dry run; it does not authorize AI/MCP,
desktop distribution, tax compliance or a hosted product.

Actual's richer imports/rules, Firefly III's routine management and PocketSmith's
cross-currency planning remain reasons to test the whole daily workflow, even
though the recent delivery effort concentrated on investments. The roadmap's
audit coverage includes those areas. Market-wide parity is not a release gate.
