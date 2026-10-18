# Investment operations, part 2 audit — 2026-10-18

**Status:** [#273](https://github.com/sergeyfarin/rekenraam/issues/273) executed;
closure remains blocked by [#308](https://github.com/sergeyfarin/rekenraam/issues/308).
The operation/correction/backdating matrix and atomic refusal evidence are
recorded below. Known-basis sequences pass replay equivalence and self-check;
supported unresolved basis still makes `lot_reconciliation` fail, so the
original clean-self-check acceptance remains unmet. Do not mark it complete
by substituting a selected-check assertion for an overall passing run.

Three small findings are fixed: #307 documentation, #309 exact source-number
comparison, and #310 share-exchange evidence handling. #311 remains an open
P2 browser source-metadata defect. This is part of #260, not a release or
tax-policy certification.

## Baseline and evidence boundaries

Audited baseline `5ba352af324aea2ea7071594917abc3a6b662a26`, initially clean
on `main`. Documentation correction: `2af1295d`; server corrections and their
fail-first regressions: `f9be0019`. These commits and this report are local,
not pushed; no remote CI run is claimed.

Reviewed the accepted slice-5, operation, unknown-basis and unresolved-non-sale
contracts, ADRs 0012/0013/0017, the capability/replay-intent matrices, actual
service and repository writers, HTTP consumers and phone correction forms.
All executions use synthetic provider-free books and temporary databases.
The browser harness redirects market data to its local stub; no live provider
semantics are certified.

The [#266 reference book](reference-book-audit-2026-10-17.md) and its unchanged
hand-derived register were rechecked through HTTP. That book intentionally
does not contain internal transfers, exchanges, spin-offs, mergers or their
corrections. Rather than pretend otherwise, this audit adds the exact
cross-position register below through the existing service fixture, and
reuses the named HTTP and browser consumers for those families.

## Family × correction × backdating matrix

**V** = verified in the named consumers, **L** = explicit supported limit or
named refusal, **D** = confirmed defect with linked disposition. Every V
below passed; severity is none unless a D or open finding is stated.
“Correction” includes replacement and reversal, with the original facts kept.
The matrix covers admitted long operations. These transfer/corporate-action
commands do not restructure named shorts; short opening/cover corrections
belong to #272 and are not implied by a long result.

| Family | Entry and independent invariant | Replacement / reversal | Backdating and later dependents | Disposition and principal consumers |
|---|---|---|---|---|
| External inbound, known / zero / unknown | V: security quantity enters; only known nonzero basis posts equity/clearing bridge; original acquisition date is separate | V: replace quantity/basis/date; reverse retires lot and inverts bridge. L: resolve/withdraw sourced basis before correcting a resolved inbound | V: older carried acquisition date reorders later FIFO sale; reduced quantity that strands a sale refuses | `TestReplaceExternalTransferInCorrectsBasisAndAppendsBridge`, `TestReverseExternalTransferInRetiresLotAndInvertsBridge`, `TestBackdatedTransferInFromOldBrokerReplaysCurrentBrokerSaleByOriginalDate`, `TestReplaceExternalTransferInBelowLaterSaleRefusedWithoutWriting` |
| Internal selected lots | V: equal source/destination quantity and basis, no cash/realization; individual-lot family remains locked | V: replace quantity/date/destination; reverse restores source, retires destination lots. L: consumed destination units/onward transfer must be unwound first | V: source and destination sales replay together at the historical slot under today's lock; impossible later operation is named | `TestInternalTransferPartialLotConservesQuantityBasisAndJournal`, `TestReplayKeepsSelectedLotTransferMethodLock`, `TestReplaceInternalTransferQuantityReplaysDestinationSale`, `TestBackdatedInternalTransferReplaysSourceAndDestinationSalesTogether`, `TestReverseTransferChainRefusesUpstreamUntilOnwardTransferReversed` |
| Internal average pool, `pooled_lot` | V: move quantity at dated pool rate; one stable destination lot, latest moved original date | V: reversal restores exact pool; replacement may change lineage; upstream corrections revise carried basis/date/depletion and destination sale | V: changed source lineage is admitted while destination ID/quantity stay fixed; chained positions replay | `TestPooledLotTransferOpensOneDestinationLotWithLatestUnitDate`, `TestBackdatedBuyRevisesPooledLotTransferAndDestinationFIFOSale`, `TestReversePooledLotTransferRestoresPoolBasisExactly`, `TestBackdatedInternalTransferIntoChainedHoldingRevisesDownstreamPooledLot` and the new register |
| Internal average pool, `source_lots` | V: same aggregate pool basis, one destination lot per source depletion | V: correct/reverse; changed basis propagates. L: a replay that changes fixed source lineage/date is refused | V: backdated transfer uses its slot; L: backdated acquisition replacing fixed lineage names the transfer and writes nothing | `TestBackdatedPooledInternalTransferReplaysLaterAverageSale` (both lineages), `TestReplaceSourceLotsTransferAsPooledLotAdmitsBackdatedBuy`, new register's `source_lots` case |
| External outbound, selected / pooled / unknown | V: security out; known bridge equals depleted basis; any unknown link suppresses the whole bridge | V: reversal cancels net original-plus-adjustment bridge; replacement depletes at root slot and rebridges | V: backdated outbound restates later sale; changed carried basis appends dated bridge adjustment. L: changed fixed pooled lineage or known→unknown bridge refuses | `TestExternalTransferOutFromAverageCostPoolCarriesPoolRate`, `TestReverseExternalTransferOutRestoresUnitsAndCancelsNetBridge`, `TestReplaceExternalTransferOutRebridgesCorrectedQuantityAtItsSlot`, `TestBackdatedExternalTransferOutReplaysLaterSale`, `TestReplayRefusesToChangeAKnownOutboundToUnknown` |
| Split / reverse split, including journal-free | V: quantity × exact ratio, total basis unchanged; no fractional rounding or invented zero journal | V: ratio/date replacement, reversal of primary-plus-adjustment quantity, correction across zero, operation identity without primary transaction | V: later sale restates; backdated acquisition appends split adjustment journal. L: impossible fraction/sale refuses | `TestSplitThreeForTwoWidensScaleExactly`, `TestSplitReplacementCorrectsRatioAndRevisesLaterGain`, `TestSplitOperationReplacementCrossesZeroBothWays`, `TestBackdatedAcquisitionEntitledToJournalFreeSplitPostsAdjustment`, `TestSplitOperationCorrectionsExportAndRestoreBothJournalShapes` |
| Share exchange / all-stock merger | V: all eligible old units become exact-ratio new units; entire basis and original dates carry; no cash/gain | V: native reversal/replacement. D: unchanged supplied evidence accepted and omitted evidence dropped, fixed #310 | V: subject `exchange_out` takes the whole holding at the dated/root slot and replays either instrument. L: later unsatisfied sale, onward move or settlement is named | `TestShareExchangeCarriesBasisAndDatesAndRealizesNothing`, `TestShareExchangeReplacementMovedLaterTakesTheWholeHoldingThen`, `TestShareExchangeBackdatedBehindANewInstrumentSaleRevisesIt`, `TestShareExchangeReversalNamesAnOnwardTransfer`, new #310 regression |
| Spin-off | V: parent units unchanged; elected basis fraction moves to ratio-derived child units; parent+child basis conserved per lot; unknown stays unknown | V: native ratio/fraction/instrument/date replacement and reversal. D: exact source-number change rejected, fixed #309; D: browser rounds unedited metadata, open #311 | V: subject `spin_off` entitles historical parent lots; parent and child later sales replay. L: fixed lineage/new units that cannot be supplied refuse | `TestSpinOffDividesBasisKeepsUnitsAndRealizesNothing`, `TestSpinOffTruncatesTheAllocationAndConservesEachLot`, `TestSpinOffCarriesUnknownBasisAndResolvesThroughReplay`, `TestSpinOffBackdatedBehindAParentSaleRevisesIt`, `TestSpinOffReversalNamesANewInstrumentSaleWithoutWriting` |
| Cash-and-stock merger | V: elected realized basis + carried basis = original; explicit cash date and quantity-zero cash result remain separate from exchanged units; unknown result NULL | V: replace election/cash/payable date/ratio/instrument; reversal inverts both journal entries and removes cash result under acknowledgement. D: exact evidence comparator fixed #309; sibling browser metadata path requires #311 | V: subject `exchange_out` carries cash election at historical slot; old/new later sales replay. L: unsatisfied sale/onward transfer/cash-in-lieu dependency refuses | `TestCashStockMergerRealizesElectedBasisAndCarriesTheRest`, `TestCashStockMergerFractionZeroAndOneAreElections`, `TestCashStockMergerUnknownBasisLeavesResultUnresolved`, `TestCashStockMergerReplacementRestatesTheCashResultWithAcknowledgement`, `TestCashStockMergerBackdatedBehindAnOldHoldingSaleRevisesIt`, new register |
| Sourced inbound basis resolution, positive / zero | V: full omitted bridge at transfer date; known zero is journal-free, distinct from NULL | V: economic replacement/withdrawal replay reached positions; equal-basis provenance correction retains financial operation. L: withdrawal that would turn known outbound/onward link unknown refuses | V: partial/closed disposals, splits, transfers and outbound bridges take revised basis; original declaration/knowledge remain immutable | `TestReplacingResolutionRevisesSplitInternalDestinationAndBridgedOutbound`, `TestReversingResolutionRestoresUnknownBasisAndUnresolvedSale`, `TestReversingResolutionRefusesKnownDependentWithOperationNamed`, `TestProvenanceCorrectionPreservesDownstreamSalesTransfersAndCheckpoints` |
| Buy / sale trade-field correction | V: corrected account/instrument/currency/date changes the appropriate positions and dated journals | V: inverse and replacement under one command; specific elections and import identities remain pinned | V: root same-day slot retained, source/destination positions replay. L: impossible dated sale/transfer/elected lot refuses | `TestReplaceBuyMovesTradeDateEarlierAndRestatesLaterSale`, `TestReplaceBuyMovesHoldingAccountAndReplaysBothPositions`, `TestReplaceBuyChangesInstrumentAndCostCurrency`, `TestReplaceSaleDateKeepsOriginalSameDaySlot`, `TestReplaceBuyAcrossPositionsRollsBackLateFailure` |
| Cash / reinvested dividend correction | V: cash dividend changes cash/income/withholding, no lots; reinvestment changes acquisition basis/units without cash | V: native replace/reverse, including removed/replaced gain disclosure where reinvested units were consumed | V: dividend is lot-free; historical reinvestment/correction uses acquisition replay and can restate later sales | `TestReplaceDividendCorrectsAmountAndWithholding`, dividend correction API tests, `TestReinvestmentGainImpactRequiresAcknowledgement`, `investments-dividend-correction.spec.ts` |
| Unresolved write-off / cash in lieu / return of capital (#169) | V: known cash/quantity/entitlement preserved; NULL loss/result or NULL reduction/excess; known independent entitlement remains calculable | V: native correction, resolution replacement/withdrawal, reversal and original NULL evidence preservation | V: dated replay through later sales and resolution after closure. L: overall diagnostic is not green while projected basis is unknown | `TestUnknownNonSaleMixedKnownUnknownAndAverageDisposals`, `TestUnknownNonSaleCorrectionWithdrawalAndReversal`, `TestUnknownNonSaleCheckpointsAndLateActionRollback`, `TestUnknownNonSaleExportResolutionAndRestoreRetainOriginalNulls` |

Fixed-lineage dependency refusals are the admitted contract, not missing
automatic corporate-action expansion. Rights, tenders/partial redemptions,
tax-wrapper sale/buy reclassification and automatic provider corporate-action
mapping remain outside these families. #136/#145/#176/#256 are adapter-only
evidence gaps, not blockers to this audit.

## Independent cross-position register

New consumer:
[`TestInvestmentAuditTransferCorporateActionChain`](../../backend/internal/app/investment_operations_2_audit_test.go).
Two cases use `pooled_lot` and `source_lots`. Values below are literal
expectations, compared as rational numbers through real lot/gain readers and
posted cash balances, with no floating point and no expected amount calculated
by production replay. Quantity/basis are displayed as `units / EUR basis`.

| Command and hand derivation | A parent | B parent | B child | B merged | Net cash | Total realized |
|---|---|---|---|---|---|---|
| Buy 10 for 100 | 10 / 100 | 0 / 0 | 0 / 0 | 0 / 0 | −100 | 0 |
| Buy another 10 for 300; total 20/400 | 20 / 400 | 0 / 0 | 0 / 0 | 0 / 0 | −400 | 0 |
| Transfer 8 at 400/20 = 20 per unit; carry 160, retain 240 | 12 / 240 | 8 / 160 | 0 / 0 | 0 / 0 | −400 | 0 |
| Split B 2:1; units double, basis stays 160 | 12 / 240 | 16 / 160 | 0 / 0 | 0 / 0 | −400 | 0 |
| Spin off 1 child per 2 parent, move 25% of 160 = 40 | 12 / 240 | 16 / 120 | 8 / 40 | 0 / 0 | −400 | 0 |
| Merge parent 1:2, cash 40 five days later, realize 50% of 120 = 60; carry other 60 | 12 / 240 | 0 / 0 | 8 / 40 | 8 / 60 | −360 | −20 |
| Sell 4 merged for 80; basis 60×4/8 = 30, gain 50; cumulative gain −20+50 | 12 / 240 | 0 / 0 | 8 / 40 | 4 / 30 | −280 | 30 |
| Correct second buy 300→500: total 600/20; transfer carries 240, child 60, merger realizes 90/carries 90, sale consumes 45. Gain (40−90)+(80−45) | 12 / 360 | 0 / 0 | 8 / 60 | 4 / 45 | −480 | −15 |
| Add earlier buy 10/600: pool 30/1200; transfer carries 8×40=320, retain 880; child 80, merger realizes/carries 120, sale consumes 60. Gain (40−120)+(80−60) | 22 / 880 | 0 / 0 | 8 / 80 | 4 / 60 | −1080 | −60 |

**Actual:** all rows match in the pooled-lot case. The fixed `source_lots`
case matches through the price correction, then refuses the last command
with the moved transfer's operation ID (`InvestmentBuyDependencyError` /
`INVESTMENT_BUY_DEPENDENCY`); every snapshotted durable row remains unchanged.
That is L, not a changed expected amount. Each accepted command and the
refusal are followed by the conservation/effective-movement oracles and an
overall passing self-check, including rolled-back per-position replay.
Both changed downstream decisions are disclosed before the final pooled buy;
the preview itself leaves no durable financial change.

This compound sequence checks figures at each intermediate state, before
a later correction could retire the revision being audited. Existing named
tests separately verify multi-hop/cyclic closure, source-lot original dates,
same-day root ordering, unknown knowledge and journal adjustments; those
claims are not inferred merely from the aggregate register.

## Refusal codes and failure atomicity

New HTTP consumer:
[`TestInvestmentAuditRefusalsPreserveExportedFinancialState`](../../backend/internal/api/investment_operations_2_audit_test.go).
It compares every exported CSV before/after **13** rejected requests. The
manifest's export timestamp is excluded. Existing service tests additionally
snapshot durable journals, lots, revisions, audit, checkpoints, prices, queue
and import identities. Export comparison proves unchanged exported financial
state, not a byte-for-byte database image or exported-bundle reconstruction.

| Refusal scenario | Observed status / code | Atomicity evidence and actual result |
|---|---|---|
| 3 units × 1/7 in split, exchange, spin-off and merger; preview and commit | 422 `INVESTMENT_SPLIT_FRACTION_UNREPRESENTABLE`, `INVESTMENT_EXCHANGE_FRACTION_UNREPRESENTABLE` (exchange and merger), `INVESTMENT_SPIN_OFF_FRACTION_UNREPRESENTABLE` | Eight new HTTP requests; all CSV data identical |
| Reverse a 2:1 split after selling all 6 units | 409 `INVESTMENT_SPLIT_DEPENDENCY` | New HTTP snapshot; `TestSplitReversalRefusesImpossibleLaterSaleAtomically` names the dependent sale and compares durable rows |
| Insert internal move before a full later sale | 409 `INVESTMENT_SALE_DEPENDENCY` | New HTTP snapshot; `TestBackdatedInternalTransferThatBreaksLaterSaleIsRefusedWithSaleNamed` and the later-source-transfer case pin dependency identity |
| Reverse the acquisition under dependent split/sale | 409 `INVESTMENT_BUY_DEPENDENCY` | New HTTP snapshot; trade-field and transfer-lineage tests name the impossible operation |
| Reverse sale with missing or stale gain token | 409 `INVESTMENT_GAIN_IMPACT_ACKNOWLEDGEMENT_REQUIRED` / `_STALE` | Two new HTTP snapshots; writer has no surviving inverse, replay revision or effective gain change |
| Reverse consumed exchange/spin-off/merger or transfer; onward transfer or settled cash in lieu | 409 `INVESTMENT_TRANSFER_DEPENDENCY` | Named corporate-action reversal/API and `…NamesAnOnwardTransfer` / `…NamesASettlingCashInLieu` service cases; durable row snapshots unchanged |
| Withdraw basis that would invalidate known outbound/onward basis; correct inbound while resolved | 409 `INVESTMENT_TRANSFER_DEPENDENCY` / `INVESTMENT_TRANSFER_BASIS_RESOLVED` | `TestReversingResolutionRefusesKnownDependentWithOperationNamed`, resolution/transfer correction consumers; originals and bridges unchanged |
| Missing reconciliation approval or changed reviewed checkpoint set | 409 `CONFLICT` (reconciliation override required) / `RECONCILIATION_IMPACT_STALE` | Existing transfer/split/exchange/spin-off/merger correction tests and #304 HTTP binding consumers; no checkpoint invalidation on refusal |
| Unchanged source/terms after #310; genuinely distinct large source number after #309 | 400 `VALIDATION_FAILED` for no-op; valid correction succeeds | Fail-first real-service regressions, existing HTTP validation envelope, fixed consumers preserve source digits and self-check; #311's browser rewrite remains separate |

Late failures are exercised after compound effects, not only at request
validation: `TestTransferPropagationRollsBackOnLateRefusal`,
`TestReplaceTransferLateReconciliationRefusalRollsBackEverything`,
`TestReplaceExternalTransferInLateReconciliationRefusalRollsBackEverything`,
`TestOutboundBridgeAdjustmentPreviewsAndRollsBackWithItsCommand`,
`TestSplitCorrectionRollsBackLateFailure`,
`TestJournalFreeSplitReplacementLateFailureRollsBackBothHeaders`,
the three corporate-action `…ReversalLateReconciliationRefusalRollsBackEverything`
cases, `TestResolutionCorrectionLateRefusalsRollBackEverything`, and
`TestUnknownNonSaleCheckpointsAndLateActionRollback`. All passed.

This records domain refusals in the audited scenarios; it does not claim
every possible malformed JSON/field combination has been enumerated.

## Findings, fixes and retained limits

| Finding | Severity, independent reproduction and actual result | Disposition |
|---|---|---|
| Active docs still refused shipped unresolved non-sale states | P3 #307; contradicted accepted #169 contract and passing service/API/six-locale browser consumers | Fixed `2af1295d`: current ledger and slice-5 contract corrected; historical ADR progression and dated reports retained |
| Spin-off/merger comparator decoded metadata numbers as floats | P3 #309; `TestCorporateActionCorrectionDistinguishesExactSourceNumbers` failed on both real previews: `9007199254740992`→`9007199254740993` incorrectly called unchanged | Fixed `f9be0019`: reuse exact-number provenance comparison. Both corrected declarations survive exactly; quantities/basis/cash and replay remain correct |
| Share-exchange correction accepted unchanged supplied evidence and lost omitted evidence | P3 #310; new real-service regression first accepted reordered identical JSON, then (after no-op guard) returned `{}` instead of the original declaration on a genuine ratio correction | Fixed `f9be0019`: pin source in the operation read, inherit when omitted, compare effective exact evidence. Preview/commit no-op writes nothing; genuine ratio correction retains digits and passes self-check |
| Browser correction silently rounds unedited source metadata | P2 #311; real phone spin-off probe seeded raw `9007199254740993`, changed only 10%→20%, then read raw successor response with `9007199254740992`. Desired-behavior assertion failed after a successful commit | **Open confirmed defect.** Original history remains; no incorrect quantity/basis/cash total confirmed. Merger uses the same parsed-object spread and requires the same sweep; share-exchange source reconstruction also needs assessment. Not fixed by the server comparator |
| Overall self-check is not clean for supported unknown basis | Existing documented diagnostic, no newly asserted corruption severity; #266 HTTP consumer reports exactly one failed `lot_reconciliation` finding, every other applicable check passes, register unchanged | **Acceptance blocker #308**, coordinated with #277. Known-basis register is all green; unknown transfer/non-sale tests verify foundation/replay/conservation while retaining the explicit unknown diagnostic |

The [browser probe](fixtures/investment-source-metadata-browser-probe.ts.txt)
is retained as an executable reproduction appended to the existing spin-off
spec; it is deliberately not installed as a failing normal test. Its temporary
addition was removed after the run. #311 must turn it into a passing permanent
regression when its evidence-preservation contract ships.

## Additional audit and closure handoff

[#308](https://github.com/sergeyfarin/rekenraam/issues/308) makes the diagnostic
assessment already requested by #272/#277 separately actionable: unknown →
positive/zero resolution → replacement/withdrawal, partial/closed positions
and non-sale actions, compared with deliberate quantity/allocation/link/bridge/
replay damage and representative native restore. Publish the expected check
statuses and governing-plan acceptance disposition; do not hide corruption
with a blanket unknown exemption or silently call an unavailable check passed.

Until that disposition or separately scoped implementation meets the original
acceptance, #273's matrix and refusal boxes can be checked, but its clean
self-check box remains unchecked. #311 is an explicit unresolved finding that
can be tracked separately under the shared evidence contract; its impact stays
visible in #260. No duplicate provider audit is required. #277 retains bundle
reconstruction/restore and #280 retains the shared final gates/performance.

## Maintainability notes

- **Corporate-action correction orchestration diverges across siblings.**
  `investment_share_exchange_correction.go`, `investment_spin_off_correction.go`
  and `investment_cash_stock_merger_correction.go` independently prepare inverse
  and replacement, preserve source, detect no-op and clear successor gain
  policy. #309/#310 are observed divergence, not a file-length concern. #284
  should rank any consolidation only against these behaviors and regressions.
- **Source preservation is enforced in both browser and service layers.**
  The spin-off/merger forms spread parsed metadata; the services separately
  inherit/compare source declarations. #311 demonstrates that exact server
  comparison cannot restore a number already rounded in the browser. Basis
  provenance has a server merge for this class. A source-shape change affects
  request types, form initialization, preview, commit, history and export.
- **Durable-row snapshot coverage is a manually maintained list.**
  `buyReplacementPreviewSnapshot` in `investment_buy_replacement_preview_test.go`
  lists financial/revision tables independently of migrations; it does not
  include `investment_split_facts`. The new HTTP audit compares all exported
  CSVs as a second observation surface. Neither helper is an automatic proof
  that every newly added durable table is covered; #280/#284 should account
  for that maintenance burden, without a snapshot-harness refactor here.
- **Unknown information and corruption share a failed check status.**
  `lotReconciliationCheck` in `self_check.go` counts unknown projected basis
  alongside mismatches; `requireHealthyUnresolvedBook` therefore asserts
  selected checks/diagnostics. This is the concrete acceptance/test obstacle
  recorded by #272 and made explicit in #308, not a new speculative invariant.

## Validation

- Baseline focused race command:
  `./scripts/go-backend.sh test -race -timeout=20m ./internal/app ./internal/api ./internal/db -run 'Transfer|Split|ShareExchange|SpinOff|CashStockMerger|CashAndStockMerger|Resolution|ReplaceBuy|ReplaceSale|ReplaceDividend|ReverseDividend|Replay|Backdat|Conservation|OperationKind|ReplayIntent|ReferenceBook'`.
  App passed **58.961 s**, db **34.838 s**. The API attempt hit sandbox
  localhost restrictions; its permitted API-only rerun, with the same relevant
  transfer/corporate-action/correction/reference-book selectors, passed
  **16.433 s**. No product failure is inferred from the environment refusal.
- Additional unresolved/non-sale, self-check damage, checkpoint binding,
  independent-register and reference-book race selection: **121 top-level
  tests passed**, app **25.210 s**, API **10.303 s**.
- After both server fixes:
  `GOCACHE=/tmp/rekenraam-go-build-cache ./scripts/go-backend.sh test -race -timeout=20m ./internal/app ./internal/api ./internal/db -run 'ShareExchange|SpinOff|CashStockMerger|CorporateActionCorrectionDistinguishesExactSourceNumbers|InvestmentAudit|ReferenceBookInvestmentAudit'`.
  **90 top-level tests passed**: app **17.586 s**, API **7.616 s**. The db
  package compiled, with no matching tests in this final narrow selection;
  its wider replay/transfer tests passed in the baseline run above.
- Initial browser selection: **39 passed, 1.8 min**, including pooled transfer,
  transfer reversal/outbound/inbound backdating, split, exchange, spin-off,
  merger, trade-field/dividend corrections and six-locale unresolved non-sale.
  Rebuilt integrated binary; catalog gate **66 passed** and embed test passed.
- After #309, **12 spin-off/merger browser cases passed, 1.2 min**. The
  separate #311 desired-behavior probe failed by its exact-metadata assertion
  (**1.6 s**), not setup/compile/selector failure; it is an open finding.
- After #310, the three affected corporate-action browser suites passed
  against a rebuilt binary: **17 tests, 1.4 min**, including share-exchange
  corrections. Catalog gate **66 passed** and embed test passed again.
- Focused vet for app/API/db, touched-file Go formatting, relative documentation
  links and `git diff --check` passed. A read-only Go-cache attempt was rerun
  using the writable `/tmp` cache; environment failures are not passing tests.
- No full backend/frontend/browser gates were run per area. #280 owns those
  once at its candidate SHA. No live provider, C-engine race instrumentation,
  native-language sign-off, tax correctness, exhaustive interleaving or
  large-book performance claim follows from this audit.
