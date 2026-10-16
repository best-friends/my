#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Build each cmd/size-* with CGO_ENABLED=0 and print stripped binary sizes and build times.
set -euo pipefail
cd "$(dirname "$0")"
out="${1:-$(mktemp -d)}"
for n in baseline modernc ncruces pgx; do
  start=$(date +%s.%N)
  CGO_ENABLED=0 go build -ldflags="-s -w" -o "$out/size-$n" "./cmd/size-$n"
  end=$(date +%s.%N)
  size=$(stat -c %s "$out/size-$n")
  printf '%-8s %6.1f MiB  build %.1fs\n' "$n" "$(echo "$size / 1048576" | bc -l)" "$(echo "$end - $start" | bc -l)"
done
"$out/size-modernc"
"$out/size-ncruces"
