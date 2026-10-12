#!/usr/bin/env bash
# Source after entering backend/; used by every supported Go command.
if [ "$(go env CGO_ENABLED)" != "1" ]; then
  echo "Native SQLite requires CGO_ENABLED=1 and a C compiler. Unset CGO_ENABLED=0 or set CGO_ENABLED=1." >&2
  return 1
fi
read -r -a REKENRAAM_CC_COMMAND <<< "$(go env CC)"
if ! command -v "${REKENRAAM_CC_COMMAND[0]}" >/dev/null 2>&1; then
  echo "Native SQLite requires a C compiler and development headers (Debian/Ubuntu: apt install build-essential). CC=$(go env CC)" >&2
  return 1
fi
export CGO_ENABLED=1
# FTS5 is a schema requirement. netgo/osusergo retain the previous pure-Go
# resolver and user lookup behavior while SQLite alone uses bundled C.
export GOFLAGS="${GOFLAGS:-} -tags=fts5,netgo,osusergo"
