# Release candidate preparation — #333, 2026-10-28

Decision: [ADR 0020](../adrs/0020-release-signing-and-platform-targets.md).
Implementation/source: `0495cc4401d713805e276c7180d33983f20e1e0b`.
This record establishes local candidate preparation, not a published release
or hosted GitHub OIDC/native ARM acceptance. The commit remains local pending
an owner-requested push; #333 remains open.

## Actual candidate

The production frontend was built through `pnpm build`. From the clean
implementation commit, the native Go executable was rebuilt twice with the
shared `fts5,netgo,osusergo`/CGO configuration and `-trimpath`; `cmp` passed.
These are same-environment builds using the normal Go cache, not independent
clean-machine reproducibility evidence.

`release-artifacts.py package` produced `v0.1.0-rc.1` amd64 archives in two
different output directories. Their bytes matched. Both are unsigned local
candidates; they are not the final two-platform, OIDC-signed Actions artifact.

| Fact | Observed |
|---|---|
| Host | Ubuntu 24.04.5 LTS, amd64 |
| Go | 1.27.3 |
| Node / pnpm | 24.22.1 / 12.11.2 |
| C compiler | Ubuntu GCC 13.3.0 |
| Host glibc | 2.39-0ubuntu8.9 |
| Required ELF libraries | `libc.so.6`, `libm.so.6`; no system SQLite |
| Highest required glibc symbol | 2.34, within the declared 2.39 baseline |
| Highest migration | 2; both SQL files and their exact hashes recorded |
| Binary SHA-256 | `fd37c027ff540e77d72140801b05fd7895745996178e61e358b91f1a67763edf` |
| Archive SHA-256 | `85fcd8ee4e7db54bbeefd728bff61c69d6bb19b544e20a65208b09865058bd13` |

The extracted archive's executable booted with a disposable database,
completed fresh migrations, returned HTTP 204 at `/healthz` and served the
embedded frontend. The five focused release-preflight browser journeys then
passed against the clean-source executable (12.8 seconds). An earlier
worktree run also passed all five; the clean-source run is the final evidence.

## Verification and workflow checks

- Ten artifact tests pass: final-byte tampering, extra/missing/symlinked files,
  unsafe/duplicate/reserved manifest paths, candidate-only labels, complete
  migration manifests, deterministic archives, path-independent build metadata,
  architecture/glibc/bundled-SQLite checks, mixed-source/platform refusal and
  workflow publication/OIDC boundaries.
- CI event-policy (four) and Go-cache-policy (six) tests pass. The new artifact
  tests run in ordinary CI without changing event selection or required checks.
- `actionlint` passes for the candidate workflow and amended CI workflow;
  shell syntax, 296 local Markdown links and `git diff --check` pass.
- Cosign 3.1.4 was downloaded from the official release and checked against
  its published SHA-256. Real signing/verification of synthetic data with an
  ephemeral local test key passed; a changed payload failed. Requiring the
  candidate OIDC identity rejected that local-key bundle. No owner identity
  or financial data was used. This is not a positive GitHub OIDC test, nor
  proof that the hosted wrong-identity controls have run.

## Remaining acceptance

After an authorized push, dispatch the manual candidate workflow from `main`.
Retain its native amd64/arm64 builds, packaged-server smoke, preflight, real
OIDC positive/wrong-identity/tampered-manifest/tampered-archive results and
final downloadable bundle with the source SHA/run URL. Archive accepted
evidence beyond the Actions artifact retention window.

The candidate workflow has no release/tag publication path. Actual release
cutting still requires explicit owner instruction, retained release gates,
all included migration checksums pinned in the freeze test, installed-release
upgrade/PR policies, and immutable publication with the approved release-tag
identity. No Wails runtime or Windows artifact was introduced.
