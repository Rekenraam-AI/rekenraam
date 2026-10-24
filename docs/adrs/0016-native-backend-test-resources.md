# ADR 0016: Native Backend Test Resources

## Status

Accepted (2026-10-12); amended 2026-10-24 by #402 (monitoring thresholds and
escalation order); remedy 1 measured and closed by #404 the same day. Supersedes ADR 0015 decision 5's retained serial-package
schedule and 20-minute deadline. FULL durability and native-driver admission
remain unchanged.

## Context

The owner clarified that the post-migration review should update the old test
resource limits. The modernc-era 20-minute deadline and serial-package schedule
were responses to translated-engine race costs. Those costs are gone.

With native SQLite and FULL, three four-CPU serial-package runs put application
race execution at 109.964–114.740 seconds. The corresponding CI run takes
56.508 seconds. A 20-minute package deadline now delays detection of hangs and
large regressions unnecessarily. The earlier four-to-six-core scaling advice
also described modernc, not the native implementation.

## Decision

1. The shared local/CI backend wrapper uses a **five-minute per-package
   deadline** for both race and ordinary coverage tests. All financial,
   migration, recovery and concurrency assertions remain; no faster test-only
   database setting is introduced.
2. Run **two packages concurrently** (`-p 2`) in both modes. Tests inside each
   package retain their isolated fixtures and ordinary Go parallelism. A
   matched native scheduling comparison must retain at least 30% application
   headroom under the new 300-second bound before this schedule is admitted.
3. Do not set a fixed `GOMAXPROCS`, test `-parallel` value or CPU affinity in
   normal validation. Use available Go CPU resources. Four CPUs describe the
   controlled scheduling comparison and historical driver comparison, not a
   minimum deployment requirement or a normal development cap. Retire the
   modernc-specific recommendation to restrict test concurrency to four cores.
4. Let CI's integrated-build job run independently of backend and frontend
   checks. It is a separate validation job with its own build prerequisites;
   all CI jobs still have to pass. This removes the serial wait without
   changing validation frequency or publishing any artifact.
5. Keep the source/feature configuration identical to production: bundled
   native SQLite, CGO, FTS5/netgo/osusergo, FULL and all Go race instrumentation.
   The native C engine remains outside Go race coverage.

## Validation and consequences

The [native gate review](../reviews/native-test-gate-2026-10-12.md) records the
matched scheduling runs, case-name digest, resource limits and local/CI results.
Command wall time and package execution are separate metrics: concurrency
shortens the gate while increasing some individual package times.

Five minutes is a failure bound, not an execution target. Future suite growth
or sustained runtime pressure should trigger measured optimization before
raising it. A CPU limit can still be used for a declared diagnostic comparison;
it must not quietly become the default developer command.

## Amendment — race-suite cost thresholds, 2026-10-24

[#402](https://github.com/sergeyfarin/rekenraam/issues/402) measured the
application package at `935ccdb7` on a ten-CPU host. The
[record](../reviews/race-suite-cost-2026-10-24.md) has the matched runs and the
profile. Results:

- Single package: median 225.7 s.
- Full `-p 2` gate: 276.6–277.7 s.
- CI per package: about 183 s.

Migration replay is 4.5% of the package's CPU. Statement compilation is 32.7%,
and per-connection schema loading is 19.4%: every test pays for schema size.
The earlier 200-second trigger did not name a measurement scope, so it is
replaced by these thresholds.

- **Investigate** when either median exceeds **210 s**: three interleaved,
  uncached single-package runs of `./internal/app` on the reference ten-CPU
  host, or CI's per-package time over the latest six successful `main` runs.
- **Act before the next feature slice** when two uncached full-gate runs
  (`GOFLAGS=-count=1 GO_PHASE_TIMINGS=1 scripts/test-backend.sh`) both exceed
  **270 s** (90% of the deadline) for any package.
- Single runs, and CI jobs without a matched base, are observations, not
  triggers. The wrapper does not bypass Go's test cache, so timing runs set
  `-count=1`.

Remedies are taken in this order:

1. Reduce statement compilation cost, measured
   ([#404](https://github.com/sergeyfarin/rekenraam/issues/404)). First reuse
   statements explicitly at the hot production sites, then evaluate the native
   statement cache. Prefer explicit reuse when it recovers most of the gain;
   adopt the global cache only after its correctness review.
2. If the full-gate threshold still fires, choose between reducing schema load
   cost and raising the deadline. Reducing schema load means fewer or smaller
   triggers and views, preferably before v0.1.0 while migrations can still be
   rewritten. A deadline change needs a new amendment with matched evidence.
   Schema slimming is already scheduled as a v0.1.0 release gate
   ([#405](https://github.com/sergeyfarin/rekenraam/issues/405)), after #404.

Do not recover headroom by:

- splitting packages to fit the per-package bound;
- trimming coverage or upgrade paths;
- pooled or reset fixtures;
- faster test-only database settings;
- lowering `-p` or test parallelism.

At `935ccdb7` the single-package and full-gate thresholds have fired; the CI
threshold has not. #404 is the response.

**#404 result (2026-10-24).** Remedy 1 is done. Explicit statement reuse in
five hot loops lowered the single-package median from 223.3 s to 219.4 s and
moved the full-gate pair from 280.2/276.6 s to 255.5/274.1 s. The act
threshold no longer fires, by a thin margin; the investigate threshold still
does. The native statement cache measured −4.4% here, about 2.7% beyond
explicit reuse, and the owner declined it: `_stmt_cache_size=0` stays. Remedy
2, schema slimming in #405, is next. See the
[#404 record](../reviews/statement-compilation-2026-10-24.md).

## References

- [ADR 0014](0014-native-sqlite-driver.md), native driver admission.
- [ADR 0015](0015-sqlite-commit-durability.md), FULL durability.
- [FULL timing evidence](../reviews/sqlite-full-durability-2026-10-12.md).
- [#164](https://github.com/sergeyfarin/rekenraam/issues/164), development feedback.
- [Race-suite cost investigation](../reviews/race-suite-cost-2026-10-24.md), #402 amendment.
- [Statement compilation cost](../reviews/statement-compilation-2026-10-24.md), #404 result.
