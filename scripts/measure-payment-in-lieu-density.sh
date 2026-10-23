#!/usr/bin/env bash
# Payment-in-lieu admission density measurement (#379): builds books with
# increasing effective payment-in-lieu density in two history shapes and
# measures short corrections (accepted and refused), an unrelated investment
# command and their rolled-back previews. Reports figures only; it asserts
# entitlement and rollback correctness, not a latency ceiling. Off the push
# gate on purpose and not scheduled in CI: the 1,000-payment build alone takes
# tens of minutes, because every payment it records runs the same admission.
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
go test -count=1 -timeout=6h -v -run '^TestPaymentInLieuAdmissionCostByDensity$' ./internal/app/ |
  grep -E '^(=== RUN|--- |\s+payment_in_lieu_density_test\.go:[0-9]+: (BUILD|RESULT)|FAIL|ok|PASS)'
