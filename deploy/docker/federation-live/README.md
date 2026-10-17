# Live interoperability test

This server, Mastodon, and GoToSocial federating with each other over HTTPS, for the live test of
[federation.md §12](../../../docs/architecture/federation.md#12-テスト) ([ADR-0006](../../../docs/adr/0006-federation-protocol-scope.md),
[ADR-0015](../../../docs/adr/0015-testing-strategy.md)). CI runs it nightly (job `federation-live` in
`.github/workflows/nightly.yml`); it is not part of `make check`.

| File | What it is |
| --- | --- |
| `compose.yaml` | The stack: the TLS proxy, PostgreSQL (this server and Mastodon), Redis, this server built from the checkout, Mastodon (web and Sidekiq), and GoToSocial |
| `Caddyfile` | The proxy: one certificate of the private CA for `my.test`, `mastodon.test`, and `gts.test`, forwarded by host name |
| `postgres-init.sh` | The role and databases of this server, prepared with `schema/postgres.bootstrap.sql` |
| `*_test.go` | The scenarios (`go test -tags federationlive`), black-box clients of the three client APIs |
| `../../../scripts/federation-live.sh` | Generates the CA and the secrets, starts the stack, creates the users, runs the scenarios, collects the logs, and removes the stack |

## How it works

- **TLS**: the script creates a private CA and one certificate for the three hosts. Every server trusts only that CA
  (`SSL_CERT_FILE`).
- **DNS**: the proxy holds the network aliases `my.test`, `mastodon.test`, and `gts.test`, so each server reaches
  the others, and itself, through the proxy, as on the internet. The servers may reach the private addresses of the
  compose network only (`MY_HTTP_ALLOW_PRIVATE_NETWORKS` in the `development` profile, Mastodon's
  `ALLOWED_PRIVATE_ADDRESSES`, GoToSocial's `http-client.allow-ips`).
- **Users**: `alice@my.test`, `bob@mastodon.test`, and `carol@gts.test`, created with each server's admin CLI.
  The scenarios sign in to this server and GoToSocial through their OAuth screens; the script issues bob's token
  with a Rails runner.
- **Mastodon** runs in secure mode (`AUTHORIZED_FETCH`), so every fetch this server makes from it must be signed.
- The scenarios reach the servers through the proxy's published port and read what this server recorded
  (`received_activities`) from the published PostgreSQL port.

## Running it

```sh
make federation-live                                    # every scenario that is not pending
make federation-live FEDERATION_LIVE_FLAGS='-run TestFollowFromPeer'
FEDERATION_LIVE_RUN_PENDING=1 make federation-live      # also the scenarios that wait for unfinished issues
FEDERATION_LIVE_KEEP=1 make federation-live             # keep the stack running afterwards
```

It needs Docker with Compose, Go, curl, jq, and openssl. The proxy publishes port 18443 and PostgreSQL port 15432 on
127.0.0.1 (`FEDERATION_LIVE_HTTPS_PORT`, `FEDERATION_LIVE_DB_PORT`); the compose network is `172.29.58.0/24`
(`FEDERATION_LIVE_SUBNET`). The output goes to `.tmp/federation-live/out` (`FEDERATION_LIVE_OUT`):

| Path | Content |
| --- | --- |
| `logs/<service>.log`, `logs/ps.txt` | The log of every container (the proxy's access log has every request's headers) and their final state |
| `captured/<host>/<nnn>-<type>.json` | Every activity this server received, as it stored it, by the sender's host |
| `scenarios.log` | The `go test -v` output |

With `FEDERATION_LIVE_KEEP=1`, rerun the scenarios against the running stack with the variables the script wrote,
and remove the stack when done:

```sh
set -a; . .tmp/federation-live/scenarios.env; set +a
go test -tags federationlive -count=1 -v ./deploy/docker/federation-live
docker compose -p my-federation-live down -v
```

In the browser, the servers are at `https://my.test:18443` and so on once the host names point at 127.0.0.1 and the
CA (`.tmp/federation-live/certs/ca.pem`) is trusted.

## Scenarios

Each scenario runs against both peers. A scenario whose server side is not built yet calls `pending` with the issues
it waits for and is skipped unless `FEDERATION_LIVE_RUN_PENDING=1`; when those issues land, remove the call in the
same PR. The scenarios that are only named (`t.Skip("not written yet: …")`) are written when their issues land.

| Scenario | Checks |
| --- | --- |
| `TestPeerResolvesLocal` | The peer finds `alice@my.test` through WebFinger and fetches her actor |
| `TestLocalResolvesPeer` | This server finds the peer user through WebFinger and fetches the actor with a signed GET |
| `TestFollowFromPeer` | The peer's signed `Follow` reaches this server's inbox and is verified and recorded; then it is accepted |
| `TestFollowFromLocal` | alice follows the peer user and the peer accepts |
| `TestPostFromPeer` / `TestPostFromLocal` | A public post reaches the follower's home timeline on the other side |
| `TestReply`, `TestBoost`, `TestFavourite`, `TestEdit`, `TestDelete`, `TestVisibilityAudiences` | Named only |

## Adding a captured activity as a fixture

The activities under `captured/` (also in the `federation-live` artifact of every CI run) are real documents of the
pinned Mastodon and GoToSocial versions, so they can become fixtures of the regression tests
([pkg/testkit/README.md](../../../pkg/testkit/README.md#federation-fixtures)):

1. Pick the document, for example `captured/mastodon.test/001-follow.json`.
2. Follow "Adding a fixture" in `pkg/testkit/README.md`: replace the hosts (`mastodon.test` → `mastodon.example`,
   `my.test` → `remote.example`, `gts.test` → `gotosocial.example`), the keys, and the texts, and save it as
   `pkg/testkit/fixtures/federation/<implementation>/<version>/<name>.json` with the version of the image in
   `compose.yaml` (`mastodon/4.7/follow.json`).
3. Record it in `pkg/testkit/fixtures/federation/<implementation>/README.md` as a capture from that server version, with
   the capture date, and update the golden files.

The stored document is the one this server read: PostgreSQL's `jsonb` does not keep the key order and the whitespace of
the request body.

## Updating the peers

The images are pinned by tag and digest in `compose.yaml`. To move to a new Mastodon or GoToSocial release, take a tag
at least two weeks old, put its multi-arch digest (`docker buildx imagetools inspect <image>:<tag>`) next to it, and
run `FEDERATION_LIVE_RUN_PENDING=1 make federation-live` to see what changed.
