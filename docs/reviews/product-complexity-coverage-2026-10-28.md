# Product complexity: coverage and selection — 2026-10-28

First review output for [#390](https://github.com/sergeyfarin/rekenraam/issues/390).
Source: `83f1e38b01cb66c65ce643e5edd5c4a7a2fe9164`, clean working tree on
`main` before this audit. This is a capability map and bounded selection,
not a claim that every product rule has been justified. It changes no runtime,
schema, API or accepted product policy. The
[roadmap](../roadmap.md#current-focus) retains work order.

## What remains after the delivered inputs

The live issue and its three comments were read on 2026-10-28. #391's
relationship inventory and #388/#389/#392/#393/#394/#395 are delivered inputs.
In particular, #395's cash-payment and reinvestment field corrections now ship;
they are not outstanding implementation work for this audit.

#390 still needs its wider capability map, necessity evidence for the selected
workflows, then a final disposition with zero to five actionable proposals.
This document supplies the first output. The second output must trace actual
consumers and test counterfactual outcomes; it must not treat tests that enforce
an existing restriction as proof of its necessity. Final proposal selection and
issue acceptance remain separate from this map. No provider, owner data or
release decision is needed for these review outputs.

## Inputs and coverage language

Reuse the [feature ledger](../implemented.md), the
[twelve-area capability/plan register](app-state-and-audit-plan-2026-10-17.md#proposed-audit-coverage),
[route/state inventory](frontend-audit-2026-10-18.md),
[sub-view inventory](subview-audit-2026-10-19.md#sub-view--check-matrix),
[#284's 34 rule groups](maintainability-invariants-2026-10-21.md),
[final mechanical dispositions](maintainability-audit-2026-10-24.md), and
[#391's relationship inventory](financial-dependency-inventory-2026-10-23.md).
The current route files were checked against those dated inventories; notably
`/app/investments/returns` now exists. CLI, worker, API-only and operational
capabilities belong in the map even without a route.

- **Reviewed input:** an earlier bounded necessity comparison is reusable;
  its original SHA/limits remain authoritative for that experiment. Selected
  current consumers still need reconciliation with this source.
- **Sampled:** current entry points/contracts and selected existing evidence
  were inspected for selection. This does not establish necessity across the
  whole family or rerun its historical acceptance.
- **Deferred:** outside the six-workflow deep pass. The reason and any reused
  safeguard evidence are named; deferred does not mean unnecessary or blocked.

The statuses below describe this map, before the second review output. A
workflow selected for depth is not already fully reviewed by being listed.

## Capability map

UI paths are browser routes; code paths are repository-relative. Evidence is
historical unless explicitly rerun in the second output.

| Capability family / entry points | Governing contract | Existing evidence and selection status |
|---|---|---|
| Ordinary ledger, correction and cleanup: `/app/transactions`, account registers, category detail, Settings → Trash; transaction lifecycle API | [Ledger plan](../plans/transaction-ledger-core-plan.md), conventions' lifecycle/audit rules | **Sampled → selected.** [Ledger journey and #296/#297](ledger-reconciliation-audit-2026-10-17.md) expose stale browser edits and stale reconciliation claims. Select an ordinary posted edit/metadata change/void/restore comparison, including immutable history and expected versions. Split/date/transfer representation is context, not another deep workflow. |
| Reconciliation and ordering: `/app/reconcile`, register reorder, lifecycle impact dialogs | [Ledger plan](../plans/transaction-ledger-core-plan.md), [reconciliation conventions](../conventions.md) | **Sampled → selected.** #240/#296/#304/#305 and [reconciliation preview review](reconciliation-preview-review-2026-10-02.md). Select actual versus neutral impact, same-day boundaries and review binding; do not infer necessity from a confirmation dialog's existence. |
| Cash income, expenses, tax and payment in lieu: ordinary editor, investment cash forms, `/app/investments/income` | [ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md), [dependency plan](../plans/financial-dependency-simplification-plan.md), [income coverage](../plans/dividend-income-coverage-plan.md) | **Reviewed input → selected current reconciliation.** #391's [four-case comparison](financial-dependency-comparison-2026-10-23.md), #393 optional payer/tax shape and #395 fields. Compare exact gross/tax/cash with repairable explanation and immutable source identity. Ordinary fees/interest and loan-payment composition reuse #391; no loan subsystem review. |
| Investments and corporate actions: `/app/investments`, instrument activity, lot/correction dialogs | ADRs [0012](../adrs/0012-investment-ledger-subledger-and-reporting-boundaries.md), [0013](../adrs/0013-investment-operations-and-position-sides.md), 0019; [operation plan](../plans/investment-operation-refactor-plan.md) | **Reviewed input → selected fractional-disposal correction only.** #392/#394 all-four-parent replay and historical entry; #395 reinvestment is delivered context. Real quantity/basis/elections versus parent standing is the selected comparison. Shorts, basis resolution, transfers, return of capital and other actions reuse #391 as context; no fresh full-family deep pass. |
| File/online import and provider evidence: `/app/import`, `/app/import/dividends`, investment provider-event completion; fetch workers | [Import plan](../plans/import-plan.md), [dividend repair](../plans/import-dividend-review-plan.md), ADRs [0010](../adrs/0010-durable-background-work-and-fx-coverage.md), [0017](../adrs/0017-provider-independent-features-and-incomplete-data.md), 0019 | **Sampled → selected.** [Import audit](import-audit-2026-10-17.md), #301 resumable batches, #242 same-payment repair and #393 ancestry identity. Select repair/retry with immutable evidence and explicit missing economics. Provider wire verification, other adapters and parser breadth remain deferred. |
| Prices, reports, saved documents and returns: `/app/pricing`, `/app/reports`, `/app/investments/returns`, saved gains/statement views | [Pricing](../plans/pricing-management-plan.md), [reports](../plans/reports-plan.md), [investment reporting](../plans/investment-reporting-plan.md), [portfolio returns](../plans/investment-returns-plan.md), ADR 0012 | **Sampled consumers; deeper policy review deferred** to keep the cap. #391 already justifies derivation lineage and immutable saved bytes. [Reports audit](reports-r18-audit-2026-10-18.md) and feature-ledger R13 delivery cover more recent readers. Check classification/coverage only where the six workflows affect it; no new pricing, returns or saved-document algorithm audit. |
| Recurring, budgets, forecast and learned spending: `/app/recurring`, `/app/budgets`, `/app/forecast`, scheduler/generator | [Recurring](../plans/recurring-transactions-plan.md), [budgets](../plans/budgets-plan.md), [forecast](../plans/projected-balances-plan.md), [learning](../plans/forecast-learning-plan.md), ADR 0010 | **Sampled → selected one composed planning workflow.** [Planning audit](planning-audit-2026-10-18.md), occurrence uniqueness, discard/retry and forecast snapshot/basis controls. Follow template → draft → post/discard → forecast explanation, with budget actuals as a consumer. Model fitting, quality gates and every cadence remain deferred. |
| Accounts, reference data, setup and settings: `/`, `/app/accounts`, categories, Settings → currencies/language/appearance; institutions/payees/tags APIs | [Account design](../design/accounts-system-design.md), [categories](../design/categories-design.md), conventions' dated structure/localization rules | **Sampled; deeper restriction review deferred.** [Reference-data audit](reference-data-audit-2026-10-17.md), #385 owner-local current reads and #417 category/holding deletion parity. Dated eligibility and book identity remain prerequisites of selected commands, not independently selected rules. No generic account-unlock or localization-completeness proposal. |
| Identity, privacy and security: setup/sign-in, Settings → Security, PAT APIs, `recover-owner` | ADRs [0002](../adrs/0002-http-security-policy.md), [0003](../adrs/0003-password-hashing-and-first-run-setup.md), [0018](../adrs/0018-personal-access-token-boundary.md); [deployment security](../deployment-security.md) | **Sampled threat reasoning; deep pass deferred.** [Identity concurrency](identity-concurrency-audit-2026-10-17.md), [expiry audit](authentication-expiry-audit-2026-10-20.md), [PAT audit](personal-access-token-audit-2026-10-20.md). Throttling bounds verified guesses; CSRF binds browser mutation origin; final expiry/revocation checks protect queued writes; read-only PAT grants constrain a stolen token. Unchanged balances would not justify removing any of these. No fresh penetration/deployment assessment. |
| Export, backup, restore and storage durability: Settings → Data, ledger CSV/QIF/bundle APIs, backup worker, operator `verify-backup`/`restore` | [Portability plan](../plans/data-portability-plan.md), ADRs [0004](../adrs/0004-sqlite-connection-migrations-and-backup.md), [0011](../adrs/0011-ledger-export-contract.md), [0015](../adrs/0015-sqlite-commit-durability.md) | **Sampled recovery reasoning; deep pass deferred.** [Recovery audit](recovery-audit-2026-10-18.md), [restore process lock](restore-process-lock-2026-10-14.md), #420 verify exit status. A valid current ledger does not prove a backup is recoverable; verification and a held process lock prevent replacement with damaged data or a live-writer race. #336 owner/off-device/key-custody evidence remains external. |
| Shared UI/accessibility/localization and Home/navigation | [Conventions](../conventions.md), [approved Home design](../design/home-and-navigation.md) | **Sampled friction surfaces; new design implementation deferred.** Route/sub-view inventories supply dialogs/steps for selected workflows. Native terminology sign-off #102 and approved Home implementation are separate; no fresh browser accessibility sweep in this output. |
| Build, deployment, release and engineering machinery | Architecture, ADRs 0014–0016 and [0020](../adrs/0020-release-signing-and-platform-targets.md), developer workflow | **Deferred necessity review**, with #284/#405 reused. [Schema review's retained controls](schema-baseline-review-2026-10-26.md#what-the-critical-review-retained) justify typed facts, versions, guards and keys. The separate static website has no financial persistence. Candidate signing, hosted workflow evidence and owner release gates are not cleared by a source audit. |

## Six workflows, twelve rules

These are selection numbers within this dated report, not new work-item IDs.
Each pair will be traced UI/API → service → transactional authority/storage →
diagnostics, reports and export in the second output. Baseline numeric,
same-book, dated-account and security rules still apply; their presence does
not turn each into another independent deep-pass rule.

| Selected end-to-end workflow | Two named rules | User outcome and counterfactual to examine |
|---|---|---|
| Ordinary posted correction/lifecycle | **1. Durable posted history. 2. Expected-version binding.** | Correct a mistaken expense or metadata without a reversal maze; distinguish retaining history from silently overwriting another tab's amount. |
| Cash-only classification and attribution repair | **3. Cash attribution is advisory. 4. Once-per-payment source identity follows corrections.** | Record known cash/gross/tax without manufacturing holdings; clearing payer must not erase tax or make a source event reusable. |
| Fractional-share disposal / parent correction | **5. Actual disposal quantity/basis/elections. 6. Historical parent is provenance, not continuing authorization.** | Correct or reverse a parent while a real disposal remains satisfiable; insufficient units and invalid specific-lot choices still refuse. |
| Import repair and partial-batch retry | **7. Unresolved required economics cannot fall back to ordinary cash. 8. Repair/retry retains source evidence and settled rows.** | Finish valid work and repair remaining rows locally without re-fetch/re-upload, duplicated cash or guessed tax. |
| Reconciliation, neutral correction and reorder | **9. Guard actual checkpoint impact. 10. Bind override to reviewed boundaries.** | Avoid confirmation for neutral metadata/attribution; refuse an override that would invalidate an unseen statement. Equal day-end totals cannot hide a same-day crossing. |
| Recurring generation/review and forecast explanation | **11. One durable occurrence per template/date, including discarded work. 12. Forecast details share one snapshot/basis.** | Generate/post/discard once and explain one forecast; avoid regenerating dismissed drafts or mixing old headline amounts with new detail. |

## Reconcile #284's 34 groups rather than start another inventory

All original group IDs below refer to the dated #284 table. They are navigation
aliases, not current dispositions. Selected rules above intentionally combine
or split those implementation-oriented groups according to user outcomes.

| Original groups (all 34 accounted for) | Treatment in this bounded product review |
|---|---|
| M1–M5 | Exactness, range, per-entry balance, book identity and date/knowledge boundaries are reused prerequisites/controls. No new necessity experiment for all arithmetic limits. |
| L1 | Dated account eligibility is sampled context; account structure/archival policy is deferred. |
| L2, L4 | Ordinary history and stale-version workflow selected (rules 1–2); L4's account dependency races remain context. |
| L3, L8 | Producer admission, import repair/source identity and recurring identity selected (rules 4, 7–8, 11). Draft/report separation is an affected consumer. |
| L5 | Generic investment-mutation refusal is a retained context boundary: the ordinary editor cannot maintain lots. The family-specific investment workflow is not redundant with ordinary editing. |
| L6–L7 | Actual checkpoint impacts and reviewed membership selected (rules 9–10). Different same-day/multi-journal algorithms are not assumed equivalent. |
| I1–I4, I6, I8–I9 | Typed operations, side/knowledge/method, lineage, atomic writer and gain acknowledgement reuse #391 and current delivery controls. No fresh every-family review. |
| I5, I7 | Real fraction conservation versus parent standing selected (rules 5–6); #401 now shares corporate-action correction preparation. |
| I10 | Historical permanent-entitlement retention is superseded by #388/#389 and ADR 0019 (rule 3). The global write-time payment replay is gone. Do not optimize or restore it. |
| I11–I12 | Report/preview consumers sampled where affected, including forecast basis (rule 12). #399 already shares browser review plumbing; no second extraction proposal. |
| S1–S9 | Threat/recovery reasons sampled in the capability map; deep security/durability review deferred. Their dated controls are reused, not counted as a seventh workflow or proved necessary by financial totals. |

## Stop boundary and validation

No new proposal or implementation issue is created by this selection. The next
output supplies twelve necessity traces, independent arithmetic/counterexamples,
matched valid controls and explicit test/probe provenance. The final output
will choose dispositions and zero to five proposals, reusing existing owners
where applicable; none of these outputs reopens delivered #391 work or creates
a release gate. Unselected workflows remain outside the audit claim.

Validation for this documentation slice: current route/feature/owner references,
all-family coverage, exactly six workflows/twelve rules, all 34 #284 groups,
local Markdown targets/anchors and `git diff --check`. No runtime tests or
browser journeys were rerun for this map.
