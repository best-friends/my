#!/usr/bin/env bash
# Regenerates the ADR table in docs/adr/README.md.
# Usage: scripts/adr-index.sh [--check]
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dir="$root/docs/adr"
readme="$dir/README.md"

table="$(mktemp)"
trap 'rm -f "$table" "$table.new"' EXIT

{
  echo "| ADR | タイトル | Status |"
  echo "| --- | --- | --- |"
  for f in "$dir"/[0-9][0-9][0-9][0-9]-*.md; do
    name="$(basename "$f")"
    num="${name%%-*}"
    title="$(sed -n '1s/^# ADR-[0-9]*: *//p' "$f")"
    status="$(sed -n 's/^- Status: *\([A-Za-z]*\( by ADR-[0-9]*\)\{0,1\}\).*/\1/p' "$f" | head -n1)"
    if [[ -z "$title" || -z "$status" ]]; then
      echo "error: $name must start with '# ADR-NNNN: <title>' and contain '- Status: <status>'" >&2
      exit 1
    fi
    echo "| [${num}](${name}) | ${title} | ${status} |"
  done
} >"$table"

awk -v table="$table" '
  /<!-- adr-index:start -->/ { print; while ((getline line < table) > 0) print line; skip = 1; next }
  /<!-- adr-index:end -->/   { skip = 0 }
  !skip { print }
' "$readme" >"$table.new"

if [[ "${1:-}" == "--check" ]]; then
  if ! diff -u "$readme" "$table.new"; then
    echo "error: docs/adr/README.md is out of date. Run 'scripts/task.sh docs:adr-index'." >&2
    exit 1
  fi
else
  mv "$table.new" "$readme"
fi
