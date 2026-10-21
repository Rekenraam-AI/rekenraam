#!/usr/bin/env bash
# The browser journeys fast enough to gate every push: everything except the
# serial release-preflight suite, which stays a deliberate pre-release run.
#
# pipefail so a failing stage of a pipeline inside this script fails the script;
# `set -e` alone only sees the last stage. It cannot help a caller that pipes
# this script — `... | tail` reports tail's status — that trap is the caller's
# to close. Needs bash: dash has no pipefail.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# CI runs these disjoint selections on independent runners. The default stays
# the complete smoke gate for local use; every original case belongs to one group.
projects=()
case "${E2E_SUITE:-all}" in
  all) ;;
  app) projects=(--project=bootstrap --project=observation-selection --project=app) ;;
  audits) projects=(--project=route-sweep --project=subview-sweep) ;;
  *) echo "E2E_SUITE must be all, app or audits" >&2; exit 2 ;;
esac

pnpm --dir "$ROOT/e2e" run test:stubs
pnpm --dir "$ROOT/e2e" run test:harness
pnpm --dir "$ROOT/e2e" exec playwright test --grep-invert "release preflight" "${projects[@]}"
