#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
# Live interoperability test (ADR-0006, ADR-0015, docs/architecture/federation.md §12): start
# deploy/docker/federation-live/compose.yaml (this server, Mastodon, GoToSocial, a private CA), create one user
# on each server, and run the scenarios in deploy/docker/federation-live (go test -tags federationlive).
#
# Whatever the outcome, it writes to $FEDERATION_LIVE_OUT (default .tmp/federation-live/out):
#   logs/<service>.log   the log of every container
#   captured/<host>/…    every activity this server received, as stored, for new fixtures (README.md)
#   scenarios.log        the go test output
# and removes the stack, unless FEDERATION_LIVE_KEEP=1.
#
# Usage: scripts/federation-live.sh [go test flags, e.g. -run TestFollow]
#   (needs docker with compose, go, curl, jq, openssl)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dir="$root/deploy/docker/federation-live"
work="${FEDERATION_LIVE_DIR:-$root/.tmp/federation-live}"
out="${FEDERATION_LIVE_OUT:-$work/out}"
certs="$work/certs"
port="${FEDERATION_LIVE_HTTPS_PORT:-18443}"
db_port="${FEDERATION_LIVE_DB_PORT:-15432}"

log() { echo "federation-live: $*" >&2; }
fail() {
	log "$*"
	exit 1
}

rm -rf "$work" "$out"
mkdir -p "$certs" "$out/logs" "$out/captured"

# --- Private CA and the certificate of every host -----------------------------------------------
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 2 \
	-subj "/CN=my federation-live CA" -keyout "$certs/ca.key" -out "$certs/ca.pem" 2>/dev/null
openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
	-subj "/CN=my.test" -keyout "$certs/server.key" -out "$certs/server.csr" 2>/dev/null
printf '%s\n' \
	'basicConstraints=critical,CA:FALSE' \
	'keyUsage=critical,digitalSignature' \
	'extendedKeyUsage=serverAuth' \
	'subjectAltName=DNS:my.test,DNS:mastodon.test,DNS:gts.test' >"$certs/server.ext"
openssl x509 -req -in "$certs/server.csr" -CA "$certs/ca.pem" -CAkey "$certs/ca.key" -CAcreateserial \
	-days 2 -extfile "$certs/server.ext" -out "$certs/server.pem" 2>/dev/null
