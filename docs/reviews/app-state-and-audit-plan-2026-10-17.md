# Application state, documentation reconciliation and audit brief — 2026-10-17

**Status:** documentation reconciliation completed; twelve application audits
proposed, execution pending. This review is a dated evidence/disposition record,
not a live delivery checklist or another roadmap. The
[roadmap](../roadmap.md#current-focus) owns order. After the owner's explicit
publication approval on 2026-10-17, the audit was published as
[#260](https://github.com/sergeyfarin/rekenraam/issues/260). That issue owns
execution checklists and findings under
[Work Tracking](../conventions.md#work-tracking); audit execution is pending.

## Evidence and limits

Reviewed the working tree at `f35cc723` on 2026-10-17. It was clean at the
start. Sources: requirements, conventions, architecture/ADR status, developer
workflow, roadmap, implemented ledger, all 27 feature-plan headers and their
relevant status/scope sections, all four design references, existing reviews,
route inventory, selected application/API/repository consumers, browser specs,
migrations and validation workflows. The register below assigns every plan
and design reference to an audit area. Historical starting-point sections are
not treated as current implementation claims.

Live GitHub issue searches used:

- `repo:sergeyfarin/rekenraam is:issue is:open`, ascending creation order,
  limit 100: **8 results** before any audit publication.
- `repo:sergeyfarin/rekenraam is:issue closed:>=2026-10-10`, descending updated
  order, limit 100: **98 results**. This uses GitHub's date qualifier through
  the time of the review, not a claim that every closed issue shipped a feature.
  Retired/duplicate reports, including overlapping browser-timeout issues,
  must retain their actual closure reasons.

The authenticated GitHub connector supplied these results; local `gh` network
access failed. Selected closure groups were cross-checked against local commits
and concrete consumers below. This review did **not** rerun financial tests,
perform a browser acceptance session, inspect an owner's data, certify all
closed tickets, measure current performance or verify remote repository security
settings. Those are explicit audit work, not implied by document cleanup.

Competitor sources were rechecked separately in the
[maintained comparison](../competitor-comparison.md). Primary documentation is
workflow evidence, not proof of product quality, adoption or financial parity.

## What changed in the baseline

| Area | Delivered evidence sampled | Retained boundary |
|---|---|---|
| Native SQLite and recovery | #195/#164 and ADRs 0014–0016; `backend/internal/db/sqlite.go`; #105/#227 subprocess/restore evidence | Native engine race instrumentation differs from Go race coverage; recorded speedups are historical measurements, not current latency guarantees |
| Daily ledger and reconciliation | #185–#187, #219–#225, #240/#248; shared workspace, posting-date helpers, lifecycle impact and reconciliation specs | Combined edit/order/posting-date/recovery journeys still need fresh baseline acceptance |
| Investment lifecycle | #115/#167/#168/#169/#170/#197 and #255–#258; transfer/corporate-action/replay consumers and phone specs | New payment-in-lieu interactions need the same independent sequence audit; provider-only admission stays separate |
| Pricing and crypto | #116/#117/#203; price-management UI, quote registry and observation-selection contract | Keys, entitlements, unsupported provider semantics and partial coverage remain explicit |
| R18 and dividend attribution | #118/#230–#235/#237/#238/#241/#242/#244/#246; live consistency and saved schema-3 statements | Gains are limited-scope figures; TWR/IRR and general snapshots are not shipped |
| Provider suggestions | #111/#254/#255/#258; Twelve Data producer, immutable revision evidence and payment linking | Dividends only; no auto-post or scheduler; #256 split mapping is evidence-gated |
| Test/development support | #106–#110, #252 and browser isolation work; shared report scenario, exact FX display, large-book and post-merge scripts | A shared fixture or passing historical test does not establish every report classification, UI state or full-book performance |

Concrete anchors sampled include
[the shared report API consumer](../../backend/internal/api/report_scenario_test.go),
[dated internal transfer cases](../../backend/internal/app/investment_transfer_internal_backdated_test.go),
[share-exchange reads](../../backend/internal/app/investment_share_exchange_reads_test.go),
[spin-off API cases](../../backend/internal/api/investment_spin_off_test.go),
[cash-and-stock API cases](../../backend/internal/api/investment_cash_stock_merger_test.go),
[payment-in-lieu service cases](../../backend/internal/app/investment_payment_in_lieu_test.go),
[the bundle version](../../backend/internal/app/exports_bundle.go), and
[large-book measurement](../../scripts/measure-large-book.sh).
Presence and recorded acceptance were inspected; these tests were not run here.

## Documentation findings and dispositions

| Stale or misleading information | Disposition in this reconciliation |
|---|---|
| Roadmap current focus contains a long delivery diary; R18 text still treats #238 as outstanding | Remove shipped focus detail, keep historical references, retain R13 as next feature and add bounded audit work |
| Requirements call accepted R10 core/learning unfinished | Record the 2026-09-09 acceptance; preserve the actual forecasting/learning contracts |
| Connections header says entirely unimplemented despite #111/#254/#255/#258 | Distinguish delivered dividend suggestions from proposed bank/broker adapters and evidence-gated splits |
| Receipt proposal names R9 as current, implies its storage proposal resolves accepted scope and treats CGO itself as disallowed | Restore proposal status, roadmap ownership and explicit scope approval; retain the no-extra-server-OCR proposal without contradicting native SQLite |
| Import plan says drafts have no producer/UI; coverage plan still gates shipped R9 generation | Record recurring draft delivery without changing file-import posted/review semantics |
| Investment ledger marks delivered transfers/corporate actions and correction UI as backend-only; provider absence downgrades manual delivery | Correct bounded capability markers and remove provider-dependent completion claims under ADR 0017 |
| Ledger still says short/correction commands are future work; bundle header says schema 13; a dated catalog count is called current | Reconcile shipped readers/writers, schema 14 and dynamic catalog parity; keep genuine API-only and partial capabilities marked |
| Competitor table retains open #237/#238 and older Monarch import/holding claims | Record delivered fixes and update primary-source workflow descriptions; keep claims narrow |
| Roadmap release text treats a September review as current certification, says there are only two CI workflows and conflates delivered security mechanisms with deployment acceptance | Preserve dated evidence, include weekly large-book validation and require current release/deployment checks |
| Generic snapshots/settings export decisions imply R18 saved documents and full database protection do not exist | Separate delivered immutable operational reports/SQLite backup from deferred general snapshots and portable settings interchange |

Accepted ADRs and historical review bodies are unchanged. No financial behavior,
formula, schema or scope lock changes in this documentation commit. R14 storage,
AI/MCP, desktop hosting and broader integrations remain proposals or deferred
scope; this review does not accept them. The old forecast execution tables and
copyable historical handoffs must be read through their completed status banners
and the current roadmap; they do not reopen delivered slices.

## Proposed audit coverage

These are bounded audit specifications, not completed results or an instruction
to rebuild each subsystem. Area names below are descriptive, not new issue IDs.
The future GitHub audit issue owns execution checklists. The roadmap owns order.

| Area | Scope and representative acceptance scenario | Evidence needed to close its audit |
|---|---|---|
| Setup, identity and privacy | Empty installation → owner/book/preferences → MFA → login/logout/session expiry → operator recovery; timezone changes; session/CSRF/proxy trust, sealed keys, financial logging and outbound opt-in | Real API/CLI paths, authorized/unauthorized cases, recovery with retained/missing key, deployment configuration inspection; distinguish API-only security management from screens |
| Reference data and accounts | Institutions, commodity precision/identity, accounts and hierarchy, category/payee/tag CRUD; effective dates, archive/restore, protected system roles and budget treatment | Current/historical reads after version changes; cycle/role/date refusals; one management journey and named limitations, including institution UI and old unlinked payees |
| Ledger and reconciliation | Expense/refund/split/opening balance; same- and cross-currency transfers on separate posting days; global/account order; approve, edit, void/delete/restore; invalidate/void checkpoint then reconcile again | Exact independent balances on entry and transaction dates; actual preview/write agreement, stale-review refusal, unchanged metadata-only balances, checkpoint history and phone journey |
| Import and cleanup | QIF/CSV encodings and decimal/date decisions, profiles/rules, investment CSV, Trading 212 paging/revisions, upload limits, partial commit and dividend repair; repeat import after correction | Real stage → review → commit consumer, >200 rows, duplicate/retry identities, raw evidence retained, no cash fallback for invalid investment rows; explicitly exclude unbuilt XLSX/OFX/rollback |
| Investment operations and replay | Capability matrix for every admitted operation: both position sides, purchases/disposals/reinvestment/write-off, known/unknown basis, all transfer families, splits/exchange/spin-off/merger, capital return/cash in lieu, payment in lieu; corrections and backdating | Independent quantity/basis/cash/gain expectations after each command; conservation plus classification; dependent-operation refusal, same-day order, gain/checkpoint acknowledgement, late rollback and export/self-check |
| Prices, providers and durable work | Manual/derived/provider price corrections, raw/adjusted selection, FX dates/age, quote identity/entitlements; leases/retries/restarts; dividend suggestions, provider revisions, completion/linking | Provider-free stubs for core acceptance, coherent observations and explicit partial states; competing claims, retry idempotence, no auto-post, preserved owner decisions; live samples only for admitted adapter semantics |
| Reports, R18 and R13 readiness | Net worth/spending/cashflow filters, descendants, gross/refund/net classification and posting dates; dated/alternative/person-pooled gains, FX, capital returns, unattributed dividends and saved documents | Independent expected figures through API and browser export; immutable bytes/schema-1/2/3 behavior; correction/withdrawal interleavings; R13 input/flow inventory and unresolved specification choices |
| Planning | Monthly budgets and effective on-budget treatment; recurring generation/review/post/discard/skip/retry; overdue and DST behavior; deterministic forecasts and optional learned spending | Actuals never include assumptions; generated identity prevents double counts; model-off remains unchanged; partial bulk failure recovery, stale event basis, quality fallback and annual-history exclusions |
| Portability, migrations and disaster recovery | Core CSV/QIF/bundle schemas/limits; forward and seeded migrations, self-check corruption detection, nightly/manual backup/retention, verify/restore, key custody and server/restore lock | Disposable restore drill comparing independent ledger/lot/report results; unsupported QIF coverage, complete manifest, immutable saved/revision evidence; interrupted/full-disk/cancel paths; no mutation from previews/checks except documented run records |
| Frontend, accessibility and localization | Every route family and shared dialog: loading/empty/error/success, keyboard/focus, 390px, zoom, light/dark and semantic token variants, URL/history state, exact locale entry, owner-local dates and fallback text | Actual rendered state evidence, including closed lots (#259); six-locale numeric/date journeys and catalog parity; #102 native sign-off remains distinct and cannot be inferred from tests |
| Deployment and release operations | Single embedded app and Compose, fresh install, persistent volumes/permissions, native dependencies, upgrade/rollback, HTTPS/MFA, dependency scans, secret history/remote scanning, release packaging/signing and adoption assets | Clean build/deploy/restart/upgrade evidence and operator checklist; verify remote settings separately; explicit missing signing/PAT/demo/household-dry-run status, without treating repository publication as launch |
| Architecture, API and quality/performance | Service/repository boundaries, composed reads/no item fan-out, exact types and ranges, OpenAPI/client/Bruno alignment, clean generated output; meaningful oracles, race isolation, deterministic browser stubs and CI equivalence | Focused tests first and shared gates once at the candidate SHA; representative book timing/heap via supported wrappers; API validation/error consistency and test-independent state; no freshness claim for old coverage percentages |

The page inventory includes setup/root and app landing, accounts/registers,
categories/detail, transactions, reconciliation, import/dividend repair,
investments/instrument detail/income coverage, pricing, reports, budgets,
recurring, forecast and settings (appearance, currencies, language, security,
data and trash). Each belongs to a row above. API-only features, CLI recovery,
workers, migrations, build/distribution and deliberate non-features are included;
screen coverage alone would miss them.

## Feature-plan and design register

Each path below was included in the status/scope review. “Contract” records
bounded delivered behavior, not a claim that every historical paragraph is a
current code specification. Follow accepted ADRs and the code for later changes.

| Reference under `docs/` | Disposition and audit coverage |
|---|---|
| `plans/backend-test-coverage-plan.md` | Mostly historical completed coverage work; future harnesses belong with consumers; architecture/quality audit checks remaining relevance |
| `plans/budgets-plan.md` | Shipped bounded contract; planning and account-treatment audits |
| `plans/connections-plan.md` | Dividend producer delivered; bank/broker adapters proposed; providers/import/deployment audits; July availability figures require fresh verification |
| `plans/csv-investment-income-plan.md` | Shipped #241 contract; import/investment/report audits |
| `plans/data-portability-plan.md` | Accepted R3 contract plus historical starting point; recovery and export audits use current schemas and ADRs |
| `plans/dividend-income-coverage-plan.md` | Shipped #244 contract; investment/report/UI disclosure audit |
| `plans/forecast-learning-plan.md` | Accepted M1–M4 contract; planning/quality/resource audit |
| `plans/import-connection-accounts-plan.md` | Delivered mapping contract with declared UI limits and subsequent dividend repair; account/import audits |
| `plans/import-dividend-review-plan.md` | Shipped #242 contract; same-payment repair and identity audit |
| `plans/import-plan.md` | R4/R5 shipped, import depth deferred; import/cleanup audit |
| `plans/investment-integrity-plan.md` | R12a accepted; operation and reporting contracts supersede early limits; lifecycle audit |
| `plans/investment-operation-refactor-plan.md` | Delivered R16 contract with retained admission fences; lifecycle/provider audits |
| `plans/investment-operation-slice-1-contract.md` | Delivered operation-foundation contract; lifecycle/schema audit |
| `plans/investment-operation-slice-5-contract.md` | Delivered transfer/basis-action contract and later refinements; lifecycle/recovery audit |
| `plans/investment-reporting-plan.md` | Delivered R18/#237/#238/#246 contract, exclusions retained; report/R13 audit |
| `plans/investment-unknown-basis-plan.md` | Delivered #160 and correction/provenance refinements; knowledge/replay/export audit |
| `plans/investment-unresolved-non-sale-plan.md` | Delivered #169 contract; lifecycle/report audit |
| `plans/investments-plan.md` | Completed R12 design history; later operation/reporting contracts govern extensions |
| `plans/native-sqlite-driver-migration-plan.md` | Delivered #195/#164; ADRs 0015/0016 supersede original durability/resource assumptions; recovery/build/quality audits |
| `plans/pricing-management-plan.md` | Delivered R11 contract; price/UI/observation audit |
| `plans/projected-balances-plan.md` | Accepted R10 contract; dated execution/handoff sections are historical; planning/integration audit |
| `plans/quote-providers-crypto-plan.md` | Delivered R17 contract, rejected scope retained; provider/precision/import audits |
| `plans/receipts-plan.md` | Unaccepted future storage/capture/OCR design; no attachment implementation or scope approval; release/scope audit only |
| `plans/recurring-transactions-plan.md` | Accepted R9 contract; planning/draft/worker audit |
| `plans/reports-plan.md` | Accepted R2 plus reporting FX; deliberate series/filter/general-snapshot limits retained; reporting audit |
| `plans/trading212-import-plan.md` | Delivered R7 history and later corrections; import/provider/identity audit |
| `plans/transaction-ledger-core-plan.md` | Shipped foundation reference, later investment/draft/date contracts apply; ledger/reconciliation/schema audit |
| `design/account-hierarchy.md` | Account/class/kind/budget axes; account and planning audits |
| `design/accounts-system-design.md` | Implemented foundation reference; effective-date/account/institution audit |
| `design/categories-design.md` | Implemented category contract; account/report/planning audits |
| `design/holding-account-opened-date.md` | Accepted historical-opening policy; backdated/import-account audit |

## R13 specification questions exposed by the review

R18 supplies valuation and basis inspection, not a ready returns calculator.
Before #119 implementation, record a feature contract covering:

- Portfolio/security/account boundaries, inclusion of cash and short liabilities,
  and treatment of transfers across the selected boundary, fees, taxes, dividends,
  payment in lieu and corporate-action cash. Internal movements must not become
  external flows merely because they touch more than one account.
- Named TWR/TTWROR and money-weighted equations, flow timing, valuation frequency,
  annualization/day count and exact input/rounding policy. Define zero or negative
  capital, no root/multiple roots and numerical convergence as explicit outcomes.
- Allocation taxonomy, weights/denominator, cash/short/unknown-value treatment and
  whether targets/rebalancing are in the bounded first delivery. No trade execution.
- Complete/partial/missing price and FX coverage, benchmark-series sources and
  calendars, correction behavior, and which saved outputs can be reproduced.
  Manual observations are valid; live benchmarks or broker feeds are optional.
- Independent provider-free expected results and equivalence cases through the
  real consumer before chart work. Reuse R18 valuation policy without implying
  that a basis-method comparison is a time-weighted return.

These are readiness questions, not formula or scope decisions accepted by this
review. The [Portfolio Performance TWR documentation](https://help.portfolio-performance.info/en/concepts/performance/time-weighted/)
illustrates why inflows/outflows and daily valuation conventions must be named;
Ghostfolio's [README](https://github.com/ghostfolio/ghostfolio/blob/main/README.md)
documents ROAI, which must not be presented as an identical metric.

## Evidence contract and completion

For each audit record the commit, scenario, independent expected result,
command/test, actual outcome, limitation and disposition. Use **verified**,
**confirmed defect**, **accepted/deferred limit**, or **not verified**. File a
bounded GitHub issue for a confirmed actionable gap; do not create speculative
defects from a proposal or turn every competitor checkbox into required scope.
Core checks run without live providers; an adapter evidence gap remains local
to that adapter. Existing tests should be reused before adding new scaffolding.

Audit closure needs evidence for all twelve areas, named unresolved findings
and same-change corrections to any capability or contract claims. An audit area
can be evaluated while a documented defect remains open; the final assessment
must state its impact. Native-language sign-off, household use and remote
security settings need their own actual evidence. R13 readiness depends on its
contract and relevant findings, not on unrelated provider payloads or every
future feature. This brief does not itself certify release readiness.

Validation of this documentation change: `git diff --check` and
`SKIP_OPENAPI=1 ./scripts/check-post-merge.sh HEAD` passed. The latter checked
number collisions, duplicate paragraphs/workflows/IDs and retained tests; API
generation was explicitly skipped because no API or generated file changed.
A local reference check verified 155 relative links/anchors, the unchanged
R-number index and coverage of all 31 plan/design references. No application
acceptance, financial race suite or browser suite was run for this docs-only
change. The twelve audits remain pending.
