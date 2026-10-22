# Owner-local current reference reads (#385)

The #381 midnight failure was a production read defect: account writes used
owner-today while current account reads used SQLite UTC today. The repair
passes the application-resolved financial day through `db.WithReferenceDate`
on each operation context. Repository query builders select by effective date,
then version sequence, against that explicit day. The value is never stored in
a connection, process-global clock, offset column or financial record.

Application services resolve the saved single-owner IANA zone before opening
repository transactions/read snapshots. Nested reads inherit the operation's
date. Each worker operation resolves again; scheduler contexts are not pinned.
Absent preferences keep UTC, invalid saved zones fail, and the next operation
sees changed preferences or a new day. Forecast keeps its existing snapshot:
its application resolver supplies `AsOfDate` to the payee enrichment read in
that same transaction.

## Current-version consumer survey

| Source | Runtime consumers and disposition | Retained regression |
|---|---|---|
| Account versions, including categories | Account/category lists, detail, parent/cycle/name/archive/reference guards and setup; transaction/register/report enrichment; budgets; pricing discovery; import resolution; investment income/split/capital-return/provider-link labels; export account paths and self-check all receive the operation date. | `TestCurrentReferenceReadsFollowOwnerDay`, `TestCurrentReferenceSiblingsAndComposedReadsUseOwnerDay`, `TestOwnerCurrentReferenceHTTPCreateListAndFutureExclusion`, `owner-current-reference.spec.ts` |
| Institution versions | Institution list/detail/lifecycle/dependency guards and account institution validation use the same date. Version history stays complete. | Sibling regression above; existing institution/account lifecycle suites |
| Commodity versions | Currency list/detail/default/setup, account currency checks, posting enrichment, budget/pricing/quote options, instrument validation and export reference tables use the same date. | Sibling regression above; existing commodity/pricing suites |
| Payee versions | Payee reads/defaults/name guards, import and recurring labels, transaction/register enrichment, export and forecast labels use the operation date; forecast uses its snapshot-resolved day. | Sibling regression above; existing recurring/forecast/export suites |
| Investment instrument versions | Instrument search/list/detail/writes, quote targets, pricing pairs and split/capital-return labels use the same date. | Sibling regression above; existing instrument/quote suites |
| FX account-version insertion trigger | Migration 0021 checks other accounts at `NEW.effective_from`; a future version cannot suppress earlier coverage and an owner-today account already using that currency prevents redundant activation. | `TestReferenceDateMigrationPreservesHistoryAndUsesSourceDates` |
| Instrument precision trigger | Migration 0021 chooses the commodity version at the new instrument version's effective date, including future-dated source fixtures. | Same migration regression, including refusal at the later reduced precision |
| Explicit historical versions | Posting account/commodity rules, account version safety, ledger/report dates, observation boundaries, effective investment replay and full source history retain their existing explicit dates and accounting rules. | Sibling historical posting-rule control; existing #292/#293/#339/#343 suites |
| Transaction/current investment projection views | Their current semantics are sequence/lifecycle-based, without a UTC calendar-day predicate. Unchanged. | Existing ledger/replay/self-check suites |

The five original UTC views remain in the immutable schema for legacy direct
SQL/repository callers and diagnostics. Runtime current-reference SQL uses
`CurrentReferenceVersions` with the application context instead. Date-less
repository callers retain the legacy views for compatibility; application
operations establish the boundary even when an unwired service uses the UTC
default. No old migration or source version is rewritten. The survey also found two
unbounded latest-version queries outside those views: export institution labels
and spending-report payee labels. Both now use the same current-reference
boundary, with future-label assertions in the sibling regression.

## Boundary evidence

The controls freeze the financial clock at `2027-01-01T23:01:00Z` in
Europe/Amsterdam (owner day January 2, just after local midnight) and
`2027-01-02T03:00:00Z` in America/Los_Angeles (owner day January 1, UTC January 2).
They create real account versions and immediately read/list/select them.
The HTTP control appends a later version through the real PATCH handler,
rewinds the test clock and proves exclusion. Sibling database fixtures append
future and same-day versions for every reference kind, retain original history,
and prove explicit historical posting rules still select their supplied date.
Budget/pricing composition and bundle export use the same reference boundary.
Preference/clock refresh and concurrent context isolation are covered by
`TestReferenceDateRefreshesPreferencesWithoutLeakingAcrossOperations`.

Playwright's fixed-clock fixture runs a test-only Go HTTP server with normal
services, authentication, middleware, isolated SQLite and the built frontend.
Authentication uses its real expiry clock. No testing endpoint, environment
clock override or test server is present in the production binary. The two
mobile controls are part of the ordinary app smoke selection. The original
six locale-amount journeys retain their existing assertions.

## Validation

- Focused account/reference and HTTP controls passed; the final named
  reference/migration/schema regressions also passed with Go race detection.
- `./scripts/test-backend.sh` passed its formatting, vet and complete native
  SQLite race gates, including account version/date safety and owner defaults.
- `./scripts/test-frontend.sh` passed with zero Svelte diagnostics and all
  56 unit-test files / 727 tests passing.
- `pnpm build` passed; the final Go source was rebuilt against that validated
  embedded frontend before browser validation.
- `./scripts/measure-large-book.sh` passed on the final source: 250,000
  postings, bundle export 11.719 s / 20 MiB peak live heap; self-check
  12.283 s / 38 MiB peak live heap. These are local observations with the
  script's bounds, not a comparison or a performance guarantee.
- Focused browser controls passed: both fixed-instant account cases and all
  six locale-amount journeys. Both fixed-instant cases passed again after
  tightening their test-only server cleanup.
- `TZ=Europe/Amsterdam E2E_SKIP_BUILD=1 E2E_WORKERS=3
  E2E_TIMING_DIR=timing-results/385-full-final ./scripts/test-e2e-smoke.sh`
  passed: all 380 cases, including the two fixed-instant controls and original
  six locale-amount journeys, plus the wrapper's stub and harness checks.
  The validated binary was reused; browser execution took 15.3 minutes locally.
  Local timing artifacts are under `e2e/timing-results/385-full-final/`.
  This is delivery verification, not a rerun or modification of #381's
  benchmark cohort.

One earlier smoke attempt was discarded after a concurrent focused invocation
reset its shared worker directory. The accepted complete run uses a fresh
worker directory and no concurrent browser invocation. The initial named
account regression failed with `account not found` before the repair.
No timeout was increased and no posted financial record or amount was changed.
