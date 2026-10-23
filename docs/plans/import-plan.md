# Import & Online Ingestion Plan

A plan for first-class, modular, extensible data ingestion — file import (QIF,
CSV, XLSX, OFX/QFX, MS Money) and, later, online imports/updates (bank feeds) —
built on one unified pipeline.

Governed by `docs/product-requirements.md` (Phase 4) and `docs/roadmap.md`
(R4–R7). Aligns with `docs/conventions.md`, `docs/early-architecture-decisions.md`,
and the ADRs (notably ADR 0010 durable background work).

Status: **Slices 1 (pipeline + QIF) and 2 (CSV + profiles + minimal rules) shipped**
(roadmap R4/R5 — see `implemented.md`, Import Pipeline). Last updated
2026-08-29.

---

## Goals

1. **Migrate off MS Money** in the first slice — the concrete motivating use case.
2. **One ingestion model** for every source. A file upload and an online fetch
   are both *sources* feeding the same staging → normalize → dedupe → review →
   commit pipeline. Nothing touches the ledger unreviewed.
3. **Modular formats.** Adding a new file format or provider is a new *adapter*
   against a stable interface, not a pipeline rewrite.
4. **Per-provider variation.** CSV/XLSX differ wildly between banks. Column
   mapping is data (saved *profiles*), not code.
5. **Online-ready.** Online sources reuse the existing durable work queue
   (ADR 0010) so polling/refresh is restart-safe from day one of that work.
6. **Trust.** Source metadata is retained; duplicates are detected; imported
   rows land in the existing `needs_review` queue; every batch is auditable and
   reversible.

### Upload resource boundary (#228)

File staging and CSV header analysis accept files through 50 MiB
(52,428,800 bytes), inclusive. The complete multipart request is bounded at
51 MiB (53,477,376 bytes), including fields, headers, extra parts and epilogue,
even without Content-Length. Multipart parsing keeps at most 1 MiB of file
content in memory before spilling to temporary storage; the parser's bounded
non-file field, part-count and header limits also apply. Temporary files are
removed on success, malformed input, missing file, size refusal and downstream
service failure.

A file or multipart resource overflow returns HTTP 413 with
`IMPORT_UPLOAD_TOO_LARGE` in the normal error envelope. Both upload and CSV
analysis show the translated size message. No adapter parsing, import batch,
staged row, journal, lot, import audit or background work is created by a size
refusal. Never pass a truncated prefix to an adapter.

## Non-goals (deferred)

- Parsing the proprietary MS Money `.mny` (Jet/ESE) database directly. MS Money
  exports per-account **QIF** (`File → Export → Loose QIF`) and can produce OFX;
  QIF is the migration path. See "MS Money specifics".
- PSD2 / open-banking connectivity, screen-scraping, and credential storage —
  later, behind the online-source interface defined here.
- Attachment/statement-PDF import (out of scope per product requirements).

---

## What already exists (reuse, don't rebuild)

The ledger side of import is largely built; this plan adds the ingestion side.

- **Transaction service** (`app/transactions_write.go`, `CreateTransactionInput`)
  already accepts `OriginType`, `ExternalRefHint`, `MetadataJSON`, and
  `NeedsReview` — exactly the import-origin fields a commit step needs.
- **Review queue** — `needs_review` flag, `GET /transactions?needs_review=true`,
  and `POST /transactions/{id}/approve` already exist (backlog B-22). The import
  commit sets `needs_review=true`; the existing queue/approve flow reviews them.
- **Durable work queue** — `background_work_items` (generic `kind` + `payload_json`,
  lease/retry/resume) from ADR 0010. Online ingestion enqueues `kind="import.*"`
  work here; no new queue is needed.
- **Reconciliation guard** — import creates `posted` transactions, so the
  period-scoped guard applies. This is *not* automatic-and-done: a backdated
  import row landing inside an already-reconciled period is rejected by
  `CreateTransaction` unless `ReconciliationOverride` is supplied. Commit must
  therefore detect and surface this (see "Reconciliation handling at commit").
- **Double-entry invariants** — the commit step calls the normal transaction
  service, so balancing, scale, and account-policy validation are enforced for
  imported data identically to manual entry.

