#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# The Claude Code hooks of .claude/settings.json (tools/agentkit/docs/adr/0004-claude-code-hooks.md):
#   agentkit-hook.sh pre | post      PreToolUse / PostToolUse: `bin/agentkit hook <event>` with the hook JSON on
#                                    stdin, or nothing when bin/agentkit is not built, so a hook never blocks a
#                                    tool call by failing
#   agentkit-hook.sh session-start   build bin/agentkit and bring the index up to date, quietly
set -uo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
case "${1:-}" in
  session-start)
    "$root/scripts/agentkit.sh" index > /dev/null 2>&1 || true
    ;;
  pre | post)
    [ -x "$root/bin/agentkit" ] || exit 0
    exec "$root/bin/agentkit" hook "$1"
    ;;
  *)
    echo "usage: scripts/agentkit-hook.sh pre | post | session-start" >&2
    exit 2
    ;;
esac
