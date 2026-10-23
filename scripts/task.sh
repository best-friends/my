#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# Runs go-task (ADR-0080) on Taskfile.yml with the version pinned in tools/go.mod, building bin/task first when it is
# missing or was built from another tools/go.mod / tools/go.sum (bin/task.sum holds their checksum, which survives a
# checkout or a CI cache restore, unlike modification times). Never the `task` on PATH: Taskwarrior's command has the
# same name, and another go-task may be another version. `scripts/task.sh --list` lists the tasks.
#
# The exit code is the failed command's own (--exit-code), not go-task's 201. Locally, TASK_CONCURRENCY defaults to
# half the CPUs, since several agents run checks on one machine; set it to override. In CI (CI set) the concurrency
# is unlimited and the output is grouped by task, showing only the tasks that failed.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$root/bin/task"
sum="$(cat "$root/tools/go.mod" "$root/tools/go.sum" | sha256sum)"

if [ ! -x "$bin" ] || [ "$(cat "$bin.sum" 2>/dev/null)" != "$sum" ]; then
  (cd "$root/tools" && CGO_ENABLED=0 go build -o "$bin" github.com/go-task/task/v3/cmd/task) >&2
  printf '%s\n' "$sum" > "$bin.sum"
fi

flags=(--exit-code --dir "$root")
if [ -n "${CI:-}" ]; then
  flags+=(--output group --output-group-error-only)
elif [ -z "${TASK_CONCURRENCY:-}" ]; then
  cpus="$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 2)"
  TASK_CONCURRENCY=$((cpus / 2))
  [ "$TASK_CONCURRENCY" -ge 1 ] || TASK_CONCURRENCY=1
  export TASK_CONCURRENCY
fi

exec "$bin" "${flags[@]}" "$@"
