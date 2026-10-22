# Rekenraam

Rekenraam is a self-hosted personal finance app for tracking accounts,
transactions, budgets and investments across currencies. It runs on your own
computer or server and is accessed through a web browser.

It is a **pre-release, single-user project** maintained by one person, with
substantial use of AI to generate code and documentation. Features and data
structures are still changing. Read the limitations below before deciding
whether to use it for your records.

*Rekenraam* is Dutch for abacus.

![Spending report in Rekenraam, showing ten months of categorized expenses using sample data](docs/images/reports-spending.png)

## Reasons to use it

Rekenraam may be useful if you want to manage everyday finances and investments
together and are comfortable running the software yourself.

- The database and backups stay on storage you control. Core use requires no
  external service account or subscription.
- Accounts can hold different currencies. Reports keep original currencies
  separate or convert them using exchange rates you can inspect and edit.
- A double-entry ledger stores monetary values as exact decimals. Versioned
  transactions and statement reconciliation help you trace changes and compare
  the ledger with bank records.
- Investment records include lots, cost basis, dividends, splits, transfers and
  gains. You can save period statements to retain the figures from that run.
- CSV, QIF and a structured archive export make records available for analysis
  or migration. Backups provide a separate way to restore the installation.

## What it does today

- Accounts, categories and payees, grouped by institution
- Transaction entry with splits and transfers, on desktop and phone
- CSV and QIF import with saved column profiles and duplicate detection
- Optional Trading 212 connection for cash, trades and dividends
- Statement reconciliation
- Net worth, spending and cashflow reports with drill-down, charts and CSV
- Monthly budgets per category and currency
- Recurring transactions you review before they post
- Projected balances from recurring items and your past spending
- Investment holdings, prices and exchange rates, with optional quote providers
- Nightly verified backups, command-line restore, and exports
- Two-factor sign-in and read-only API tokens for your own scripts
- English, plus draft Dutch, German, French, Spanish and Russian translations

The [feature ledger](docs/implemented.md) records what ships and the remaining
gaps in each area.

## Limitations and tradeoffs

- **You operate it yourself.** Installation, updates, HTTPS for remote access,
  backups and recovery are your responsibility. The database is not encrypted
  at rest by the app; use storage encryption if you need it. Keep a copy of
  backups off the machine running the app.
- **Pre-release compatibility is limited.** Development schema changes may
  require resetting a disposable database. There is no supported upgrade path
  from every development revision; see the [upgrade policy](docs/upgrades.md).
- **One owner per installation.** There are no shared household accounts or
  multi-user permissions, and no native desktop or mobile app.
- **Import and connection coverage is limited.** File imports support CSV and
  QIF, not OFX or XLSX. Trading 212 is the current online import connection;
  general bank sync is not available. Some provider events require manual
  entry or linking. Optional providers receive requests when enabled and may
  require API keys or paid plans.
- **Investment reports have a defined scope.** Gains depend on the recorded
  basis, prices and exchange rates; missing inputs can leave results
  unavailable. Tax filing, jurisdiction-specific tax rules and time-weighted
  or money-weighted return analytics are not implemented.
- **Translations need review.** English is the initial implementation language.
  Dutch, German, French, Spanish and Russian catalogs are drafts awaiting
  native terminology review.

## AI use and review

AI-generated code and documentation are part of the development process.
Changes go through repeated code reviews, regression tests and recorded audits
of areas such as ledger calculations, reconciliation, authentication and
backup/restore. The [review records](docs/reviews/) describe their scope,
findings and validation; the [roadmap](docs/roadmap.md) retains release gates
and follow-up work.

As of 22 October 2026, the repository has more than 1,000 commits. That is a
record of development activity, not a count of independent reviews or a
guarantee of correctness. Bugs have been found through these reviews and may
remain despite them.

## Try it

The Docker Compose example builds the app from source. Generate a random setup
token of at least 32 characters; you'll enter it in the browser to create the
owner account.

```sh
export SETUP_TOKEN="$(openssl rand -hex 32)"
docker compose -f deploy/docker/compose.yaml up --build
```

Then open <http://127.0.0.1:16888>. The example only listens on localhost.

Before storing financial data:

- Set `REKENRAAM_SECRET_KEY` to a durable secret before connecting online
  providers or turning on two-factor sign-in, and keep it somewhere other than
  your backups.
- For access from other devices, put it behind HTTPS. The
  [deployment security guide](docs/deployment-security.md) walks through it.
  Enrol in two-factor sign-in before exposing it to the internet.
- Read the [upgrade and restore guide](docs/upgrades.md) before upgrading an
  installation you care about.

Quote providers are optional and require your own API key: set
`COINGECKO_DEMO_API_KEY` or `TWELVE_DATA_API_KEY`, then assign them to
instruments under Investments → Instruments. Details are in the
[quote provider notes](docs/plans/quote-providers-crypto-plan.md).

## Backups and exports

Use a backup to restore Rekenraam. Exports are for use outside the app.

- **Backups** (Settings → Data → Backups) contain the database, including the
  ledger, investments, budgets, recurring items, reconciliations and settings.
  Rekenraam makes one every night and on request, and verifies each before
  keeping it. The separate `REKENRAAM_SECRET_KEY` is not included; retain it
  to recover encrypted connection and two-factor secrets. Stop the app before
  restoring with `rekenraam restore --from <backup>`, following the
  [restore guide](docs/upgrades.md).
- **Exports** (Settings → Data → Export) are ledger CSV, QIF, or a full archive
  for spreadsheets and other finance tools. They are for reading elsewhere and
  **cannot be imported back** into Rekenraam as a restore.

## Development

The backend is Go with SQLite, the frontend is SvelteKit, and production is a
single Go binary that serves the built frontend.

You'll need Go 1.27.3+ (with CGO and a C compiler, e.g.
`sudo apt install build-essential`), Node 24 and the pnpm version pinned in
`package.json`.

```sh
pnpm install
pnpm dev          # http://localhost:1888, API proxied to 127.0.0.1:16888
```

Checks and build:

```sh
./scripts/test-backend.sh      # gofmt, vet, race tests
./scripts/test-frontend.sh     # catalog parity, API types, Svelte checks, unit tests
./scripts/test-e2e-smoke.sh    # provider stub tests + browser smoke suite
pnpm build                     # writes dist/rekenraam
```

The full browser suite is `./scripts/test-e2e.sh`. Focused Go commands, Go cache
settings, E2E timing, the API contract audit, resetting a disposable dev
database and local password recovery are documented in the
[developer workflow](docs/developer-workflow.md).

| Path | What's there |
|---|---|
| `backend/` | Go server, services, SQLite migrations, tests |
| `frontend/` | SvelteKit app, compiled for embedding |
| `api/` | OpenAPI contract and Bruno requests |
| `e2e/` | Playwright browser journeys |
| `deploy/` | Docker build and Compose example |
| `scripts/` | Dev, validation and build commands |
| `docs/` | Requirements, decisions, plans and status |

Start with the [documentation map](docs/README.md). The
[roadmap](docs/roadmap.md) sets the work order, open work is tracked in
[GitHub Issues](https://github.com/sergeyfarin/rekenraam/issues), and product
and architecture decisions live in the
[requirements](docs/product-requirements.md),
[conventions](docs/conventions.md) and [ADRs](docs/adrs/). Security reports go
through [SECURITY.md](SECURITY.md).

## License

[AGPL-3.0](LICENSE).
