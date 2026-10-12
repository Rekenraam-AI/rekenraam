# ADR 0016: Native Backend Test Resources

## Status

Accepted (2026-10-12). Supersedes ADR 0015 decision 5's retained serial-package
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

## References

- [ADR 0014](0014-native-sqlite-driver.md), native driver admission.
- [ADR 0015](0015-sqlite-commit-durability.md), FULL durability.
- [FULL timing evidence](../reviews/sqlite-full-durability-2026-10-12.md).
- [#164](https://github.com/sergeyfarin/rekenraam/issues/164), development feedback.
