# QIF/CSV and Trading 212 imports audit — 2026-10-17

**Status:** sub-audit
[#271](https://github.com/sergeyfarin/rekenraam/issues/271) (area 4) of
[#260](https://github.com/sergeyfarin/rekenraam/issues/260). It is complete,
with five findings filed separately rather than fixed in the audit change, per
the evidence contract:
[#299](https://github.com/sergeyfarin/rekenraam/issues/299) (P2), and
[#300](https://github.com/sergeyfarin/rekenraam/issues/300),
[#301](https://github.com/sergeyfarin/rekenraam/issues/301),
[#302](https://github.com/sergeyfarin/rekenraam/issues/302) and
[#303](https://github.com/sergeyfarin/rekenraam/issues/303) (P3). None is a
P0/P1 integrity defect. No parser was observed to change an amount's
magnitude. The P2 can double-count a transaction, but only through an explicit
owner commit of rows the review screen marks `new`. The audit change adds
evidence only and corrects two stale capability claims.

## Scope and method

The audited SHA is `410b177a`, with a clean working tree. Evidence comes from
the real HTTP handler (`newSetupTestHandler`) over a temporary SQLite book with
a synthetic owner, and from the integrated binary driven by Chromium at 390 px.
Trading 212 runs only against the stubbed provider (the fetcher's test server
and the e2e `TRADING212_BASE_URL` stub). No live provider is involved (ADR 0017).

Expected results come from the file bytes and these documents, not from the
code under test:

- the [import plan](../plans/import-plan.md): dedupe identity, partial commit,
  reconciliation handling, rollback
- the [Trading 212 plan](../plans/trading212-import-plan.md) and
  [connection accounts](../plans/import-connection-accounts-plan.md)
- the [import dividend review plan](../plans/import-dividend-review-plan.md) and
  the [CSV investment income contract](../plans/csv-investment-income-plan.md)
- the documented locale rules in `import_locale.go`: one day above 12 settles a
  file's date order, and one unambiguous amount settles its decimal separator.
  A profile's declaration overrides detection.

Exploratory probes ran as a throwaway API test and were then removed. Each
finding quotes what it observed. The scenarios that had no consumer are now
named tests in
[`import_audit_test.go`](../../backend/internal/api/import_audit_test.go).

## Findings and disposition

| Finding | Severity and evidence | Disposition |
|---|---|---|
| QIF identity depends on the file name and on the record's **position** in the file, not on the occurrence among identical rows. An overlapping re-export, or a renamed file, stages already-committed rows as `new` | P2, [#299](https://github.com/sergeyfarin/rekenraam/issues/299). Coffee −10.00 and Lunch −20.00 committed from `bank.qif`. A later `bank.qif` with an extra earlier row Bus −5.00 staged 3 × `new`; the same two rows as `bank-2026-06.qif` staged 2 × `new`. Committing either batch posts them twice. The plan specifies an intra-file occurrence index; CSV implements it | **Fixed** 2026-10-17: the QIF identity is content plus the occurrence among identical rows, without file name or position (`TestImportAuditQIFReexportIdentityIgnoresFileNameAndPosition`). Account scope fixed by #302 |
| A row rejected under the file's declared or detected convention stays committable. Commit re-reads it with per-value detection, and the parse warning is not stored with the batch | P3, [#300](https://github.com/sergeyfarin/rekenraam/issues/300). Declared `,`/DMY profile, rows `-12,50`, `-1.234`, `-12.50`: the third gave `row 4 has an unrecognized amount`. After `GET /imports/{id}` nothing was flagged; the commit posted 3/3 and the balance was −1259.00 (12.50 + 1234 + 12.50). The CSV investment path refuses the same case ("never reinterpret a rejected source in another locale") | **Fixed** 2026-10-17: the row keeps `parse_errors` and stages `needs_attention`, commit fails it, and the batch keeps its warnings for `GET` (`TestImportAuditRejectedConventionRowsAreFlaggedAndNeverCommitted`, CSV and QIF; the CSV case now commits 2 rows, balance −1246.50) |
| Partial-commit recovery differs from the plan. A reconciliation-skipped row stays skipped after a retry **with** override. A batch whose only row was skipped reports `committed`. A failed row cannot be patched in a `partially_committed` or `failed` batch | P3, [#301](https://github.com/sergeyfarin/rekenraam/issues/301). Retry with `reconciliation_override:true`: still 1 committed, 1 skipped, balance unchanged. One-row skipped batch: `status committed`, `committed_count 0`; retry `409 not in an open state`. Patch of the failed row: `409` | **Fixed** 2026-10-18 (option a, the batch resumes): held rows stay pending, failed rows are retried, `PATCH` and commit accept `partially_committed` and `failed` batches, and a batch is `committed` only with nothing pending or failed. The retry with override posts the held row once (balance −34.50 = −4.50 − 20.00 − 10.00); the one-row held batch reports `partially_committed`, then `committed` (`TestImportAuditHeldAndFailedRowsResumeInTheirBatch`). Re-upload still recovers too |
| CSV identity is scoped by profile, not account. An identical row in two accounts' statements under one profile stages as `duplicate` in the second | P3, [#302](https://github.com/sergeyfarin/rekenraam/issues/302). `Bank fee −2,50` committed from the checking statement; the savings statement's row staged `duplicate`, so it is skipped by default | **Fixed** 2026-10-18: CSV and QIF bank identities are scoped to the account the row posts to, checked at review and commit (`TestImportAuditIdenticalRowsInTwoAccountsStatementsCommitToEach`) |
| The CSV adapter gives no warning for all-ambiguous or conflicting dates when no layout is set (QIF does) | P3, [#303](https://github.com/sergeyfarin/rekenraam/issues/303). An API profile without `date_layout` staged `05/01/2026` as 2026-05-01 with no warning. The UI always writes a layout, so this path is API-only | **Confirmed defect**, filed. The `docs/implemented.md` claim is corrected |
| `docs/implemented.md` described decimal detection as per amount and the date warning as covering both formats | Decimal detection is whole-file (`detectDecimalSeparatorAcrossValues`), and the date warning is QIF-only | **Corrected** in the same change |

## Parser inventory

The sweep searched for the pattern across `backend/internal` and the import
frontend (`canonicalDecimal`, `parseDecimalAmount`, `parseFlexibleDate`,
`parseQIFDate`, `ParseFloat`, `Number(`, `replace(/,/g`). It did not rely on a
list of named copies. `ParseFloat` has no call site in import code. Frontend
`Number(` calls convert only ids.

| Parser | Input it sees | Convention | Evidence | Disposition |
|---|---|---|---|---|
| `canonicalDecimal` (`app/import_locale.go`) | Every CSV and QIF amount and split, and CSV investment gross/tax | Declared separator, else file-wide detection, else per value | `TestCanonicalDecimal_*`, `TestDetectDecimalSeparatorAcrossValues`, `TestCSVAdapterUsesTheWholeFileToSettleAnAmbiguousDecimalSeparator`, `TestParseQIF_DecimalCommaAmountsAreNotHundredfold`, matrix test below | Verified |
| `parseDecimalAmount` → `canonicalDecimal(raw, 0)` (`app/import_service.go`) | Text with no file-level convention: Trading 212 values (short, source correction, invest), crypto CSV's dot-only cells, owner-entered dividend review and CSV investment overrides | Per value | `TestParseDecimalAmount` | Verified. Since #300 staged CSV/QIF text no longer reaches it at commit |
| `parseCanonicalAmount` (`app/import_service.go`) | Staged amount and splits at commit (`buildTransactionSpec`), CSV investment cash | Canonical only; anything else refused | `TestBuildTransactionSpec_RefusesNonCanonicalStagedText`, `TestImportAuditRejectedConventionRowsAreFlaggedAndNeverCommitted` | **Fixed** (#300) |
| `parsePositiveCoefficient` / `parsePositiveDecimalAmount` (`app/import_trading212_invest.go`) | Trading 212 `json.Number` text (quantity, net value) | Dot decimal; positive only | `TestCommitImportBatch_BuyOrderFillCreatesInstrumentHoldingAndLot`, `TestFetchOrdersParsesRealShape` | Verified. Exponent forms are an adapter-only gap (#136/#145/#256 boundary) |
| Crypto CSV (`app/import_crypto_csv.go`) | Fixed interchange columns | UTF-8 only; `^[0-9]+(\.[0-9]+)?$`; ISO dates; scale ceilings 24/12 | Shape enforced before `parseDecimalAmount` | Verified by code reading; deliberate fixed format |
| `detectDateOrder` + `parseFlexibleDate` | CSV and QIF dates | Declared layout, else file-wide detection, else MDY | `TestDetectDateOrder`, `TestParseFlexibleDate*`, `TestParseQIF_EUDatesDetectedFromTheFile`, `TestParseQIF_AmbiguousDatesWarnAndKeepTheUSDefault`, `TestQIFAdapterParse_ProfileDateLayoutOverridesDetection` | Verified. CSV warning gap #303; commit fallback fixed (#300) |
| `parseCanonicalDate` (replaced `parseQIFDate`, `dateOrderAuto`) | Staged date at commit | ISO only; anything else refused | `TestBuildTransactionSpec_RefusesNonCanonicalStagedText` | **Fixed** (#300) |
| `decodeImportText` (`app/import_text_encoding.go`) | CSV and QIF bytes (not crypto) | Explicit, then BOM, then valid UTF-8, then chardet ≥ 50 % | `TestDecodeImportText_*`, `TestQIFAdapterParse_Windows1251Cyrillic`, `TestCSVAdapterParse_LegacyEncodingUsesSameDecoderAsQIF`, matrix test below | Verified |
| Frontend `parseDecimalAmount` (`lib/money/amount.ts`) | Review working copies: CSV investment gross/tax, dividend review, crypto preview | Canonical source read as `en`; owner input in the active locale | `csv-investment.test.ts`, `csv-investment-income.spec.ts`, `import-dividend-review.spec.ts` | Verified |

## Acceptance evidence by scope item

### Formats, encodings and decimal conventions

`TestImportAuditEncodingAndDecimalMatrixCommitsHandDerivedAmounts` uploads each
file, checks the staged dates and decoded payees, commits it to its own account
and reads the balance. The hand sums:

| File | Encoding (reported) | Convention | Rows | Expected balance | Observed |
|---|---|---|---|---|---|
| CSV, BOM | `utf-8/bom` | Detected DMY (13/02) and comma (`1.234,56`) | −1234.56, −2.05 | −1236.61 | Equal |
| CSV, Windows-1252 with `é ä ü €` | `windows-1252/detected` | Detected DMY and comma; `-3.000` is 3000 because `10,10` settles the file | −10.10, −3000, −0.95 | −3011.05 | Equal |
| CSV, Windows-1252 selected | `windows-1252/selected` | Declared DMY on all-ambiguous dates; declared comma makes `1.000` 1000 | −10.10, +1000 | 989.90 | Equal |
| CSV, Latin-1 selected | `iso-8859-1/selected` | Declared DMY (05/01 is 5 January) and comma | −7.5 | −7.50 | Equal |
| QIF, Latin-1 selected | `iso-8859-1/selected` | Dotted dates (15.06 proves DMY), comma (`1.234,56`) makes `-1.000` −1000 | −1234.56, −1000 | −2234.56 | Equal |
| QIF, BOM | `utf-8/bom` | 07/15 proves MDY; `1,234.56` makes `-1,000` −1000 | +1234.56, −1000 | 234.56 | Equal |

Every case staged with no warnings. Auto-detection of a legacy encoding needs
chardet confidence ≥ 50 %. Below that, the upload is refused with a request to
choose the encoding (`TestDecodeImportText_AmbiguousLegacyTextRequiresExplicitEncoding`).
Accepted limit: a short Latin-1 file usually needs an explicit selection,
which the import screen offers.

### Profiles and rules

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| Profile create, list, update, delete; deletion keeps history | Plan, Slice 2 | `TestCSVProfileCreateListAndUploadAcceptance`, `TestImportProfileUpdateAndDeletePreserveHistoricalBatch`, `TestUpdateImportProfileOmittedConfigPreservesMapping` | Verified |
| Two banks' layouts with no code change | Slice 2 acceptance | `TestCSVAdapterTwoBankLayoutsUseProfileDataWithoutCodeChanges`; browser `csv-import.spec.ts` | Verified |
| Declared layout and separator win over detection | Profile is authoritative | `TestQIFAdapterParse_ProfileDateLayoutOverridesDetection`, `…DecimalSeparatorOverridesDetection`, matrix rows 3–4 | Verified at staging; at commit a rejected row is refused (#300, fixed) |
| Rules apply: first enabled match wins, in order; a transfer hint never gets a category | Plan, Slice 2 rules | `TestImportRules_FirstContainsMatchIsVisibleAndCommitsThroughLedger`, `TestImportRules_CRUDIsOrderedAndAudited`, `TestApplyImportRules_DescriptionMatchAndTransferSafety` | Verified |
| Rule re-apply over an existing batch or committed rows | The plan excludes retroactive application | Rules run only at staging (`stageParseResult`) | **Accepted limit** (by design) |

### Stage → review → commit, identity, retry

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| More than 200 rows, paged review, real commit | 250 rows; one left unresolved fails; total of the rest is 31377.50 − 8.01 = 31369.49 | `TestImportAuditLargeBatchPartialCommitAndRepeatImportAreIdempotent`: 250 rows across 3 pages, `partially_committed`, 249/1, balance −31369.49 | Verified |
| Raw source evidence retained | Every staged row keeps its raw fields and source file | Same test: `raw._source_file` on all 250 rows | Verified |
| Repeat import after owner corrections | Row 1 edited −1.01 → −1.11 and row 2 voided. Re-import offers only the never-committed row; corrections survive. Expected −31369.49 − 0.10 + 2.01 − 8.01 = −31375.59 | Same test: 249 `duplicate` + 1 `new`; balance −31375.59; a third import is 250 × `duplicate` and commits nothing | Verified |
| Re-import of a byte-identical file | No-op | `TestStartImport_ReimportingSameFileFlagsDuplicatesAndSkipsOnCommit` | Verified |
| Overlapping or renamed QIF re-export | Already-committed rows are `duplicate` | Was 3 × `new` / 2 × `new`; now `TestImportAuditQIFReexportIdentityIgnoresFileNameAndPosition`: 1 `new` + 2 `duplicate` / 2 × `duplicate`, balance −35.00 | **Fixed** (#299) |
| Identical rows within one file | Kept as separate rows; later identical rows within a batch need attention | `TestParseQIF_FingerprintsAreUnique`, `TestStageParseResult_ReStagingSameBatchFlagsNeedsAttentionNotDuplicateNew`, `TestCSVInvestmentNoReferenceIdentityKeepsDistinctPayersAndOccurrences` | Verified |
| Same row in two accounts' CSV statements | Committed once per account | Second statement's row is `duplicate` | **Defect #302**, fixed 2026-10-18 |
| Concurrent commits of one batch | One winner; no double post | `TestCommitImportBatch_ConcurrentPlainRowCommitsKeepOneWinner`, `TestCommitImportBatch_ConcurrentTrading212CommitsKeepTheCommittedRow` | Verified |
| Row and identity atomic | A failed identity write rolls back the transaction | `TestCommitImportedTransaction_RollsBackLedgerWhenIdentityWriteFails`, `TestImportIdentityOrdersTwoTransactionsAndRollsBackPartialEffects`, `TestCSVInvestmentIdentityFailureRollsBackJournalAndFacts` | Verified |
| Partial commit, then fix and commit again | Plan: fix resolutions or override, commit again | Not possible in the batch; re-upload works (large-batch test) | **Defect #301**, fixed 2026-10-18 (`TestImportAuditHeldAndFailedRowsResumeInTheirBatch`) |
| Reconciliation crossing | Skipped without override, committed with it | First commit verified (`TestCommitImportBatch_InvestmentBuyCrossingReconciledPeriodRequiresOverride`, `TestCSVInvestmentImportAPIReconciliationConflictLeavesBatchReviewable`); retry with override does not commit the skipped row | Verified on first commit; **#301** on retry, fixed 2026-10-18: the row is held pending and the retry commits it |
| Discard | Mid-review discard leaves the ledger untouched; a committed batch cannot be discarded | `TestDiscardImportBatch_MidReviewLeavesLedgerUntouchedAndBlocksFurtherMutation`, `TestDiscardImportBatch_AfterCommitRejected`, `TestCommitImportBatchRefusesRowsOfBatchDiscardedMidCommit` | Verified |
| Auth and CSRF | Every batch endpoint gated | `TestImportBatchEndpoints_RequireAuthentication`, `TestImportBatchMutations_RequireCSRFToken`, `TestStartImport_RequiresCSRFToken` | Verified |

### Investment CSV

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| An invalid investment row is never posted as cash | Contract: named writer or refusal, never the cash fallback | `commitCSVInvestmentRow` returns handled for every CSV investment row, so the plain-cash path is unreachable. A pre-pass validates every pending row before any commit. `TestCSVInvestmentIncomeRefusesUnresolvedAndMismatchedRowsWithoutWrites`, `TestCSVInvestmentPreviewAndReconciliationRefusalLeaveNoWrites` | Verified (code path plus tests) |
| Rejected source decimals are not reinterpreted | `CashValid` false refuses; gross/tax rejected become empty | `parseCSVInvestmentRow`, `TestCSVInvestmentAutoDecimalDetectionIncludesGrossAndTax` | Verified |
| Review resolves kind/instrument; commit, correct, export, dedupe | Contract | `TestCSVInvestmentIncomeCommitsCorrectsExportsAndDeduplicates`, `TestCSVInvestmentIncomeReviewCanResolveUnknownKindAndInstrument`, `TestCSVInvestmentImportAPIReviewCommitActivityAndExport`; browser `csv-investment-income.spec.ts` | Verified |

### Trading 212 (stubbed provider)

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| Paging, page budget continuation, cursor boundaries, `Retry-After`, absolute next path refused | Trading 212 plan | `TestFetchPaginatesAcrossPages`, `TestFetchReportsHasMoreWhenPageBudgetExhausted`, `TestFetchIncludesMovementsAtExactCursorTimestamp`, `TestFetchHonorsRetryAfter`, `TestFetchRefusesAbsoluteNextPagePath`, `TestStartOnlineImport_ContinuesPastPageBudgetWithoutTruncatingOrDuplicating` | Verified |
| Durable fetch: lease reclaim, terminal versus retryable, one fetch in flight | ADR 0010 | `TestImportFetchWork_RestartReclaimsExpiredLease`, `TestProcessTrading212FetchWork_*`, `TestStartOnlineImport_ConcurrentStartsRaceSafely` | Verified |
| Source identity on re-fetch; changed fills held for correction; local resolution is not a revision | Plan, T-122 | `TestTrading212Adapter_FingerprintsStableAcrossReparse`, `TestRefreshImportConnection_IncrementalOnlyNewMovementsAndSkipsCommitted`, `TestChangedTrading212FillIsHeldForSourceCorrection`, `TestTrading212FillSourceComparisonIgnoresLocalResolution`; browser `import-gain-review.spec.ts` (revised buy older than the cursor) | Verified |
| Revisions keep identity through correction | Checklist item 20 | `TestReplaceImportedBuyKeepsSourceIdentityAndDedupesTheFill`, `TestReplaceImportedSaleKeepsSourceIdentityAndDedupesTheFill`, `TestCorrectTrading212SalePreservesSpecificElectionWithoutGuessingQuantityChanges` | Verified |
| Unsupported or non-TRADE fills never post or use the cash fallback | Checklist item 21 | `TestImportUnsupportedFillCannotPost`, `TestUnsupportedSaleFillCannotUseCashFallback`, `TestUnsupportedFillCannotCorrectAcceptedBuyOrSale`; browser `import-short-link.spec.ts` | Verified |
| Dividend review and repair (#242) | Known payer kept; missing facts pending; repair keeps payment and identity | `TestTrading212DividendOwnerReviewRetainsKnownPayerWithoutDefaults`, `TestImportedDividendRepair*`, `TestReviewedDividend*`; browser `import-dividend-review.spec.ts` | Verified |
| Order-fill cash fallback for expected gaps (no cash account, insufficient lots) | Documented Slice 4b behavior | `TestCommitImportBatch_OrderFillWithoutCashAccountFallsBackToGenericCommit`, `TestCommitImportBatch_InsufficientLotFallbackCleansCreatedInvestmentSetup`; an unexpected error fails the row instead (`…UnexpectedInvestmentErrorFailsRowInsteadOfFallingBack`) | **Accepted limit**, documented |

### Upload limits and error copy

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| The exact limit works; one byte over is refused with no durable change | #228 | `TestImportUploadFileSizeBoundary`, `TestImportUploadMultipartResourceLimits` | Verified |
| Translated size error, recovery, 390 px | Every locale | `import-upload-limit.spec.ts`: 12 cases (QIF and CSV × en, es, fr, nl, de, ru) | Verified |

### Excluded, unbuilt

XLSX (Slice 3), OFX/QFX (Slice 4) and a dedicated batch rollback (Slice 3) are
not built. Rollback today is void or soft delete of the batch's transactions
through the ordinary lifecycle endpoints. **Accepted limits**, as the issue
scoped them.

## Maintainability notes

- **Two decimal policies for one staged value.** Adapters parse with the
  file's convention (`canonicalDecimal(raw, sep)`). Commit re-parses the same
  value with `parseDecimalAmount` → `canonicalDecimal(raw, 0)`, which is safe
  only because staged text is normally canonical. The CSV investment path
  encodes the opposite rule in a comment and the `CashValid` flag. #300 is
  where the two diverge, and this is the T-36/T-45/T-47 class. Since #300 a
  rejected value is recorded on the row (`parse_errors`) and commit parses
  staged text as canonical only, so there is one policy.
- **Two dedupe identity builders.** `buildQIFFingerprint` (position and file
  name) and the CSV builder (per-content occurrence and profile id) implement
  the plan's one contract differently. Trading 212 and crypto CSV each have
  their own as well. #299 and #302 are where they drift; #299 has since aligned QIF with the CSV builder (`qifContentFingerprint`), and #302 scopes both to the account at commit (`importCommitKey`).
- **Commit routing is a long ordered chain in one function.**
  `commitImportBatch` (`backend/internal/app/import_service.go`, about 300
  lines) decides unsupported fill, source change, duplicate, identity,
  investment writer, review hold, reconciliation skip and plain commit by
  order of `if` blocks, and assigns terminal states (`skipped`, `failed`) at
  several points. The retry semantics in #301 follow from which branches are
  terminal, and that is hard to see or test as a table. #301 has since made
  the reconciliation branch a pending hold and failed rows retryable.
- **Parse warnings live only in the start response.** They are index-addressed
  `ParseWarning`s, never persisted, and the online fetch path merges them per
  chunk with offset arithmetic (`import_fetch_worker.go`). Any reviewable state
  derived from them has to be rebuilt elsewhere.

## Validation

- Focused backend suites:
  `./scripts/go-backend.sh test ./internal/app ./internal/api ./internal/db ./internal/onlinesource/... -run 'Import|QIF|Qif|CSV|Csv|Trading212|Decimal|DateOrder|FlexibleDate|DecodeImportText|Fetch|Dividend|…'`
  passed in `app`, `api`, `db` and `onlinesource/trading212`, including the two
  new audit tests. The throwaway probe was removed before the passing run.
- Browser, on a rebuilt integrated binary:
  `pnpm --dir e2e test csv-import.spec.ts csv-investment-income.spec.ts import-dividend-review.spec.ts import-gain-review.spec.ts import-short-link.spec.ts import-upload-limit.spec.ts`:
  21 passed.
- Per the evidence contract, the full backend, race and browser gates were not
  run (#280 owns them).
