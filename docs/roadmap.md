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

R13 status reviewed: 2026-10-26 against `b9c57ea3`, its successful hosted CI,
live #119 and the owner's closure/follow-up decision. Queue reviewed
2026-10-26 against `76f6eece` and live GitHub state: the maintainability queue
is complete and #405's follow-ups are listed under Current focus. The
remaining queue retains the 2026-10-24 owner-reviewed order. The documentation and competitor review is recorded in the
[app-state review and audit brief](reviews/app-state-and-audit-plan-2026-10-17.md).
R16, R11, R17 and R18 have met their bounded delivery bars. **R13's
returns_v1 core is delivered: all six accepted slices (live read, screen,
series, allocation, benchmark, saved runs) shipped 2026-10-24..25, and
per-instrument returns (`instrument_returns_v1`, slices 8–9) followed by
2026-10-26. #119's bounded delivery is accepted for closure; targets and advisory
rebalancing are separately deferred as #411/#412.**
The cross-app engineering audit is delivered; owner/release follow-ups remain
explicit below. **#402 is delivered with the owner's disposition: ADR 0016
thresholds amended. #404 followed the same day: explicit statement reuse
shipped and the native statement cache was declined. #405 is delivered on
`main` (2026-10-26): the [schema review](reviews/schema-baseline-review-2026-10-26.md)
records the slimmed baseline, workflow findings and matched measurements.
Hosted CI is green at `0ac0a6cf`; #405 is closed.
#403, found on the way, is fixed; #284's owner review is complete. #399
(app-owned native realized totals), #400 (shared browser review
lifecycles) and #401 (corporate-action correction preparation) were delivered
early at the owner's request on 2026-10-24.** Earlier milestone detail lives in the
[completed roadmap record](reviews/completed-roadmap-2026-09-27.md)
and the [capability ledger](implemented.md).

## Slice index

Every R-number ever used, so references in other documents stay resolvable.
Statuses: ✅ shipped · ◐ partly shipped ahead of its slice · ▶ current ·
⏭ planned (ordered below) · ⏸ deliberately later · — retired/unassigned.

