# Live gains consistency review — 2026-10-15

Issue [#237](https://github.com/sergeyfarin/rekenraam/issues/237) exposed a response
that retained EUR 60 market value and EUR 40 operational basis, then re-read
corrected EUR 80 basis during historical FX composition: EUR +20 became EUR -20
even when the reporting currency was EUR. The named regression failed before
isolation was added, reproducing that mismatch and a person-pooled comparison
that switched to the corrected basis.

## Boundary and implementation

`InvestmentService.ReadGains` and `ReadGainsStatement` now capture one disposable
SQLite image before any component is read. The handlers call those composed
services once. Native gains, capital-return allocations, alternative/person-pooled
results, both statement boundaries, realized disposals, income, historical basis,
currency metadata and FX rates all read that image. Standalone `GainsStatement`
also captures once; an already isolated saved-run reader reuses its image.

The implementation reuses the native backup owner from #234 and all existing
financial readers. Dated and alternative replay temporarily write projection
tables, then roll back. An ordinary read-only WAL transaction therefore cannot
host these readers unchanged. One live writer transaction across composition
would block unrelated requests through replay. The disposable image permits
source writes after capture, while every component retains the captured state.

The live pool's single connection is occupied during native capture, then released
before opening/replaying the image. The existing native-copy deadline is 30 seconds
and request cancellation propagates. Temporary directories are 0700 and database
files 0600; cleanup closes the pool and removes the image and sidecars after
success, failure or cancellation. Live capture skips the saved-run SHA-256 pass.
No financial, audit or saved-run rows are written by these GETs. They make no
provider requests and do not change schema, money arithmetic or response shapes.

## Cost

This deliberately copies the **whole database for each request**. Capture I/O and
temporary disk scale with database size, including saved reports and unrelated
tables. Replay adds its own temporary WAL and processing. This bounds the fix
without introducing another replay engine or a transaction-aware rewrite of all
readers; it is not a claim that copying is free or an eventual scaling strategy.

Repeatable synthetic benchmark, Linux/amd64, AMD EPYC 3251, native SQLite/FULL,
ten iterations per image size on this validation host:

```sh
GOCACHE=/tmp/rekenraam-go-build-cache ./scripts/go-backend.sh test ./internal/db \
  -run '^TestLiveGainsSnapshot' -bench '^BenchmarkLiveGainsSnapshot$' \
  -benchtime=10x -count=1
```

| Input image bytes | Synthetic padding | Capture + open + cleanup, mean |
| ---: | ---: | ---: |
| 1,556,480 | None; migrated empty database | 66.30 ms |
| 18,354,176 | 16 MiB blob | 112.17 ms |
| 68,734,976 | 64 MiB blob | 210.11 ms |

These measurements include native copy, consolidation, PRAGMA-validating open and
cleanup. They exclude report replay and formatting and are not representative-book
latencies or production capacity limits. A later optimization must preserve the
same composition boundary and named interleaving tests.

## Validation and review

- `TestLiveGainsCorrectionBetweenOperationalAndFXKeepsSameCurrencyGain` corrects
  EUR 40 acquisition basis to EUR 80 after native calculation. Native,
  person-pooled and same-currency FX keep EUR +20 for both FX methods; a fresh
  request sees EUR -20.
- `TestLiveGainsObservationWithdrawalsBetweenStagesKeepQuoteAndFXEvidence`
  withdraws quotes and FX after native calculation. Comparison values and
  reporting evidence stay intact for both FX methods; a fresh request discloses
  missing coverage instead of substituting zero.
- `TestLiveStatementBoundariesDisposalsIncomeAndFXShareEffectiveHistory`
  corrects an acquisition and reverses income after the closing stage, then
  withdraws observations before FX. Opening, closing, disposals, income and
  historical basis retain their original values for both methods.
- `TestLiveGainsAPICorrectionBetweenCompositionStages` and
  `TestLiveStatementAPIObservationWithdrawalBetweenNativeAndReporting` perform
  authenticated correction/withdrawal requests during GET composition, and
  compare complete financial/observation/audit/run rows afterward.
- `TestLiveGainsSuccessFailureAndCancellationLeaveFinancialAuditAndRunRowsUnchanged`
  covers successful comparison, rejection after replay and cancellation.
  `TestLiveGainsSnapshotIsPrivateDisposableAndIndependent` verifies permissions,
  independence, pool closure and removal of the temporary directory.
- Existing browser journeys for dated valuation, statements, alternatives,
  person-pooled comparison, FX and saved runs passed: eight tests including the
  two authentication bootstrap cases, 2.2 minutes. Mobile and six-locale cases
  remain included; the journey runner also rebuilt the integrated binary.

- Full `GOCACHE=/tmp/rekenraam-go-build-cache ./scripts/test-backend.sh` passed,
  including gofmt, vet and all packages under `-race -p 2 -timeout=5m`.
  Application execution was 123.871 seconds; API 97.198 seconds.
- `./scripts/test-frontend.sh` passed: generated OpenAPI types, catalog compilation,
  zero Svelte diagnostics and all 628 unit tests in 45 files.

The initial sandboxed backend/browser attempts refused local TCP listeners;
validation was repeated with local socket access rather than skipping tests.

Review checked that every read dependency is constructed from the image, errors
return no partial result, saved runs avoid nested capture, and the handlers only
map the composed service output. No new independent defect was found. The already
tracked capital-return period-disclosure issue #238 remains separate.

## Retained meaning of live

The descriptor remains `ledger_knowledge: current_effective`. A cutoff admits
observations; it does not reconstruct earlier correction or retirement knowledge.
Each response is internally consistent but discarded. Later requests may restate
it. Only an explicit saved run retains its output for retrieval, and an unsaved
past response cannot be recovered from a cutoff.
