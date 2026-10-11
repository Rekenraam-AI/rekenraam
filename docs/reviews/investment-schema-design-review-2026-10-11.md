# Investment schema design review (2026-10-11)

Reviewed at `b769f09e`. Product requirements, conventions and ADRs 0012/0013
govern. This review records evidence and proposals. `docs/roadmap.md` alone
defines work order, and GitHub owns issue acceptance, state and priority.

**Question asked:** is the SQL design overly complicated, in a way that hides
issues or invites mistakes? Speed was the trigger
([#164](https://github.com/sergeyfarin/rekenraam/issues/164)) but not the
question.

**Scope:** the investment model only, meaning schema, `internal/db/investment*`,
replay and the effective reads. The core ledger (transactions, versions,
postings), imports, recurring and forecast were not reviewed.

**Revision:** the same day, an independent challenge re-checked the first draft.
The section *Challenge and corrections* lists what changed and why. Every
factual claim below was re-verified against the code after that challenge.

## Summary

- **The foundations are sound.** Immutable facts, a correction chain with one
  successor, explicit long/short sides, explicit basis knowledge, and replay
  that reuses the forward disposal and corporate-action arithmetic. No ledger
  corruption was found.
- **The demonstrated weakness is incomplete effective reads.** Each change
  family (disposals, splits, returns of capital, transfer links) stores its own
  revisions. Each reader removes superseded facts through a shared view and
  then adds every family's revisions back **by hand**. One reader forgot the
  disposal revisions: F1, a confirmed defect that every existing self-check
  passes.
- **The recommended direction is narrow:**
  - fix F1 with tests of the visible warning, not only the quantity;
  - give *committed effective movements* one precisely specified owner;
  - add an independent behavioural oracle, so passing self-checks means more
    than "replay agrees with replay";
  - tighten the number CHECKs where a real test motivates it.
- **The larger restructurings stay conditional.** Replay-only writes, a merged
  revision table and typed replay intents each need a demonstrated failure and
  an executable test before they start.

The test for every proposal is: *which demonstrated failure does it prevent,
and which executable test proves the prevention?* File counts and struct size
describe maintenance cost. On their own they justify nothing.

## Scale

| Measure | Value |
| --- | --- |
| Baseline schema | 1 migration, 4,676 lines, 231 KB |
| Schema objects | 104 tables (35 `investment_*`), 142 triggers, 107 indexes, 14 views, 443 CHECKs |
| Investment production Go | 27,288 of 100,355 backend lines (27%), 41 `internal/db/investment*` files |
| Investment test Go | 26,617 lines |
| Commits since 2026-09-01 | 288, of which 48 are `fix` (20 of them `fix(investments)`) |

The triggers are not the problem:
- 79 are append-only guards (`_no_update` / `_no_delete`) and 21 are same-book
  guards.
- The 21 `_valid` triggers are at most 32 lines each.
- They enforce the financial invariants the early architecture decisions
  endorse.

## What is right and should stay

- **Immutable operations with one correction successor.**
  `investment_operations.correction_of_operation_id` is `UNIQUE`, and a
  correction names an earlier ID. Every chain has one effective end, and
  `effective_investment_operations` is a two-line view.
- **Original facts stay as evidence.** Lots, lot events, decisions,
  allocations, links and their first committed values are never edited.
- **Explicit `basis_knowledge` with paired NULL amounts** (unknown is never
  zero), and an **explicit `position_side`**.
- **Shared arithmetic between forward commands and replay.**
  - Replay disposes through the same `disposeLotsWithAuditTx` as a sale
    (`investment_replay_simulation.go`).
  - Every direct `investment_lot_state` writer is also the function replay
    calls: `applySplitEffectsTx`, `applyCapitalReturnEffectsTx`,
    `spinOffLotTx`, `realizeCashStockMergerLotTx`, `disposeLotTx`,
    `pooledTransferOutTx`, and average cost through `disposeLotsWithAuditTx`.
  - Replay re-creates only the state rows of openings; the immutable lot rows
    are not rewritten.
- **Replay preserves committed elections.** It recomputes a disposal's
  allocation from the original method or specific-lot election. It never
  treats the latest derived allocation as an input.
- **Revision rows.** They record which result was effective when. Gain-change
  acknowledgement, dated bridge and split adjustment journals, and the audit of
  superseded results depend on them.
- **Family-specific revision tables.** Their subjects differ: a disposal
  decision, a split per cost currency, a transfer link, a return-of-capital
  operation. Dedicated foreign keys encode those relationships (see P6).

## Findings

### F1 — Confirmed defect: a revised cover disappears from the named short quantity

`shortLotQuantityEvents` (`internal/db/investment_short_positions.go`) folds
`effective_investment_lot_events` for short lots. That view **excludes** the
original disposal events of any decision that has a replay revision. This
reader never adds back the revision allocations. Realized gains and the
self-check lot reconciliation do.

Reproduction (temporary test, removed; reproduced independently by the
challenge):

1. Open a short of 10 on 2026-03-02.
2. Cover 4 on 2026-05-01 with LIFO. The named short is **6**.
3. Enter a backdated short of 5 on 2026-04-01 and acknowledge it. The May
   cover is revised (one `investment_disposal_revisions` row).
4. The named short now reads **15**. The correct value is **11**
   (10 + 5 − 4).

So a holding balance of −12 is treated as a fully explained short, although 1
unit is unexplained. All four existing investment self-checks still pass,
including replay equivalence: stored projections agree with replay. The error
is in how this reader interprets them.

Impact:
- **Warnings.** Net worth, the net-worth series and the
  `commodity_position_sign` self-check use this quantity. Up to the forgotten
  cover quantity of genuinely unmatched negative units is silently classified
  as short. The product requirements demand a warning ("an ordinary negative
  holding must not silently acquire that meaning").
- **Reported totals.** The net-worth series withholds a converted total for a
  bucket with an unclassified short (`ledger.go`, `NetWorthSeries`:
  `len(bucketResult.UnclassifiedShorts) == 0`). Hiding the warning therefore
  shows a converted total that should be withheld.
- Posted journal amounts are unchanged.

`requirePositionSideAvailableTx` (`investment_position_side.go`) reads the same
view and also ignores revised cover dates. A probe did not produce a wrong
admission: the open-lot branch caught it. It is the same latent pattern.

### F2 — The effective history is assembled by hand in every reader

`effective_investment_lot_events` defines "effective" by **removing** events.
Each family has its own `NOT EXISTS` clause, five in total:
- superseded operations;
- revised disposals;
- revised splits (per cost currency);
- revised returns of capital;
- revised transfer links, including the spin-off parent's `basis_reduction`.

The view never adds the replacements back. Each consumer must union the
`latest_*` revision rows for every family it cares about:

| Reader | Disposal revisions | Split revisions | Capital return revisions | Transfer link revisions |
| --- | --- | --- | --- | --- |
| Self-check lot reconciliation (`SelfCheckLotEvents`) | added | added | added | added |
| Realized gains (`ListRealizedGains`) | added | n/a | n/a | n/a |
| Named short quantity (F1) | **missing** | n/a | n/a | n/a |
| Position-side guard | **missing** | n/a | n/a | n/a |

The same class was fixed before, for example `7f16fe19 fix(investments):
exclude corrected disposals from current reads`. Nothing checks that a new
family or a new reader handles every clause and every union.

Readers do not all need the same thing. Committed effective movements,
replay's economic inputs, original and superseded audit evidence, and future
ADR 0012 reporting profiles are different contracts (see P2).

### F3 — Forward commands and replay orchestrate shared arithmetic differently

*Corrected after the challenge.* The first draft said the lot projection had
"two writers" with separate formulas. That is wrong. The arithmetic is shared
(see *What is right*). What differs is the orchestration:
- A forward command applies its effects against current state.
- Replay rebuilds the affected positions from effective intents.
- A preview runs either one in a rolled-back savepoint.

The cited fixes are orchestration fixes, not arithmetic drift:

- `a19b1344` — replay plain buys during impact preview;
- `4cf39833` — validate buy replacement replay in previews;
- `9381e37b` — replay reinvestment previews.

The concern that remains is narrower: deciding *whether* a command must replay,
and making preview and commit follow the same decision. No example was found of
the two paths computing different amounts from the same inputs. Until one is
found, this justifies tests of the replay decision, not a rewrite (P5).

### F4 — Revision headers look alike, but their subjects differ

Disposals, splits, returns of capital and transfer links each have a revision
header (`revision_seq >= 2`, `supersedes_revision_id`, `caused_by_operation_id`,
an audit event), a chain-validation trigger and a `latest_*` view.

The similarity is real, but it is not proof of accidental duplication:
- **Different subjects:** a decision; a split per cost currency; an operation
  plus link sequence; an operation.
- **Real constraints:** dedicated foreign keys enforce each subject.
- **Necessary differences:** only split revisions carry an adjustment journal,
  and only transfer-link revisions carry original-date knowledge.

The maintenance cost is in the *readers* that walk four families (F2), which P2
addresses, not in the tables.

### F5 — Operation kinds are special-cased across generic code

Operation kinds are referenced by string in generic code:

| Kind | Files that mention it |
| --- | --- |
| `spin_off` | 17 |
| `cash_and_stock_merger` | 13 |
| `cash_in_lieu` | 11 |
| `share_exchange` | 10 |
| `return_of_capital` | 6 |

The replay input `InvestmentReplayIntent` has about 45 fields, mostly specific
to one family, plus "subject" flags for proposed rather than recorded inputs.
Operation kinds and replay intent kinds are different axes: several operations
share opening/disposal intents, and spin-offs and share exchanges reuse
transfer-link intents.

This is a maintenance cost, not a demonstrated failure. It matters only when a
supported kind lacks exercised behaviour for replay, correction, reversal,
export or self-check. That is a test question (P3), not a registry question.

### F6 — Number columns are validated unevenly

The 48 exact-number columns are `TEXT` coefficients with a separate scale. Their
CHECKs vary:

| CHECK on the coefficient column | Columns |
| --- | --- |
| length only | at least 25 |
| digits only, or a sign rule | the rest, in varying combinations |
| digits, no sign, no leading zero | a few (for example `entitled_quantity_value`) |

Examples:
- `investment_disposal_allocations.quantity_value` accepts any 1–38 characters,
  such as `'abc'`, `'-5'` or `'007'`.
- Capital-return revision effects reject `'abc'` and `'-5'` but still accept
  `'007'` in `allocated_value`, `reduction_value` and `excess_value`.

Domain legality also differs legitimately, and a tightening must keep that:
- some columns are signed and others strictly positive;
- some allow zero, such as merger cash allocations, which have zero quantity;
- known/unknown basis needs paired NULLs.

Application code writes canonical values through `exact.Coefficient`, so
nothing is wrong today. The database's last line of defence is inconsistent.

### F7 — Documentation drift

- `docs/product-requirements.md` still says a trade dated before a recorded sale
  is refused, and that replay "is a separate, later capability". Dated replay
  ships for supported operations (#139, #175, #167). ADR 0013 decision 5
  anticipated this.
- `InvestmentReplayEquivalence` (`self_check_replay.go`) is documented as
  replaying "every long position". It groups by `position_side` and covers
  shorts.

## Proposals

Tracked as [#188](https://github.com/sergeyfarin/rekenraam/issues/188)–[#193](https://github.com/sergeyfarin/rekenraam/issues/193).
`docs/roadmap.md` decides order: race-gate headroom (#164) already goes before
the next investment slice.

**P1 ([#188](https://github.com/sergeyfarin/rekenraam/issues/188)) — Fix F1 and prove the visible warning.**
- Keep the 15-versus-11 regression.
- Add an extra unexplained negative unit, and assert that both the
  `commodity_position_sign` self-check and net worth report it.
- Assert that the net-worth series withholds the converted total.
- Cover dates before and after the cover, repeated revisions, and replacement
  and reversal of the cover and of the backdated opening.
- Apply the same fix to `requirePositionSideAvailableTx`.

*Prevents:* F1. *Size:* S–M.

**P2 ([#189](https://github.com/sergeyfarin/rekenraam/issues/189)) — One owner for committed effective movements, narrowly specified.**
- Before choosing a view or a Go function, write the contract: per-lot
  movements under the committed operational elections, with event date,
  correction-root slot, effect sequence, side, cost currency, basis knowledge
  and provenance.
- Explicitly cover zero-quantity merger disposals, pooled transfers (null
  source lot, revised depletions) and spin-off basis reductions.
- It serves current-state and dated readers: named short quantity, the side
  guard, realized gains and self-check reconciliation.
- It does **not** serve:
  - replay inputs, which must keep the original elections and recompute
    allocations;
  - audit and export of original and superseded evidence, which read the base
    tables;
  - future ADR 0012 reporting profiles.
- A source-scanning test keeps direct `effective_investment_lot_events` and
  `latest_*_revisions` reads inside the owner and the named audit and replay
  readers.
- This reduces the risk of omissions. It does not remove the bug class: the
  owner itself can be incomplete, which P4 guards against.

*Prevents:* F1-style omissions in current and future readers. *Size:* M.

**P3 ([#190](https://github.com/sergeyfarin/rekenraam/issues/190)) — Independent behavioural oracle.**
- Add conservation checks whose expectations do not come from the replay
  engine:
  - dated named short and long quantities against journal security quantities;
  - basis conservation across transfers, splits, spin-offs, mergers and
    returns of capital;
  - preservation of unknown basis.
- Run them over combinations of corrections and revisions across families
  (for example, a backdated opening plus a cover replacement plus a transfer),
  not one family at a time.
- As commands and verification share more of one engine, agreement between
  them is weaker evidence of correctness. This is the counterweight.

*Prevents:* errors where replay and storage agree but a reader or the engine is
wrong. F1 would have been caught. *Size:* M.

**P4 ([#191](https://github.com/sergeyfarin/rekenraam/issues/191)) — Capability matrix that drives real tests (replaces the "registry").**
- For each supported operation kind, either exercise the behaviour in a test or
  assert an explicit named refusal, for replay, correction, reversal, export
  (check the exported contents) and self-check.
- A declaration that says "export supported" proves nothing unless a test runs
  the export.
- Keep operation kinds and replay intent families as separate axes.

*Prevents:* a supported kind silently missing a behaviour. *Size:* M.

**P5 ([#192](https://github.com/sergeyfarin/rekenraam/issues/192)) — Uniform number CHECKs, where a test motivates them.**
- Separate canonical representation (digits, optional `-`, no leading zero, no
  `-0`) from domain legality (signed, positive, zero allowed, NULL pairs) per
  column. Change the baseline before v0.1.0 (`BREAKING DEV DATABASE`).
- Acceptance: signed and unsigned 38-digit boundaries, `-0`, leading zeros,
  malformed text, valid zeros and NULL pairs, the merger zero-quantity
  allocation, the updated schema checksum and the seeded-schema checks.

*Prevents:* non-canonical values that would bypass `exact.Coefficient`.
*Size:* M, not S.

**P6 ([#193](https://github.com/sergeyfarin/rekenraam/issues/193)) — Fix the documentation.**
- Product requirements: state what is supported now. Dated replay admits
  supported backdated operations. A change replay cannot satisfy is refused
  with the blocking operation named.
- Correct the replay-equivalence comment.

*Size:* XS.

**Conditional, not scheduled:**

- **Replay-only projection writes** (was P5). Only if F3 turns up a real
  amount divergence, and only after a workload benchmark: long-lived positions,
  large imports, backdated corrections, previews, closure size and SQLite
  write-lock duration. Replaying a growing history after every imported command
  can make aggregate import work quadratic. "Single-user" does not bound
  history.
- **One revision header** (was P6). Only if, after P2, a concrete benefit is
  shown that keeps constraints as strong as today's foreign keys. A generic
  `(subject_type, subject_id)` header would lose them. Consolidate reader
  behaviour and shared validation helpers first.
- **Typed replay intents** (was P7). If pursued, use typed payloads per
  replay-intent family, with proposed and recorded inputs kept distinct. An
  interface alone does not make handling exhaustive. The P4 tests are what
  prove completeness.

## Rejected

- **Dropping the trigger layer for simplicity.** The triggers are the cheap,
  uniform part, and they protect invariants against application bugs.
- **Replacing stored revisions with replay on every read.** It would lose which
  result was effective when, which acknowledgement, adjustment journals and
  audits use.
- **Whole-book rebuild per command.** Already rejected in ADR 0013 (T-124).
- **A schema rewrite in one change.** Against the working rules.
- **A blanket freeze on investment work.** At most, pause *new operation kinds*
  until P1 and P2 land. Work on existing kinds, such as zero-delta splits
  (#146), continues in roadmap order.

## Challenge and corrections

An independent challenge of the first draft was reviewed point by point against
the code.

| Challenge | Verified | Disposition |
| --- | --- | --- |
| F1 reproduces; all four investment self-checks pass | Yes | Recorded in F1 |
| F1 is broader than diagnostic: the net-worth series withholds converted totals when a short is unclassified | Yes, `NetWorthSeries` in `ledger.go` | F1 impact corrected; P1 tests the warning and the totals |
| F3: replay already shares disposal, split, capital-return, spin-off and merger arithmetic | Yes, all six direct writers are called by replay simulation | F3 rewritten; replay-only writes made conditional |
| P2 must distinguish committed movements, replay inputs, audit evidence and reporting profiles | Agreed; replay keeps elections by design | P2 narrowed |
| Replay equivalence cannot catch reader errors; an independent oracle is missing | Yes, it passed with F1 present | New P3 |
| A registry would drift; it needs executed tests | Agreed | P4 is a test-driven capability matrix |
| Revision headers differ by subject and foreign keys | Yes | F4 reframed; merging made conditional |
| Number CHECKs: domain rules differ; capital-return `allocated_value` accepts `'007'` | Yes (schema `investment_capital_return_revision_effects`) | F6 corrected; P5 resized to M |
| `SelfCheckLotEvents` already adds capital-return revisions | Yes | F2 table corrected |
| Replay-only writes need a workload benchmark; a freeze needs an exit and must not override the roadmap | Agreed; roadmap places #164 before the next slice | Conditional proposal; freeze limited to new kinds |

## Recommended sequence

1. P1, with visible-warning coverage.
2. P6, with precise supported-backdating language.
3. P2 as specified, together with P3.
4. P4 and P5, each motivated by a failing or missing executable test.
5. The conditional items only on new evidence.

## Questions for the owner

1. Should new *operation kinds* (not existing-kind work) pause until P1 and P2
   land?
2. Should the core ledger (transactions, versions, postings) and imports get
   the same review?
