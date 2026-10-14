# Developer Workflow

This document defines default development workflow conventions for contributors and coding agents.

## Source Of Truth

Before changing durable behavior, read:

- `docs/product-requirements.md`
- `docs/conventions.md`
- `docs/early-architecture-decisions.md`
- `docs/adrs/`
- `docs/localization-glossary.md` — before adding or translating a domain term.

## Daily Commands

Run from repo root unless noted otherwise.

### Install

```sh
pnpm install
```

### Development

```sh
pnpm dev
pnpm dev:backend
pnpm dev:frontend
```

The frontend development server binds to `127.0.0.1:1888`. It does not expose a
plain-HTTP LAN URL because secure authentication cookies only work over HTTPS or
on a browser's trusted localhost origin. For a headless development machine,
run this on the machine with the browser and keep the connection open:

```sh
ssh -N -L 1888:127.0.0.1:1888 user@development-host
```

Then open `http://localhost:1888`. Vite proxies `/api` to the backend on the
development host, so the frontend port is the only required forward. Use an
HTTPS reverse proxy for direct access from phones or other LAN clients.

### Backend Validation

The minimum Go toolchain is 1.27.2, declared in `backend/go.mod`. This patch
fixes the standard-library vulnerabilities reported by the 2026-10-09
`govulncheck` run. GitHub Actions reads that same minimum with
`go-version-file`; local Go commands automatically select it when needed.
The Docker builder pins `golang:1.27.2-trixie`; the runtime remains Debian
trixie. Native SQLite requires CGO and a C compiler with development headers
(Debian/Ubuntu: `sudo apt install build-essential`; macOS: Xcode command-line
tools). Windows needs a target C toolchain; no Windows release artifact is
claimed by this Linux/amd64 validation.

Supported commands source `scripts/native-sqlite-env.sh`, which checks CGO
and the compiler and enables `fts5,netgo,osusergo`. FTS5 is required by the
schema; the other tags preserve the previous pure-Go network/user lookup
behavior. Do not use the `libsqlite3` tag. `CGO_ENABLED=0` fails early with
setup guidance. For focused Go commands from repo root, use
`./scripts/go-backend.sh test ./internal/db -run TestNative -count=1` rather
than an untagged raw command. Advanced commands may source the same helper
after entering `backend/`; its required tags supersede a caller
`GOFLAGS=-tags=...`.

