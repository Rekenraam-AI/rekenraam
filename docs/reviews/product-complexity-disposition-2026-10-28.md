# Product complexity: final dispositions — 2026-10-28

**Third and final output of [#390](https://github.com/sergeyfarin/rekenraam/issues/390)'s
bounded first pass.** The [capability map](product-complexity-coverage-2026-10-28.md)
and [necessity evidence](product-complexity-necessity-2026-10-28.md) supply the
all-family inventory and six-workflow/twelve-rule traces. This output gives
final dispositions, two bounded implementation findings, rejected simplifications
and explicit limits. It completes this audit without claiming that every
application rule or deployment was audited.

## Source and acceptance boundary

The first two outputs pin runtime/test source
`83f1e38b01cb66c65ce643e5edd5c4a7a2fe9164`; their documentation commits are
`cd7720c8` and `000d44e2`. The final critical review began at `000d44e2`, whose
runtime was unchanged. Two newly reproduced defects were filed separately and
fixed in scoped commits:

- [#433](https://github.com/sergeyfarin/rekenraam/issues/433), `dd9e8579`:
  imported-dividend form/version snapshot binding.
- [#434](https://github.com/sergeyfarin/rekenraam/issues/434), `76f643d8`:
  honest learned-event presentation and matching detail columns.

Final runtime reviewed here is `76f643d8`. The acceptance/implementation
authorization for these small fixes is the owner's 2026-10-28 instruction to
file needed follow-ups and fix small findings at their root. They restore
existing contracts; no new financial product policy, API or schema is adopted.
The final audit commit itself changes only review/tracking documents and
retained evidence. Local delivery does not imply a push, hosted CI or release.

#391 and #388/#389/#392/#393/#394/#395 are delivered inputs, not open proposals.
In particular, #395's financial field corrections need no further implementation
under this issue. #284's accepted mechanical work is also delivered; its
explicitly deferred candidates are not silently reactivated by this audit.
Work order remains in the [roadmap](../roadmap.md#current-focus).

## Final disposition of the twelve rules

The numbered rules are the original selection, not new issue IDs. Full
authority paths, financial figures, threat/recovery distinctions and matched
controls remain in the necessity report. A retained protection's reason below
is the concrete failure it prevents, not the existence of a passing test.

| Rule | Final disposition | Protected result and consequence if removed / mismatched comparison |
|---|---|---|
| 1. Durable posted history | **Retain.** Ordinary direct editing is already the simpler composition. | A 10 → 15 expense can leave correct current cash while erasing how a prior statement or imported record arose. Append versions/audit; do not mandate inverse/re-entry for every ordinary open-period edit or replace version identities with generic JSON. |
| 2. Expected-version binding | **Retain and repair the browser application of it (#433).** | Stale 10 fields must not overwrite another tab's 20 expense. The service and writer correctly reject stale versions; the form must send the version belonging to its fields, not a refreshed token. The new browser control closes a gap that the existing API controls could not detect. |
| 3. Cash attribution is advisory | **Retain the delivered removal of permanent attribution authorization.** | Gross 100/tax 15/separate tax 2 still means cash 83, income 100 and tax 17 after payer clearing. Requiring holdings or a manufactured short adds work without protecting those facts. Keep real cash/shape/account validation and warning/unavailable diagnostics. |
| 4. Once-per-payment source ancestry | **Retain.** | Two provider events claiming one corrected payment can leave balances unchanged while violating source identity/dedupe. Preserve the root claim across original/inverse/replacement, permit owner/source disagreement and keep a separate actual equal-valued payment linkable. |
| 5. Actual fraction allocation/elections | **Retain.** | Disposing 0.5 against 0.3 units creates an impossible −0.2 remainder; adequate units in a different lot cannot silently replace an explicit election. Actual disposal replay and quantity/basis/unknown-state controls remain required. |
| 6. Historical parent provenance | **Retain the delivered removal of the current-standing veto; retain original identity.** | A split's reversal need not make an independently supported 0.5 disposal invalid. The original typed reference and actual scope/date/quantity remain auditable; current ancestry can be unavailable without blocking valid entry. A generic unlink workflow would discard evidence without removing a necessary user step. |
| 7. Required import economics | **Retain.** | Cash 85 cannot distinguish gross 85/tax 0 from gross 100/tax 15. Keep unknown required inputs pending, accept explicit owner facts without a provider round trip and distinguish missing optional payer from missing financial economics. |
| 8. Import repair/retry identity | **Retain existing local composition.** | Repair one 85 payment without posting a second 85. With −4.50 already posted, a committed 20 and held 10, retry reaches −34.50 once. Immutable source/effects plus reviewable partial/failed batches provide recovery; blanket rollback or permanent skipping would strand valid work. |
| 9. Actual positional checkpoint impact | **Retain, with different algorithms for edits and appended commands.** | A neutral description/payer repair preserves the statement; moving 10 across its same-day boundary changes that statement even if day-end cash agrees. Keep per-line diffs and per-boundary command netting, rather than invalidate all statements or sum across dates. |
| 10. Reviewed checkpoint binding | **Retain exact-set versus batch-subset semantics.** | Review A, then create B: override cannot invalidate unseen B. A multi-row import's later row may legitimately touch only a subset of the already reviewed union. One universal equality rule either weakens trust or creates repeated/unnecessary review. |
| 11. Durable recurring occurrence | **Retain.** | Deleting a generated draft without its skipped identity lets schedule re-enumeration recreate dismissed work; separate draft/identity commits permit crash duplicates. Keep the tombstone and atomic generation; they add no extra owner step or posted journal. |
| 12. Coherent forecast evidence | **Retain and repair source presentation (#434).** | A snapshot prevents omitted/doubled materialization; a response basis prevents new +300 details explaining an old +100 headline. Within that coherent response, an estimate must remain distinguishable from a saved recurring record. Correct labels/navigation without changing arithmetic or requiring acknowledgement for reads. |

## Two actionable findings, both delivered

These are the **only two new proposals filed in this pass**, below the cap of
five. Both are small correctness fixes within the selected workflows. They
are separate from the audit document and from the earlier financial-policy
decisions. No scanner finding is promoted to an issue without a demonstrated
user consequence.

| Finding / before → after | Smallest change and user benefit | Retained guarantees / migration, API and report impact | Verification and cost of doing nothing |
|---|---|---|---|
| **#433, form snapshot mismatch.** Open a legacy cash-dividend repair with income A. Another tab changes classification to B; reconnect refetches the list. Previously the old A fields silently acquired version 2 instead of the original 1. Now that form sends 1 and receives the existing stale-entry refusal until deliberately reloaded. | Capture the expected version with the editable resolution. The owner can keep unsaved input during refetch without accidentally overriding a newer classification. No generic controller/framework or extra confirmation. | Keep server writer/version/source/checkpoint guards, exact same-payment cash, unsaved input and translated recovery. No API/schema migration or calculation change. Governing clarification: [import review plan](../plans/import-dividend-review-plan.md) and [frontend conventions](../conventions.md). | Real browser regression first fails `Expected: 1, Received: 2`; fixed preview returns 409 and preserves the newer ordinary journal/category/version. The whole import/editor-refetch selection passes. Without the fix, correct backend guards can be defeated by the browser misdescribing what was reviewed. |
| **#434, event-source fallthrough.** A learned Wednesday outflow previously displayed “Recurring template” and “Review recurring entries,” despite having no saved record; its expanded row spanned six of seven columns. Now it says “Estimated spending,” has no nonexistent record link and spans the actual columns. | One exhaustive typed map owns source label/tone/navigation; derive colspan from the displayed overlay. The owner can distinguish an assumption from a saved bill in its detail view. No new model engine or source registry. | All four existing API sources remain distinct. Posted/draft/template navigation, opt-in/off, exact native amounts, basis rejection and read purity stay. Six catalogs gain one label; no wire, schema, model or arithmetic change. Governing clarification: [learning plan](../plans/forecast-learning-plan.md). | The real-history browser case first fails to find the estimate label after expanding its next Wednesday. Fixed coverage checks the label, absence of recurring navigation, seven/six columns, phone/accessibility and unchanged ledger. A source added to the API union now requires an explicit presentation choice at compile time. |

## Critical code review and refactoring dispositions

This is the requested code-smell/improvement review along the six workflows'
affected consumers, not another whole-repository scanner pass. Necessity and
implementation quality were considered separately.

| Candidate / evidence | Decision, retained difference and reconsideration trigger |
|---|---|
| **Split form state and reactive guard token**, one reproduced caller in `imports/dividend-review-row.svelte` | **Fix root cause in #433.** The pattern search covered all explicit frontend `expected_version_id`, `expected_transaction_version_id` and `transaction_version_id` consumers. Ordinary editing reloads fields with its transaction; recurring confirmation retains its reviewed row snapshots; lifecycle/undo use captured transaction records. No second reproduced mismatch was found. Do not solve it by resetting unsaved fields on every query refetch or removing the writer check. |
| **Catch-all source branch and repeated presentation decisions**, `forecast-events.svelte` | **Simplify the implementation in #434 while correcting the defect.** Three sources had explicit cases and the fourth inherited a false meaning. The source union now drives one local exhaustive presentation map. No cross-feature event registry or speculative abstraction is justified. |
| **Repeated preview/commit/retry transitions** | **Reuse delivered #400; reject another extraction project.** `previewReview` and `commitReview` already share compatible protocols. The earlier coverage output mistakenly named #399 here; #399 owns native realized totals. Source-specific import failure/reload behavior need not be forced into a gain-review controller. A future protocol change or demonstrated divergence can justify another bounded extraction. |
| **Repeated corporate-action preparation** | **Reuse delivered #401.** The four selected parent families already share admission/verification. Keep typed family lookups, no-op rules, original facts and financial replay explicit. Do not infer a universal correction engine from similar orchestration. |
| **Courtesy validation plus writer checks plus SQL triggers** | **Retain distinct authorities.** Early errors improve feedback, the writer handles races in the state it commits, storage backstops preserve immutable evidence and cross-book/type rules. Identical prose or repeated IDs do not prove duplicate protection. The [#405 rejection table](schema-baseline-review-2026-10-26.md#what-the-critical-review-retained) remains the bounded storage argument; no trigger removal is proposed. |
| **Checkpoint walkers, balancing alignment, error leaves, numeric leaves and source hygiene from #284** | **Leave explicitly deferred.** No newly demonstrated defect or named representation change activates their recorded triggers. The [owner-reviewed table](maintainability-audit-2026-10-24.md#explicitly-deferred-inventory-original-ranks-713) governs; the user's request to fix small findings is not evidence that every listed cleanup is now useful. |
| **Sequential recurring confirmation and per-row import commits** | **Retain recovery semantics.** Completed work stays committed and remaining work stays reviewable. Merging them into one generic bulk transaction or parallelizing financial writes changes ordering and failure recovery. A UI cost hypothesis needs measurement and its own contract before changing those semantics. |
| **Backend composition and repeated dividend preparation/context reads** | **No new optimization issue.** The selected screens use composed reads; repeated backend lookups/preparation are visible but no new latency/allocation regression was measured. Share only after a named payoff and same-snapshot authority analysis, not because a service call appears twice. #379's old global entitlement cost does not measure these paths. |

Rejected product simplifications are likewise explicit: zero-filling unknown
gross/tax/basis; discarding original provider/parent evidence; removing stale
review binding for a single owner; automatic checkpoint invalidation; mandatory
reversal chains for ordinary edits; restoring retired attribution vetoes;
freezing all source corrections because a saved report exists; or treating
security/recovery controls as removable whenever balances stay unchanged.
Their financial, trust, threat or recovery counterexamples are in the prior
necessity/coverage reports and #391/#405; no new generic rule engine, loan
subsystem or universal review queue is introduced.

## Coverage, remainder and stop

The six selected workflows are **reviewed within their twelve-rule scope**:
ordinary correction/lifecycle, cash attribution/classification, fractional
disposal/parent correction, import repair/retry, reconciliation/order, and
recurring/forecast explanation. Budget actuals, diagnostics, exports and source
history were affected consumers. This pass added browser evidence where the
earlier API/service selection missed two real UI defects; it does not reinterpret
the earlier 38 passing tests as whole-workflow proof.

Other investment families, account structural locks, report/returns algorithms,
provider wire semantics, every parser/cadence/model path, general saved-report
policy, setup/security/operational safeguards and the new Home design remain
**sampled or deferred** as recorded in the capability map. Security/recovery
samples retain explicit threat/restore reasons; they were not certified by
financial totals. Native terminology sign-off, actual owner household/off-device
recovery evidence, upstream native-driver work and release/candidate acceptance
remain their existing independent responsibilities.

The audit stops here. There is no residual implementation slice in #390, no
additional proposal needing owner product input, and no new blanket initiative
or release gate. New independent evidence may justify a future bounded issue;
“review the rest of the application” is not silently added to this issue's
acceptance. Existing deferred maintenance retains its original owner decisions.

## Validation and reproducibility

Retained [follow-up evidence](evidence/product-complexity-2026-10-28/followups.json)
contains original/fixed source references, exact commands, failing controls and
the successful run summary. The original [38-test race selection](evidence/product-complexity-2026-10-28/backend.json)
remains valid at its recorded backend source; these fixes change no Go code or
test assertions, API or migration. No full backend re-run or performance claim
is substituted for focused consumer evidence.

- Both new browser assertions failed on the original runtime before their fixes.
- `pnpm build` passes, including static frontend embedding and the web test.
- `./scripts/test-frontend.sh` passes: zero Svelte errors/warnings, all 781 unit
  tests and catalog parity; no OpenAPI/generated-client drift.
- All **18** affected browser journeys pass: 3 import review, 2 editor refetch,
  8 core forecast and 5 forecast learning. They include actual API writes,
  stale-basis controls, phone/keyboard/accessibility and ledger-read purity.
- Source-owner/test references, report/plan links, issue count and twelve-rule
  coverage were checked; `git diff --check` is clean.

The browser fixtures are synthetic and local. Historical source-link fixtures
intentionally require the disposable local database; this is not live-provider,
external-deployment or owner acceptance. Hosted CI and publication remain pending
an explicitly requested push. The local commits are the reviewable delivery.
