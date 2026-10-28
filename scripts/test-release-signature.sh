#!/usr/bin/env bash
# Exercise the real keyless verifier; no synthetic certificate substitutes.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ "$#" -ne 2 ]; then
  echo "Usage: $0 DOWNLOAD_DIRECTORY EXPECTED_CERTIFICATE_IDENTITY" >&2
  exit 2
fi
REKENRAAM_DOWNLOADS="$(cd "$1" && pwd)"
REKENRAAM_CONTROLS="$(mktemp -d)"
trap 'rm -rf "$REKENRAAM_CONTROLS"' EXIT
"$ROOT/scripts/verify-release.sh" "$REKENRAAM_DOWNLOADS" "$2"
if "$ROOT/scripts/verify-release.sh" "$REKENRAAM_DOWNLOADS" "$2/unexpected-workflow"; then
  echo "unexpected signer identity was accepted" >&2
  exit 1
fi
cp -R "$REKENRAAM_DOWNLOADS" "$REKENRAAM_CONTROLS/manifest"
echo tampered >> "$REKENRAAM_CONTROLS/manifest/SHA256SUMS"
if "$ROOT/scripts/verify-release.sh" "$REKENRAAM_CONTROLS/manifest" "$2"; then
  echo "tampered checksum manifest was accepted" >&2
  exit 1
fi
cp -R "$REKENRAAM_DOWNLOADS" "$REKENRAAM_CONTROLS/archive"
for archive in "$REKENRAAM_CONTROLS/archive/"*.tar.gz; do
  echo tampered >> "$archive"
  break
done
if "$ROOT/scripts/verify-release.sh" "$REKENRAAM_CONTROLS/archive" "$2"; then
  echo "tampered platform archive was accepted" >&2
  exit 1
fi
"$ROOT/scripts/verify-release.sh" "$REKENRAAM_DOWNLOADS" "$2"
echo "Real signature, identity and tamper controls passed"
