# Commands

The `make` targets of the repository and the toolchain they use (`make help` lists the targets too).
Coding agents run the workflow steps through agentkit ([tools/agentkit/README.md](../../tools/agentkit/README.md)).

| Command | Purpose |
| --- | --- |
| `make check` [`CHECKS=...`] [`CHECK_JOBS=N`] | Every offline check CI runs, one target per context, the contexts in parallel (each context's steps serially; output grouped per context, then a summary): `docs-check`, `go-check`, `i18n-check`, `api-check`, `ui-check`, `web-check`, `docker-check`, `agentkit-check` |
| `make check-changed` [`BASE=origin/main`] | `make check` of only the contexts changed since the merge base with `BASE` (working tree and untracked files included) and of their dependents (`scripts/changed-contexts.sh`); shared configuration (`Makefile`, `.tool-versions`, workflows, lockfiles, `api/`, `locales/`, `tools/i18n/`) or an unknown path selects every context. CI skips the jobs of the other contexts the same way |
| `make docs-check` / `docker-check` | markdownlint and the ADR index check / the Dockerfile image version check (Go, Node.js, Bun) |
| `make agentkit` / `agentkit-check` | Build `bin/agentkit`, the context and workflow tools for coding agents ([tools/agentkit/README.md](../../tools/agentkit/README.md); `scripts/agentkit.sh` builds it on demand) / check `tools/agentkit` (tidy, vet, lint, tests, dependencies within `go.sum`) and that `tools/agentkit/context-map.yaml` covers every path |
| `make go-check` | `go mod tidy` check, `go vet`, `golangci-lint`, `go test`, `CGO_ENABLED=0 go build`, the build-tag matrix, license check |
| `make go-lint` / `go-test` / `go-build` / `go-build-tags` | Run one Go step (`go-build-tags` builds and vets with the representative sets of the build tags that leave adapters out, [docs/reference/build-tags.md](../reference/build-tags.md); `go-lint` uses `.golangci.yml`, including the depguard architecture rules; `go-test` runs the PostgreSQL tests on pgmem, without Docker, and the Valkey tests on a `valkey/valkey` container, skipped without Docker) |
| `make go-test-postgres` | Run the PostgreSQL tests on a real `postgres:18-alpine` container through testcontainers-go instead of pgmem; needs Docker (CI job `postgres-contract` on the pushes to main that touch the database code, queries, or schema, and nightly, [ADR-0041](../adr/0041-postgres-tests-on-pgmem.md)) |
| `make go-test-stream` | Run the `stream.Bus` contracts of `internal/adapter/stream` on broker containers (`valkey/valkey` through `pkg/testkit/valkeytest`, the Floci AWS emulator through `pkg/testkit/awstest`, the Pub/Sub emulator) instead of the in-process fakes (miniredis, the SNS / SQS fake of `pkg/testkit/awstest`, pstest) that `go-test` uses; needs Docker (CI job `stream-contract` on pushes to main and nightly, [ADR-0050](../adr/0050-stream-pubsub-across-processes.md)) |
| `make build [VERSION=...] [GO_TAGS=...]` | Build `bin/my` with the web client embedded (`make web-build` first) and the version stamped in (`git describe` by default); `GO_TAGS` leaves adapters out ([build tags](../reference/build-tags.md)) |
| `make sqlc` / `sqlc-check` | Regenerate the sqlc code from `sqlc.yaml` / fail when the committed code is stale (`sqlc-check` is part of `make check`) |
| `make generate` / `generate-check` / `api-lint` / `api-spec-check` | Bundle the split spec `api/spec/` (the source of truth, one file per path and per component) into the generated `api/openapi.yaml` with Redocly CLI and regenerate the client API code from the bundle / fail when the bundle or the code is stale / lint `api/spec/` with Redocly CLI (`api/redocly.yaml`) / fail when the root `api/spec/openapi.yaml` does not list every file of `api/spec/` ([ADR-0066](../adr/0066-openapi-split-and-bundle.md)); `make api-check` runs the last three and is part of `make check`. Edit `api/spec/`, never `api/openapi.yaml`. Follow [docs/architecture/client-api.md](../architecture/client-api.md) to add an endpoint |
| `make emoji-table` | Regenerate `internal/domain/content/emoji_table.gen.go`, the Unicode emoji that emoji reactions accept and normalize, from the `emoji-test.txt` of `EMOJI_VERSION` (downloaded and checked against `EMOJI_TEST_SHA256` in the `Makefile`; ADR-0052) |
| `make config-docs` | Regenerate [docs/reference/configuration.md](../reference/configuration.md) from the config schema (`go-test` fails when it is stale) |
| `make go-licenses` | Fail on Go dependency licenses incompatible with AGPL-3.0 (allowlist in `Makefile`) |
| `make lint-shell` | shellcheck on `scripts/*.sh` |
| `make adr-new TITLE="..." [SLUG=...]` | Create a new Proposed ADR (pass `SLUG` when the title is not ASCII) |
| `make adr-index` | Regenerate the ADR table in `docs/adr/README.md` |
| `make lint-docs` | markdownlint on all Markdown files |
| `make db-apply` / `db-diff` / `db-reset` [`DB_DIALECT=sqlite\|postgres`] | Apply / preview / reset-and-apply `schema/<dialect>.sql` with Atlas (override `DB_URL`, `DB_DEV_URL`) |
| `make db-check` [`DB_DIALECT=...`] | Reset the DB, apply the schema, and fail if a diff remains (CI runs it for both dialects) |
| `make db-up` / `db-down` | Start / remove the dev PostgreSQL container (`my` and dev DB `my_dev`, bootstrapped) |
| `make docker-build` [`VERSION=...`] | Build the server image `my:<version>` (`Dockerfile` target `app`; target `schema` is the Atlas image) |
| `make docker-smoke` | Build and start `deploy/docker/compose.yaml`, create an admin, check WebFinger / Actor / NodeInfo, tear down (CI job `docker-smoke`: pull requests that touch the docker context, `cmd/`, or `internal/platform/`, pushes to main that change more than docs, and nightly) |
| `make federation-live` [`FEDERATION_LIVE_FLAGS=...`] | Live interoperability scenarios against Mastodon, GoToSocial, and Misskey in `deploy/docker/federation-live` (private CA, container DNS); logs and captured activities go to `.tmp/federation-live/out`. Not part of `make check` (CI job `federation-live`, nightly) |
| `make i18n-gen` | Regenerate pseudo-locales, Go / TS typed accessors, and parity fixtures from `locales/` (commit the result) |
| `make i18n-check` | Catalog lint (ICU syntax, placeholders, plural categories, shared subset), stale generated files, npm licenses, TS runtime tests |
| `make ui-check` | `app/ui`: `ui-typecheck` (incremental tsc) and `ui-unit-test` (Vitest in jsdom). A change to `app/ui` also runs `web-check` |
| `make web-check` | `app/web` and the workspace-wide client checks: `web-typecheck` (incremental tsc), `web-lint`, `web-dist` (Vite build into `app/web/dist`; fails when React Compiler compiled nothing or skipped a function that does not opt out; `make web-build` also copies it to `internal/interface/web/dist` for embedding), `web-routes-check` (the committed TanStack Router route tree is up to date), `web-licenses` (npm licenses), `web-tokens` (design token contrast and color / motion usage, [app/ui/README.md](../../app/ui/README.md)), `web-test` (Vitest in jsdom with Testing Library and MSW), `playwright-image-check` (every Playwright image and package version equals `PLAYWRIGHT_IMAGE`) |
| `make web-lint` | Oxlint (`app/.oxlintrc.json`, enforcing the 強制 column of [web-client.md](../architecture/web-client.md), [ADR-0049](../adr/0049-client-lint-oxlint.md)) with warnings as errors, the tests of its restrictions and of a violation per 強制 row (`app/oxlint`, `node --test`), knip, and a no-`100vh` check on CSS |
| `make web-e2e` [`E2E_MATRIX=full`] | Build `app/web` and `bin/my` and run the Playwright E2E (axe, horizontal scroll, ARIA snapshots) against `vite preview`, and for the server-rendered sign-in / sign-up / consent pages against a real `my serve` on SQLite (`app/web/e2e/server.ts`): the default subset of the width × locale matrix, or all of it (CI workflow `web-e2e.yml` on pushes to main only, one build tested by 4 Playwright shards, ADR-0040 / ADR-0047 — in single-issue work, run it locally before a PR that touches the web client, or on the PR's branch with `gh workflow run web-e2e.yml --ref <branch>`; nightly runs `full` on 8 shards). `make web-e2e-install` downloads the browser for local runs |
| `make web-install` | Install the client workspace dependencies from `app/bun.lock` |
| `make storybook` / `storybook-build` | Start / build the `@my/ui` Storybook (locale, theme and instance theme toolbars) |
| `make ui-test` | Run every `@my/ui` story in Chromium: axe violations and screenshots at 3 container widths × light / dark × LTR / RTL (`Foundations/Theme sample` shows representative components under the sample server theme). Needs a browser, so not in `make check`; CI job `ui-test` runs it on pull requests and pushes to main that touch `app/ui` (not in the merge queue), in the pinned Playwright image (`PLAYWRIGHT_IMAGE` in the `Makefile`). Screenshots match only there: use `make ui-test-docker` locally |
| `make ui-snapshots-update` | Regenerate the `@my/ui` screenshot baselines in the pinned Playwright image (Docker); never commit screenshots made outside it |

Toolchain versions (Go, golangci-lint, Bun, shellcheck, Node.js) are pinned in `.tool-versions` (asdf / mise).
Atlas Community Edition is pinned and checksum-verified in `scripts/install-atlas.sh` (`make atlas` installs `bin/atlas`).
Go tools (go-licenses, sqlc, oapi-codegen) are pinned with `tool` directives in `tools/go.mod` and run via
`go tool -modfile=tools/go.mod <tool>`.
The i18n generator and checks are a Bun package in `tools/i18n` with exact versions in `package.json` and a committed `bun.lock`.
The clients live in the Bun workspace `app/` (`app/ui` = `@my/ui`, shipped as TypeScript source; `app/web`) with exact versions,
one committed `app/bun.lock`, and `app/bunfig.toml` (isolated installs, no version younger than two weeks).
Bun installs packages and runs scripts; Vite runs on Node.js — never add `--bun` ([ADR-0025](../adr/0025-web-client-stack.md)).
