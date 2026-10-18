#!/usr/bin/env bash
# Exercise Docker's actual ignore rules with synthetic private/generated files.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

private_paths=(backend/var/backups/private.sqlite backend/.env frontend/.env.local
  frontend/node_modules/private/index.js frontend/build/old.html
  frontend/.svelte-kit/generated.js backend/internal/web/dist/old.html
  .git/config .archive/private.txt dist/rekenraam)
source_paths=(package.json pnpm-lock.yaml pnpm-workspace.yaml backend/go.mod
  backend/cmd/rekenraam/main.go frontend/src/routes/+page.svelte
  frontend/static/robots.txt api/openapi/openapi.yaml scripts/native-sqlite-env.sh)
for path in "${private_paths[@]}" "${source_paths[@]}"; do
  mkdir -p "$WORK/context/$(dirname "$path")"
  printf 'synthetic audit sentinel\n' > "$WORK/context/$path"
done
if [[ -f "$ROOT/.dockerignore" ]]; then
  cp "$ROOT/.dockerignore" "$WORK/context/.dockerignore"
fi
printf 'FROM scratch\nCOPY . /\n' > "$WORK/probe.Dockerfile"
docker build --quiet -f "$WORK/probe.Dockerfile" \
  --output "type=local,dest=$WORK/result" "$WORK/context" >/dev/null
for path in "${private_paths[@]}"; do
  if [[ -e "$WORK/result/$path" ]]; then
    echo "FAIL: private/generated path reached Docker COPY: $path" >&2
    exit 1
  fi
done
for path in "${source_paths[@]}"; do
  if [[ ! -f "$WORK/result/$path" ]]; then
    echo "FAIL: required source excluded: $path" >&2
    exit 1
  fi
done
echo 'Docker context: all private/generated sentinels excluded; required sources retained.'
