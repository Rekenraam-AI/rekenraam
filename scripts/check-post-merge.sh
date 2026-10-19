#!/usr/bin/env bash
# pipefail so a failing stage of a pipeline inside this script fails the script;
# `set -e` alone only sees the last stage. Needs bash: dash has no pipefail.
set -euo pipefail

# Post-merge audit (#109). Run it after any non-trivial merge, or while one is
# still being resolved: code review looks at the code, but every defect the
# 2026-08 merge of two long-diverged branches shipped lived in the *resolved
# conflicts* — a duplicated docs/conventions.md paragraph, duplicated `case`
# arms, a duplicate CodeQL workflow, colliding backlog IDs. Each check below is
# one of those shapes, made mechanical.
#
# Usage:
#   scripts/check-post-merge.sh                 audit the working tree; the
#                                               lost-test check compares against
#                                               HEAD and MERGE_HEAD mid-merge, or
#                                               against HEAD's parents when HEAD
#                                               is a merge commit
#   scripts/check-post-merge.sh REF [REF...]    audit the working tree; the
#                                               lost-test check compares against
#                                               the given refs (e.g. both sides
#                                               of a merge you are about to make)
#
# SKIP_OPENAPI=1 skips the generated-client check (it needs frontend deps).
#
# Every check reports and keeps going; the exit status is 1 when any check
# found something. A finding is a prompt to look, not proof of a defect — an
# intended test deletion is reported too, and is then simply acknowledged.

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

FAILED=0
fail() {
  FAILED=1
  printf '  %s\n' "$@"
}
section() {
  printf '\n== %s\n' "$1"
}

# --- 1. Numbered-file collisions ---------------------------------------------
# Two branches that each added "the next" migration or ADR pick the same number.
# goose orders by version, so a migration collision silently applies one file.
section "numbered-file collisions (migrations, ADRs)"
for spec in 'backend/migrations:*.sql' 'docs/adrs:*.md'; do
  dir="${spec%%:*}"
  pattern="${spec#*:}"
  dupes="$(find "$dir" -maxdepth 1 -name "$pattern" -printf '%f\n' |
    grep -oE '^[0-9]+' | sort | uniq -d || true)"
  for n in $dupes; do
    fail "$dir: number $n is used by: $(find "$dir" -maxdepth 1 -name "${n}[-_]*" -printf '%f ')"
  done
done

# --- 2. Generated-client drift -------------------------------------------------
# A conflict resolved in api/openapi/ but not regenerated leaves schema.d.ts
# describing a contract neither branch had.
section "generated OpenAPI client drift"
GENERATED=frontend/src/lib/api/schema.d.ts
if [ "${SKIP_OPENAPI:-0}" = 1 ]; then
  echo "  skipped (SKIP_OPENAPI=1)"
elif ! command -v pnpm >/dev/null || [ ! -d frontend/node_modules ]; then
  fail "cannot regenerate: pnpm or frontend/node_modules missing (run pnpm install, or SKIP_OPENAPI=1)"
else
  before="$(git hash-object "$GENERATED")"
  pnpm --silent --dir frontend run openapi:generate >/dev/null
  after="$(git hash-object "$GENERATED")"
  if [ "$before" != "$after" ]; then
    fail "$GENERATED changed on regeneration; review and commit it"
  fi
fi

