#!/usr/bin/env bash
# Post-#388 payment-in-lieu density measurement. Compare with #379's historical
# global admission baseline using the same two history shapes. Measures accepted
# short corrections, unrelated writes and rolled-back previews; checks cash,
# diagnostic and rollback correctness without a latency ceiling. Defaults to
# 0/10/100/300 payments; 1,000 remains an explicit opt-in. Off the push gate.
#
#   ./scripts/measure-payment-in-lieu-density.sh
#   REKENRAAM_PIL_DENSITIES=0,10,100 REKENRAAM_PIL_SHAPES=unrelated ./scripts/measure-payment-in-lieu-density.sh
#
# REKENRAAM_PIL_REPEATS (default 5) sets runs per operation.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$ROOT/backend"
source "$ROOT/scripts/native-sqlite-env.sh"

export REKENRAAM_PIL_DENSITY=1
# No -race: the figures are the point, and race instrumentation distorts them.
go test -count=1 -timeout=15m -v -run '^TestPaymentInLieuAdmissionCostByDensity$' ./internal/app/ |
  grep -E '^(=== RUN|--- |\s+payment_in_lieu_density_test\.go:[0-9]+: (BUILD|RESULT)|FAIL|ok|PASS)'
