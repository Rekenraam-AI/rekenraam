# Quote providers and crypto instruments (#117)

R17 delivery contract, 2026-10-14. Execution order belongs to the [roadmap](../roadmap.md).
ADRs 0008–0013 and the [pricing management contract](pricing-management-plan.md)
govern accounting, precision, market data and background work.

## Instrument and entry contract

An instrument has `instrument_type=crypto` and a `commodities.kind=crypto`
commodity, with quantity precision 0–24; every other instrument uses a security
commodity and precision 0–12. Price precision remains 0–12 in instrument metadata;
observations retain the existing exact int64/scale-18 pricing contract. Linking
an existing commodity must match kind and maximum precision. Editing cannot
change the commodity or cross the crypto/security boundary. First instrument and
commodity versions begin at genesis: creation is bookkeeping, whereas a holding
account's owner-chosen opening date is a real constraint on trade history.

The Instruments tab creates/edits instruments, declares provider identities,
creates holding accounts, enables/disables pair assignments and queues quotes.
Buy/sell, external/internal transfers, correction, reconciliation and disposal
methods reuse the existing investment writers and immutable lots. Crypto is not
a parallel ledger. Self-check includes crypto postings in journal/lot comparison.
No exchange API, DeFi, staking-reward or NFT integration is admitted.

## Provider identity and valuation

Provider identifiers live in the versioned instrument `identifiers` object:
`coingecko_id` is a unique coin ID (e.g. `bitcoin`), `yahoo_symbol` includes any
exchange suffix (e.g. `ASML.AS`), and `twelve_data_symbol` is the exact provider
symbol (e.g. `ASML`), disambiguated with the instrument MIC (e.g. `XAMS`). The optional `quote_identifiers` API object merges these named fields on the
server; empty strings clear them. PATCH preserves omitted identifiers/metadata
as exact JSON, so editing a provider identity cannot round unrelated numeric
metadata through browser JSON parsing. Explicit generic objects still replace
their corresponding object. An active dated instrument/currency/source
assignment explicitly opts into fetching.
An FX default never guesses an equity or crypto mapping. Multiple assigned
providers can supply facts; valuation priority remains the shared `live_raw_v1`
policy from [the investment reporting plan](investment-reporting-plan.md).

The single market-data registry resolves both FXRateProvider and PriceProvider
capabilities. Adapters return normalized facts without database access. Quote
numbers stay decimal strings, including scientific notation, through exact
integer conversion. Unsupported precision/range, absent prices/timestamps,
wrong symbol/market/currency and future observations are refused. No guessed
GBp/GBP conversion, split/dividend adjustment, or synthesized valuation date.
CoinGecko preserves `last_updated_at` as observed time and uses its UTC date;
its aggregate spot quote uses the existing `spot_mid` category. Daily equity bars
use exchange-local dates and only completed sessions; their bar-open timestamp
is source metadata, not a fabricated observation/publication time. Twelve Data
requests `adjust=none` and conservatively excludes the current exchange date
until the next date, avoiding an in-progress daily bar. Yahoo reads close rather than adjusted-close, refuses a
sample crossed by a returned later split, and retains the unofficial marker.
No provider events create investment operations.

Storage appends immutable observations, source metadata, audit and a stable
content identity in one transaction. Repeated identical quotes coalesce;
same-provider-observation corrections append a superseding observation. A voided
identical quote is not resurrected by another refresh. Refresh changes valuation
reads, never journals, lots or reconciliation checkpoints. Manual prices remain
available. Holdings retain labelled old quotes; diagnostic coverage and reports
keep their existing, explicitly different age rules. This slice fetches current
spot/latest completed close only; historical quote backfill and return analytics
are outside this contract.

## Refresh and recovery

`POST /api/v1/pricing/quotes/refresh` accepts `{}` and returns 202 with
`queued_count`, including coalesced already-pending assignments. The existing
manual pricing refresh and owner-local daily pricing schedule enqueue the same
`pricing.quote_refresh` items. Their existing FX response continues to describe
the FX run; it does not assert quote completion.

Each item pins an assignment ID and reason. Execution rechecks current dated
assignments and source/instrument status; removed assignments complete without
fetching. Provider downloads occur outside write transactions. The pricing worker lifecycle runs quote and FX loops independently so slow
historical FX downloads cannot delay manual quotes. It leases and reclaims work; it processes up to four quote items per
wake/tick, with a 15-minute lease and 15-second default request timeout. Each
attempt uses existing ingest runs/items and per-pair source health. A crash can
repeat a request, but cannot duplicate its stored observation. Interrupted run
history can remain `running`; work recovery still proceeds from its lease and
stored observations, as in the existing pricing worker.

