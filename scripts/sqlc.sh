#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# Generate Go code from sqlc.yaml with the sqlc version pinned in tools/go.mod (ADR-0003).
#
#   scripts/sqlc.sh          regenerate every output directory in place
#   scripts/sqlc.sh --check  regenerate into a scratch copy and fail if the committed code differs
#
# Every `out:` directory in sqlc.yaml holds only generated code: generation empties it first,
# so files of removed queries disappear. Generated files get the SPDX header that sqlc omits.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
sqlc=(env CGO_ENABLED=0 go tool -modfile="$root/tools/go.mod" sqlc)
header='// SPDX-License-Identifier: AGPL-3.0-only'

# Output directories, relative to the repository root.
mapfile -t outs < <(awk '$1 == "out:" { print $2 }' "$root/sqlc.yaml")
if [ "${#outs[@]}" -eq 0 ]; then
  echo "sqlc.sh: no out: directories in sqlc.yaml" >&2
  exit 1
fi

# generate DIR regenerates every output directory under DIR, which mirrors the repository root.
generate() {
  local dir="$1" out file
  for out in "${outs[@]}"; do
    rm -rf "${dir:?}/$out"
  done
  # sqlc resolves paths relative to the config file; go runs from the root to find tools/go.mod.
  (cd "$root" && "${sqlc[@]}" -f "$dir/sqlc.yaml" generate)
  for out in "${outs[@]}"; do
    for file in "$dir/$out"/*.go; do
      printf '%s\n\n' "$header" | cat - "$file" > "$file.tmp"
      mv "$file.tmp" "$file"
    done
  done
}

if [ "${1:-}" != "--check" ]; then
  generate "$root"
  exit 0
fi

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
# Copy every input sqlc.yaml may reference (schema, queries, and fixtures beside the adapters).
for path in sqlc.yaml schema queries internal; do
  if [ -e "$root/$path" ]; then
    cp -R "$root/$path" "$scratch/"
  fi
done
generate "$scratch"
stale=0
for out in "${outs[@]}"; do
  if ! diff -ruN "$root/$out" "$scratch/$out"; then
    stale=1
  fi
done
if [ "$stale" -ne 0 ]; then
  echo "sqlc-generated code is stale; run 'scripts/task.sh go:sqlc' and commit the result" >&2
  exit 1
fi
echo "sqlc-generated code is up to date"
