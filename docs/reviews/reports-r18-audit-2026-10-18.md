# Reports, R18 and R13 input-readiness audit — 2026-10-18

**Status:** [#275](https://github.com/sergeyfarin/rekenraam/issues/275) executed;
**BLOCKED for closure by [#318](https://github.com/sergeyfarin/rekenraam/issues/318)**.
Its literal browser-export acceptance extends beyond the deliberately shipped
R18 export boundary. The other two acceptance items are met. No new financial
or security defect was confirmed. The small documentation finding
[#317](https://github.com/sergeyfarin/rekenraam/issues/317) is corrected here.
This is area 7 of [#260](https://github.com/sergeyfarin/rekenraam/issues/260),
not certification of the whole app or implementation of R13.

## Scope and baseline

Audited baseline `f5b943c30ff71e90e611d8c2b4399c14537d6a74`, initially clean.
The changes in this audit add focused consumer evidence and clarify existing
filter documentation; production calculations, schema and API are unchanged.
Reviewed R2 report/filter/FX readers and CSV consumers, R18 dated and alternative
projections, person pooling, gains FX, statements, income coverage, saved-run
capture/read/download and schema migrations 7/8/10. Relevant governing sources:
[reports](../plans/reports-plan.md), [investment reporting](../plans/investment-reporting-plan.md),
[income coverage](../plans/dividend-income-coverage-plan.md), ADRs 0012/0013/0017,
and the [shared audit brief](app-state-and-audit-plan-2026-10-17.md).

All books are synthetic and provider-free. Browser workers use the existing
`MARKET_DATA_BASE_URL` stub; no live price, credentials or provider sample is
required. The independent oracle is the unchanged
[#266 register](../../fixtures/reference-book.json), with its written `why`
derivations, and the smaller [report scenario](../../fixtures/report-scenario.json).
Existing focused tests below state separate hand-derived expectations. No
expected financial figure was changed to follow the implementation.

### Dependencies disposition

#266 is delivered and #281's chained security/FX conversion is fixed. The
#272/#273 operation evidence is available; #307/#309–#312 are fixed in this
baseline, with no unresolved gains defect identified by those audits. #313's
incomplete self-check outcome ships and #273 is closed. #272 remains open for
#277's bundle reconstruction; it does not block this read-side audit. No
all-green self-check or reconstruction acceptance is claimed here.

## Findings and acceptance disposition

| Finding | Severity and evidence | Disposition |
|---|---|---|
| Shared filter docs imply every report resolves descendants at each reporting date | P3, #317. Authenticated reparenting control: 1000 USD salary on June 5; non-posting group moved into selected subtree June 20. Net worth excludes it before June 20, while range-wide income/cashflow include the June 5 flow from the range-end scope | **Confirmed documentation defect, fixed.** Common report semantics and feature ledger now distinguish the two boundaries. No calculation changed |
| Original #275 export acceptance includes figures with no shipped browser download | P2 acceptance gap, #318. Reference FIFO/LIFO/average comparisons work in API/browser; saving while average cost is selected downloads `operational_v1`, as the governing saved-run plan requires. Person-pooled and gains-FX snapshots are also deliberately outside that export contract | **Accepted/deferred product limit; original audit acceptance unmet.** Core report CSVs and operational gains JSON pass. #318 owns explicit disposition of the acceptance/export boundary. #275 stays open; this audit does not authorize a new persisted projection format |
| Schema-2 read evidence was missing from the existing schema-upgrade test | Evidence gap, no runtime defect. The original test made only schema 1 and schema 3 | **Verified after extending evidence.** It now persists a genuine schema-2-shaped statement on migration 8, migrates forward and checks unchanged reads/downloads/export after reversal |

First checkbox: **not met in full**, specifically live-only comparison/export
parity. Second checkbox: saved bytes and schema 1/2/3 compatibility shown.
Third checkbox: the R13 inventory below is ready for #119 and is linked there.
#318 must explicitly accept the existing live-only boundary or establish an
approved bounded export contract before the first checkbox can be completed.
The report × surface matrix below preserves the original acceptance gap.

## Independent figures and actual consumer evidence

All rows below passed the named consumers. Unless a limitation is stated, the
disposition is **verified** and no defect severity applies.

| Scenario / independent expected result | Consumer / actual result | Limit |
|---|---|---|
| Reference spending: groceries 175 EUR, dining 55 EUR, travel 45 EUR, borrow costs 8 USD; income salary 5000 EUR and dividend 15 USD | `TestReferenceBookProducesItsRegister`; new `reports.spec.ts` reference spending/net-worth journey compares every figure to both API and downloaded CSV; exact match | Account-scoped browser reads isolate the cohort from other worker scenarios |
| Reference cashflow EUR: in 5000, out 130, operating 4870, transfer in 3550/out 3120/net 430, movement 5300. USD: in 15, out 8, operating 7, transfer in 5040/out 2255/net 2785, movement 2792 | Same API register and existing reference cashflow browser CSV; exact gross/net match. `TestCashflow*` also verifies per-bucket balance identities, selected-boundary changes, mixed scales and system-account refusal | Default liquid scope and explicit asset/liability scope differ; system accounts are counterparts, never selected cash |
| Month-end net worth: Jan 31 EUR 2840 + USD 2750 => 5315 EUR; February conversion absent (stale FX / missing security price); March 31 EUR 5275 + USD 2792 + REF 13.5 − SHRT 6 => 8247.45 EUR | API register plus new browser CSV checks every raw commodity total and both converted totals; February has no partial converted row | Browser explicitly selects reference accounts; month ends have no clearing residual. Security CSV labels carry the run suffix |
| Operational realized 350 USD; FIFO 380; LIFO 255; average 300. Known unrealized: operational 50/105/60; FIFO 50/75/60; LIFO 125/125/60; average 100/105/60. Gift gain absent in every method | API register verifies all rows and book-wide totals. New `investments-reference-audit.spec.ts` compares each reference row and exact cohort total, checks visible comparison position figures, then downloads operational JSON and compares bytes with the server attachment | Browser workers may hold unrelated positions; cohort sums use test-only exact integers. Only the fresh Go book asserts these as whole-book totals. Alternative projections have no download |
| Single-account purchase dated May 1, posted May 3: spending/cashflow 25 on May 3, none May 1. Two-account split: expense 50 on May 10, cash −150 May 11/+100 May 12 through clearing; whole-book worth −75 from May 10 onward | New `TestReportAuditPostingDateAxesAndClearingScope` through authenticated public endpoints; every date/amount matches | Cashflow classifies dated clearing counterparts as financing, not operating expense. Spending account filters match the category's own journal entry, not another dated leg |
| Reparented non-posting group: net worth empty before June 20, 1000 after; June-wide income and June 5 cashflow remain 1000 under end-date scope | Extended `TestNetWorthSeriesResolvesSubtreeAgainstEachBucketsAccountVersions`; actual match in all three reports | Historical balances and fixed flow scopes intentionally answer different questions; #317 corrects the claim |
| Refund 20 against expense 120 leaves 100; transfers/voids/deleted/superseded versions cannot inflate expense/income; account filter correlates to the same entry | `TestSpendingReport*`, `TestSpendingAccountFilterMatchesPerJournalEntry`, `TestEditingVoidingAndDeletingLeaveOneVersionInEveryReport`, report scenario and browser filter/drill-down cases | Multi-account/unattributed drill-downs that transactions cannot represent remain unlinked |
| Dated position includes acquisitions/disposals through chosen date; future trades stay out of default owner-today valuation; closed longs and shorts reconstruct at the date | `TestDatedValuation*`, `TestInvestmentGainsDefaultTodayKeepsFutureSaleOutOfValuation`, `investments-dated-valuation.spec.ts`; quantities, market/basis/gain and stale/missing disclosures pass | This replays current effective history, not past knowledge-time history |
| Person pool: 4 units at 40 + 4 at 120 => 20/unit. Selling 2 consumes 40; remaining 2/4 units carry 40/80, unrealized 20/40 at price 30 | `TestPersonPooledGains*`; six-locale mobile `investments-person-pooled-gains.spec.ts`; exact conservation, neutral transfers, separate currencies/sides, unknown propagation and structural refusal pass | A hypothetical basis comparison is not a return or a tax election |
| FX example: 2 units cost 40, sell 1 for 50, remaining worth 60. Acquisition/sale/close FX 0.50/0.80/1.20 => realized/unrealized 36/48 under valuation-date-all; 30/62 under historic-basis | `TestGainsFX*`, `TestInvestmentGainsReportingCurrencyAPI`, `investments-gains-fx.spec.ts`; source amounts and dated rate evidence preserved. Missing/out-of-window components withhold complete totals; same-currency sub-cent amounts remain exact | Component half-up rounding is named; gains-FX is live-only |
| Receipt 10 on basis 7 => reduction 7/excess 3. Limited statement total includes the 7 unrealized change once, excludes excess 3; unknown reduction/excess absent | `TestGainsStatementCapitalReturn*`, `TestGainsFXStatementCapitalReturnDatesCoverageAndUnknown`, API disclosure and six-locale mobile capital-return statements pass | Effective-date basis movement differs from payment-date disclosure. The limited result is not a complete economic return |
| Named payer-less dividend retains known gross/tax; legacy imported cash 85 does not establish gross or zero tax and is excluded from statement income until owner repair | `TestDividendCoverage*`, `TestIncomeCoverageAPIDisclosesCashOnlyImportAndFollowsRepair`, `TestGainsStatementDisclosesUnclassifiedImportedDividendCash`, `income-coverage.spec.ts`; unassigned, source-identified, known-zero, repair and frozen disclosures pass | Income coverage is book-wide/lifetime with instrument summary filtering, not a portfolio-period return input API |

### Saved bytes, old schemas and interleavings

`TestCapitalReturnStatementSchemaUpgradePreservesLegacyBytesAndNewFrozenDisclosure`
now constructs schema 1 without capital-return or unclassified-dividend fields,
and schema 2 with capital-return fields but no unclassified-dividend fields.
They are inserted on migrations 7 and 8 respectively. Forward migration leaves
all bytes/hashes unchanged; authenticated downloads return both without field
backfill. A new schema-3 save contains the current disclosure. Reversing the
receipt leaves all three records unchanged; out-of-period bundle export retains
all three exact output documents and schema numbers. These are synthetic prior
wire formats, not a supplied historical production database.

`TestInvestmentGainsRunReproducesBeforeSaleReversal` preserves saved gains and
statement rows through reversal, bundle export and native backup/restore. Its
normal SQL update/delete refusals and deliberate corruption control distinguish
immutability from document-integrity failure. `TestInvestmentGainsRunPreservesWithdrawnPrice`
retains the saved quote after live withdrawal. `TestGainsRunSnapshotIgnoresWritesBeforePersistence`
and `TestGainsRunSnapshotFailureLeavesEveryFinancialRowUnchanged` exercise capture
versus concurrent mutation and failed persistence. Existing saved-run browser
journeys reopen/download documents after a sale reversal in six locales.
The new reference browser control compares the downloaded attachment's bytes
with the server attachment, in addition to checking its financial fields.

Live consistency is independently exercised by `TestLiveGains*` and
`TestLiveStatement*` in application/API tests: correct a buy between operational
and FX stages, withdraw quote/FX observations between stages, and reverse
income between statement boundaries and composition. One response retains its
captured state and rate evidence; the next sees the corrected history. Durable
financial/audit/run rows remain unchanged beyond the deliberately injected
writes. This proves the R18 response boundary, not a multi-request historic
reconstruction guarantee or an audit of every R2 concurrent read interleaving.

| Report surface | Live API/browser | Shipped browser export | #275 disposition |
|---|---|---|---|
| R2 spending, cashflow, net worth | Register and focused consumers pass | CSV parity passes | Verified |
| Operational R18 gains | Register and dated consumers pass | Freshly computed saved schema-1 JSON; reference cohort and bytes pass | Verified |
| Operational period statement | Components, coverage and interleavings pass | Schema-3 saved JSON; old schema-1/2 API reads/export pass | Verified within current contract |
| Alternative / person-pooled comparison | API exact expectations and visible browser comparison pass | None; saving explicitly computes operational output | Accepted product limit; literal export acceptance remains unmet (#318) |
| Gains reporting FX | Both named methods and incomplete figures pass | None; original-currency operational save only | Accepted product limit; same acceptance disposition needed |

## R13 readiness hand-off to #119

**Specification can proceed; implementation is not ready without its own
calculation contract.** R18 supplies useful facts and consistent projections,
not TWR/IRR equations. The table answers every question in the audit brief.
“Missing” here describes R13 work, not a defect in an already delivered feature.

| Question / input | Current state and source pointer | Required #119 decision / limitation |
|---|---|---|
| Portfolio, security and account boundary | **Partial.** R2 has account/descendant filters (`app/reports.go`, `app/cashflow.go`); R18 gains/statements are book-wide position projections (`app/investment_gains_read.go`). Instrument/account/cost-currency/side identities exist | Define selected portfolio, included accounts, instruments, cash and short liabilities together. R2 default liquid cash is not automatically portfolio cash |
| External versus internal flows | **Partial.** Dated journals/posting axes plus effective investment operations and transfer links exist (`db/reports.go`, `db/investment_transfer_internal.go`, effective operation/link views) | Define externality relative to the selected boundary. A transfer internal to the book can cross a portfolio boundary; income/expense versus financing in R2 is not an investor contribution/withdrawal classifier |
| Fees, taxes, dividends, payment in lieu and corporate cash | **Facts exist; return classification partial.** Named operation/component facts and dates are in investment operations, dividend facts, capital-return reads and merger disposal revisions; `app/investment_gains_statement.go` supplies gross named dividend income | Decide net/gross policy, fee/tax handling, reinvestment and payment-in-lieu effects. Limited R18 total excludes excess and does not supply a complete return. Preserve payable versus effective dates |
| Valuation at a selected date | **Exists.** `db/investment_dated_valuation.go`, `app/investment_gains_read.go`; side-aware current-effective replay plus compatible raw observations | Reuse quantities/market liabilities and explicit unknown/unpriced results. Unknown basis need not make a known market value unknown |
| Daily valuation frequency and flow timing | **Partial.** A dated replay and statement's two boundaries share one captured image; separate daily requests do not share a retained image | Choose daily/subperiod convention, opening/closing and same-day flow timing, market calendar and sampling. A loop over independent live requests is not one frozen period |
| TWR / TTWROR equations | **Missing.** No accepted R13 calculator/profile | Name the equation, subperiod chaining, flow convention and independent equivalence cases; basis-method gains are not time-weighted returns |
| MWR / IRR equations and numerical outcomes | **Missing.** Exact coefficient/rational helpers exist (`internal/exact`), not an accepted dated-flow solver | Specify equation, solver, root selection, bounds, tolerance/convergence and no-root/multiple-root outcomes; never silently choose a root |
| Zero/negative capital, annualization/day count | **Missing R13 policy.** Shorts can create negative liabilities/market amounts; current projections retain signs | Define undefined versus supported cases, short intervals, leap years, annualization and day count before charts |
| Precision and rounding | **Exists for R18; partial for R13.** Exact string coefficients, scaled arithmetic and half-up valuation; gains FX uses `component_half_up_currency_scale` (`app/investment_gains_fx.go`) | Specify return arithmetic and final presentation precision separately; do not sum rounded screen amounts or introduce floating monetary storage |
| Allocation taxonomy and denominator | **Partial facts; missing analytics contract.** Instrument types/metadata, account hierarchy and signed dated values exist; no accepted portfolio allocation grouping/weight model | Choose categories, denominator, cash, short liabilities, unknown values, weights and coverage. No implied netting of long/short or unlike currencies |
| Targets and rebalancing | **Missing / unaccepted scope.** No R13 target/rebalance delivery | Explicitly include or defer these; no trade execution implied |
| Price/FX coverage | **Exists, consumer policy differs.** `db/observation_selection.go`, `app/valuation.go`, dated valuation and gains FX expose rate/observation dates, stale uses and unavailable reasons | Select R13 age/calendar/FX conventions. R2 withholds out-of-window conversion; R18 native valuation can retain a stale marked price. Neither policy should be silently substituted for the other |
| Income attribution coverage | **Exists but partial for a period portfolio.** `db/investment_dividend_coverage.go`, `app/investment_gains_statement.go`, `api/investment_dividend_coverage.go` | Keep known cash separate from unknown gross/tax and payer. Define effects on a selected portfolio-period result; existing coverage summary is not itself that input model |
| Benchmark sources and calendars | **Missing benchmark-series contract.** Manual price observations exist; no accepted benchmark identity, total-return semantics, calendar or comparison profile | Manual input must be valid; define benchmark meaning, split/dividend adjustment and calendar. Live providers/broker comparison are optional integrations, not prerequisites |
| Corrections and period reproducibility | **Exists for saved original-currency operational output; partial for returns.** Live readers use current effective history; `app/investment_gains_runs.go` captures/fingerprints a consistent database image and stores output | Choose frozen R13 output/input policy. A timestamp ceiling alone does not recreate past corrections/retirements. Alternative/FX saves are excluded today; #318 is an audit-boundary follow-up, not a returns-formula blocker |
| Independent expected returns / equivalence | **Partial.** #266 establishes exact flows, valuations, gains and incomplete cases; this audit validates their consumers | Add hand-derived TWR/MWR/allocation cases under the chosen contract, including neutral internal movements, cash/short scope and failure states, before chart implementation |

No provider availability or adapter evidence blocks the specification. #284
still owns investment/reporting refactoring dispositions before R13
implementation, as the roadmap requires. This audit makes no allocation,
benchmark, tax or numerical-policy decision on #119's behalf.

## Maintainability notes

Recorded for #284 only; no refactor is performed or proposed as a new issue.

- `api/investment_gains_runs.go:formatGainsRun` and the live gains/statement
  handlers separately attach profile, capital returns and currency metadata.
  Statement schema changes also touch migrations, `db.ValidGainsRunDocument`,
  OpenAPI and frontend optional-field rendering. The missed schema-2 fixture
  above shows why a current-shape-only check is insufficient. Any refactor
  must retain old bytes and the intentional operational-only save boundary.
- Filter expansion has shared owners in `app/reports.go`, but net-worth series
  and flow consumers apply them at different dates. #317 demonstrates actual
  documentation drift across those consumers. Preserve the semantic difference
  rather than merging all date scopes during cleanup.
- `app/investment_gains_statement.go`, `app/investment_gains_fx.go` and
  `api/investment_gains_statement.go` each carry complete-only coverage and
  capital-return component mappings. Their native/FX amount types and date
  policies differ intentionally. A new disclosure changes several mappings;
  existing #238/#246 tests and captured-state interleavings are the behavior
  boundary, not just field presence.

## Validation and retained limits

- Initial focused report/R18 application, API and database selection passed
  (10.612 / 14.386 / 0.767 seconds respectively).
- Final focused selection with `-race -count=1 -timeout=20m -json`: **156
  top-level tests passed** (85 application, 70 API, 1 database), no failures;
  package elapsed times 15.735 / 20.078 / 2.559 seconds. Selection covers
  `TestReport*`, `Spending*`, `Cashflow*`, `NetWorth*`, `ReferenceBook*`,
  `Gains*`, `InvestmentGains*`, live-capture, dated/alternative/person-pool,
  observation-selection, income/dividend-coverage and classification controls.
  The added date, historical-subtree and schema-2 consumers also passed alone.
- Fresh integrated `pnpm build` and the existing focused browser selection:
  **60 passed**, covering reports, dated/alternative/person-pooled gains,
  gains FX/statements/runs, capital-return statements and income coverage.
  After adding the two register controls, `E2E_SKIP_BUILD=1 E2E_WORKERS=2
  pnpm --dir e2e exec playwright test reports.spec.ts
  investments-reference-audit.spec.ts`: **13 passed**. This reuses the same
  production binary: only tests/docs changed. There are **62 distinct browser
  journeys** across those selections, not 73 distinct cases.
- `pnpm --dir frontend exec vitest run src/lib/reports
  src/lib/investments/income-coverage.test.ts`: **11 files, 141 tests passed**.
- `GOCACHE=/tmp/rekenraam-go-build-cache ./scripts/go-backend.sh vet
  ./internal/app ./internal/api ./internal/db`, formatting, relative links and
  `git diff --check` passed. Structural check:
  `SKIP_OPENAPI=1 ./scripts/check-post-merge.sh HEAD`; OpenAPI regeneration
  intentionally skipped because no API/generated file changed. The checker
  reports the existing identical Chromium-install step in `ci.yml` and
  `release-preflight.yml`; neither workflow changes in this audit. No migration
  collision, duplicate document paragraph, work-item collision or lost-test
  finding was reported.

The first browser attempt was refused a local listener by the sandbox; its
permitted rerun passed. Final Go validation used a writable `/tmp` cache after
the default cache produced a read-only-filesystem setup failure. Neither setup
failure is financial evidence or a product finding.

No full backend/race/browser shared gate is run for this area; #280 owns those
at the candidate SHA. No production deployment, owner household dry run, live provider
wire verification, push or remote CI result is claimed. #277 retains complete
bundle reconstruction/recovery; #278 retains the wider route/accessibility audit.
The additional #318 audit is the only new closure dependency identified here.