Credential/configuration/unsupported-pair and HTTP 4xx failures except 408/429
are terminal. Connection/5xx/429 errors retry with the existing backoff, bounded
to eight attempts; 429 honors Retry-After (seconds or HTTP date, up to 24 hours).
Provider errors never expose URLs, keys or response bodies. Requests refuse
redirects, so operator credentials cannot be forwarded to another host.
Failed quote items appear beside failed FX work in the existing retry surface;
manual requeue resets attempts. The instrument screen polls source health and
links to shared run/recovery settings. Each quote assignment has an independent
item, so one unavailable provider does not block other pairs.

## Crypto CSV interchange

Import mode `crypto_csv` is explicitly selected, with one existing active crypto
holding and one active currency asset account. No profile or exchange-specific
schema is inferred. UTF-8, comma-separated, up to 10,000 trades:

```csv
date,side,quantity,amount,external_ref,memo
2026-01-15,BUY,0.123456789012345678901234,100.00,opening-1,Initial purchase
2026-02-15,SELL,0.023456789012345678901234,25.00,sale-1,Partial sale
```

Dates are ISO calendar dates; sides are only BUY/SELL. Quantity and cash amount
are positive unsigned dot decimals, limited respectively to scale 24 and 12 and
their existing coefficient/storage ranges. Amount is net settlement: no gross or
fee breakdown is inferred. Use distinct external references per holding/cash
pair. Reusing a reference remains duplicate even if source content changed;
correct the original through native investment correction, then use a new
reference for a new trade. No automatic instrument/account creation happens.

Parsing and preview create no journals/lots. The UI shows side/quantity and the
fixed selected settlement facts, with ordinary exclusion/dedupe review. Rows
commit in stable date order; same-day file order is preserved. Each row uses the
real buy/sell writer with import identity, staged-row completion and effects in
the same transaction. Invalid investment rows never fall back to cash-only
posting. A failed row has no financial effects, but a batch may partially commit.
Preview runs against existing posted history, not a hypothetical sequential batch;
when earlier committed rows change gains, subsequent acquisitions remain pending
for a fresh exact acknowledgement through the existing gain-review surface.
Reconciled changes require the existing override or are skipped explicitly.

## Provider review (primary sources checked 2026-10-14)

These are operator-supplied adapters, not a coverage or licensing promise. No
live operator credentials were available to verify a particular subscription.
The shipping adapters are fixture-tested against documented schemas; Yahoo's
chart endpoint is undocumented and remains explicitly unofficial.

| Provider | Verified evidence and delivery boundary |
|---|---|
| CoinGecko | [Demo simple price](https://docs.coingecko.com/demo/reference/simple-price) documents the Demo key header, explicit coin IDs, quote currencies and freshness timestamp. [Rate limits](https://docs.coingecko.com/docs/errors-and-rate-limits) currently lists 100 calls/minute for Demo, with errors also consuming quota; quota can change. This adapter uses `COINGECKO_DEMO_API_KEY` and the Demo host only. Crypto/EUR is a supported denomination, not EU equity-exchange coverage. [API terms](https://www.coingecko.com/en/api_terms) govern use and redistribution; [attribution guidance](https://support.coingecko.com/articles/1043089972-what-are-the-differences-between-commercial-and-custom-licenses) requires a linked attribution. Investment/pricing screens show “Data provided by CoinGecko”. No keyless quota or redistribution entitlement is assumed. |
| Twelve Data | [Individual pricing](https://twelvedata.com/pricing) lists Basic at 8 credits/minute and 800/day, with internal non-display use; wider global EOD/display and real-time EU access require appropriate paid entitlements. Free access is not a promise of ordinary EU holdings coverage. [API reference](https://twelvedata.com/docs/ai/ai-data-assistant) describes time-series currency/MIC and adjustment control; [timezones](https://support.twelvedata.com/en/articles/5745849-timezones) distinguishes exchange-local daily dates. `TWELVE_DATA_API_KEY` is operator configuration. [Terms](https://twelvedata.com/terms) and exchange entitlements remain the operator's responsibility; a key alone does not establish display/data rights. |
| Yahoo Finance | [Yahoo's exchange list](https://help.yahoo.com/kb/finance/SLN2310.html) covers Amsterdam/Paris/XETRA and suffixes/delays, and prohibits redistribution. [Yahoo API terms](https://legal.yahoo.com/us/en/yahoo/terms/product-atos/apitnc/index.html) restrict use and circumvention. The keyless chart interface has no published supported API contract, fixed quota or availability guarantee. The owner-approved unofficial adapter is opt-in through an explicit assignment; it never scrapes pages, acquires cookies/crumbs or bypasses 401/403. An operator must have applicable use rights. EU website coverage is not a guaranteed automated-data entitlement. |

Provider keys stay in operator configuration, never instrument metadata or SQLite.
Any hosted, commercial, shared or redistributed use requires a separate rights
assessment; this contract is the existing single-owner self-hosted product.

## Delivery disposition

This contract retains the original R17 fence: priced manual/CSV holdings with
lots, one provider registry and explicit staleness/recovery. Current quotes are
read-side market facts; no returns, tax compliance, historical quote backfill,
exchange connectivity or provider-event automation is implied. R18 owns the
remaining reproducible reporting work.
