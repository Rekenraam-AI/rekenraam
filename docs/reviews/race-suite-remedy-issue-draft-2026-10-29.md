# Proposed GitHub follow-up to #419

Local draft; not filed. Automatic approval review refused GitHub creation
because publishing locally derived profiling results needs explicit user
authorization. No new local issue ID is allocated.

**Title:** Reduce preparation cost of effective investment reads after #419

**Proposed labels:** `go`, `priority: P2`

## Problem and evidence

The [#419 reprofile](race-suite-reprofile-2026-10-29.md) at
`d524eecf4be639494597ea7b2f6fcc6b3969c7fc` has an isolated application race
median of 226.265 s. Both uncached full gates time out in that package at 300 s;
every other package passes. Both ADR 0016 thresholds fire.

Explicit preparation accounts for 469.71 of 1,530.40 sampled CPU-seconds
(30.69%). Preparation beneath these three production read paths accounts for
139.41 CPU-seconds (9.11%), counted as a union:

- `SelfCheckService.investmentFoundationCheck`: 42.27 s.
- `db.EffectiveLotMovements`: 56.49 s.
- `db.investmentReplayIntentsQuery`: 40.65 s.

## Bounded delivery scope

Measure and simplify compilation of these three production read paths, one
query group at a time. First inventory exact SQL texts and compile-versus-execute
timings on populated synthetic books; a call-stack profile does not identify
which textual rewrite is safe. Prefer equivalent query simplification or
call-scoped reuse where measured repetition supports it. Preserve effective
operation selection, long/short and known/unknown basis, corrections/reversals,
transfer lineage, read snapshots and independent self-check/oracle coverage.

Keep bundled native SQLite, FULL, race, `-p 2`, the 300 s deadline and cache 0.
No package splitting, coverage reduction, pooled/reset fixtures or test-only
fast settings. Global cache admission and deadline amendments are separate
owner decisions.

## Expected gain and limits

This is a candidate, not a demonstrated optimization. Halving preparation in
these paths would remove about 69.7 CPU-seconds (4.55% of sampled CPU), roughly
10 s at the isolated median **if** wall time scaled proportionally. Eliminating
the entire measured bucket has a 9.11% CPU ceiling. Neither is an observed wall
gain or a promise to restore 30% full-gate headroom. Timed-out gates do not
establish their uncensored completion time.

## Acceptance

- [ ] Record exact source, SQL inventory, fixture sizes, repeated prepare/execute
  timings and a semantic-equivalence coverage map for each changed query group.
- [ ] Preserve every existing financial assertion and case. Add independent
  populated/damaged/correction controls where a rewrite changes the selection
  shape; reuse needs a compilation-count sensitivity control.
- [ ] Admit only a repeatable improvement: target at least 25% lower median
  preparation time for the changed group. If it cannot be met safely, retain no
  speculative rewrite and record that disposition.
- [ ] Run three interleaved uncached base/proposed application race pairs and
  two uncached full gates per variant with no competing jobs. Retain failures,
  censoring, phase/package timings and case digests.
- [ ] Report actual suite gain and remaining ADR 0016 trigger state. The act
  rule remains unresolved until both current full-gate application runs are
  below 270 s; separately report the <=210 s headroom objective.
- [ ] If this bounded change cannot clear the act trigger, retain that gate and
  record a remaining production-cost option or an explicit owner resource/deadline
  decision as a separate follow-up. No policy change is implied here.

## Dependencies and validation

No provider, credential or upstream-release dependency. #419 supplies evidence;
GitHub filing awaits explicit publication authorization. ADR 0016 puts the
response before the next feature slice; the roadmap owns execution order.

Use the #419 runner/reducer and native wrappers, affected service/repository
tests, independent investment oracles and self-check damage controls, then the
uncached shared race gates and vet/format checks. Suite timing does not establish
production request-latency gains.
