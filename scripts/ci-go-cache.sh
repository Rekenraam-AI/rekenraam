#!/usr/bin/env bash
# Called after setup-go, before installing frontend dependencies/build output.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"
source "$ROOT/scripts/native-sqlite-env.sh"

: "${GO_CACHE_WORKLOAD:?workload required}" "${GO_CACHE_SOURCE:?source digest required}"
case "$GO_CACHE_WORKLOAD" in
  race|coverage|build|e2e-app|e2e-audits) ;;
  *) echo "unknown Go cache workload: $GO_CACHE_WORKLOAD" >&2; exit 2 ;;
esac

# Native cache compatibility includes runner image, libc and compiler, beyond
# Go's own artifact hashes. Never fall back across this boundary.
configuration="$(
  set -e
  go env -json GOVERSION GOOS GOARCH GOAMD64 GOARM GOEXPERIMENT CGO_ENABLED \
    CC CXX CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS GOFLAGS
  printf 'ImageOS=%s\nImageVersion=%s\n' "${ImageOS:-local}" "${ImageVersion:-local}"
  cat /etc/os-release
  "${REKENRAAM_CC_COMMAND[@]}" --version
  ldd --version
)"
printf '%s\n' "$configuration"
native="$(printf '%s\n' "$configuration" | sha256sum | cut -d ' ' -f 1)"
dependencies="$(sha256sum go.sum | cut -d ' ' -f 1)"
prefix="go-v1-$native-"
dependency_prefix="$prefix$dependencies-"
workload_prefix="$dependency_prefix$GO_CACHE_WORKLOAD-"
gocache="$(go env GOCACHE)"
gomodcache="$(go env GOMODCACHE)"
{
  printf 'gocache=%s\ngomodcache=%s\n' "$gocache" "$gomodcache"
  printf 'native-prefix=%s\ndependency-prefix=%s\nworkload-prefix=%s\n' \
    "$prefix" "$dependency_prefix" "$workload_prefix"
  printf 'key=%s%s\n' "$workload_prefix" "$GO_CACHE_SOURCE"
} >> "$GITHUB_OUTPUT"
# Children, including Playwright's pnpm build, inherit exactly these paths.
printf 'GOCACHE=%s\nGOMODCACHE=%s\nGO_PHASE_TIMINGS=1\n' "$gocache" "$gomodcache" >> "$GITHUB_ENV"
printf 'GOCACHE=%s\nGOMODCACHE=%s\nsource=%s\ncommit=%s\n' "$gocache" "$gomodcache" "$GO_CACHE_SOURCE" "$(git rev-parse HEAD)" | tee -a "$GITHUB_STEP_SUMMARY"
