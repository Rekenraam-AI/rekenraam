#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"
source "$ROOT/scripts/native-sqlite-env.sh"
exec go "$@"
