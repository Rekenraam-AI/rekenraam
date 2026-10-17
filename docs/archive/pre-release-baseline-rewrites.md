# Pre-release baseline rewrite notices (historical)

Status: superseded development notices, archived 2026-10-14. These record
individual rewrites and bundle versions at the time; they are not the current
schema head or reset procedure. Use [the developer workflow](../developer-workflow.md#migrations-and-resetting-your-database)
for baseline compatibility, forward migrations and resetting disposable data,
and [the upgrade guide](../upgrades.md) for data-preserving upgrades and restore.
The notices below retain their original wording.

**BREAKING DEV DATABASE, R16 slice 2a:** the pre-release `0001` investment
foundation rewrite changes table columns and the baseline checksum. Stop the
app and remove the SQLite file named by your disposable `DATABASE_URL` along
with its `-wal` and `-shm` sidecars before restarting. For the common local
`dev.sqlite` path, run:

```sh
rm -f backend/var/dev.sqlite backend/var/dev.sqlite-wal backend/var/dev.sqlite-shm
```

No installed v0.1 database exists; do not use this reset rule for a later
installed release.

**BREAKING DEV DATABASE, R16 slice 2b:** `0001` now stores ordered import
identity effects and links committed staged rows to their identity. The
baseline checksum changed again. Stop the app and reset the same disposable
`DATABASE_URL` file and its `-wal`/`-shm` sidecars using the command above
before restarting. No installed v0.1 database exists.

**BREAKING DEV DATABASE, R16 slice 3:** `0001` now stores trade charge kinds
and explicit operational disposal proceeds and per-lot proceeds allocations. The baseline checksum changed.
Stop the app and reset the same disposable `DATABASE_URL` database and its
`-wal`/`-shm` sidecars using the command above before restarting. No installed
v0.1 database exists.

**BREAKING DEV DATABASE, R16 slice 4c:** `0001` now stores append-only
effective disposal revisions and their allocations. The baseline checksum
changed. Stop the app and reset the same disposable `DATABASE_URL` database
and its `-wal`/`-shm` sidecars using the command above before restarting. No
installed v0.1 database exists.

**BREAKING DEV DATABASE, R16 slice 5b:** `0001` now records typed in-kind
transfer facts and lot links, and admits transfer-in lot events. Its checksum
changed. Stop the app and reset the same disposable `DATABASE_URL` database
and sidecars using the command above before restarting. No installed v0.1
database exists.

**BREAKING DEV DATABASE, R16 slice 5d:** `0001` now admits explicit
`transfer_out` lot events and enforces one source-lot link per transfer.
Its checksum changed. Stop the app and reset the same disposable
`DATABASE_URL` database and sidecars using the command above before
restarting. No installed v0.1 database exists.

**BREAKING DEV DATABASE, R16 review correction:** `0001` now keys disposal
decisions by `(operation_id, decision_seq)` rather than requiring one decision
per journal transaction/version. Its checksum changed. Stop the app and reset
the same disposable `DATABASE_URL` database and sidecars using the command
above before restarting. No installed v0.1 database exists.

**BREAKING DEV DATABASE, R16 component integrity:** `0001` now records
whether a charge was paid separately and links journal-backed source components
to their posting versions. Its checksum changed. Stop the app and reset the
same disposable `DATABASE_URL` database and sidecars using the command above
before restarting. No installed v0.1 database exists.

**BREAKING DEV DATABASE, R16 journal-link authority (2026-09-30):** `0001`
now removes `investment_operations.transaction_id`; pinned journal links are
the sole relationship. The checksum and frozen seed changed together. Stop the
app and reset the same disposable `DATABASE_URL` database and sidecars using
the command above before restarting. No installed v0.1 database exists.

**BREAKING DEV DATABASE, R16 splits (T-122, 2026-10-03):** `0001` now
records sourced split ratios and append-only split effect revisions. The
checksum changed. Stop the app and reset the same disposable `DATABASE_URL`
database and sidecars using the command above before restarting. No installed
v0.1 database exists.

**BREAKING DEV DATABASE, split adjustments (T-129, 2026-10-03):** `0001`
adds `investment_split_revisions.adjustment_transaction_version_id` and its
trigger guard. The checksum changed. Reset the disposable `DATABASE_URL`
database and sidecars the same way before restarting.

**BREAKING DEV DATABASE, R16 transfer basis propagation (T-132, 2026-10-03):**
`0001` adds `investment_transfer_link_revisions` and its latest-revision view,
and the effective lot-event view now excludes revised transfer events. The
checksum changed. Stop the app and reset the same disposable `DATABASE_URL`
database and sidecars using the command above before restarting. No installed
v0.1 database exists.

**BREAKING DEV DATABASE, R16 effective reads (T-124, 2026-10-03):** `0001`
merges `investment_lot_facts` into `investment_lots.operation_id`, adds the
effective-read views, and writes the former ALTER/recreated view and trigger
replacements as final DDL. The checksum and frozen seed changed together.
Stop the app and reset the same disposable `DATABASE_URL` database and
sidecars using the command above before restarting. No installed v0.1
database exists.

**BREAKING DEV DATABASE, consolidated baseline (2026-09-30):** no legacy
or installed databases exist. Source revision tables and lot opening guards
are folded into `0001`, with an updated checksum. The embedded migration head
is again 1. Fresh and seeded equivalence tests cover the complete baseline.

**BREAKING DEV DATABASE, lot identity/state split (2026-09-30):** the
consolidated `0001` now separates immutable `investment_lots` from mutable
`investment_lot_state`. The checksum and frozen seed are updated together.
Fresh/seeded tests cover schema, exact balances, replay and bundle schema 3.
No legacy or installed databases exist.

**BREAKING DEV DATABASE, disposal-clearing attribution (2026-09-30):** the
consolidated `0001` adds immutable decision-to-posting proceeds portions.
The checksum and frozen seed are updated together. Fresh/seeded bundle schema
4 preserves these links; self-check conserves decisions and shared postings
independently. No legacy or installed databases exist.

**BREAKING DEV DATABASE, projected-basis knowledge (2026-10-01):** consolidated
`0001` stores explicit known/unknown projection knowledge and requires NULL
remaining basis/scale for unknown. Checksum and frozen known-basis seed are
updated together. Fresh/seeded bundle schema 5 appends knowledge; API unknown
basis fields are NULL, with quantities and independently priced value retained.
Unknown-input commands still require their immutable-fact/replay contracts.
No legacy or installed databases exist.

**BREAKING DEV DATABASE, transfer-basis evidence (T-145, 2026-10-07):**
`0001` now requires both carried-basis amount fields to be NULL when an
immutable transfer link is marked unknown. A named cost currency may remain;
known basis still requires the complete amount and currency. This closes the
partial-amount admission gap without enabling unknown transfer commands.
The candidate checksum changed; stop the app and reset the disposable
development database using the reset procedure above. The known-basis frozen
seed remains valid without changes to its data or column layout.

**BREAKING DEV DATABASE, known-zero resolution (#168, 2026-10-10):**
`investment_basis_resolutions.basis_value` admits `'0'` for a journal-free
known zero. The checksum changes. Reset the disposable database and its
sidecars using the procedure above.

**BREAKING DEV DATABASE, resolution correction (#168, 2026-10-10):**
`0001` replaces the one-resolution-per-link unique constraint on
`investment_basis_resolutions` with a link index. The insert guard admits a
second fact only when no other resolution of the link is still effective, and
a replacement must correct a resolution of the same link. The checksum and
index count change. Reset the disposable database and its sidecars using the
procedure above.

**BREAKING DEV DATABASE, sourced basis resolution (T-145, 2026-10-09):**
`0001` adds `investment_basis_resolutions` with its insert guard and
immutability triggers. Its effective-link view applies an effective
resolution, and transfer-link revisions may move unknown to known. The
checksum and schema-object count change, and the bundle adds
`investment-basis-resolutions.csv`. Reset the disposable database and its
sidecars using the procedure above.

**BREAKING DEV DATABASE, unknown transfer links (T-145, 2026-10-09):**
`0001` gives transfer-link revisions and their pooled depletions
`basis_knowledge` with paired nullable amounts. It refuses a revision that
changes its link's knowledge, admits an unknown pooled-lot link, and makes the
effective-link view read revision knowledge and amounts as one tuple. The
checksum changes and bundle schema 9 appends the columns. Stop the app and
reset the disposable database and its sidecars using the procedure above.

**BREAKING DEV DATABASE, immutable disposal knowledge (T-145, 2026-10-07):**
`0001` adds `basis_knowledge` with paired nullable basis amounts to disposal
decisions, allocations, replay revisions and revision allocations, plus an
insert guard that ties each allocation's knowledge to its decision/revision
total and disposal lot event. The checksum, the frozen seed's explicit known
flags and the schema-object count change together. Bundle schema 9 appends the
knowledge columns. Stop the app and reset the disposable database and its
sidecars using the procedure above. No installed database exists.

**BREAKING DEV DATABASE, immutable opening knowledge (T-145, 2026-10-07):**
`0001` adds `opening_basis_knowledge` to immutable lots and `basis_knowledge`
to lot events, permitting only paired NULL amounts for unknown evidence.
The checksum and explicit known flags in the frozen event seed change together.
Bundle schema 9 appends original/event knowledge; original lot amounts become
nullable in the API. Public unknown-transfer/disposal commands remain gated.
Stop the app and reset the same disposable database and its sidecars using the
procedure above before starting this baseline. No installed database exists.

**BREAKING DEV DATABASE, return-of-capital quantities (T-148, 2026-10-07):**
`0001` adds the `explicit_quantities` entitlement rule and expands its insert
guard; whole-lot entitlements retain their original meaning. The checksum
changed, with fresh/seeded schema equivalence validation. Stop the app and
reset only the same disposable `DATABASE_URL` database and sidecars using the
command above, then redo owner setup. No installed v0.1 database exists.

**BREAKING DEV DATABASE, exact-number CHECKs (#192, 2026-10-12):** every
`0001` coefficient column now requires the canonical form `exact.Coefficient`
writes over the entire stored value: at most 38 ASCII digits, an optional `-`,
no leading zero and no `-0`. An explicit NUL search also rejects embedded
and trailing NULs, which SQLite's text predicates otherwise stop before. Each
column also states whether it is signed, signed and nonzero, nonnegative or
positive; `TestExactNumberColumnChecks` lists the rule per column. The checksum
changed again for the NUL admission fix. Stop the app and reset only the same
disposable `DATABASE_URL` database and sidecars using the command above, then
redo owner setup. No installed v0.1 database exists. Candidate seed rows remain canonical; seeded
schema equivalence and a fresh HTTP-built release seed validate the stricter
baseline without regenerating historical financial figures.

**BREAKING DEV DATABASE, required lot openings (#148, 2026-10-13):**
`0001` now requires `investment_lots.operation_id` and `source_transaction_id`
to be `NOT NULL`. Repository fixtures use posted buys through the shared writer;
replay and dated reads no longer skip lots without openings. The checksum and
candidate seed review change together; its existing opening IDs and financial
figures remain unchanged, with fresh/seeded equivalence preserving those links.
Stop the app, export anything worth keeping, remove the disposable database
named by `DATABASE_URL` and its `-wal`/`-shm` sidecars as above, restart and redo
owner setup. Existing development databases and backups from the prior baseline
are incompatible; do not use restore across this rewrite. No installed v0.1
release exists. Forward migration `0002` remains unchanged.

The final pre-`v0.1.0` consolidation changed the highest schema version from 8
to 1. Databases and backups made before that consolidation are incompatible in
both directions: export anything worth keeping before updating, then recreate
the development database from the consolidated baseline. Do not use restore to
cross that boundary.

**BREAKING DEV DATABASE, #289 (2026-10-17):** `0001` now gives `auth_sessions`
`AUTOINCREMENT` and stores `audit_events.auth_session_id` and
`authentication_events.auth_session_id` as plain integers instead of
`ON DELETE SET NULL` foreign keys; `0014` rebuilds `authentication_events` the
same way. Events keep naming their session after the daily cleanup deletes it,
and the id is never reused. A forward migration was not possible: SQLite
cannot drop a foreign key in place, and rebuilding `audit_events` (referenced
under `ON DELETE RESTRICT` by every ledger table) or `auth_sessions` (whose
drop would null every event reference) requires foreign keys off, which a
transactional migration cannot do. The checksum changed; the frozen seed loads
unchanged. Stop the app, export anything worth keeping, remove the disposable
database named by `DATABASE_URL` and its `-wal`/`-shm` sidecars as above,
restart and redo owner setup. Do not use restore across this rewrite. No
installed v0.1 release exists.
