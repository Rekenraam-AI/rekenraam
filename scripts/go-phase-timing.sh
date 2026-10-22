#!/usr/bin/env bash
# Source from shared wrappers. Opt-in diagnostics keep network, compilation/vet
# and execution separate; normal local commands retain their existing behavior.
go_phase() {
  local phase="$1"
  shift
  if [ "${GO_PHASE_TIMINGS:-0}" = "1" ]; then
    printf 'Go phase %s starting\n' "$phase"
    /usr/bin/time -f "Go phase $phase: wall=%es user=%Us sys=%Ss" "$@"
  else
    "$@"
  fi
}

go_download_phase() {
  if [ "${GO_PHASE_TIMINGS:-0}" = "1" ]; then
    printf 'Effective Go paths: GOCACHE=%s GOMODCACHE=%s\n' "$(go env GOCACHE)" "$(go env GOMODCACHE)"
    go_phase modules go mod download
  fi
}
