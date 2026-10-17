# Daily ledger, posting dates, lifecycle and re-reconciliation audit — 2026-10-17

**Status:** sub-audit
[#270](https://github.com/sergeyfarin/rekenraam/issues/270) (area 3) of
[#260](https://github.com/sergeyfarin/rekenraam/issues/260). It is complete,
with three findings filed separately rather than fixed in the audit change, per
the evidence contract:
[#296](https://github.com/sergeyfarin/rekenraam/issues/296) (P2), and
[#297](https://github.com/sergeyfarin/rekenraam/issues/297) and
[#298](https://github.com/sergeyfarin/rekenraam/issues/298) (P3). None is a
P0/P1 integrity defect. No finding computes a balance wrongly or bypasses a
checkpoint without an override. The audit change adds evidence only and
corrects two capability claims in `docs/implemented.md`.

## Scope and method

The audited SHA is `7476ca0b`, with a clean working tree. Evidence comes from
the real HTTP handler (`newSetupTestHandler`) over a temporary SQLite book with a
synthetic owner, and from a rebuilt integrated binary driven by Chromium at
390 px. No provider is involved (ADR 0017).

Expected figures are hand-derived from the steps and these contracts, not from
the code under test:

- `docs/conventions.md`: lifecycle, same-day order, posting dates (#187), the
  reconciliation guard and re-reconciliation (#240)
- [ledger core plan](../plans/transaction-ledger-core-plan.md): the
  shared workspace, approval, and reconciliation checkpoints
- the [reference book](reference-book-audit-2026-10-17.md) register (#266),
  for the same- and cross-currency in-transit balances

Two first-draft figures in the new journey test were wrong: groceries after the
split (140.00, not 120.00) and the edited day's balance. Both were corrected
from the step amounts before anything was compared with the app.

Kept evidence:

- [`ledger_audit_test.go`](../../backend/internal/api/ledger_audit_test.go)
  `TestLedgerAuditJourneyBalancesOnBothDateAxesThroughLifecycleAndReReconciliation`:
  a small EUR book whose balances are compared as reduced `big.Rat` strings
  (no float).
- [`ledger-phone-journey.spec.ts`](../../e2e/playwright/ledger-phone-journey.spec.ts):
  entry and reconciliation at 390 px.

The cross-request probes behind #296 and #297 ran as a throwaway test against
the same handler and were then removed. Their observed responses are quoted in
the findings.

## Findings and disposition

| Finding | Severity and evidence | Disposition |
|---|---|---|
| Generic ledger writes are not bound to what the owner reviewed. A stale form's save overwrites a newer amount, and an override invalidates checkpoints the preview never named | P2, [#296](https://github.com/sergeyfarin/rekenraam/issues/296). (1) Tab B edits 10.00 → 20.00 (`200`). Tab A saves a description-only change from its stale form: `200`, and the balance returns from −20.00 to −10.00. (2) The edit preview names `[1]`. A later statement is then finished, and the override write answers `invalidated_checkpoint_ids: [1, 2]`. `PATCH /transactions/{id}` takes no client version, and `reconciliation_override` is a boolean | **Confirmed defect**, filed; **fixed** 2026-10-17: the edit requires `expected_version_id` and an override names its reviewed checkpoint ids (`RECONCILIATION_IMPACT_STALE`). The in-request seam is closed (T-94). The guard always runs on the current version, so a reconciled posting is never changed silently. Only an unreconciled amount can be lost, and only a later checkpoint can be invalidated unseen |
| A financial edit keeps the posting's `reconciled` status and `cleared_on` | P3, [#297](https://github.com/sergeyfarin/rekenraam/issues/297). Probe: after an override edit of a reconciled 10.00 posting to 15.00, the new version shows `reconciled` with the old `cleared_on 2026-05-12`. Journey: the edited 92.40 posting is offered `reconciled` and preselected in the next session. `cleanPosting` carries the status by line key whatever changed | **Confirmed defect**, filed. The checkpoint is invalidated as required, so nothing is locked. Only the trust claim on the edited row is wrong |
| On the entry-date basis, the transaction list matches `account_id` in any entry, not the entry that matched the date | P3, [#298](https://github.com/sergeyfarin/rekenraam/issues/298). `?after_date=2026-05-07&before_date=2026-05-07&date_basis=entry&account_id={savings}` returns the transfer, although savings posts on the 9th. `matchingEntryClause` pins category filters only | **Confirmed defect**, filed. A list shows an extra row. No balance or report total changes |
| The override-required refusal uses the generic `409 CONFLICT` code | `ErrReconciliationOverrideRequired` → `CONFLICT` (`api/transactions_errors.go`). The stale refusals have their own codes (`TRANSACTION_VERSION_STALE`, `RECONCILIATION_START_STALE`) | **Verified behavior**, recorded. The browser flows rely on the endpoint having no other conflict (see maintainability notes) |
| `docs/implemented.md` said the T-94 version guard means "a prepared edit cannot land on top of someone else's newer" version | True within one request only (#296) | **Corrected** in the same change. The reports drill-down row now names #298 as a known limit |

## Acceptance evidence

### Balances on both date axes

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| Opening balance, salary, expense, refund, three-way card split | Checking 1000 + 2500 − 82.40 + 12.40 = **3430.00**. Card **−120.00**. Groceries 82.40 − 12.40 + 45 + 25 = **140.00**. Dining **50.00** | Journey test: all exact | Verified |
| Same-currency transfer, transaction date 05-07, savings leg `posted_on` 05-09 | On the 7th and 8th: checking **2930.00**, savings **0**, `transfer_clearing` **500.00**. On the 9th: savings **500.00**, clearing **0** | Journey test: all exact. The response holds two entries, 05-07 and 05-09 | Verified |
| Transaction-date axis | A transaction-date range on the 7th, filtered to savings, holds the transfer | Journey test: one row, the transfer | Verified. The entry-date list over-matches (#298) |
| Cross-currency transfer (reference book T11): 2500.00 EUR on the 26th, 2750.00 USD on the 28th | 2750.00 USD in `transfer_clearing` on the 27th, 0 on the 28th. Commodity-trading legs stay on the transaction date | `TestReferenceBookProducesItsRegister`: all 24 balance figures exact, including the in-transit rows | Verified |
| Register running balance matches the as-of balance | After the reorder: payment row **2810.00**, then coffee **2805.50**. Day-end balance **2805.50** | Journey test: exact | Verified |

### Ordering

| Scenario | Expected | Observed / test | Disposition |
|---|---|---|---|
| Place before a same-day neighbour, in the journal and every shared register | Order changes everywhere both appear. No balance changes | Journey test (checking register order and balances). `TestPlaceTransactionAppliesInTheJournalAndEverySharedAccount`, browser `transactions-reorder.spec.ts` | Verified |
| "Only in this account" switch | That register alone is reordered | `TestPlaceTransactionKeepsAnAccountOnlyOrderItDoesNotContradict`; browser case *only in this account reorders one register and leaves the other* | Verified |
| Reorder across a reconciled same-day boundary | Override required, per account | `TestPlaceTransactionAcrossAReconciledBoundaryNeedsTheOverride`, `TestPostingMoveAcrossEarlierSameDayCheckpointRequiresOverride` | Verified |
| Move to the adjacent day (#187) | An edit of the posting date, under the guard | `TestMovePostingToDayOutOfAReconciledPeriodIsGuarded`; browser *reorder mode moves a posting past the day edge…* | Verified |

### Lifecycle

| Step (system draft, 15.00 dining from checking) | Expected today's checking balance | Observed | Disposition |
|---|---|---|---|
| Recurring draft generated | **2790.50**: a draft is outside the ledger | Exact | Verified |
| Void the draft | Refused (`409 CONFLICT`): a draft is posted or deleted, never voided | As expected | Verified |
| Post | **2775.50** | Exact | Verified |
| Hard delete of the posted transaction | Refused (`409 CONFLICT`) | As expected | Verified |
| Edit to 18.00 | **2772.50** | Exact | Verified |
| Void | **2790.50**, and the transaction is still readable as `voided` | Exact | Verified |
| Unvoid | **2772.50**: the edited amount returns | Exact | Verified |
| Soft delete | **2790.50**. The read is `404`, and the trash lists it | Exact | Verified |
| Restore | **2772.50**. `version_seq` is 5 (draft, post, edit, void, unvoid; the delete flag adds no version) | Exact | Verified: append-only, nothing hard-deleted |
| Metadata-only edit under an active checkpoint (description, and a category move of 82.40 to dining) | No override needed. The checkpoint stays active. Checking stays **2805.50**; groceries becomes 140 − 82.40 = **57.60** | Exact | Verified |
| Hard delete of a never-posted draft | Allowed; leaves an audited skipped occurrence | `TestDiscardEditedGeneratedDraftPreservesAuditAndSkippedIdentity` (T-81), not repeated | Verified by existing test |
| Approval | Metadata-only; the journal, order and reconciliation are kept | `TestApproveTransactionPreservesJournalBalancesOrderAndReconciliation`, `TestApproveDraftPreservesJournalWithoutPostingIt` | Verified by existing test |

### Reconciliation and re-reconciliation

| Scenario | Expected (independent) | Observed / test | Disposition |
|---|---|---|---|
| First statement through 05-10 at **2805.50** | Seven candidates (T1–T4, the transfer's checking leg, T7, T8). With all selected, the difference is 0 | Exact. The finish wrote checkpoint 1 | Verified |
| Back-dated amount edit 82.40 → 92.40 inside the period | Refused without override. Preview names checkpoint 1; the override write invalidates exactly `[1]`. Balance **2795.50** on the 10th, **3407.60** on the 4th | Refusal `409 CONFLICT`. Preview `[1]` equals write `[1]`. Balances exact | Verified (preview = write while nothing else changes; see #296) |
| Reconcile again after the invalidation | No starting checkpoint. Seven candidates. Every unchanged once-reconciled posting starts selected | Exact. The edited posting is preselected too (#297) | Verified, with #297 |
| Void the second checkpoint, then reconcile a third time | History: checkpoint 1 `invalidated` with reason "receipt was 92.40", checkpoint 2 `voided` with its reason, checkpoint 3 `active` at **2795.50** | Exact | Verified |
| Stale start refused at commit | Two sessions start from checkpoint 3; both preview a 0 difference (2795.50 − 5.00 = **2790.50**). One finishes; the other's finish is refused | `409 RECONCILIATION_START_STALE`, and no checkpoint is written | Verified |
| Preview equals write for create, edit and lifecycle | Each preview names the complete set its write invalidates | `TestCreatePreviewReportsEveryCheckpointCommitInvalidates`, `TestLifecycleReconciliationImpactNamesEveryCheckpointTheWriteInvalidates`, journey (edit) | Verified while the book does not change between the two calls. Not bound across that gap: #296 |

### Phone (390 px) journey

| Journey | Expected | Observed / test | Disposition |
|---|---|---|---|
| Entry in the account register | The form starts in the account. 250.00 − 12.40 = **237.60**. No horizontal scroll on the register or the open form | `ledger-phone-journey.spec.ts`: balance `23760`/2, `scrollWidth ≤ 390` at each step | Verified |
| Reconciliation on the reconcile screen | Both postings ticked, *Balanced*, finished. One active checkpoint at **237.60**. No horizontal scroll | Same spec | Verified |
| Existing 390 px specs | Workspace, reorder, posting dates, lifecycle guard, recovery, void checkpoint, trash restore, correction chain | 21 cases passed in the same run | Verified |

## Seam sweep

For every ledger write with a preview or prepare phase: which facts it reads
before the write transaction, and where they are rechecked.

| Operation | Facts read before `BeginTx` | Rechecked at commit | Evidence |
|---|---|---|---|
| Create, including corrections | Account posting rules and roles; checkpoint candidates | Account-rule dependency versions (`requireAccountRuleDependenciesTx`, T-100). Candidates re-resolved by `enforceCheckpointBoundaryTx` (T-94). Posting eligibility on the entry date (#292, in-tx) | `TestCreateTransactionRejectsACheckpointCreatedAfterValidation`, `TestCreateTransactionWithOverrideInvalidatesACheckpointCreatedAfterValidation`, `TestPreparedPostingIsRefusedAfterTheAccountBecomesAHolding`, `TestAccountStructureChangeIsRefusedWhenAPostingCommitsFirst` |
| Update | The current version, its diff-based candidates, account rules, voided state | `requireExpectedVersionTx` against the service's read; account dependencies; voided and deleted re-read; guard in-tx | `TestStaleMetadataEditCannotOverwriteANewerReconciledAmount`, `TestEveryLifecycleWriteRefusesASupersededVersion`, `TestWritePathsEnforceTheCheckpointBoundaryInTheRepository`. **Gap across requests: #296** |
| Post (draft promotion) | The draft and its spec, via two reads | Both reads bound to one version; promotion without a version refused | `TestDraftPromotionPreparedBeforeAnEditIsRefused`, `TestStaleDraftPromotionIsRefused`, `TestDraftPromotionWithoutItsSourceVersionIsRefused` |
| Void, unvoid, soft delete, restore | Status, deleted flag, stored positions | Expected version; status and `deleted_at` re-read in-tx; guard in-tx. The delete flag adds no version, but the write re-reads it and the candidates are the version's own positions | `TestEveryLifecycleWriteRefusesASupersededVersion`, lifecycle impact test |
| Investment-linked refusal (all generic paths) | `TransactionHasInvestmentLinks`, outside the write | Not rechecked | **Verified by construction.** A transaction becomes investment-linked only in the transaction that creates its journal: `transaction_kind` is part of its version, and lots and lot events are written with their own journals. Event-suggestion linking targets an already investment-kind payment. **Limitation:** read from the code; no test interleaves it |
| Delete draft | Investment links | The never-posted check (`status IN ('posted','voided')`) runs in-tx | `TestDiscardEditedGeneratedDraftPreservesAuditAndSkippedIdentity` |
| Approve | None | Single phase: status, deleted and review flag read in-tx | `TestApproveTransactionPreservesJournalBalancesOrderAndReconciliation` |
| Place, move posting, journal move | None | Single phase: crossing computed per checkpoint in-tx | `TestPlaceTransactionAcrossAReconciledBoundaryNeedsTheOverride`, `TestPostingMoveAcrossEarlierSameDayCheckpointRequiresOverride` |
| Move posting to another day | The transaction, rebuilt as a dated spec | Sent through `UpdateTransaction` bound to that read's version | `TestMovePostingToDayOutOfAReconciledPeriodIsGuarded` |
| Reconciliation start | None (the starting checkpoint is read in-tx) | The preselection is a second write, revalidated by the selection's own checks | Journey test |
| Selection and preview | The session and its candidates; totals computed in the service | In-tx: session open; each posting version current, posted, undeleted, in the account and commodity, dated on or before the statement, and not covered. The totals stay valid because a posting version's amount is immutable and the session's starting and statement balances are fixed | Journey test. **Limitation:** read from the code; the totals are trusted rather than recomputed in-tx (maintainability notes) |
| Finish | None beyond the session | In-tx: open; difference zero; starting checkpoint still the latest active one (`RECONCILIATION_START_STALE`); selected versions still current and uncovered | `TestFinishRefusesAStartingCheckpointThatIsNoLongerCurrent`, journey test (API, with code) |
| Void session, void checkpoint, posting status change | None | Single phase. The status change refuses covered postings in-tx | `TestUncoveredReconciledPostingStatusIsChangeable`, `reconciliation-void-checkpoint.spec.ts` |
| Override acknowledgement (every guarded write) | The impact the owner reviewed | Generic ledger writes: bound to `reconciliation_override_checkpoint_ids` in the write transaction (`TestOverrideReviewedAgainstOneCheckpointIsRefusedOnceTheSetGrows`). Was a bare boolean until #296 | **#296** fixed |

## Limits

- Investment writes, imports and recurring generation also meet the
  checkpoint guard. Their own seams belong to #272/#273, #271 and #276. Only
  their shared entry points were read here.
- Concurrency evidence for the in-request seam comes from the existing
  decomposed prepare/commit tests. This audit added no new interleaving test.
- The browser journey checks layout by `scrollWidth` and the visible states.
  Accessibility of these screens is #278's.

## Maintainability notes

- **"Override required" is inferred from a generic conflict.**
  `ErrReconciliationOverrideRequired` maps to `409 CONFLICT`
  (`backend/internal/api/transactions_errors.go`). The reorder mode treats any
  `CONFLICT` as an override prompt, because "the reorder endpoints have no
  other conflict" (`frontend/src/lib/transactions/reorder-mode.svelte.ts`). A
  new conflict on those endpoints would be shown as a reconciliation question.
  The stale refusals already have their own codes.
- **The early checkpoint refusal is written twice.**
  `rejectIfCheckpointAlreadyInTheWay` and an inline copy in
  `prepareCreateTransactionForWriteWithOptions`
  (`backend/internal/app/transactions_write.go`) both resolve candidates and
  return `ErrReconciliationOverrideRequired`. Both are courtesies before the
  in-tx guard, but they must stay identical.
- **Lifecycle state is checked in both layers, with different coverage.**
  Voided and deleted are checked in the service and again in the repository.
  The investment-link refusal is checked in the service only. The rule "which
  facts the commit re-asks" differs per fact and is documented only in
  comments (T-94, T-100).
- **Reconciliation totals are trusted, not recomputed.** The service computes
  `selected_balance` and `difference` from an out-of-transaction read
  (`replaceReconciliationSelection`, `backend/internal/app/reconciliation.go`).
  The repository stores them, and finish trusts the stored difference. That is
  correct only because a posting version's amount is immutable and the
  repository revalidates the version set. A future change that lets a version's
  amount change would break it silently.
- **Two filter builders for the transaction list.** The account and commodity
  filters live in the shared `where` list, the entry-basis category filters in
  `matchingEntryClause` (`backend/internal/db/transactions_read.go`). #298 is
  the divergence.
- **Three reorder writers compute boundary crossing separately.**
  `MovePosting` (`db/transactions_move.go`) and `PlaceTransaction`
  (`db/transactions_reorder.go`) each work out the crossed same-day checkpoints
  inline, besides the shared `activeCheckpointRefsAtOrAfter`. The journal-only
  `MoveTransaction` remains a third path, still used by the typed client.

## Validation

- Focused backend suites with the race detector:
  `./scripts/go-backend.sh test -race ./internal/api ./internal/app ./internal/db -run 'Transaction|Reconcil|Register|Ledger|Checkpoint|PostedOn|MovePosting|Posting|Place|Reorder|Draft|Lifecycle|Stale|Approve|Void|Trash|Restore|DaySequence|ReReconcile|ReferenceBook|Correction'`
  passed (api 24 s, app 38 s, db 20 s), including the new journey test.
- Browser, on a rebuilt integrated binary:
  `pnpm --dir e2e exec playwright test ledger-phone-journey.spec.ts transactions.spec.ts transactions-workspace.spec.ts transactions-reorder.spec.ts transactions-posted-on.spec.ts transactions-lifecycle-reconciliation.spec.ts reconciliation-recovery.spec.ts reconciliation-void-checkpoint.spec.ts trash-restore-reconciliation.spec.ts register-correction-chain.spec.ts`:
  22 passed.
- Per the evidence contract, the full backend, race and browser gates were not
  run; #280 owns them.