---

## Key decision: committed rows are `posted` + `needs_review`

The lifecycle taxonomy permits two paths for imported rows: commit them as
`posted` with `needs_review=true`, or have import act as a future producer that
persists `draft` rows for review. **This plan locks the `posted` + `needs_review`
path** for these reasons:

- The review-queue machinery for it already exists end-to-end (B-22:
  `needs_review` flag, list filter, `/approve`). R9 now supplies recurring
  drafts and their own review UI, but file imports retain this posted/review
  contract; recurring delivery does not change import commit semantics.
- `posted` rows are immediately visible in registers/reports/reconciliation, which
  is the migration experience a Money/Quicken user expects: "my data is here,"
  with `needs_review` as a *soft* "please confirm" overlay rather than a *hard*
  "not in the ledger yet" gate.
- It keeps imported and manual data on one code path (balancing, guard, audit),
  avoiding a draft→posted promotion step and its edge cases.

Trade-off accepted: imported rows affect balances before the user reviews them.
This is mitigated because (a) `needs_review` makes them filterable/auditable, and
(b) the whole batch is reversible via rollback. If a future, riskier producer
(e.g. speculative online auto-pull) needs a not-yet-in-ledger holding state, it
can adopt the reserved `draft` status without disturbing this file-import path.

---

## Architecture: one unified pipeline

```
        ┌─────────────── SOURCE ───────────────┐
        │  file upload          online fetch    │   ← adapters
        │  (QIF/CSV/XLSX/OFX)   (bank API/feed)  │
        └───────────────┬───────────────────────┘
                        │ raw bytes / raw rows
                ┌───────▼────────┐
                │  Format adapter │  parse → canonical staged rows
                │  (+ profile)    │  (provider profile maps columns)
                └───────┬─────────┘
                        │ StagedRow[]
                ┌───────▼────────┐
                │  Normalize      │  dates, money→Coefficient+scale,
                │                 │  payee/category/account resolution
                └───────┬─────────┘
                ┌───────▼────────┐
                │  Dedupe         │  vs existing ledger + within batch
                └───────┬─────────┘
                ┌───────▼────────┐
                │  Import batch   │  persisted staging tables, reviewable
                │  (preview)      │  PREVIEW — nothing in the ledger yet
                └───────┬─────────┘
                        │ user reviews / maps / approves
                ┌───────▼────────┐
                │  Commit         │  → TransactionService.CreateTransaction
                │                 │    (OriginType=import, NeedsReview=true)
                └───────┬─────────┘
                ┌───────▼────────┐
                │  Review queue   │  existing needs_review + /approve
                └────────────────┘
```

The boundary that makes it modular: **the format adapter is the only
format-specific code.** Everything downstream of `StagedRow[]` is shared.

### The core interface (Go)

Lands in slice 1; only the QIF adapter implements it initially.

```go
// Source adapters produce canonical staged rows from some input.
type SourceAdapter interface {
    // Format/source identifier, e.g. "qif", "csv", "ofx", "online.gocardless".
    Kind() string
    // Detect reports confidence that this adapter can handle the input
    // (sniff extension/magic bytes/headers) so the UI can auto-suggest.
    Detect(input RawInput) Confidence
    // Parse turns raw input + an optional provider profile into staged rows.
    // It MUST NOT touch the ledger; it only emits canonical rows + warnings.
    Parse(ctx context.Context, input RawInput, profile *Profile) (ParseResult, error)
}

type RawInput struct {
    Filename    string
    ContentType string
    Bytes       []byte          // file sources
    Rows        [][]string      // pre-tabulated sources (CSV/XLSX rows)
    // online sources carry their fetched payload here too
}

type ParseResult struct {
    Rows     []StagedRow
    Warnings []ParseWarning      // recoverable: unknown category, bad row, etc.
    Meta     SourceMeta          // account hints, currency hints, date range
}
```

`StagedRow` is the canonical, format-independent shape — close to a
`TransactionInput` but with *unresolved* references (payee/category/account as
raw strings/hints, money as raw string + detected scale) plus the composite
`dedupe_fingerprint` (see "Dedupe identity" below):

