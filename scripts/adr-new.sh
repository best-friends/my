#!/usr/bin/env bash
# Creates a new ADR from docs/adr/template.md and refreshes the index.
# Usage: scripts/adr-new.sh "Title of the decision" [kebab-slug]
set -euo pipefail

if [[ $# -lt 1 || -z "$1" ]]; then
  echo "usage: $0 \"Title\" [slug]" >&2
  exit 2
fi

root="$(cd "$(dirname "$0")/.." && pwd)"
dir="$root/docs/adr"
title="$1"
# Non-ASCII titles cannot be slugified reliably; pass an explicit slug for them.
slug="${2:-$(printf '%s' "$title" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+|-+$//g')}"
if [[ -z "$slug" ]]; then
  echo "error: could not derive a slug from the title; pass one explicitly" >&2
  exit 2
fi

last="$(find "$dir" -maxdepth 1 -name '[0-9][0-9][0-9][0-9]-*.md' -printf '%f\n' | sort | tail -n1 | cut -c1-4)"
next="$(printf '%04d' $((10#${last:-0} + 1)))"
file="$dir/${next}-${slug}.md"

sed -e "1s/.*/# ADR-${next}: ${title//\//\\/}/" \
    -e "s/^- Status: .*/- Status: Proposed/" \
    -e "s/^- Date: .*/- Date: $(date +%F)/" \
    "$dir/template.md" >"$file"

"$root/scripts/adr-index.sh"
echo "$file"
