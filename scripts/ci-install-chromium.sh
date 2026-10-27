#!/usr/bin/env bash
# CI only: Playwright's Chromium and the system packages it needs (#415).
#
# The packages come from the runner's apt mirror and the browser from
# Playwright's CDN. Either can stall: one hosted run spent 10 minutes fetching
# 26 MB of fonts from azure.archive.ubuntu.com at 42 kB/s. Each phase therefore
# gets a deadline and one retry, and the package retry moves apt off the Azure
# mirror, so a stalled mirror costs at most one deadline instead of the job.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# A normal run needs well under a minute for each phase.
attempt_seconds="${CHROMIUM_INSTALL_ATTEMPT_SECONDS:-180}"

# GNU timeout signals its whole process group, so apt under sudo stops too.
attempt() { timeout --kill-after=15 "$attempt_seconds" pnpm --dir "$ROOT/e2e" exec playwright "$@"; }

use_primary_ubuntu_archive() {
  local file
  for file in /etc/apt/apt-mirrors.txt /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do
    if [[ -f "$file" ]] && grep -q 'azure\.archive\.ubuntu\.com' "$file"; then
      sudo sed -i 's/azure\.archive\.ubuntu\.com/archive.ubuntu.com/g' "$file"
    fi
  done
  # Finish anything the timed-out attempt left half-configured.
  sudo dpkg --configure -a || true
}

if ! attempt install-deps chromium; then
  echo "::warning::Chromium system packages failed or exceeded ${attempt_seconds}s; retrying from archive.ubuntu.com"
  use_primary_ubuntu_archive
  attempt install-deps chromium
fi

if ! attempt install chromium; then
  echo "::warning::Chromium download failed or exceeded ${attempt_seconds}s; retrying"
  attempt install chromium
fi
