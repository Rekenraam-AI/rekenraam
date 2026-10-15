# Roadmap

The single source of truth for **what to build next, in what order** —
initiatives and the current focus within them. Governed by
`docs/product-requirements.md`.

Other questions have exactly one home each:

- Ticket acceptance, state, priority and discussion: [GitHub Issues](https://github.com/sergeyfarin/rekenraam/issues).
- What ships today: `docs/implemented.md`.
- Feature design contracts: `docs/plans/`. They never define a sequence.
- Resolving a historical local code to its issue: `docs/backlog.md`.

Do not restate shipped detail here; when a focus item ships, delete it and
record the capability in `implemented.md`.

Last reviewed: 2026-10-15. R16’s agreed gates, R11 price management and R17
quotes/crypto are delivered. R18’s six gains-projection slices are delivered; R13 returns analytics is next.
Owner-local date correctness is delivered; native catalog review remains
parallel work. Prior roadmap detail is retained in the [completed roadmap record](reviews/completed-roadmap-2026-09-27.md).

## Slice index

Every R-number ever used, so references in other documents stay resolvable.
Statuses: ✅ shipped · ◐ partly shipped ahead of its slice · ▶ current ·
⏭ planned (ordered below) · ⏸ deliberately later · — retired/unassigned.

| Slice | Name | Status | Primary document |
|---|---|---|---|
| R1 | Reconcile workflow screen (trust loop) | ✅ | `docs/implemented.md` (Reconciliation) |
| R2 | Reports users can act on | ✅ | `docs/plans/reports-plan.md` |
| R3 | Portable **and protected** core data (CSV/QIF export, backups, restore, self-check) | ✅ | `docs/plans/data-portability-plan.md` |
| R3a | Accessibility regression coverage | ✅ | this file |
| R4 | QIF import | ✅ | `docs/implemented.md` (Import Pipeline) |
| R5 | Ordinary-bank CSV import + profiles | ✅ | `docs/plans/import-plan.md` |
| R6 | Import depth (XLSX/OFX, matching, rollback) | ⏸ | `docs/plans/import-plan.md` |
| R7 | Trading 212 online connections + lots | ✅ | `docs/plans/trading212-import-plan.md` |
| R7a | Daily-entry convenience | ⏸ | this file |
| R8 | Budgets | ✅ | `docs/plans/budgets-plan.md` |
| R9 | Recurring transactions | ✅ | `docs/plans/recurring-transactions-plan.md` |
| R10 | Projected balances / forecasting | ✅ | `docs/plans/projected-balances-plan.md` |
| R11 | Pricing/FX management UI | ✅ | `docs/plans/pricing-management-plan.md` |
| R12 | Investments UI + gains reporting | ✅ | `docs/plans/investments-plan.md` |
| R12a | Investment journal/subledger integrity correction | ✅ | `docs/plans/investment-integrity-plan.md`, ADR 0012 |
| R13 | Investment return analytics (TWR/MWR) | ⏭ | this file |
| R14 | Receipts & attachments (capture, OCR, inbox) | ⏸ | `docs/plans/receipts-plan.md` |
| R14a | Attachment storage + manual attach (after R5) | ⏭ | `docs/plans/receipts-plan.md` |
| R15 | Connections expansion (IBKR Flex → GoCardless → #111 producer) | ⏸ | `docs/plans/connections-plan.md` |
| R16 | Investment lifecycle completeness (correction, transfers, basis actions, splits, short sales) | ✅ agreed gates; related work open | `docs/plans/investment-operation-refactor-plan.md`; ADR 0013 |
| R17 | Crypto instrument type + `PriceProvider` registry and quote adapters | ✅ | `docs/plans/quote-providers-crypto-plan.md` |
| R18 | Reproducible investment basis + gains projections | ✅ | ADR 0012; `docs/plans/investment-reporting-plan.md` |

## Decisions adopted 2026-08-05

The `docs/reviews/roadmap-review-2026-07-19.md` (§3, §4) proposals were
decided by the owner on 2026-08-05. **All were accepted**, each with a scope
fence recorded in the relevant section below:

| Proposal | Decision |
|---|---|
| §3a EU import correctness (T-35, T-36) | Accepted — fixed as defects *and* a hard pre-announcement gate |
| §3b R3 backups + trial-balance self-check | Accepted — self-check is read-only; a documented restore path ships with it |
| §3c Minimal import rules v1 in R5 | Accepted — contains-match, preview-time only, no retroactive apply |
| §3d R10 forecasting promotion | Accepted — planning loop is **R9 → R10 → R8** |
| §3e Investment lifecycle completeness | Accepted — R16 framework in `docs/plans/investment-operation-refactor-plan.md`; each structural action still needs its posting, basis, and date specification before implementation |
| §3f Personal-access tokens | Accepted — scoped, expiring, hashed, revocable; before announcement |
| §4.1 Crypto-holding expat persona | Accepted — new R17, after R16, with a standing guardrail in `product-requirements.md` |

The §2 documentation-accuracy fixes are separate and unconditional; the R12
slice-index row is fixed above.

The three remaining `plans/` questions were decided the same day:

| Question | Decision |
|---|---|
| Connections sequencing | Order adopted, **contents amended**: the quote-provider slice moves into R17 (it builds the `PriceProvider` registry once). R15 is now IBKR Flex → GoCardless → #111 producer |
| Receipts R14a pull-forward | **No** — but R3 designs the backup/self-check with a documented attachments hook, and R14a ships after R5 |
| GoCardless / IBKR "verify" items | Not a decision — reclassified as blocking slice-start preconditions on GC-1 and IBKR-1 in `connections-plan.md` |

The R14/R15 plans (2026-07-19) remain deliberately later. Their sequencing
was decided on 2026-08-05: quotes belong to R17; R15 is IBKR → GoCardless →
#111 producer. R14a stays after R5 and was not pulled forward alongside R3. The Yahoo Finance
quote-provider question inside R15 was decided 2026-08-05 (ship it, labeled
unofficial) — `docs/plans/connections-plan.md`.

## Direction

Build a polished, self-hosted personal-finance daily driver first. The order is:

1. Trust and visibility: reports and exports.
2. Lower manual effort: CSV import with reusable profiles.
3. Repair any trust-boundary defect before adding more financial producers.
4. Daily planning: budgets and recurring transactions.
5. Differentiate for multi-currency, cross-border households and investors.

The product is deliberately single-user and self-hosted. Connections are
bring-your-own-key adapters, never guaranteed coverage. Native apps,
small-business accounting, a hosted service, and broker trade execution are out
of current scope. The trade-execution decision and market analysis are retained
in `docs/reviews/competitive-analysis-2026-07.md`.

## Provider independence decision (2026-10-15)

[ADR 0017](adrs/0017-provider-independent-features-and-incomplete-data.md)
separates app delivery from online automation. Build manual/file-based workflows
and explicit partial attribution without waiting for Trading 212, another API,
or live provider samples. Missing required financial effects still need manual
input or a supported unknown-state workflow; optional enrichment may remain
unresolved with visible coverage. Provider evidence gates apply only to their
adapter mappings. Later R15 placement is sequencing, not a hard dependency for
core features or the existing event/suggestion infrastructure.

## Current plan

Do not start a new roadmap initiative until the current one has met its
acceptance criteria. Priority labels on issues express urgency; the order
below is execution order, not a chain of hard dependencies.

### Current focus

**Now**

1. R13 investment returns and allocation planning
   [#119](https://github.com/sergeyfarin/rekenraam/issues/119)
   can proceed from the delivered R18 contracts
   [#230–#235](plans/investment-reporting-plan.md#delivery-slices) (2026-10-15).
2. Daily-use parallel work. The shared transaction workspace
   [#185](https://github.com/sergeyfarin/rekenraam/issues/185) is delivered
   (2026-10-14): contextual entry, same-day reorder and row actions on the
   journal, account and category screens. Investment entry from holding
   accounts and instruments [#186](https://github.com/sergeyfarin/rekenraam/issues/186)
   is delivered (2026-10-14) with a per-instrument activity view. Cash
   dividends and their withholding now retain the paying instrument through
   corrections and appear in that activity
   ([#229](https://github.com/sergeyfarin/rekenraam/issues/229), 2026-10-15). CSV
   dividend/tax mapping and phone review are delivered in
   [#241](https://github.com/sergeyfarin/rekenraam/issues/241) (2026-10-15).
   App-owned partial-data work proceeds independently of provider automation:
   dividend fallback review/repair [#242](https://github.com/sergeyfarin/rekenraam/issues/242),
   visible attribution coverage [#244](https://github.com/sergeyfarin/rekenraam/issues/244),
   and manual short-operation import links [#245](https://github.com/sergeyfarin/rekenraam/issues/245).
   #229 remains open for the known-payer fallback defect until its repair;
   it does not gate R13 or other features. Reconciliation recovery is tracked
   independently in #240. Approval preserves the journal
   (#219); guarded void/unvoid/delete/restore preview their own impact (#220,
   2026-10-14; trash restore too, #225), register rows are per posting line
   (#221), the register status filter is explicit Posted/Voided (#223) and
   category amounts are per category (#222).

**Delivered native SQLite and test-feedback improvement (2026-10-12):**
[#195](https://github.com/sergeyfarin/rekenraam/issues/195) installs the bundled
native driver under ADR 0014 with schema/seed and durability policies retained.
[#164](https://github.com/sergeyfarin/rekenraam/issues/164) restores race-gate
headroom: matched median full race wall time 27.67 → 3.76 minutes (86.4% less),
application race execution 962.65 → 85.71 seconds, same named regression cohort.
Cold compilation needs a C toolchain and takes longer. See the
[delivery review](reviews/native-sqlite-driver-2026-10-12.md) for all runs,
validation and the native C-engine race-coverage boundary.

**Delivered two-process SQLite validation (2026-10-14):**
[#105](https://github.com/sergeyfarin/rekenraam/issues/105) exercises independent
process work claims, scheduled backup identity and server/restore refusal.
This retains one server per database. [#227](https://github.com/sergeyfarin/rekenraam/issues/227)
also closes the restore/startup and lock-file handoff gaps: restore retains its
lock throughout replacement, and lock files persist between owners. See the
[#105 validation review](reviews/two-process-sqlite-guards-2026-10-14.md) and
[#227 review](reviews/restore-process-lock-2026-10-14.md).

**Delivered zero-delta split gate (2026-10-13):**
[#146](https://github.com/sergeyfarin/rekenraam/issues/146) records sourced splits
without eligible lots journal-free, with entitled backdated acquisition replay,
guarded quantity adjustments, export/restore and self-check. Operation-addressed
inspection, correction, settlement and import linking also ship in
[#196](https://github.com/sergeyfarin/rekenraam/issues/196) (2026-10-13), with the
mobile Splits history surface and both journal shapes covered.

**Delivered historical-entry and recovery gates:** dated holdings/lot selection
[#166](https://github.com/sergeyfarin/rekenraam/issues/166) (2026-10-09),
backdated internal transfer admission
[#167](https://github.com/sergeyfarin/rekenraam/issues/167) (2026-10-10),
whose internal-transfer picker adopts #166, and sourced resolution correction
with journal-free known-zero resolution
[#168](https://github.com/sergeyfarin/rekenraam/issues/168) (2026-10-10).

**Delivered from [#103](https://github.com/sergeyfarin/rekenraam/issues/103):** named short sale and cover —
core commands [#173](https://github.com/sergeyfarin/rekenraam/issues/173), reads and mobile entry
[#174](https://github.com/sergeyfarin/rekenraam/issues/174), correction and backdating
[#175](https://github.com/sergeyfarin/rekenraam/issues/175), all 2026-10-09. Verified provider import
[#176](https://github.com/sergeyfarin/rekenraam/issues/176) stays evidence-blocked outside the R16 bar.

**Delivered from [#114](https://github.com/sergeyfarin/rekenraam/issues/114):**
- Outbound transfers with entry, bridge adjustments, backdating and correction (#157–#159, 2026-10-05).
- Return of capital, completed 2026-10-07 (#161, [#163](https://github.com/sergeyfarin/rekenraam/issues/163)).
- Cash in lieu, completed 2026-10-07 (#162, [#165](https://github.com/sergeyfarin/rekenraam/issues/165)).
- Unknown basis [#160](https://github.com/sergeyfarin/rekenraam/issues/160), completed 2026-10-09. It covers immutable knowledge, unresolved sales and average pools, unknown transfers in, out and between holdings, splits, and sourced resolution with mobile entry; see the [#160 implementation contract](plans/investment-unknown-basis-plan.md).

Every bounded #114 child is delivered; #114 is closed following the
[2026-10-09 audit](reviews/in-kind-basis-actions-audit-2026-10-09.md).
The audit found historical-entry and resolution-recovery gaps; these are
separate R16 completion gates above, rather than reopening delivered children.

**Delivered parallel trust follow-up (2026-10-13):** recorded return-of-capital
excess visibility [#171](https://github.com/sergeyfarin/rekenraam/issues/171),
with effective allocations and immutable history in transaction detail and
the investment summary. Period disclosure and its limited-scope reporting
policy also ship in [#238](https://github.com/sergeyfarin/rekenraam/issues/238)
(2026-10-15). Write-off mobile entry [#172](https://github.com/sergeyfarin/rekenraam/issues/172)
is delivered (2026-10-13), with dated lot selection, exact/unresolved loss
preview and shared gain/checkpoint review. Mixed-cost-currency write-off selection
[#199](https://github.com/sergeyfarin/rekenraam/issues/199) is delivered (2026-10-13),
including historical entry and native replacement.
Neither follow-up reopens #114/#160 or expands R16’s bounded bar.
Unresolved non-sale results [#169](https://github.com/sergeyfarin/rekenraam/issues/169)
are delivered (2026-10-13); see the [non-sale contract](plans/investment-unresolved-non-sale-plan.md).
Basis-resolution provenance declarations [#170](https://github.com/sergeyfarin/rekenraam/issues/170)
are delivered (2026-10-13), with typed source entry and history. Provenance-only
correction [#197](https://github.com/sergeyfarin/rekenraam/issues/197) is also
delivered: immutable evidence revisions at unchanged exact basis, without
financial writes, with transfer history, export and stale-correction guards.

**Automation only, awaiting provider evidence:** [#145](https://github.com/sergeyfarin/rekenraam/issues/145), [#136](https://github.com/sergeyfarin/rekenraam/issues/136).
The [2026-10-13 split evidence recheck](reviews/trading212-split-evidence-2026-10-13.md)
records the current official schema, missing live payloads and collection guide
for #145; the provider-mapping gate remains. Neither it nor execution-cancellation
mapping #136 or short mapping #176 blocks native manual entry/correction or
unrelated app features. Manual split linking already ships; broader manual
linking is app-owned work (#245).

### Completed initiatives through R10

R2 reports, R3 portability and backups, R3a accessibility, the reporting
currency selector, and the R9 → R10 → R8 planning loop are shipped. See the
[capability ledger](implemented.md) for current behavior and the
[dated completed-roadmap record](reviews/completed-roadmap-2026-09-27.md)
for the original scope and acceptance notes.

### R16 — investment lifecycle completeness

Complete the long-position lifecycle on ADR 0013's named operation/side
foundation: corrections for every operation family, in-kind transfers and
basis actions, splits, named short positions and compound corporate actions.
Every new command integrates the shared gain-impact acknowledgement, the
reconciliation preview and export/self-check in its own acceptance; each stays
independently runnable and audited. Negative holdings without named short
events remain unclassified warnings. The
[operation plan](plans/investment-operation-refactor-plan.md) and
[slice 5 contract](plans/investment-operation-slice-5-contract.md) govern
design; [implemented](implemented.md) records the shipped boundary.

R16's agreed completion gates are delivered (2026-10-13). The bounded issues
are historical-entry selection
[#166](https://github.com/sergeyfarin/rekenraam/issues/166), backdated
internal transfer admission [#167](https://github.com/sergeyfarin/rekenraam/issues/167),
sourced resolution recovery [#168](https://github.com/sergeyfarin/rekenraam/issues/168),
short sale/cover [#103](https://github.com/sergeyfarin/rekenraam/issues/103) (delivered via #173–#175),
compound actions [#115](https://github.com/sergeyfarin/rekenraam/issues/115),
zero-delta splits [#146](https://github.com/sergeyfarin/rekenraam/issues/146)
and Refresh reach for revised fills
[#152](https://github.com/sergeyfarin/rekenraam/issues/152).

**Remaining R16-related work.** Delivery of the agreed gates does not mean
that every investment-lifecycle issue is closed. These retained issues keep
their stated scope and placement. R11’s bounded operator surface is delivered;
R17 and R18 are also delivered; the next initiative is R13.

| Issue | Remaining gap | Disposition |
|---|---|---|
| [#145](https://github.com/sergeyfarin/rekenraam/issues/145) | Trading 212 split mapping | Blocked on verified provider semantics and representative payloads |
| [#136](https://github.com/sergeyfarin/rekenraam/issues/136) | Provider execution cancellations | Blocked on provider evidence |
| [#176](https://github.com/sergeyfarin/rekenraam/issues/176) | Provider short/cover import | Blocked on provider evidence |

Provider return-of-capital/cash-in-lieu suggestions also remain follow-ups.

Operation-addressed correction/settlement of initially
journal-free splits is delivered (#196). #114's delivered transfer/basis-action
family is closed under the audit above; the other agreed R16 gates were
delivered separately.

### R11 — price and FX management UI, delivered

The bounded operator surface (#116, #200–#202) is delivered. Its durable
contract is [price management](plans/pricing-management-plan.md); shipped
scope is in [the capability ledger](implemented.md). R18's bounded
[#203](https://github.com/sergeyfarin/rekenraam/issues/203) slice now shares
compatible observation selection across readers and records their intentional
age differences in the [reporting policy](plans/investment-reporting-plan.md).
That policy refinement does not reopen the R11 operator surface.

### Parallel trust work for cross-border entry

[#101](https://github.com/sergeyfarin/rekenraam/issues/101) owner-local default
dates are delivered (2026-10-13): shared authenticated server date, preserved
explicit dates and opposite-midnight browser coverage. Before presenting the
five drafted non-English catalogs as complete or using them in a migration
demo, close [#102](https://github.com/sergeyfarin/rekenraam/issues/102)
(native review; catalog parity and its regression gate are delivered). That
communication work remains independently shippable and does not require waiting for R16 or R11.

### R17 — delivered reference

[#117](https://github.com/sergeyfarin/rekenraam/issues/117) delivers quote-provider
registration/adapters and manual/CSV crypto holdings with lots. Durable behavior,
provider evidence, operator limits and the rejected exchange/DeFi/staking/NFT
scope are recorded in the [quote/crypto contract](plans/quote-providers-crypto-plan.md);
shipped capabilities live in [implemented.md](implemented.md).

### R18 — reproducible investment basis and gains projections

The six delivery slices [#230–#235](plans/investment-reporting-plan.md#delivery-slices)
are delivered (2026-10-15). Their contracts and retained limits live in the
[reporting policy](plans/investment-reporting-plan.md); shipped capabilities and
validation live in [the feature ledger](implemented.md). Formal postings remain
outside this bounded initiative until a need is evidenced. The
[completion audit](reviews/r18-completion-audit-2026-10-15.md) records the
independent live-read consistency (#237, delivered 2026-10-15) and capital-return
period disclosure (#238) follow-ups. R13 follows; its planning does not close #238.

### R13 — investment returns and allocation analytics

Follow R18's reproducible valuation and basis projections. Plan explicit
TTWROR/TWR and money-weighted/IRR equations, external-flow classification,
portfolio versus security scope, missing-price behavior, FX policy, benchmark
selection, and period reproducibility before implementing charts. Use
[Portfolio Performance's documented methods](https://help.portfolio-performance.info/en/concepts/performance/time-weighted/)
as a comparison, not an implicit formula choice. Keep the figures read-only;
reports do not create postings. Core implementation uses existing manual/file
ledger and price/FX inputs with explicit incomplete coverage; Trading 212 history,
a broker-comparison feed and a live benchmark are optional integration work,
not start or completion gates. Manual benchmark observations are a valid input
once their series contract is specified. Missing calculation inputs suppress
only the affected figures under the chosen policy.

## Deliberately later

These are valuable, but they are not allowed to displace the current plan:

- R6: XLSX and OFX/QFX adapters, import matching, per-split mapping, batch
  rollback, and richer import history. `docs/plans/import-plan.md` retains the detailed
  data, lifecycle, and acceptance design.
- Import rules **beyond the R5 v1 fence**: regex and amount predicates,
  retroactive re-run over committed transactions, and a rule audit trail.
  Also duplicate payee merge, bulk recategorization, and the imported
  `needs_review` queue. (The minimal contains-match rules are in R5 as of
  2026-08-05.)
- R15 connections expansion, in this order (adopted 2026-08-05):
  **IBKR Flex Query → GoCardless EU/UK banks → the #111 dividend/
  corporate-action event producer**, then SimpleFIN Bridge (US) later.
  Security quotes moved out of R15 into R17. CSV mapping-profile presets for
  API-less brokers (Trade Republic, DeGiro, Raisin, HL/AJ Bell/ii) are R5
  documentation-plus-fixtures tasks, not adapters. All bring-your-own-key;
  assessment matrix, free-vs-paid verdicts, and slice designs in
  `docs/plans/connections-plan.md`. Both first slices carry blocking
  provider-verification preconditions for their online adapters — see that plan.
  These do not block shared manual/file workflows or unrelated app features;
  #111's later position is order, not an IBKR/GoCardless technical prerequisite.
- R7a daily-entry convenience: transaction templates, payee defaults, saved
  views, and keyboard-first entry plus general entry review/edit helpers such
  as previous/next entry, open/edit, save-and-next, duplicate, split, and
  audit-safe bulk changes. Add user-customizable action bindings and selectable
  shortcut presets modelled on Rekenraam, Quicken, Microsoft Money, and other
  documented schemes (for example, an MS Money-style `Ctrl+M` reconciled
  action). Presets are starting points rather than claims of full compatibility:
  conflicts and platform differences must be visible, commands must remain
  discoverable and have mouse/touch equivalents, and no shortcut may bypass
  reconciliation guards, confirmations, or audit history.
- R14 receipts & attachments: durable attachment storage (resolves the open
  attachment product decision), receipt capture with in-browser OCR, and a
  match-or-draft inbox using the reserved draft-producer workflow —
  `docs/plans/receipts-plan.md`. **R14a ships after R5**, not alongside R3
  (decided 2026-08-05): it is not an announcement gate, so it does not go
  between two slices that are. R3 carries the attachments hook instead.
- General report snapshots beyond R18’s saved operational investment
  gains/statement documents, multi-user, and household features. Core
  reporting-currency conversion is already shipped.

## Competitor and parity check

Keep `docs/competitor-comparison.md` as the maintained parity matrix and
`docs/reviews/competitive-analysis-2026-07.md` as the dated deep dive. Before declaring
each roadmap initiative complete, update the comparison's implication section
and record either the parity gained or the deliberate gap retained.

The 2026-09-27 primary-source recheck in
`docs/competitor-comparison.md` supports the following order: complete R16's
correction, transfer, basis and split workflows; provide the R11 price/FX
operator surface; add the R17 quote registry and crypto entry; then build
R18 reproducible gains and R13 returns. #101 owner-local date defaults are
delivered parallel correctness work. This ordering is a product
inference from verified competitor workflows and the present code boundary,
not a claim of feature uniqueness.

## Public-release gates

**Fourth-pass review (2026-09-19):**
`docs/reviews/ledger-investments-fourth-pass-2026-09-19.md` verifies that the
previous six regression tests pass, and found an earlier investment-role
validation window (reopened T-100, P1), precision-dependent realized gains
(T-101, P2), and non-cash legs counted in cashflow transfers (T-102, P2).
**All three are fixed as of 2026-09-19**, each with named regression tests that
fail without their fix and with the reviewer's own probes passing, so the
previous reproductions are closed. Subsequent test hardening found **T-103
(P2): partial-disposal basis depends on purchase text precision**, **fixed
2026-09-20** by fixing the allocation precision to the cost commodity's own
maximum scale, one scale per position, with explicit range backoff — so the
original code-side reproduction is closed. Follow-up verification found T-104
(later acquisitions could exceed already widened projection range), fixed by
atomic range admission checks with active sequence regressions. Gains-summary
display now uses standard currency precision; see
`docs/reviews/allocation-and-gains-verification-2026-09-20.md`. See
`docs/reviews/financial-test-hardening-2026-09-19.md`. T-94's draft-promotion
correction remains verified. The household dry run and investment feature
limits still remain required; the candidate baseline must be validated after
ADR 0013's pre-release redesign.

T-106's dated negative-position detection and net-worth warning shipped
2026-09-26. Out-of-order imports remain accepted; a genuine short sale needs
an explicit named workflow (#103, commands delivered in #173). T-107's investment money JSON precision
boundary also closed on 2026-09-26.

These are a parallel release-readiness track, not a reason to delay local
daily-driver work.

The earlier v0.1 migration freeze and reliability dispositions are recorded in
`docs/reviews/v0.1-release-triage-2026-09-11.md`. ADR 0013 supersedes the
unused candidate's freeze before installation. The first-pass fixes close
their original seven reproductions and the second-pass fixes close its four, so
the latest review above now governs the code release gate; the non-code gates
also still stand.
Public-announcement work below is a separate, later gate.

### Public repository hygiene

The repository is already public (verified 2026-09-27). `LICENSE`,
`SECURITY.md`, Dependabot and the `govulncheck` workflow exist. Verify the
complete Git history for secrets and confirm the repository's GitHub secret
scanning and push-protection settings; the current checkout alone cannot
establish those remote settings. Maintain the disclosure and dependency
scanning paths before an announcement.

### Before public announcement or marketplace listings

1. ✅ Complete R2, R3, and R5 so a newcomer can migrate, inspect, and export
   data. All three shipped.
2. ✅ **EU import correctness: T-35 and T-36 fixed and regression-tested**
   (added 2026-08-05, review §3a) — done 2026-08-06 for the QIF path
   (`app/import_locale.go`). **CSV re-check done 2026-09-12** now that R5 has
   landed, and it found one more (T-91): both adapters resolved the *date*
   layout across the whole file but the *decimal separator* one value at a
   time, so a German export's "1.234" read as 1.234 rather than 1234 — a
   silent 1000x error on the exact migration path the announcement leads with.
   Now detected file-wide for both adapters, the way dates already were.
   The announcement's centerpiece is the
   migration story, the QIF parser targets MS Money exports, and the persona
   is European — so the demo the launch rests on must not corrupt dates and
   amounts for exactly the target audience. A correctness-branded finance app
   does not get a second first impression.
3. **Cross-border entry trust — open.** #100’s locale-aware amount input and
   #101’s owner-local default dates are delivered.
   #102 catalog parity and its regression gate are delivered. Assign native
   reviewers and record terminology sign-off before describing those catalogs
   as complete; see the [review packet](reviews/localization-native-review-2026-10-13.md).
4. **Personal-access tokens — open** (added 2026-08-05, review §3f). The
   typed OpenAPI surface is the foundation for an ecosystem, but session
   cookie + CSRF header auth means no script, tool, or community client can
   call it. Announcement is the moment of maximum developer attention.
   Required shape: hashed at rest, scoped, expiring by default, revocable,
   and emitting authentication events.
5. ◐ Produce signed release binaries with reproducibility notes.
   **Reproducibility rechecked 2026-10-12** (#195): native SQLite uses
   `-trimpath`, CGO and a pinned bundled engine. Identical Go/C toolchain,
   source and embedded frontend inputs produce matching hashes. The Linux
   artifact links libc/libm; fully static linking is separate work. See
   `docs/developer-workflow.md` § Reproducibility. **Signing is still open** and needs an owner decision:
   a signing identity (Sigstore keyless via GitHub OIDC, or a held key) and a
   release workflow, neither of which exists yet — there is no release job in
   `.github/workflows/`, only `ci.yml` and `govulncheck.yml`.
6. Prepare adoption assets: seeded demo, README screenshots, and a short
   migration walkthrough — courting the plain-text-accounting audience
   explicitly by leading with the correctness architecture (append-only
   versions, exact decimals, trial balance).

**Every public-deployment security gate is closed, and parked pending an
internet-exposed deployment.** MFA (S-06) shipped 2026-08-07 as TOTP plus
single-use recovery codes, with a Settings → Security screen and the second
step in the login flow; lockout-safe login throttling (S-04) and
authentication-event visibility (S-07) closed 2026-08-06. **Parked 2026-08-19
(owner decision):** the app is self-hosted locally for now, so none of S-04,
S-06, or S-07 is scheduled work; the trigger to unpark is the first time an
internet-exposed deployment is planned, R3 at the earliest. What is left
before that point is an operator action rather than product work: the owner
account must be **enrolled** before real financial data goes on the internet,
and `REKENRAAM_SECRET_KEY` must be configured for enrolment to be possible.
See `docs/deployment-security.md`.

## Open product decisions

Resolve these only when their related slice becomes current work:

- Export scope beyond core ledger CSV/QIF, including a full structured JSON
  backup of settings and metadata.
- The highest-priority mobile workflow. (First non-English UI languages were
  decided 2026-08-19: Spanish, French, Dutch, German, Russian — recorded in
  `product-requirements.md`. Native review of the drafted translations still
  needs an owner to pick a reviewer per language.)
- Attachment storage, retention, access-control, backup, and encryption model
  — a proposed resolution now exists in `docs/plans/receipts-plan.md` (R14a);
  decide when that slice is scheduled.

## Completed milestones

Foundation, accounts, ledger transactions, reconciliation UI, QIF import,
Trading 212 ingestion (including investment lots), reports (net worth,
spending, cashflow, with filters and drill-down), and investments UI/gains are
shipped. See `docs/implemented.md` for the capability ledger and the historical
design records for rationale.