The Unix subprocess guard regressions (#105) run in the ordinary backend gate.
For a focused repeat:

```sh
./scripts/go-backend.sh test -race ./cmd/rekenraam -run '^TestTwoProcess' -count=3 -timeout=3m
```

They re-execute the same test binary with independent SQLite pools and pipe
barriers, retaining the selected native driver and Go race instrumentation.
Local TCP sockets are required for the real server/restore cases. #227 adds
paused-restore lifetime checks and repeated lock-file handoffs; their additional
child processes have a three-minute bound for this focused three-repeat command.
The full backend gate retains its five-minute per-package deadline. These tests
do not enable multiple servers on one database; see the
[#105 review](reviews/two-process-sqlite-guards-2026-10-14.md) and
[#227 review](reviews/restore-process-lock-2026-10-14.md).

Checks formatting (`gofmt -l`), runs `go vet ./...`, then the full Go suite with
the race detector. The formatting and vet gates live inside the wrapper script
so CI enforces them too — before they were added, `gofmt` drift could sit in the
tree with a green pipeline (backlog T-49).

The script resolves gofmt as `"$(go env GOROOT)/bin/gofmt"` rather than trusting
PATH. Run it that way by hand too: a version manager pinning an older Go leaves
its gofmt first on PATH even while the `go` command switches to the newer
toolchain `go.mod` requires, and gofmt's output is not stable across releases,
so a bare `gofmt` both misses drift CI will reject and flags files that are
correctly formatted for the toolchain actually compiling them.

```sh
./scripts/test-backend.sh
```

The race gate runs `go test -race -p 2 -timeout=5m ./...`: two packages run
concurrently, with an explicit five-minute deadline per package. The non-race
coverage pass uses the same package concurrency and deadline. Local validation
and CI use the same wrapper. Neither command sets a fixed GOMAXPROCS,
test `-parallel` limit or CPU affinity; use available Go CPU resources.
CI's integrated build runs independently of other checks; all jobs still
must pass. [ADR 0016](adrs/0016-native-backend-test-resources.md) and the
[native gate review](reviews/native-test-gate-2026-10-12.md) govern these limits.

[T-111 #126](https://github.com/sergeyfarin/rekenraam/issues/126) reviewed the
complete suite after the 2026-09-30 gate took 598.347 seconds against the old
600-second limit. The race profile and duplication review are recorded in
`docs/reviews/test-runtime-review-2026-09-30.md`. All financial, migration,
restore and race coverage is retained. Independent financial matrix cases
run in parallel with isolated fixtures; the owner authorized the explicit
15-minute bound to unblock further P0 work. Continue reviewing measured fixture
cost rather than treating that bound as a runtime target. The full revised
gate passed with the application package at 513.745 seconds, down from
598.347 seconds; the focused race run retained all 60 named matrix cases.
On 2026-10-11 the owner raised the bound from 15 to 20 minutes after the
application package reached 856 seconds of 900 on this workspace
([#164](https://github.com/sergeyfarin/rekenraam/issues/164)). #164 still
owns restoring headroom by removing cost, not by raising the bound again.

**Native SQLite cutover (2026-10-12):**
[ADR 0014](adrs/0014-native-sqlite-driver.md) and
[#195](https://github.com/sergeyfarin/rekenraam/issues/195) install
`github.com/mattn/go-sqlite3` v1.14.52 with bundled SQLite 3.53.4. Runtime,
tests, migrations, seed tooling and builds use the same driver/features.
The owner confirmed that no legacy databases/backups exist; current schema,
seed/checksum and backup/restore contracts remain unchanged. The
[delivery review](reviews/native-sqlite-driver-2026-10-12.md) records
compatibility, performance and build evidence. #164's measured headroom
acceptance is met: matched full race wall time improves from 27.67 to 3.76
minutes, with the same named test set and timeout under the historical
WAL/NORMAL writer policy. No race-instrumentation
exclusions are applied:
`-race` covers Go application and driver-wrapper code, while the native
SQLite C engine is outside Go's race instrumentation. A deliberate Go race
must still fail the gate.

**Commit durability follow-up (2026-10-12):**
[ADR 0015](adrs/0015-sqlite-commit-durability.md) requires WAL/FULL for the
writer and FULL consistently across migrations, tests and auxiliary opens.
The native driver's hook enforces FULL on direct and replacement connections;
startup checks its effective value. No schema/seed migration is needed.
The [FULL review](reviews/sqlite-full-durability-2026-10-12.md) records current
performance and the connection/build settings audit. The read pool's maximum
four connections is independent of CPU count. The shared gate does not cap
GOMAXPROCS or test parallelism at four; four-CPU affinity is a controlled
measurement condition. The owner's subsequent clarification requested updated
test limits: ADR 0016 replaces the old serial-package/20-minute gate with two
concurrent packages and five-minute deadlines after native measurements.

The financial property tests run in the normal backend suite. For a focused
iteration on ledger, cashflow, lot conservation and gains arithmetic:

```sh
./scripts/go-backend.sh test ./internal/app ./internal/api -run '^TestFinancial' -count=1
```

Investment tests also run an independent conservation oracle
(`requireConservationOracle`, [#190](https://github.com/sergeyfarin/rekenraam/issues/190)).
Its expected values never come from replay or from the effective-movement
owner:

- dated long and named-short quantities against the posted journal, per
  holding and date;
- per-operation basis conservation, checked against the operation's own facts;
- unknown basis staying unknown until a sourced resolution.

Every shared investment self-check pass helper calls it, so a new
investment test should end with one of them. When a later step corrects or
reverses the revised operation, call the oracle in between: the correction
retires the revision a buggy reader would mishandle. The named multi-family
sequences are `TestOracle*`:

```sh
./scripts/go-backend.sh test ./internal/app -run '^TestOracle' -count=1
```

Operation kinds have a capability matrix
([#191](https://github.com/sergeyfarin/rekenraam/issues/191),
`backend/internal/app/investment_capability_matrix_test.go`). Writers record
only the kinds in `db.InvestmentOperationKinds`. Each kind's row names, for
replay behind a later depletion, replacement, reversal, export and self-check:

- the test that exercises it,
- the test that asserts its named refusal, or
- the GitHub issue that tracks the gap.

`TestEveryOperationKindExportsItsRowsAndPassesSelfCheck` records every kind,
checks the exported bundle rows and runs the self-checks. A second table covers
the replay-intent families. To add an operation kind, add it to the list, give
it a matrix row and record it in one of the capability scenarios:

```sh
./scripts/go-backend.sh test ./internal/app -run 'CapabilityMatrix|ReplayIntentCapabilities|EveryOperationKind' -count=1
```

These include independent exact expected amounts, fixed-seed trade sequences,
gross cashflow scope checks and HTTP scale/total assertions. Keep the full
backend script as the completion gate. See
[financial test hardening](reviews/financial-test-hardening-2026-09-19.md) for
scope, mutation evidence and limitations; coverage percentage alone is not a
financial correctness gate.
The test-hardening review records T-103, the partial-disposal allocation
defect it found; that is fixed (backlog T-103) and its regression passes under
all four methods, so the whole backend script is green again. Basis is split at
the cost commodity's maximum scale, one scale per position — when a test's
expected basis or gain looks deeper than the amount that produced it, that is
the policy, not a rounding slip.

The backend suite enforces the consolidated migration checksum and exercises
fresh and seeded baseline equivalence, including durable-data preservation
when migration admission is repeated. No legacy databases exist. A checksum
failure normally means the old migration must be restored and the change moved into the next numbered
migration. ADR 0013 permits an explicitly declared pre-release redesign of
the unused v0.1 baseline, with an updated checksum and fixtures. See
`docs/upgrades.md` for the operator path.

`COVERAGE=1 ./scripts/test-backend.sh` runs the same suite without `-race`,
writes `backend/coverage.out`, and prints the merged total. CI runs this in a
non-gating `backend-coverage` job and fails it below a soft floor
(`scripts/check-coverage-floor.sh`, `COVERAGE_FLOOR` default 73.0%) — raise
the floor deliberately when the level rises; never lower it to make a change
pass.

### Local Owner Recovery

Reset the owner password locally with a verified SQLite backup first:

```sh
printf '%s\n' 'new-password' | DATABASE_URL=file:var/dev.sqlite ./scripts/go-backend.sh run ./cmd/rekenraam recover-owner --password-stdin
```

Write the verified backup to an explicit path when needed:

```sh
printf '%s\n' 'new-password' | DATABASE_URL=file:var/dev.sqlite ./scripts/go-backend.sh run ./cmd/rekenraam recover-owner --password-stdin --backup-path ../dist/recovery.sqlite
```

Only use the emergency override when backup creation or verification is impossible and you accept the risk of proceeding without a fresh verified backup:

```sh
printf '%s\n' 'new-password' | DATABASE_URL=file:var/dev.sqlite ./scripts/go-backend.sh run ./cmd/rekenraam recover-owner --password-stdin --allow-no-backup
```

### Online Provider Secret Key

`REKENRAAM_SECRET_KEY` encrypts online provider credentials **and the two-factor
shared secret** at rest; without it, both connection creation and MFA enrolment
return `CONFIG_REQUIRED` rather than storing anything in the clear. It must be
base64 for exactly 32 random bytes:

```sh
openssl rand -base64 32
```

Keep the value outside Git in the service environment or deployment secret
manager, and back it up with the SQLite database. If the key is lost, existing
online connection secrets cannot be decrypted. Restore the old key from backup,
or stop the app, take and verify a SQLite backup, start with a new key, delete
the affected online import connections, and re-add them with fresh provider
credentials. Existing imported ledger data and import history remain durable.

There is no in-place secret-key rotation command yet. Intentional rotation uses
the same backup-first delete-and-re-add procedure for stored online import
connections.

### Trading 212 Provider Stub (Development Only)

`TRADING212_BASE_URL` points the Trading 212 key probe and history fetches at
another server, such as the e2e stub in `e2e/stubs/trading212.mjs`:

```sh
node e2e/stubs/trading212.mjs
APP_ENV=development TRADING212_BASE_URL=http://127.0.0.1:16890/api/v0 pnpm dev:backend
```

It is honored **only** with `APP_ENV=development`. Startup refuses it in
production (and a hand-built runtime config refuses it too), because the API
key travels in the `Authorization` header of every provider request. The stub
answers only API keys registered through `PUT /__control/accounts/{key}` and
serves their orders, dividends and cash transactions in the provider's wire
shape; `e2e/playwright/support/trading212.ts` wraps that control call. Starting
an online import or refresh wakes the fetch worker immediately, so a stubbed
fetch is staged in seconds rather than on the worker's one-minute tick.

### Frontend Validation

Runs SvelteKit checks and the Vitest unit suite.

The dev, check and build commands first run `catalogs:check`. That gate runs
`svelte-kit sync` before Vitest so SvelteKit's generated `$app/tsconfig` exists
on a fresh checkout (#207). After `pnpm install --frozen-lockfile`, no separate
warm-up command is needed. The same bootstrap applies when running
`pnpm --dir frontend run catalogs:check` directly; catalog parity and OpenAPI
generation remain required parts of the normal frontend commands.

```sh
./scripts/test-frontend.sh
```

### Integrated Build

```sh
pnpm build
```

Builds the frontend, copies it into `backend/internal/web/dist/`, runs the
embed test, and compiles `dist/rekenraam`.

#### Reproducibility

Native SQLite builds use `CGO_ENABLED=1`, `-trimpath` and the shared
`fts5,netgo,osusergo` features. SQLite C source is bundled in the pinned module,
so the artifact does not need a separately installed `libsqlite3`. The default
Linux artifact dynamically links libc. A libc-independent static artifact is
separate work; cross-compilation is not a gate for a platform-native build.

```sh
pnpm build
sha256sum dist/rekenraam
ldd dist/rekenraam
./scripts/go-backend.sh build -trimpath -o /tmp/rekenraam-repeat ./cmd/rekenraam
sha256sum /tmp/rekenraam-repeat dist/rekenraam
```

Matching hashes require identical source, embedded frontend, Go patch version,
C compiler/linker, headers, libc target, build tags, CGO flags and module graph.
Record those inputs and base-image digests with release artifacts. `-trimpath`
removes builder paths; it does not make differing C toolchains equivalent.
The Docker build uses the Debian trixie C toolchain/runtime together. A binary
built on a newer libc may not run on an older host: build for the actual target
baseline or use the container. The delivery review records measured local and
container dependencies and same-input repeated-build hashes.

### E2E

```sh
./scripts/test-e2e.sh
E2E_BASE_URL=http://localhost:16888 ./scripts/test-e2e.sh
```

Run just the fast journeys — everything except the serial release-preflight
suite. This is what CI runs on every push:

```sh
./scripts/test-e2e-smoke.sh
```

Run the release preflight when preparing a release:

```sh
pnpm test:release-preflight
```

- **Do not read the exit code of a piped run.** `./scripts/test-e2e-smoke.sh | tail -25` reports `tail`'s status, which is always 0, so a failing suite looks green — that is shell pipeline semantics, and nothing inside the script can change it. Run it unpiped, or set `set -o pipefail` in the calling shell first, which is what the CI step that tees backend coverage does.
- `./scripts/test-e2e.sh` now builds the integrated app, starts a fresh local instance on `127.0.0.1:16889`, and uses a dedicated SQLite file at `backend/var/e2e.sqlite` unless `E2E_BASE_URL` is set.
- The Playwright suite runs with one worker because the default harness shares one app instance and SQLite database.
- Set `E2E_PORT` when the self-managed e2e port needs to move.
- Set `E2E_BASE_URL` when you want Playwright to target an already-running app instead of booting its own fresh instance.
- The harness also boots the Trading 212 provider stub on `127.0.0.1:16890` (move it with `TRADING212_STUB_PORT`) and starts the app with `TRADING212_BASE_URL` pointing at it, so import journeys can create connections and fetch fills without the real provider. `MARKET_DATA_BASE_URL` points every built-in quote/FX provider at the same stub, whose 404s keep background FX coverage from importing live rates into the shared e2e book (#226); both overrides are refused outside `APP_ENV=development`. Specs that need it call `requireTrading212Stub()`; against an external `E2E_BASE_URL` they skip unless `E2E_TRADING212_STUB_URL` names a stub that app is configured to use.
- The harness sets a throwaway `REKENRAAM_SECRET_KEY` for the instance it boots, because the MFA journey cannot enrol without one — enrolment returns `CONFIG_REQUIRED` rather than storing the shared secret in the clear. Export your own `REKENRAAM_SECRET_KEY` to override it.
- `mfa.spec.ts` shares the run's database with every other spec, so it turns MFA back off in `afterAll`. A spec that changes account-wide authentication state must clean up the same way.
- Set `PLAYWRIGHT_CHROMIUM_EXECUTABLE` to an existing Chromium binary when the sandbox or image cannot download the revision this Playwright release pins (`pnpm exec playwright install` fails, and the run dies with "Executable doesn't exist"). Container images that preinstall a browser usually expose one at `/opt/pw-browsers/chromium`. Prefer this over patching the browser cache by hand; leave it unset locally so Playwright uses its own pinned build.

## Area Notes

- Backend code lives in `backend/`; focused commands use `./scripts/go-backend.sh`. The full validation gate remains `./scripts/test-backend.sh`.
- `scripts/test-e2e-acceptance.sh` runs only the browser cases tagged
  `[acceptance]` — the ones that map onto a plan's validation matrix, so closing
  an initiative can point at a suite rather than at an argument (T-61). The
  broader suites are split by cost instead: smoke on every push, release
  preflight before a release.
- The backend binary also exposes local maintenance commands: `recover-owner`
  (backup-first password recovery and session revocation), `verify-backup`
  (integrity, schema version, row counts, and whether sealed rows decrypt under
  the configured `REKENRAAM_SECRET_KEY`), and `restore` (refuses while the
  server holds its lock, preserves the previous database as a whole file set,
  installs atomically).
- Frontend code lives in `frontend/`; SvelteKit builds static output that is copied into `backend/internal/web/dist/` for the single binary.
- Frontend foundation libraries are pinned in `frontend/package.json`: Tailwind CSS, Bits UI, shadcn-svelte, Paraglide JS, `@tanstack/svelte-query`, `@tanstack/svelte-table`, `openapi-typescript`, `openapi-fetch`, `date-fns`, and MiniSearch. Add TanStack Virtual only when a concrete screen needs it.
- Money parsing, editable prefills and exact arithmetic belong to `frontend/src/lib/money/amount.ts`; read-only display belongs to `frontend/src/lib/money/format.ts` (`formatQuantity`). These use strings/BigInt and `Intl.NumberFormat`, with no money-formatting dependency. Use `date-fns` for date manipulation and `Intl.DateTimeFormat` for final locale-aware date display. See [the formatting conventions](conventions.md#frontend-conventions) and the frontend-screen skill.
- The frontend runs **TypeScript 6.0** as a deliberate, tested exception: `openapi-typescript` 7.13 still declares `peerDependencies.typescript: ^5.x`, but schema generation, `pnpm --dir frontend run check` and the build pass on 6.0 with unchanged generated output. Re-run those three checks on every TypeScript or generator bump, and fall back to the newest 5.x if generation breaks. TypeScript 7 still breaks the schema generator; that upgrade stays tracked as T-48 [#112](https://github.com/sergeyfarin/rekenraam/issues/112), which the 6.0 upgrade does not close.
- Frontend message catalogs live in `frontend/messages/`, split by domain as `frontend/messages/<domain>/<locale>.json`. Keep message IDs flat and prefixed by domain or screen intent instead of using one growing locale file or deep nested objects. Generated Paraglide output lives in `frontend/src/lib/paraglide/`. Regenerate the typed message layer with `pnpm --dir frontend run paraglide:compile` when changing catalog structure outside the normal `dev`, `check`, or `build` scripts.
- Shipping locales are `en, es, fr, nl, de, ru`, listed in `frontend/project.inlang/settings.json`. All configured message directories compile into the single `m` namespace, so **a key must be unique across all configured domains** — which file it lives in is grouping, not namespacing.
- Add each new message to every configured locale in the same change; the parity gate rejects missing keys. Per-message English fallback remains a runtime safeguard. Adding a *term* requires review — put it in `docs/localization-glossary.md` first, because a plausible-but-wrong financial term is worse than English.
- **Locale resolution is configured in two places that must agree**: `strategy` in `frontend/vite.config.ts` and the `--strategy` flag on the `paraglide:compile` script in `frontend/package.json`. Both list `localStorage`, `preferredLanguage`, `baseLocale` — the CLI flag is variadic, so the names are space-separated there, not comma-separated. If they drift, the plugin's compile usually wins and hides the mistake — until something skips it and the app silently ships English only.
- Run `pnpm --dir frontend run catalogs:check` to compare every configured domain and locale with the Inlang base locale, including keys, placeholders, nonempty values and shared-namespace collisions. Frontend dev/check/build and the shared frontend/CI gate run it automatically. Catalog parity does not establish native terminology approval; use [the #102 review packet](reviews/localization-native-review-2026-10-13.md). `e2e/playwright/language.spec.ts` covers the switcher end to end and deliberately asserts strings from both the app and settings catalogs, since a missing file would otherwise show up only as English on half a screen.
- Multi-state Svelte routes should stay thin route entrypoints. Move reusable workflow UI and route-specific helpers into feature folders under `frontend/src/lib/`, as with `frontend/src/lib/install-gate/`.
- Frontend OpenAPI types are generated from `api/openapi/openapi.yaml` into `frontend/src/lib/api/schema.d.ts`. The root OpenAPI file is only the entrypoint: keep top-level metadata, global components, and `$ref` registrations there rather than inline path or schema bodies.
- Add new API endpoints by creating or extending a path item under `api/openapi/paths/`, then referencing that file from `api/openapi/openapi.yaml` under `paths`. Keep each path item focused on one route pattern such as `institutions.yaml` or `institution-versions.yaml`.
- Add or extend schema groups under `api/openapi/components/schemas/`, then expose individual schemas through the root `components.schemas` map. Path files should reference shared schemas through `../openapi.yaml#/components/schemas/...`; schema group files should reference siblings through `../../openapi.yaml#/components/schemas/...`.
- Regenerate frontend types with `pnpm --dir frontend run openapi:generate` when changing the contract outside the normal `dev`, `check`, or `build` scripts.
- Keep the OpenAPI client seam language-agnostic. Do not embed user-facing English fallbacks in `frontend/src/lib/api/`; translate API status and error presentation at the UI layer with Paraglide using stable error codes or screen-specific fallback copy.
- API examples and contract assets live in `api/`; use the Bruno `local` environment for the backend dev server and `app` for an integrated binary or Docker app.
- Login-created sessions default to 30 days. Set `SESSION_LIFETIME_HOURS` to a positive integer number of hours when testing shorter or longer session expiry behavior.
- Production generates a one-time setup token when `SETUP_TOKEN` is absent; operators should set a durable random token of at least 32 characters and enter it only for the first owner-creation request. See `docs/deployment-security.md`.
- Forwarded proxy headers are ignored by default. Set `TRUST_PROXY_HEADERS=1` only when the app is behind a trusted reverse proxy that rewrites those headers, and set `TRUSTED_PROXY_CIDRS` to the proxy source ranges that are allowed to supply them.
- Browser e2e tests live in `e2e/` and use Playwright. Keep them focused on user journeys that need a browser.
- `e2e/playwright/release-preflight.spec.ts` is the local release preflight for critical financial workflows. It is intentionally serial against one fresh SQLite database and is not part of fast CI yet. Because it is `describe.serial`, a failure in the first test skips the rest of the file — check "did not run" counts before concluding a journey passed.
- **Never hard-code a calendar date for a transaction, statement, or imported row in an e2e spec.** Everything a spec seeds is versioned as effective from *today*, and posting validation resolves account and commodity rules as of the entry date, so a date in the past relative to that seeded data matches no version and the posting is rejected. Use `todayISO()` / `todayQIF()` from `e2e/playwright/support/dates.ts`. A hard-coded `2026-07-08` silently broke every browser transaction-entry journey once the wall clock passed it (fixed 2026-08-06).
- **Assert a saved transaction with `expectSavedTransaction()`, not a bare `getByText(description)`.** Saving opens a detail panel, so the description can be on screen three times at once and a loose text locator fails Playwright strict mode — a failure that reads as "not saved" when the opposite is true. Note also that the list row shows the *payee* instead of the description when there is one, so asserting against the row alone is not portable either.
- Docker assets live in `deploy/docker/` and must preserve the same single-app production shape as the binary.
- `backend/var/`, `backend/internal/web/dist/`, and `dist/` contain local or generated files. Their README files are placeholders that keep those ignored directories present in Git.

## Default Change Workflow

1. Read the active requirements and conventions for the touched area.
2. Make the smallest durable change that satisfies the slice.
3. Run the narrowest relevant validation.
4. Update docs or ADRs if the change introduces a durable new rule.
5. Keep commits focused.

## Migrations And Resetting Your Database

The governing rule is **Project Lifecycle And Migration Immutability** in
`docs/conventions.md`; this section is only how to carry it out. The unused
v0.1 candidate baseline may be redesigned under ADR 0013; installed release
migrations are immutable. Local development databases are disposable.

Adding schema:

```sh
# new sequential file, goose format (-- +goose Up / -- +goose Down)
$EDITOR backend/migrations/00NN_short_name.sql
./scripts/test-backend.sh
```

Reset only a disposable development database after a baseline rewrite. Stop the
app first, export anything worth keeping, and remove the SQLite file selected
by `DATABASE_URL` together with its `-wal` and `-shm` sidecars. For the default
`pnpm dev` database:

```sh
rm -f backend/var/dev.sqlite backend/var/dev.sqlite-wal backend/var/dev.sqlite-shm
```

Restart the app to run migrations and repeat owner setup. If `DATABASE_URL`
names another file, remove that file and its sidecars instead. For a populated
installation, follow [the upgrade and restore guide](upgrades.md).

The per-change reset notices are retained in the
[pre-release baseline rewrite archive](archive/pre-release-baseline-rewrites.md).
Databases and backups made before a baseline rewrite or consolidation are
incompatible with that baseline: export anything worth keeping before updating,
then recreate only the disposable development database. Do not use restore to
cross a rewritten-baseline boundary. Forward migrations below preserve data
only when the existing database uses the same `0001` baseline.

Forward migration `0002_basis_resolution_provenance.sql` (#197, 2026-10-13)
adds immutable evidence revision history without rewriting `0001` or resetting
databases made with the same `0001` baseline. It does not bridge a baseline
rewrite such as #148. Startup upgrades schema 1 to 2 while preserving all
financial rows and original declarations. Bundle schema 10 adds
`investment-basis-resolution-provenance.csv`; backups retain every revision.

Forward migration `0003_unresolved_capital_return.sql` (#169, 2026-10-13)
preserves known original/revised return-of-capital effects and adds explicit
knowledge with nullable reduction/excess pairs. Startup upgrades existing
schema-2 databases without a reset. Downgrade refuses unknown evidence; use the
verified backup/restore workflow for rollback. The baseline checksums remain
unchanged. Bundle schema 11 includes the new effect knowledge.

Forward migration `0004_posting_line_once_per_version.sql` (#224, 2026-10-14)
adds a unique index so a posting line has at most one posting per transaction
version. Startup upgrades existing schema-3 databases without a reset; one that
already holds such a duplicate fails the migration and must be recreated (no
installed release is affected). Bundle schema is unchanged.

The e2e database resets itself on every self-managed run and needs no manual reset.

- Keep nothing in a local database you would be sorry to lose. Reproducible
  fixtures belong in the repo; `backend/var/*.sqlite` is scratch.
- Rewriting an already-committed migration requires `BREAKING DEV DATABASE` as
  the first line of the commit body, plus the reset instruction for other
  developers. Grep for past ones with `git log --grep='BREAKING DEV DATABASE'`.
- Two branches that both added `00NN_` collide. The one merged second renumbers
  **its own** file to the next free number while resolving the merge. Renumbering
  a migration already on `main` counts as a rewrite.
- Migration numbers are monotonic database sequence numbers, not release
  numbers. Release notes record the highest migration included in each release.
- CI validates the fresh-install path on every run. Candidate seeded equivalence
  is covered by `TestMigrateSeededBaselineMatchesFreshSchema` in `internal/db`:
  it loads `internal/db/testdata/v01_seed.sql` into the consolidated baseline,
  upgrades through forward migrations, repeats migration admission, and checks schema equivalence and every durable
  figure — row counts, exact coefficients/scales, lifecycle states, lot and
  checkpoint conservation, per-commodity balance and `foreign_key_check`.
  Legacy upgrade fixtures are added once installed releases actually exist.
- **Installed-release seed fixtures are frozen.** The unused v0.1 candidate
  seed may be revised with its baseline under ADR 0013. Once a release is
  installed, its seed stands in for a database written by that version, so
  regenerating it against a later schema defeats the test it feeds. A new
  installed release gets a *new* seed file, built with
  `./scripts/build-release-seed.sh <out.sql>` — which drives the real HTTP API
  (`TestBuildReleaseSeedFixture` in `internal/api`) so the fixture is a book the
  app would actually have written, then shapes the dump. Output is not
  byte-reproducible: it carries real timestamps and request UUIDs.
- This is what stands between a post-`v0.1.0` schema redesign and silent data
  loss. SQLite's twelve-step table rebuild is how such a redesign has to happen,
  and a rebuild that re-points the wrong foreign key or rescales a coefficient
  leaves every row count identical. Widen the seed before writing a migration
  that touches something it does not cover.

## Work Tracking

Use GitHub issue numbers for active work and the
[work-item template](../.github/ISSUE_TEMPLATE/work-item.md) for a bounded
problem, delivery scope, acceptance, dependencies and validation. Future
initiatives may stay brief; complete actionable acceptance before coding.
Existing local codes remain historical aliases in `docs/backlog.md`.

Follow [Work Tracking](conventions.md#work-tracking) for event-driven progress
and closure. Update the roadmap/capability ledger when their facts change;
post validation and commit evidence on the feature issue. Delivery checklists
live on GitHub, while durable behavior and scope decisions live in plans/ADRs.

## Commit Conventions

This repo uses Conventional Commits.

Preferred examples:

- `feat(backend): add opening balance endpoint`
- `feat(frontend): add account list empty state`
- `fix(api): validate unbalanced postings`
- `docs(requirements): lock export scope`
- `test(backend): cover reconciliation lock behavior`

Use the smallest honest scope. Avoid mixing unrelated concerns in one commit.

A commit that rewrites an already-committed migration must additionally start
its body with `BREAKING DEV DATABASE` — see *Migrations And Resetting Your
Database* above.

## Branches And PRs

**Commit directly to `main`.** This is the default for all work, including
changes to product rules, conventions, workflows, and CI. Do not open a feature
branch unless there is a specific reason to (owner decision, 2026-08-23).

Why: the project is pre-release with a single contributor, so there is no
release to protect and no second reviewer for a PR to reach. Branches bought
nothing and cost something — the 2026-08 merge of two long-diverged branches is
where the duplicated `docs/conventions.md` paragraph, the duplicated
`writePricingServiceError` arms, the duplicate CodeQL workflow, and the
colliding T-42–T-47 backlog IDs all came from.

What replaces the PR as the safety net:

- **`main` stays green.** Run the narrowest relevant validation before pushing
  (see *Validation Expectations By Area* below). CI runs the same scripts, so a
  local failure is a push that should not happen.
- **Commits stay focused and honest.** The commit message is now the only
  review artifact, so it carries the reasoning a PR summary would have. A commit
  that changes product requirements, conventions, or ADRs says so in its body.
- **Push often.** Long-lived local work recreates exactly the divergence this
  rule exists to avoid.

Branch when the work genuinely needs isolation — a spike you may throw away, or
a change you want CI to run before it reaches `main`. That is a judgement call,
not a default.

**This flips at the `v0.1.0` release**, the same milestone that freezes
migrations (see *Project Lifecycle And Migration Immutability* in
`docs/conventions.md`). From that tag, feature branches and PRs become the norm:
once a released version exists, `main` is something users can be running.

## Validation Expectations By Area

### Backend

- Usually validate with `./scripts/test-backend.sh` — it gates on `gofmt -l`, then runs `go vet ./...` and `go test -race -p 2 -timeout=5m ./...`. Two packages run concurrently with a five-minute deadline each; normal test parallelism uses available Go CPU resources.
- If backend changes affect the integrated binary shape, also run `pnpm build`.

### Frontend

- Usually validate with `./scripts/test-frontend.sh`.
- If the change affects static build output, also run `pnpm build`.

### Docs And Requirements

- Keep cross-references and commands consistent.
- Run `git diff --check -- ...` for touched docs when no narrower executable validation exists.

### Testing

- Treat local validation scripts as the development contract.
- CI commands must match the local wrapper scripts exactly.
- Ordinary backend integration tests open an isolated copy of the process-wide
  migrated template through `internal/testdb.Open`. Use `db.Open` plus
  `db.Migrate` directly only when the test is specifically proving migration,
  restore, or fresh-start behavior; repeating the complete schema setup in
  ordinary fixtures makes the race-detector feedback loop needlessly expensive.

## GitHub Actions

The repo uses a fast CI workflow in `.github/workflows/ci.yml`.

Workflow conventions:

- Fast CI covers race-detected backend tests, frontend checks plus Vitest, and
  the integrated build.
- Go vulnerability scanning runs in `.github/workflows/govulncheck.yml` with
  `govulncheck ./...` from `backend/` on a weekly schedule, manually via
  `workflow_dispatch`, and on backend-affecting pushes to main and pull requests.
- CodeQL code scanning runs as GitHub's **default setup**, configured in
  repository settings (Security → Code scanning), covering `actions`, Go, and
  the JS/TS languages. There is no committed CodeQL workflow.

  From 2026-08-23 to 2026-09-10 this was an *advanced* setup — a committed
  `.github/workflows/codeql.yml` running `actions/setup-go` with
  `go-version-file: backend/go.mod` — because default setup builds Go with
  whatever the runner preinstalls under `GOTOOLCHAIN=local`, and CodeQL's own
  bundled Go extractor could not parse a 1.27 module besides. Both halves are
  now fixed: the extractor gained 1.27 support (github/codeql#22042, CodeQL
  bundle 2.26.4, early September 2026) and the hosted runners preinstall Go
  1.27, so default setup extracts the module cleanly. The `codeql.yml` workflow
  was removed 2026-09-10.

  Default and advanced setup are mutually exclusive. While default setup is
  enabled, re-adding a `codeql.yml` gets its SARIF rejected on upload with
  "CodeQL analyses from advanced configurations cannot be processed when the
  default setup is enabled". If the Go-toolchain problem ever recurs, restore
  advanced setup by disabling default setup in repository settings *first*,
  then committing a single workflow file that pins the toolchain with
  `go-version-file: backend/go.mod`.
- `gosec` scanning was removed 2026-09-10 (backlog T-59). It existed only as
  interim first-party Go coverage while CodeQL's Go analysis was paused; with
  CodeQL's Go queries restored under default setup, `govulncheck.yml` covers
  known dependency CVEs and CodeQL covers first-party taint bugs (SQL
  injection, SSRF, path traversal). Its 53 stale analyses and 71 orphaned
  Security-tab alerts — almost all false positives, see T-59 — were deleted
  through the code-scanning analyses API at the same time.
- Dependabot version updates are configured in `.github/dependabot.yml` for
  GitHub Actions, the backend Go module, the root pnpm workspace, and the
  Docker runtime image. The root npm entry updates `frontend/package.json`
  together with `pnpm-lock.yaml`; a separate `/frontend` entry creates PRs
  whose manifests fail CI's frozen install.
- Browser journeys run in the `E2E Smoke` job of `ci.yml` on every push and pull
  request: `./scripts/test-e2e-smoke.sh`, which builds the single binary, boots
  a throwaway instance, and runs every spec except `release-preflight.spec.ts`.
  The Playwright report uploads as an artifact on failure. The preflight suite
  stays out of CI and is run deliberately before a release.
- Use `pnpm install --frozen-lockfile` in CI.
- Keep CI commands aligned with the local wrapper scripts in `scripts/`.
- Use Node 24.
- Use the Go version declared in `backend/go.mod`.
- Keep job structure simple and readable.
- If workflows are reintroduced, start with manual triggers before considering automatic gates.

## Documentation Update Rules

- Update `docs/product-requirements.md` when scope or durable product behavior changes.
- Update `docs/conventions.md` when a repo-wide rule changes.
- Add or update an ADR in `docs/adrs/` when a decision locks a long-lived tradeoff.
- Update `README.md` and this document when developer commands, repo layout, or workflow expectations change. Avoid reintroducing area README files for short command notes; use them only when an area needs substantial local guidance.

## Archive Rules

- `.archive/` is reference only.
- Do not port archive implementation details directly.
- If an archive idea is adopted, rewrite it in current-stack terms and add it to active docs.

## Quote provider configuration (#117)

`COINGECKO_DEMO_API_KEY` enables the CoinGecko Demo quote adapter;
`TWELVE_DATA_API_KEY` enables Twelve Data quotes with the operator's own market
and display entitlements. Keys belong in deployment environment/secrets, never
in instrument identifiers. Yahoo Finance is keyless and explicitly unofficial.
Create/edit instruments and explicit provider identifiers under Investments →
Instruments, select a quote currency, save, and enable its provider assignment.
**Refresh quotes** queues durable work; source health and pricing refresh history
show completion. The saved daily pricing schedule also queues assigned quotes.
For provider evidence, failure recovery and the fixed crypto CSV columns, see the
[quote/crypto contract](plans/quote-providers-crypto-plan.md).

Forward migration `0006_crypto_instruments.sql` preserves existing instrument
versions, IDs and financial evidence, widens instrument/lot commodity guards to
crypto and registers quote providers. No development reset is required for the
same baseline. Downgrade preserves a stock-only database with unused quote sources; it
refuses crypto versions or referenced quote sources. Restore a verified
pre-upgrade backup when downgrade is refused. Ledger bundle schema is unchanged because instrument type,
commodity kind, precision, identifiers and generic pricing data were already
exported fields.
