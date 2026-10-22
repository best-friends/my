#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# Fails when tools/agentkit uses a module version the server module does not: agentkit's dependencies are
# limited to modules whose licenses `make go-licenses` already checks (tools/agentkit/docs/adr/0001-agentkit.md).
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
modules() { grep -v '/go.mod ' "$1" | awk '{ print $1, $2 }' | sort -u; }
missing="$(comm -23 <(modules "$root/tools/agentkit/go.sum") <(modules "$root/go.sum"))"
if [ -n "$missing" ]; then
  echo "tools/agentkit uses module versions go.sum does not have (keep to the server's versions, or add the module to the server first):" >&2
  echo "$missing" >&2
  exit 1
fi
