#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# Generate the client API code from api/openapi.yaml with the oapi-codegen version pinned in
# tools/go.mod (ADR-0029).
#
#   scripts/generate.sh          regenerate every target in place
#   scripts/generate.sh --check  generate into a scratch directory and fail if the committed code differs
#
# Generated files get the SPDX header that oapi-codegen omits.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
spec="api/openapi.yaml"
header='// SPDX-License-Identifier: AGPL-3.0-only'

# Targets as "<oapi-codegen config> <output file>", relative to the repository root.
targets=(
  "internal/interface/clientapi/oapi-codegen.yaml internal/interface/clientapi/api.gen.go"
)

# generate CONFIG OUT writes the code that CONFIG generates from the spec to OUT.
generate() {
  local config="$1" out="$2"
  mkdir -p "$(dirname "$out")"
  (cd "$root" && go tool -modfile=tools/go.mod oapi-codegen -config "$config" -o "$out.tmp" "$spec")
  printf '%s\n\n' "$header" | cat - "$out.tmp" > "$out"
  rm "$out.tmp"
}

if [ "${1:-}" != "--check" ]; then
  for target in "${targets[@]}"; do
    read -r config out <<< "$target"
    generate "$config" "$root/$out"
  done
  exit 0
fi

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
stale=0
for target in "${targets[@]}"; do
  read -r config out <<< "$target"
  generate "$config" "$scratch/$out"
  if ! diff -u "$root/$out" "$scratch/$out"; then
    stale=1
  fi
done
if [ "$stale" -ne 0 ]; then
  echo "generated client API code is stale; run 'make generate' and commit the result" >&2
  exit 1
fi
echo "generated client API code is up to date"
