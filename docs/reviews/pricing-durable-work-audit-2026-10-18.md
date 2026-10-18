# Prices, FX, provider boundaries and durable work audit — 2026-10-18

**Status:** [#274](https://github.com/sergeyfarin/rekenraam/issues/274), area 6
of [#260](https://github.com/sergeyfarin/rekenraam/issues/260), complete with
one P2 finding remaining. Every scope bullet has a disposition. The exercised
provider paths stage observations/suggestions without posting or replacing
owner decisions; partial states are exposed through real APIs and mobile UI.
This is bounded evidence, not certification of every possible interleaving or
live provider payload.

## Baseline and method

- Baseline: `10a03db733385faea10bf18660b743db35217741`; clean working tree.
- Fixes: `4485c5b5` (#314, atomic FX continuation) and `d9bb27a7` (#316,
  FX error redaction). Final implementation:
  `d9bb27a70b70350c86f0661fbc5b5a02fe4c43f9`, plus the recovery tests and
  evidence in this change. Local commits only; no push or remote CI claimed.
- Reused named service/repository/API tests first. Provider-free books and
  exact constants supply the oracles, independently of the calculation under
  test. New workers use built-in HTTP adapters through `DefaultRegistryAt`,
  the `MARKET_DATA_BASE_URL` seam. Browser fixtures set that override for every
  provider; no live-provider calls were used. Existing pure unit tests retain
  their deterministic fixtures.
- Added three passing app tests and one queue test. The intentionally failing
  #315 probe is retained as text under `fixtures/`, outside the normal suite.

## Findings

| Finding | Severity | Expected versus actual | Disposition |
|---|---|---|---|
| [#314](https://github.com/sergeyfarin/rekenraam/issues/314), continuation coalesces with itself | P2 | One-date execution budget, June 9–12 demand: expected eight observations, four per direction. Before fix: only June 9/10 USD→EUR, no active work. Enqueueing before completion coalesced an identical continuation with the running item. | **Confirmed defect, fixed** in `4485c5b5`. Complete and enqueue in one lease-fenced transaction; failed enqueue rolls back completion. All eight observations now arrive across service reconstruction. |
| [#315](https://github.com/sergeyfarin/rekenraam/issues/315), publication gaps starve later history | P2 | Synthetic June 10 request returns June 9's fixing. Expected June 9/11/12 in both directions without inventing a June 10 observation. After 12 ticks / 48 successful chunks: one USD→EUR observation and one active continuation. Every chunk spends its budget rechecking the gap. | **Confirmed defect, open.** Needs a durable gap/progress policy distinguishing absent publication from failed/unknown provider data. No journal/lot/observation amount was changed. Missing/stale report disclosure remains; complete historical fetching cannot be claimed. Capability ledger corrected. |
| [#316](https://github.com/sergeyfarin/rekenraam/issues/316), FX transport errors persist operator key | P2 | Expected no URL/key in ingest history. Real refresh/triangulation against a connection-closing stub persisted two errors containing the full request URL and synthetic `app_id`. No real credential used. | **Confirmed defect, fixed** in `d9bb27a7`. Safe request/transport/read errors and trusted HTTP status text; unwrapping preserves classification. Existing historical errors are not rewritten. |

#274 can close under its shared contract allowing scoped P2/P3 findings to
remain open with explicit impact. Neither #315 nor the corrected findings
demonstrates a provider posting ledger facts or erasing an owner decision.
There is no unmet original acceptance or closure blocker. No further independent
audit is required by the observed evidence; #315 owns targeted gap/restart/new
demand validation. #277 and #280 retain wider recovery and final shared gates.

## Scope and evidence register

Each row uses the baseline and recorded fixes above. “Verified” means the named
consumer passed this run. The named tests run under the focused command below.

| Scenario | Independent expected result | Consumer and actual result | Severity, disposition and limit |
|---|---|---|---|
| Manual prices, exact display, correction and void | `9007199254740993` and replacement `9007199254740995` retain all digits; same pair/date; old evidence preserved; no posting change | `TestPriceCoefficientsRemainExactAboveBrowserSafeInteger`, `TestManualPriceAndCorrectionHTTPPreserveExactProvenanceAndNoLedgerWrites`, six-locale pricing browser journeys pass | **Verified**, no defect. Existing int64 coefficient/scale-18 observation storage is an **accepted limit**. |
| Derived evidence, retirement and review | 0.9 USD/EUR × 0.8 EUR/GBP × 200 GBP/JPY = 144 USD/JPY; voiding a leg retires dependants under one audit; stale review/late failure writes nothing | Multi-hop FX, void cascade, reviewed correction/rollback/concurrency/depth tests and browser stale-review journey pass | **Verified**. Depth beyond 16 is atomically refused, an **accepted limit**. |
| Raw/adjusted selection (#203), valuation and knowledge dates | Same-date manual 80 beats provider 90; two units value at 160. Later eligible valuation date wins before source priority. Adjusted/bid/future/later-known quotes do not displace raw facts | Four `TestObservationSelection*` app cases, API counterparts and both browser journeys pass; independent exact constants cover approximate/exact/ties, cutoff and owner zones | **Verified**. Live knowledge cutoff does not reconstruct past retirements/book state, an **accepted limit**. |
| FX as-of, age, missing rates and exact display | Seven-day report boundary inclusive, eight days stale; holdings retain labelled old values. Source 100.00 × manual 0.80 = 80.00; withdrawing eligible rates keeps source 100.00 with future-only explanation | Observation selection, `TestPriceWorkspaceDatedCoverageExcludesFutureAndVoidedPrices`, gains-FX coverage cases and reporting browser pass | **Verified**. Diagnostic configured age differs intentionally from report/holdings rules. Historical-fetch completeness is **confirmed defective** under #315. |
| Quote identity, entitlements, malformed input and raw prices | Explicit CoinGecko ID/key/time; Twelve Data symbol/MIC/currency and raw close; Yahoo GBp/null refusal/unofficial marker; 401 terminal; 429 waits 600 seconds; quote redirects never forward credentials | Marketdata identity/decimal/HTTP cases and `TestQuoteWorkerTerminalFailureRetryRecoveryAndRateLimit` pass | **Verified** for synthetic fixtures. Live subscription rights, quota and additional wire semantics **not verified**; no new claim or provider admitted. |
| Quote revision, dedupe and owner assertions | Identical quote leaves one observation; changed quote appends supersession; identical voided quote not resurrected; same-date manual quote wins; no journals/lots | `TestQuoteWorkerDurableDedupeRevisionHealthAndNoLedgerWrites`, selection/correction tests, review of `StoreProviderQuote` and `insertPriceObservationTx` | **Verified** for exercised paths. Provider observations and owner assertions remain separate. |
| Crypto precision (R17) | Acquire 1.000000000000000000000001 units, sell 0.000000000000000000000001, retain exactly 1; CSV duplicate cannot post twice | Crypto app/API precision/import/identity cases and mobile `crypto-instruments` journey pass, including self-check | **Verified**. No exchange, DeFi or staking scope implied. |
| FX chunking, retry and idempotence | Four dates per direction at 0.8/1.25 despite a one-date budget; no duplicate observations; eight failed attempts end failed; manual retry resets attempts | `TestFXCoverageWorkerFinishesEveryContinuationChunk`, historical FX, FX/quote poison, failure/exhaustion/requeue/health tests pass after #314 | **Verified** for continuous publication; #315 is a **confirmed defect** for gaps. |
| Restart mid-job and expired/live lease | Interrupted HTTP leaves attempt 1 running. Reopened SQLite cannot fetch before expiry. At +15 minutes attempt 2 stores effects; injected completion failure remains recoverable. At +30 minutes attempt 3 completes without duplicate effects | `TestPricingWorkersRecoverInterruptedHTTPAfterDatabaseReopen` passes for real FX/quote/dividend workers: respectively two FX observations, one quote, one suggestion; ledger/lot counts unchanged. FX schedules a pending retry after completion error; quote/dividend retain lease | **Verified** using `StartBackgroundWorker`, adapters, SQLite close/open and real processing loops. Deterministic interruption, not an OS power-loss test. |
| Competing claims, startup and atomic handoff | One live owner/item; expired ownership reclaimed; stale owner cannot complete/enqueue; successor failure leaves original running | `TestClaimHandsOneItemToOneWorker`, `TestAnExpiredLeaseIsReclaimedAndCountsAsAnotherAttempt`, `TestTwoProcessWorkClaims`, `TestBackgroundWorkContinuationIsAtomicAndLeaseFenced` pass | **Verified** with independent processes and SQLite claims. Go race detector does not instrument native SQLite C. |
| Dividend proposals/completion/linking, no auto-post | Ten units × 1.6 = estimate 16, but missing payment/cash/amount/currency leaves incomplete proposal and no dividend. Owner completion alone invokes native posting. Linking existing dividend/payment creates no second payment; late failed acceptance rolls back | Dividend producer/completion/link/rollback tests, short/long+short cases and all three provider-dividend browser journeys pass | **Verified**. Manual check, last 365 days, dividends only; mixed long/short remains informational. |
| Immutable revisions preserve decisions | 1.60 and 1.6 coalesce; distinct content appends once; pending/ignored/accepted/linked state and recorded payment stay unchanged | Refetch, pending/linked revision tests and browser 0.50→0.55 disclosure pass; event/revision writer review finds no investment-writer call | **Verified**. Return to initial content is intentionally not recorded again; revision evidence is no financial command. |
| API/UI partial coverage and independent manual work | Exact source values survive missing/future/stale conversion. Mobile UI names missing dividend inputs; entitlement errors become failed work with retry; manual entry usable | Workspace/API/observation browsers, dividend completion/revision browsers, `TestDividendProducerFailureNeverBlocksManualEntry` and failed-work tests pass | **Verified**. Interrupted ingest-run history may remain `running` after work recovers; quote plan documents this **accepted limit**. |

## Validation and reproduction

Initial focused non-race selection: 131 top-level tests passed. Final focused
race checks: 265 distinct top-level tests across app, db, API, marketdata and
command packages, including the separately rerun HTTP addition. During test
development, an unused import and the recovery probe's FX completion-error
expectation were corrected; final affected tests pass. Those development
failures are not application findings.

```sh
./scripts/go-backend.sh test -race -p 2 -timeout 5m \
  ./internal/app ./internal/db ./internal/api ./internal/marketdata ./cmd/rekenraam \
  -run 'Pric|FX|Quote|Crypto|Observation|Dividend|BackgroundWork|Refresh|SourceAssignment|LocalDaily|ClaimHands|AnExpiredLease|TwoProcessWorkClaims|DerivedObservation|NonDerivedObservation|TwelveData|CoinGecko|Yahoo|Registry|ECB|Frankfurter|ExchangeRateAPI|OpenExchangeRates|MarketDataHTTP|DecimalToScaled|IntegrationSuitesRunTestsInParallel'
pnpm --dir e2e exec playwright test --project app --workers 2 \
  playwright/pricing-management.spec.ts playwright/observation-selection.spec.ts \
  playwright/crypto-instruments.spec.ts playwright/investments-provider-dividends.spec.ts
./scripts/go-backend.sh vet ./...
```

All 15 browser journeys passed both on the baseline and on the rebuilt final
implementation (1.6 minutes), including phone/keyboard/axe and six locales.
Integrated build includes 66 catalog checks, OpenAPI generation, static
embedding and serving test. Vet, formatting and diff checks pass. Full backend,
full browser and shared architecture gates were not run; #280 owns them.

To reproduce open #315, copy
[`fixtures/pricing-publication-gap-probe.go.txt`](fixtures/pricing-publication-gap-probe.go.txt)
to `backend/internal/app/pricing_holiday_probe_test.go`, run
`./scripts/go-backend.sh test ./internal/app -run '^TestFXCoverageWorkerHolidayDoesNotStarveLaterPublicationDates$' -count=1`,
then remove the temporary copy. Its intentional 1-versus-3 observation failure
and still-active continuation are evidence, not passing acceptance. June 10
is a synthetic gap, not a claim about a real provider's holiday calendar.

## Maintainability notes

- **Worker outcomes diverge across related paths.** `app/pricing_worker.go`,
  `app/pricing_quotes.go` and `app/investment_dividend_events.go` each own
  claims, attempt caps, retry and completion. FX retries a returned completion
  error; quote/dividend wait for lease expiry. The new three-kind test records
  the difference. FX discards provider error classification before its retry
  decision; quotes/events preserve terminal and Retry-After information.
- **Progress is inferred from observations.** `app/pricing_coverage.go` starts
  again at the earliest demand and treats an exact-date observation as progress.
  A non-publication day has no progress fact; #315 demonstrates starvation.
  Chunk handoff and knowledge of rate coverage are separate invariants.
- **FX and quote HTTP boundaries evolved separately.** `marketdata/http.go`
  and `marketdata/prices.go` have distinct status, size, redirect, error and
  retry handling. Quote errors were safe; FX rendering leaked a key (#316).
  No consolidation was attempted.
- **Suggestion schema crosses several layers.**
  `app/investment_dividend_events.go` emits typed evidence and map-shaped
  proposals; `app/investments.go` interprets completion; frontend
  `src/lib/investments/event-suggestions.svelte` renders missing inputs and
  revisions. New families must preserve immutable source and owner facts at
  each seam. Native writer/rollback tests establish what producer-only tests
  cannot prove.

These notes feed #284; no refactoring or refactor issues were created.
