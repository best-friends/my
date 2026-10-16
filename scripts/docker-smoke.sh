#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Smoke test of deploy/docker/compose.yaml: build the images, start the stack (schema apply, then the server),
# wait for readiness, create an admin, and check WebFinger, the Actor, and NodeInfo.
# Prints the logs on failure and always removes the stack and its volume.
# Usage: scripts/docker-smoke.sh   (needs docker with compose, curl, jq, openssl)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
port="${SMOKE_HTTP_PORT:-18080}"
base="http://127.0.0.1:$port"

# Throwaway settings for this run only; the shell environment takes precedence over any deploy/docker/.env.
export MY_HOST="localhost:$port"
export MY_HTTP_PORT="$port"
export MY_VERSION="${MY_VERSION:-smoke}"
MY_KEK="$(openssl rand -base64 32)"
export MY_KEK

compose=(docker compose --project-directory "$root/deploy/docker" -f "$root/deploy/docker/compose.yaml" -p my-smoke)

fail() {
	echo "docker-smoke: $*" >&2
	exit 1
}

cleanup() {
	status=$?
	if [[ $status -ne 0 ]]; then
		echo "docker-smoke: failed; container status and logs follow" >&2
		"${compose[@]}" ps -a >&2 || true
		"${compose[@]}" logs --no-color >&2 || true
	fi
	"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
	exit "$status"
}
trap cleanup EXIT

"${compose[@]}" up -d --build

echo "docker-smoke: waiting for $base/readyz" >&2
for i in $(seq 60); do
	if curl -fsS "$base/readyz" >/dev/null 2>&1; then
		break
	fi
	[[ $i -eq 60 ]] && fail "/readyz did not return 200 within 60 s"
	sleep 1
done

"${compose[@]}" exec -T my my version
printf 'smoke-test-password\n' | "${compose[@]}" exec -T my my admin create -username alice -email alice@example.com

accept_ap='application/activity+json'
webfinger="$(curl -fsS "$base/.well-known/webfinger?resource=acct:alice@$MY_HOST")"
echo "$webfinger" | jq -e --arg subject "acct:alice@$MY_HOST" '.subject == $subject' >/dev/null ||
	fail "WebFinger subject is not acct:alice@$MY_HOST: $webfinger"
actor_id="$(echo "$webfinger" | jq -er '.links[] | select(.rel == "self" and .type == "application/activity+json") | .href')" ||
	fail "WebFinger has no ActivityPub self link: $webfinger"
[[ "$actor_id" == "https://$MY_HOST/ap/actors/"* ]] || fail "unexpected Actor id in WebFinger: $actor_id"

# The id is https://{host}/...; fetch it from the published port over plain HTTP.
actor="$(curl -fsS -H "Accept: $accept_ap" "$base${actor_id#"https://$MY_HOST"}")"
echo "$actor" | jq -e --arg id "$actor_id" '
	.id == $id and .type == "Person" and .preferredUsername == "alice"
	and (.inbox | startswith($id)) and (.outbox | startswith($id))
	and .publicKey.id == ($id + "#main-key") and .publicKey.owner == $id
	and (.publicKey.publicKeyPem | startswith("-----BEGIN PUBLIC KEY-----"))
' >/dev/null || fail "unexpected Actor: $actor"

nodeinfo="$(curl -fsS "$base/nodeinfo/2.1")"
echo "$nodeinfo" | jq -e --arg v "$MY_VERSION" '.version == "2.1" and .software.version == $v and .usage.users.total == 1' >/dev/null ||
	fail "unexpected NodeInfo: $nodeinfo"

echo "docker-smoke: ok ($actor_id)" >&2
