# Investment operations audit, part 1 — 2026-10-18

**Execution:** [#272](https://github.com/sergeyfarin/rekenraam/issues/272),
under [#260](https://github.com/sergeyfarin/rekenraam/issues/260).
No new financial defect was reproduced. The operation matrix and exact-figure
checks below passed. **#272 stays open:** its last acceptance item also asks
for an all-green self-check with unknown basis and an exported-bundle round
trip. The present diagnostic deliberately flags unknown basis, and there is
no bundle importer. Those qualifications must not be reported as passing.
The recovery audit [#277](https://github.com/sergeyfarin/rekenraam/issues/277)
needs the additional checks described below.

## Baseline and method

Audited `c11fb917299f055ff5f237850df7cc9dd110e5ad` on `main`, initially clean.
Only tests and this evidence/roadmap documentation are changed by this audit.
The [#266 reference book](reference-book-audit-2026-10-17.md) was replayed
through `TestReferenceBookProducesItsRegister`; its independent figures were
unchanged. Its existing unknown lot is held, not sold, so the unresolved-sale
and resolution tests below supply that coverage.

All books are synthetic. Provider consumers use local stubs; no live broker,
market-data account or credential was required. ADRs 0012/0013 and the
[operation](../plans/investment-operation-refactor-plan.md),
[unknown-basis](../plans/investment-unknown-basis-plan.md) and
[non-sale](../plans/investment-unresolved-non-sale-plan.md) contracts govern.
Transfers/corporate-action sequences and the general correction/replay audit
remain #273; only prerequisites for the operations here were exercised.

## Capability matrix

V = **verified** through the named consumers below. A = **accepted limit**.
N = **not admitted** by the named-operation contract. Columns explicitly cross
side, knowledge and FIFO/average cost. LIFO and specific-lot evidence is also
recorded where the command takes a method. A method does not reinterpret a
lot-free cash payment or an acquisition's immutable opening facts.

| Operation | Long known FIFO | Long known average | Long unknown FIFO | Long unknown average | Short known FIFO | Short known average | Short unknown FIFO | Short unknown average |
|---|---|---|---|---|---|---|---|---|
| Buy/open position | V | V | V: known buy beside unknown | V: known buy beside unknown | V: named short sale | V: named short sale | N: opening proceeds required | N: opening proceeds required |
| Partial/full sale or cover | V | V | V: result unresolved | V: pool result unresolved | V: named cover | V: named cover | N | N |
| Trade fees | V | V | V: known acquisition/fee evidence; unknown disposal basis stays unresolved | V: same; no partial pool rate | V | V | N | N |
| Reinvested dividend | V | V | V: new known lot beside unknown | V: new known lot beside unknown | N: long acquisition; side-conflict guard | N | N | N |
| Resolve sourced inbound basis | V: unknown→known | V: unknown→known pool | V: original evidence stays unknown | V: original evidence stays unknown | N: no unknown short inbound | N | N | N |
| Write-off and native correction | V | V | V: zero proceeds, NULL result | V: zero proceeds, NULL result | N: long lots only | N | N | N |
| Return of capital | V | V | V: exact receipt, unresolved reduction/excess | V: no rate inferred from known subset of unknown pool | N: long lots only | N | N | N |
| Cash in lieu | V | V | V: exact fraction/proceeds, unresolved result | V: unresolved pool | N: long lots only | N | N | N |
| Cash dividend / withholding | V: lot-free | V: lot-free | V: no basis calculation | V: no basis calculation | A: cash dividend does not settle a short's obligation | A: same | A: cash operation independent of basis | A: same |
| Payment in lieu / recovery | N: requires pre-ex-date short entitlement | N | N | N | V: lot-free expense | V: lot-free expense | N: no admitted unknown short | N |

The unknown fee cells combine two independently verified boundaries: exact
trade charges and unresolved disposal-basis propagation. They do not claim a
new exhaustive charge × mixed-knowledge × method cross-product test.
Cash dividends may be recorded without holdings; they do not establish
entitlement or fabricate units. A provider proposal with short-only entitlement
uses payment in lieu, and mixed long/short holdings remain informational.

## Independent register and consumer evidence

Every row below passed; severity is **none** unless a limitation is called out.
Amounts are exact decimals, not floating point. The new
[`investment_operations_audit_test.go`](../../backend/internal/app/investment_operations_audit_test.go)
compares rational values to literal expectations after every command.

| Scenario and independent expected result | Consumer and actual result |
|---|---|
| Open 10 units for 100.00, then 10 for 130.00: 10/100.00 then 20/230.00 quantity/basis; long cash −100.00 then −230.00, short cash +100.00 then +230.00 | `TestInvestmentAuditCommandRegisterAndRestoredExport`, all eight side/method cases. Exact lots, cash, side and absence of realized decisions matched after each opening |
| Dispose/cover 7 for 77.00: FIFO basis/opening proceeds 70.00, LIFO and specific-new-lot 91.00, average 80.50. Remaining 13 units carry 160.00 / 139.00 / 149.50. Long gains +7.00 / −14.00 / −3.50; short results −7.00 / +14.00 / +3.50 | Same register. Close the last 13 for 120.00: zero units/basis, cumulative long loss 33.00 or short gain 33.00, regardless of method; exact residual and released method lock verified |
| Short payment 50.00 less recovery 7.50: cash −42.50, expense +50.00, recovery income −7.50; no lot/basis/gain change. After both covers total short cash is −9.50, separately from +33.00 cover gain | Same register, with payment before partial cover and restore. `TestPaymentInLieuChargesExpenseAndLeavesTheShortLot` also verifies immutable entitlement/components and a 200.00 full-cover gain unaffected by the payment |
| Average-family switching after partial depletion refuses before mutation; restore preserves that lock, then full closure removes it | Same register snapshots all durable rows around the refused switch after restore; FIFO/LIFO/specific→average and average→FIFO all refused atomically on both sides |
| Long buy gross 100.00 + commission 2.00, sale gross 120.00 − commission 2.00: net cash gain 16.00. Clearing-included basis/proceeds/result = 102.00/118.00/16.00; separately expensed = 100.00/120.00/20.00 with 4.00 separate expense | `TestExactTradeEconomicsFeeTreatmentAndGrossPrice`; `TestForeignTradeFeeKeepsCostCurrencyBasisAndPostsOwnCashLegs`, `TestDisposalClearingAttributionHandlesDatedFees`. Both gross and net facts, implied gross price, fee treatment and dated legs passed |
| Short gross 100.00 − opening fee 1.00; cover 70.00 + fee 0.50: result 28.50. A separately expensed fee must remain outside cover gain. Mixed-scale 100.00 − 70.001 = 29.999 | `TestShortCoverResultIsExactAcrossChargesAndScales`; all named charge/scale cases passed |
| Reinvest 50.00 into 2 units: new basis 50.00, income −50.00, no cash leg. Existing unknown basis does not erase the new known acquisition | `TestReinvestedDividend_OpensNewLotFromIncomeWithoutCashLeg`, `TestReinvestedDividendBesideAnUnknownBasisLotPosts`, `TestReinvestmentGainImpactRequiresAcknowledgement`; passed |
| Unknown inbound 2 units, sell 1: quantity and proceeds known, basis/gain NULL. Resolve total basis 80.00: 40.00 disposed + 40.00 held; original opening/link remain unknown. An average pool uses the entire resolved basis, never a known subset | `TestResolvingUnknownInboundAfterPartialSalePostsFullBridgeAndResolvesBoth`, `TestResolvingUnknownLotResolvesAveragePoolDisposals`, `TestSaleOfMixedKnownAndUnknownLotsLeavesGainUnresolvedInEitherOrder`, `TestAverageCostPoolWithUnknownBasisStaysUnresolvedUntilExhausted`; API `TestUnresolvedSaleReportsNullBasisGainAndTotal`; passed |
| Known zero is numeric zero, distinct from unknown. Invalid/missing source declarations refuse; equal-basis evidence corrections add provenance only and preserve exact additional metadata | `TestResolvingUnknownInboundAsKnownZeroIsJournalFree`, `TestBasisResolutionRequiresSourceDeclarationWithoutWriting`, `TestProvenanceCorrectionPreservesExactAdditionalMetadataOnServer`, `TestProvenanceCorrectionImmutableHistoryExportsAndSurvivesRestore`; passed |
| Full unknown write-off of 2 units: zero proceeds and NULL loss; resolution at 80.00 supplies the full disposed basis without reopening units. Native reversal restores quantity; replacement replays dependent sales | `TestUnknownWriteOffKeepsZeroProceedsAndResolvesAfterClosure`, `TestReverseFullWriteOffRestoresPosition`, `TestReplacePartialWriteOffReplaysDependentSale`, `TestWriteOffMixedCurrenciesEachMethodConsumesOnlySelectedCurrency`; passed |
| Dividend 100.00 gross − 15.00 withheld = 85.00 cash; replacement 120.00 − 18.00 = 102.00; reversal nets everything to zero. No lot changes | `TestReplaceDividendCorrectsAmountAndWithholding`, `TestDividend_WithholdingLegOnlyPresentWhenConfigured`; API lifecycle and instrument attribution consumers passed |
| Return 10.00 against 7.00 basis: 7.00 reduction + 3.00 separately disclosed excess, zero remaining basis, 10.00 cash. With unknown basis only receipt/entitlement remain known | `TestCapitalReturnBeyondBasisReducesToZeroAndRecordsExcess`, `TestUnknownCapitalReturnKeepsReceiptAndResolvesBeforeClosedSale`, `TestUnknownCapitalReturnExplicitKnownEntitlementAndAveragePool`; passed |
| Three units costing 30.00 split 3:2 to 4.5; cash-in-lieu 0.5 for 8.00 leaves 4 units. Allocated fraction basis 3.333333, remainder 26.666667; result 4.666667. Unknown 2 units split 5:4 to 2.5, fraction 0.5 resolved from total 80.00 takes 16.00 | `TestCashInLieuDisposesFractionWithCashOnItsPaymentDate`, `TestCashInLieuBackdatedBehindALaterSaleRevisesIt`, `TestUnknownCashInLieuSettlesSplitAndResolvesLater`; passed. Security date and cash payment date remain distinct |
| Same-day acquisition/disposal order is entry order; replacing an earlier acquisition retains its root slot. Changed gains require the exact current token; changing the reviewed checkpoint set refuses | `TestBackdatedSaleTakesSameDaySlotAfterEarlierEntries`, `TestReplaceOldManualBuyReplaysDependentSameDaySale`, `TestBuyGainImpactRejectsAcknowledgementOfAnEarlierChangeSet`, investment checkpoint-binding API tests under #304; passed |
| Over-cover, opposite-side consumption and simultaneous long/short refuse. Unexplained negative countable units remain a dated diagnostic, not a named short; ordinary currency overdrafts remain legal | `TestShortCoverRefusesOverCoverAndOtherSideLots`, `TestPositionSideConflictRefusesOverlappingSides`, `TestNamedShortExplainsNegativeHoldingOnlyUpToOwedUnits`, `TestUnexplainedNegativeUnitBesideARevisedCoverIsFlagged`, focused `TestSelfCheckFinds…`/`TestSelfCheckAccepts…`; passed |
| Removing the last pre-ex-date short would strand a payment: preview and commit refuse with that payment named and no writes. A partial pre-ex-date cover or full cover on the ex-date remains legal | `TestBackdatedShortCoverBeforeTheExDateNamesTheDependentPaymentInLieu`, `TestShortSaleReversalRechecksPaymentInLieuDependencyInsideTheWrite`; passed |
| Linking a provider suggestion to an existing payment must add no journal; wrong ex-date, instrument, holding, reversed payment and cash dividend refuse; second linking refuses | `TestProviderPaymentInLieuLinksOwnerRecordedPaymentOnce`; passed with local provider evidence. Provider refresh/scheduling/revision concurrency remains #274 |
| Backup a partly depleted position, restore to a fresh SQLite file, compare every exported CSV row, then close through freshly wired services | New eight-case register: all CSV datasets equal after restore; quantities, cash, gain and method lock match; closing on the restored database conserves the remaining basis. This verifies native restore/re-export, not bundle import |

`TestFinancialTradeSequencesPreserveJournalLotsBasisAndGains` adds deterministic
multi-step conservation checks across all four methods and mixed scales.
`TestEveryOperationKindExportsItsRowsAndPassesSelfCheck` checks named operation
families, their export files, journal/date/correction links and lot-free
classification. Their conservation assertions supplement the literal figures;
replay agreement alone is not an independent financial oracle.

## Acceptance limits and recovery handoff

| Check | Actual result, severity and disposition |
|---|---|
| Self-check after each new known-basis register command, after restore and after closure | Every check passed, including `commodity_position_sign`; conservation and effective-movement oracles also passed. **Verified** |
| Self-check on intentionally unknown basis, including the #266 reference book's unknown holding | `lot_reconciliation` reports unknown projected basis and marks the run failed; quantity, allocation and replay checks still execute. This is existing documented diagnostic behavior (`self_check.go`, `lotReconciliationCheck`), not reproduced corruption. **Accepted limit**, no new defect severity. The phrase “self-check is clean after every sequence” is not met literally by unresolved books |
| All investment CSV data preserved by native backup/restore | Eight partly depleted known long/short books passed. Existing unknown non-sale/provenance restore cases preserve original NULLs and revisions. **Verified** |
| Exported ZIP imported/reconstructed as a new investment book | No bundle importer is exposed; QIF explicitly excludes investments, and ordinary ledger CSV import cannot reconstruct investment evidence. **Not verified**; native restore is not evidence for this requirement. No new importer or migration is authorized by an audit |

Additional bounded work for **#277**, also tracked on **#260**:

1. Independently validate investment bundle sufficiency: manifest hashes,
   dimension/operation references, original/revised knowledge, elections,
   payment-in-lieu facts and independently reconstructed financial figures
   from exported data. State whether a reconstruction round trip is possible
   with the shipped format; do not substitute SQLite restore evidence.
2. In its #266 restore drill, compare every register figure and document the
   expected unknown-basis self-check finding separately from deliberate damage.
   Assess whether its “clean” acceptance needs an explicit unresolved state;
   any behavior change requires its own scoped issue.

These extend the existing recovery audit; no duplicate audit issue or
speculative financial defect is filed. #272's first two acceptance boxes can
be checked; the third stays unchecked pending the recovery disposition.
No product capability or financial rule changed in this audit.

`TestReferenceBookInvestmentAuditSelfCheckAndRegister` in
[`api/investment_operations_audit_test.go`](../../backend/internal/api/investment_operations_audit_test.go)
confirms the second row through HTTP: exactly one failed check and one finding,
the unknown lot. Every other applicable check passes; attachments are explicitly
`not_applicable`. Every hand-derived register figure still matches after the
self-check.

## Maintainability notes

- **Short result sign is applied in several consumers.** The cover preview's
  `shortCoverResult` (`backend/internal/app/investment_short.go`) adds positive
  opening proceeds to signed cover cash. Realized-gain aggregation reverses
  the negative event basis for the short side in
  `backend/internal/db/investments.go`; unrealized positions apply another
  side-aware sign there. The independent register and existing precision tests
  agree, but a sign/knowledge change requires coordination across these readers.
- **The payment dependency guard scans book-wide payments per command.**
  `backend/internal/db/investment_writer.go` calls `entitledPaymentsInLieuTx`
  before writes and `requirePaymentsInLieuStillEntitledTx` afterward. Each
  payment resolves dated holdings per cost currency in
  `investment_payment_in_lieu.go`. The refusal/interleaving evidence is sound;
  no payment-dense performance measurement was made. #280 should include this
  shape in its representative write measurements before #284 ranks changes.
- **Unknown information and corrupt lots share a failed diagnostic status.**
  `lotReconciliationCheck` adds unknown-basis findings to the same failed
  result as quantity/event/basis mismatches. `requireHealthyUnresolvedBook`
  therefore checks selected diagnostic messages rather than an overall pass.
  This is a concrete obstacle to an unambiguous “healthy but incomplete”
  acceptance assertion. Record the distinction in #277/#284; no refactor here.

## Validation

- `./scripts/go-backend.sh test -race -timeout=20m ./internal/app ./internal/api -run 'Investment|Buy|Sell|Sale|Short|Cover|Dividend|Reinvest|WriteOff|CapitalReturn|CashInLieu|PaymentInLieu|Basis|ReferenceBook|Conservation|Oracle|Fee|Charge|Financial|CommodityPosition'`
  passed: app 100.111 s; API 42.394 s.
- `./scripts/go-backend.sh test -race -timeout=20m ./internal/db -run 'Investment|Dispose|Disposal|Lot|PositionMethod|Allocation'`
  passed: 21.135 s.
- `./scripts/go-backend.sh test -race -timeout=20m ./internal/app -run '^TestInvestmentAudit'`
  passed: 6.007 s, eight cases including restored family-switch refusals.
- `./scripts/go-backend.sh test -race -timeout=20m ./internal/api -run '^TestReferenceBookInvestmentAudit'`
  passed: 4.035 s; the reference register matches before and after the check.
- `./scripts/go-backend.sh test -race -timeout=20m ./internal/app -run '^Test(SelfCheckFindsA|SelfCheckFindsAn|SelfCheckAccepts|EveryOperationKindExportsItsRowsAndPassesSelfCheck|OperationKindCapabilityMatrixCoversEveryAcceptedKind|ReplayIntentCapabilities)'`
  passed: 3.332 s.
- `pnpm --dir e2e exec playwright test investments-short.spec.ts investments-unknown-basis.spec.ts investments-unknown-non-sale.spec.ts investments-write-off-entry.spec.ts investments-write-off-correction.spec.ts investments-payment-in-lieu.spec.ts investments-capital-return.spec.ts investments-cash-in-lieu.spec.ts investments-historical-entry.spec.ts investments-reconciliation.spec.ts investments-gain-review.spec.ts`
  passed: **53 tests**, 2.6 minutes, rebuilt integrated binary. Includes phone
  journeys and six-locale write-off/unresolved-action cases. The build's
  catalog validation passed too; native-language sign-off remains #102.
- Initial sandbox attempts could not bind local provider test servers; the
  authorized reruns above passed. A separate diagnostics attempt met a
  read-only Go cache; its authorized rerun passed.
- Go formatting, focused `go vet ./internal/app ./internal/api`, relative
  documentation links and `git diff --check` passed.
- No full backend/frontend/browser gate was run; #280 owns the shared final
  gates. No C-engine race instrumentation, live-provider wire certification,
  release certification or exhaustive concurrency/performance claim is made.
