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
consistency task, not a reason to reorder #395. Tracked as #417.

**Retained opportunities, without promoting them into the queue.** The
unreferenced `InstrumentSearchProvider`/`InstrumentSearchCandidate` declarations
can be removed in a later focused code cleanup (#418). The unreachable legacy pricing
schema probe is already deferred in the
[owner-reviewed maintainability inventory](maintainability-audit-2026-10-24.md#explicitly-deferred-inventory-original-ranks-713);
this review supplies fresh schema evidence, not a claim that it was removed.
No new local work IDs are allocated.

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

Implementation: `f4ee790fad60b01efb2262dc626efdb412c97b3b`; original source:
`9b414c08`. The [measurement artifact](fixtures/schema-baseline-measurements-2026-10-26.json)
contains source hashes, UTC start times, phase/package timings, case digests,
case changes and native probe batches. The host exposed ten CPUs (AMD EPYC
3251), 17.8 GiB at the recorded snapshot, Linux 6.8.0-147 and Go 1.27.3.
SQLite is bundled 3.53.4 (`mattn/go-sqlite3` 1.14.52). Both sources retain
CGO/FTS5, FULL, race instrumentation, statement cache 0 and unset GOMAXPROCS.
The shared gate retains `-p 2 -timeout=5m`.

### Native schema operations

The temporary [probe](fixtures/schema-baseline-probe-2026-10-26_test.go.txt)
runs five batches of 100 connection open/close operations per fresh schema
stage, and 100 prepare/close operations for
`INSERT INTO posting_versions DEFAULT VALUES`. It never executes that insert.
The medians below include closing each connection or statement. All captured
SQL is ASCII, so `length(sql)` also equals its byte length.

| Schema | Objects | Stored SQL, bytes | Open + close, ms | Prepare + close, ms |
|---|---:|---:|---:|---:|
| Original migration 1 | 457 | 190,814 | 7.469 | 0.286 |
| Original migration 20 | 499 | 206,026 | 8.224 | 0.305 |
| Original head 27 | 511 | 217,145 | 8.842 | 0.312 |
| Proposed head 1 | 489 | 212,067 | 8.236 | 0.293 |

Final stored SQL shrinks **2.3%** (5,078 bytes), not by the migration-file
ratio. The measured open/close median falls **6.9%**; write-preparation batch
ranges overlap, so its approximately 0.312 → 0.293 ms medians are weaker
evidence. Empty-file open/close is about 0.72–0.74 ms. These are sequential
single-host schema probes, not a randomized experiment, confidence interval or
production request-latency claim.

### Matched application and shared gates

The [runner](fixtures/schema-baseline-matched.py) prebuilds the application race
binaries, then executes three interleaved original/proposed rotations. Isolated
runs retain #402's ten-minute diagnostic deadline; real shared gates retain
five minutes. No competing test/build jobs run during the accepted measurements.
A one-off inventory scan overlapped part of original full gate 1; that limits
attributing command-time differences solely to this change. The earlier development trials
and failed pre-correction head gate are excluded, with the latter recorded in
the artifact. Successful original gate 1 was retained because its source did
not change; both proposed gates use the final implementation.

| Variant | Application seconds, three runs | Median | Leaf cases |
|---|---|---:|---:|
| Original | 245.465, 240.712, 229.327 | **240.712** | 2,455 |
| Proposed | 231.395, 229.568, 215.163 | **229.568** | 2,453 |

All six application runs pass. Each variant has one stable leaf-name digest across
its three runs, recorded in the artifact. The four unchanged skips are
opt-in measurement tools. The application changes exactly twelve old leaf
names into ten: ten renamed final-behavior/damage-control leaves and two
retired transition-only leaves (crypto downgrade and known-capital-return
forward migration). The full repository's four retired transition-only tests,
retained controls, new names and same-name changes are in the
[case map](fixtures/schema-baseline-test-moves-2026-10-26.json).

The observed median reduction is **4.6%**; paired reductions are about
4.6–6.2%. Both variants also drift faster in the third rotation, and their
ranges overlap. This is a modest measured suite gain, not proof that all
production work is 4.6% faster. Retirement of obsolete transitions contributes;
only the separate probes isolate schema operations. No result supports
weakening a financial guard for a larger timing reduction.

| Full gate | Application, s | Database, s | Race execution phase, s | Whole command, s |
|---|---:|---:|---:|---:|
| full-base-1 | 271.774 | 220.109 | 371.55 | 421.326 |
| full-head-1 | 248.514 | 97.684 | 316.85 | 359.606 |
| full-base-2 | 289.042 | 194.396 | 400.03 | 406.508 |
| full-head-2 | 262.509 | 102.379 | 340.44 | 347.012 |

All four full gates pass, uncached, including formatting, vet and every race
package. Command time includes compilation/vet and scheduling; it is not a
per-package deadline metric. At this R13-inclusive original source both full
application runs exceed 270 s. At the proposed source neither does, so ADR
0016's **act** threshold no longer fires. The **investigate** threshold still
fires: the isolated median exceeds 210 s. The 30% headroom objective is not
restored; thresholds, concurrency, cache policy and durability are unchanged.
Reprofile the resulting baseline before choosing further cost work; this
review does not promote a deadline or cache change into the feature queue.

### Validation and reproduction limits

Frontend validation passed **778 unit tests**, and the integrated build passes.
Fresh release-seed generation prepares 272 statements and passes the native
source/consumer round trip, including admission triggers, FKs and existing
ledger/lot/checkpoint oracles. The original-schema cleanup negative control
fails only unused crypto; stock cleanup and both referenced/posted controls
pass. The final-source cleanup controls all pass.

The initial 450-case browser smoke run passed 449 cases and timed out once in
the error-state route sweep. That case passed twice unchanged in isolation;
the complete 12-case audits group passed on rerun. After the unversioned-file
admission correction, the rebuilt binary passed both owner setup/login
bootstrap cases. No frontend change was justified by that single timeout.
This remains an explicitly recorded intermittent validation observation.

To reproduce, use clean original/final checkouts, run the native probe
separately in each (versions `0,1,20,27` original, `0,1` proposed), and remove
the temporary Go file before running the matched runner. Export a writable
GOCACHE and invoke `schema-baseline-matched.py BASE HEAD OUTPUT`, with no
competing validation jobs. The [reducer](fixtures/schema-baseline-results.py)
records terminal outcomes/digests and explains the case changes. Resume only
runs whose already-passing source is unchanged. The schema inventory's own
script accepts the native captures and reads only index/FK metadata through
Python SQLite; it does not substitute that engine for native behavior tests.

Local implementation and the required measured acceptance are complete.

## Post-push record (2026-10-26)

Hosted CI at `4b3ef084` (the same runtime source as `f4ee790f`): Backend
Tests, Backend Coverage, Frontend Check, Integrated Build and E2E Smoke
(audits) pass. E2E Smoke (app) failed one of 438 cases. In
`investments-capital-return-reads` (nl, mobile), the reversal click stalled in
the browser for 28 s while every backend request finished in under 260 ms.
No evidence links it to this change; it is #414. The next run, at `0ac0a6cf`,
passed every job, and #405 is closed.

Follow-ups are tracked on GitHub: #417 (category-version deletion guard),
#418 (unreferenced search declarations), #419 (reprofile the race suite;
ADR 0016 investigate still fires) and #420 (`verify-backup` reports an
unrestorable backup with exit status 0). The post-push audit is recorded on
#405.
