# Unresolved-basis diagnostic audit — 2026-10-18

**Status:** [#308](https://github.com/sergeyfarin/rekenraam/issues/308)
completed as an audit. The required application-status change is separately
scoped in [#313](https://github.com/sergeyfarin/rekenraam/issues/313).
#272/#273's original overall-clean-self-check acceptance remains unmet and
is handed to #313; #272 also retains #277's bundle-reconstruction requirement.
No runtime verdict, financial record or historical run was relabelled here.

## Baseline, scope and disposition

Audited `cdfcc7c3b2cb6d26b80f435fd6b4bcd7f53c4573`, with an initially clean
tree, using isolated provider-free books and the existing investment fixtures,
real application writers, complete self-check service and native SQLite
backup/restore. The unchanged #266 register and actual authenticated HTTP
self-check consumer were rerun. No live provider or new harness is needed.

The accepted unknown-basis/non-sale contracts say unknown information is not
damaged financial evidence. The current diagnostic nevertheless has only
`passed`/`failed` for an applicable completed check: unknown projected basis
adds a `lot_reconciliation` finding and makes the whole run failed. Positive
and sourced-zero resolution remove that finding; withdrawing the resolution
restores it, including on a closed lot. Original immutable opening evidence
stays unknown throughout. This is an **existing diagnostic limitation**, not
a newly confirmed financial corruption or an excuse to replace unknown by zero.

The governing-plan disposition is recorded in the
[portability plan](../plans/data-portability-plan.md#unresolved-investment-basis-diagnostic-acceptance-308).
The audit meets #308 by publishing the matrix, damage/restore controls and
the separately scoped implementation needed. It does not redefine a failed
run as satisfying #272/#273's original clean acceptance. #313 proposes a
distinct incomplete outcome with separate unavailable coverage and integrity
failures, across persistence, aggregation, API and the owner-visible screen.

## Consumers and independent figures

New consumers in
[`investment_diagnostic_audit_test.go`](../../backend/internal/app/investment_diagnostic_audit_test.go):

- `TestInvestmentDiagnosticAuditResolutionLifecycleAndRestore`: six families,
  literal financial figures after every knowledge transition, all 13 check
  IDs/statuses/counts, exact original/effective knowledge, unchanged financial
  export, and **24 native restore comparisons**.
- `TestInvestmentDiagnosticAuditDamageBesideUnknownBasis`: six deliberate
  damage controls beside supported unknown basis. Each requires the exact
  failed-check set and counts, actionable summary, and unchanged durable
  financial rows after the diagnostic. Some controls drop an immutability
  trigger in a disposable fixture; they are not admitted application writes.

The fixture enters two units with unknown basis. Its independent sourced
declarations are 80.00, then known zero, then 120.00, then withdrawn. Quantities
and cash do not change at these transitions; disposed/remaining basis and
gain are hand-derived below and compared with exact rational numbers. The
sale consumer reads actual realized entries; the capital-return consumer
separately checks reduction/excess. Every original opening remains
`opening_basis_knowledge=unknown` with a NULL original amount, even when its
effective projection is known.

### Diagnostic matrix — every family and transition

Each family below runs this complete matrix. **V** = verified, severity none
for financial correctness; **L** = verified existing diagnostic limitation.

| State | Overall status / failed checks | `lot_reconciliation` status / findings | Other checks | Disposition |
|---|---|---|---|---|
| Initial inbound: 2 units, NULL basis, no cash/disposal | failed / 1 | failed / 1; source lot ID sampled | All applicable checks passed; attachments `not_applicable` | L: supported missing basis is reported as failure |
| After partial/closed sale or non-sale action, basis still unknown | failed / 1 | failed / 1; same source lot ID | Same | L: zero remaining quantity does not invent known basis or loss |
| Sourced resolution 80.00 | passed / 0 | passed / 0 | Same | V: current basis known; original unknown source retained |
| Replace resolution with sourced known zero | passed / 0 | passed / 0 | Same | V: zero is known and journal-free, not missing information |
| Replace known zero with 120.00 | passed / 0 | passed / 0 | Same | V: revised basis and dependent gains agree |
| Withdraw effective resolution | failed / 1 | failed / 1; same source lot ID | Same | L: knowledge returns to NULL and the warning returns |

For every unresolved/withdrawn row the **entire** lot-check summary is:

> 1 lots have unknown projected basis; basis reconciliation is unavailable

This exact sole finding is checked, not merely the absence of a few corruption
phrases. All 13 results are present; every status and count is asserted.
The passing checks are `entry_balance`, `transaction_balance`, `book_balance`,
`version_integrity`, `commodity_position_sign`, `checkpoint_integrity`,
`account_version_coverage`, `investment_foundation`,
`investment_replay_equivalence`, `price_dependency_integrity`,
`sqlite_integrity`, plus `lot_reconciliation` when resolved. The attachments
hook remains `not_applicable`, with zero findings.

### Financial matrix — hand-derived expected amounts

Amounts below are EUR. Each triple is
**remaining basis / disposed basis / cumulative realized result**.
Unknown phases keep NULL projected basis and unresolved decision knowledge;
the numeric triples apply only to the three known declarations.

| Family and independent derivation | Remaining units / cash / decisions | At 80.00 | At known zero | At 120.00 |
|---|---|---|---|---|
| Sell 1 of 2 for 50.00: consume half the basis, leave half; gain = 50 − consumed | 1 / 50.00 / 1 | 40 / 40 / 10 | 0 / 0 / 50 | 60 / 60 / −10 |
| Sell both for 100.00: consume all basis | 0 / 100.00 / 1 | 0 / 80 / 20 | 0 / 0 / 100 | 0 / 120 / −20 |
| Write off both for no proceeds: loss = entire basis | 0 / 0 / 1 | 0 / 80 / −80 | 0 / 0 / 0 | 0 / 120 / −120 |
| Split 2 × 5/4 = 2.5; settle 0.5 for 8.00: consume one fifth of basis, retain four fifths | 2 / 8.00 / 1 | 64 / 16 / −8 | 0 / 0 / 8 | 96 / 24 / −16 |
| Same fractional settlement, then sell remaining 2 for 100.00: aggregate proceeds 108.00, all basis disposed | 0 / 108.00 / 2 | 0 / 80 / 28 | 0 / 0 / 108 | 0 / 120 / −12 |
| Receive 10.00 capital return, then sell both for 100.00: reduce basis by min(10, known basis), dispose the remainder | 0 / 110.00 / 1 | 0 / 70 / 30 | 0 / 0 / 100 | 0 / 110 / −10 |

Capital-return reduction/excess is 10/0 at basis 80 and 120, but 0/10 at
known zero. That 10.00 excess stays outside the disposal gain; it is not
added again to the 100.00 result. With unknown basis, receipt and entitlement
remain known, while reduction/excess and disposal basis/result are unresolved.
Intermediate split rows additionally assert exactly 2.5 units, NULL basis,
zero cash and zero disposal decisions before settlement.

**Actual:** every quantity, nullable knowledge pair, basis, cash, decision
count, realized figure and capital-return split matches these literals.
Conservation and complete diagnostic assertions pass after each phase.
This establishes financial correctness in these scenarios despite the
unsupported distinction in the overall status vocabulary.

## Corruption controls beside unknown information

The table records exact failed results after the specified damage. Other
applicable checks pass; attachments remain `not_applicable`. Every whole run
is failed, and diagnostics do not repair the injected state.

| Injected damage and independent expectation | Lot check findings | Foundation findings | Replay findings | Failed checks / actual disposition |
|---|---|---|---|---|
| Change unknown lot's projected units 2→1 while journal/events still say 2 | 3: unavailable basis + holding mismatch + quantity-event mismatch | 0 | 1: source lot balance | 2; V: both quantity comparisons remain active |
| Unknown disposal decision has proceeds 50.00; change its allocation to 60.00 | 1: original disposal allocation set disagrees with proceeds | 0 | 1: disposal allocations | 2; V: NULL basis does not exempt known proceeds |
| Move 1 unknown unit internally; change source link quantity to 2 while its effects still move 1 | 2: both current lots have unknown basis | 1: internal transfer missing linked source/destination effects | 2: source lot balance and method lock | 3; V: linkage and replay still detect the damage |
| Resolve inbound at 80.00, add another unresolved inbound, then alter immutable resolution fact to 70.00 without changing its 80.00 bridge | 1: independently unresolved lot | 1: resolution disagrees with bridge/pinned transfer | 1: resolved source lot balance | 3; V: bridge consistency is independently verified |
| Beside unknown lot, acquire 2 known units for 20.00; alter their current basis to 21.00 | 1: unknown projected basis only | 0 | 1: damaged known lot balance | 2; V: replay detects a known-lot defect even where aggregate basis comparison is unavailable |
| Resolve a capital-return sequence so its original unknown effect is superseded; relabel that original NULL effect as known zero reduction/full excess, beside another unresolved lot | 1: independently unresolved lot | 1: original capital-return effect fails receipt/lot-effect conservation | 0 | 2; V: the effective view does not hide superseded evidence from the audit |

Two distinctions matter for acceptance:

1. `lot_reconciliation` returns early for damaged allocation sets. Its single
   finding then means **damage**, rather than the single missing-basis warning.
   An acceptance helper cannot whitelist a run on `failed_check_count == 1`
   or the presence of any unknown lot.
2. With an unknown lot, the lot check cannot compare aggregate basis for that
   whole position. It can therefore miss a known peer's altered basis, while
   per-position replay correctly fails it. Passing the foundation check and
   checking only the unknown summary is insufficient: all invariant and replay
   checks must remain active. No undetected corruption was found in these six
   controls, but their independence is essential.

Existing `TestSelfCheckCatchesEachCorruption`, allocation conservation,
`TestReplaySelfCheckReportsStaleLotState`,
`TestReplaySelfCheckReportsStaleEffectiveRevision`,
`TestReplaySelfCheckReportsStaleTransferBasis`, known-zero bridge and original/
revised non-sale knowledge consumers passed as controls without a blanket
unknown exemption.

## Native restore and non-mutation

For each of the six families, back up and restore the **unresolved**, **80.00**,
**known-zero**, and **withdrawn** states: **24 restored books**. Use the actual
`OnlineBackupSQLiteDatabase` → `RestoreSQLiteDatabase` → `db.Open` path,
construct fresh services against the restored database, and repeat the same
literal financial and full diagnostic assertions.

Every restored check's status, count, sample, summary and narrative equals
the source run. Every exported CSV dataset also matches the source. Running
self-check before/after comparison leaves exported financial evidence
unchanged, including original unknowns and every resolution revision.
The damage controls separately preserve their durable-row snapshots.

This is representative native recovery evidence for #277, not its complete
#266 fresh-instance restore drill, bundle reconstruction, key-custody,
full-disk/cancellation or process-lock audit. No exported ZIP importer or
round-trip capability is implied.

## Acceptance disposition and implementation handoff

**Current:** unknown projected basis means the current application reports a
failed check/run. Known zero and positive resolution pass; withdrawal restores
the failure. Users can inspect the precise summary; the UI and API do not yet
have a structured incomplete status. Existing historical verdicts are facts
and must not be rewritten by a later improvement.

**Required follow-up #313:** specify and implement an incomplete outcome for
supported unavailable basis, with separate unavailable scope/counts and
preserved integrity failures. A proposed incomplete result must never hide
mixed damage, skip closed lots, suppress replay or relabel unknown as zero.
This crosses enum CHECKs, result/run aggregation, OpenAPI, the Data settings
screen, translations and scheduled-run presentation, so it is a separate
implementation slice, as #308 explicitly requires.

#308 can close with these four acceptance items established. #273's remaining
clean-self-check item stays unchecked and its blocker moves from completed
#308 to #313. #272 stays open for #313 and its separate #277 export acceptance.
#260/#277 receive the evidence and handoff; no product capability is claimed
delivered merely because the follow-up has been designed.

## Maintainability notes

- **Status meaning is duplicated across persistence and consumers.**
  `app/self_check.go` aggregates failed results, `db/self_check.go` filters
  terminal statuses, the migration enum CHECKs and OpenAPI repeat them, and
  `frontend/src/lib/settings/data-settings.svelte` chooses passed/errored/failed
  copy. A supported incomplete state requires coordinated edits; #313 must
  own that whole path rather than changing one badge.
- **Unknown-basis acceptance helpers cover selected phrases/checks.**
  `requireHealthyUnresolvedBook` in `investment_unknown_disposal_replay_test.go`
  checks foundation/replay and absence of several lot-summary phrases. The
  allocation early return and known-peer control above demonstrate why a
  single failure count or partial summary test cannot certify the whole run.
  This audit asserts all results for its small register; no helper refactor
  is included. #284 can rank a typed diagnostic assertion after #313's contract.
- **Independent checks catch different classes.** `lotReconciliationCheck`
  skips unavailable position basis, while `InvestmentReplayEquivalence`
  checks each stored lot against replay. Their overlap is justified by the
  known-peer damage control, not duplicate code to remove automatically.

## Validation

- Baseline focused resolution/non-sale/known-zero/diagnostic/reference-book
  race selection passed: **23 top-level tests**, app **10.100 s**, API
  **6.033 s**, db **2.319 s**.
- New lifecycle/damage consumers passed before tightening exact full-result
  assertions: app **10.839 s**. The final run below includes those assertions
  and all 24 restore comparisons.
- Final focused command:
  `GOCACHE=/tmp/rekenraam-go-build-cache ./scripts/go-backend.sh test -json -race -timeout=20m ./internal/app ./internal/api -run 'TestInvestmentDiagnosticAudit|TestReferenceBookInvestmentAuditSelfCheckAndRegister|TestUnknownNonSale|TestReplacingResolution|TestReversingResolution|Test.*KnownZero|TestReplacingResolutionBetweenZero|TestReplaySelfCheck|TestSelfCheckCatchesEachCorruption|TestUnknownCapitalReturnSelfCheck|TestSelfCheckDetectsDisposalAllocation|TestSelfCheckWritesNothing'`.
  **31 top-level tests passed**, app **21.700 s**, API **6.016 s**. Includes
  the unchanged #266 expected-figure register through its real HTTP consumer.
- Touched-file formatting, `go vet ./internal/app`, documentation-relative
  links and `git diff --check` passed.
- No self-check production code, schema, API or UI changed. No browser rebuild,
  large-book measurement or full shared gate was needed for this audit-only
  change. #313 needs the appropriate runtime/UI/migration/performance checks
  when implemented; #280 still owns final combined-app gates.
- Evidence is committed locally, without push or a claimed remote CI result.