| Slice | Name | Status | Primary document |
|---|---|---|---|
| R1 | Reconcile workflow screen (trust loop) | ✅ | `docs/implemented.md` (Reconciliation) |
| R2 | Reports users can act on | ✅ | `docs/plans/reports-plan.md` |
| R3 | Portable **and protected** core data (CSV/QIF export, backups, restore, self-check) | ✅ | `docs/plans/data-portability-plan.md` |
| R3a | Accessibility regression coverage | ✅ | `docs/implemented.md` (Accessibility) |
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
| R13 | Investment return analytics (TWR/MWR; per-instrument slices 8–9 delivered) | ✅ | [returns contract](plans/investment-returns-plan.md) |
| R14 | Receipts & attachments (capture, OCR, inbox) | ⏸ | `docs/plans/receipts-plan.md` |
| R14a | Attachment storage + manual attach (scope decision still required) | ⏸ | `docs/plans/receipts-plan.md` |
| R15 | Connections expansion (IBKR Flex → GoCardless; the #111 dividend producer shipped early, 2026-10-17) | ⏸ | `docs/plans/connections-plan.md` |
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

These are historical sequencing decisions, not the current delivery queue.
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

1. **Retain the completed engineering audit and its separate release gates.**
   [#260](https://github.com/sergeyfarin/rekenraam/issues/260)'s twelve original
   areas and subsequent engineering audits have delivered their bounded evidence;
   the issue retains unchecked owner items #336/#337. The
   [audit brief](reviews/app-state-and-audit-plan-2026-10-17.md) and linked dated
   reports preserve the scenarios, findings, shared-gate SHAs and explicit limits.
   #326's URL selections and corrected route evidence are also
   [delivered](reviews/url-selection-delivery-2026-10-21.md).
   Remaining follow-ups are signing decisions
   [#333](https://github.com/sergeyfarin/rekenraam/issues/333), actual owner
   deployment/household evidence
   [#336](https://github.com/sergeyfarin/rekenraam/issues/336), owner image-advisory
   disposition [#337](https://github.com/sergeyfarin/rekenraam/issues/337), and the
   recorded upstream driver-release prerequisite
   [#356](https://github.com/sergeyfarin/rekenraam/issues/356).
   Adoption assets [#335](https://github.com/sergeyfarin/rekenraam/issues/335)
   remain P3 for later. These retained gates do not block the accepted
   maintenance; release remains on hold and #260 is not closed by this
   sequencing update.
2. **Deliver the immediate accepted maintainability slices, one at a time.**
   [#284](https://github.com/sergeyfarin/rekenraam/issues/284)'s six measurement
   slices and [final owner-reviewed dispositions](reviews/maintainability-audit-2026-10-24.md)
   complete the bounded audit. The owner accepted original candidates 1–6 and
   deferred 7–13. The immediate execution queue is complete (2026-10-26):

   | Order | Issue | Bounded outcome |
   |---:|---|---|
   | 1 — delivered 2026-10-24 | [#397](https://github.com/sergeyfarin/rekenraam/issues/397) | One strict self-check assertion owner (`requireKnownBasisSelfCheckPasses`); the unknown-basis variant and independent oracles are kept. |
   | 2 — delivered 2026-10-24 | [#402](https://github.com/sergeyfarin/rekenraam/issues/402) | [Measured](reviews/race-suite-cost-2026-10-24.md): migration replay is 4.5% of application race CPU; growth is schema size (per-connection load, statement compilation) plus new tests. No coverage-preserving test change is justified. Explicit thresholds (ADR 0016 amendment) replace the 200 s trigger; the owner sequenced compilation-cost work as #404. |
   | 3 — delivered 2026-10-24 | [#398](https://github.com/sergeyfarin/rekenraam/issues/398) | Book-wide self-check assertions run in a per-test owned book (`support/owned-book.ts`) with exact status, counts and samples and no outcome branching; transaction facts read the per-transaction diagnostic; a saturated owned book (21 unrelated records) pins the sample cap. |
   | 4 — delivered 2026-10-24 | [#404](https://github.com/sergeyfarin/rekenraam/issues/404) | [Measured](reviews/statement-compilation-2026-10-24.md): 22% of compile time repeats inside a transaction, 47% only across transactions. Track A (statement sets in five hot loops) lowered the app median 223.3 → 219.4 s and the full-gate pair to 255.5/274.1 s, so the act threshold no longer fires (thinly); investigate still does. The native cache measured −4.4% (≈2.7% beyond A) and the owner declined it; `_stmt_cache_size=0` stays. |
   | 5 — delivered 2026-10-26 | [#405](https://github.com/sergeyfarin/rekenraam/issues/405) | [Reviewed and measured](reviews/schema-baseline-review-2026-10-26.md): one baseline, two dormant tables/fifteen indexes/five views removed; all 164 triggers retained. Fix unused crypto cleanup and validate seed generation through its native consumer. Application median 240.7 → 229.6 s; full-gate application pair 271.8/289.0 → 248.5/262.5 s. Act no longer fires; investigate remains. Follow-ups #417 (category-version deletion guard), #418 (dead search declarations; delivered 2026-10-26), #419 (reprofile), #420 (`verify-backup` exit status). Hosted CI green at `0ac0a6cf`; the earlier unrelated E2E app flake is #414. |

   #405's retained follow-ups are outside this queue.
   Three are CI/browser reliability:
   [#414](https://github.com/sergeyfarin/rekenraam/issues/414) (P3; a
   capital-return reversal flake that turned `main` red once; failed attempts
   now keep traces, and the cause awaits a recurrence),
   [#415](https://github.com/sergeyfarin/rekenraam/issues/415) (runner
   setup stalls; delivered 2026-10-27: the Go cache source key uses git object
   IDs and fails open, and Chromium installs with per-phase deadlines and one
   retry) and [#416](https://github.com/sergeyfarin/rekenraam/issues/416)
   (an axe contrast flake). The other three are bounded P3 items:
   [#417](https://github.com/sergeyfarin/rekenraam/issues/417) (database-level
   category-version deletion guard; delivered 2026-10-27 as forward migration
   `0002`),
   [#419](https://github.com/sergeyfarin/rekenraam/issues/419) (reprofile the
   race suite while ADR 0016's investigate threshold still fires) and
   [#420](https://github.com/sergeyfarin/rekenraam/issues/420)
   (`verify-backup` exit status). After #405, ADR 0016's app-median
   "investigate" threshold still fires. Reprofile (#419) before new suite-cost
   work if a full-gate run again exceeds 270 s.

   These are the chosen maintenance slots, not prerequisites retroactively
   imposed on delivered R13. The separate product-policy
   audit [#390](https://github.com/sergeyfarin/rekenraam/issues/390), delivered
   #391/#388/#389/#392/#393/#394 contracts, and accepted financial-field
   correction feature [#395](https://github.com/sergeyfarin/rekenraam/issues/395)
   remain distinct from behavior-preserving maintenance; do not restore
   retired attribution blockers.
3. **Continue the remaining accepted maintenance after R13 core.**
   [#400](https://github.com/sergeyfarin/rekenraam/issues/400) is delivered
   (2026-10-24, owner-requested ahead of this slot): `previewReview` and
   `commitReview` own the compatible browser review lifecycles, including
   #395's dividend-correction and payment-in-lieu sites.
   [#395](https://github.com/sergeyfarin/rekenraam/issues/395)'s
   financial-field correction feature is delivered (2026-10-26, three slices:
   cash dividends and standalone withholding, payments in lieu, reinvested
   dividends) through the existing forms, writers and review controllers; see
   the [plan's delivery record](plans/financial-dependency-simplification-plan.md#financial-field-correction-delivery-395).
   Independent corporate-action preparation
   [#401](https://github.com/sergeyfarin/rekenraam/issues/401) is delivered
   (2026-10-24, owner-requested ahead of this slot): one leaf owns the
   admission and verification the four corporate-action correction families
   share. [#399](https://github.com/sergeyfarin/rekenraam/issues/399) is already
   delivered (2026-10-24, owner-requested ahead of this slot). Each accepted refactor remains a separate bounded slice.
   Original candidates 7–13 are **explicitly deferred without issues** in the
   [deferred inventory](reviews/maintainability-audit-2026-10-24.md#explicitly-deferred-inventory-original-ranks-713).
   Revisit only on the recorded trigger and refreshed evidence; open a bounded
   issue then if justified. They are outside this accepted execution queue.
4. **Continue independent readiness work.**
   [#102](https://github.com/sergeyfarin/rekenraam/issues/102) needs native
   terminology review, not another catalog-parity implementation. Recheck
   [#112](https://github.com/sergeyfarin/rekenraam/issues/112)'s upstream
   TypeScript/OpenAPI-generator compatibility when considering the upgrade;
   its old blocker text is not fresh compatibility evidence.

### R13 — accepted delivery and deferred extensions

[#119](https://github.com/sergeyfarin/rekenraam/issues/119)'s bounded return
analytics delivery is accepted for closure on 2026-10-26. The
[plan's disposition and evidence](plans/investment-returns-plan.md#accepted-delivery-and-follow-up-disposition-2026-10-26)
record the shipped slices, regression cases, successful CI and accepted limits;
[implemented.md](implemented.md) records the capability. R13 is removed from
current focus.

The owner retained two **deferred P3 extensions**, outside the current execution
queue: [#411](https://github.com/sergeyfarin/rekenraam/issues/411) for target
allocation/drift, and [#412](https://github.com/sergeyfarin/rekenraam/issues/412)
for advisory rebalancing. Each needs its own accepted contract; rebalancing
depends on the target model. No trade execution is implied. Keeping these
issues does not move them ahead of the retained release/readiness work
above. [#413](https://github.com/sergeyfarin/rekenraam/issues/413), also
P3, measures the returns read (including `instruments=true` and saved runs)
on a large investment book, which the large-book harness has never seeded. It
is evidence work, not a gate. #405 has since changed the cost baseline, so
measure against the new baseline. No additional R13 implementation is required to close #119.

### Completed initiatives through R10

R2/R3/R3a, R4/R5 and the R9 → R10 → R8 planning loop are accepted.
[Implemented features](implemented.md) records capabilities and retained limits;
the [completed record](reviews/completed-roadmap-2026-09-27.md) preserves earlier
scope decisions. Audit their present interactions rather than restarting them.

### R16 — investment lifecycle completeness

The agreed gates are delivered. The [operation contract](plans/investment-operation-refactor-plan.md),
[slice 5 contract](plans/investment-operation-slice-5-contract.md) and ADR 0013
record named operations, corrections, dated replay and explicit refusals.
Provider automation remains separate from native/manual acceptance (ADR 0017).

| Adapter-only follow-up | Evidence needed before automatic admission |
|---|---|
| [#136](https://github.com/sergeyfarin/rekenraam/issues/136), Trading 212 execution cancellations | Verified execution identity, cancellation/revision relationship and financial effects |
| [#145](https://github.com/sergeyfarin/rekenraam/issues/145), Trading 212 splits | Verified ratio, effective date and entitlement payloads |
| [#176](https://github.com/sergeyfarin/rekenraam/issues/176), provider short/cover rows | Explicit provider side semantics and representative payloads |
| [#256](https://github.com/sergeyfarin/rekenraam/issues/256), Twelve Data splits | Unambiguous forward/reverse ratio and effective-date semantics |

None blocks manual operations, file workflows, R13, another adapter or the
existing reviewed dividend producer. Other event-family mappings are deferred
extensions, not implied by the completion of #111.

### R11 — price and FX management UI, delivered

The [price-management contract](plans/pricing-management-plan.md) records the
bounded operator surface. Consumer-specific observation selection and age rules
live in the [R18 policy](plans/investment-reporting-plan.md).

### Parallel trust work for cross-border entry

Locale-aware amounts and owner-local financial dates ship. Native terminology
sign-off remains [#102](https://github.com/sergeyfarin/rekenraam/issues/102);
use the [review packet](reviews/localization-native-review-2026-10-13.md) before
claiming the five drafted non-English catalogs are complete.

### R17 — delivered reference

The [quote/crypto contract](plans/quote-providers-crypto-plan.md) records priced
manual/CSV crypto holdings, quote adapters and entitlements. Exchange APIs,
DeFi, staking and NFT scope remains rejected.

### R18 — reproducible investment basis and gains projections

All six bounded slices and the live-consistency/capital-return disclosure
follow-ups are delivered. The [reporting policy](plans/investment-reporting-plan.md)
records supported projections, frozen operational documents and exclusions.
TWR/IRR, saved alternative/FX runs, broker comparisons, tax policy and automatic
accounting postings are not implied by R18 acceptance.

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
  Also duplicate payee merge, bulk recategorization and a dedicated import
  review inbox beyond the delivered `needs_review` filter/approval flow. (The minimal contains-match rules are in R5 as of
  2026-08-05.)
- R15 connections expansion, in this order (adopted 2026-08-05):
  **IBKR Flex Query → GoCardless EU/UK banks**, then SimpleFIN Bridge (US)
  later. The #111 dividend event producer (Twelve Data, dividends only) was
  delivered ahead of this order on 2026-10-17 at the owner's request. The
  evidence-blocked Trading 212 mappings stay separate (split fills #145,
  execution cancellation #136, short/cover rows #176). The payment-in-lieu
  operation short dividends need (#255) was delivered on 2026-10-17, and the
  #111 producer now proposes it for a short-only entitlement.
  Security quotes moved out of R15 into R17. CSV mapping-profile presets for
  API-less brokers (Trade Republic, DeGiro, Raisin, HL/AJ Bell/ii) are R5
  documentation-plus-fixtures tasks, not adapters. All bring-your-own-key;
  assessment matrix, free-vs-paid verdicts, and slice designs in
  `docs/plans/connections-plan.md`. Both first slices carry blocking
  provider-verification preconditions for their online adapters — see that plan.
  These do not block shared manual/file workflows or unrelated app features;
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
- R14 receipts & attachments: proposed durable attachment storage (requires
  accepting the open attachment product decision), receipt capture with in-browser OCR, and a
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

The [2026-10-17 source recheck and 2026-10-26 local status update](competitor-comparison.md)
record R13 returns/allocation as delivered alongside R16 lifecycle, R11 pricing,
R17 crypto and R18 gains inspection. Targets/drift (#411) and advisory
rebalancing (#412) remain deliberate gaps. These delivered capabilities are
not claims of full product, metric or tax parity.
Actual's import/rules and PocketSmith's planning workflows also justify auditing
everyday migration and planning usability now. R6 depth and R7a convenience
remain deliberate gaps, not automatically promoted features. Verify the core
journeys before expanding integrations; provider signup/coverage claims need
fresh adapter-specific evidence. These are product inferences from documented
workflows, not market-wide uniqueness claims.

## Public-release gates

Historical financial reviews and their fixed reproductions remain in
[the September release triage](reviews/v0.1-release-triage-2026-09-11.md),
[the fourth pass](reviews/ledger-investments-fourth-pass-2026-09-19.md) and
[subsequent verification](reviews/allocation-and-gains-verification-2026-09-20.md).
They do not certify the October operation, driver and reporting changes.
The [new audit brief](reviews/app-state-and-audit-plan-2026-10-17.md) covers the
present combined baseline, failure/recovery paths and release evidence.
A candidate still needs the documented release preflight, an owner household
dry run, tested restore/export and an explicit list of investment limits.
Synthetic automated acceptance does not substitute for that dry run.

Release readiness is a parallel track; announcement and marketplace listing
remain separate from local daily-driver delivery.

### Before the v0.1.0 tag

- **Schema consolidation delivered, 2026-10-26:
  [#405](https://github.com/sergeyfarin/rekenraam/issues/405).**
  The [review](reviews/schema-baseline-review-2026-10-26.md) and per-object
  inventory retain financial contracts and named evidence with coverage limits.
  Native objects fall 511 → 489 and stored SQL falls 2.3%; one baseline file
  packages the reviewed schema. Head is 1, generation 20261026. Requires the
  documented `BREAKING DEV DATABASE` reset; no legacy databases exist.
  Installed-release migrations become append-only at the tag. Separate release
  gates remain unchanged; retained findings do not jump ahead of them. Hosted CI is green at `0ac0a6cf`; an unrelated browser
  flake seen on the first hosted run is tracked as #414.

### Public repository hygiene

The repository is already public (verified 2026-09-27). `LICENSE`,
`SECURITY.md`, Dependabot and the `govulncheck` workflow exist. Verify the
complete Git history for secrets and confirm the repository's GitHub secret
scanning and push-protection settings; the current checkout alone cannot
establish those remote settings. Maintain the disclosure and dependency
scanning paths before an announcement.

The [2026-10-18 deployment audit](reviews/deployment-audit-2026-10-18.md)
records a complete local-history scan with five inspected synthetic-test
detections, no reachable Go vulnerability and no JavaScript advisory. Remote
settings and the real operator/household acceptance remain #336; residual
runtime-image advisory triage remains #337. These are dated, bounded checks.

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
4. ✅ **Personal-access tokens — #334** (delivered 2026-10-20). Dedicated
   read-only `ledger:read` and `reports:read` credentials are hash-only,
   scoped by exact route, expiring by default, revocable and emitting
   attributed authentication events. Owner management includes password
   confirmation and one-time disclosure. [Contract](plans/personal-access-tokens-plan.md),
   ADR 0018. No write/export/settings/backup grant exists. Independent
   adversarial/proxy/restore audit #357 is complete with #359 fixed
   ([evidence](reviews/personal-access-token-audit-2026-10-20.md)). Adjacent
   expiry-clock audit #360 is complete with #362–#365 fixed
   ([evidence](reviews/authentication-expiry-audit-2026-10-20.md)); reservation
   settlement repair/audit #366 is complete
   ([evidence](reviews/throttle-reservation-audit-2026-10-20.md)). Actual owner
   deployment #336 retains its evidence requirements. This delivery is not public-release sign-off.
   [Dated delivery evidence](reviews/personal-access-tokens-delivery-2026-10-20.md)
   records the 16 browser cases, shared gates and fixed selector finding #358.
5. ◐ Produce signed release binaries with reproducibility notes (#333).
   **Reproducibility rechecked 2026-10-12** (#195): native SQLite uses
   `-trimpath`, CGO and a pinned bundled engine. Identical Go/C toolchain,
   source and embedded frontend inputs produce matching hashes. The Linux
   artifact links libc/libm; fully static linking is separate work. See
   `docs/developer-workflow.md` § Reproducibility. **Signing is still open** and needs an owner decision:
   a signing identity (Sigstore keyless via GitHub OIDC, or a held key) and a
   release workflow, neither of which exists yet — there is no release job in
   `.github/workflows/`. The existing `ci.yml`, `govulncheck.yml` and scheduled
   `large-book.yml` and `release-preflight.yml` are validation workflows.
6. Prepare adoption assets (#335): seeded demo, README screenshots, and a short
   migration walkthrough — courting the plain-text-accounting audience
   explicitly by leading with the correctness architecture (append-only
   versions, exact decimals, trial balance).

**Authentication mechanisms ship; deployment acceptance is still required.**
TOTP/recovery codes, lockout-safe throttling and authentication-event recording
are delivered; trusted-device management and authentication-event APIs lack
management screens. Local deployment does not require reimplementing these
mechanisms. An internet-exposed installation requires HTTPS, retained secret-key
configuration and actual owner MFA enrollment, plus current operator/security
checks in [deployment guidance](deployment-security.md). An older closed
security issue alone does not prove a deployment safe.

## Open product decisions

Resolve these only when their related slice becomes current work:

- (Decided 2026-10-18: no settings/metadata interchange or bundle importer. The
  native backup is the only full-installation round trip, and the bundle stays a
  portable ledger export. Recorded in `product-requirements.md` and decision 14
  of the [data portability plan](plans/data-portability-plan.md).)
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
