# Reference book and expected-figure register — 2026-10-17

**Status:** [#266](https://github.com/sergeyfarin/rekenraam/issues/266) delivered.
This is the shared oracle for the #260 sub-audits (#270, #272/#273, #275, #277).
Every register figure is reproduced by the app except one, filed as
[#281](https://github.com/sergeyfarin/rekenraam/issues/281) (P2).
This report records the oracle and what it found. It does not complete the
ledger, investment, report or portability audits, which consume it.

## Scope and baseline

Audited baseline `3f46b4b1`, with a clean tree. The book is synthetic and
provider-free (ADR 0017). It has manual FX and prices only, no owner data, and
no provider stub is needed.

| Piece | Where |
|---|---|
| Book and register | [`fixtures/reference-book.json`](../../fixtures/reference-book.json): 62 public `/api/v1` steps and the hand-derived register. Each figure has a one-line `why` derivation |
| API consumer | `TestReferenceBookProducesItsRegister` ([`reference_book_test.go`](../../backend/internal/api/reference_book_test.go)): `seedReferenceBook` replays the steps on a fresh owner/book (EUR default, USD, system accounts), and `assertReferenceBookRegister` reads every register figure back |
| Browser seeding helper | [`e2e/playwright/support/reference-book.ts`](../../e2e/playwright/support/reference-book.ts) `seedReferenceBook(page)`: the same steps from inside the page, with a run suffix on names and instrument codes because the e2e book is shared |
| Browser consumer | `[acceptance] the reference book downloads its register cashflow` in `reports.spec.ts`: the cashflow CSV for the book's cash scope equals the register's gross and net measures |
| Restore-drill entry point | `assertReferenceBookRegister(t, handler, seeded)` only reads. #277 seeds, backs up, restores, signs in, then calls it with the restored handler and the same `seeded` ids. A restore keeps ids. The drill itself belongs to #277 |

The existing `fixtures/report-scenario.json` stays a separate, smaller subset
for the spending and cashflow reports. Both its readers still pass.

### What the book contains

- **Commodities:** EUR (book default) and USD, with manual USD→EUR rates on
  2026-01-31 (0.90) and 2026-03-31 (0.85). Two instruments are held to four
  decimal places, with manual USD prices.
- **Accounts:** a non-posting parent with EUR checking, EUR savings and USD
  checking children; a credit card (liability); a cash account that is emptied,
  closed and archived; an off-budget savings account; and four holdings (FIFO,
  average cost, unknown basis, short).
- **Ledger:** opening balances, salary, an expense and its refund, a
  three-category split, a same-currency and a cross-currency transfer each
  posted on different days (#187), a card payment, a voided duplicate, a
  recurring-generated system draft, and one finished reconciliation.
  There are also January and February budget targets.
- **Investments:** buys into a FIFO holding, a 2:1 split, a partial FIFO sale;
  buys into an average-cost holding, a partial average sale and a return of
  capital; an unknown-basis external transfer in; a cash dividend; and a short
  sale, payment in lieu and partial cover.
- **Register:** balances at seven dates, including both in-transit days of the
  posting-date transfers and the transfer-clearing balance. Also subtree
  balances, account status, the checkpoint, the draft, spending and income per
  category, gross cashflow per commodity, budget actuals, net worth at three
  month ends (per commodity and converted), positions, lots, and realized and
  unrealized gains: operational and as FIFO, LIFO and average-cost alternatives.
  The four methods give four distinct realized totals (350, 380, 255 and
  300 USD), so a method mix-up cannot pass.

## Findings and disposition

| Finding | Severity and evidence | Disposition |
|---|---|---|
| Net worth in EUR omits securities quoted in USD, despite a same-day USD price and a same-day USD→EUR rate | P2, #281. The 2026-03-31 bucket has no `converted` total, and `valuation.gaps` names both instruments `no_observation_in_window`. The register expects 8247.45 EUR. The rate table reads only stored `base → reporting` observations, and derived rates come only from the FX refresh worker for currency pairs | **Confirmed defect, open.** No wrong figure is shown: the incomplete total is omitted, never understated. The register keeps 8247.45, marked `known_defect: "#281"`. The reader requires that figure to still disagree, so the fix fails the test until the marker is removed |
| The cross-currency transfer's `commodity_trading` legs cannot carry `posted_on` | Request refused: "a posted date applies only to an account posting" | **Verified behaviour, not a defect.** The trading legs stay on the transaction date and the server bridges the dollars through `transfer_clearing`. The register asserts the 2750.00 USD in transit on 2026-01-27 and zero once it arrives |
| Realized `disposed_basis_value` is signed (negative for a long, positive for a short's opening proceeds) | The OpenAPI schema has no sign description for it. The ADR 0013 short rule and the debit-positive convention imply it: gain = proceeds + signed basis on both sides | **Accepted documentation limit.** The register states the signed representation. The economic figures were already hand-derived and are unchanged |

### Derivations rechecked before filing

The issue requires a mismatch to be rechecked against the documented contract
before anything is filed. Two first-draft derivations were wrong. Both were
corrected from the documents, not from the app's output:

- **Recurring drafts never backfill** (recurring plan D2). A template starting
  in February does not generate a February draft today. The book now runs a
  daily template once and archives it. The draft is dated the owner's today,
  and the register checks it by description and that today's checking balance
  excludes it.
- **Budget targets are per month and never roll over** (budgets plan rule 6).
  February needed its own target row.

## Acceptance evidence

| Requirement | Evidence |
|---|---|
| Every register figure has a one-line derivation; none produced by code | Each register entry carries `why`, derived from the steps and the cited contracts. The two corrections above came from the plans |
| The API consumer seeds through public endpoints and asserts every figure exactly, without floats | Values compare as reduced `big.Rat` strings, so `300`, `300.00` and `300.0000` are equal and nothing passes through a float. An absent optional figure is `null`, never zero. Mutation check: changing the EUR cashflow `transfer_in`, a LIFO realized gain and a lot's remaining basis by 0.01 each failed three named assertions |
| Browser seeding helper and restore-drill entry point exist | Listed in the scope table. The browser consumer passed against the integrated build |
| Mismatches filed, not edited away | #281, plus the `known_defect` mechanism |

## Limits

- The browser consumer checks the cashflow register only. The other figures are
  asserted through the API, and the screens that show them belong to #278.
- Net worth reads every account in the book, so the register's net-worth
  figures only hold in a book that contains nothing else (the Go reader, the
  restore drill). In the shared e2e book, use only account-scoped reads.
- The book does not cover share exchanges, spin-offs, mergers, internal
  transfers, corrections or dated replay. #273 extends it or adds its own
  scenarios.
- Unknown basis is held, not sold, so realized totals stay known. An unresolved
  realized total is #272's to exercise.

## Validation

- `./scripts/go-backend.sh test ./internal/api -run 'TestReferenceBook|TestReportScenario'`:
  passed. `go vet` and `gofmt` are clean for the package.
- `pnpm --dir e2e exec playwright test reports.spec.ts`: all 11 cases passed,
  including the new acceptance case and the existing shared-scenario case.
- `./scripts/test-e2e-smoke.sh` was run once, because the book now lives in
  every worker's shared e2e database: 305 passed, 1 failed. The failure was
  `recurring.spec.ts` › *blocked recurring occurrence…* (`blocked` 0, not 1).
  The reference-book case ran after that failure, and the spec passed on both
  reruns, alone and after `reports.spec.ts`. It is logged as an unrelated flake,
  [#282](https://github.com/sergeyfarin/rekenraam/issues/282) (P3).
- `./scripts/check-post-merge.sh HEAD`: clean. `git diff --check` is clean, and
  the relative links resolve.
