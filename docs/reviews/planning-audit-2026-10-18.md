# Budgets, recurring, forecasts and learned spending audit — 2026-10-18

**Status:** [#276](https://github.com/sergeyfarin/rekenraam/issues/276), area 8
of [#260](https://github.com/sergeyfarin/rekenraam/issues/260), complete. Every
delivery bullet has a disposition. Two P2 budget defects were confirmed through
the real budget API and fixed with regressions that fail on the unfixed code:
[#319](https://github.com/sergeyfarin/rekenraam/issues/319) (posting-dated
splits dropped out of actuals) and
[#320](https://github.com/sergeyfarin/rekenraam/issues/320) (archiving a
category removed it from earlier months). No recurring, forecast or
learned-spending defect was confirmed. Recorded, assumed and estimated values
stayed separate in every scenario. This is bounded evidence, not certification
of every interleaving or release readiness.

## Scope and baseline

- Baseline: `2082c48c2c7d695aac73199e231038ad3a09d634`, clean working tree.
- Governing sources: [budgets](../plans/budgets-plan.md),
  [recurring](../plans/recurring-transactions-plan.md),
  [projected balances](../plans/projected-balances-plan.md),
  [forecast learning](../plans/forecast-learning-plan.md) and the
  [shared audit brief](app-state-and-audit-plan-2026-10-17.md).
- Books are synthetic and provider-free; no provider is called. Budget and
  forecast oracles are figures worked out by hand in the test comments, never
  read back from the code under test. Fixture A of the projected-balances plan
  runs through the real HTTP API and the real recurring producer. Its dates are
  shifted to the API test clock: A = 2026-09-07, H = 5.
- Named tests were reused first. New audit consumers are in
  `backend/internal/api/planning_audit_test.go` and
  `backend/internal/app/planning_audit_test.go`. The #319/#320 regressions are in
  `backend/internal/api/budgets_test.go`.

## Findings

| Finding | Severity | Expected versus actual | Disposition |
|---|---|---|---|
| [#319](https://github.com/sergeyfarin/rekenraam/issues/319): posting-dated splits drop out of budget actuals | P2 | A 50.00 grocery split dated May 10, funded by on-budget checking (posted May 11) and card (posted May 12), is bridged through `transfer_clearing` (#187). The spending report counts 50.00. Before the fix, the May budget actual was **0**: the category entry's only other leg is the system clearing account. | **Confirmed defect, fixed.** A bridged category entry is funded by the on-budget legs of its sibling clearing entries in the same transaction version, each classified on its own date. The category date still selects the month. `TestBudgetCountsPostingDatedSplitsFundedThroughClearing`: May = 57.00 (50.00 + a 7.00 split whose funding posts in June); a 9.00 split funded only off-budget stays out; June is empty. It fails on the baseline. |
| [#320](https://github.com/sergeyfarin/rekenraam/issues/320): archiving a category rewrites past months | P2 | February expenses 10.00 + 9.99 = 19.99. After the 9.99 category was archived, February showed **10.00** and no row. An archived income category's 300.00 and a 15.00 target also disappeared. | **Confirmed defect, fixed.** An archived category is returned with `status: archived` in every month where it has a target or actual, counts toward totals with its income/expense sign, and shows a read-only target on the screen (six locales). It stays hidden in months with no activity. `TestBudgetKeepsArchivedCategoryHistoryInItsMonths` fails on the baseline logic: 1000 vs 1999, 0 vs 1500 target, 0 vs 30000 income. |

Neither defect changed ledger data. Both understated budget actuals and
remaining amounts. Both are fixed in this change, so #276 has no open finding
and no closure blocker.

## Scope and evidence register

"Verified" means the named consumer passed in this run. Amounts are written in
display units; the wire carries scale-2 coefficients.

### Budgets

| Scenario | Independent expected result | Consumer and actual result | Disposition and limit |
|---|---|---|---|
| Monthly figures, signs, scale | Feb 2028 USD groceries 12.34 − 2.00 refund + 3.00 (split) + 10.00 (two funding legs, counted once) = 23.34; target 20 → −3.34. Household 4.00 + 0.50 = 4.50 → 0.50. Salary 2,500.00 − 100.00 = 2,400.00, target 3,000.00 → 600.00. Expense totals 25.00 / 27.84 / −2.84 | `TestPlanningAuditBudgetMonthHandDerivedFigures` (real HTTP) passes with every figure; `TestBudgetMonthUsesExactPostedActualsAndAccountTreatment` (leap day 12.34 → remaining 7.66) | **Verified.** |
| Month boundaries, no rollover | Jan 31 and Mar 1 (1.11 each) are outside February; January 1.11 against 100.00 → 98.89; March has no copied target | Same consumer; `rollover_policy=none` | **Verified.** |
| Currency isolation | EUR groceries 7.00 stay a separate amount and total; nothing converted | Same consumer | **Verified.** |
| On-budget treatment by effective-dated version | Wallet switched to off-budget from Feb 15: 0.50 on the 14th counts, 0.60 on the 15th does not. Brokerage cash defaults off-budget (9.99 excluded). A transfer without a category counts zero | Same consumer | **Verified.** |
| Actuals never include assumptions | A voided 44.44 and a 33.33 recurring draft contribute zero; posting the draft makes it an ordinary actual (23.34 → 56.67) | Same consumer, through the real recurring producer and post route | **Verified.** Forecast and learned estimates have no path into the budget reader, which reads posted `posting_versions` only. |
| Target lifecycle and audit | Zero removes the target and leaves the actual. Audit operations in order: `budget.account_treatment.set`, four `budget.target.set`, `budget.target.remove` | Same consumer | **Verified.** The plan named the treatment operation `budget.account-treatment.set`; the plan is corrected to match the shipped name. |
| Concurrent target upserts | Eight parallel PUTs leave one target fact; audits equal committed writes; the surviving value links to the last audit | `TestPlanningAuditConcurrentBudgetTargetUpsertsLeaveOneFact` passes | **Verified** within one process's writer pool. |
| Owner-local current month | 2026-10-31 23:30 UTC in Amsterdam is November; 2026-11-01 06:30 UTC in Los Angeles is October | `TestPlanningAuditOwnerLocalDateAgreesAcrossDSTAndMonthEnds`; `TestBudgetMonthDefaultsToCurrentOwnerLocalMonth` | **Verified.** |
| Posting-dated splits | See #319 | Regression above | **Confirmed defect, fixed.** |
| Archived categories | See #320 | Regression above | **Confirmed defect, fixed.** |

### Recurring

| Scenario | Independent expected result | Consumer and actual result | Disposition and limit |
|---|---|---|---|
| Generate, review, post, discard, skip, retry | Drafts only; discard leaves an audited skipped identity that never regenerates; skip ahead stops generation; a blocked date retries only on request and rejects stale attempts/revisions | `TestGenerationIsIdempotentAcrossTicks`, `TestDiscardingGeneratedDraftPreservesOccurrenceAndCannotRegenerate`, `TestDiscardEditedGeneratedDraftPreservesAuditAndSkippedIdentity`, `TestSkippingAFutureOccurrenceThroughReviewStopsItGenerating`, `TestBlockedOccurrenceRetriesOnlyWhenAskedTo`, `TestRecurringRetryRejectsStaleAttemptsAndTemplateRevisions`, `TestRecurringRetryRollsBackDraftAndAuditOnOccurrenceFailure`; seven `recurring.spec.ts` journeys | **Verified.** |
| Overdue handling | An unposted fee dated Sep 6 stays dated Sep 6 and is assumed on Sep 8 (F); exactly one carried-forward event | Fixture A (below); `TestRecurringTemplateNextDueIncludesUnmaterializedOverdueDates`; `TestForecastCarriesOverdueAndTodayToTomorrow` | **Verified.** |
| DST and owner time zone | Amsterdam 22:30 UTC on Oct 24 = Oct 25; 22:30 UTC on Oct 25 (the 25-hour day) = Oct 25; 23:30 UTC = Oct 26. Auckland spring-forward day: 14:30 UTC on Sep 26 = Sep 27 | `TestPlanningAuditOwnerLocalDateAgreesAcrossDSTAndMonthEnds`: recurring "today", forecast A and the budget month agree in all seven cases; `TestDailyAcrossADaylightSavingBoundaryProducesEveryDate`, `TestRecurringSchedulerUsesOwnerLocalDateAndUTCAttribution` | **Verified.** Date-only enumeration has no wall-clock arithmetic. |
| Generated identity idempotence | `UNIQUE (template_id, occurrence_date)`: double generation and racing retries converge on one row/draft | `TestRecurringOccurrenceIsUniquePerTemplateAndDate`, `TestConcurrentGeneratorsProduceOneOccurrence`, `TestConcurrentRecurringGeneratorsWithIndependentDatabasePools`, `TestConcurrentRecurringRetriesAcrossPoolsCreateOneDraft`, `TestRecurringSkipAndGenerationRacePreservesOneAuditedIdentity`; Fixture A: four concurrent re-runs generate 0, two generated rows remain, forecast body identical | **Verified.** Double generation and retry are impossible by identity, not by timing. |
| Partial bulk failure | Posts that complete stay posted; the confirmation keeps only unposted rows; a changed checkpoint impact or draft version stops the loop | Browser `bulk review preserves completed posts when a later draft fails` and `changed reconciliation impact stops posting and asks for a new review`; review of `recurring-screen.svelte` `confirm()` (sequential, version- and impact-bound) | **Verified.** There is no server bulk endpoint; recovery is per item by design. |
| Stale event basis | A financial mutation changes `basis_token`; event pages from an old basis return 409 and are never mixed | `TestForecastBasisChangesAfterFinancialMutation`, `TestForecastCursorRejectsChangedRecipeAndMalformedPayload`; browser `forecast refreshes after stale event basis without mixing pages` | **Verified.** |

### Forecasts

Fixture A, shifted (default cash scope: USD checking 1,000.00, savings 500.00,
EUR cash 200.00; card owes 300.00, outside the scope):

| Date | Posted | Draft | Template | Recorded | Projected |
|---|---:|---:|---:|---:|---:|
| Opening Sep 7 | — | — | — | 1,500.00 | 1,500.00 |
| Sep 8 | +200.00 | −25.00 (fee from Sep 6, carried) | 0 | 1,700.00 | 1,675.00 |
| Sep 9 | 0 (transfer nets) | 0 (rent moved off Sep 9) | 0 | 1,700.00 | 1,675.00 |
| Sep 10 | 0 | −125.00 (edited rent) | 0 | 1,700.00 | 1,550.00 |
| Sep 11 | −100.00 (card) | 0 (skipped −60) | 0 | 1,600.00 | 1,450.00 |
| Sep 12 | 0 | 0 | +1,000.00 (salary) | 1,600.00 | 2,450.00 |

| Scenario | Independent expected result | Consumer and actual result | Disposition and limit |
|---|---|---|---|
| Projected-balance contract | Table above; checking 1,175.00 / 875.00 / 750.00 / 650.00 / 1,650.00; savings 500.00 / 800.00 ×4; EUR 200.00, 200.00, 195.00 ×3; minimum 1,450.00 on Sep 11; no negative date; assumptions complete; source counts posted 4 / draft 2 / template 1 | `TestPlanningAuditForecastFixtureAThroughRealConsumers` (real HTTP and recurring producer) passes with every figure | **Verified.** |
| Scope and transfers | Selecting the card too: opening 1,200.00; the Sep 11 payment nets to 0; closing 1,200.00 + 200.00 − 25.00 − 125.00 + 1,000.00 = 2,250.00 | Same consumer | **Verified.** The first draft of this expectation (2,150.00) was an arithmetic slip in the audit and was corrected before the run was accepted; it was not adjusted to fit the output. |
| Recorded versus projected separation | Materializing salary unchanged: projected curve identical, source moves template → draft. Posting it: projected identical, recorded Sep 12 becomes 2,600.00 | Same consumer; `TestForecastServiceUsesRealRecurringGenerationAndPromotion`; browser `[acceptance] edited recurring draft replaces the template assumption` | **Verified.** |
| Deterministic output | The same request twice is byte-identical apart from `computed_at` | Same consumer (`assertForecastBodyEqual`) | **Verified.** |
| Constant FX | EUR→USD 1.10 on A: opening 1,500.00 + 220.00 = 1,720.00; projected 1,895.00, 1,895.00, 1,764.50, 1,664.50, 2,664.50 (−5.00 → −5.50 as its own component); recorded Sep 12 = 1,814.50. A 2.00 rate dated Sep 9 is ignored. Exact per-currency totals unchanged | Same consumer; `TestForecastConstantFX*`, `TestForecastMissingFXOmitsWholeConvertedSeries`; browser `forecast keeps exact currencies when constant FX is unavailable` | **Verified.** Test prices are recorded at wall-clock time, after the fixed forecast clock, so the test aligns `recorded_at` to the clock: the knowledge cutoff correctly ignored them otherwise. |
| Posting-dated draft legs | A recurring draft edited into a transfer whose savings leg posts a day later projects checking −100.00 on Sep 9 and savings +100.00 on Sep 10; no exclusion | `TestPlanningAuditForecastProjectsPostingDatedDraftLegs` | **Verified.** |
| Invalid, blocked and broken assumptions; budgets | Whole-transaction exclusion, diagnostics, `complete=false`; `FORECAST_TOO_LARGE` without partial totals | `TestForecastInvalidDraftExcludesWholeTransaction`, `TestForecastDiagnosticsDoNotHideExclusions`, `TestForecastInputAndOutputBudgets`, `TestForecastAPISizeErrorHasNoPartialTotals` | **Verified.** |
| No read side effects; one coherent basis | Reads write nothing; concurrent materialization yields template or draft, never both | `TestForecastReadsLeaveLedgerReportsAndExportsUnchanged`, `TestForecastConcurrentMaterializationKeepsOneBasis`, `TestForecastSnapshotSeesOneSideOfConcurrentPosting` | **Verified.** |

### Learned spending

| Scenario | Independent expected result | Consumer and actual result | Disposition and limit |
|---|---|---|---|
| Model on versus off | `spending_model=off` is byte-identical to the omitted option; enabling the model leaves series, totals and assumptions identical | `TestPlanningAuditLearnedSpendingOnOffThroughRealHistory`, on a book with 20 complete weeks of real history and a ready overlay; `TestForecastLearningAPIOffMatchesPreModelResponse`, `TestForecastLearningAPIKeepsCoreSeriesIdentical`; browser `estimated spending is opt-in, explained and reversible` | **Verified.** Model-off equivalence is shown on a ready overlay, not only on an empty book. |
| Estimate figures | 20 weeks × 70.00 every Wednesday: one −70.00 on Wednesday Sep 9; core 566.67 (2,000.00 − 1,400.00 − 33.33) flat; learned projected 496.67 from Sep 9 | Same consumer: 20 complete / 20 positive periods, deltas exactly as expected | **Verified.** |
| Recurring exclusions | A posted recurring-linked 33.33 Friday purchase is not training data; its archived template does not disable the group | Same consumer (20 positive periods; all weight on Wednesday); `TestForecastLearningUsesPostedHistoryOnly`, `TestForecastLearningOverlayRespectsRecurringOverlap` | **Verified.** |
| Quality fallback | A learned method is used only when validated better; otherwise a labelled baseline | `TestForecastLearningFallsBackUnlessValidatedBetter`, `TestForecastLearningNoFutureLeakage`, `TestForecastLearningVariationIsNotConfidence` | **Verified.** |
| Annual-history exclusions | 35 months → `insufficient_seasonal_history`; never a flat substitute | `TestForecastLearningColdStartAndSparseHistory`, `TestForecastLearningSeasonalValidation`; browser `a seasonal request explains the 36-month requirement` | **Verified.** |
| Resource bounds | Group/row limits, busy slot and deadline give an explicit unavailable overlay with no partial curve | `TestForecastLearningLimitsNeverReturnGroupPrefixes`, `TestForecastLearningSnapshotRejectsPostingPrefixes`, `TestForecastLearningOverlayRefusesWithoutGroupPrefix` | **Verified.** Heap and latency targets are not re-measured here; the M2 benchmark record stands. |
| Posting-dated splits | A bridged split has more than one funding account, so it is ineligible under the existing multi-funding rule and is counted as an exclusion | Code review of `classifyForecastLearningEntry` against plan §3 | **Verified (review).** Consistent with the plan; not a defect. |

## Acceptance

- [x] Each bullet has a disposition with hand-derived expected values for the
  budget and forecast figures.
- [x] Double generation and retry are shown to be impossible by identity.
- [x] Model-off equivalence is shown.

## Validation

```sh
./scripts/go-backend.sh test -race -p 2 -timeout 15m ./internal/app ./internal/db ./internal/api ./internal/recur \
  -run 'Budget|Recurring|Recurrence|Forecast|Generation|Generator|Occurrence|Skipping|Archiving|Creating|Concurrent|Downtime|Discard|BrowserApi|GeneratedDraft|Monthly|Yearly|Interval|Weekly|LastDay|MaxOccurrences|EndsOn|Window|Daily|Next|Validate|Occurrences|Unsaved|BlockedOccurrence|PlanningAudit|ReferenceBook|Schema'
pnpm --dir frontend run check
pnpm --dir e2e exec playwright test --project app --workers 2 \
  playwright/recurring.spec.ts playwright/forecast.spec.ts playwright/forecast-learning.spec.ts playwright/accessibility.spec.ts
```

Results on the final implementation: **241 top-level backend race tests** pass
across app, db, api and recur (including `TestReferenceBookProducesItsRegister`,
unchanged). Frontend check: 0 errors and 0 warnings; **688** frontend unit tests
pass. **29 browser journeys** pass against a fresh integrated build of the final
implementation: 7 recurring, 8 forecast, 4 learned-spending and 10
accessibility. The new 390 px journey `an archived budget category keeps its
actual and a read-only target` was added afterwards and passes, with the
existing budget journey, on that same build (30 distinct journeys). `go vet ./...`, gofmt and `git diff --check` are clean. Both
regressions failed on the baseline logic before the fixes. Full backend, full
browser and shared architecture gates were not run; #280 owns them.

Retained limits:

- A posting-dated (`posted_on`) transaction returned HTTP 500 on a test book
  whose setup never created system accounts. Completed setup always creates
  them, so this is not reachable through the product and is not filed.
- Concurrency evidence uses one process's writer pool for budgets; recurring
  also has independent-pool tests. The Go race detector does not instrument
  native SQLite.
- No live provider or real household data was used.

## Maintainability notes

- **Readers re-derive "who funded this entry".** `db/budgets.go` (on-budget
  EXISTS), `app/forecast_learning.go` (`classifyForecastLearningEntry`) and the
  R2 spending/cashflow readers each decide from journal-entry postings which
  account funded a category. #187 posting dates changed the entry shape, and
  only the budget reader missed it (#319). A new reader of category spending
  must know about `transfer_clearing` bridges independently.
- **Two "default cash" account-kind sets.** The budget on-budget default
  (`cash, checking, savings, credit_card, line_of_credit`) is repeated as an SQL
  `CASE` twice in `db/budgets.go` (treatment list and actuals). The forecast default scope
  (`cash, checking, savings, brokerage_cash`, `app/forecast_inputs.go`) differs
  on purpose, but the two lists change independently.
- **Category visibility rules are per reader.** The budget reader filtered on
  current `status='active'` and derived income/expense signs from that filtered
  list, so visibility and arithmetic were coupled (#320). The forecast learning
  category options apply their own active filter.
- **Owner-local date is computed three times.** `BudgetService.CurrentMonth`,
  `RecurringService.localToday` and `forecastDateBounds` each do
  `LoadLocation` + `now.In(...)`. They agree today; the new DST table test pins
  them together.
- **Forecast eligibility mirrors the transaction writer by design** (plan D5).
  Parity fixtures exist; a new posting rule needs both places.

These notes feed #284; no refactoring or refactor issues were created.