# --- 3. Duplicate doc paragraphs -----------------------------------------------
# Both sides kept in a conflict hunk: the same paragraph twice in one file.
# Short paragraphs (headings, table rules, one-word lines) repeat legitimately,
# so only paragraphs of at least 80 non-space-collapsed characters count.
# Fenced code blocks are skipped: repeated commands in examples are normal.
section "duplicate paragraphs in Markdown docs"
dupes="$(
while IFS= read -r -d '' file; do
  awk -v file="$file" '
    function flush() {
      if (length(para) >= 80) {
        if (para in seen) {
          printf "%s:%d duplicates the paragraph at line %d\n", file, start, seen[para]
        } else {
          seen[para] = start
        }
      }
      para = ""
    }
    /^[ \t]*(```|~~~)/ { flush(); fence = !fence; next }
    fence { next }
    /^[ \t]*$/ { flush(); next }
    {
      line = $0
      gsub(/[ \t]+/, " ", line)
      sub(/^ /, "", line)
      if (para == "") start = NR
      para = para (para == "" ? "" : " ") line
    }
    END { flush() }
  ' "$file"
done < <(git ls-files -z --cached --others --exclude-standard -- \
  '*.md' ':!:.archive/**' ':!:docs/archive/**' ':!:**/node_modules/**')
)"
if [ -n "$dupes" ]; then
  while IFS= read -r line; do fail "$line"; done <<<"$dupes"
fi

# --- 4. Duplicate workflows ----------------------------------------------------
# Two workflow files doing the same job (codeql.yml next to codeql2.yml) double
# the CI cost and the alert noise. Flags a top-level name, a non-setup action,
# or a one-line run command that appears in more than one workflow file.
section "duplicate GitHub workflows"
dupes="$(
  for wf in .github/workflows/*.y*ml; do
    {
      grep -E '^name:' "$wf" | sed -E 's/^name:[[:space:]]*/name /' || true
      grep -oE 'uses:[[:space:]]*[^@[:space:]]+' "$wf" | sed -E 's/uses:[[:space:]]*/uses /' |
        grep -vE '^uses (actions/(checkout|setup-[a-z]+|cache|upload-artifact|download-artifact)|pnpm/action-setup)$' || true
      grep -E '^[[:space:]]*run:[[:space:]]*[^|>[:space:]]' "$wf" | sed -E 's/^[[:space:]]*run:[[:space:]]*/run /' |
        # Chromium is setup shared by the independent browser workflows (#347).
        grep -vE '^run (pnpm install|pnpm --dir e2e exec playwright install --with-deps chromium$)' || true
    } | sort -u | sed "s|\$|	$wf|"
  done | sort | awk -F'\t' '
    $1 == prev { files = files ", " $2; n++; next }
    { if (n > 1) print prev ": " files; prev = $1; files = $2; n = 1 }
    END { if (n > 1) print prev ": " files }
  '
)"
if [ -n "$dupes" ]; then
  while IFS= read -r line; do fail "$line"; done <<<"$dupes"
fi

# --- 5. Colliding work-item IDs ------------------------------------------------
# A historical alias mapped twice, or one `### T-nn` heading twice in a file.
# Local IDs are no longer allocated (GitHub numbers are canonical), so a new
# hit is a merge artifact. Collisions that predate this check and are explained
# in their file are listed here as "file:ID" so they do not re-alarm.
KNOWN_ID_COLLISIONS=(
  docs/reviews/resolved-backlog-2026-09-27.md:T-90
  docs/reviews/resolved-backlog-2026-09-27.md:T-91
)
section "colliding work-item IDs"
dupes="$(grep -oE '^\| [A-Z]-[0-9]+ \|' docs/backlog.md | sort | uniq -d || true)"
for id in $dupes; do
  [ "$id" = "|" ] && continue
  fail "docs/backlog.md maps $id more than once"
done
while IFS= read -r -d '' file; do
  dupes="$(grep -oE '^#{2,4} [A-Z]-[0-9]+\b' "$file" | sed -E 's/^#+ //' | sort | uniq -d || true)"
  for id in $dupes; do
    [[ " ${KNOWN_ID_COLLISIONS[*]} " == *" $file:$id "* ]] && continue
    fail "$file has more than one $id heading"
  done
done < <(git ls-files -z -- 'docs/*.md' 'docs/**/*.md' ':!:docs/archive/**')

# --- 6. Lost tests -------------------------------------------------------------
# A test that existed on either side but not in the result was deleted or
# renamed by the merge. Go tests are keyed by package directory, so moving one
# between files of a package is not a loss; TS tests by file and title.
section "tests lost relative to the merged sides"
REFS=("$@")
if [ ${#REFS[@]} -eq 0 ]; then
  if git rev-parse -q --verify MERGE_HEAD >/dev/null; then
    REFS=(HEAD MERGE_HEAD)
  elif git rev-parse -q --verify HEAD^2 >/dev/null; then
    REFS=(HEAD^1 HEAD^2)
  fi
fi

# test_names [REF]: one "path-key<TAB>name" line per test, from REF or, with no
# REF, from the working tree (tracked plus untracked, so a mid-merge resolution
# counts before it is staged).
test_names() {
  local opts=(--untracked) rev=()
  # A resolved commit id keeps git grep's "REV:" prefix free of colons, so it
  # can be stripped without guessing where the ref ends and the path begins.
  [ $# -gt 0 ] && opts=() && rev=("$(git rev-parse "$1^{commit}")")
  {
    git grep "${opts[@]}" -E '^func (Test|Fuzz)[A-Za-z0-9_]*\(' "${rev[@]}" -- '*_test.go' || true
    git grep "${opts[@]}" -E "^[[:space:]]*(test|it)(\.(only|skip|fixme|serial))?\([[:space:]]*['\"\`]" "${rev[@]}" -- \
      'frontend/src/*.test.ts' 'e2e/playwright/*.spec.ts' || true
  } | sed -E 's/^[0-9a-f]{40,64}://' | sed -E \
    -e 's/^(.*)\/[^/]+_test\.go:func ((Test|Fuzz)[A-Za-z0-9_]*)\(.*/\1\t\2/' \
    -e "s/^([^:]+):[[:space:]]*(test|it)(\.[a-z]+)?\([[:space:]]*['\"\`]([^'\"\`]*).*/\1\t\4/"
}

if [ ${#REFS[@]} -eq 0 ]; then
  echo "  skipped: HEAD is not a merge and no merge is in progress; pass the merged refs to compare"
else
  current="$(test_names | sort -u)"
  for ref in "${REFS[@]}"; do
    git rev-parse -q --verify "$ref^{commit}" >/dev/null || { fail "unknown ref: $ref"; continue; }
    lost="$(comm -23 <(test_names "$ref" | sort -u) <(printf '%s\n' "$current"))"
    if [ -n "$lost" ]; then
      while IFS=$'\t' read -r where name; do
        fail "missing since $ref: $where: $name"
      done <<<"$lost"
    fi
  done
fi

echo
if [ "$FAILED" -ne 0 ]; then
  echo "post-merge audit: findings above — review each before pushing"
  exit 1
fi
echo "post-merge audit: clean"
