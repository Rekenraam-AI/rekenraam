#!/usr/bin/env bash
# pipefail so a failing stage of a pipeline inside this script fails the script;
# `set -e` alone only sees the last stage. It cannot help a caller that pipes
# this script — `... | tail` reports tail's status — that trap is the caller's
# to close. Needs bash: dash has no pipefail.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT/backend"
source "$ROOT/scripts/native-sqlite-env.sh"
source "$ROOT/scripts/go-phase-timing.sh"

# Formatting gate. Without this in the wrapper script, CI never sees gofmt
# drift (backlog T-49: four files sat unformatted with a green pipeline).
#
# Resolve gofmt from the toolchain `go` actually selected, never whichever one
# happens to be first on PATH. Those differ whenever a version manager pins an
# older Go while go.mod's `go` directive makes the go command switch to a newer
# one, and gofmt's output is not stable across releases (1.27 reindents
# multi-value composite literals in return statements). A stale gofmt reports
# both false negatives — drift that CI then rejects — and false positives on
# files that are correctly formatted for the toolchain being compiled with.
GOFMT="$(go env GOROOT)/bin/gofmt"
if [ ! -x "$GOFMT" ]; then
  echo "no usable gofmt in the selected toolchain at $GOFMT" >&2
  echo "go is $(command -v go) reporting $(go version)" >&2
  exit 1
fi

unformatted="$("$GOFMT" -l .)"
if [ -n "$unformatted" ]; then
  echo "gofmt needs to be run on:" >&2
  echo "$unformatted" >&2
  echo "Fix with: $GOFMT -w <files>" >&2
  exit 1
fi

go_download_phase
go_phase vet go vet ./...

# COVERAGE=1 runs a separate, non-race coverage pass (a combined run is slow
# and muddies both signals). Writes backend/coverage.out and prints the merged
# total. Both modes share the native scheduling and per-package deadline.
if [ "${COVERAGE:-0}" = "1" ]; then
  if [ "${GO_PHASE_TIMINGS:-0}" = "1" ]; then
    # Build/link every test binary without running TestMain or any test. The
    # execution wrapper is part of Go's result-cache identity; -count=1 also
    # prevents retaining these synthetic successes as test results.
    go_phase coverage-compile-vet go test -exec /bin/true -count=1 -p 2 -timeout=5m ./... -coverpkg=./... -coverprofile=coverage.out
  fi
  go_phase coverage-tests go test -p 2 -timeout=5m ./... -coverpkg=./... -coverprofile=coverage.out
  go tool cover -func=coverage.out | tail -1
else
  # Ordinary integration fixtures copy a process-wide migrated SQLite template
  # (internal/testdb) instead of replaying the full schema per test, so every
  # top-level application and database test runs with t.Parallel() and uses
  # available Go parallelism inside its package (T-125;
  # TestIntegrationSuitesRunTestsInParallel keeps it that way). Native SQLite
  # with FULL supports two concurrent packages and a five-minute package
  # deadline (ADR 0016). No fixed GOMAXPROCS, -parallel or CPU-affinity cap.
  # Keep every assertion and Go race instrumentation; the native C engine is
  # outside that instrumentation. The deadline is a failure bound, not a target.
  # Measurements: docs/reviews/native-test-gate-2026-10-12.md.
  if [ "${GO_PHASE_TIMINGS:-0}" = "1" ]; then
    go_phase race-compile-vet go test -exec /bin/true -count=1 -race -p 2 -timeout=5m ./...
  fi
  go_phase race-tests go test -race -p 2 -timeout=5m ./...
fi
