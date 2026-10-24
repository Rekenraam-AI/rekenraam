# Maintainability audit: final ranking — 2026-10-24

**Status: seventh and final slice of [#284](https://github.com/sergeyfarin/rekenraam/issues/284);
ranking published, owner review pending.** This report consolidates the six
measurement slices into one ranked candidate table, explicit
investment/reporting dispositions for R13, and one "considered and rejected"
list. It accepts no refactor and opens no issue. After the owner reviews this
list, each accepted candidate gets its own bounded issue, as #284 requires.
"Nothing worth doing" was an allowed result; the evidence supports a short
list instead.

Recorded source: runtime and tests at `a514e188951178b54ca9b43b901c92e725a215e7`;
working HEAD `6c7ec383` differs only in audit documents. Every candidate's
evidence was re-checked at that source: owners named below exist at the cited
paths, and the corporate-action clone group from slice 2 reproduces at the same
lines (`dupl -t 150`: 71 runtime rows, down from 75).

## Measurement record

Each #284 method has a bounded, reproducible pass with its own limits:

| Method | Report | Source |
|---|---|---|
| Input reconciliation, July §5–§7 and October schema review, churn × size, change amplification, bug-fix history | [Slice 1](maintainability-audit-2026-10-21.md) | `13d15eab` |
| Runtime duplication (Go `dupl`, TS/Svelte jscpd), all 87 rows triaged | [Slice 2](maintainability-duplication-2026-10-21.md) | `13d15eab` |
| Invariant-enforcement inventory: 34 rule groups | [Slice 3](maintainability-invariants-2026-10-21.md) | `13d15eab` |
| Layer leaks, frontend money/date logic, request fan-out | [Slice 4](maintainability-layers-2026-10-21.md) | `13d15eab` |
| Dead code, Knip, single-caller interfaces/options | [Slice 5](maintainability-reachability-2026-10-21.md) | `13d15eab` |
| Reconciliation to current source, test fixtures/oracles/duplication, race cost | [Slice 6](maintainability-tests-2026-10-24.md) | `a514e188` |

Criterion numbers are #284's: **1** divergence or bug history, **2** a named
upcoming change touching N sites, **3** a financial or security rule enforced
in more than one place, **4** a single-caller abstraction with measured cost,
**5** structure that makes meaningful tests disproportionately hard.

## Ranked candidates

| # | Candidate | Evidence | Crit. | Affected modules | Cost | Payoff tied to named work | Risk | Safety net | If nothing is done | Recommendation |
|---:|---|---|---|---|:-:|---|---|---|---|---|
| 1 | **One owned self-check assertion family** (test-only) | Two of four copies missed `10a03db7`'s whole-run sweep; 81 call sites never assert checks outside a short list or the run status; one copy is byte-identical. Strict bodies passed in all 74 reaching tests, with a negative control ([slice 6](maintainability-tests-2026-10-24.md#self-check-assertion-helpers-diverged)) | 1, 5 | `backend/internal/app/*_test.go`: four helpers; `requireHealthyUnresolvedBook` stays distinct | S | R13's portfolio tests will assert self-check health; new checks are then covered everywhere at once | Strengthening could expose a latent finding (none found today) | The experiment itself; conservation and movement oracles; #266 register | The next new check is silently skipped at 81 sites, as `investment_attribution` is today | **Do now** |
| 2 | **Owned browser assertions for book-wide reads** | Two specs check containment in the book-wide self-check sample, capped at 20 and shared across files on one worker; one spec branches on the observed run status | 5 | `investments-cash-in-lieu.spec.ts`, `investments-payment-in-lieu.spec.ts`, `data-settings.spec.ts` | S | R13 whole-portfolio journeys must not inherit the pattern. Smallest resolution: assert the per-transaction diagnostic the same specs already open, or use a dedicated book | Losing the end-to-end self-check link | Existing per-transaction diagnostic assertions; backend self-check tests | Latent order-dependent failure as warning producers grow | **Before R13 implementation** |
| 3 | **Application-owned native realized totals** | `api/investments.go:gainsProjectionResponse` aligns scales, counts unknown basis and enforces int64 range for per-currency totals; `app/investment_gains_statement.go` repeats the grouping ([slice 4](maintainability-layers-2026-10-21.md#backend-layer-dispositions)) | 3 | `api/investments.go`, `app/investment_gains_statement.go` | S/M | Needed only if R13's contract consumes native realized totals; R13 must then read an app-owned model, not copy an HTTP helper (N = 2 today, 3 with R13) | Changing ordering, null/unknown semantics, range errors or saved bytes | HTTP mixed-scale, large-string and unknown-basis tests; saved-output tests | A third copy of financial policy in R13 | **Before R13, conditional** on its contract using realized totals; otherwise **later** |
| 4 | **Shared browser review-lifecycle controller** | 13 components repeat preview/review/stale-impact/commit state transitions around shared `gain-impact.ts` leaves ([slice 2](maintainability-duplication-2026-10-21.md#frontend-review-lifecycle-add-a-bounded-candidate)) | 2, 3 | `frontend/src/lib/investments/`, `transactions/`, provider completion forms | M | #395 adds preview/commit correction forms for cash dividends, withholding, payments in lieu and reinvestments (N = 4 more). Not an R13 dependency: R13 reads and adds no preview/commit protocol | Reusing a stale acknowledgement, auto-retrying a write, focus/pending changes | Stale-review and reconciliation browser journeys; `gain-impact` unit tests; server-side enforcement | #395 copies the protocol four more times | **Before #395's UI slice**; not an R13 prerequisite |
| 5 | **Corporate-action correction preparation** | #309/#310 sibling divergence; the clone group (share exchange, spin-off, cash-and-stock merger inverse plans) persists; #392 removed one guard from four correction checks (N = 4) | 1, 3 | `app/investment_{share_exchange,spin_off,cash_stock_merger}_correction.go`; split correction for the guard step | M | Each correction-policy change touches 3–4 siblings. No named upcoming change targets them; R13 reads results and does not edit these paths | Erasing family-specific entitlement, no-op or acknowledgement rules; moving checks out of the writer transaction | Per-family late-refusal/recheck tests; `TestCorporateActionCorrectionDistinguishesExactSourceNumbers`; #266 and part-2 registers | Coordinated review of 3–4 pipelines per policy change | **Later**, as an independent bounded issue; **no longer before R13** (#119 requires a concrete R13 conflict) |
| 6 | **Migration-chain test cost** | Across two matched local pairs, 74 migration/restore/backup/bundle tests grew 15.5–18.7% as the chain went from 20 to 26 migrations; new tests are the other stable cause. CI median is about 183 s against ADR 0016's 210 s line ([slice 6](maintainability-tests-2026-10-24.md#race-suite-cost-and-headroom)) | 5 | Application migration and restore tests | S/M | Keeps R13's added tests inside ADR 0016 headroom without raising the deadline | Losing coverage of real upgrade paths that a nearer start would skip | Head-version and downgrade-refusal tests; ADR 0016 bound | Headroom keeps shrinking by about six migrations' worth per feature week | **Later**; re-measure with the slice 6 method each slice and act when a matched run or the CI median crosses 200 s |
| 7 | **Checkpoint candidate walkers** | `periodScopedCandidatesFromTransaction`/`FromRecord` copy the same four posting fields; the early courtesy refusal also repeats (#270) | 3 | `app/transactions_reconciliation.go`, `transactions_write.go` | S | Later ledger producers | Losing sequence/date fields; moving authoritative checks outside the writer | Lifecycle, reorder and checkpoint-impact HTTP tests | A candidate-field change must edit both | **Later** |
| 8 | **Error-classification leaves** | July missing-overflow history; four validation writers and three typed-overflow arms repeat classification | 1 | `api/transactions_errors.go`, `ledger.go`, `investments.go`, `pricing.go` | S | Revisit when R13 defines its numerical failure codes | Case precedence and checkpoint payloads are public behavior | Response-schema audit; precision/refusal API tests; generated error typing | A new shared error can miss one writer | **Later**, triggered by R13's error contract |
| 9 | **Frontend exact comparison/range leaves** | Correction lot choice and price admission repeat scaled comparison and int64 checks owned by `money/amount.ts` | 3 | `investments/trade-economics.ts`, `pricing/form.ts` | S | Fewer coordinated numeric-admission edits | Changing error precedence or pricing's scale-18 limit | Mixed-scale, large-coefficient lot and pricing tests | Two more places to update on an admission change | **Later** |
| 10 | **Shared browser investment builder** | 41 specs create an instrument and holding through local setups; no coordinated sweep since 10-01 | 2 (conditional) | `e2e/playwright/*.spec.ts`, `support/` | M | Only if a named change alters instrument/holding creation (N = 41) | A shared builder can change seeded order or hide expectations | Existing spec assertions | None measured | **Later**, triggered as stated |
| 11 | **Ledger balancing scale alignment** | `validateBalanced` still aligns scales manually with `big.Int` after July's `ScaledInt` delivery | 3 | `app/transactions_validate.go` | S | None named | Cross-entry cancellation or a scale bug in balancing | Mixed-scale per-entry characterization, to be written first | Low; no imbalance found | **Later**, characterization first |
| 12 | **Legacy price-schema probe** | `ListPriceObservations` runs a `PRAGMA table_info` per call and keeps `listLegacyPriceObservations`. Baseline 0001 defines `base_commodity_id NOT NULL` and no migration drops it, so the branch is unreachable on any migrated database | 4 | `db/pricing.go` | S | None named; removes one schema query per list call | A direct repository caller on an unmigrated file; the runtime always migrates on start (`appruntime`) | Pricing list tests; migration head-version tests | One extra query per call; dead branch | **Later** hygiene |
| 13 | **Source hygiene bundle** | Slice 5's list (not re-scanned): five runtime helpers unreachable in both views once `ExportService.SetNowForTest` gained a test caller; three unreferenced interfaces; 41 unreferenced frontend exports and 35 types; two unreachable test helpers; the `AlternativeGainsProjection` test-only boundary ([slice 5](maintainability-reachability-2026-10-21.md), [slice 6](maintainability-tests-2026-10-24.md#test-only-entry-points-traced-to-live-consumers)) | — | Listed in the slice 5 catalogs | S | None; no measured cost | Removing the wrapper while believing the live guard was removed (`requirePositionBasisRangeQueryTx` stays) | Slice 5 catalogs and live-path traces | Minor reading cost | **Later**, optional; does not qualify on its own |

Rank orders recommendations by urgency, then evidence strength. Costs are
relative S/M/L estimates. "Before R13" means before R13 implementation; R13
specification may proceed now.

## Investment and reporting dispositions for R13

#119 lets R13 implementation start once these dispositions exist and its own
calculation contract is accepted. Each named hotspot now has one:

| Hotspot | Disposition | R13 consequence |
|---|---|---|
| `db/investments.go:valuePositions`, `DatedPositionsWithGains`, observation selection | **Retain and reuse** | R13 valuation reads build on these owners; no new valuation engine |
| Side/knowledge-aware projection policy (shortlist candidate 2) | **Reject** the broad consolidation; reuse `valuePositions`, `InternalTransferAllocation` and `EffectiveLotMovements` | Any smaller helper needs identical-input characterization first |
| Native, FX, statement and saved reporting contracts | **Reject** merging | R13 keeps original-currency figures and labelled coverage separate |
| `api/investments.go:gainsProjectionResponse` | Candidate 3 | Conditional before R13 |
| Shared investment writer (`runInvestmentJournalsWithGuardTx`) | **Retain** | R13 adds no investment journal writer; any saved-run storage follows the existing gains-run contract |
| Corporate-action correction preparation | Candidate 5, later | Not an R13 conflict |
| `app/self_check.go`, `api/investments.go`, `app/investments.go`, `db/investments.go` size | **Reject** splitting by length | No R13 effect |
| Test oracle for R13 | Candidates 1 and 2; plus R13's own literal cross-family register (payment/cash in lieu, withholding, spin-off, share exchange, merger, capital return, write-off, reinvestment) | Part of R13's calculation contract, tests only, not a #284 refactor |

R13's concrete prerequisites from #284 are therefore limited to candidates 1
and 2, plus candidate 3 if its contract uses native realized totals. All three
are small and test- or read-side. Nothing here requires a schema change before
v0.1.0.

## Considered and rejected

Consolidated from all slices, so these are not raised again without new
evidence:

- **Splitting files or adding an operation registry** because files are long
  (slices 1, 2). Length alone does not qualify.
- **Repeating #189's effective-movement consolidation**: already delivered.
  Replay inputs and immutable evidence use deliberately different contracts.
- **One calculation for native, FX and saved reporting**, or saved-schema
  changes: their date policies, numeric types and immutable bytes are intended.
- **Removing SQL CHECKs or runtime override checks** because Go validates
  first: storage and hand-built configs are separate admission boundaries.
- **Whole-book replay per write, a generic revision header or a baseline
  rewrite** as cleanup.
- **Rebuilding modal or unknown-basis status abstractions**: #327/#342/#313
  delivered them.
- **A generic CRUD/handler, form or SQL-reader rewrite** from clone matches
  (slice 2). Matching AST shapes encode different scopes and copy.
- **Moving transaction-coupled policy out of `db`** to satisfy a directory rule
  (slices 3, 4). It must see the same admitted facts as its effects.
- **A browser report engine, merged chart policies, or a localized parser for
  trusted machine decimals** (slice 4).
- **Removing single-consumer interfaces wholesale** (slice 5). Narrow SQL,
  backup and provider boundaries have no measured removal benefit.
- **Deleting test-root-only wrappers** because production does not reach them
  (slices 5, 6). Each traced wrapper sits on a live owner.
- **Consolidating raw-SQL test fixtures** (slice 6). Six schema changes edited
  none, and damage probes must stay raw.
- **Extracting Go test clones** (slice 6). One pair at the runtime threshold.
- **Replacing differential, rollback or preview/commit assertions with
  literals** (slice 6). Each financial one is anchored or has a literal sibling.
- **Merging `requireHealthyUnresolvedBook` into the strict helper**: it encodes
  the unknown-basis contract (#313).
- **Raising the 300-second deadline, lowering parallelism or removing
  assertions** to recover headroom (ADR 0016).
- **Consolidating #395's cash-family fixed-field preparation as a refactor**:
  #395 already owns that product change and its single correction contract.
- **The shared private `apiFetch` copies** in `api/connections.ts` and
  `api/imports.ts` as a ranked item: real duplication, but no history or named
  change. It joins the hygiene bundle if touched.

## Owner review and next steps

For each ranked row, the owner accepts (a bounded issue follows), defers or
rejects. Accepted issues follow #284's rules: behavior preserved, one bounded
slice each, characterization before moves, and a pattern sweep rather than only
the named copies. Accepting zero remains valid.

After that review, #284 can close: the report, ranked table and rejected list
are published here, July §5–§7 were reconciled in slice 1, and follow-up issues
carry any accepted work. Roadmap order, release holds and #260 are unchanged.

## Validation

Re-checked at `a514e188`: all owner functions named in the table exist at the
cited paths; the slice 2 corporate-action clone group reproduces at the same
line ranges; the legacy price branch is unreachable from the baseline schema.
Relative links and anchors, `./scripts/check-post-merge.sh HEAD` and
`git diff --check` pass. No runtime, API, schema or test change; no suites
rerun beyond slice 6's.
