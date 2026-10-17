# Receipts & Attachments Plan (proposed R14)

Status: **design proposed; no attachment feature implemented**. Written
2026-07-19; status reconciled 2026-10-17. Feature proposal for receipt capture,
automatic recognition, and attachment storage. Proposes a resolution of the
open product decision "attachment storage, retention, access-control, backup, and
encryption model" (`docs/product-requirements.md`), which currently lists
attachments as deliberately out of scope. Sequencing is decided in
`docs/roadmap.md`; this plan does not authorize starting attachments ahead
of the roadmap's current initiative or accepting attachment scope without a
product decision.

## Goal

The user photographs or uploads a receipt; Rekenraam stores the file
durably in a human-browsable folder, recognizes date / amount / currency /
merchant, and either attaches it to the matching existing transaction or
creates a draft transaction for review. Receipts without a match wait in an
inbox.

## Constraints (non-negotiable, from existing decisions)

- **Single binary stays single.** Recognition is proposed as browser-side or
  user-provided, with no additional server OCR installation. ADR 0014 already
  uses CGO for bundled SQLite; CGO itself is not prohibited. Any OCR dependency
  still needs an explicit build, packaging and maintenance decision.
- **Privacy-first by default.** No receipt image leaves the user's machines
  unless the user explicitly configures an external endpoint (BYO, same
  spirit as provider keys).
- **Financial documents are financial records.** Storage gets the same
  care as the ledger: dedupe, integrity hashes, inclusion in the backup
  story, no silent deletion.
- **Drafts are producer-owned.** Receipt-generated transactions use the
  reserved system-only `draft` status — this is the second planned producer
  for that workflow (R9 recurring is the first). A receipt never posts a
  transaction without review.

## Design

### Storage (slice R14a — the foundation, also useful alone)

- Files live under `ATTACHMENTS_DIR` (default `<data dir>/attachments/`),
  organized human-browsable as `YYYY/MM/<yyyy-mm-dd>-<slug>-<id>.<ext>`
  (slug from merchant/description once known; renamed on recognition). The
  folder is the user's — browsable, rsync-able, theirs.
- DB carries the index and integrity, not the bytes:
  - `attachments`: id, sha256, byte_size, mime_type, original_filename,
    stored_path, source (`upload` | `camera` | future `email`), created_at,
    created_by_user_id, audit linkage.
  - `attachment_links`: attachment_id → transaction_id (nullable —
    NULL = inbox). One receipt may attach to one transaction v1;
    many-to-many stays open.
- Content-hash (sha256) dedupe: re-uploading the same receipt links the
  existing file. Deletion is soft (unlink + trash semantics) consistent
  with ledger recovery philosophy; hard delete only from the inbox.
- Limits: configurable max file size (default ~10 MB), allowlisted types
  (JPEG/PNG/WebP/HEIC?/PDF). HEIC likely needs client-side conversion —
  decide during R14a.
- **Backup story extension (required, same slice):** extend the existing
  SQLite online-backup workflow with an attachments-directory copy; the
  self-check gains an attachments integrity pass (every DB row's file exists
  and hashes match). Update `docs/deployment-security.md` and
  `docs/upgrades.md` with the combined backup and restore procedure.
- Access control: existing session auth; files served only through an
  authenticated endpoint, never as static paths.
- Encryption at rest: deferred with the same reasoning as SQLite encryption
  (documented, revisit before higher-risk deployment recommendations) —
  keep the two decisions aligned.

### Capture & inbox (slice R14b)

- Upload on the transaction editor (attach directly) and a **Receipts
  inbox** page: drag-drop / file picker / mobile camera capture
  (`<input capture>` — the existing responsive-web mobile story, no native
  app).
- Inbox shows thumbnail, recognized fields once available, and actions:
  attach-to-transaction (search assisted), create-draft, dismiss.
- Transaction read models gain attachment counts; the register/table shows
  a paperclip affordance.

### Recognition (slice R14c)

Two engines, one interface, both optional:

1. **Default: in-browser OCR** (tesseract.js WASM at capture/review time).
   Zero server dependencies, zero privacy exposure, works offline on LAN.
   Quality is adequate for printed register receipts; the user corrects
   fields in the inbox UI (corrections are cheap; totals get first-class
   treatment).
2. **Optional: BYO vision endpoint** — user-configured OpenAI-compatible
   endpoint (which may be a local Ollama/llava instance — still private) for
   markedly better extraction on crumpled/handwritten/foreign receipts.
   Same BYO-key rules as providers: off by default, explicit opt-in,
   endpoint + key sealed in secretbox.

Extraction targets, in priority order: **total amount + currency** (exact
decimal parse — the no-float rule applies to OCR output too), **date**,
**merchant**, optional line items/VAT later. Every extraction carries a
confidence and is user-editable before anything touches the ledger.

### Matching & drafts (slice R14c, continued)

- Matcher: candidate posted transactions within ±3 days and amount within
  rounding tolerance in the same currency (exact comparison at aligned
  scales — reuse ledger arithmetic, no floats). One candidate → suggest
  attach; several → pick list; none → offer **create draft** with payee =
  merchant (payee matching against existing payees), amount, date, and the
  user's default account prefilled.
- Draft creation goes through the existing producer-owned draft semantics:
  system-created, excluded from ledger, promoted via the normal post flow
  (which, per the 2026-07-13 fixes, runs the full reconciliation guard).
- Import-rules synergy is proposed only. R5 literal contains-match rules ship
  at file staging; applying them to receipt drafts needs its own explicit
  contract and tests. Existing rules do not authorize another producer.

## Scope and prerequisite disposition

[The roadmap](../roadmap.md#deliberately-later) owns placement. The 2026-08-05
decision kept R14a after R5 and excluded it from announcement gates; it did
not accept attachment scope or require it before the now-delivered R16.
R5, backups/self-check and R9 draft lifecycle already ship. Their availability
removes foundation prerequisites, not the need to decide storage, retention,
authenticated access, crash consistency, encryption and combined restore.
R14a storage/manual attach, R14b capture/inbox and R14c recognition/matching
remain proposed boundaries. No files, attachment endpoint or integrity pass
are implemented. The existing backup documentation's future attachment hook
is not a backup of receipt files. Recheck OCR/browser capabilities and any
external endpoint terms when this proposal is considered for implementation.

## Non-goals (v1)

- Email-in ingestion, bulk document management, full-text search over
  receipts, warranty/return tracking, multi-page statement parsing (that's
  the import pipeline's job), server-side OCR binaries, any cloud OCR
  default.

## Open questions to resolve at slice start

- HEIC handling (convert client-side vs reject with guidance).
- Whether attachments join the trash/restore UI or get their own.
- Retention policy surface (none v1 — keep forever — but decide the
  deletion audit shape).
- Whether R14a should also cover *statement* PDFs attached to
  reconciliation sessions (cheap adjacency, same table).
