# Native backend test gate review — 2026-10-12

The owner clarified that the post-migration review should update the old test
limits, specifically the four-core tuning advice and 20-minute budget.
[ADR 0016](../adrs/0016-native-backend-test-resources.md) governs this follow-up.
The native driver and FULL policy delivered in `6b6ff543` remain unchanged.

## Updated defaults

| Condition | Previous policy | Current policy |
| --- | --- | --- |
| Race package deadline | 20 minutes | 5 minutes |
| Ordinary coverage deadline | Go default, 10 minutes | Explicit 5 minutes |
| Concurrent packages | 1 | 2 |
| Within-package CPU resources | Wrapper had no fixed cap; old modernc advice favored four cores | Available Go CPU resources; retire the modernc-specific four-core recommendation |
| Controlled comparison | Four CPUs for matched driver/FULL runs | Still four CPUs when declaring that diagnostic comparison, with no effect on default developer commands |
| Integrated-build CI prerequisites | Wait for backend and frontend checks | Independent job; every check remains |

The wrapper does not set GOMAXPROCS, `-parallel` or CPU affinity. The local
caller has GOMAXPROCS unset on a ten-logical-CPU VM; Go can honor its normal
CPU/quota behavior. The 4-CPU comparison below is deliberately conservative
and reproducible, not a prescribed normal test budget.

## Scheduling evidence

Three uncached native/FULL race runs use two concurrent packages and a
five-minute deadline, on the same host/toolchain, affinity 0–3 and GOMAXPROCS=4
as the prior serial-package FULL runs. Every historical case and assertion is
retained: **2,834 names**, digest
`85f28b35c41dce481495a6a6337c2fabfa7a52fb1825c8311c3e6731367bc629`.
The timing cohort excludes `TestNative*` only to match the original comparison;
the updated shared correctness gate includes them.

| Measurement, seconds | Serial-package FULL median (range) | Two-package FULL median (range) |
| --- | --- | --- |
| Full command wall | 256.51 (254.95–303.42) | 156.63 (155.41–162.61) |
| Application execution | 113.309 (109.964–114.740) | 140.454 (139.764–145.119) |
| API execution | 52.965 (52.688–54.582) | 65.895 (65.828–68.890) |
| Database execution | 50.004 (49.447–53.643) | 68.637 (66.739–70.996) |

Median command wall decreases **38.9%**. Individual packages take longer as
they share resources, but the worst application run retains **51.6% headroom**
under the new 300-second deadline, exceeding the retained 30% margin criterion.
The prior CI serial-package application run at `6b6ff543` took 56.508 seconds;
its ordinary coverage gate reported 79.6% total coverage. A 20-minute deadline
is no longer needed for these native workloads.

This is a shared VM, with the serial series measured earlier the same day.
Keep all runs and ranges; do not promise this percentage on every machine.
`time`'s maximum child RSS is not the combined memory peak of concurrent
packages. The [JSON evidence](native-test-gate-2026-10-12.json) retains each
run, package times, skipped cases, source hashes and validation results.

## CI and validation

The original integrated-build job at `6b6ff543` started at 12:48:28 UTC after
the backend job completed at 12:48:25. It finished at 12:50:51, while browser
smoke finished at 12:49:07. Removing the job prerequisite removes that serial
wait; it does not skip backend/frontend checks or publish an artifact. Cache
and runner variability prevent treating the old interval as a guaranteed saving.

Both modes of `scripts/test-backend.sh`, CI comments, README, conventions,
developer workflow and backend/validation skills now state the same concurrency
and five-minute deadline. All Go race instrumentation and the production
native/FULL configuration remain. No product runtime, financial schema, test
assertion, fixture storage mode or test-driver choice changes.

The updated shared race gate passes formatting, vet and every native/financial
case with normal host CPU resources in **136.70 seconds**. Application execution
is 117.373 s, API 55.707 s and database 58.514 s. No CPU override is set.
The updated ordinary coverage gate passes in **122.03 seconds** and retains
**79.6%** total coverage. Both use the actual shared wrapper's two-package
schedule and five-minute deadlines; all native cases are included.

Shell syntax, active documentation references, measured source hashes and CI
job structure pass. All five CI jobs remain; the integrated build has no
`needs` prerequisite. The native/FULL product runtime was already validated
and pushed in `6b6ff543`; this follow-up changes only test/CI resources and
guidance. Remote CI is pending at commit time and recorded on #164/#195.
