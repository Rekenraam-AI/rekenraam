# Product complexity: necessity evidence — 2026-10-28

Second review output for [#390](https://github.com/sergeyfarin/rekenraam/issues/390),
following the [capability map and selection](product-complexity-coverage-2026-10-28.md).
Runtime/test source: `83f1e38b01cb66c65ce643e5edd5c4a7a2fe9164`.
The intervening coverage commit `cd7720c8` changes documentation only.
Exactly the selected six workflows and twelve rules are examined here.

This supplies necessity evidence and provisional rule-level conclusions, not
final proposal selection or whole-application acceptance. No runtime, schema,
API, test expectation or accepted policy changes. Final disposition and zero
to five independently actionable proposals remain the third output; zero is
allowed. Existing accepted #391 changes are not new proposals.

## Method and evidence boundaries

Trace the actual UI/API, preparation, authoritative transaction, independent
storage backstop and affected readers. Hand-derived counterfactuals below ask
what becomes wrong if a protection disappears; passing tests alone do not
justify the protection. They are arithmetic/logical counterexamples, **not
claims that a new ablation ran**. Historical executable removals in
[#391's comparison](financial-dependency-comparison-2026-10-23.md) and
[inventory](financial-dependency-inventory-2026-10-23.md) retain their own SHAs.
Current positive/negative controls were rerun; the retained
[result and exact selection](evidence/product-complexity-2026-10-28/backend.json)
separate those observations from reasoning.

UI friction below counts required semantic actions visible in current source,
not measured clicks, owner usability acceptance or new latency measurements.
Change amplification means which owners must change together, not LOC or
scanner-derived check counts. #379's 21-second historical payment-dense result
belongs to the retired global entitlement policy; it is not a current command
timing. No new performance result is claimed here.

## Ordinary posted correction and lifecycle

**Task/friction.** Open the shared transaction workspace, edit an ordinary
expense or its description, save. Open-period posted records already support
direct edit; retaining history does not require the owner to reverse and re-enter
every expense. Void and soft-delete have distinct visible/recovery meanings.
An edit that changes reconciled financial facts adds impact review; metadata
does not. A stale full-spec edit must reload before saving. These costs protect
different outcomes.

**Actual path.** `frontend/src/lib/transactions/transaction-editor.svelte`
adds the displayed `expected_version_id`; the workspace/detail panel owns
edit/correct/lifecycle actions. `api/transactions_write.go` requires that field
for PATCH. `app/transactions_write.go:UpdateTransaction` compares reviewed and
current versions, validates the spec and builds financial checkpoint candidates.
`db/transactions_write.go:UpdateTransaction` takes write admission, rereads the
current version, calls `requireExpectedVersionTx` and the account dependency
guard, appends a version/audit, then enforces checkpoints in the same transaction.
All backend paths here and below are under `backend/internal/` unless stated.

| Rule / authority | Counterfactual and matched valid control | Necessity conclusion |
|---|---|---|
| **1. Durable posted history.** Ledger plan, conventions; `transaction_versions`, `journal_entries`, `posting_versions`, lifecycle/deletion/correction records and transactional audit. Baseline migration 1's `*_no_update`/`*_no_delete` triggers independently reject rewriting posted evidence; never-posted draft deletion has a deliberate exception. | Start 1,000, spend 10, edit to 15: current cash must be 985 while old 10 and the initiating edit remain explainable. Void/soft-delete removes the current 15 from active balances, restore returns it once. Erasing old rows can leave the same current 985 but destroys the evidence needed to explain a statement or source-linked correction. The real ledger lifecycle journey also checks the two date axes, history and metadata-only checkpoint preservation. | **Retain evidence; reject mandatory reversal for every ordinary edit.** Current version append is already simpler for the owner. An audit JSON blob cannot replace version identities referenced by postings/checkpoints/import effects. |
| **2. Expected-version binding.** Browser requirement plus `requireReviewedVersion` give early feedback; `requireExpectedVersionTx` is the final concurrency authority. | Two tabs open a 10 expense. Tab B changes it to 20, leaving cash `1,000 − 20 = 980`. A stale description save from A still carries 10; without binding it restores cash to 990 even though A intended no amount change. HTTP control refuses it with `TRANSACTION_VERSION_STALE`, keeps 980 and writes no version; reloading then editing the description succeeds at 980. The prepared-write control also covers a newer amount reconciled between preparation and commit. | **Retain both boundaries.** The request field binds what the browser saw; the writer check binds prepared facts after a concurrent write. Neither the single-owner scope nor an empty metadata-impact list removes the race. |

**Consumers/change cost.** `db/transactions_read.go` selects current versions;
history and export retain superseded evidence. Reconciliation retains posting
version/line identity. Changes to lifecycle semantics would affect current reads,
CSV/QIF/bundle, trash/history, source identity and checkpoint integrity together.
No money computation or source migration is needed to keep the present policy.
The generic investment-edit refusal remains necessary context: this ordinary
writer cannot append investment lot effects atomically.

## Cash classification and attribution repair

**Task/friction.** Record known cash/gross/tax with an optional payer, or use
the existing correction form to assign/clear attribution. There is no mandatory
short-opening, historical-holding repair or clearing of unrelated warnings.
Attribution repair still records a reason/history; actual cash/date/account/
currency corrections use #395's normal impact review. These are distinct tasks.

**Actual path.** `investments/dividend-correction-form.svelte` and
`payment-in-lieu-form.svelte` under `frontend/src/lib/`, native dividend/payment
API handlers, `app/investment_dividend_correction.go` and
`app/investment_payment_in_lieu.go` preserve omission/null/value semantics.
The typed writers in `db/investment_dividend_correction.go` and
`db/investment_payment_in_lieu.go`, plus foundation recording in
`db/investment_foundation.go`, use `db/investment_writer.go` for
inverse/replacement, audit and net checkpoint effects. Provider candidate reads
in `db/investment_provider_events.go` use `cashPaymentClaimSQL`; migration 1's
`investment_cash_payment_identity` view and
`investment_event_payment_identity_insert/update` triggers also fence competing
claims across original, inverse and replacement journals.

| Rule / authority | Counterfactual and matched valid control | Necessity conclusion |
|---|---|---|
| **3. Cash attribution is advisory.** ADR 0019, #388/#389/#393, nullable payer/payment facts and independent payment shape/components. Diagnostics report unsupported/unavailable explanation. | Gross 100, embedded tax 15 and separate tax 2 give cash `100 − 15 − 2 = 83`, income 100, withholding 17. Clearing payer changes none of those figures or lots/checkpoints. The withholding control checks these exact balances and coverage: tax-only gross is absent, not zero income masquerading as a dividend. An unassigned payment in lieu posts cash −50/expense +50 and no invented short; attribution self-check is incomplete, not a corrupt journal. Requiring holdings adds no financial guarantee for either cash-only task. | **Retain the delivered relaxation.** The permanent entitlement restriction is already removed. Supplied invalid IDs/dates and required cash/shape/account rules remain validation; optional attribution is not an authorization token. |
| **4. Once-per-payment source identity follows corrections.** ADRs 0017/0019, `cashPaymentClaimSQL`, immutable source records and storage claim guards. | Provider event A claims payment P. Owner corrects P to P′ without adding cash. If another event B can claim P′, cash can remain correct while two events now purport to explain one payment; future dedupe/repair is untrustworthy. The descendant-identity control refuses a second link; the source-repair control also tests direct SQL admission across the ancestry, preserves original evidence and permits owner attribution differences. A separate actual payment with identical economics remains linkable. | **Retain identity, not forced agreement with the source.** An owner can change/clear payer without freeing the original source claim. Equal cash alone cannot justify deleting this relationship. |

**Consumers/change cost.** Correction-chain/attribution reads distinguish warnings,
unavailable support and integrity damage. Income coverage retains unassigned
known amounts; exported typed facts preserve original payer/shape/components and
correction/source history. Removing the claim guard would require coordinated
candidate, writer and storage changes and would break provenance even without
moving money. No new migration is proposed; historical migration 25 is now
consolidated into baseline 1, not a missing current migration file.

## Fractional disposal and corporate-action correction

**Task/friction.** Select the original split/exchange/spin-off/merger from its
activity history, preview a fraction disposal or correct the parent, then save.
Actual changed gains/checkpoints require acknowledgement; a retired parent alone
does not demand reversal of the disposal. A true insufficient-unit or explicit
lot conflict still names the dependent operation for correction.

**Actual path.** `frontend/src/lib/investments/cash-in-lieu-form.svelte` and
parent activity/correction forms call native entry/preview/available-lot and
correction endpoints. `app/investment_cash_in_lieu.go:cashInLieuTrade` reads the
selected historical parent's typed scope, validates fractional quantity/date,
then uses the sale/disposal path. `db/investment_cash_in_lieu.go` resolves that
original scope without requiring current standing; fact writers retain its
operation ID. Parent preparation now shares
`app/investment_corporate_action_correction.go` (#401). The investment writer,
disposal allocation and chronological replay owners commit actual effects or
roll them all back. Baseline 1's `investment_cash_in_lieu_facts_valid` checks
book/type/scope/date integrity, not permanent parent standing.

| Rule / authority | Counterfactual and matched valid control | Necessity conclusion |
|---|---|---|
| **5. Actual disposal quantity/basis/elections.** ADRs 0012/0013, typed decisions/allocations and replay. | With 3 units costing 30, a valid 0.5 disposal for 8 consumes basis `30 ÷ 3 × 0.5 = 5`, leaves 2.5 units/basis 25 and gain 3. Removing a quantity guard would also admit a 0.5 disposal against only 0.3 units, producing −0.2 remaining. Enough aggregate units in another lot do not authorize replacing an explicit lot election. Current all-parent valid-replay and insufficient-unit controls plus the specific-lot/later-sale control cover both sides. The 3/30/0.5/8 figures are the hand-derived #391 comparison, not the distinct figures in every current fixture. | **Retain actual allocation/election/replay guards.** Parent optionality is not disposal optionality. Preserve unknown-basis semantics; this known-basis example cannot justify manufacturing unknown cost. |
| **6. Historical parent is provenance, not continuing authorization.** ADR 0019 and delivered #392/#394. | Reverse a split but retain independently sufficient original shares: the same 0.5 disposal can remain financially valid. A current-parent-only veto strands that task without protecting quantity/basis. The eight current historical-entry cases (four parent families × replaced/reversed) allow preview/entry against original identity, compare actual allocations, retain bundle provenance and expose unresolved current ancestry. Invalid original scope/type and late failure still refuse/roll back. | **Retain the delivered removal of the standing veto.** Keep the original typed reference and actual date/scope rules. Deleting provenance altogether solves no remaining user step and loses explanation. |

**Consumers/change cost.** Lot state, effective disposal revisions, gains review,
reconciled holding/cash boundaries, original bundle facts and foundation self-check
must agree. Advisory current-parent context may be unavailable while the disposal
and immutable original reference remain valid. Reintroducing a blanket guard or
generic unlink workflow would undo the accepted contract. #401 already removes
duplicate preparation; there is no second mechanical extraction proposed here.

## Import repair and partial retry

**Task/friction.** Resolve known financial amounts/accounts in the existing import
review, commit eligible rows, then repair held/failed work in that batch. Legacy
cash-only dividends use the existing same-payment repair, with a reason and
displayed source cash; no provider round trip or delete/reimport chain is needed.
The UI offers an explicit owner declaration of no tax. That is an assertion,
not a default inferred from missing provider withholding.

**Actual path.** `frontend/src/lib/imports/dividend-review-row.svelte` sends
resolution plus the displayed transaction version and previews actual checkpoint
impact. `api/import_dividend_review.go` calls
`app/import_dividend_review.go:ReviewImportDividend`; required gross/tax and
eligible accounts are prepared before native writing. `commitCSVInvestmentRow`
in `app/import_csv_investment.go` always reports the row handled, even on error,
so invalid typed income cannot reach ordinary-cash fallback.
`app/import_service.go:commitImportBatch` prevalidates CSV investment rows,
processes ordinary rows with per-row partial-commit semantics and keeps
`partially_committed`/`failed` batches reviewable. `db/import_dividend_review.go`
pins current version and source eligibility, appends the classification version,
typed operation, audit, resolution and source effect in one transaction. Repair
retains the staged raw evidence; source revisions and commit identity effects
also have independent immutability triggers. Staged-row commit admission checks
its book/fingerprint/effect linkage; it is not a blanket raw-JSON immutability
trigger.

| Rule / authority | Counterfactual and matched valid control | Necessity conclusion |
|---|---|---|
| **7. Unresolved required economics cannot fall back to cash.** ADR 0017, import/dividend contracts, `reviewedDividendInput` preparation and typed commit routing. | An 85 receipt could mean gross 85/tax 0 or gross 100/tax 15. Both satisfy cash 85; guessing the first loses 15 income and 15 tax. The unknown-economics control holds that row while an independent 10 cash row commits, then accepts explicit 100/15 only after the owner supplies a tax account. Missing-payer control allows unassigned payer while refusing missing required economics. | **Retain the financial-input requirement, reject a provider-completeness gate.** Manual repair works without provider access. Absent payer and absent gross/tax are materially different facts. |
| **8. Repair/retry retains source evidence and settled rows.** Import plan, #242/#301, `RepairImportDividend` and atomic commit identity/effects. | Legacy cash 85 repaired to gross 100/tax 15 must still mean one payment of 85; appending a new ordinary payment would make 170. The current repair keeps the transaction ID, advances its version, retains metadata/order, supports native correction and makes reimport duplicate. A batch with reconciled spend 4.50, held 10 and committed 20 moves from −24.50 to −34.50 on reviewed retry, not −54.50. The HTTP partial-retry control also repairs fully failed batches; late source-identity failure rolls back typed facts/audit. | **Retain immutable source/settlement identity and local resumability.** All-or-nothing batch semantics would change accepted recovery; treating held rows as permanently skipped would strand work. Neither simplifies the owner's actual task. |

**Consumers/change cost.** Import row/batch status, dividend coverage, ordinary
versus typed classification, history/export and later source dedupe are affected.
For legacy repair, financial cash account/date stay the source settlement while
classification is repaired; subsequent native financial correction is separate.
This is justified input distinction, not evidence of the retired #395 field
freeze. The import's per-row writer needs checkpoint-subset semantics described
below; it cannot safely copy a single-command exact-set check unchanged.

## Reconciliation, neutral changes and reorder

**Task/friction.** Preview actual consequences, review only affected statements,
save or override the named set, then re-reconcile invalidated work. Metadata and
cash-neutral attribution repair require no extra statement confirmation. A stale
review refreshes the set before it can authorize a changed consequence.

**Actual path.** Workspace/reconciliation/reorder controls and shared
`frontend/src/lib/investments/reconciliation-confirm.svelte` send reviewed
checkpoint IDs. `app/transactions_reconciliation.go:reconciliationCandidates`
compares financial fields by posting line, not description. The early service
refusal is courtesy feedback. `db/reconciliation.go:enforceCheckpointBoundaryTx`
resolves current boundaries and `bindCheckpointOverride` binds/invalidate them
under the same transaction as the financial write. Append-only investment
commands use `db/reconciliation_netting.go` at each `(date, account sequence)`
boundary; editing existing lines retains per-posting rules. Checkpoints and their
linked evidence are durable; a UI boolean is not an independent storage guard.

| Rule / authority | Counterfactual and matched valid control | Necessity conclusion |
|---|---|---|
| **9. Guard actual checkpoint impact.** Ledger/conventions, transaction candidate diff, command netting, transactional invalidation. | Change only a description or payer at cash 85: the statement remains 85, so invalidating it adds rework without protection. By contrast move a 10 posting from after a checkpoint to before its same-day sequence: day-end cash is unchanged but the statement boundary differs by 10. Current neutral-correction/metadata controls preserve checkpoints; boundary-move and settlement-date controls require override. Summing only a command's final cash across all dates would miss the invalid result. | **Retain actual positional impact; reject blanket invalidation.** Neutral ordinary edit and multi-journal correction achieve the same owner outcome through intentionally different algorithms. A common final binder does not make the algorithms duplicates. |
| **10. Bind override to reviewed boundaries.** #296/#304/#305; `bindCheckpointOverride`, exact set for one command, subset of reviewed union for partial batch rows. | Review checkpoint A; another tab finishes B. A bare `override=true` would now invalidate A+B without B being shown. HTTP control returns `RECONCILIATION_IMPACT_STALE`, preserves both and cash 985; after reviewing A+B the 15 expense plus 5 later expense leaves `1,000 − 15 − 5 = 980`. Conversely, requiring each import row's current set to equal the whole reviewed union rejects legitimate rows as earlier rows invalidate their own checkpoints. The batch control permits subsets and holds a row touching a new unseen checkpoint. | **Retain exact-versus-subset distinction.** It binds real consequences without a repeated batch review per settled row. Courtesy preview cannot replace final checks after concurrency waits. |

**Consumers/change cost.** Session candidate inclusion uses covering active
checkpoint chains, not just a stale posting status. Actual financial edits reset
verification fields; metadata retains them. History, re-reconciliation and
checkpoint self-check must agree with invalidation. Moving the authority out of
the transaction reopens a demonstrated trust failure; sharing only repeated
presentation/preparation belongs to #284, whose accepted work is delivered.

## Recurring review and forecast explanation

**Task/friction.** Save a schedule, review a generated draft, post or discard it,
then inspect recorded/projected amounts and event details. Posting remains an
explicit action; drafts and forecast assumptions are outside actuals. A bulk post
retains completed items and reviews remaining ones on partial failure. Discarded
work must not reappear merely because the schedule is enumerated again.

**Actual path.** `frontend/src/lib/recurring/recurring-screen.svelte` reviews
impacts and posts each displayed draft with its version and reviewed IDs; each
success removes that item from pending review. Native recurring endpoints call
`app/recurring_generation.go` and review services.
`db/recurring_generation.go:materializeRecurringOccurrence` guards template revision
and occurrence status after taking the writer, writes draft+occurrence+audit
atomically, and baseline 1 has `UNIQUE(template_id, occurrence_date)`.
`db/transactions_write.go:DeleteDraftTransaction` keeps an audited skipped
occurrence when removing an eligible draft.

`frontend/src/lib/forecast/forecast-events.svelte` sends the headline basis token
and discards mismatched/stale pages before refreshing. The forecast HTTP API
calls `app/forecast.go` and `app/forecast_events.go:Events`; `Balances` is rebuilt
and its basis compared before returning detail. `db/forecast.go:WithSnapshot`
loads the recipe's financial/reference/recurring inputs in one read-only
transaction. Forecast coherence is a read guarantee; there is no database
trigger that can bind two independent browser responses.

| Rule / authority | Counterfactual and matched valid control | Necessity conclusion |
|---|---|---|
| **11. Durable occurrence identity, including discarded work.** Recurring plan; writer revision/status guards, unique template/date and skipped tombstone. | One 100 expense on a date must generate one draft. If deleting it also erases occurrence identity, re-enumerating that date regenerates what the owner discarded. Two concurrent generators can likewise create two drafts without atomic identity. The HTTP discard control deliberately rewinds generation and still finds one skipped occurrence/zero transactions; independent-pool generators and late-failure rollback controls protect duplicate/crash cases. Forecast assumes the event once; a draft contributes zero to budget actuals until posted. | **Retain identity/tombstone and atomic creation.** A watermark alone cannot remember an owner decision after schedule edits. Keeping a skipped identity imposes no extra user step or financial journal. |
| **12. One forecast snapshot and response basis.** Projected-balances plan; read-only snapshot, `Events` basis equality and frontend stale-page reset. | Headline says tomorrow +100; a +200 transaction is then added. Returning new details +300 under the old +100 headline is a false explanation even though no ledger write occurs. Independently, materialization between separate template/occurrence reads could omit or double-count an event. Current API control returns `FORECAST_BASIS_CHANGED`; concurrent materialization returns exactly one template or draft event. Real generation/posting leaves projected amounts identical while recorded amounts change on posting. | **Retain coherent snapshot and cross-request basis.** They protect different races. Automatic refresh is recovery, not a substitute for rejecting mixed evidence; no owner acknowledgement or write lock is needed for this read. |

**Consumers/change cost.** The real forecast fixture's final projected USD
aggregate is `1,500 + 200 − 25 − 125 − 100 + 1,000 = 2,450`; materialization
must only change source identity, and posting changes the recorded curve.
Budget control separately proves `23.34 + 33.33 = 56.67` only after posting its
generated draft. These independent figures supplement conservation, not vice
versa. Forecast reads leave ledger reports/exports unchanged. Changing identity
or snapshot semantics affects recurring recovery, forecast event lists, budget
actuals and the draft/posted boundary together. Learned-model fitting is outside
this selected workflow.

## What the evidence supports, and what remains

The selected current controls support retaining history/version, financial
input/allocation, source identity, actual checkpoint and planning coherence
protections. The matched cash-attribution and historical-parent restrictions
have already been relaxed under ADR 0019. Ordinary direct edits and same-payment
repair already compose existing primitives without a new reversal subsystem.
No additional necessary simplification is established by check counts or by
these passing tests. This is evidence for final disposition, not an instruction
to invent a proposal to fill a quota.

Security/recovery still use their own threat and restore reasoning in the
coverage map; these financial controls do not certify them. Unselected investment
families, returns calculations, provider wire semantics, account lock policy,
all planning cadences/models, native translation review, deployment/household
acceptance and fresh browser usability/accessibility remain outside this pass.

## Reproduction and validation

**Passed:** all 38 selected top-level tests, plus 75 nested cases (113 completed
test events), with `-race -p 2 -timeout=5m -count=1` across app/API. No selected
test is missing, failed or skipped. Runtime/test sources still match the pinned
source. This is focused current-source evidence, not a full backend gate or
fresh browser run; Go race detection does not instrument bundled SQLite C.

Key named controls, grouped by the selection rather than a new test inventory:

| Workflow / rules | Current test owners and representative matched controls |
|---|---|
| Ordinary correction, 1–2 | [HTTP journey](../../backend/internal/api/ledger_audit_test.go), [browser binding API tests](../../backend/internal/api/ledger_review_binding_test.go), [prepared-write tests](../../backend/internal/app/transactions_stale_version_test.go): `TestStaleBrowserSaveAfterNewerAmountEditIsRefusedAndKeepsTheNewerAmount`, `TestStaleMetadataEditCannotOverwriteANewerReconciledAmount`, `TestOrdinaryMetadataEditOfAReconciledTransactionStillCommits`. |
| Cash classification, 3–4 | [Payer/tax tests](../../backend/internal/app/investment_dividend_attribution_repair_test.go), [payment repair](../../backend/internal/app/investment_payment_attribution_repair_test.go), [provider ancestry](../../backend/internal/app/investment_dividend_attribution_source_test.go): `TestSeparateWithholdingWithoutPayerPreservesShapeAndExactCoverage`, `TestDividendPayerAssignClearAndOmissionPreserveReconciledAmounts`, `TestDividendSourceAttributionRepairPreservesEvidenceAndWholePaymentIdentity`. |
| Fraction correction, 5–6 | [All-parent controls](../../backend/internal/app/investment_cash_in_lieu_parent_test.go): `TestCashInLieuAllParentsRefuseInsufficientUnitsWithoutWriting`, `TestCashInLieuParentReversalRetainsSpecificLotAndLaterSaleDependencies`, `TestCashInLieuHistoricalParentEntryAllFamilies`, `TestHistoricalCashEntryStorageKeepsOriginalParentIntegrity`. |
| Import repair, 7–8 | [Service repair](../../backend/internal/app/import_dividend_review_test.go), [API repair](../../backend/internal/api/import_dividend_review_test.go), [batch recovery](../../backend/internal/api/import_audit_test.go): `TestTrading212DividendUnknownEconomicsStayPendingWhileCashCommits`, `TestImportedDividendRepairLateIdentityFailureRollsBackAllFacts`, `TestImportAuditHeldAndFailedRowsResumeInTheirBatch`. |
| Reconciliation, 9–10 | [Neutral/changed boundaries](../../backend/internal/app/reconciliation_correction_netting_test.go), [same-day order](../../backend/internal/app/reconciliation_sequence_boundary_test.go), [bound overrides](../../backend/internal/api/ledger_review_binding_test.go): `TestChainedCashNeutralCorrectionsPreserveCashCheckpoint`, `TestPostingMoveAcrossEarlierSameDayCheckpointRequiresOverride`, `TestOverrideReviewedAgainstOneCheckpointIsRefusedOnceTheSetGrows`; batch recovery file above also supplies `TestImportBatchOverrideIsBoundToTheReviewedCheckpointUnion`. |
| Planning, 11–12 | [Generation](../../backend/internal/app/recurring_generation_test.go), [discard](../../backend/internal/api/recurring_generation_test.go), [forecast snapshot](../../backend/internal/app/forecast_test.go), [event basis](../../backend/internal/api/forecast_test.go), [independent planning figures](../../backend/internal/api/planning_audit_test.go): `TestConcurrentRecurringGeneratorsWithIndependentDatabasePools`, `TestDiscardingGeneratedDraftPreservesOccurrenceAndCannotRegenerate`, `TestForecastConcurrentMaterializationKeepsOneBasis`, `TestForecastBasisChangesAfterFinancialMutation`. |

The JSON artifact records the selected names, exact command, package and leaf
outcomes, uncached execution and source reconciliation. Reproduce without another
test list:

```sh
python3 - <<'PY'
import json, subprocess
from pathlib import Path
evidence = json.loads(Path('docs/reviews/evidence/product-complexity-2026-10-28/backend.json').read_text())
subprocess.run(evidence['command'], check=True)
PY
```

The restricted first attempt passed the API selection but the app package
stopped when `httptest.NewServer` could not bind a loopback socket. That is an
environment limitation, not a financial assertion failure. The unchanged
selection passed with local sockets permitted; no guard, assertion or test was
removed. Local Markdown targets/anchors, referenced source/test names, artifact
completeness and `git diff --check` were checked. No new CI gate or audit
framework is introduced.
