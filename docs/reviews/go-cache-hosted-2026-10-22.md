# Hosted Go cache validation (#382)

Date: 2026-10-22. This supplements the [implementation and local validation](go-cache-2026-10-22.md).
The [JSON evidence](go-cache-hosted-2026-10-22.json) records every sampled job's
source, runner, native configuration, primary/restored/saved keys, effective
paths, phase wall/CPU timings, transfer/save overhead and cache inventory.

## Sources and measurement

Baseline: `7d5d3cdf1f7ca9ecc57a8c75ffcd93786712fbc6`,
[run 39731690708](https://github.com/sergeyfarin/rekenraam/actions/runs/39731690708).
[Attempts 1](https://github.com/sergeyfarin/rekenraam/actions/runs/39731690708/attempts/1),
[2](https://github.com/sergeyfarin/rekenraam/actions/runs/39731690708/attempts/2)
and [3](https://github.com/sergeyfarin/rekenraam/actions/runs/39731690708/attempts/3)
each pass all six jobs (five logical gates, including the
two browser matrix jobs). Attempt 4 reruns only coverage successfully to get
a second warm coverage sample on the matching image. It is not another full
workflow measurement. Every actual invocation uses a fresh GitHub-hosted runner.
Inherited jobs in attempt 4 are excluded from the measurements.

GitHub rolled out runner images during the observations. Image A is
`20261012.334.1`; image B is `20261018.339.1`. Keys intentionally isolate these
native environments. Comparisons below use the same source and image within
each row, rather than treating a different-image miss as a failed warm cache.
Both use Go 1.27.3 linux/amd64, GOAMD64=v1, GCC 13.3.0
(`Ubuntu 13.3.0-6ubuntu2~24.04.1`), glibc 2.39, CGO=1, `-O2 -g` and the
unchanged `fts5,netgo,osusergo` tags. Logged Go environment fields, compiler/libc
versions and image identifiers are in the JSON.

The helper, restore/save action and real E2E child build agree on:

```text
GOCACHE=/home/runner/.cache/go-build
GOMODCACHE=/home/runner/go/pkg/mod
```

The baseline source digest is
`1ecafd43dce867a81a639dd6d8c053405fc6bb931f3fb5260c4c7adbcecf55db`.
The dependency digest remains
`6947d5845a3d03145716cc3021c5fc38b6ed2dc9861e0eacc6079fd4bcc89366`
throughout. Native key digests are
`846f5e461e97f03441697bdd5d53b16a7817cc6e679adf36c65e161211383909`
(A) and `d50c165fc0aab1aa55dd6741a45d2bb7216406e972bf3fedede1a042c5b2b85c`
(B). The full keys in JSON are the values logged by Actions, not reconstructed
claims about a setup-go cache.

## Cold and repeated warm compilation

Times are seconds. Compile/vet sums explicit vet plus the compile/link/default
vet phase for backend tests; ordinary builds sum embed compile/vet and the
trimpath binary build. The real embed test and complete race/coverage test
commands are separate, and none of their time is counted as compilation.

| Workload / image | Cold compilation/vet | Warm 1 | Warm 2 | Samples |
| --- | ---: | ---: | ---: | --- |
| Race / A | 161.64 | 4.51 | 3.39 | attempts 1, 2, 3 |
| Coverage / B | 102.54 | 3.72 | 3.29 | attempts 2, 3, 4 |
| Ordinary embed/trimpath build, E2E audits / B | 51.93 | 2.45 | 2.20 | attempts 1, 2, 3 |

Coverage's B cold sample is **cold for coverage artifacts**, with modules and
ordinary build artifacts restored from B's E2E audits cache. It is not an
empty module/build directory: coverage compiled its missing instrumented
variant and saved its own key, then two exact-hit runs used it. The initial
full cache-miss coverage sample was on A (82.87s); it is recorded but is not
used as B's matched comparator. All other cold rows above are cache misses
under the new namespace.

The independent integrated build also passes: A cold compilation/vet 76.49s
becomes 2.29s on its exact-hit rerun. On B it takes 2.72s using the compatible
ordinary build cache and saves its own workload key. This shares the same
build wrapper/profile measured twice warm by E2E, while preserving the
independent build job. E2E app on B takes 2.54s from a compatible audits cache
and 2.43s on its following exact hit. Workload fallbacks can therefore be
useful without suppressing distinct writers.

The complete baseline observations, including module and execution time, are
in the JSON. Module prefetch is 0.60–1.02s on the first cache misses and 0.00s
on subsequent runs (reported to hundredths). Race execution commands take
258.83s, 169.98s and 247.44s; B coverage execution takes 133.01s, 113.82s and
114.66s. These retain the original checks and may include valid Go test-result
cache hits. They include driver/link overhead and are not pure test-body CPU.

Baseline misses spend 0.209–0.365s attempting restore and 3–5s saving.
Useful restores across the subsequent baseline observations take 1.628–4.557s;
exact hits skip saving. The two B workload fills in attempt 2 cost 4s each,
and B's integrated-build fill in attempt 3 costs 5s. Restore measurements span
the action's setup/download/extract to the following log step; save durations
use Actions step timestamps with one-second granularity. Stored compressed
archive bytes are in the cache inventory. These costs must be included when
evaluating the compile/vet reduction.

Browser execution remains the principal workflow bottleneck. For example,
audits job wall time is 491s cold, 649s warm and 600s warm despite its much
shorter Go build. Frontend build, browser download/startup and browser tests
are additional costs. No whole-workflow or whole-E2E-setup speedup is claimed.

## Source-only refresh and retention

The probe `18707b1866eb8d47d4036be9be54ee89f183a388` adds only a Go documentation
comment on configuration loading. It changes neither executable behavior nor
`backend/go.sum`. Its source digest is
`d6e214b496e0f6076cbeb47f1ebb89f43df2bd275d3915ead2b34661f5c29b2d`.
[Run 39742965739](https://github.com/sergeyfarin/rekenraam/actions/runs/39742965739)
checks the fallback/save and subsequent restore at that source.

Both attempts pass all six jobs. In attempt 1, all five Go workloads restore
their previous-source workload key and save the new source key after their
checks pass. The native and dependency portions remain identical within
each image. Attempt 2 runs again at exactly the probe source on fresh runners.

| Workload | First image | Refresh compile/vet | Following image | Following restore | Following compile/vet |
| --- | --- | ---: | --- | --- | ---: |
| Backend Tests | A | 5.01 | A | exact | 4.69 |
| E2E Smoke (audits) | B | 2.09 | B | exact | 2.53 |
| Backend Coverage | A | 4.59 | A | exact | 3.12 |
| Integrated Build | A | 2.45 | A | exact | 2.29 |
| E2E Smoke (app) | A | 2.33 | A | exact | 2.73 |

In the following run, five Go workloads directly restore keys saved
by the first probe run. Full key matches, source digests, paths and phase timings
are recorded per job in JSON. All five are exact hits on their matching
native image and workload; all five skip saving. This verifies useful
source refresh and reuse without creating a new immutable entry per rerun.
The refresh run spends 1.794–2.323s restoring and 3–4s per workload saving;
the following exact-hit run spends 1.501–3.413s restoring and nothing saving.

Branch-scoped cache inventory after each completed sample:

| After | Entries | Stored bytes | GiB |
| --- | ---: | ---: | ---: |
| cold | 5 | 615,561,942 | 0.573 |
| warm1 | 7 | 900,248,379 | 0.838 |
| warm2 | 8 | 1,008,676,079 | 0.939 |
| source-refresh | 13 | 1,635,308,448 | 1.523 |
| source-refresh-warm | 13 | 1,635,308,448 | 1.523 |

All entries from the earlier snapshots remain in the final snapshot; no
eviction churn is observed in this interval. Growth is attributable to
previously missing image/workload combinations and the deliberate source
transition. Exact hits do not create a cache per rerun. These observations
do not establish indefinite retention: repository quota and inactivity
eviction still apply, and larger histories require continued monitoring.

## Retained checks and limits

The shared backend scripts still run formatting, vet, the full race gate and
the separate non-race coverage gate with its 73% floor. Compile-only timing
passes run no tests; they precede the original complete commands. Frontend
checks, unit/catalog tests, integrated single-binary build, embedded-output
test, browser harness/stub controls and both smoke selections remain enabled.
Native bundled SQLite, FTS5, netgo/osusergo, FULL durability, two-package Go
scheduling, five-minute deadlines and independent build scheduling are retained.
No event frequency, runtime dependency or browser assertion was weakened.

The evidence verifies cache behavior for the sampled fresh runners and source
transition. Repository quota/inactivity eviction still bound retention; future
runner images or compiler changes intentionally start separate native keys.
It does not promise permanent cache availability or faster browser execution.