```go
type StagedRow struct {
    DedupeFingerprint string   // provider id, else scoped content hash (see below)
    Date              string   // raw, normalized later
    Amount            string   // raw decimal text; → Coefficient at normalize
    CommodityHint     string   // currency code/symbol if present
    PayeeHint         string
    CategoryHint      string   // QIF L / CSV column → category mapping
    AccountHint       string   // which account this row belongs to
    TransferHint      string   // QIF transfer [Account] / OFX transfer
    Memo              string
    ExternalRef       string   // bank txn id (OFX FITID, etc.) if any
    Splits            []StagedSplit
    Raw               map[string]string // verbatim source fields for audit
}
```

### Provider profiles (the per-bank variation layer)

A **Profile** is saved data, not code: it maps a concrete source layout to the
canonical fields. CSV/XLSX from different banks differ in column order, headers,
date format, decimal separator, sign convention, and amount-in-one-column vs
debit/credit-columns. A profile captures all of that.

```go
type Profile struct {
    ID            int64
    Name          string   // "ABN AMRO CSV", "Chase CSV", "MS Money QIF"
    AdapterKind   string   // which SourceAdapter it parameterizes
    ConfigJSON    string   // column map, date layout, decimal sep, sign rules…
}
```

- QIF/OFX are largely self-describing → they use a minimal default profile.
- CSV/XLSX need an explicit profile; the UI's column-mapping step *creates and
  saves* one so the next statement from the same bank is one click.
- Profiles are book-scoped data → a `import_profiles` table, not migrations.

This is the extension point that satisfies "CSV/XLSX can be very different
between providers" without touching Go code per bank.

---

## Data model (new migrations)

Staging is deliberately separate from the ledger so a preview never risks ledger
integrity and a batch is fully reversible until commit.

The pre-beta baseline migration (`0001_initial_schema.sql`):

- `import_batches` — one per upload/fetch. Columns: `id`, `book_id`, `source_kind`,
  `profile_id` (nullable), `status` (current state, mutable:
  `previewing`/`committed`/`partially_committed`/`discarded`/`failed`),
  `original_filename`, `source_meta_json`, `created_at`. `status` is a denormalized
  *current* pointer; the authoritative history lives in `import_batch_events`.
- `import_batch_events` — append-only lifecycle log, one row per state change
  (`created`, `parsed`, `committed`, `partially_committed`, `discarded`,
  `failed`, `rolled_back`). Columns: `id`, `batch_id`, `event_kind`, `detail_json`,
  `audit_event_id`, `created_at`. This replaces a single `audit_event_id` on the
  batch so every operation (preview, commit, discard, fail, rollback) keeps its
  own attribution — consistent with the version/lifecycle + `audit_events` split
  used elsewhere (`who/when/how/why` lives in `audit_events`).
- `import_staged_rows` — canonical rows for a batch: `id`, `batch_id`,
  **`book_id`** (denormalized so uniqueness/dedupe constraints are enforceable in
  SQLite without traversing `import_batches`), `dedupe_fingerprint`, `raw_json`,
  `normalized_json`, `dedupe_status`
  (`new`/`duplicate`/`needs_attention`/`excluded`), `resolution_json` (chosen
  account/category/payee), `commit_status` (`pending`/`committed`/`skipped`/`failed`),
  `committed_transaction_id` (nullable, set on commit), `commit_error` (nullable).
- `import_commit_identities` — the durable idempotency record. One row per
  *committed* import row: `id`, `book_id`, `dedupe_fingerprint`,
  `committed_transaction_id`, `source_kind`, `account_id`, `created_at`, with
  `UNIQUE(book_id, dedupe_fingerprint)`, plus `source_fingerprint`, the staged
  row's fingerprint an account-scoped key was derived from (#302). Keeping idempotency here (not on the
  transient staged rows) means re-importing the same file finds prior commits even
  after the originating batch is discarded, and staging stays disposable.
- `import_profiles` — saved per-provider profiles (above).

### Dedupe identity (not just source content)

