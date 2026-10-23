# Payment-In-Lieu Attribution Diagnostics — 2026-10-23

> Status update: #388 completes optional attribution, audited repair and retirement of write-time entitlement gates. The evidence below records the earlier delivery; see [repair delivery and new measurements](payment-attribution-repair-2026-10-23.md).

This is #389's diagnostic-first implementation under
[ADR 0019](../adrs/0019-financial-effects-and-repairable-attribution.md) and the
[four-case contract](../plans/financial-dependency-simplification-plan.md#diagnostic-contract-for-389).
#388 still owns nullable entry/clearing, audited attribution repair and removal
of the local/global write blockers. #390/#391 and other dependency families
remain open.

The change replaces opening-existence eligibility in the foundation check
with an advisory `investment_attribution` check. One effective dated movement
stream, read through `EffectiveLotMovements`, serves all payments in a sweep;
cost currencies stay separate. It performs no savepoint replay and runs on a
query-only connection. Investment commands do not call this diagnostic.
Missing facts, malformed dates and wrong object kinds remain foundation
failures; an execution error never becomes unsupported or unavailable input.

Warnings have their own counts and typed transaction/reason samples. Migration
`0022` preserves historical runs and refuses a lossy downgrade. The completed
precedence is failed > warning > incomplete > passed, with warning and missing
coverage counts visible alongside failures. Data settings links to transaction
detail, which evaluates the same rule on demand. The saved check is historical.
There is no new queue, worker, acknowledgement or repair command in this slice.

## Retained Controls

| History at March 31 close, April 1 ex-date | Independent expected result |
|---|---|
| Short 10, cover 4 before close | 6 owed; supported |
| Short 10, cover all on March 31 | 0 owed; warning |
| Short 10, cover all on April 1 | 10 owed at previous close; supported |
| Opening reversed or moved to April 2 | 0 owed; warning |
| March cover reversed | 10 owed; supported |
| April cover replaced onto March 31 | 0 owed; warning |
| EUR short closed, USD short 5 still open | USD 5 supports attribution independently |
| Context missing versus complete without short | Unavailable versus known contradiction |
| Known ex-date after payment | Chronology warning, including alongside unavailable context |
| Facts missing, malformed date or currency used as instrument | Integrity failure, never unavailable attribution |
| Closed read transaction | Execution error, never an inferred zero |

Real correction cases compare hand-derived quantities with a separate dated
replay oracle. The full-cover fixture uses the ordinary balanced journal and
foundation writer, bypassing only payment-local entitlement admission until
#388 retires it. It records EUR 1,000 opening proceeds − 700 cover − 50 payment
= 250 cash and 50 expense, with ledger/replay checks passing and the payment
named by a warning. Reversal retires its current warning; the saved run retains
the earlier verdict. Financial facts and source history are not rewritten.

Browser controls use explicit response fixtures for warning/missing-context
histories until #388 admits them through the UI. They cover translated mixed
severities, named links, detail visibility, 390 px layouts and accessibility;
the existing real payment entry/correction/reversal journeys remain included.

## Validation And Measurement

Toolchain: Go 1.27.3 with native SQLite. Resource measurements exclude race
instrumentation and fixture construction. Three read samples per density,
using the unchanged #379 fixture builder, all passed their independently
constructed support/supersession controls.

| Shape | Payments | Read wall range (ms) | Maximum Go allocation (bytes) | Fixture build (s) |
|---|---:|---:|---:|---:|
| Unrelated | 0 | 0.272–0.456 | 13,384 | 0.485 |
| Unrelated | 10 | 2.086–2.442 | 48,936 | 0.887 |
| Unrelated | 100 | 4.390–4.821 | 395,688 | 44.968 |
| Concentrated | 0 | 0.278–0.312 | 2,288 | 0.179 |
| Concentrated | 10 | 1.942–2.794 | 45,472 | 0.756 |
| Concentrated | 100 | 3.873–4.172 | 283,952 | 122.346 |

Maximum measured additional Go heap for a density read was 211,072 bytes.
The 250,000-posting large-book gate passed: export 12.039 s / 433 MiB
allocated / 20 MiB additional peak heap; self-check 11.785 s / 137 MiB /
39 MiB. These characterize this diagnostic and the existing whole-book reads;
they do not claim to reduce the write-time entitlement replay that #388 will
remove. Densities 300/1,000 were not measured in this bounded read-side slice.

Validation commands:

```sh
./scripts/test-backend.sh
./scripts/test-frontend.sh
pnpm build
E2E_SKIP_BUILD=1 E2E_WORKERS=2 pnpm --dir e2e exec playwright test investments-payment-in-lieu.spec.ts data-settings.spec.ts
./scripts/measure-large-book.sh
REKENRAAM_PIL_DENSITY=1 REKENRAAM_PIL_DENSITIES=0,10,100 \
  ./scripts/go-backend.sh test -count=1 -timeout=15m -v \
  -run '^TestPaymentInLieuAttributionCostByDensity$' ./internal/app
./scripts/check-post-merge.sh HEAD
```

All commands passed. The complete backend gate includes formatting, vet and
race tests. Frontend checks reported zero Svelte errors/warnings and 727 unit
tests; the integrated build and all 17 browser journeys passed. Focused race
controls also passed, including migration preservation/downgrade and actual
query-only reads. The structural audit and 194 local documentation links
passed. Validation is local; no push, pull request or remote CI run is claimed.
