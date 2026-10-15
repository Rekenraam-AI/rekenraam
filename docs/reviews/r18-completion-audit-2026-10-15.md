# R18 completion audit — 2026-10-15

Audited the six delivered slices against [#118](https://github.com/sergeyfarin/rekenraam/issues/118),
their GitHub acceptance and delivery notes, the implementation through
`dce8b7bc`, ADR 0012, requirements, roadmap, feature ledger, reporting/export
contracts and competitor implications. The clean local checkout initially
ended at #234; fetching and fast-forwarding brought in the delivered #235
before code review. No missing slice was inferred from the stale checkout.

The six bounded slices are implemented. Live-read consistency and
return-of-capital period disclosure are independent follow-ups below. They
remain visible even when the delivery umbrella closes; R13 planning does not
resolve them.

## Delivery evidence

| Slice | Implementation | Audited behavior and named evidence |
| --- | --- | --- |
| [#230](https://github.com/sergeyfarin/rekenraam/issues/230) | `eb3b4919` | `operational_v1`, dated long/short holdings, price/knowledge/stale policy, absent values and read-only replay. `TestDatedValuation*`, `TestInvestmentGainsValuationAsOfAPI`; dated-valuation browser journey. Default-date gap fixed in this audit. |
| [#231](https://github.com/sergeyfarin/rekenraam/issues/231) | `270f4a71` | Opening/closing unrealized, period realized and gross dividends, exact per-currency change/total and null coverage. `TestGainsStatement*`, `TestInvestmentGainsStatementAPI*`; statement browser journey. |
| [#232](https://github.com/sergeyfarin/rekenraam/issues/232) | `61213354` | FIFO/LIFO/average comparisons, explicit specific-lot fallback, transfer/structural policy, conservation and rollback. `TestAlternativeGains*`, alternative-method API tests; comparison browser journey. |
| [#233](https://github.com/sergeyfarin/rekenraam/issues/233) | `95fb5428` | Both named FX methods, dated component provenance, same-currency exact no-op, missing/stale coverage, shorts and overflow. `TestGainsFX*`, reporting/invalid-option/overflow API tests; FX browser journey. |
| [#234](https://github.com/sergeyfarin/rekenraam/issues/234) | `3bbc4d1d` | Consistent snapshot capture, immutable original-currency operational documents, correction/withdrawal reproduction, actor/audit attribution, integrity checks, cursor history, bundle schema 12 and restore. `TestInvestmentGainsRun*`, `TestGainsRunSnapshot*`; save/reopen/download browser journey. |
| [#235](https://github.com/sergeyfarin/rekenraam/issues/235) | `dce8b7bc` | Owner-wide pools by security/cost currency/side, account quantities, internal-transfer neutrality, conservation, unknown knowledge, corrected history and rollback. `TestPersonPooledGains*`, `TestInvestmentGainsPersonPooledAverageAPI`; pooled browser journey. |

## Findings and disposition

**Fixed: undated gains included recorded future events.** The omitted
valuation date used current lot state while an explicit owner-local today
replayed to that date. A future purchase increased today's quantity; a future
sale or cover removed today's holding. The same profile/date could therefore
disagree with its alternative comparison and with the date used by a saved
run. The new service regression failed in all three cases before the fix.
Every gains valuation now uses the dated replay, including the default.
`TestDatedValuationDefaultExcludesFutureInvestmentEvents` verifies long buys,
sales and short covers, exact agreement with explicit today and unchanged
durable state. `TestInvestmentGainsDefaultTodayKeepsFutureSaleOutOfValuation`
checks the authenticated wire contract and alternative comparison. This does
not change the independent realized disposal-period filter or rewrite earlier
saved documents. The existing unknown-basis service/API fixtures now use actual
unknown inbounds and prices/partial sales instead of manually corrupting known
acquisitions' rebuildable state; their null/quantity/market-value assertions stay.

**P1 [#237](https://github.com/sergeyfarin/rekenraam/issues/237): live report
components can mix input states.** A deterministic service probe read EUR +20
gain (two shares, EUR 40 basis, EUR 60 market value), corrected the acquisition
to EUR 80, then called historical-basis FX with the original inputs and EUR
reporting currency. The no-op conversion returned EUR -20 because it re-read
the corrected basis. The live handler permits that interleaving. Statements
and alternative comparisons also compose separate reads. A common observation
cutoff does not freeze corrections or withdrawals. This needs a consistent
request input boundary, not a local arithmetic patch. Saved #234 runs already
have that boundary and retain their guarantee.

**P2 [#238](https://github.com/sergeyfarin/rekenraam/issues/238): capital-return
excess lacks period disclosure.** The delivered #171 summary exposes excess,
and the non-sale contract deliberately excludes it from disposal gains/income.
The period statement neither exposes the excluded amount nor its coverage;
the roadmap still delegated this decision to #118. A EUR 7 basis/unchanged
market value with a EUR 10 receipt produces EUR 7 unrealized change, while the
EUR 3 excess is outside the period formula. The follow-up owns an explicit
reporting decision, disclosure, FX dates and saved-schema implications. It
does not authorize taxable-income classification or automatic postings.

**Fixed documentation drift.** The competitor comparison now records R18
capabilities and deliberate gaps while preserving the date of its external
research. The roadmap removes resolved I-03/I-04 research from open decisions,
puts #237 before R13 implementation, links the capital-return decision to #238 and distinguishes
general report snapshots from shipped investment runs. The reports plan,
profile table, feature ledger and documentation map reflect the same boundary.
GitHub remains the delivery-status owner.

## Retained limits

- Saved schema 1 covers original-currency operational gains and statements;
  it does not save alternative/person-pooled or reporting-currency projections.
- Live reads use current effective history and current withdrawals. An
  observation cutoff is not retrospective book/withdrawal reconstruction;
  unsaved earlier figures cannot be recovered retroactively.
- Person pooling separates cost currencies and long/short sides. A jurisdiction
  note selects no tax rules; same-day/30-day matching is absent.
- Historic FX converts operational per-lot allocations, including the existing
  original-date convention for pooled transfers; it creates no historical FX
  basis pool. Structural entitlements remain sourced and impossible
  hypothetical entitlements refuse explicitly.
- Broker-basis comparison awaits provider evidence. Formal gain/revaluation
  postings remain unscheduled. R13 owns TWR/MWR equations and external-flow
  classification; gains statements do not establish returns/tax parity.

## Validation

- Before the fix, the new default-date regression failed for future buy, sale
  and short-cover cases. After the fix, those service cases and the
  authenticated API comparison pass with race instrumentation.
- The focused R18 service/API suites passed: dated valuation, period statements,
  alternative methods, pooled methods, FX and immutable runs. The deterministic
  #237 probe reproduced the +20/-20 inconsistency; it was removed from the
  shipping suite because its repair is separately tracked.
- Final `./scripts/test-backend.sh` passed formatting, vet and all packages
  under the race detector, including application tests (144.767 s), API tests
  (88.432 s) and database tests (79.712 s). The first broad run identified the
  old cache-editing unknown-basis fixtures; their real-workflow replacements
  passed focused checks and the final full run.
- `./scripts/test-frontend.sh` passed: zero Svelte errors/warnings and all 628
  unit tests. OpenAPI generation succeeded with the clarified default-date
  contract and checked generated client.
- `pnpm --dir e2e test investments-dated-valuation investments-gains-statement
  investments-alternative-gains investments-gains-fx investments-gains-runs
  investments-person-pooled-gains` passed all eight tests (six R18 journeys and
  two auth bootstrap cases), including integrated single-binary build, 390px,
  six-locale journeys and axe assertions. Market-data providers were stubbed.
- Changed-document local link targets and `git diff --check` passed.

Audit fixes/documentation are committed locally; no push or new CI run is
part of this audit. GitHub #118 and the previously unchecked closed-slice
acceptance lists are reconciled with the delivery evidence above; #237/#238
retain their independent open scope.