A naive book-global hash of source text is wrong: two legitimate rows can share
the same date/payee/amount (recurring bills; the same coffee twice; the same
amount across two accounts). The `dedupe_fingerprint` is therefore a composite,
chosen by precedence:

1. **Provider transaction id when present** — OFX/QFX `FITID`, or any bank-unique
   id a CSV profile maps. This is the strongest key; prefer it whenever available.
2. **Otherwise a scoped content hash** over
   `(source_kind, profile_id, resolved_account_id, date, amount, commodity,
   normalized_payee, memo, intra-file occurrence-index)`. The occurrence-index
   disambiguates genuinely-identical same-file rows (e.g. two identical coffees)
   so they are *not* collapsed into one.

Dedupe runs in two passes: **within the batch** (mark later identical-fingerprint
rows for review, never silently drop) and **against the ledger** via
`import_commit_identities` (a fingerprint already committed → `duplicate`,
excluded from commit by default but overridable per row in the preview).
Account scope matters: the same row can legitimately appear in two accounts'
exports, so the committed identity includes the resolved account (checked at
commit, see below), and the importer pairs two-account transfers into one
balanced transaction rather than double-counting.

> **Implemented identity (2026-10-17, [#299](https://github.com/sergeyfarin/rekenraam/issues/299)).**
> QIF and CSV content keys are built the same way: `qif|date|amount|payee|memo`
> and `csv|profile:<id>|date|amount|payee|memo` (payee lower-cased), each plus
> `|occurrence:<n>`, where `n` counts the identical rows above it **in the same
> file**. Neither includes the file name or the row's position, so a renamed or
> overlapping re-export finds the rows it already committed
> (`TestImportAuditQIFReexportIdentityIgnoresFileNameAndPosition`).
>
> **Account scope (2026-10-18, [#302](https://github.com/sergeyfarin/rekenraam/issues/302)):
> checked per account at commit, option (b).** A CSV or QIF bank row is
> committed under an identity scoped to the account it posts to: the
> `import_commit_identities.dedupe_fingerprint` key is
> `hash(<staged fingerprint>|account:<id>)`, and `source_fingerprint` keeps
> the staged fingerprint it came from. The same row in two accounts'
> statements is therefore two transactions, and a repeat of either statement
> is a duplicate of its own account only
> (`TestImportAuditIdenticalRowsInTwoAccountsStatementsCommitToEach`).
>
> - *Why not a staging input (option a).* A bank statement names no account
>   until review, and the QIF `!Account` name is not reliable (a
>   single-account `!Type:Bank` export normally omits it, so the identity
>   would depend on how the file was exported, the #299 failure again).
>   Option (c), one profile per account, would not cover QIF at all.
> - *Review before the account is picked.* Staging flags a row `duplicate`
>   if its staged fingerprint was committed to **any** account, so a repeat
>   import still reads as duplicates at once. When an import rule already
>   resolved the account, staging checks that account only. A `duplicate`
>   flag kept through review (as the review screen sends it) is re-decided
>   when the patch names an account: it stays `duplicate` only if the row was
>   committed to that account, and otherwise becomes `new` (`needs_attention`
>   if the row carries `parse_errors`, #300). The commit-time identity check
>   uses the scoped key too, so an included row already committed to its
>   account is still skipped, and a resumed batch (#301) re-checks each row
>   against the account it resolves to then.
> - *Bank references.* A CSV `external_ref` identity is scoped to the
>   account as well. A bank may number each account's statement on its own,
>   and a false duplicate drops a transaction by default, while the scope
>   costs nothing for a repeat of the same account's statement.
> - *Book-global identities.* Trading 212 rows (scoped by connection and
>   provider id), crypto CSV rows (scoped by holding and cash account) and
>   CSV investment rows keep one book-global key, which equals their source
>   fingerprint.
> - *Not covered.* Pairing the two sides of a transfer that appears in both
>   accounts' exports is still a separate step; each side is its own row.

---

## API surface (OpenAPI-first, `/api/v1`)

Preview/commit is a two-phase flow; nothing is committed implicitly.

- `POST /api/v1/imports` — start a batch from an uploaded file (multipart) or an
  online source ref. Runs detect + parse + normalize + dedupe → returns the
  batch id and a preview (staged rows, warnings, dedupe flags, account/currency
  hints).
- `GET /api/v1/imports/{batch_id}` — fetch batch + paginated staged rows
  (reuses cursor pagination conventions).
- `PATCH /api/v1/imports/{batch_id}` — apply resolutions: choose target accounts,
  map category/payee hints, mark rows include/exclude, attach/save a profile.
- `POST /api/v1/imports/{batch_id}/commit` — create `posted`,
  `needs_review=true` transactions via the transaction service; set per-row
  `commit_status` and `committed_transaction_id`. Accepts a body with commit
  semantics and an optional `reconciliation_override` (see below). Idempotent:
  re-running skips rows already in `import_commit_identities`.
- `POST /api/v1/imports/{batch_id}/preview-commit` — dry-run that returns the
  reconciliation impact (affected checkpoints per account) and the new-vs-duplicate
  counts **without** writing, so the UI can warn before a real commit. Reuses the
  existing `reconciliation-impact` machinery.
- `POST /api/v1/imports/{batch_id}/discard` — drop a preview batch.
- `GET /api/v1/imports` — batch history (audit trail / rollback entry point).
- `import_profiles` CRUD under `/api/v1/import-profiles`.

### Reconciliation handling at commit

A backdated import row can fall inside an already-reconciled period. The commit
flow handles this explicitly rather than relying on the guard to "just work":

- **Detect at preview.** `preview-commit` runs the existing per-transaction
  reconciliation-impact check for every includable row and flags those that would
  cross an active checkpoint, naming the affected checkpoint(s) — the same data
  the manual editor's warning modal already consumes.
- **Decide at commit.** The commit body carries `reconciliation_override`
  (default `false`). Without it, rows that would cross a checkpoint are **skipped**
  (`commit_status='skipped'`, reason recorded) rather than aborting the whole
  batch. With it, those rows commit and invalidate the affected checkpoints
  through the same override-and-invalidate path used by manual edits.

### Commit semantics: partial by default

Batch commit is **per-row transactional, partial by default**, not
all-or-nothing. One unbalanced/invalid/guarded row must not block an otherwise
clean 500-row statement.

- Each row commits in its own DB transaction; failures set
  `commit_status='failed'` + `commit_error` and leave the row re-committable:
  every later commit of the batch retries it.
- A row that would cross a reconciliation checkpoint without
  `reconciliation_override` is **held**, not skipped: it stays `pending` with
  `commit_error` "reconciliation override required" and is listed in the
  response's `reconciliation_hold_row_ids`. Gain-review and dividend-review
  holds are pending the same way.
- `skipped` is final and only for rows that must not post from this batch:
  duplicates, owner exclusions, rows whose identity already committed,
  unsupported Trading 212 fills and changed-source fills awaiting a correction.
- The batch lands in `committed` only when no row is pending or failed. With a
  held row it is `partially_committed` (even if nothing committed yet); with
  only failed rows left it is `partially_committed`, or `failed` when nothing
  committed. Counts go in the response and an `import_batch_events` row.
- `partially_committed` and `failed` batches stay open: `PATCH` re-resolves
  their pending and failed rows (a re-resolved failed row is `pending` again),
  and preview and `commit` run again. Committed and skipped rows are immutable.
  Idempotency ensures already-committed rows are not duplicated. Discard stays
  limited to `previewing` batches, which have committed nothing.
- The result screen offers "review remaining rows" for failed and held rows,
  back to the preview with the commit controls. Re-uploading the file also
  still works, because identities are recorded only on commit
  ([#301](https://github.com/sergeyfarin/rekenraam/issues/301),
  `TestImportAuditHeldAndFailedRowsResumeInTheirBatch`).

### Rollback

Slice 1 ships rollback as a **documented use of existing lifecycle endpoints**: a
batch knows its `committed_transaction_id`s, so the UI offers "void/soft-delete
these N imported transactions" through the existing void/soft-delete endpoints —
no new ledger deletion path. A **dedicated batch-rollback endpoint**
(`POST /imports/{batch_id}/rollback`, recording a `rolled_back` event and
clearing the matching `import_commit_identities`) arrives in Slice 3 alongside the
import history view, once the multi-row UX justifies it.

---

## MS Money specifics (first-slice target)

MS Money (discontinued; the free "Sunset" build still runs) has **no clean
programmatic export of its `.mny` database**. The supported migration path is:

1. In MS Money: `File → Export`, choose **Loose QIF**, export **per account**
   (MS Money exports one QIF per account; investment accounts export separately
   and more lossily).
2. Upload each `.qif` to Rekenraam; map its account to a Rekenraam account in the
   preview; commit.

QIF caveats the importer must handle (and surface as warnings):

- QIF has **no currency field** → currency comes from the chosen target account /
  a batch-level currency selection.
- QIF dates are ambiguous (`MM/DD/YY` vs `DD/MM/YY`) → date layout is a profile
  setting with a detected default and a preview so the user can correct it.
- QIF transfers use `[Account Name]` in the category field → mapped to a transfer
  via `TransferHint` against resolved accounts; produces one balanced transaction.
- Splits use `S`/`E`/`$` lines → mapped to `StagedSplit[]`.
- Investment (`!Type:Invst`) actions are partially lossy in MS Money's QIF →
  slice 1 imports cash/banking accounts cleanly and flags investment rows as
  `needs_attention` rather than guessing. Full investment import is later, after
  the investments UI.

The UI ships a short "Exporting from MS Money" help panel with these steps,
localized like all copy.

---

## Delivery slices

Each slice leaves the app runnable, tested, documented. Slices map to roadmap
R4–R7.

### Slice 1 — Pipeline + QIF (MS Money migration) — ✅ shipped
- Beta baseline: `import_batches`, `import_batch_events`,
  `import_staged_rows` (with `book_id`), `import_commit_identities`,
  `import_profiles`.
- `SourceAdapter` interface + a small adapter **registry**; **only QIF**
  implemented. Normalize, composite-fingerprint dedupe, and the
  staging/preview/commit pipeline.
- API: `POST /imports`, `GET /imports/{id}`, `PATCH`, `preview-commit`, `commit`
  (partial, override-capable), `discard`.
- Commit goes through `TransactionService` with `OriginType=import`,
  `NeedsReview=true`, per-row transactional; reuses the existing review queue.
  Reconciliation-crossing rows are skipped unless `reconciliation_override`.
- Frontend: upload → preview table (dedupe + reconciliation + warning flags) →
  account/currency map → commit; MS Money export help. All copy via Paraglide.
- Rollback in slice 1 = "void/soft-delete these imported transactions" over
  existing lifecycle endpoints (the batch knows its transaction ids).
- Tests: QIF parse table tests (splits, transfers, date layouts, MS Money
  sample), within-batch + against-ledger dedupe idempotency,
  commit-creates-balanced-posted, partial-commit on a bad row, reconciliation
  skip/override, rollback via void.
- **Acceptance:** a real MS Money loose-QIF export imports end-to-end, lands in
  the review queue, re-importing the same file is a no-op, and a row backdated
  into a reconciled period is skipped-with-warning unless overridden.

### Slice 2 — CSV + provider profiles (the variation engine)
- ✅ First vertical cut (2026-08-28): CSV adapter + saved **column-mapping
  profile** creation/list/reuse UI (header detection,
  date/decimal/sign config, amount-single vs debit/credit, save profile).
- ✅ Profile update/delete and safe filename/header auto-suggestion
  (2026-08-28). A uniquely best compatible profile is selected; tied matches
  remain a user choice. Deleting a profile preserves historical batches.
- ✅ Grouped unknown-payee resolution (2026-08-28): distinct unresolved names
  are handled once per preview; the user links an existing fuzzy near match or
  explicitly creates a payee, and the choice is applied to every matching row.
- ✅ Minimal preview-time rules (2026-08-29): ordered, case-insensitive literal
  contains matching on payee or description; the first match snapshots category,
  payee, and/or tags visibly into new staged rows. No regex, amount predicate,
  retroactive application, or per-application audit trail.
- **Acceptance:** two different banks' CSV layouts import via two saved profiles
  with no code changes.

### Explicit CSV investment income — #241 (2026-10-15)

Opt-in profiles map existing instrument codes and explicit dividend/withholding
kinds, signed cash, separate gross/tax and account defaults. Review choices,
exact net checks, named writer admission and source-identity atomicity follow
[the CSV investment income contract](csv-investment-income-plan.md).
The #393 refinement permits absent payer columns/cells while preserving exact
known income/tax and import identity. Supplied unresolved codes still require
review; no payer or economics is guessed. Bank-only
profiles retain Slice 2 behavior. Trading 212's separate missing-default fallback
is #242, not part of this CSV slice.

### Slice 3 — XLSX + duplicate-review depth
- XLSX adapter (reuse CSV's row pipeline; XLSX → `[][]string`).
- Richer duplicate review UI; import audit-trail/history view; explicit
  batch rollback action.

### Slice 4 — OFX/QFX
- OFX/QFX adapter (FITID-based dedupe, native currency, statement balances).
- Covers bank statement downloads and is the bridge toward online sources.

### Slice 5 — Online ingestion (durable, restart-safe) — ✅ foundation shipped
- `OnlineSource` adapters implementing the same `SourceAdapter` contract, driven
  by the **durable work queue** (`kind="import.fetch.*"`), so scheduled/refresh
  pulls resume after restart exactly like FX coverage (ADR 0010).
- Fetched payloads flow into the *same* batch/preview/commit pipeline.
- Trading 212 now proves credential handling, scheduling UI, a provider-specific
  adapter, durable fetching, and investment-lot import end to end. See
  `docs/plans/trading212-import-plan.md` and `docs/implemented.md`.

### Slice 6 — Import automation and cleanup (deferred)

- Persistent user-defined rules run during staging before human review. A rule
  matches payee/description/amount and can set category, payee, and tags.
- Merge duplicate payees; support bulk recategorization; make the existing
  `needs_review` queue useful for imported-but-unreviewed transactions.
- Keep every automated transformation visible in staging and auditable. Rules
  must not bypass preview or commit directly to the ledger.

### Slice 7 — Additional bring-your-own-key providers (deferred)

- Evaluate IBKR Flex Query for the investor persona, GoCardless Bank Account
  Data for EU bank feeds, and SimpleFIN Bridge for US bank feeds only after the
  CSV/profile and automation slices demonstrate demand.
- Each provider reuses the same source → stage → review → commit pipeline and
  preserves the no-promised-coverage product boundary.

---

## Cross-cutting rules

- **Nothing implicit:** parse/preview never writes to the ledger; only `commit`
  does, and only through the transaction service.
- **Idempotent:** the composite `dedupe_fingerprint` recorded in
  `import_commit_identities` makes re-imports safe even after the source batch is
  discarded; commit re-runs skip already-committed fingerprints.
- **Auditable:** each batch state change is its own `import_batch_events` row with
  its own `audit_event_id`; committed transactions carry `OriginType=import` and
  source metadata in `MetadataJSON`.
- **Reversible:** slice-1 rollback reuses void/soft-delete (no new deletion
  semantics); a dedicated batch-rollback op arrives in slice 3.
- **i18n / a11y / mobile:** all import UI follows the same boundaries as the rest
  of the app; loading/empty/error/success states defined per screen.
- **No floating point:** money parses raw text → `exact.Coefficient` + scale at
  the normalize step, never through float.

## Unrecognized payees in review (follow-up, opened 2026-08-19)

T-44 made a typed payee name resolve to the record that carries it on every
write path, and made manual entry confirm before creating a new record. A bulk
import cannot ask per row, so it deliberately commits an unrecognized name as
free text and leaves it unlinked — those transactions land in the spending
report's "no payee recorded" group.

The interim path was to open such a transaction and edit the payee, where the
editor forces link-or-create. Shipped 2026-08-28: preview now groups staged rows
by distinct unknown payee name and resolves each once — link to an existing
payee (with the same fuzzy near-match search the editor uses) or explicitly
create the record — applying the choice to every row carrying that name.
`ImportResolution.payee_id` was already present, so this remained review UI
over the existing contract.
