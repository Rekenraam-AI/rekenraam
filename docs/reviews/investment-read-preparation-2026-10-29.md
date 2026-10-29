# Investment read preparation — 2026-10-29

[#445](https://github.com/sergeyfarin/rekenraam/issues/445) follows the
[#419 profile](race-suite-reprofile-2026-10-29.md). One production query changes:
the long-position transfer-depletion read in `investmentReplayIntentsQuery`.
Three alternating native measurements reduce its preparation median by
**26.9–27.6%**. This percentage describes that statement, not the complete
replay path, all three profiled paths, or a production request.

Both proposed full gates pass below 270 seconds (**263.4 / 269.2 s**), meeting
#445's clearance criterion with only 0.8 s to the threshold in the slower run.
Both fresh baseline gates are also below 270 s; this does not establish
that the query change recovered headroom. Investigate and the 30% headroom
objective remain. The
[structured evidence](fixtures/investment-read-preparation-2026-10-29.json)
retains the measurements and checksums.

## Source and preserved policy

Baseline: `e3edbc2d80ea417cd8a80776fd85dfc8e97b2496`. Its backend is identical
to #419's `d524eecf`; intervening commits change documentation. The proposed
variant differs only in `backend/internal/db/investment_replay_intents.go`.
The structured evidence records both source-file inventories, binary hashes,
the proposed patch hash, commands, host and raw-log hashes.

Both variants use Go 1.27.3, bundled go-sqlite3 1.14.52 / SQLite 3.53.4, CGO,
`fts5,netgo,osusergo`, FULL, statement cache 0 and Go race instrumentation.
The reference host exposes ten AMD EPYC 3251 CPUs, with GOMAXPROCS unset and
no test-parallelism or affinity override. Full gates retain `-p 2` and the
300-second per-package deadline. A writable compilation cache in `/tmp` is
warm; test results are uncached. Timing jobs run sequentially; host spot checks
find only the expected test processes and no competing build/test job. This is
local evidence, not a hosted CI measurement.

## Inventory and bounded choice

Temporary capture instrumentation forwards the original queries and records
SQL and arguments while calling the real movement, replay and foundation
readers on service-created synthetic books. It observes **48 distinct SQL
texts**: eight movement texts, ten replay texts including capital helpers,
and 31 foundation structural-anomaly texts. One text is shared between paths.
This inventories the structural checks within the foundation path, not every
auxiliary repository read performed by the whole self-check. Capture code is
absent from both measured application trees and from production.

The initial five capability books supply 321 distinct statement/argument
observations. Preparation is timed separately from execution: five batches
of 100 native driver `Prepare`/`Close` calls; then a single prepared statement
and five batches of 20 executions, consuming every row. The Go race build is
used throughout. Driver/SQLite preparation cost is not an isolated native
parser cost; execution measurements exclude preparation and include row reads.

Some initial medians across fixture/input observations:

| SQL SHA-256 prefix | Reader | Prepare, µs | Execute, ms |
|---|---|---:|---:|
| `8b0f37eb4fa9` | Foundation structural check | 396.61 | 0.0163 |
| `5b10a631d565` | Replay transfer depletion, selected | 294.00 | 0.0197 |
| `42f59dd28af6` | Foundation structural check | 292.95 | 0.0153 |
| `b08ebb3b808a` | Foundation structural check | 291.23 | 0.0287 |
| `0eaabec963b9` | Revised pooled-transfer movements | 213.10 | 0.0197 |
| `213433281cde` | Effective event movements | 202.19 | 0.0463 |

The exact texts, arguments and all five batches are retained in the evidence.
These inputs establish compilation-heavy reads, not representative production
request latency. Within an ordinary replay call these statements already
compile once each; an additional call-scoped statement container would not
remove those preparations. Broad INNER-to-CROSS experiments on the inventory
did not justify a general rewrite. Only the selected statement's bounded
planning constraint is retained.

## Equivalent query and preparation result

The selected query originally joins transfer facts to links, effective
operations, then source lots. It now joins facts to effective operations,
then uses `CROSS JOIN ... ON` for links and their source lots. In SQLite,
[CROSS JOIN constrains join ordering](https://sqlite.org/optoverview.html#manual_control_of_query_plans_using_cross_join).
The effective-operation lookup therefore precedes link/source-lot lookups;
other inner joins retain planning freedom. No selection predicate, bound
argument, projection, revision choice, LEFT JOIN or result ordering changes.
Adjacent inner joins have the same relational meaning under these constraints.
No statement survives a call or crosses a read snapshot.

An exploratory constraint with links before effective-operation selection was
rejected: it could repeat retirement selection per link. The retained ordering
matches the original physical plan, including indexes, on all **14 long-position
inputs** across eight books. The reversed fifty-lot book checks that retired
operations still stop before link expansion. This is evidence on these books,
not a guarantee of the optimal plan on every future distribution or SQLite
version. Revisit the hint when upgrading the native engine.

| Rotation | Original median prepare, µs | Proposed, µs | Reduction |
|---|---:|---:|---:|
| Base then proposed | 298.35 | 215.90 | 27.64% |
| Proposed then base | 298.92 | 217.00 | 27.40% |
| Base then proposed | 296.76 | 217.08 | 26.85% |

Each table entry is the median of per-input five-batch medians. Every input
has identical row count and ordered row-value SHA-256 between variants, in
every rotation. Both zero-row and populated reads are included. Execution
timings and unchanged EXPLAIN QUERY PLAN details are retained separately;
the admission claim is reduced preparation, not an execution speedup.

## Fixture sizes and semantic coverage

Fixtures use actual application commands and preserve immutable facts and
revision evidence. No owner data or network provider is involved.

| Book | Operations | Lots | Lot events | Disposal / link revisions | Postings |
|---|---:|---:|---:|---:|---:|
| Trades and income | 8 | 2 | 6 | 0 / 0 | 22 |
| Corporate actions | 5 | 3 | 7 | 0 / 0 | 16 |
| Cash and stock merger | 2 | 2 | 4 | 0 / 0 | 10 |
| Transfers, including unknown basis | 5 | 3 | 5 | 0 / 0 | 14 |
| Shorts | 3 | 1 | 2 | 0 / 0 | 10 |
| Corrected transfer and dependent sale | 5 | 4 | 6 | 2 / 2 | 26 |
| Fifty-lot transfer | 51 | 100 | 150 | 0 / 0 | 202 |
| Reversed fifty-lot transfer | 52 | 100 | 150 | 0 / 0 | 204 |

The short book has no long-transfer query; its existing reader behavior is
preserved. The correction book replaces the originating buy twice after an
internal transfer and dependent disposal. The dense active/reversed pair
exercises 50 links and retired-operation exclusion. Row-hash comparisons are
supplementary equivalence controls; the existing independent financial and
damage assertions remain the semantic authority.

| Property retained | Existing coverage |
|---|---|
| Effective originals, replacements and same-day order | `TestEffectiveMovementsOriginalRevisedAndCorrectedSameDayOrder`, `TestEffectiveMovementsPooledRevisionExpandsDepletionsBeforeSameDayDestination` |
| Transfer reversal, revised evidence and no-write refusals | `TestReverseInternalTransferRestoresSourceAndRemovesDestinationLot`, `TestReverseInternalTransferRefusedWhenDestinationSoldWithoutWriting`, `TestReverseRevisedInternalTransferKeepsRevisionAsEvidence` |
| Source-lot / pooled lineage and deep propagation | `TestInvestmentAuditTransferCorporateActionChain`, `TestReversePooledLotTransferRestoresPoolBasisExactly`, `TestTransferChainDeeperThanFormerRoundLimitPropagatesInOnePass` |
| Exchange, spin-off and merger corrections | `TestShareExchangeReplacementRestatesALaterGainWithAcknowledgement`, `TestSpinOffReversalRestoresParentBasisAndRetiresTheNewLots`, `TestCashStockMergerReplacementRestatesTheCashResultWithAcknowledgement` |
| Known/unknown basis and independent conservation | `TestShareExchangeReadsKeepUnknownBasisUnknownAndShowRevisedBasis`, `TestOracleUnknownInboundThroughTransferSaleAndResolution`, `TestOracleShortCoverRevisionsBesideARevisedTransferAndSale` |
| Independent damaged-ledger detection | `TestLotSelfCheckReportsMissingAndCorruptState`, `TestSelfCheckDetectsOffsettingDisposalProceedsDamage`, `TestSelfCheckFindsInvestmentOperationWithoutPostedVersionLink`, `TestInvestmentDiagnosticAuditDamageBesideUnknownBasis` |

No test source, assertion, fixture schedule, snapshot boundary, financial write
or schema changes. The focused race selection covering movements, transfer
reversal/correction, corporate actions, audit lineage and oracles passes in
22.052 s. A physical planning constraint introduces no new financial selection
rule; no implementation-mirroring test is added to the ordinary suite.

## Matched application and full gates

| Source | Isolated application seconds | Median, s | Leaf cases |
|---|---|---:|---:|
| Original | 180.355, 215.457, 228.126 | 215.457 | 2,476 |
| Proposed | 208.769, 221.865, 227.173 | 221.865 | 2,476 |

All six runs pass, each with 2,472 passing leaf cases and the same four opt-in
measurement skips. The unchanged case-name digest is
`7d5ee6c814b12f0c286f9b689d6a8f0c5f55031ce9cd77219a831f4f8287e157`.
All backend test sources are byte-identical. Leaf reduction excludes every
ancestor, including slash-containing subtest names, as in #419.

The proposed median is **3.0% higher**, not a demonstrated suite improvement.
Same-source ranges are 47.8 s original and 18.4 s proposed; the third pair is
228.1 / 227.2 s. The first original run is unusually fast, as happened in
#419. These runs do not isolate a causal suite regression either. The admitted
claim is the repeatable reduction in this statement's preparation, with
unchanged observed execution plans and results. The whole-suite investigate
threshold still fires. Comparing either source directly to #419's historical
226.3 s median would not establish a gain from this change.

| Full gate | Application, s | API, s | Database, s | Race phase wall, s | Command wall, s | Result |
|---|---:|---:|---:|---:|---:|---|
| Original 1 | 254.841 | 145.297 | 96.986 | 326.08 | 378.824 | Pass |
| Proposed 1 | 263.387 | 165.208 | 101.997 | 330.12 | 386.505 | Pass |
| Proposed 2 | 269.197 | 174.149 | 99.586 | 344.03 | 351.156 | Pass |
| Original 2 | 260.626 | 154.665 | 93.866 | 335.71 | 343.026 | Pass |

Every package passes in all four gates; no failure, race warning or censored
timeout occurs. Vet and formatting pass. The first gates compile each variant's
dependent packages; later gates use a warm compilation cache. Compilation is
separate from the execution deadline:

| Phase wall seconds | Original 1 | Proposed 1 | Proposed 2 | Original 2 |
|---|---:|---:|---:|---:|
| Modules | 0.01 | 0.01 | 0.01 | 0.01 |
| Vet | 17.27 | 20.07 | 0.55 | 0.53 |
| Race compile/vet | 34.98 | 35.83 | 6.10 | 6.27 |
| Race execution | 326.08 | 330.12 | 344.03 | 335.71 |

The evidence also retains phase and command user/system times, package
statuses, commands, log hashes and unchanged source/case inventories. Both
fresh baseline gates pass below 270 s as well. The proposed application gate
times are 8.5 / 8.6 s higher than their baseline counterparts. This comparison
does not demonstrate a suite gain, and neither the nominal threshold clearance
nor the contrast with #419's historical timeouts is attributed to the hint.

Both proposed application times are below 270 s, meeting the issue's explicit
action-clearance criterion on this pair. The slower run has only **0.803 s** to
that threshold and **30.803 s** to the 300-second failure bound. Neither run
meets the <=210-second 30% headroom objective; investigate still fires at the
221.865-second isolated median. Ordinary ADR 0016 monitoring remains in force.
The conditional follow-up for an uncleared act trigger is not invoked because
the required pair is below 270 s. Recurrence requires a fresh response; this
record does not authorize a cache, resource or deadline policy change.

## Disposition and validation

Retain the equivalent planning constraint: it meets the per-statement
preparation target in every native rotation, with unchanged observed plans,
results and financial assertions. Do not describe it as a whole-suite or
production-request speedup. This completes #445's bounded delivery scope.

Validation: the focused race selection, all six uncached isolated race runs,
and all four uncached full backend gates pass. The reducer checks SQL/source
identity, row equality, all case sets, plan equality and unchanged SQLite
snapshots. The retained input generator reproduces every accepted rotation
exactly; the probe patch applies cleanly to the baseline. Hosted CI is pending
publication and is not claimed here. No frontend, API contract, schema or
whole-book reader implementation changes.

## Reproduction

The [matched runner](fixtures/investment-read-preparation-matched.py) accepts
clean baseline/proposed archives and an empty output directory. It compiles
both race binaries, runs three isolated pairs (reversing the middle pair),
then two full gates per variant (reversing the second pair). Gate failures
are retained and do not stop collection. It returns failure if any run fails.
Full gates use `GOFLAGS=-count=1 GO_PHASE_TIMINGS=1`; isolated binaries use
`-test.count=1 -test.timeout=5m` and the complete default test selection.

For the native probe, apply the
[capture/measurement patch](fixtures/investment-read-preparation-probe.patch)
to a separate baseline archive using `patch -p1` (or `git apply --unidiff-zero`).
Run only `TestCapture445Queries` with
`CAPTURE445_OUTPUT` pointing to an empty directory. Its capture state is
deliberately private to this one diagnostic test; do not run other tests in
that archive concurrently. Compile `./cmd/measure445` and `./cmd/plan445` with
the native wrapper and `-race` (`-buildvcs=false` for these archive commands).
The measure command accepts a capture directory and output JSON path. Name the
eight-book capture directory `final-evidence`; copy the first five books'
SQLite/query/info files into `evidence` and measure that directory into
`baseline-sql.json` for the initial inventory. Then use the
[input generator](fixtures/investment-read-preparation-inputs.py) with the
eight-book capture directory and proposed Go file to construct three fresh
`accepted-query-*` directories under the raw root. Measure each sequentially,
writing its JSON beside it. The plan command accepts one such directory and
writes JSON to stdout; retain that as `final-plans.json`. Use `matched` under
the same raw root for the matched runner output and retain `host.json` with
the exact baseline SHA, patch hash and declared conditions. Keep all
probe/build work separate from suite timing.

The [reducer](fixtures/investment-read-preparation-results.py) accepts the raw
directory and output JSON; it reuses #419's leaf-case reduction, checks exact
base/proposed source differences and validates row/plan equality. Raw logs,
SQLite snapshots and binaries remain under `/tmp/rekenraam-445`; committed
evidence keeps hashes, counts, exact SQL, batches, plans and outcomes. Fresh
fixture IDs/audit timestamps can differ when regenerating synthetic books;
compare both variants against the same regenerated snapshots.
