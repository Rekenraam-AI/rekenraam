# Reference data, accounts, hierarchy and effective dates audit — 2026-10-17

**Status:** sub-audit
[#269](https://github.com/sergeyfarin/rekenraam/issues/269) (area 2) of
[#260](https://github.com/sergeyfarin/rekenraam/issues/260). It is complete,
with five findings filed and left open per the evidence contract:
[#291](https://github.com/sergeyfarin/rekenraam/issues/291) and
[#292](https://github.com/sergeyfarin/rekenraam/issues/292) (P2), and
[#293](https://github.com/sergeyfarin/rekenraam/issues/293),
[#294](https://github.com/sergeyfarin/rekenraam/issues/294) and
[#295](https://github.com/sergeyfarin/rekenraam/issues/295) (P3). None is a
P0/P1 integrity defect: no finding changes a posted amount or a balance. This
change adds evidence only, and one stale plan claim is corrected. No
application code changed.

## Scope and method

The audited SHA is `aa14477e`, with a clean working tree. Evidence comes from
the real HTTP handler (`newSetupTestHandler`) over a temporary SQLite book with a
synthetic owner, and from a rebuilt integrated binary driven by Chromium at
390 px. No provider is involved.

Expected results are derived from these documents, not from the code under test:

- [account hierarchy](../design/account-hierarchy.md)
- [accounts system design](../design/accounts-system-design.md), for version
  selection, the structural lock, tree rules and the lifecycle
- [categories design](../design/categories-design.md)
- [holding-account opened date](../design/holding-account-opened-date.md)
- ADR 0007 and ADR 0009

The selection rule used throughout: an as-of read takes, per account, the
version with the greatest `effective_from` on or before the date, then the
greatest `version_seq`. Category rollups use the tree as of the read's end date.

Exploratory probes ran as a throwaway test against the same handler, with the
server log enabled, and were then removed. Every finding below quotes its
observed status and body. The scenarios with no existing consumer are now kept
as named tests:

- [`reference_data_audit_test.go`](../../backend/internal/api/reference_data_audit_test.go)
- [`reference-data-management.spec.ts`](../../e2e/playwright/reference-data-management.spec.ts)

## Findings and disposition

| Finding | Severity and evidence | Disposition |
|---|---|---|
| `/accounts` ignores every category rule. A protected built-in category can be renamed, closed and **hard-deleted**, and any account PATCH strips `metadata.category`, which makes a used category vanish from the category API | P2, [#291](https://github.com/sergeyfarin/rekenraam/issues/291). `PATCH`/`close`/`DELETE /accounts/{expense_other}` returned `200`/`200`/`204`, and `GET /categories/{id}` then returned `404`. A user category with postings got the same `404` after an account rename. API-only, because the accounts screen hides categories | **Confirmed defect**, filed |
| A close dated before existing postings is accepted. Those transactions can then be neither edited nor voided, and void returns `500` | P2, [#292](https://github.com/sergeyfarin/rekenraam/issues/292). Close at 2026-02-01 over a 2026-03-15 posting: `200`. Edit: `400`. Void: `500 INTERNAL_ERROR` (trigger `posting account is not eligible on entry date`). Reopening today does not repair it. In the same window, a posting dated **on** `closed_on` is refused, although the design defines `closed_on` as the day after which postings stop | **Confirmed defect**, filed |
| Backdated versions. A write that a later version hides answers as if it were current; parent and cycle checks look only at today's tree | P3, [#293](https://github.com/sergeyfarin/rekenraam/issues/293). A close at 2026-02-01 after a reopen today answered `status "closed"`, but the account stayed active. A backdated re-parent of containers produced X→Y→X in `account-balances?as_of=2026-04-01` | **Confirmed defect**, filed. The parent check also sits before the write transaction (seam class): seen in code, not triggered |
| Archiving a parent account or an institution leaves active dependent accounts. Once those have postings, even a rename is refused | P3, [#294](https://github.com/sergeyfarin/rekenraam/issues/294). Close/archive of the parent and archive of the institution: `200`. The child's rename: `400 "account parent is invalid"` / `"account reference is invalid"`; dropping the link: `409` structure lock | **Confirmed defect**, filed. Workaround: restore, edit, archive again |
| A user account can be created under a system account | P3, [#295](https://github.com/sergeyfarin/rekenraam/issues/295). `POST /accounts` with `parent_account_id` = `transfer_clearing`: `201`. The category API already refuses a system parent | **Confirmed defect**, filed |
| The ledger-core plan, marked implemented, lists `GET /payees/{id}/versions` | No such route exists. Payee versions are stored but not listed | **Corrected** in [the plan](../plans/transaction-ledger-core-plan.md) |

## Acceptance evidence by scope item

### Accounts

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| Create, rename and re-parent with effective dates; append-only history | Each change appends a version and an audit event | `TestUpdateAccountCreatesAppendOnlyVersionAndClearsOptionalFields`, `TestAccountParentDerivesInstitutionOnCreateAndUpdate` | Verified |
| Current versus as-of reads after a version change | A posting of 1000.00 on 2025-11-15 and 4.50 out on 2025-11-20 give 995.50 throughout. The name is `Savings` as of 2025-11-20 and 2025-11-30, and `Reserve` as of 2025-12-01 and 2026-01-01, after a rename effective 2025-12-01. The current read is `Reserve`, with two versions | `TestHistoricalReadsFollowAccountAndCategoryVersions`: all values as derived | Verified |
| Close/reopen/archive/restore lifecycle | Archive only from closed; restore returns to closed; close no earlier than the opened date | `TestAccountLifecycleRequiresCloseBeforeArchiveAndRestoresClosed`, `TestAccountCloseRejectsDateBeforeOpenedOn`; `archive` while active gives `409 CONFLICT` | Verified, except close dated before postings (#292) |
| Class/kind axis | Kind belongs to the class and is user-assignable; equity is reserved for system workflows | `TestCreateAccountSupportsUserFacingGroupedKinds`; user equity gives `400 VALIDATION_FAILED` | Verified |
| Budget-treatment axis | Separate versioned table; only user asset/liability accounts qualify | `TestBudgetMonthUsesExactPostedActualsAndAccountTreatment`. Probe: `transfer_clearing` and a category gave `404 NOT_FOUND`, a user checking account `200` | Verified |
| Cycle refusal | Self-parenting and descendant-parenting refused | `TestAccountParentRulesRejectCyclesAndClassMismatch`; in the refusal table, `400 VALIDATION_FAILED` for both | Verified for today's tree. As-of trees: #293 |
| Structural lock after postings | Kind, parent, institution, currency, precision, posting behavior and opened date are locked; descriptive fields stay editable | `TestUpdateAccountLocksStructureAfterPostingsExistButAllowsDescriptionEdits`, `TestUpdateAccountRejectsCurrencyChangeAfterPostingsExist`; `409 CONFLICT` in the refusal table | Verified |
| Protected system roles cannot be edited, closed or deleted | `CONFLICT` | `TestSystemAccountSetupCreatesRolesAndProtectsThem`; refusal table `409 CONFLICT` for edit, close and delete of `transfer_clearing` | Verified. Misuse as a parent: #295. Category bypass: #291 |
| Ordinary postings to system accounts | No document forbids them. Opening-balance entries use `opening_balance` | Direct postings to `transfer_clearing` and `opening_balance` gave `201` | **Not verified** as a rule. Recorded only, since no source of truth says either way |

### Commodities, opened dates, categories, payees, tags and institutions

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| Commodity precision and identity (ADR 0009) | Kind ceilings of 12/24; scales immutable once used | `TestCommodityPrecisionCeilingAllowsCryptoOnlyThroughTwentyFour`, `TestCommodityVersionsAreAppendOnly`, `TestPostingCommodityScaleTriggerUsesEntryDate`. No API changes a commodity's `standard_scale`/`max_quantity_scale`. An instrument's `quantity_scale` must stay within the commodity maximum and only seeds new holding accounts. An account's `quantity_scale_override` is structurally locked (`409 CONFLICT`) and refused above the commodity maximum (`400 VALIDATION_FAILED`) | Verified. A precision change is refused by absence or by the lock |
| Holding-account opened date for backdated entries | Import-created holding accounts open at `0001-01-01`; hand-created ones keep the owner's date | `TestCommitImportBatch_LaterImportCarryingAnEarlierTradeStillCommits`, `TestCommitImportBatch_BackdatedFirstTradeNeedsNoInstrumentBackdating`, `TestCreateTransaction_BeforeAccountOpenedReportsTheDateNotTheAccount` | Verified |
| Category CRUD, hierarchy and archive | Same-type parents; immutable type; disable refused while active children exist; unique sibling names; delete only when unused | `TestCategoryCreateUpdateDisableRestoreAndDeleteUnused`, `TestCategoryValidationRejectsParentMismatchCycleAndAccountFields`, `TestCategoryDeleteRejectsCategoryUsedByPosting`; the refusal table's category rows | Verified through the category API. Through `/accounts`: #291 |
| Category move over time | 4.50 on 2025-11-20 rolls up to Food for a range ending 2025-11-30, and to Personal for a range ending 2025-12-31, after a move effective 2025-12-01; Coffee's own total is 4.50 in both | `TestHistoricalReadsFollowAccountAndCategoryVersions` | Verified |
| Genesis date for categories (T-43) | Any category accepts earlier history | `TestUserCreatedCategoryOpensAtGenesisSoItTakesEarlierHistory` | Verified |
| Built-in labels by stable key (ADR 0007) | Setup stores keys, not English names; the frontend has a label for every seeded key | `TestCategoriesSetupSeedsBuiltInsAndStarterSuggestionsWithoutEnglishCanonicalNames`. A hand diff of the seeded keys in `app/categories.go` against `builtinCategoryLabel` found 55 = 55, identical | Verified by observed run. **Accepted limit:** no automated parity check (maintainability note) |
| Payees: CRUD, rename effect on history | A rename keeps the transaction's `payee_name` snapshot (ledger-core plan), while reports name the payee as it is now | `TestPayeeCreateUpdateArchiveRestoreAndRead`. Probe: after renaming `Shop` → `Shop Renamed` and archiving it, `GET /transactions?payee_id=` still finds the row, which shows `payee_name "Shop"`. A duplicate gave `409 CONFLICT` | Verified, as designed |
| Payee merge | Not implemented; deferred in the roadmap ("duplicate payee merge") | No endpoint | **Accepted/deferred limit** |
| Old unlinked payees | Rows with a free-text `payee_name` and no `payee_id` are reported in the unattributed group | Recorded as 🟦 in `docs/implemented.md` (T-50) and in the reports plan | **Accepted limit**, no new evidence needed |
| Tags: CRUD, rename and kind change everywhere | A tag is a mutable record; associations hold ids | `TestCreateTagPersistsAndAudits`, `TestUpdateTagAndFilters`, `TestArchiveAndRestoreTag`, `TestTagActiveNameKindMustBeUnique`; duplicate gives `409 CONFLICT` | Verified |
| Institutions: API behavior | CRUD, archive/restore, append-only versions; delete refused while referenced | `TestUpdateInstitutionCreatesAppendOnlyVersion`, `TestArchiveInstitutionHidesFromDefaultListAndRestoreShowsIt`, `TestDeleteInstitutionRejectsAccountReferences` | Verified. Archive while in use: #294 |
| Institution management UI | Minimal (ledger: ✅ API / 🟦 UI); quick create in the account editor | No workflow is blocked: accounts can be created with or without an institution. The institution editor is reachable from the accounts screen | **Accepted limit** |

### Browser journeys at 390 px

| Journey | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| Accounts | Create, rename and close at 390 px without horizontal scroll; the rename appends a version | `reference-data-management.spec.ts`: two versions `[renamed, original]`, status `closed`, `scrollWidth ≤ 390` | Verified |
| Categories | Create under a starter parent and archive at 390 px | Same spec: the parent is `expense_personal`, the status becomes `archived`, no horizontal scroll | Verified |

## Refusal codes

`TestReferenceDataRefusalsCarryStableErrorCodes` asserts the status **and** the
envelope code for 21 refusals:

- **`400 VALIDATION_FAILED`:** account cycle, self-parent, parent of another
  class, user equity account, future effective date, precision above the
  commodity maximum, category cycle, category type change.
- **`409 CONFLICT`:** system account edit, close and delete; structure lock;
  precision lock; delete of a used account; archive of an active account;
  duplicate category sibling; disable of a category with an active child;
  built-in category edit; duplicate payee; duplicate tag.
- **`404 NOT_FOUND`:** a system account addressed as a category.

## Maintainability notes

- **Two definitions of "is a category".** `categoryAccountPredicate`
  (`backend/internal/db/categories.go`) requires `metadata.category.type`. The
  spending and category-totals reads (`backend/internal/db/reports.go`,
  `ledger.go`) match on `account_kind = account_class` without metadata. #291
  shows them diverging: a category stripped of metadata still counts in
  spending, but the category API no longer finds it. The category rules live
  only in `CategoryService`, while the same rows are writable through
  `AccountService`.
- **The as-of version pick is hand-written in many places.**
  `ORDER BY effective_from DESC, version_seq DESC LIMIT 1` over
  `account_versions` appears in `transactions_rules.go`, `reports.go`,
  `ledger.go`, `exports.go`, `transactions_read.go` and `investments.go`, besides
  the `current_*` views and `LedgerAccountVersionsThrough`. Any change to the
  backdated-version contract (#293) has to edit each of them.
- **Parent rules are checked outside the write transaction, the posting lock
  inside it.** `accountSpec` (`backend/internal/app/accounts.go`) validates the
  parent, the archived status and cycles before `UpdateAccount` begins. The
  structural lock is re-asked inside the transaction (T-100). The two
  invariants on the same write follow different seam disciplines.
- **Built-in category keys are listed twice and nothing checks parity.**
  The seed list in `backend/internal/app/categories.go` and the switch in
  `frontend/src/lib/categories/category-labels.ts` matched 55/55 only by hand.
  ADR 0007 asks for an automated check.
- **The API test handler mixes clocks.** `newSetupTestHandler` pins the
  category, currency, budget and forecast services to 2026-01-01, while the
  account and transaction services use the wall clock. A cross-service as-of
  test has to place its dates before 2026-01-01 to change a category on a
  chosen date (`TestHistoricalReadsFollowAccountAndCategoryVersions` does this).

## Validation

- Focused backend suites:
  `./scripts/go-backend.sh test ./internal/api ./internal/app ./internal/db -run 'Account|Categor|Payee|Tag|Institution|Currenc|Commodit|Genesis|LaterImportCarryingAnEarlierTrade|BackdatedFirstTrade|Budget|CurrentVersionViews|Precision'`
  passed, including the two new tests.
- `pnpm --dir e2e test reference-data-management.spec.ts` passed both journeys
  on a rebuilt integrated binary.
- Per the evidence contract, the full backend, race and browser gates were not
  run (#280 owns them). `./scripts/check-post-merge.sh HEAD` covers this change.
