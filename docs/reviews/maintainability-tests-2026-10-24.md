# Maintainability: test ownership, oracles and duplication — 2026-10-24

**Status: sixth slice of [#284](https://github.com/sergeyfarin/rekenraam/issues/284),
bounded test-ownership review delivered; overall audit partial.** This slice pins
current source, reconciles the earlier fixed-SHA slices with the 34 commits since
then, and inventories backend/browser fixtures, oracle independence and test
duplication. It also measures the application race-suite cost that #394 reported
close to its deadline. No runtime, API, schema or test code changes. No test is
deleted, weakened or rewritten. No refactor is accepted here; final ranking and
owner review remain.

Source: `a514e188951178b54ca9b43b901c92e725a215e7` (2026-10-24), reconciled
against the slice 1–5 baseline `13d15eabd68ed2d3b6177512c9c8fe56c528bbf0`. The
earlier reports and their retained artifacts remain fixed-SHA evidence and are
not rewritten; this report refreshes the dispositions those intervening commits
affect. Work order stays with the [roadmap](../roadmap.md).

## Retained evidence

| Evidence | Method / coverage | Limit |
|---|---|---|
| [Inventory driver](fixtures/maintainability-test-inventory.py) | Extracts both refs with `git archive`, runs the two Go tools below, writes every CSV/JSON listed here and [provenance](fixtures/maintainability-test-provenance-2026-10-24.json) | Lexical/AST navigation; every category the report relies on was triaged by hand |
| [Function hashes](fixtures/maintainability-test-function-hashes.go), [reconciliation](fixtures/maintainability-test-reconciliation-2026-10-24.csv) | `go/printer` hash of every runtime Go function at both refs; flags names that the slice 3 ownership matrix cites | Detects body changes, not semantic equivalence. Renamed functions appear as removed plus added |
| [Assertion provenance tool](fixtures/maintainability-test-assertions.go), [6,667 assertions](fixtures/maintainability-test-assertions-2026-10-24.csv), [financial differential index](fixtures/maintainability-test-financial-differential-2026-10-24.csv) | Every testify `Equal`/`EqualValues`/`Exactly`/`JSONEq`/`ElementsMatch`/`InDelta` in Go tests; each side traced to a production call directly or through a same-function local | Name-based, not type-resolved. `exact.*` representation leaves and standard-library/testify calls count as literal. Custom `require*` helpers are only counted as anchors by the second index |
| [Fixture consumers](fixtures/maintainability-test-fixtures-2026-10-24.csv), [raw SQL writes](fixtures/maintainability-test-raw-sql-2026-10-24.json) | 20 named shared constructors; raw `INSERT`/`UPDATE`/`DELETE` statements in Go tests at both refs | Call sites, not executions. SQL table names are lexical |
| [Browser classes](fixtures/maintainability-test-e2e-classes-2026-10-24.csv), [exact counts](fixtures/maintainability-test-e2e-counts-2026-10-24.csv) | All 86 Playwright specs; ten lexical classes; all 164 `toHaveCount(n)` lines with a scope class | Class counts are matches, not assertions proved meaningful. Every unscoped-looking count was read by hand |
| [Test clone reports](fixtures/maintainability-test-dupl-2026-10-24.txt), [TS test clones](fixtures/maintainability-test-jscpd-2026-10-24.json) | Pinned `dupl` from slice 2 over 384 Go test files at thresholds 150/100/75; pinned jscpd 4.0.5 over 163 E2E and frontend test sources | Syntactic clones only; semantic setup duplication is measured separately below |
| [Race timing](fixtures/maintainability-test-race-timing-2026-10-24.json) | `go test -race -json` of the application package at both refs on one 10-CPU host; CI app-package durations from 45 sampled successful `main` runs since 2026-10-11; serial fixture probes | One host, sequential runs; CI is a different machine class and is noisy. Timing is a cost observation, not a regression proof |

## Source reconciliation since `13d15eab`

The 34 commits include #380, #381–#383, #385, #388/#389, #391–#394 and the
toolchain bump. Runtime Go functions went from 3,336 to 3,364: 223 changed,
36 added, eight removed. The baseline count reproduces slice 3's 3,336 function
declarations exactly, which acts as the extractor's control. Six migrations
(0021–0026) were added under the pre-v0.1.0 rewrite policy.

Owners named in the [slice 3 matrix](maintainability-invariants-2026-10-21.md)
that changed:

| Matrix row | Change at `a514e188` | Refreshed disposition |
|---|---|---|
| I10 payment-in-lieu entitlement | **Removed** by #388: `entitledPaymentsInLieuTx`, `requirePaymentsInLieuStillEntitledTx`, `checkPaymentInLieuEntitlementTx` and the book-wide before/after recheck in the writer | The historical "retain" row is retired, not restorable. The replacement owners are `db/investment_attribution.go` (`ValidatePaymentInLieuAttribution`, `evaluatePaymentInLieuAttributions`) and its cash-in-lieu/dividend siblings, which feed the `investment_attribution` self-check. They are read-time diagnostics, not write blockers ([ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md)) |
| I8 shared writer | `runInvestmentJournalsWithGuardTx` lost the payment snapshot/recheck; guard, journals, effects, net checkpoints, gain acknowledgement and final callback remain | **Retain shared writer.** One fewer step; no new admission path |
| I1 foundation record | `recordInvestmentFoundationTx` takes a typed dividend fact and nullable payment attribution (`NULLIF`) | **Retain typed families.** Optional attribution is storage-level, so it is not a wrapper candidate |
| I7 correction preparation | #392 removed the same `requireNoCashInLieuSettlesTx` call from **four** family correction checks: split, share exchange, spin-off and cash-and-stock merger | **Strengthens candidate 1**: one policy change, N = 4 identical edit sites in sibling preparation. Family financial effects still differ |
| L1 account eligibility | `requireArchiveConsistencyTx` now reads owner-local reference dates | See M5 |
| M5 calendar boundaries | #385 (`2047c017`) introduced `db.WithReferenceDate`/`ReferenceDate` with `SettingsService.referenceContext`, touching 62 runtime files | **Delivered owner, not a candidate.** The sweep is the cost of introducing one context boundary; later readers use it rather than recomputing UTC days |

Other reconciliation facts that affect this slice:

- `api/recurring.go`'s private nullable field was generalised to
  `api/nullable_field.go` and `app/nullable_patch.go` during #388. That is an
  in-feature consolidation with a real second consumer, already delivered.
- Production test-clock hooks grew from 11 to 16 `SetNowForTest` methods, plus
  four new `SetSettings` setters. They are test seams on services, not fixture
  builders. Slice 5's unreferenced `ExportService.SetNowForTest` is now used by
  `reference_date_test.go` (#385), so that hygiene row is resolved.
- Schema changes needed no edits to existing raw-SQL fixtures (below).
- #380 is repaired in `9fea1d45` (recorded in `72b77b0d`), but the GitHub issue
  remains open. Closing it is an owner action outside this slice.

## Backend fixture ownership

At `a514e188`: 384 Go test files and 101,675 lines; 3,224 test-file functions,
of which 2,357 are top-level `Test*` and 867 are helpers (12,319 lines). Helpers
are concentrated in `app` (459) and `api` (316). Only four helper names occur in
more than one package (`readMFAStatus`, `assertMoneyValue`, `posting`,
`countRows`); package-local copies are not a sweep problem.

Fixture construction is **already centralised**, not fragmented:

| Constructor | Call sites / files | Builds |
|---|---:|---|
| `app.newInvestmentsTestFixture` | 774 / 134 | Copied migrated database, EUR and one security, cash/holding/income accounts, trading account, real account/transaction/pricing/investment services |
| `api.newSetupTestHandler` | 506 / 100 | Full HTTP handler over a fresh database |
| `api.createInstrumentForSession` / `createHoldingAccountForSession` | 131 / 44, 132 / 41 | Public-API investment setup |
| `app.seedTestSecurityCommodity` | 100 / 20 | Additional security |
| `app.newInvestTestFixture` | 60 / 15 | Trading 212 import fixture |

All of them sit on `testdb.Open`, which copies one process-wide migrated
template per test. A serial probe in disposable trees measured the copy at
13.6 ms (baseline) and 13.1 ms (head) per open under race, and the investment
fixture at a median 4.50 s versus 4.77 s per 40 constructions (about 112 ms vs
119 ms each). Six migrations therefore did not materially raise the per-test
floor.

**Raw SQL in tests.** 575 statements (311 `INSERT`, 217 `UPDATE`, 47 `DELETE`)
in 141 files touch 103 tables, up from 538 in 131 files. The heaviest targets
are `commodity_versions` (30), `investment_lot_state` (29) and `account_versions`
(24). Many `UPDATE`/`DELETE` statements are deliberate damage or trigger probes
for storage defenses. The six schema commits changed raw-SQL test lines only by
**adding** migration and damage tests: 2047c017 (2 files), 87154f3a (2),
9e5b8a26 (2, plus one deletion in the density probe), 4ad8373f (2), ec837aaf (3)
and a514e188 (1). No existing fixture needed a schema repair. The #302 incident
remains historical; there is no measured amplification in this window.

**Syntactic test clones are rare.** At slice 2's runtime threshold of 150, Go
tests produce one pair (`import_trading212_invest_test.go`). Lower thresholds
give 65 rows in 29 files (100) and 118 rows in 47 files (75), led by HTTP
handler table cases in `api/transactions_test.go` (21). Those establish distinct
request contracts. TypeScript tests have 1.81% duplicated lines (360 of 19,876;
34 clones), mostly within one spec.

### Self-check assertion helpers diverged

Five named helpers assert a test book's self-check, with materially different
strength:

| Helper | Calls | Asserts | Origin |
|---|---:|---|---|
| `requireInvestmentSelfCheckPasses` | 152 | Every applicable check passed, run passed, effective movements, conservation oracle | Strengthened by `10a03db7` (2026-10-18) |
| `requireSpinOffSelfCheckPasses` | 15 | **Identical body** to the above | Strengthened by the same commit |
| `requireSelfCheckPasses` | 77 | Four named checks only (lot reconciliation, foundation, position sign, replay); no run status | Not updated by `10a03db7` |
| `requireInvestSelfCheckPasses` | 4 | Two named checks only, on the import fixture | Not updated |
| `requireHealthyUnresolvedBook` | 72 | Every result, with lot reconciliation allowed to be `incomplete` exactly when lots are unavailable | Intentionally different unknown-basis contract |

`10a03db7` swept "every #273 sequence" to whole-run assertions and missed two
copies. That is the duplicated-helper sweep failure recorded in the
[validate-and-ship checklist](../../.claude/skills/validate-and-ship/SKILL.md)
(T-36/T-45/T-47), this time in tests. The consequence is concrete: at those 81
call sites, every check outside the named selection goes unasserted, including
#389's later `investment_attribution` check, and so does the overall run status.
That includes the capability-scenario test meant to prove that every operation
kind passes self-check.

**Disposable strengthening experiment.** In an extracted copy of `a514e188`,
the bodies of `requireSelfCheckPasses` and `requireInvestSelfCheckPasses` were
replaced with the strict whole-run form; the investment fixture also kept the
movement and conservation oracles. All 121 top-level tests in files that
call either helper passed under race. A negative control then changed the
run-status expectation to an impossible value: 74 top-level tests failed with
the probe message (111 occurrences), and the remaining 47 do not call these
helpers. So the strict form is reached in 74 tests, and it passes in all of
them today. The weaker copies protect no intentional exception; they only
leave later checks unasserted. The repository's tests are unchanged.

## Oracle independence

Of 6,667 equality assertions, 4,754 compare literal or test-computed values on
both sides and 1,584 compare a literal expectation with a production result.
329 (4.9%) take the **expected** side from a production call: 224 against
another production value and 105 against a literal or test value. By the
expected expression, 207 are identity/state (IDs, versions, statuses, dates)
and 84 are other non-financial values. 38 assertions in 28 test functions look
financial.

Each of those 28 functions was read and classified. With custom literal helpers
such as `requireScaled(t, 3000, 2, …)` counted, 22 also pin literal figures.
The pattern is a rollback snapshot (before equals after), preview-equals-commit
agreement, or a reversal equal to the negated original, each anchored by
literal amounts. The six without a literal anchor in the same function:

| Test | Purpose | Disposition |
|---|---|---|
| `TestPreviewSellAndSellProduceIdenticalAllocationsAcrossCostBasisMethods` | Preview/commit agreement across FIFO, LIFO, average cost and specific lot | Agreement contract. Method figures are pinned literally elsewhere, for example FIFO/LIFO ordering, average-cost pool and split/specific-lot conservation tests |
| `TestDisposalDecisionPreservesEveryResolutionTier` | Method-resolution tier | Not a financial oracle |
| `TestInvestmentLotsSimulateDoesNotMutateLots` | Simulation leaves lots unchanged | No-mutation snapshot; literal figures are not its purpose |
| `TestCostBasisAllocationScaleBacksOffForRangeOnMagnitudeNotSpelling` | Scale policy property | Uses `basisFitsInt64At` bounds both ways; property, not an oracle |
| `TestCorrectTrading212SalePreservesSpecificElectionWithoutGuessingQuantityChanges` | Lot identity is retained | Regex false positive ("Lots"); identity |
| `TestQuoteDecimalNeverUsesBinaryFloat` | Table literals | Provenance false positive: table rows are literals |

The two property oracles compute expectations from independently tracked
inputs. `investments_sequence_properties_test.go` tracks shares, proceeds and
acquisitions in `big.Rat` and checks journal, lots and gains against them.
`investment_conservation_oracle_test.go` reads the stored capital-return
receipt, an immutable input fact. **No Go test derives expected gains or
valuation by calling replay or valuation helpers.**

Frontend unit tests (56 files; four use mocks) never compute an expectation by
calling the module they import. Browser specs import only API schema types and
translation catalogs. They format literal figures with `Intl.NumberFormat`,
which is the same platform presentation leaf production uses.
`locale-amounts.spec.ts` keeps a literal grouping control.

### #266 register coverage before R13

`fixtures/reference-book.json` and its seeders are unchanged since the
baseline. The literal register covers dividends, splits, shorts, transfers,
FIFO/average cost and FX. It has **no rows** for payment in lieu, cash in lieu,
withholding, spin-off, share exchange, cash-and-stock merger, capital return,
write-off or reinvestment. Those families have per-family literal tests, but
not a cross-family register. #119 must classify exactly these cash flows as
external/internal for return equations, so R13's independent oracle needs
them. This is a test-safety gap to close as part of R13's calculation contract,
not a refactor. It does not change the #266 contract or its known-defect marker.

## Test-only entry points traced to live consumers

Slice 5 listed 55 functions reachable only from tests. The financially relevant
ones were traced:

| Test-root wrapper (test calls / files) | Live owner reached | Disposition |
|---|---|---|
| `InvestmentService.ListUnrealizedGains` (13 / 7) | `UnrealizedGainsProjection` → `db.DatedPositionsWithGains` | Thin convenience over the live dated owner. **Retain** |
| `InvestmentService.AlternativeGainsProjection` (7 / 2) | Resolves its own current boundary, then `alternativeGainsAtBoundary` → `AlternativeMethodGains`. Production enters via `AlternativeGainsForProfile` with the operational profile's boundary | Its boundary resolution is not the live path. The live path is covered by five HTTP files, including the reference book. **Later hygiene**: retarget the seven app calls if the wrapper is retired |
| `db.PositionsWithGains` (4 / 2) | `valuePositions` | Retain (slice 5) |
| `db.DisposeLots` / `SimulateDisposeLots` (33 / 2, 3 / 1) | `disposeLotsWithAuditTx`, shared with the sale writer, replacement, alternative gains and replay simulation | Algorithm tests over the shared leaf. **Retain** |
| `InvestmentService.ReverseSplit` (21 / 5) | `ReverseSplitWithIdentity` → repository `ReverseSplit` | Thin return-shape wrapper. Retain |

No test-only wrapper hides an otherwise untested live path within these traces.

## Browser fixture and assertion ownership

86 specs declare 285 tests (380 executed cases with locale loops, per #385's
run). Each Playwright worker owns one app process and SQLite file, **shared by
every file that worker runs** (`fullyParallel: false`; route and subview sweeps
are separate projects). Lexical classes per spec:

| Class | Specs |
|---|---:|
| Public-API fixture calls | 74 |
| Unique fixture names (`Date.now()`, test info) | 69 |
| Literal financial text | 50 |
| Route landing | 47 |
| Exact counts | 47 |
| Accessibility scans/contrast | 36 |
| Route stubs (`page.route`) | 18 |
| Keyboard focus | 17 |
| Self-check | 3 |
| Direct database edit | 2 (`legacy-dividend.ts`) |

These classes establish different facts. Route landing, opened sub-view,
keyboard focus, accessibility and independent financial checks are not
redundant with one another merely because they visit the same screen.

**Exact counts are owned.** Of 164 `toHaveCount(n)` lines, 78 use a scoped
locator, 84 a named variable and two count page rows. All 86 non-scoped lines
were read. They count rows inside the spec's own dialog, history, article or
row; restrict reports to the spec's own `account_id`s in the URL (as
`reports.spec.ts` does); or, in the two page-row cases, count rows of a
route-stubbed response (`investment-gains-display.spec.ts`). The #344 shared-book
lookup class is not present in the current count assertions.

**Book-wide self-check reads are not owned.** Self-check runs over the whole
worker book:

- `data-settings.spec.ts` branches on the observed run (`incomplete` vs
  `failed`) because the worker book's other contents decide which applies. It
  is a deliberate tolerance, and also the only conditional assertion of its kind.
- `investments-cash-in-lieu.spec.ts` and `investments-payment-in-lieu.spec.ts`
  assert that their own transaction appears in a **sample** capped at
  `SelfCheckSampleLimit = 20`. Sample order is payment-in-lieu records, then
  cash-in-lieu, then dividends, each by operation ID. A worker that has already
  accumulated 20 earlier warnings (or unavailable records for the payment
  check) drops the newest transaction from the sample, and the containment
  assertion fails for reasons unrelated to the journey. In this tree, the
  current warning producers in the `app` project are the payment-in-lieu spec
  (about seven), the cash-in-lieu locale loop (six) and dividend clearing. So
  the margin exists but is not enforced. **Not reproduced**; latent order
  dependence. The owned alternative already exists: the per-transaction
  `investment-attribution-diagnostic` the same specs assert.

**Investment setup is duplicated but stable.** 41 specs create an instrument
and holding through their own setup functions; `support/` has ledger builders
but no investment builder. Since 2026-10-01 only two commits touched five or
more investment specs (`12b88ec3` worker parallelism, 30 files; `38052707`
locale split, 5), both test infrastructure. No request-shape change has
required a coordinated spec sweep. A shared builder has no criterion-1 history.
It becomes criterion 2 only if a named change alters instrument/holding
creation, with N = 41.

## Race-suite cost and headroom

ADR 0016 sets a 300-second package deadline and requires 30% application
headroom (at most 210 s) for its `-p 2` schedule. #394 recorded 293.507 s locally
under the full gate.

**CI history.** App-package race durations from 45 sampled successful `main`
runs ([retained](fixtures/maintainability-test-race-timing-2026-10-24.json)):

| Window | Runs | Min | Median | Max |
|---|---:|---:|---:|---:|
| 2026-10-12 to 15 (after ADR 0016) | 12 | 68.4 s | 116.5 s | 140.2 s |
| 2026-10-16 to 19 | 17 | 84.6 s | 140.4 s | 172.8 s |
| 2026-10-20 to 21 | 6 | 111.3 s | 181.2 s | 196.5 s |
| 2026-10-22 to 24 | 6 | 139.1 s | 182.6 s | 203.7 s |

Hosted runners are noisy: the baseline `13d15eab` took 152.6 s while the
docs-only `6cb8b044` (same runtime and tests) took 196.5 s. The latest run,
203.7 s at `a514e188`, leaves 32.1% headroom, just above ADR 0016's 30%
admission line. The trend is real; single runs are not attributable.

**Matched local runs.** Two sequential baseline/head pairs on one 10-CPU host:

| Run | Wall | Leaf tests | Leaf work | Effective parallelism |
|---|---:|---:|---:|---:|
| Baseline 1 | 171.6 s | 2,195 | 1,704.5 s | 9.93 |
| Head 1 | 198.0 s | 2,316 | 1,976.7 s | 9.98 |
| Head 2 | 219.8 s | 2,316 | 2,203.3 s | 10.02 |
| Baseline 2 | 198.4 s | 2,195 | 1,976.1 s | 9.96 |

Same-source runs differ by 13–16%, but head exceeds baseline in both pairs
(+15.4% and +10.8%). Effective parallelism is about 10 on 10 CPUs: the package
is CPU-saturated, so wall time is total leaf work divided by cores. No single
test is the critical path. The largest leaf is 33.2 s
(`TestTransferChainDeeperThanFormerRoundLimitPropagatesInOnePass`); the top 100
leaves are 27% of work; investment tests are 73%.
`TestEveryOperationKindExportsItsRowsAndPassesSelfCheck` reports 130.8 s, but
its five parallel scenarios take 1.3–1.8 s each. The parent's time is
parallel-slot queueing, not work.

Attribution across both pairs:

| Source of added work | Pair 1 | Pair 2 |
|---|---:|---:|
| 135 new leaves (63 s in `investment_cash_in_lieu_parent_test.go` in pair 1) | +137.9 s | +164.9 s |
| 74 existing migration/restore/backup/bundle leaves (chain 20 → 26 migrations) | +39.8 s (+15.5%) | +55.6 s (+18.7%) |
| 2,107 other existing leaves | +107.1 s (+7.5%) | +21.3 s (+1.3%) |

The stable causes are **new tests** and **longer migration chains** in tests
that migrate or restore across versions. The "other" growth does not reproduce
across pairs (pair 1's 7.5% versus pair 2's 1.3%), so it is treated as host
noise. Self-check gained one check (13 → 14), a plausible small contributor
that was not measured separately. Fixture construction is not the cause: the
database copy did not grow, and the investment fixture rose about 6% serially.
In CI, the 10-18 audit suites (`investment_diagnostic_audit_test.go`, 6.1% of
head leaf work; `investment_operations_audit_test.go`, 2.2%) and the 10-20
throttle rollover tests (2.9%) coincide with the median step from about 140 s
to 181 s.

#394's 293.507 s was the full `-p 2` gate, where the application package
shares the host with another package. That is a different measurement from
CI's per-package time and from these single-package runs.

## Consequences for the provisional ranking

These refine, not finalise, the [provisional shortlist](maintainability-audit-2026-10-21.md#provisional-ranked-shortlist):

| Candidate | New evidence | Criterion | Cost | Recommendation |
|---|---|---|---|---|
| 1. Corporate-action correction preparation | #392 removed one guard from four sibling correction checks (N = 4) | 1, 2 | M | **Before R13** disposition stands, now with a measured edit count |
| 3a. One owned self-check assertion family (new split of 3) | `10a03db7` missed two of four copies; 81 call sites miss checks added since; one helper is a byte-identical copy | 1, 5 | S | **Do now, test-only**: one strict helper plus the explicit unknown-basis variant. The disposable strict run passed in all 74 consuming tests, so this strengthens assertions without changing behavior |
| 3b. Owned browser assertions for book-wide reads (split of 3) | Capped-sample containment and status branching depend on other files in the worker book | 5 | S | **Before R13**: assert the spec's own diagnostic or a dedicated book; R13 whole-portfolio journeys must not inherit the pattern |
| 3c. Shared browser investment builder (split of 3) | 41 local setups; no amplification since 10-01 | 2 (conditional) | M | **Later**; trigger only on an instrument/holding request change |
| New: R13 cross-family literal register | #266 register lacks nine cash-flow families that #119 must classify | 5 | M | **Before R13 implementation**, as part of R13's calculation contract; tests only |
| Race work budget | CI median about 183 s against ADR 0016's 210 s admission line; growth tracks new tests and migration-chain length; CPU-saturated | — | — | Not a refactor and not a deadline change. **Before R13 implementation**: the owner decides whether migration-chain tests should start from a nearer version (behavior-preserving test cost) or whether R13 accepts the trend. Re-measure per slice with this method rather than single CI runs |

Candidates 2, 4, 5, the native-total and frontend-leaf rows, and the browser
review-lifecycle candidate are unchanged by this slice.

## Considered and rejected in this slice

- Consolidating raw-SQL fixtures into builders: six schema changes needed no
  edits to existing raw fixtures, and damage probes must stay raw.
- Extracting Go test clones: one pair at the runtime threshold; lower-threshold
  matches are HTTP contract cases.
- Replacing differential, rollback or preview/commit assertions with literals:
  they prove lifecycle facts, and each financial one is literally anchored or
  backed by a literal sibling test.
- Treating `requireHealthyUnresolvedBook` as a duplicate: it encodes the
  unknown-basis contract (#313) and must stay distinct.
- Raising the 300-second deadline, lowering test parallelism or removing
  assertions to recover headroom: excluded by ADR 0016 and this issue's scope.
- A frontend unit-test builder library: there are no repeated builders to share.

## Reproduction and validation

```sh
python3 docs/reviews/fixtures/maintainability-test-inventory.py \
  --base 13d15eabd68ed2d3b6177512c9c8fe56c528bbf0 \
  --head a514e188951178b54ca9b43b901c92e725a215e7 \
  --output /tmp/rekenraam-284-tests
```

Clone reports use the slice 2 tool pins: `dupl -t {150,100,75} -plumbing -files`
over the head's `backend/**/*_test.go`, and jscpd 4.0.5 with
`--format typescript --min-lines 5 --min-tokens 100 --mode mild` over its
`e2e/**/*.ts` (excluding `node_modules`) and `frontend/src/**/*.test.ts`.
Race timing ran `go test -race -count=1 -timeout=10m -json ./internal/app` with
`scripts/native-sqlite-env.sh`, sequentially on one host: head run 1 from the
clean checkout via `./scripts/go-backend.sh`, the other three from extracted
trees. Leaf work counts tests and subtests without subtests of their own,
because parent elapsed includes parallel-slot queueing. The fixture probe and
the strict-helper experiment edited only disposable extracted copies.

**Passed:** a second inventory run produced byte-identical outputs for all eight
driver files. The baseline function count reproduces slice 3's 3,336. The
assertion index's financial rows were hand-read (28 functions), as were all 86
non-scoped exact counts. The strict-helper experiment passed with a reaching
negative control (74 failures). All four race runs passed. Also passed: 126
relative links and anchors across the five touched Markdown documents, `gofmt`
on both Go tools, Python syntax, JSON parsing of every retained JSON artifact,
`./scripts/check-post-merge.sh HEAD` and `git diff --check`. Not run: the full
backend gate, frontend checks or browser suites, because runtime and tests are
unchanged. No new CI result is claimed.

## Remaining acceptance

#284 remains open. With this slice, every measurement the issue names has a
bounded pass. Still required: the final ranked table with investment/reporting
do-now/before-R13/later/reject dispositions, the consolidated rejected list,
and owner review; then accepted candidates get their own bounded issues. R13
specification may proceed. R13 implementation waits for those dispositions and
its own calculation contract, which should absorb the cross-family register
above.
