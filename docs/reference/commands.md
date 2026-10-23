# Commands

The tasks of the repository and the toolchain they use. Tasks run with go-task ([ADR-0080](../adr/0080-task-runner-go-task.md))
through `scripts/task.sh <task> [VAR=value ...]`, which builds `bin/task` from the version pinned in `tools/go.mod` and never uses
a `task` on PATH (Taskwarrior's command has the same name). `scripts/task.sh --list` lists every task with its description.
Coding agents run the workflow steps through agentkit ([tools/agentkit/README.md](../../tools/agentkit/README.md)).

## How tasks run

- `Taskfile.yml` holds the includes, `check`, `check:changed`, `generate`, and the CI jobs `ci:*`. Each area keeps its tasks in
  `tasks/<area>.yml`, named `<area>:<task>` (`go:test`, `web:check`). A new area is a new file there and one line under `includes`.
- The `deps` of a task run in parallel, also inside a context (`go:vet`, `go:lint`, and `go:test` at once). A failed task does not stop
  the others, so one run shows every failure; the exit code is the failed command's own.
- Locally `scripts/task.sh` limits the parallel tasks to half the CPUs (`TASK_CONCURRENCY=N` overrides it), since several agents
  check on one machine, and prefixes each line with its task's name. In CI (`CI` set) the concurrency is unlimited and the output
  is grouped by task, showing only the tasks that failed.
- Generators declare their inputs and outputs (`sources` / `generates`) and skip when the checksums of their inputs have not changed
  since their last run (kept in `.task/`). `--force` runs a task anyway.
- Variables are arguments (`scripts/task.sh go:bin VERSION=v1.2.3`) or environment variables.

## Checks across areas

| Task | Purpose |
| --- | --- |
| `check` [`CHECKS="go web"`] | Every offline check, one `<context>:check` per context, the contexts and the checks inside each in parallel: `docs:check`, `go:check`, `i18n:check`, `api:check`, `ui:check`, `web:check`, `docker:check`, `agentkit:check` |
| `check:changed` [`BASE=origin/main`] | `check` of only the contexts changed since the merge base with `BASE` (working tree and untracked files included) and of their dependents (`scripts/changed-contexts.sh`); shared configuration (`Taskfile.yml`, `tasks/`, `.tool-versions`, workflows, lockfiles, `api/`, `locales/`, `tools/i18n/`) or an unknown path selects every context. It names `shell:check` and `db:check-all` when the shell scripts or the schema changed. CI skips the jobs of the other contexts the same way |
| `generate` | Every generator whose inputs changed since its last run: `api:generate`, `go:sqlc`, `go:config-docs`, `i18n:gen`, `web:tokens-gen` |
| `ci:preflight` / `ci:server-static` / `ci:server-test` / `ci:client-static` / `ci:client-test` [`SHARD=1/2`] / `ci:repo` / `ci:docker` | The CI jobs of [ADR-0079](../adr/0079-ci-single-gate-merge-queue.md), each a set of the area tasks below in parallel: the checks a pull request's own files decide (markdownlint, shellcheck, gofmt, licenses, image versions, tokens, no `100vh`) / tidy, vet, golangci-lint, the builds, sqlc and the client API code, the i18n catalogs, `db:check-all` / `go:test` / tsc, Oxlint, knip, the Vite build and route tree, the i18n catalogs / Vitest of `app/web` (`SHARD` runs one shard; the first or an unsharded run also runs `app/ui` and `tools/i18n`) / agentkit and the ADR index / the Dockerfile check and the Docker smoke test |

## Areas

| Task | Purpose |
| --- | --- |
| `docs:check` | `docs:lint` (markdownlint on all Markdown files) and `docs:adr-index-check` (the ADR index is up to date) |
| `docs:adr-new TITLE="..." [SLUG=...]` | Create a new Proposed ADR (pass `SLUG` when the title is not ASCII) |
| `docs:adr-index` | Regenerate the ADR table in `docs/adr/README.md` |
| `shell:check` / `shell:lint` | shellcheck on `scripts/*.sh` |
| `agentkit:build` / `agentkit:check` | Build `bin/agentkit`, the context and workflow tools for coding agents ([tools/agentkit/README.md](../../tools/agentkit/README.md); `scripts/agentkit.sh` builds it on demand) / check `tools/agentkit` (tidy, vet, lint, tests, dependencies within `go.sum`) and that `tools/agentkit/context-map.yaml` covers every path (`agentkit:map-check`) |
| `go:check` | `go:mod-check` (`go mod tidy -diff` of `go.mod` and `tools/go.mod`), `go:vet`, `go:lint`, `go:test`, `go:build` (`CGO_ENABLED=0 go build ./...`), `go:build-tags`, `go:licenses`, `go:sqlc-check` |
| `go:lint` / `go:test` / `go:build-tags` / `go:fmt-check` | Run one Go step (`go:lint` uses `.golangci.yml`, including the depguard architecture rules; `go:test` runs the PostgreSQL tests on pgmem, without Docker, and the Valkey tests on a `valkey/valkey` container, skipped without Docker; `go:build-tags` builds and vets with the representative sets of the build tags that leave adapters out, [docs/reference/build-tags.md](../reference/build-tags.md); `go:fmt-check` fails on a file `gofmt` would change) |
| `go:test-postgres` | Run the PostgreSQL tests on a real `postgres:18-alpine` container through testcontainers-go instead of pgmem; needs Docker (CI job `postgres-contract` on the pushes to main that touch the database code, queries, or schema, and nightly, [ADR-0041](../adr/0041-postgres-tests-on-pgmem.md)) |
| `go:test-stream` | Run the `stream.Bus` contracts of `internal/adapter/stream` on broker containers (`valkey/valkey` through `pkg/testkit/valkeytest`, the Floci AWS emulator through `pkg/testkit/awstest`, the Pub/Sub emulator) instead of the in-process fakes (miniredis, the SNS / SQS fake of `pkg/testkit/awstest`, pstest) that `go:test` uses; needs Docker (CI job `stream-contract` on pushes to main and nightly, [ADR-0050](../adr/0050-stream-pubsub-across-processes.md)) |
| `go:bin` [`VERSION=...`] [`GO_TAGS=...`] | Build `bin/my` with the web client embedded (runs `web:build` first) and the version stamped in (`git describe` by default); `GO_TAGS` leaves adapters out ([build tags](../reference/build-tags.md)) |
| `go:sqlc` / `go:sqlc-check` | Regenerate the sqlc code from `sqlc.yaml` / fail when the committed code is stale (`go:sqlc-check` is part of `go:check`) |
| `go:emoji-table` | Regenerate `internal/domain/content/emoji_table.gen.go`, the Unicode emoji that emoji reactions accept and normalize, from the `emoji-test.txt` of `EMOJI_VERSION` (downloaded and checked against `EMOJI_TEST_SHA256` in `tasks/go.yml`; ADR-0052) |
| `go:ai-crawlers` | Regenerate `internal/interface/web/ai_crawlers.gen.go`, the AI crawlers that `/robots.txt` names, from ai.robots.txt's `robots.json` at `AI_ROBOTS_COMMIT` (checked against `AI_ROBOTS_SHA256` in `tasks/go.yml`; ADR-0057) |
| `go:config-docs` | Regenerate [docs/reference/configuration.md](../reference/configuration.md) from the config schema (`go:test` fails when it is stale) |
| `go:licenses` | Fail on Go dependency licenses incompatible with AGPL-3.0 (allowlist `GO_ALLOWED_LICENSES` in `Taskfile.yml`) |
| `api:check` | `api:spec-check` (the root `api/spec/openapi.yaml` lists every file of `api/spec/`), `api:lint` (Redocly CLI with `api/redocly.yaml`), `api:generate-check` (the bundle and the generated code are up to date) |
| `api:generate` | Bundle the split spec `api/spec/` (the source of truth, one file per path and per component) into the generated `api/openapi.yaml` with Redocly CLI and regenerate the client API code from the bundle ([ADR-0066](../adr/0066-openapi-split-and-bundle.md)). Edit `api/spec/`, never `api/openapi.yaml`. Follow [docs/architecture/client-api.md](../architecture/client-api.md) to add an endpoint |
| `db:apply` / `db:diff` / `db:reset` [`DB_DIALECT=sqlite\|postgres`] | Apply / preview / reset-and-apply `schema/<dialect>.sql` with Atlas (override `DB_URL`, `DB_DEV_URL`) |
| `db:check` [`DB_DIALECT=...`] / `db:check-all` | Reset the DB, apply the schema, and fail if a diff remains / both dialects, PostgreSQL on the `db:up` container (CI runs it) |
| `db:up` / `db:down` | Start / remove the dev PostgreSQL container (`my` and dev DB `my_dev`, bootstrapped) |
| `db:atlas` | Install the pinned Atlas Community Edition into `bin/atlas` |
| `docker:check` | `docker:dockerfile-check`: the Dockerfile's Go, Node.js, and Bun images match `.tool-versions` |
| `docker:build` [`VERSION=...`] | Build the server image `my:<version>` (`Dockerfile` target `app`; target `schema` is the Atlas image) |
| `docker:smoke` | Build and start `deploy/docker/compose.yaml`, create an admin, check WebFinger / Actor / NodeInfo, tear down (CI job `docker-smoke`: pull requests that touch the docker context, `cmd/`, or `internal/platform/`, pushes to main that change more than docs, and nightly) |
| `docker:federation-live` [`FEDERATION_LIVE_FLAGS=...`] | Live interoperability scenarios against Mastodon, GoToSocial, and Misskey in `deploy/docker/federation-live` (private CA, container DNS); logs and captured activities go to `.tmp/federation-live/out`. Not part of `check` (CI job `federation-live`, nightly) |
| `i18n:gen` | Regenerate pseudo-locales, Go / TS typed accessors, and parity fixtures from `locales/` (commit the result) |
| `i18n:check` | `i18n:lint` (catalog lint: ICU syntax, placeholders, plural categories, shared subset; stale generated files), `i18n:licenses` (npm licenses), `i18n:test` (TS runtime tests), `i18n:typecheck` |
| `ui:check` | `app/ui`: `ui:typecheck` (incremental tsc) and `ui:unit-test` (Vitest in jsdom). A change to `app/ui` also runs `web:check` |
| `web:check` | `app/web` and the workspace-wide client checks: `web:lint-rules` first (it copies violation fixtures into `app/web/src`), then `web:static` (`web:typecheck` with incremental tsc, `web:oxlint`, `web:knip`, `web:routes-check` on the Vite build of `web:dist`, which fails when React Compiler compiled nothing or skipped a function that does not opt out), `web:lint-css`, `web:licenses` (npm licenses), `web:tokens` (design token contrast and color / motion usage, [app/ui/README.md](../../app/ui/README.md)), `web:test` (Vitest in jsdom with Testing Library and MSW), `web:playwright-image-check` (every Playwright image and package version equals `PLAYWRIGHT_IMAGE`) |
| `web:lint` | `web:lint-rules` (the tests of the Oxlint restrictions and of a violation per 強制 row, `app/oxlint`, `node --test`), then `web:oxlint` (`app/.oxlintrc.json`, enforcing the 強制 column of [web-client.md](../architecture/web-client.md), [ADR-0049](../adr/0049-client-lint-oxlint.md), warnings as errors), `web:knip`, and `web:lint-css` (no `100vh` in CSS) |
| `web:dist` / `web:build` | Build `app/web` into `app/web/dist` / also copy it to `internal/interface/web/dist` for embedding |
| `web:tokens-gen` | Regenerate the list of the tokens a server theme may override (ADR-0059) from `app/ui/src/theme.css` and `tokens.ts` |
| `web:e2e` [`E2E_MATRIX=full`] | Build `app/web` and `bin/my` and run the Playwright E2E (axe, horizontal scroll, ARIA snapshots) against `vite preview`, and for the server-rendered sign-in / sign-up / consent pages against a real `my serve` on SQLite (`app/web/e2e/server.ts`): the default subset of the width × locale matrix, or all of it (CI workflow `web-e2e.yml` on pushes to main only, one build tested by 4 Playwright shards, ADR-0040 / ADR-0047 — in single-issue work, run it locally before a PR that touches the web client, or on the PR's branch with `gh workflow run web-e2e.yml --ref <branch>`; nightly runs `full` on 8 shards). `web:e2e-install` downloads the browser for local runs |
| `web:install` / `i18n:install` | Install the client workspace dependencies from `app/bun.lock` / the i18n tool's from `tools/i18n/bun.lock` (the tasks that need them run them first, once per run) |
| `ui:storybook` / `ui:storybook-build` | Start / build the `@my/ui` Storybook (locale, theme and instance theme toolbars) |
| `ui:test` | Run every `@my/ui` story in Chromium: axe violations and screenshots at 3 container widths × light / dark × LTR / RTL (`Foundations/Theme sample` shows representative components under the sample server theme). Needs a browser, so not in `check`; CI job `ui-test` runs it on pull requests and pushes to main that touch `app/ui` (not in the merge queue), in the pinned Playwright image (`PLAYWRIGHT_IMAGE` in `Taskfile.yml`). Screenshots match only there: use `ui:test-docker` locally (`ui:test-install` downloads the browser for `ui:test`) |
| `ui:snapshots-update` | Regenerate the `@my/ui` screenshot baselines in the pinned Playwright image (Docker); never commit screenshots made outside it |

## Toolchain

Toolchain versions (Go, golangci-lint, Bun, shellcheck, Node.js) are pinned in `.tool-versions` (asdf / mise).
Atlas Community Edition is pinned and checksum-verified in `scripts/install-atlas.sh` (`scripts/task.sh db:atlas` installs `bin/atlas`).
Go tools (go-task, go-licenses, sqlc, oapi-codegen) are pinned with `tool` directives in `tools/go.mod` and run via
`go tool -modfile=tools/go.mod <tool>` (go-task as `bin/task`, built by `scripts/task.sh`).
The i18n generator and checks are a Bun package in `tools/i18n` with exact versions in `package.json` and a committed `bun.lock`.
The clients live in the Bun workspace `app/` (`app/ui` = `@my/ui`, shipped as TypeScript source; `app/web`) with exact versions,
one committed `app/bun.lock`, and `app/bunfig.toml` (isolated installs, no version younger than two weeks).
Bun installs packages and runs scripts; Vite runs on Node.js — never add `--bun` ([ADR-0025](../adr/0025-web-client-stack.md)).