# Throwaway keys, read by containers that run as other users.
chmod 0644 "$certs"/*

# --- Secrets of this run only -------------------------------------------------------------------
rand() { openssl rand -hex "$1"; }
my_password="$(rand 16)"
gts_password="Aa1!$(rand 16)"
export FEDERATION_LIVE_CERTS="$certs"
export FEDERATION_LIVE_HTTPS_PORT="$port"
export FEDERATION_LIVE_DB_PORT="$db_port"
export MY_VERSION="${MY_VERSION:-federation-live}"
MY_KEK="$(openssl rand -base64 32)"
MASTODON_SECRET_KEY_BASE="$(rand 64)"
MASTODON_OTP_SECRET="$(rand 64)"
MASTODON_AR_DETERMINISTIC_KEY="$(rand 16)"
MASTODON_AR_KEY_DERIVATION_SALT="$(rand 16)"
MASTODON_AR_PRIMARY_KEY="$(rand 16)"
export MY_KEK MASTODON_SECRET_KEY_BASE MASTODON_OTP_SECRET MASTODON_AR_DETERMINISTIC_KEY \
	MASTODON_AR_KEY_DERIVATION_SALT MASTODON_AR_PRIMARY_KEY

compose=(docker compose --project-directory "$dir" -f "$dir/compose.yaml" -p my-federation-live)

collect() {
	"${compose[@]}" ps -a >"$out/logs/ps.txt" 2>&1 || true
	for service in $("${compose[@]}" config --services); do
		"${compose[@]}" logs --no-color --timestamps "$service" >"$out/logs/$service.log" 2>&1 || true
	done
	# Every received activity as stored (received_activity_documents), one file per activity under the
	# sender's host, for adding fixtures (README.md, pkg/testkit/README.md "Federation fixtures").
	"${compose[@]}" exec -T db psql -U postgres -d my -AtX -c "
		SELECT json_build_object('actor', a.actor_uri, 'type', a.type, 'document', d.document)
		FROM received_activities a JOIN received_activity_documents d USING (activity_uri)
		ORDER BY a.received_at, a.activity_uri" 2>/dev/null |
		{
			n=0
			while IFS= read -r row; do
				n=$((n + 1))
				host="$(jq -r '.actor | sub("^https://"; "") | sub("/.*$"; "")' <<<"$row")"
				name="$(jq -r '.type | ascii_downcase' <<<"$row")"
				mkdir -p "$out/captured/$host"
				jq --indent 2 '.document' <<<"$row" >"$out/captured/$host/$(printf '%03d' "$n")-$name.json"
			done
		} || true
}

cleanup() {
	status=$?
	collect
	if [[ $status -ne 0 ]]; then
		log "failed; the logs of every server are in $out/logs"
		"${compose[@]}" ps -a >&2 || true
	fi
	if [[ "${FEDERATION_LIVE_KEEP:-0}" == 1 ]]; then
		log "keeping the stack (FEDERATION_LIVE_KEEP=1); remove it with: docker compose -p my-federation-live down -v"
	else
		"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
	fi
	exit "$status"
}
trap cleanup EXIT

log "starting the stack"
"${compose[@]}" up -d --build --quiet-pull

# Reach a server through the proxy as the other servers do: by its host name, over TLS with the private CA.
https() {
	local host="$1" path="$2"
	shift 2
	curl -fsS --cacert "$certs/ca.pem" --connect-to "$host:443:127.0.0.1:$port" "$@" "https://$host$path"
}

wait_ready() {
	local host="$1" path="$2"
	log "waiting for https://$host$path"
	for i in $(seq 300); do
		if https "$host" "$path" -o /dev/null 2>/dev/null; then
			return 0
		fi
		if [[ $i -eq 300 ]]; then
			fail "https://$host$path did not answer within 300 s"
		fi
		sleep 1
	done
}
wait_ready my.test /readyz
wait_ready mastodon.test /health
wait_ready gts.test /readyz

# --- One user on each server --------------------------------------------------------------------
log "creating alice@my.test, bob@mastodon.test, and carol@gts.test"
printf '%s\n' "$my_password" | "${compose[@]}" exec -T my my admin create -username alice -email alice@my.test >/dev/null

"${compose[@]}" exec -T mastodon-web bin/tootctl accounts create bob --email bob@mastodon.test --confirmed --approve >/dev/null
mastodon_token="$("${compose[@]}" exec -T mastodon-web bin/rails runner '
	user = Account.find_local!("bob").user
	app = Doorkeeper::Application.create!(name: "federation-live", redirect_uri: "urn:ietf:wg:oauth:2.0:oob", scopes: "read write follow")
	token = Doorkeeper::AccessToken.create!(application: app, resource_owner_id: user.id, scopes: "read write follow")
	puts "token=#{token.plaintext_token}"
' | sed -n 's/^token=//p')"
[[ -n "$mastodon_token" ]] || fail "could not issue an access token for bob@mastodon.test"

gts=(exec -T gotosocial /gotosocial/gotosocial admin account)
"${compose[@]}" "${gts[@]}" create --username carol --email carol@gts.test --password "$gts_password" >/dev/null
"${compose[@]}" "${gts[@]}" confirm --username carol >/dev/null

# --- Scenarios ----------------------------------------------------------------------------------
log "running the scenarios"
# Also written to $work/scenarios.env, to rerun the scenarios against a stack kept with FEDERATION_LIVE_KEEP=1
# (README.md).
export FEDERATION_LIVE_ADDR="127.0.0.1:$port"
export FEDERATION_LIVE_CA="$certs/ca.pem"
export FEDERATION_LIVE_MY_PASSWORD="$my_password"
export FEDERATION_LIVE_MASTODON_TOKEN="$mastodon_token"
export FEDERATION_LIVE_GTS_PASSWORD="$gts_password"
env | grep '^FEDERATION_LIVE_' >"$work/scenarios.env"
cd "$root"
go test -tags federationlive -count=1 -v "$@" ./deploy/docker/federation-live 2>&1 | tee "$out/scenarios.log"
