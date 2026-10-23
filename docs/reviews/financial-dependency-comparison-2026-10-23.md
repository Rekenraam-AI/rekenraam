# Financial Dependencies: Four-Case Comparison — 2026-10-23

First bounded design slice of [#391](https://github.com/sergeyfarin/rekenraam/issues/391).
The [four-case contract](../plans/financial-dependency-simplification-plan.md#four-case-contract-2026-10-23)
is specified; the remaining family inventory and all runtime changes stay open.
This is a worked example for #390, not its whole-application audit.

Source: `b7cd389a0855645287a3c54b5b1101c1d4b75269` (runtime unchanged from
`8549045a`). Five provider-free scenarios run under race mode against that
source and an isolated copy with only two relationship fences disabled.
[Retained output](financial-dependency-comparison-2026-10-23.txt),
[probe](fixtures/financial-dependency-comparison-2026-10-23_test.go.txt) and
[runner](fixtures/financial-dependency-comparison-2026-10-23.py) reproduce the
comparison without changing the working tree or owner data.

## Source And Relationship Inventory

| Case | Current consumers and restriction | Concrete accounting reason / disposition |
|---|---|---|
| Dividend | `dividendPlan`, `prepareDividendWrite`, native dividend replacement, facts migration 9, income coverage and export. Payer optional; replacement sets/changes it but omission/null preserves the previous payer. | Cash, gross/tax components and payment shape are authoritative; payer changes require no lot history. Retain exact effects and coverage; support explicit clearing (#393). |
| Withholding | Same cash operation, with `withholding_only` in `investment_dividend_facts`. Both `validateDividendInput` and `recordInvestmentFoundationTx` require a payer for standalone tax. | Tax amount/payment shape need preservation; mandatory instrument is a representation coupling. Make payer optional while retaining the shape independently (#393). |
| Payment in lieu | `validatePaymentInLieuInput`, holding/date role validation, migration 13, `CreatePaymentInLieu`/replacement guards, shared writer before/after scans, effective correction terms, provider event completion/linking, foundation row and bundle facts. Entitlement and attribution fields are fixed. | Cash/expense/recovery has no lot effect. Remove historical-short permission, permit partial/repairable attribution, preserve source/financial validation and expose accurate diagnostics (#388/#389). |
| Fractional cash in lieu | Ordinary disposal/replay plus `requireNoCashInLieuSettlesTx`, settled-operation readers, insert trigger, split activity, correction context, dated lot selector, foundation row and export. | Actual quantity/basis/elections remain strong inputs. Parent effectiveness is an excessive blanket fence for a still-valid disposal; retain historical provenance and validate the real downstream disposal (#392). |

Key source owners: [cash planning](../../backend/internal/app/investments.go),
[dividend correction](../../backend/internal/app/investment_dividend_correction.go),
[payment correction](../../backend/internal/app/investment_payment_in_lieu.go),
[foundation facts](../../backend/internal/db/investment_foundation.go),
[payment admission](../../backend/internal/db/investment_payment_in_lieu.go),
[cash-in-lieu link](../../backend/internal/db/investment_cash_in_lieu.go),
[self-check](../../backend/internal/app/self_check.go).

The parent fence has four correction callers: split, share exchange, spin-off
and cash-and-stock merger. The latter three are source-reviewed here, not
dynamically established safe by the split experiment. All four need their
own valid/invalid reversal and replacement controls before #392 closes.

## Independent Scenarios And Observed Results

Amounts below are EUR. Expected figures are hand-derived and asserted using
integer coefficients/scales; they are not copied from replay output.

| Scenario | Baseline | Only relationship fences disabled |
|---|---|---|
| Gross dividend 100, embedded tax 15, separate tax 2; no lots. Add payer attribution after reconciling cash. | Cash 83, income -100, tax 17, zero lots. Payer repair preserves amounts and active cash checkpoint. Separate tax without payer refuses; adding payer permits it without holdings. | Same result; neither disabled fence affects these commands. |
| Short 10 for 1,000; pay 50. Correct opening from March 2 to April 1, the ex-date. | Payment dependency refuses; durable snapshot unchanged. | Cash 950, expense 50, open short 10 with proceeds 1,000. Payment remains effective. Selected financial checks pass; foundation fails solely for unsupported attribution. |
| Same short/payment; cover all 10 for 700 before the ex-date. | Payment dependency refuses; durable snapshot unchanged. | Cash 250, expense 50, zero open shorts. Old foundation passes: reproduces #389's missed fully-covered case through a real cover in the isolated copy. |
| Buy 3 for 36, split 3:2; dispose 0.5 for 8. Replace split with 5:2. | Blanket parent fence refuses; durable snapshot unchanged. | Quantity 7; disposed basis 2.40; remaining basis 33.60; cash -28. Replay/gain review succeeds; foundation fails solely because the historical parent is ineffective. |
| Same fraction; replace split with 1:10, leaving 0.3 before a 0.5 disposal. | Refused; durable snapshot unchanged. | Still refused by actual downstream replay; durable snapshot unchanged. This is the necessary protection. |

The basis example follows `36 / 4.5 * 0.5 = 4` before correction and
`36 / 7.5 * 0.5 = 2.40` after it; the remainder is `36 - 2.40 = 33.60`.
The isolated change bypasses only `requirePaymentsInLieuStillEntitledTx` and
`requireNoCashInLieuSettlesTx`. Entry guards, facts triggers, replay,
reconciliation, gain acknowledgement, source handling and immutability remain.

The probe checks entry/transaction/book balance, lot reconciliation, replay
equivalence, checkpoint integrity and SQLite integrity after accepted changes.
Where expected, it requires exactly one foundation finding with the named
attribution message. It is not enough to make an aggregate balance agree.

## Design Consequences And Limits

The contract chooses nullable cash attribution with explicit omit/preserve,
null/clear and value/replace semantics; immutable typed facts and the existing
audited replacement lifecycle remain. One `warning` severity in the existing
self-check distinguishes known contradictions from unavailable coverage and
financial damage. A transaction repair link and on-demand detail diagnostic
provide visibility without a new inbox, acknowledgement or review engine.
Those API/storage/UI changes belong to #388/#389; none are implemented here.

#392 owns all-parent cash-in-lieu adaptation. #393 owns optional/clearable payer
attribution and preserving withholding-only shape. Provider identity/dedupe,
all parent kinds, a new nullable schema, UI accessibility, dense-book diagnostic
cost and full end-to-end repair are implementation acceptance, not proved by
this experiment. No latency improvement or full-suite result is claimed.

The first draft probe read a superseded short lot after replacement; it was
corrected to join effective operations before accepting the final evidence.
The initial sandbox run could not write the Go cache; the retained successful
run used the existing cache with permission. Neither was a production defect.

## Reproduction

```sh
python3 docs/reviews/fixtures/financial-dependency-comparison-2026-10-23.py
```

The runner archives the pinned source plus its test fixtures into a temporary
directory, compiles the retained probe, runs five cases with `-race`, then
disables the two named fences in that copy and reruns the same five cases.
It records source/probe identity and Go version, and deletes the temporary
copy. Native SQLite prerequisites are those of `scripts/go-backend.sh`.
