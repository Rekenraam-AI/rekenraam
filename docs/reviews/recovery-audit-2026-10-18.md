# Export, migrations, self-check, backup/restore and locks audit — 2026-10-18

**Status:** [#277](https://github.com/sergeyfarin/rekenraam/issues/277), area 9
of [#260](https://github.com/sergeyfarin/rekenraam/issues/260), complete. Every
delivery bullet has a disposition. The **restore drill matches the #266 register
exactly** on a fresh instance. All three corruption classes the issue names
(plus an investment-operation orphan) are detected on a restored copy, beside
the reference book's documented unknown-basis coverage. Every failure path ends
in a documented, non-corrupt state, and previews, exports, status reads and
checks write nothing beyond their own run records. No financial, security or
data-loss defect was found. Three P3 findings were filed:
[#321](https://github.com/sergeyfarin/rekenraam/issues/321) (export exclusion
list incomplete) and [#323](https://github.com/sergeyfarin/rekenraam/issues/323)
(two recovery documentation gaps) are fixed in this change;
[#322](https://github.com/sergeyfarin/rekenraam/issues/322) (bundle id columns
that resolve to nothing in the archive) was fixed afterwards in bundle schema 15
(see the addendum). This is bounded evidence, not certification of every interleaving or
of release readiness.

## Scope and baseline

- Baseline: `17bc40e10d5c7caf4620ebec0ad27cf8406f4114`, clean working tree.
  Migration head is `0018`; the issue text's "head (0013)" predates migrations
  0014–0018.
- Governing sources: [data portability plan](../plans/data-portability-plan.md),
  [native SQLite plan](../plans/native-sqlite-driver-migration-plan.md),
  ADR 0004, 0011, 0014, 0015 and 0016, [upgrades](../upgrades.md),
  [deployment security](../deployment-security.md) and the
  [shared audit brief](app-state-and-audit-plan-2026-10-17.md).
- Oracle: the #266 reference book (`fixtures/reference-book.json`) and its
  hand-derived register. Figures are never read back from the code under test.
  The bundle reconstruction recomputes every figure from the archive with its
  own arithmetic (`math/big`) and compares it with the register.
- Books are synthetic and provider-free; no provider is called.
- New audit consumers: `backend/internal/api/recovery_audit_test.go` (restore
  drill, corruption on restored copies, bundle reconstruction) and
  `TestRestoreCommandRerunsCleanlyAfterAnInterruptedReplacement` in
  `backend/cmd/rekenraam/restore_test.go`. `newTestInstanceOnDatabase` in
  `backend/internal/api/setup_test.go` wires the test API over a given database
  file, so the drill can serve the restored file from a second instance.

## Findings

| Finding | Severity | Expected versus actual | Disposition |
|---|---|---|---|
| [#321](https://github.com/sergeyfarin/rekenraam/issues/321): export exclusion list omits owner data | P3 | The manifest/preview `excluded` list and README say they name what the archive leaves out. Comparing the schema's tables with the bundle files found eight kinds of owner data absent and unnamed. Among them: budget targets and account treatment, recurring templates, reconciliation checkpoints, effective-dated history, cost-basis profiles, pricing policies and preferences. The README also did not explain that average-cost disposal events carry the pool rate, so one lot's events can sum below zero (B3: 200.00 − 230.00). | **Confirmed, fixed.** `ExportExcludedData`, the README and the plan name each category; README adds the average-cost note. No file layout or bundle schema change. `TestRecoveryAuditBundleReconstructsTheRegister` failed on the baseline on exactly these assertions and passes now. |
| [#322](https://github.com/sergeyfarin/rekenraam/issues/322): bundle id columns resolve to nothing | P3 | Every ledger, lot, event, operation, decision, commodity and account reference resolves. `posting_version_id` in `investment-operation-components.csv` and `disposal-clearing-allocations.csv`, and account-version/cost-basis-profile ids in `disposal-decisions.csv` and `investment-transfer-facts.csv`, point at records no file carries. Nothing says so. | **Confirmed, fixed after the audit** (addendum). No figure depended on these joins. |
| [#323](https://github.com/sergeyfarin/rekenraam/issues/323): recovery documents | P3 | `docs/upgrades.md` said to keep the pre-upgrade backup "until the result is `passed`". The reference book is `incomplete` before and after restore (#313), so that instruction could never be met. No document described what a killed restore leaves behind. | **Confirmed, fixed.** The upgrade step and maintainer checklist accept `incomplete` with unchanged coverage and no failure. `deployment-security.md` gains an *Interrupted restore* paragraph, backed by the new re-run regression. |

Handed to #278, not filed here: the Data screen renders the export's
`excluded` strings as sent by the server, in English, in every locale
(`frontend/src/lib/settings/data-settings.svelte`, the preview's excluded list).
That is a localization-boundary question for the frontend audit.

## Scope and evidence register

"Verified" means the named consumer passed in this run.

### Restore drill (#266 book)

| Scenario | Independent expected result | Consumer and actual result | Disposition and limit |
|---|---|---|---|
| Source book | Every register figure: 24 balances, 2 subtree balances, account status, reconciliation checkpoint 5460.00, draft list, spending, cashflow, budget, net worth Jan 5315.00 / Feb unavailable / Mar 8247.45 EUR, 4 positions, 6 lots, realized and unrealized gains under operational/FIFO/LIFO/average (350/380/255/300 USD realized) | `TestRecoveryAuditReferenceBookRestoreDrill` → `assertReferenceBookRegister` on the source | **Verified.** |
| Backup through the product | `POST /maintenance/backups` → 202; the production `RunBackup` writes a verified file, completes the run and chains a `scheduled` self-check | Same consumer: run `completed`, `verified=true`, chained run present | **Verified.** The background worker loop is covered by `TestWorkerCompletesAClaimedBackup`; the drill drives the same `RunBackup`. |
| verify-backup | `integrity_check` and `foreign_key_check` ok; schema = binary head; the file is not modified | `db.InspectBackup`; backup SHA-256 equal before and after | **Verified.** |
| Restore to a fresh instance | Lock held across restore; nothing preserved at a fresh path; `serve`-style open re-admits migrations; a fresh password sign-in works | `restoreOntoFreshInstance` + `signInOwner` | **Verified.** |
| Restored figures | The register exactly, with the same seeded ids | `assertReferenceBookRegister` against the restored handler | **Verified — every figure matches.** |
| Restored self-check | Same verdict as the source: run `incomplete`, 0 failed, 1 incomplete; `lot_reconciliation` incomplete with 0 findings and unavailable sample `[lot_g1]`; `attachments` not_applicable; all others passed | `assertReferenceSelfCheckVerdict` on source manual, chained scheduled and restored runs | **Verified.** The unknown-basis result is supported unavailable coverage, not corruption. |
| Restored exports | `ledger.csv`, every bundle file and every QIF file byte-identical; manifests and preview identical except `generated_at` | Byte/JSON comparison, source vs restored | **Verified.** |

### Self-check corruption (restored copies)

Each case restores the drill's backup to a new location, corrupts it with raw
SQL, signs in and runs `POST /maintenance/self-check`.

| Corruption | Expected | Actual | Disposition |
|---|---|---|---|
| One-sided 10.00 posting in checking's current posted opening-balance entry | `entry_balance` failed, run failed | As expected | **Verified.** The same row on a superseded version is history and is rightly ignored. The first draft of the audit picked that version and saw a pass, which is correct behaviour. |
| B2 lot state 6 units against a holding of 5 | `lot_reconciliation` failed | As expected; unavailable count still 1 (`lot_g1`) beside the finding | **Verified.** |
| Recurring occurrences whose template was deleted with foreign keys off | `sqlite_integrity` failed | As expected | **Verified.** New: no earlier test fed `foreign_key_check` a real orphan through the self-check. |
| Journal-backed `buy` operation with no posted version link | `investment_foundation` failed | As expected | **Verified.** |
| Repairs nothing | Only `self_check_runs` and `self_check_results` change | Whole-database fingerprint, before and after, for every case | **Verified.** |

Unit-level coverage of every check id is reused: `TestSelfCheckCatchesEachCorruption`,
`TestSelfCheckWritesNothingToLedgerTables`, `TestReplaySelfCheckReports*`,
`TestSelfCheckIncomplete*`, `TestPriceDependencyCheck*`, and
`TestInterruptedSelfCheckIsRecordedAsErrored` /
`TestStartupRecoversSelfCheckRunInterruptedByCrash` for an errored run.

### Previews and checks do not mutate

| Operation | Allowed writes | Actual | Disposition |
|---|---|---|---|
| Export preview, `ledger.csv`, bundle, whole-book QIF, self-check and backup status reads, every register report read | none | No table changed (fingerprint of every row of every table) | **Verified**, on the source and on the restored instance. |
| Manual self-check | `self_check_runs`, `self_check_results` | Exactly those | **Verified.** |
| Manual backup run | run, work item, chained self-check | `background_work_items`, `backup_runs`, `self_check_runs`, `self_check_results` | **Verified.** No audit event is written for a backup run. That is consistent with the plan, which lists the run record as the audit. |
| verify-backup | none, file unchanged | SHA-256 unchanged | **Verified.** |

### Exports

| Scenario | Independent expected result | Consumer and actual result | Disposition and limit |
|---|---|---|---|
| Manifest completeness | Every archive file listed; each listed digest, byte count and CSV row count recomputed independently | `TestRecoveryAuditBundleReconstructsTheRegister`: all 45 listed files match; nothing unlisted | **Verified.** `TestExportBundleChecksumsVerify` reused. |
| Exclusion list | Names every kind of owner data the archive omits | See #321 | **Confirmed defect, fixed.** |
| References | Every id column resolves to an exported row or is a documented exclusion | See #322; all others resolve | **Confirmed defect, fixed in schema 15.** |
| Balances rebuilt from `ledger.csv` | All 24 register balances by entry date, including #187 in-transit `transfer_clearing` figures; both subtree balances via `accounts.csv` parents; archived status of old cash | Same consumer | **Verified.** |
| Reconciliation rebuilt | Reconciled postings cleared by 15 Jan sum to the 5460.00 statement | Same consumer | **Verified.** The checkpoint row itself is not exported (#321 names it). |
| Spending and cash movement | Expense 175.00 / 55.00 / 45.00 EUR, 8.00 USD; income 5000.00 EUR, 15.00 USD; cashflow net movement per currency | Same consumer | **Verified.** Cashflow inflow/outflow/transfer classification is not rebuilt; the API register check covers it. |
| Net worth rebuilt | Asset and liability accounts, rates and prices at most 7 days old: Jan 5315.00, Feb unavailable (the USD rate is 28 days old), Mar 8247.45 EUR | Same consumer, from `prices.csv` | **Verified.** |
| Lots and positions | `lots.csv` matches all six register lots. Lot events reproduce every quantity, and every position's basis (300.00 / 315.00 / unknown / 300.00) | Same consumer | **Verified.** Per-lot event sums under average cost differ from per-lot state by design (README note, #321). |
| Gains rebuilt | From lot facts, split facts, capital-return facts, disposal facts and prices: operational 300 + 30 + 20 = 350; FIFO 380, LIFO 255, average 300; every unrealized figure, unknown stays null | Independent replay `replayBundleInvestments` | **Verified.** |
| Payment in lieu, dividends | Typed facts name the short holding and the paying instrument; amounts land on borrow costs and dividends | Same consumer | **Verified.** |
| Budget | Targets and treatment are not in the archive | No budget file | **Stated limit.** The restore drill recovers them natively. |
| Saved and revision evidence | Immutable gains runs, disposal revisions, transfer-link and capital-return revisions exported append-only | Reused: `TestBundleLotsFileCarriesCostBasisAtItsOwnScale`, investment export tests in #272/#273/#308 | **Verified (reuse).** |
| CSV and QIF | Balanced per entry and commodity; stored scale, no float; QIF round trip and unsupported-account coverage list | Reused `exports_test.go`, `exports_qif_test.go` (incl. `TestExportPreviewReportsQIFSupportAndExclusions`, `TestQIFWholeBookExportSkipsInvestmentAccountsWithoutRefusing`) | **Verified (reuse).** |

**Round trip, stated explicitly:** the shipped bundle supports
**reconstruction of every ledger and investment figure** in the register:
balances, reconciled statement, spending, net movement, net worth, lots,
positions, realized and unrealized gains under all four methods. It does not
support a full **installation** round trip. Budgets, recurring templates,
reconciliation checkpoints, reference-data history and settings are not in it
(#321 now says so). Since bundle schema 15 every id resolves inside the archive (#322). The native
SQLite backup is the complete round trip, and the drill proves it exactly. No
bundle importer exists or is implied. This disposes of #272's remaining
bundle-reconstruction acceptance item.

### Migrations

| Scenario | Expected | Consumer | Disposition |
|---|---|---|---|
| Empty database to head | All 18 migrations apply; schema equals the expected shape | `TestMigrateAppliesEmbeddedMigrations`, `TestMigrationsProduceTheExpectedSchema` | **Verified.** |
| Seeded database to head | The frozen v0.1 seed loads into the baseline and every durable row survives admission to head | `TestMigrateSeededBaselineMatchesFreshSchema` | **Verified.** |
| Data-preserving later migrations | Each round trip keeps its rows and refuses lossy downgrades | `TestSelfCheckIncompleteMigrationPreservesHistoricalRuns`, `TestFXCoverageProgressMigrationPreservesQueuedWork`, `TestImportIdentityScopeMigrationRoundTrips`, `TestProviderEventLinkMigrationPreservesSuggestionsAndRefusesLossyDowngrade` | **Verified.** |
| Restored file re-admission | `db.Migrate` on a restored head-version file is a no-op | Restore drill | **Verified.** |
| Freeze and immutability | `freeze_test` pins `0001`/`0002`; conventions allow rewriting before the first installed release only with `BREAKING DEV DATABASE`, and the release checklist pins every released migration | `TestPinnedMigrationChecksums`; review of `docs/conventions.md` and `docs/upgrades.md` | **Verified, accepted limit.** `0003`–`0018` are unpinned by design until v0.1.0. |
| Newer schema | Restore refuses | `TestRestoreRefusesNewerSchema`, `TestRestoreCommandReleasesLockOnFailure/newer_schema` | **Verified.** |

### Backup, keys and locks

| Scenario | Consumer | Disposition |
|---|---|---|
| Nightly scheduling in owner local time, once per day, DST, disabled policy | `TestSchedulerArrangesOneBackupPerLocalDay`, `TestSchedulerFollowsTheOwnersClockAcrossDST`, `TestSchedulerRespectsADisabledPolicy`, `TestScheduledOccurrenceIsCreatedOnlyOnce`, `TestTwoProcessScheduledBackupOccurrence` | **Verified (reuse).** |
| Manual backup is accepted work, not a completed claim | `TestRequestBackupReportsAcceptedWorkNotACompletedBackup` | **Verified (reuse).** |
| Online backup API from the read-only pool, writes not blocked, uncheckpointed WAL captured | `TestBackupUsesOnlineBackupAPI`, `TestBackupSourcesFromReadOnlyPoolAndDoesNotBlockWrites`, `TestRestorePreservesUncheckpointedWALContent` | **Verified (reuse).** |
| Retention prunes only its own files; symlink and outside-directory refusal | `TestBackupWorkerPrunesOnlyItsOwnFiles`, `TestPruneRefusesSymlinkAndPathOutsideBackupDir`, `TestPruningFailureStillLeavesASelfCheckBehind` | **Verified (reuse).** |
| Key custody | `TestSealedDataDecryptsAfterRestoreWithRetainedKey`; `TestVerifyBackupReportsSealedDataOpeningWithTheRetainedKey`, `…WrongOne`, `…ExplainsTheCostWhenNoKeyIsConfigured`; MFA key-loss sign-in from #290 | **Verified (reuse).** |
| Server/restore lock | `TestRestoreRefusesWhileServeHoldsTheLock`, `TestRestoreCommandRefusesWhileTheServerIsRunning`, `TestTwoProcessRestoreOwnsLockThroughoutReplacement`, `TestTwoProcessLockHandoffsKeepOneFileIdentity`, `TestTwoProcessServerAndRestoreGuards`, `TestRestoreCommandReleasesLockOnFailure` | **Verified (reuse).** |

### Failure paths

| Path | Documented end state | Consumer | Disposition |
|---|---|---|---|
| Interrupted backup | No final-named file from a failed copy; retry adopts a verified final file or rewrites `.part`; no untracked file | `TestBackupRetryAfterCrashLeavesNoDuplicateOrUntrackedFile`, `TestBlockedCopyNeverPublishesAFinalNamedBackup`, `TestBackupWorkerVerifiesBeforePublishingFinalName`, `TestNativeOnlineBackupContentionCancelsAndCleansUp` | **Verified (reuse).** |
| Full disk (simulated) | Retryable, legible, recovers when space returns; partial output removed | `TestDiskShortageIsRetryableAndRecovers`, `TestWorkerRetriesToTheCapAndThenStops` | **Verified (reuse).** |
| Cancelled restore | Cancellation before preservation: nothing replaced, database byte-identical, lock released. After preservation the copy does not observe the context, so the install completes. | `TestRestoreCommandReleasesLockOnFailure/cancelled`; review of `db.RestoreSQLiteDatabase` | **Verified.** |
| Restore failing part-way | Exact message for each of the three states; preserved copy intact | `TestRestoreCommandSaysWhereTheDataIsWhenItFailsPartWay`, `…/after_preserving`, `…/after_installing` | **Verified (reuse).** |
| Restore killed part-way | Nothing at the path, preserved set whole, partial `.restoring`; re-running installs the backup and leaves the first preserved copy untouched | `TestRestoreCommandRerunsCleanlyAfterAnInterruptedReplacement` (new) | **Verified; documented by #323.** |
| Missing key | Ledger intact; sealed rows unreadable and reported before restore | Key-custody tests above | **Verified (reuse).** |

## Acceptance

- [x] The restore drill matches the register exactly.
- [x] Every corruption class is detected by the self-check.
- [x] Every failure path ends in a documented, non-corrupt state (killed
  restore documented by #323 in this change).
- [x] Previews and checks are shown not to mutate data.
- [x] #272's added scope: manifest hashes, references, original/revised
  knowledge, basis elections and payment-in-lieu facts checked independently;
  figures rebuilt from exported evidence; the round-trip boundary is stated
  above. The unknown-basis `lot_reconciliation` result is distinguished from
  deliberate corruption in the drill.

#322 (P3) was fixed after the audit; see the addendum.

## Validation

```sh
./scripts/go-backend.sh test -race -p 2 -timeout 15m ./internal/api ./internal/app ./internal/db ./migrations ./cmd/rekenraam ./internal/testdb \
  -run 'RecoveryAudit|Export|Bundle|QIF|Qif|SelfCheck|Backup|Restore|ReferenceBook|Migrat|Pinned|Seeded|Baseline|TwoProcess|VerifyBackup|Sealed|Parallel|Recover'
pnpm --dir e2e exec playwright test --project app --workers 2 playwright/data-settings.spec.ts
```

Results on the final change: **273 top-level backend race tests** pass across
api (64), app (157), db (25), migrations (1), cmd/rekenraam (24) and testdb (2),
including the four new audit consumers and the unchanged
`TestReferenceBookProducesItsRegister`. **9 browser journeys** in
`data-settings.spec.ts` pass against a fresh integrated build (export, backup,
self-check and the six-locale incomplete disclosure). `go vet`, gofmt,
`git diff --check` and a local link check of the changed documents are clean.
`TestRecoveryAuditBundleReconstructsTheRegister` failed on the baseline on the
#321 assertions alone (exclusion list and README) before the fix.

`./scripts/measure-large-book.sh` was not run: the export and self-check code
changed only in static exclusion and README strings. Full backend, browser and
shared gates were not run; #280 owns them.

Retained limits:

- The drill restores into a fresh directory of the same host and process
  environment. A different host, file system or container is #279's deployment
  scope.
- The Go race detector does not instrument native SQLite; lock evidence comes
  from the independent-process tests.
- Cashflow classification and budget figures are not rebuilt from the bundle:
  the first is checked through the API, the second is not exported.

## Maintainability notes

- **Two export exclusion texts.** `ExportExcludedData` (`app/exports.go`) and
  the README paragraph in `app/exports_bundle.go` restate the same list in
  different words, and the plan restates it a third time. #321 had to edit all
  three; nothing ties them together except this audit's test.
- **The archive's id vocabulary is implicit.** Which columns are foreign keys,
  and into which file, is known only from names. The audit had to hand-write a
  column→target map (`recovery_audit_test.go`) to find #322. A new export file
  can add a dangling id without any test noticing unless it matches that map.
- **"Current version" is re-derived in raw SQL by tests and repairs.** The first
  corruption probe picked a superseded transaction version with
  `ORDER BY pv.id`. The current-version rule (status plus highest
  `version_seq`) lives in views and service code, and is easy to get wrong
  outside them.
- **Test wiring duplicated production wiring.** The API test handler built
  every service inline. It is now `newTestInstanceOnDatabase`, but it still
  mirrors `appruntime` by hand, so a new service must be added in both places.
- **Restore steps live in two layers.** Lock ownership is in the command
  (`cmd/rekenraam/restore.go`), preservation and install in
  `db.RestoreSQLiteDatabase`, which documents that the caller must hold the
  lock. The drill had to reproduce that pairing.

These notes feed #284; no refactoring or refactor issues were created.

## Addendum: #322 fixed (2026-10-18)

The owner noted that no exports exist yet, so the layout could change freely
before the first release. Bundle schema 15 adds `posting_version_id` to
`ledger.csv` beside `posting_line_key`. It drops the account-version and
cost-basis-profile ids from `disposal-decisions.csv` and
`investment-transfer-facts.csv`. ADR 0011 records the pre-release exception
to append-only. `TestRecoveryAuditBundleReconstructsTheRegister` now requires
**every** id column to resolve inside the archive, except audit-event and user
attribution, with no known-gap list.
