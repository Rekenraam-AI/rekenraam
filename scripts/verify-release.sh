#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ "$#" -ne 2 ]; then
  echo "Usage: $0 DOWNLOAD_DIRECTORY EXPECTED_CERTIFICATE_IDENTITY" >&2
  exit 2
fi
cd "$1"
if [ -L SHA256SUMS ] || [ -L SHA256SUMS.sigstore.json ]; then
  echo "release manifest and signature bundle must be regular files" >&2
  exit 1
fi
# The expected identity is supplied by the verifier, never by downloaded metadata.
cosign verify-blob SHA256SUMS --bundle SHA256SUMS.sigstore.json \
  --certificate-identity="$2" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
python3 "$ROOT/scripts/release-artifacts.py" verify-files .
