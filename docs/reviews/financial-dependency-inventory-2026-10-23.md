# Financial Relationship Inventory — 2026-10-23

[#391](https://github.com/sergeyfarin/rekenraam/issues/391) design review.
Runtime source: `4ad8373f772eb02b40126f5e9d746f0eb50c9e79`.
The accepted dispositions live in the
[governing plan](../plans/financial-dependency-simplification-plan.md#completed-family-dispositions-2026-10-23)
and [ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md).
This change delivers an audit and contracts; production code is unchanged.

The consistent model is **recorded effects, calculated allocations, repairable
explanations and preserved commitments**. These are distinctions in the existing
typed commands, not new tables, a relationship registry or a generic rules engine.
Dropping all links would lose real cost allocations, source identity and statement
commitments. Keeping every link as an authorization condition creates unnecessary
workflows. Necessity is established by a counterexample, not by a foreign key,
a repeated check or an unchanged aggregate balance.

## Coverage And Method

Every family requested in #391 is dispositioned below. Review starts at entry,
preview and correction services, follows the shared atomic writer and typed
facts/replay, then checks reporting, exports, self-check and discovery. Existing
named tests supply positive/negative accounting controls. New counterfactuals
run only in a disposable source snapshot. This is a bounded relationship review,
not a claim to have inspected every line or every product workflow in #390.

The original [four-case experiment](financial-dependency-comparison-2026-10-23.md),
[payment repair evidence](payment-attribution-repair-2026-10-23.md) and
[all-four-parent evidence](cash-in-lieu-parent-replay-2026-10-23.md) remain dated
records. In particular, #379's density measurements describe its historical
policy; they are not rewritten as evidence for retaining that policy.

## Relationship Matrix

“Required” below means required for the financial command, not permanently
immutable across all corrections. Every correction still needs eligible accounts,
valid exact numbers, book/type integrity, current versions and auditable history.
The controls in the next section make each retained boundary falsifiable.

| Family / relationship meaning | Authoritative effects versus explanation | Entry, correction and refusal at the pinned source | Disposition and bounded owner |
|---|---|---|---|
| Cash dividend and embedded withholding | Cash, known gross/tax, date/currency/accounts and payment shape are effects. Payer is explanation; holdings are not an input. | Entry allows no payer; replacement can assign/change but omission and null both preserve. Date/cash account/currency are fixed by service checks and a matching repository replacement-date check. Native inverse/replacement preserves components and reviews actual checkpoint effects. | Keep effects; allow explicit clearing and no payer consistently in #393. #395 owns actual financial field correction, distinct from neutral payer repair. Do not introduce ex-date/holding authorization. |
| Separate withholding | Tax cash outflow and `withholding_only` shape are effects; payer and a link to a dividend are explanatory. | Entry requires payer because shape shares its facts row; clearing is unavailable. No historical holdings check. | Remove coupling in #393 with typed nullable facts and migration; keep known tax distinct from income and unknown tax. |
| Dividend payment in lieu | Cash/expense or recovery is the effect; instrument/holding/ex-date explain it. | #388/#389 already allow optional/clearable attribution, no short-entitlement veto on entry, replacement or later position edits. Detail/self-check diagnose contradictions or unavailable history. Financial date/cash account/currency remain fixed by service checks and a matching repository replacement-date check. | Retain shipped attribution contract and read-only diagnostics. #395 owns financial field correction. No lot is manufactured for an unsupported cash explanation. |
| Fractional cash in lieu — split, exchange, spin-off, cash-and-stock merger | Fraction disposal, dated proceeds, elected quantity/basis and lot allocations are effects. Original parent is typed historical provenance. | #392 allows parent replacement/reversal if real replay succeeds. Existing disposal correction retains its original reference; new entry/lot lookup still require a current parent. | Keep actual disposal/election replay and immutable original reference. Relax residual entry standing consistently in #394; all four parent families require entry controls. |
| Reinvested dividend | Income funds a real acquisition, opening lots and an implied price. Holding/instrument/cost currency/date are financial inputs, not optional payer metadata. | Quantity/amount correction and reversal already use acquisition replay. Replacement freezes all four input fields even when there is no dependent sale/checkpoint. | Retain typed acquisition and family identity; allow field correction through existing old/new-position replay in #395. Do not downgrade to a cash dividend or freeze input values by analogy with required presence. |
| Fees and interest | Standalone fees/interest are ordinary expense/income postings. Trade charges additionally determine net settlement, basis/proceeds treatment, currency and posting lineage. | Ordinary postings need no trade, loan or provider parent. Trade commands snapshot the selected policy, retain exact components and correction dates, and validate linked clearing legs. | Keep settlement/component links where they allocate actual amounts; keep ordinary fees/interest independent. No general “related payment” relationship. Recompute trade allocations on financial correction. |
| Return of capital | Receipt is cash; entitled lot cohort, per-lot reduction and excess are actual basis allocations. Not every receipt is income. | Requires eligible dated lots; explicit elections stay explicit. Corrections replay reductions/excess and later disposals. Unknown basis is not zero. | Retain allocation lineage and conservation; recompute supported basis/cohort changes. Unknown reduction/excess stays unavailable. Do not apply cash-dividend optionality to calculation inputs. |
| Splits, exchanges, spin-offs and cash/stock mergers | Ratio, source/destination, allocation fractions and dated lot lineage change units/basis. Merger cash can realize elected basis. | Native correction replays actual dependent holdings/disposals. Unsatisfiable units or explicit source/cohort allocations refuse. Fixed cohort/election boundaries cannot guess a new owner choice. | Retain/recompute actual transformations and elections; reject blanket veto solely because a separately recorded fraction names the parent (#392 delivered). No generic sever-link repair. |
| Transfers and inbound basis resolution | Internal transfer links move quantity/cost between lots; external basis evidence can create a real bridge journal and revise sold/remaining basis. | Supported basis revisions propagate; removing required source lineage or onward units refuses. An effective inbound resolution must be reversed before reversing the transfer. | Keep real transfer allocation and resolution dependency. The existing resolution-reversal path withdraws its financial effect before transfer reversal. Do not retain a bridge for a nonexistent transfer. A composed command is not justified by this review. |
| Disposal/cover elections | Dated available units, FIFO/pool/specific election and selected lot ancestry determine cost/proceeds and realized result. | Buy/sale/short/cover corrections replay affected positions; impossible quantities or invalid explicit elections refuse with a named dependency. | Retain admission and explicit elections; recompute supported automatic allocations, disclose changed gains. An equal position total does not justify silently selecting another specific lot. |
| Provider/import identity and correction ancestry | Raw evidence and source revisions are immutable; owner classification/attribution can change. Once-per-payment identity is a trust commitment. | Imports preserve committed source ancestry and refuse duplicate effects. Payment-in-lieu provider linking follows correction ancestry; dividend linking only excludes an exact transaction ID, allowing a second event to claim a corrected descendant. | Retain source identity/dedupe; repair explanation without erasing evidence/reopening decisions. Strengthen #393 with the demonstrated dividend ancestry defect; no new issue per guard. |
| Prices and derived observations | Observations and their derivation legs establish valuation inputs; live selections are derived. They are not authorization for an otherwise valid cash payment. | Voiding an input cascades retirement of dependent observations atomically; excessive/cyclic/invalid dependency chains refuse rather than leave active stale derivations. | Retain exact derivation lineage and bounded atomic cascade; recompute live selections. Do not cascade financial journal reversals from a quote change. |
| Saved reports | Saved input/output bytes, hashes and profile are an explicit reproducibility commitment, separate from a live report. | Gains runs use an isolated snapshot; later writes do not mutate saved output or block those writes. Invalid saved hashes are integrity damage. | Retain immutable snapshots and provenance; let the owner create a new run. Reject a dependency from saved reports that freezes source corrections. |
| Reconciliation and ordering | Checkpoints commit to an account/commodity boundary and reviewed set, not every descriptive field. Root/same-day ordering affects dated allocation and boundary membership. | Existing compound writer nets each command at each checkpoint boundary. Neutral changes preserve checkpoints; changed balances/membership require bound review and invalidate the affected chain. | Retain actual commitment and stale-preview guards; avoid per-leg or book-wide confirmation on a neutral repair. Preserve correction-root order. |
| Mortgage/loan comparison | Cash, liability principal reduction and interest expense are real postings. An origination, amortization schedule or suggested allocation is not required to record owner-known amounts. | Ordinary ledger supports one balanced multi-posting payment. No loan schedule engine or mandatory origination/payment link exists. | Keep ordinary composition and explicit supplied principal/interest. No added subsystem, inferred amortization, mandatory schedule, special reversal family or new repair queue. |

## Source Owners And Falsifiable Controls

Paths are relative to `backend/internal/`. Each retained financial dependency has
a negative scenario below; tests are named so review can distinguish a real
invariant from a test that merely repeats a policy. No production test is deleted
or renamed by this audit.

| Family | Entry/correction and storage owners | Counterexample if the real dependency disappears; named controls |
|---|---|---|
| Dividend/tax | `app/investments.go`, `app/investment_dividend_correction.go`, `db/investment_foundation.go`, migration 9 | Conflating standalone tax with gross income changes classification while cash agrees. `TestDividendCoverageSeparatesKnownUnassignedAndCashOnlyIncome`, `TestReverseDividendRequiresReconciliationOverride`, `TestDividendReplacementRollsBackLateFailure`, `TestImportedDividendCorrectionPreservesCommittedIdentity`; first comparative probe's EUR 100/15/2. Payer clearing/nullable tax controls remain implementation acceptance in #393. |
| Payment in lieu | `app/investment_payment_in_lieu.go`, `db/investment_payment_in_lieu.go`, migration 23 | Reimporting a repaired payment posts twice without ancestry identity. `TestPaymentAttributionRepairPreservesProviderEvidenceAndPaymentIdentity`; `TestPaymentAttributionRepairPreservesReconciledCashAndOriginalFacts`, `TestReversePaymentInLieuRequiresReconciliationOverride`, `TestPaymentInLieuReplacementRollsBackLateFailure`. Entitlement-only refusals have already been dispositioned by #388. |
| All four fraction parents | `app/investment_cash_in_lieu.go`, `db/investment_cash_in_lieu.go`, migration 24; split/exchange/spin-off/merger correction writers | A 0.5 disposal cannot consume 0.3 units; a named lot cannot be replaced silently. `TestCashInLieuAllParentsRefuseInsufficientUnitsWithoutWriting`, `TestCashInLieuParentReversalRetainsSpecificLotAndLaterSaleDependencies`. Valid controls: `TestCashInLieuAllParentsReplaceByActualReplayAndRetainProvenance`, `TestCashInLieuAllParentsReverseWhenOtherUnitsSupportDisposal`, `TestCashInLieuAllParentCorrectionsReplayLaterDisposalsInRootOrder`. |
| Reinvestment | `app/investment_reinvestment_correction.go`, shared acquisition correction writer, `db/investment_buy_replacement.go` | Moving the only acquisition after a sale leaves no units; reversing it can invalidate a sale. `TestReverseReinvestedDividendImpossibleThroughLaterDisposalRollsBack`; retained date counterfactual below. `TestReplaceReinvestedDividendAmountAndQuantityReplaysDependentSale` proves valid recomputation and observed gain acknowledgement. |
| Fees/interest | `app/transactions_validate.go`, `app/investment_trade_economics.go`, `db/investment_foundation.go` | Matching a fee component to the wrong equal-valued clearing line can preserve totals but lose allocation identity. `TestTradeComponentLinksToItsOwnClearingLegWhenAmountsCollide`; `TestExactTradeEconomicsFeeTreatmentAndGrossPrice`; `TestForeignTradeFeeKeepsCostCurrencyBasisAndPostsOwnCashLegs`; `TestCorrectionMovingSeparatelyPaidFeeNetsPerBoundary`. |
| Capital return | `app/investment_capital_return.go`, `db/investment_capital_return.go`, typed allocation/revision facts | Cash 10 against basis 7 must reduce only 7 and expose excess 3; deleting lineage or treating it as dividend overstates basis. `TestCapitalReturnBeyondBasisReducesToZeroAndRecordsExcess`, `TestCapitalReturnAllocatesPerShareWithExactRemainder`, `TestBuyCorrectionRevisesCapitalReturnReductionAndExcess`, `TestExplicitEntitlementRefusesLotNotOpenOnEffectiveDate`. |
| Corporate actions | `app/investment_share_exchange_correction.go`, `app/investment_spin_off_correction.go`, `app/investment_cash_stock_merger.go`, `db/investment_replay_simulation.go` | Reversing an exchange can remove units already sold; a backdated acquisition changes the merger's explicitly elected cohort. `TestShareExchangeReversalNamesADestinationSaleWithoutWriting`, `TestBuyCorrectionBeforeCashStockMergerRevisesRealizedAndCarriedBasis`, `TestBackdatedPurchaseBeforeCashStockMergerIsRefusedWithMergerNamed`, `TestCashStockMergerConservesBasisPerLotAndSharesCashByUnits`. |
| Transfer/resolution | `app/investment_transfer_correction.go`, `db/investment_transfer_correction.go`, `db/investment_basis_resolution.go` | Erasing a resolved inbound transfer but retaining its EUR 80 bridge leaves journal support for absent units; dropping an onward link can lose transferred basis. `TestTransferCorrectionAfterResolutionReversal`, `TestReplacingResolutionRevisesPartialSaleAndKeepsEvidence`, `TestResolvingUnknownInboundReachesInternalTransferDestination`, `TestReverseTransferChainRefusesUpstreamUntilOnwardTransferReversed`, `TestBuyCorrectionCarriesChangedBasisToLinkedInternalTransfer`. |
| Disposal elections | `app/investment_buy_replacement.go`, `db/investment_replay_simulation.go`, typed disposal decisions/revisions | Moving a specifically elected lot to another holding may leave enough total shares but invalidates that election. `TestReplaceBuyRefusesInvalidatedSpecificLotElection`, `TestReplaceBuyDateAfterDependentSaleNamesSale`, `TestCorrectTrading212SalePreservesSpecificElectionWithoutGuessingQuantityChanges`. |
| Provider/import | `db/investment_provider_events.go`, `app/investment_dividend_events.go`, typed source revision acceptance/correction writers | Two source events claiming different versions of one payment violate once-per-payment identity despite unchanged cash. Retained provider probe below; `TestProviderDividendRevisionAfterLinkLeavesLinkAndPayment`, `TestDividendProducerRefetchKeepsOwnerDecisionsAndNeverDuplicates`, `TestCorrectTrading212SalePreviewAndSourceAcceptanceRollback`, `TestReplaceImportedBuyDatePreservesDedupeIdentity`. |
| Pricing | `app/pricing_management.go`, `db/pricing.go`, `db/pricing_management.go`, dependency index | A derived EUR/GBP quote built from a voided leg must not remain live. `TestPriceDependenciesAreImmutable`, `TestPriceDependencyIndexCannotFormACycle`, `TestVoidPriceObservationCascadesAcrossBranches`, `TestVoidPriceObservationRefusesAChainBeyondTheDepthLimit`. Failure must leave the whole cascade unchanged. |
| Saved reports | `app/investment_gains_runs.go`, `db/investment_gains_runs.go` | Reading live inputs during report persistence mixes states; mutating historical output destroys reproducibility. `TestGainsRunSnapshotIgnoresWritesBeforePersistence`, `TestGainsRunSnapshotFailureLeavesEveryFinancialRowUnchanged`. |
| Reconciliation/order | `app/reconciliation*.go`, `db/reconciliation*.go`, shared investment writer | Equal overall cash can cross a statement boundary; netting across checkpoints would hide it. `TestCorrectionMovingSettlementAcrossBoundaryInvalidates`, `TestEqualCorrectionAmountsAcrossSameDayBoundaryDoNotCancel`, `TestChainedCashNeutralCorrectionsPreserveCashCheckpoint`, `TestSalePreviewReportsEveryCheckpointCommitInvalidates`. |
| Ordinary loan payment | `app/transactions_validate.go`, ordinary transaction writer/versioning | Principal 900 plus interest 200 cannot be funded by cash 1,000. `TestDependencyInventoryLoanComposition` refuses this without writes. `TestCreateTransactionRejectsPostingsToSubledgerManagedAccounts` is the necessary control against using generic posting composition to bypass an investment lot. |

## Independent Results And Counterfactuals

Amounts are in EUR except the explicitly changed USD-cost case. Figures are
hand-derived, not taken from replay's own output. Original/repaired states use
effective facts; superseded journals and facts remain historical evidence.

| Scenario and independent arithmetic | Pinned runtime | Isolated check removal / supported runtime correction |
|---|---|---|
| Fraction entry after parent reversal: original buy 3 for 30, split 3:2, reverse split; dispose 0.5 for 8. Basis `30 / 3 * .5 = 5`, remainder 25; cash `-30 + 8 = -22`, gain 3. | Entry and preview refuse retired parent. Snapshot unchanged. | Removing only service/insert entry-standing checks permits preview=commit with quantity 2.5 and the expected basis/cash. Financial balance, replay, lot, foundation, checkpoint and SQLite checks pass; current parent remains advisory. Whole-share entry and a later 0.75 disposal against 0.5 remaining units refuse without writes. |
| Reinvestment date: income funds 10 for 20; January 1 corrected to January 2, with no intervening activity. | Blanket fixed-field check refuses; snapshot unchanged. | Removing that check preserves cash 0, quantity 10, basis 20 and income -20; preview writes nothing and commit passes self-check. Moving the sole acquisition after a January 3 sale still refuses atomically. |
| Reinvestment holding, instrument or cost currency changed individually. Same 10 units and 20 income-funded cost, with no dependent sale. | Each refuses fixed-field check. | Each succeeds with check removed: 10 units and basis 20 in the selected position/cost currency. Holding/instrument cases pass complete investment self-check; USD case checks original-currency accounting/replay/foundation, without claiming FX coverage or EUR equivalence. |
| Cash dividend and payment financial date/cash account/currency changed individually. Dividend gross 100, tax 15, net 85; payment expense/cash outflow 50. | Each refuses a fixed-field check. | Removing only those service checks and matching repository replacement-date checks permits each correction with exact typed cash effects and passing financial checks; changes stay in the selected original currency. No claim of FX conversion equivalence. |
| Provider: manual dividend 16, link July event; replace with same economics; stage second event for same instrument/date and link replacement. | **Second link succeeds.** No journal is added, original raw evidence is intact, but two events claim two versions of one payment. | Same defect with unrelated checks removed. Payment-in-lieu ancestry regression already rejects the corresponding case. #393 must apply that identity contract to dividends. |
| Loan payment cash -1,000, liability debit +800, interest +200. No schedule/origination link. | Ordinary transaction succeeds, exact postings balance. | No ablation needed. Changing principal to 900 without changing cash/interest refuses and leaves snapshot unchanged. Starting loan balance is independent of this payment delta. |
| Return of capital: 1 unit costing 7, receipt 10. | Existing control reduces basis 7 to zero, records excess 3 and cash 10; no disposal/income manufactured. | Existing supported corrections replay allocations and later sale. Removing all lot links would lose reduction/excess, so retain them. |
| Resolved inbound: 2 units, total basis 80, one sold. Correct basis to 60. | Actual resolution correction changes sold/remaining basis from 40/40 to 30/30, inverses 80 bridge and posts full 60 bridge, under gain review. | Existing transfer reversal refuses until resolution withdrawn; subsequent reversal succeeds. Merely deleting the relationship cannot withdraw the old bridge. |
| Trade buy 100, sell 120, fees 2 at each end. | Capitalized treatment: basis 102, proceeds 118, gain 16. Expensed: basis 100, proceeds 120, gain 20, separately recorded expense 4. | Cash alone is 16 in either case. Removing treatment/component lineage loses the distinction; keep the snapshotted actual election. |

The first four-case and #392 reports retain the independent before/after figures
for short opening/cover changes, dividend/tax attribution and **all four** parent
replacement/reversal, basis, order and election controls. They are part of this
review's evidence, not rerun or relabelled as newly implemented behavior.

## Consumer And Repair Contract

The adapters below consume the same typed financial contracts. Entries identify
their distinct surface/storage/export consequences, rather than treating a
service-only probe as a completed product change. Frontend paths are under
`frontend/src/lib/`; API routes/types are in `backend/internal/api` and
`api/openapi/`. Bundle assembly is `app/exports_bundle.go`; self-check
owners are `app/self_check.go`, `db/self_check.go` and typed replay checks.

| Family | UI/API discovery and correction | Storage, reporting/export and diagnostic consequence |
|---|---|---|
| Dividend / standalone tax | `investments/dividend-form.svelte`, `dividend-correction-form.svelte`, transaction history and dividend coverage; nullable payer decoding in native dividend requests. | #393 separates payment shape from payer, preserves tax components and coverage/bundle meaning. #395 exposes financial date/account/currency corrections with actual checkpoint review. No holdings diagnostic. |
| Payment in lieu | `investments/payment-in-lieu-form.svelte`, `attribution-diagnostic.svelte`, native replacement/detail/self-check. | Migration 23 nullable attribution already ships. Keep warning versus unavailable support, exact expense/recovery and historical export; #395 adds actual cash-field changes without turning warnings into write vetoes. |
| All four fractional parents | `investments/cash-in-lieu-form.svelte`, `split-activity.svelte`, transaction history, dated available-lots/preview/native disposal. | #394 needs a new insert-guard migration, not rewritten original facts. Parent context and warning remain separate from actual quantity/basis/gain, original-reference bundle export and integrity failures. |
| Reinvestment | Shared `investments/dividend-correction-form.svelte` currently shows fixed fields read-only; native acquisition-backed replacement/preview. | #395 enables corrected financial inputs, preserves operation family and original/replacement journals/prices; old/new holdings replay and gain coverage. No payer-only shortcut or cash manufactured. |
| Fees / interest / loan comparison | Ordinary transaction editor for independent postings; `investments/trade-economics-fields.svelte` for actual trade charges. | No new storage/loan API. Preserve postings and CSV/QIF; trade facts export snapshotted treatments and dates, with clearing-identity self-check. No mandatory origination or related-payment picker. |
| Capital return | `investments/capital-return-form.svelte`, `capital-return-summary.svelte`, native typed entry/correction and affected gains preview. | Preserve per-lot allocations/revisions and receipt versus reduction/excess reporting dates. Unknown basis remains unavailable; conservation failures remain errors. No optional-allocation schema. |
| Corporate actions / transfers / disposal elections | Existing native action/transfer forms, `investments/transfer-correction-form.svelte`, specific-lot selection and gain/reconciliation preview; name actual dependent operation on refusal. | Keep typed facts, elected source/destination ancestry and revisions in bundle/history. Preserve quantity/basis replay self-check; resolution withdrawal keeps bridge and lot knowledge consistent. No generic unlink endpoint. |
| Provider/import | `investments/provider-dividend-completion.svelte`, provider event review, `imports/dividend-review.svelte`, source and correction history; existing accept/link commands. | #393 corrects dividend ancestry exclusion and surfaces payer/source disagreement. Raw/normalized evidence and settled suggestion IDs stay immutable; report source coverage separately from owner attribution. Import source acceptance and journal stay atomic. |
| Prices / saved reports | `pricing/price-detail.svelte` impact review, `investments/gains-runs.svelte` and `save-gains-run.svelte`; existing observation/run endpoints. | Atomic derived-price retirement; unchanged saved snapshot bytes/hashes in export. Dependency/hash damage is integrity failure. Neither introduces a journal-correction blocker. |
| Reconciliation / ordering | Existing reconciliation screen, `investments/reconciliation-confirm.svelte`, per-command impact preview and correction history. | Preserve checkpoint links/boundaries and root slots; exports retain audit evidence. Neutral repairs need no new review. Changed commitments require current bound review; incomplete or corrupt evidence is not attribution uncertainty. |

- **UI/API:** optional cash explanation uses field presence end to end: omission
  preserves on replacement, explicit null clears, value assigns. New entry
  omission/null means unassigned. Retain valid typed identities when supplied;
  never use zero as an unknown instrument. Existing detail/correction history
  and on-demand diagnostics provide discovery; existing coverage pages expose
  unassigned cash. Historical-parent entry uses that parent's existing history
  surface. Reinvestment changes use the existing correction form and lot/gain
  preview. No new inbox, cascading manual checklist or recurring acknowledgement.
- **Writer/preview:** services plan typed effects, the existing writer verifies
  current versions/source commitments under the write transaction, replays affected
  positions and appends journal/subledger/observation/audit effects atomically.
  Preview runs the same operation with rollback. Review tokens bind actual gain
  and checkpoint consequences; neither an explanatory link nor a warning creates
  an acknowledgement requirement. Stale plans or late failures leave no effects.
- **Storage/history:** retain immutable original facts, historical foreign keys
  and correction roots. New schema meanings need forward migrations and explicit
  populated downgrade controls. #393 preserves tax-only shape independently from
  nullable payer. Historical-parent entry changes admission, not parent identity.
  Reinvestment corrections retain their typed family and correct original versus
  replacement journal/observation dates. Source acceptance follows the payment's
  ancestry, never only its newest transaction ID.
- **Diagnostics:** a known contradictory explanation is a scoped warning with
  reason and transaction/history link. Missing/unassigned history is unavailable
  support, not evidence of contradiction or zero. Malformed typed facts, missing
  journals, broken quantity/basis conservation or invalid report hashes remain
  integrity failures. #388/#389 deliver the payment diagnostics; #392 delivers
  advisory parent context. #393 must disclose source/payer mismatch or unassigned
  attribution without erasing the old source claim or falsely calling it current
  support. Do not add a historical-holdings diagnostic for ordinary dividends,
  interest or tax, whose entry does not assert entitlement.
- **Reports/export:** original-currency cash, known gross/tax, quantity, basis and
  fees stay exact. Unassigned payer does not erase tax; unknown imported gross/tax
  stays unknown rather than zero. Instrument totals disclose missing attribution;
  unknown basis withholds dependent gain/reduction/excess. Existing bundle facts
  preserve original source/parent/correction history, with compatible schema
  version handling when meaning changes. Saved runs retain their original bytes;
  live reports recompute. No quote or report link freezes a valid cash correction.

## Delivery Boundaries And Test Policy

Accept historical-parent entry (#394) and typed financial-field changes (#395)
as bounded runtime follow-ups linked from the governing plan. Strengthen #393 for the
observed source-ancestry defect and mismatch visibility. Retain the other
relationships for the concrete counterexamples above; do not create tickets
for each existing guard or for unproven simplification. #388/#389/#392 remain
completed runtime deliveries. #391 can close when this inventory/contract and
its evidence are accepted; these separate runtime follow-ups remain open.
#390's broader product/workflow audit and #284's mechanical audit remain open.

`TestCashInLieuOfReversedSplitIsRefused` is a current-policy test, not an
accounting oracle. Its changed expectation belongs with the historical-entry
implementation and new four-parent controls. The reinvestment fixed-field
check has no dedicated field-change regression at this source; the future
implementation must add meaningful positive/negative controls. Existing typed
family, allocation, source, checkpoint and rollback tests remain. The provider
characterization intentionally asserts the observed defect in the disposable
probe; #393 must invert that expectation in a production regression. No test
is removed here, and no policy change is smuggled in by relaxing a test alone.

## Reproduction And Validation

```sh
python3 docs/reviews/fixtures/financial-dependency-inventory-2026-10-23.py
```

[Retained output](financial-dependency-inventory-2026-10-23.txt),
[probe](fixtures/financial-dependency-inventory-2026-10-23_test.go.txt) and
[runner](fixtures/financial-dependency-inventory-2026-10-23.py) preserve source
and probe identity. The retained run passes 13 leaf scenarios in each baseline/ablation and 51
existing named accounting/trust controls under race mode. Document targets,
source owners, named controls and `git diff --check` pass; the structural audit
is clean with unchanged OpenAPI regeneration explicitly skipped.
The runner uses `git archive` in `/tmp`, provider-free test
fixtures, native SQLite and race mode; it does not access the owner's database.
Only CIL entry standing and typed fixed-field admission checks change in the
isolated copy; original/inverse date and source/version checks remain.
Available-lots/UI admission remains unmodified: its consistent adaptation is follow-up acceptance,
not claimed proved by a service/insert probe. All-four-parent historical *entry*
and typed field changes with complex downstream transfers remain implementation
controls; earlier #392 controls establish correction behavior, not those new paths.

Runner development exposed matching repository date-policy checks after the
service check was removed; those date-only checks are explicitly included in the
final ablation. Early fixture assertions also needed scale-aware cash summation:
the loan helper returns cents and a dividend has separate gross/tax cash postings.
Correcting those probe oracles yields the figures above. This was a probe
error in the probe rather than a production defect. No runtime, frontend, migration,
API or dependency code changes in this audit, so full build/browser/resource
suites are not claimed rerun. Focused existing accounting controls and document
validation are recorded in the retained output. No performance claim follows
from these probes.
