#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ "$#" -ne 3 ]; then
  echo "Usage: $0 VERSION ARCH OUTPUT_DIRECTORY" >&2
  exit 2
fi
cd "$ROOT"
python3 scripts/release-artifacts.py validate-version "$1"
pnpm build
REKENRAAM_REPEAT_DIR="$(mktemp -d)"
trap 'rm -rf "$REKENRAAM_REPEAT_DIR"' EXIT
./scripts/go-backend.sh build -trimpath -o "$REKENRAAM_REPEAT_DIR/rekenraam" ./cmd/rekenraam
cmp dist/rekenraam "$REKENRAAM_REPEAT_DIR/rekenraam"
python3 scripts/release-artifacts.py package --version "$1" --arch "$2" --output "$3"
tar -xzf "$3/rekenraam-$1-linux-$2.tar.gz" --strip-components=1 -C "$REKENRAAM_REPEAT_DIR"
python3 scripts/smoke-release-binary.py "$REKENRAAM_REPEAT_DIR/rekenraam"
