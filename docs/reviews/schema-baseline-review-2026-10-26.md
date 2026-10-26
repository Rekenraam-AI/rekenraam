# Schema baseline and workflow review — 2026-10-26

[#405](https://github.com/sergeyfarin/rekenraam/issues/405), against
`9b414c08`, after #404 and the completed R13 core. The owner reconfirmed that
there are **no legacy databases**. The [plan](../plans/schema-baseline-plan.md)
sets the delivery boundaries; the issue owns acceptance and the
[roadmap](../roadmap.md) owns subsequent work order.

This review distinguishes schema simplification, corrections discovered while
reviewing it, and retained opportunities. It does not infer that fewer tables
or passing balances prove a sound financial model.

## Decisions and schema diff

Consolidate migrations **0001–0027** into `0001_initial_schema.sql`, head **1**,
with baseline generation **20261026** in `PRAGMA user_version`. No conversion
of an obsolete development database is attempted. The next additive migration
is 0002 and retains the same generation. Installed-release migrations and seed
fixtures remain immutable after v0.1.0.

The native `sqlite_schema` comparison is recorded per object in the
[inventory](fixtures/schema-baseline-inventory-2026-10-26.json). It includes
stored table constraints and test pointers, separates direct refusals from
workflow/read evidence, and does not claim exhaustive predicate-branch
coverage. Implicit indexes belong to their table's PRIMARY KEY/UNIQUE contract;
FTS5 shadow objects belong to SQLite's search-index implementation.

| Object family | Change | Reason and proof |
|---|---|---|
| Six duplicate/prefix indexes | Remove `journal_entries_version_idx`, `posting_versions_journal_idx`, `posting_versions_version_idx`, `investment_instruments_book_idx`, `import_staged_rows_batch_idx`, `import_connection_holdings_connection_idx` | Existing non-partial unique keys cover the same equality prefixes and the required entry/line/row ordering. The lookup test checks retained keys and actual planner paths, without a temporary sort. Uniqueness remains on the retained keys. |
| Six descending sequence indexes | Remove the entity-sequence indexes on commodity, institution, account, payee, transaction and instrument versions | A reverse scan of the unique `(entity_id, version_seq)` key supports descending sequence under equality on the entity. These are not removed on a superficial column-name match: collation, direction, equality prefix and uniqueness were checked. |
| Five UTC reference views | Remove `current_account_versions`, `current_commodity_versions`, `current_institution_versions`, `current_payee_versions`, `current_investment_instrument_versions` | Production services pin an owner date. The deeper review found the dynamic repository fallback in `CurrentReferenceVersions`; it now inlines the same UTC predicate instead of returning a view name. Existing date/sequence selection and owner-day tests remain. Transaction and investment effective-history views are retained. |
| Dormant provider storage | Remove `instrument_source_links`, `instrument_search_cache` and their three indexes | Neither table has a runtime producer, reader, export, fixture or API/frontend contract. Local search reads instrument versions; R17 stores quote identities in versioned `identifiers_json`. The source-link clause in the instrument-version deletion guard therefore has no reachable application state to protect. All remaining reference checks stay. The R17 contract records this disposition. |
| Commodity-version deletion guard | Permit unused crypto alongside unused securities | The existing compensation repository already allows both kinds, but its guard rejected every crypto commodity. The real cleanup regression fails on the original guard; both unreferenced kinds now clean up, while referenced and posted controls preserve all financial rows and audit history. This is a correction, separately identified below. |

All surviving table definitions, including their coefficient CHECKs, NULL
knowledge pairs, foreign keys and keys, are retained verbatim from the native
head. No immutability trigger is removed. Apart from the two guard changes
above, surviving trigger bodies remain verbatim and retain their relative
creation order. This avoids inventing a new precedence among overlapping
refusals. Built-in rows keep their codes and IDs.

`TestBaselineIndexesKeepOrderedAndForeignKeyLookups` proves the twelve retained
access paths. `TestCurrentVersionViewsUseEffectiveDateBeforeVersionSequence`,
the reference-date baseline cases and `TestCurrentReferenceSiblingsAndComposedReadsUseOwnerDay`
exercise selection behavior. The existing financial, correction, reconciliation,
import, quote, report and restore suites exercise the remaining model.

## What the critical review retained

| Candidate | Disposition |
|---|---|
| Merge journal versions with the audit log | Reject. Versions preserve reconstructable facts; audit rows identify the operation. A generic before/after JSON log would change the accepted model and its queries (ADRs 0012/0013). |
| Collapse typed investment facts into generic operation JSON | Reject. Transfer lineage, lot elections, disposal clearing, short obligations, explicit entitlement and basis knowledge have different calculation and evidence roles. Removing those relationships does not preserve allocations merely because the journal balances. |
| Merge mutable lot state into lot openings | Reject. Original acquisition evidence and a rebuildable current projection are deliberately separate. Replay, dated valuation and unresolved basis depend on that distinction. |
| Remove validity views because insert guards look similar | Reject. A guard admits a staged write before all children are present; a validity view checks the completed fact graph. Dividend shape/component checks illustrate the difference. Effective views also select correction standing, while historical exports must retain original facts. |
| Combine update/delete immutability guards | SQLite requires one event per trigger. A shared application-only guard or connection authorizer would weaken protection and complicate external tools. The small event-specific guards remain. |
| Drop positive-value guards already covered by coefficient CHECKs | Retain. Their named refusal messages and precedence are existing behavior. Whole-string CHECKs still reject malformed representations, NULs, padded zero and unsupported signs; shorter approximate predicates would weaken #192. |
| Drop every prefix-looking index | Reject. Partial predicates, mixed direction, collation, uniqueness and covering/order requirements can make similarly named indexes independent. The review removes only the twelve demonstrated redundant keys, plus storage belonging to the two unused tables. |
| Drop book keys or actor/audit references for a single-user app | Reject. The book boundary and attributed immutable evidence are accepted financial contracts. An apparently redundant same-book check can also bind nullable audit attribution or transaction lineage. Table count is not a necessity test. |
| Flatten price-derivation dependencies back into JSON | Reject. The indexed graph serves void cascades and self-check; #104 removed repeated JSON scans. |
| Replace Goose with per-table versions, add a schema framework or enable a statement cache | Reject for this slice. Existing migration and durability decisions stand; the owner declined the cache in #404. |

Some duplicated lot-value CHECK text could be shorter, but changing its
admission/error details for a few hundred characters is not justified by this
measurement. Quantity representation and exact arithmetic remain unchanged.

## Workflow findings and their disposition

**Fixed: unused stock versus crypto compensation.**
`DeleteUnusedInstrument` removes an instrument and its newly created commodity
only when unreferenced. Its final commodity DELETE allowed `security` and
`crypto`, while `commodity_versions_no_delete` exempted only securities. A
stock/crypto control through the real repository reproduced the difference.
The new tests also cover rollback after a commodity reference and early refusal
for a posted instrument, so making cleanup consistent does not admit deletion
of used financial history. The operation's audit rows survive cleanup.

**Fixed: the release-seed tool used incompatible workflow helpers.** The
ordinary soft-delete ran through an investment helper that read the now-hidden
transaction back, producing a false 404 assertion. Its ordinary edit also
omitted the expected version that the browser supplies. The seed builder now
uses the existing ordinary transaction helper, retaining its real HTTP writes.

**Fixed: a successful dump was not a usable seed.** Real round-trip validation
first refused lot openings before their operation headers and then refused
clearing allocations before their journal links. FK deferral does not defer
trigger reads. The generator now orders those headers/links, restores the
book currency after commodities and before account/posting admission, emits
explicit column lists, and validates the proposed file before replacing its
output. `TestValidateReleaseSeedFixture` loads it through native constraints,
compares source counts/exact figures (including trigger side effects), runs the
existing ledger/lot/checkpoint oracles, and repeats migration admission.
The original pinned fixture's financial values and IDs are preserved; its
recorded gross-dividend shape is carried explicitly without guessing a payer.

**Retained follow-up: category versus holding version deletion.** The
`account_versions_no_delete` trigger's custom-category branch permits deleting
a used category's version, while the equivalent used holding version is
refused. The native [probe](fixtures/schema-category-deletion-probe-2026-10-26_test.go.txt)
reproduces both outcomes and rolls back all changes. The category service
already refuses used-category deletion, as
`TestCategoryDeleteRejectsCategoryUsedByPosting` proves; this is a database
defense/complexity discrepancy, not a demonstrated HTTP bypass. A bounded
follow-up can remove the special exception after preserving unused/starter
cleanup, historical children and default-payee references. This baseline keeps
the existing rule explicit rather than quietly changing its refusal policy.
Suggested follow-up acceptance: the same SQL-level attempted removal of a used
category or holding version refuses atomically; unused custom/starter cleanup
still works; historical-child and default-payee references still refuse; the
existing API refusal and error contract remain. This is a bounded database
consistency task, not a reason to reorder #395.

**Retained opportunities, without promoting them into the queue.** The
unreferenced `InstrumentSearchProvider`/`InstrumentSearchCandidate` declarations
can be removed in a later focused code cleanup. The unreachable legacy pricing
schema probe is already deferred in the
[owner-reviewed maintainability inventory](maintainability-audit-2026-10-24.md#explicitly-deferred-inventory-original-ranks-713);
this review supplies fresh schema evidence, not a claim that it was removed.
No new local work IDs are allocated.

Automatic approval review rejected publishing a GitHub follow-up because it
could not establish trust/authorization for the destination and payload.
The findings are therefore recorded here; no alternative external posting was
attempted.

## Migration and backup semantics

The generation marker distinguishes an old version-1 file from the new version
1; comparing only Goose numbers would silently accept it. Startup refuses
obsolete generations and populated unversioned files before Goose writes.
The recovery command still creates its verified pre-migration backup before
admission; its old unversioned-auth fixture now proves refusal with the password,
sessions and missing migration history unchanged. Escaped-path and read-only
backup assertions remain. An interrupted first migration that
left only empty version-zero bookkeeping can retry. A local empty baseline
down/up remains a fresh-file control.

Inspection reports an incompatible baseline separately from a genuinely newer
forward schema. Restore checks generation before preserving/replacing any live
file. Named database and CLI tests cover old development heads 1/26/27,
same-number backup refusal, byte preservation, process-lock release and the
existing newer-schema refusal. No automatic data reset or conversion exists.

The [reset notice](../archive/pre-release-baseline-rewrites.md),
[developer procedure](../developer-workflow.md#migrations-and-resetting-your-database)
and [upgrade guide](../upgrades.md) agree. Export with the original build before
updating, then remove only disposable development data and redo setup.

## Test preservation

The [case map](fixtures/schema-baseline-test-moves-2026-10-26.json) distinguishes
retired historical transitions from retained final-schema assertions. Fresh
migration, baseline-only seeded admission, repeated startup, schema equivalence
and restore tests remain. The seeded source still uses only migration 1, so a
future 0002 will genuinely upgrade that populated source rather than loading
the frozen fixture into an already-upgraded schema.

Old intermediate up/down and backfill cases no longer represent an installed
database. Their final facts/immutability, incomplete/warning states, source
links, nullable attribution, saved document bytes and exact financial results
remain tested. Saved report document schemas 1/2/3 remain supported; they are
not obsolete database baselines. Damaged duplicate source-claim diagnostics use
an explicit baseline damage control rather than reconstructing an old schema.

New protection covers both cleanup kinds, posted/reference refusals, retained
index paths, baseline identity, old-backup file preservation, empty migration
recovery, tag-link admission, explicit entitlement immutability and the release
seed's actual consumer. Case-name digest changes are explained by the map and
measurement artifact, rather than described as an unchanged suite.

## External cross-check

The [plan](../plans/schema-baseline-plan.md#external-comparison-and-research)
records SQLite, Flyway and 2024/2025 research sources and limits.

- [Actual's performance-index migration](https://github.com/actualbudget/actual/blob/master/packages/loot-core/migrations/1780606215001_add_performance_indexes.sql)
  adds targeted transaction indexes, while its
  [transaction-view migration](https://github.com/actualbudget/actual/blob/master/packages/loot-core/migrations/1608652596044_trans_views.sql)
  demonstrates a composed read surface. The applicable lesson is to justify
  storage/access paths from real consumers, not copy another application's
  transaction or audit model.
- [GnuCash's SQL backend](https://code.gnucash.org/docs/STABLE/classGncSqlBackend.html)
  has explicit per-table version operations, and its
  [object backend](https://code.gnucash.org/docs/STABLE/gnc-sql-object-backend_8cpp_source.html)
  checks versions on creation. It is evidence for explicit compatibility
  handling, not a reason to replace this app's accepted global Goose sequence.
- The EDBT 2025 schema-evolution study describes multiple change patterns;
  it does not justify extrapolating a permanent weekly migration rate or
  asserting consolidation's performance benefit without measurement. The 2024
  survey's separate semantics, propagation and integrity dimensions inform the
  separate write/read/export/restore checks here. These are engineering
  inferences, not research validation of this implementation.

## Measurements and validation

The implementation slice passed the complete native backend race/vet gate,
frontend validation (778 unit tests), integrated build, fresh release-seed
generation and native source/consumer round trip. The initial 450-case browser
smoke run passed 449 cases and timed out once in the error-state route sweep.
That case passed twice unchanged in isolation and the complete 12-case audits
group passed on rerun. After the unversioned-file admission correction, the
rebuilt binary also passed both owner setup/login bootstrap cases. No frontend
change was justified by the transient timeout. Hosted CI has not been run.

Final matched measurements follow in the measurement slice. Development trial
timings and the failed pre-correction head gate are excluded. #405 stays open
until the required rotations and two full-gate pairs are assessed.
