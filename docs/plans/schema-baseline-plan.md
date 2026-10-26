# Schema baseline and necessity review (#405)

This is the design and evidence contract for [#405](https://github.com/sergeyfarin/rekenraam/issues/405),
requested 2026-10-26 against `9b414c08`. The issue owns acceptance and progress;
the [roadmap](../roadmap.md) owns work order. This review includes head migration
27, not just the 26 migrations in the original issue.

## Purpose and boundaries

Reduce the schema that every connection loads and every write compiles, then
package it as one unused v0.1 baseline. Do not equate fewer migration files,
tables or triggers with a better model. Preserve exact values, version history,
source identity, reconciliation, audit attribution, supported partial data and
the existing refusal messages. ADRs 0012, 0013, 0017 and 0019 govern these
boundaries. A newly discovered defect gets a named regression and a separate
disposition; a policy change needs its own accepted family contract.

Prior evidence matters:

- [#402](../reviews/race-suite-cost-2026-10-24.md): migration replay was 4.5%
  of application race CPU; schema loading was 19.4%, preparation 32.7%.
- [#404](../reviews/statement-compilation-2026-10-24.md): explicit reuse shipped;
  the owner declined a connection statement cache. Keep that setting and ADR
  0016's race instrumentation, package concurrency and deadline unchanged.
- [#385](../reviews/owner-current-reference-2026-10-22.md): reference reads use
  an application-pinned owner date, while insertion guards use source dates.
  UTC-current views are not the runtime contract.
- [#391](../reviews/financial-dependency-inventory-2026-10-23.md): actual
  effects/allocation inputs differ from repairable attribution. Do not restore
  historical payment/parent blockers while simplifying SQL.
- #192's full-string coefficient checks, #148's required lot openings,
  #224's posting-line uniqueness, #289's retained session attribution,
  #366's throttle generations and #403's reused-reader FTS behavior must
  survive. A passing balance alone cannot prove those properties.

## Review and delivery slices

1. **Inventory and protection before edits.** Capture the actual migrated
   native SQLite schema, built-in rows and object ordering. Map every trigger,
   view, index and table constraint to its purpose and named behavioral tests.
   Distinguish direct refusal coverage, workflow coverage and unproven coverage;
   a text match or an object-existence assertion is not behavioral proof.
   Add missing refusal/read tests before modifying the corresponding object.
   Record all candidates, including those rejected, and workflow asymmetries.
2. **Simplify within the model.** Examine duplicate/prefix indexes including
   uniqueness, collation, ordering, predicates and FK lookup support; dead
   views; repeated trigger lookups; superseded DDL; duplicated table CHECKs;
   immutable-fact versus mutable-projection boundaries. Keep independently
   meaningful typed facts. Do not replace constraints with application-only
   checks or introduce SQL functions that make external tools/backups depend
   on connection initialization. Preserve errors and NULL semantics. Compare
   query plans and invalid-write outcomes; test multiple-invalid-input cases
   when guard composition or trigger order changes.
3. **Collapse and compatibility.** Write final DDL once, in dependency-safe
   order, retaining meaningful comments and built-in data. Document the
   `sqlite_schema` diff by object with the justification for each change.
   Rebase the unused candidate seed while independently preserving its exact
   financial facts; exercise fresh HTTP seed generation. Pin the checksum.
   Move final-behavior assertions out of migration-path tests; explain every
   retired path/case. Keep fresh-file startup, seeded equivalence, idempotent
   admission and restore tests. Refuse old development files and backups before
   any replacement, including old version-1 files: a version number alone
   cannot distinguish rewritten baselines. Keep genuine future-schema refusal
   separate. Document export-before-update and disposable-file reset; never
   auto-reset a database. Use `BREAKING DEV DATABASE` in the commit body.
4. **Measure and review delivery.** Prebuild matched base/head binaries. Run
   three interleaved uncached application-package rotations and two uncached
   full-gate pairs, sequentially without competing validation work. Measure
   native per-connection open and representative write preparation separately
   from migration replay. Record host/toolchain, object counts/SQL bytes,
   run spread, medians, failures and case-name digests, with an explicit
   moved/retired/added-case map. Compare with ADR 0016's 210/270 s thresholds;
   do not claim noise as a gain. Run backend race/vet, frontend checks,
   integrated build, E2E smoke and fresh seed generation. Update the roadmap,
   capability ledger and current operator guidance together. Commit each
   independently validated slice; leave #405 open until all acceptance is met.

## External comparison and research

Sources inspected 2026-10-26. These inform choices; they do not supersede the
accepted accounting model.

- [SQLite schema storage](https://www.sqlite.org/fileformat.html#storage_of_the_sql_database_schema)
  stores table/index/view/trigger SQL in `sqlite_schema`. Its
  [query-planner guidance](https://www.sqlite.org/queryplanner.html#multi_column_indices)
  recommends avoiding an index that is a prefix of another. Apply that only
  after checking uniqueness, predicates, collations, ordering and representative
  plans. [FK guidance](https://www.sqlite.org/foreignkeys.html#fk_indexes)
  explains why child-key indexes still matter even when deletes are rare.
- [SQLite trigger semantics](https://www.sqlite.org/lang_createtrigger.html)
  require one event per trigger; update and delete immutability guards cannot
  simply become one trigger. `UPDATE OF` silently ignores unknown column
  names, so shortening an immutability guard that way needs a complete-column
  test. Prefer preserving a sound small guard over moving its protection into
  a new abstraction. Guard order and overlapping refusals need explicit tests.
- [Flyway's rebaselining guidance](https://documentation.red-gate.com/fd/baselines-273973441.html)
  distinguishes greenfield consolidation from baselining an existing installed
  schema. Our owner-confirmed absence of installations makes the former
  possible; it is not a license to erase installed history after v0.1.
- [Actual's migration directory](https://github.com/actualbudget/actual/tree/master/packages/loot-core/migrations)
  is a useful finance-application comparison for incremental migration history.
  Its presence does not establish equivalent accounting/audit constraints or
  justify copying its storage model. Inspect concrete source before asserting
  a more specific similarity.
- Vassiliadis and Karakasidis,
  [*Time-Related Patterns of Schema Evolution*, EDBT 2025](https://www.cs.uoi.gr/~pvassil/publications/2025_EDBT/),
  examine 151 projects and several evolution patterns. This cautions against
  extrapolating a permanent migration-growth rate from a short feature burst;
  it does not establish a performance benefit from consolidation.
- Brahmia, Grandi and Oliboni,
  [*A Literature Review on Schema Evolution in Databases*, 2024](https://cris.unibo.it/handle/11585/974714),
  compare semantics, propagation, integrity and software evolution as separate
  dimensions. Applied here: inspect reads, writes, constraints, exports and
  restore separately. This is an engineering inference, not a claim that the
  paper validates our implementation. No reviewed research justifies adding a
  schema-evolution platform to this single-user SQLite application.

## Initial candidates, subject to proof

The preliminary SQL inventory found six explicitly declared indexes covered by
other keys: `journal_entries_version_idx`, `posting_versions_journal_idx`,
`posting_versions_version_idx`, `investment_instruments_book_idx`,
`import_staged_rows_batch_idx`, `import_connection_holdings_connection_idx`.
The first two duplicate unique-key prefixes; all require native query-plan
verification and inspection of ordering before removal.

The initial text search found five UTC-based reference views with no literal
production Go references after #385:
`current_account_versions`, `current_commodity_versions`,
`current_institution_versions`, `current_payee_versions`,
`current_investment_instrument_versions`. Tests still read or assert them.
The deeper review found the repository fallback constructs their names in
`CurrentReferenceVersions`; removal must inline the same UTC fallback as well
as moving direct test reads to the selector. Transaction and
investment effective-history views serve different purposes and remain subject
to their own review.

Compatibility currently relies on migration numbers and the source checksum
test; the runtime does not persist baseline identity. A new head of 1 would
otherwise silently accept an old development database also at 1, while goose
would emit its generic missing-migration error for other old heads. Review
startup, inspection and restore together, before selecting the smallest durable
identity mechanism.


## Review refinement

The full source/schema reachability sweep also found two dormant tables,
`instrument_source_links` and `instrument_search_cache`, their three indexes,
and a deletion-guard reference to the former. No runtime producer, reader,
export, frontend/API contract or fixture uses them. R17 stores quote identities
in versioned instrument metadata and local search uses the instrument versions.
Remove these objects in the unused baseline, with the current R17 contract
clarified in the same change. Keep all other reference-protection clauses.
Six more redundant indexes serve descending entity-version sequences already
supported by reverse scans of unique `(entity_id, version_seq)` keys; verify
those plans alongside the first six.

## Delivery record

The implementation is committed as `f4ee790f`, following the separately
committed plan. The [dated review](../reviews/schema-baseline-review-2026-10-26.md)
records the complete native object diff, test preservation map, research
cross-check, corrections, rejected alternatives, retained workflow discrepancy
and final matched measurements. All six isolated application runs and four
uncached full gates passed. The act threshold clears; investigate remains.
The browser timeout/reruns, hosted-CI limit and rejected external follow-up
publication are explicit in that record. No release gate or feature order is
changed by the retained opportunities.
