#!/usr/bin/env bash
# Large-book measurement (#108): generates a 250k-posting book across ~3000
# accounts and runs the bundle export and the self-check against it, failing
# when wall time or peak live heap crosses a sharp-regression ceiling. Off the
# push gate on purpose; CI runs it weekly (.github/workflows/large-book.yml).
#
#   ./scripts/measure-large-book.sh           # bounded assertions + logged figures
#   BENCH=1 ./scripts/measure-large-book.sh   # also the testing.B benchmarks
#
# pipefail so a failing stage of a pipeline inside this script fails the script;
# `set -e` alone only sees the last stage. Needs bash: dash has no pipefail.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT/backend"
source "$ROOT/scripts/native-sqlite-env.sh"

export REKENRAAM_LARGE_BOOK=1
# No -race: the figures are the point, and race instrumentation distorts them.
go test -count=1 -timeout=15m -v -run '^TestLargeBookExportAndSelfCheckStayBounded$' ./internal/app/
if [ "${BENCH:-0}" = "1" ]; then
  go test -count=1 -timeout=15m -run '^$' -bench '^BenchmarkLargeBook' -benchtime=1x ./internal/app/
fi
