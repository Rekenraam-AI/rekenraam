# FX coverage progress and publication gaps — #315, 2026-10-18

Baseline: `3e75a69c`, after #314's atomic continuation fix. The original
`TestFXCoverageWorkerHolidayDoesNotStarveLaterPublicationDates` again failed
on that baseline: one observation rather than three in the first direction,
with a continuation still active after 48 chunks. This change promotes that
probe into the normal app suite and fixes the bounded acceptance of
[#315](https://github.com/sergeyfarin/rekenraam/issues/315).

The governing behavior is in the
[pricing contract](../plans/pricing-management-plan.md#historical-fx-progress-315).
Work payloads now retain a stable pair/date cursor and the sweep's owner-local
end date. Failed dates get independent retry items alongside the continuation
in the parent's completion transaction. All download attempts consume the
budget, and their initial failure counts toward the eight-attempt cap. An
earlier observation keeps its real date; it does not establish a holiday or
complete coverage. Invalid/future returned valuation dates are refused, with
owner-local today used for current refreshes.

Migration `0018_fx_coverage_progress.sql` changes the posting-outbox trigger:
only an unattempted full pending scan can coalesce fresh demand. A suffix,
running scan or date retry cannot swallow a newly posted demand. Existing
work rows, observations, journals and lots are not rewritten; no new tables,
API fields, generated clients or frontend copy are needed. Legacy queue
payloads remain readable. Network I/O remains outside writer transactions.

| Acceptance / independent expectation | Observed test evidence |
|---|---|
| One-date budget: June 9/11/12 in both directions, no fabricated June 10 rate | Promoted original regression passes with exactly six observations and no active work. |
| Longer history, multiple gaps, four directions, repeated reopen | `TestFXCoverageCursorSurvivesReopenAcrossManyGapsAndPairs`: ten weekdays × four pairs = 40 attempts; four gap dates per direction give exactly 24 real observations and 16 earlier-date evidence items. Every run attempts at most one date. SQLite closes/reopens between worker ticks. |
| Failed dates cannot starve other dates/pairs or become holidays | `TestFXCoverageFailedDatesDoNotStarveHistoryAndRetainBoundedRetry`: transient HTTP, unsupported HTTP, malformed JSON, future and missing dates each leave one explicit failed slot and 15 other observations. No failed slot is marked skipped; pricing workspace shows the affected pair missing and the unaffected pair usable. |
| Bounded retries and operator recovery | Same test: exactly eight calls to each failed slot, including its initial scan attempt; one failed item names pair/date. Manual retry after repair adds exactly the missing sixteenth observation, without duplicating the other fifteen. |
| Late publication, new/backdated demand while progress exists | `TestFXCoverageFreshDemandRevisitsLatePublicationAndEarlierDates` posts through the real transaction writer/outbox with a currency-scoped suffix queued. Fresh demand is queued independently, the late quote arrives once per direction, and an earlier posting extends coverage before the prior start. |
| A finished sweep does not permanently exclude a gap | `TestFXCoverageCompletedSweepDoesNotHideLatePublicationFromManualRefresh`: six initial observations become eight through public `RunRefresh` after a late fixing appears. Another manual refresh stays at eight. |
| Completion/retry/continuation atomicity and ownership | `TestBackgroundWorkSuccessorsCommitTogetherAndPreserveRetryBudget`: failure on the second insert rolls back the parent and first insert; stale owner creates no successors; an existing active retry keeps its attempts, lease and availability; a new retry starts at claim attempt 2. Existing single-successor and real interrupted-worker tests pass. |
| Upgrade and old queue compatibility | `TestFXCoverageProgressMigrationPreservesQueuedWork` round-trips 17→18→17→18, restores the exact previous trigger and preserves the queued record. Full schema/fresh-upgrade migration tests pass. |
| Invalid cursor / local date boundary | Invalid pair IDs, dates or simultaneous scan/retry cursors fail at attempt 1 without starting an ingest run. `TestFXRefreshValuationCeilingUsesOwnerLocalToday` accepts June 13 in Amsterdam at June 12 22:30 UTC; the future-date failure case refuses a date beyond its requested historical ceiling. |

Validation: **157 distinct focused top-level backend tests pass with race
instrumentation**, including pricing/FX/observation consumers, queues, API,
marketdata, migrations and test-parallelism checks. **17 rebuilt browser
journeys pass** for pricing management, reporting/holding coverage and
six-locale mobile currency-provider accessibility. The integrated build also
checks catalog parity, generates API types and embeds/tests the static app.
Vet, formatting, local links and diff checks pass. Full shared audit gates
remain #280's responsibility; no live-provider, push or remote-CI claim is made.

```sh
./scripts/go-backend.sh test -race -p 2 -timeout 5m \
  ./internal/app ./internal/db ./internal/api ./internal/marketdata ./migrations \
  -run 'Pricing|FX|Price|ObservationSelection|BackgroundWork|RunDueFX|RunScheduledRefresh|RetryBackgroundWork|ProcessFX|RefreshFX|SourceAssignment|LocalDaily|QuoteWorker|Migrate|Migration|IntegrationSuitesRunTestsInParallel'
pnpm --dir e2e exec playwright test --project app --workers 2 \
  playwright/pricing-management.spec.ts playwright/observation-selection.spec.ts \
  playwright/currency-provider-accessibility.spec.ts
./scripts/go-backend.sh vet ./...
```

Retained boundaries: a sweep is not a permanent coverage watermark. New
manual/scheduled demand can revisit an earlier-date response; no fabricated
rate or forever-cached holiday is introduced. Failed provider availability and
unsupported pairs can still leave honest missing/stale report coverage after
the retry cap; the existing manual-price and explicit retry workflows remain
available. Current observation correction/void and selection rules are unchanged.
