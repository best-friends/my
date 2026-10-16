#!/bin/sh
# Runs the notifications tests repeatedly under CPU load to reproduce timing failures.
cd "$(dirname "$0")/../app/web" || exit 1
pids=""
for j in $(seq 1 "${LOAD:-24}"); do
  timeout 900 sh -c 'while :; do :; done' &
  pids="$pids $!"
done
for i in $(seq 1 "${RUNS:-10}"); do
  CI=true bunx vitest run ${FILES:-src/features/notifications} 2>&1 | grep -E "Tests |×|ECONNREFUSED" | head -3
done
kill $pids 2>/dev/null
echo done
