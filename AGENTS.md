# AGENTS.md

Instructions for coding agents (Claude Code, Codex, Copilot, Gemini, etc.) working in this repository.
Human contributors should read [CONTRIBUTING.md](CONTRIBUTING.md); the rules are the same.

## Project in one paragraph

An ActivityPub-compatible microblog server written in Go, designed to run at near-zero cost on Google Cloud, AWS,
or a self-hosted VM (Docker / microk8s), with swappable RDB / queue / storage adapters, its own client API (no Mastodon client API compatibility),
a PWA web client, a Tauri 2 mobile app, and i18n from day one. Distributed as OSS.

## Current phase: M4 — Feature Completeness

- Goal: extend the basic features and raise the completeness of the service — custom emoji (local and remote) and emoji reactions
  that federate with Mastodon, Misskey and Pleroma (Akkoma), and the remaining unimplemented Must / Should
  [functional requirements](docs/requirements/01-functional.md) — on the server, the client API and the web client
  ([vision](docs/requirements/00-vision.md)). Deployment templates for GCP / AWS / microk8s belong to M6.
- The server is Go ([ADR-0001](docs/adr/0001-server-language-and-runtime.md)); clients are TypeScript under `app/` with Bun.
  Cloudflare Workers is **not** a target.
- **Web client work must follow [docs/architecture/web-client.md](docs/architecture/web-client.md).** Do not write React from memory or
  from typical online examples; when unsure, check react.dev, the WAI-ARIA APG and the official docs of the library in use.
- The web-client stack and libraries are fixed by Accepted ADRs (from ADR-0025; see [docs/adr/README.md](docs/adr/README.md));
  do not pick libraries outside them without a new ADR.
- The Go package layout follows [overview §6](docs/architecture/overview.md#6-リポジトリ構成); `cmd/my` is the single binary.
  Build only what an issue asks for.
- Spike reports go in the spike issue as a comment and spike code stays on an unmerged `spike/<issue>-<slug>` branch;
  only the resulting ADR / doc updates are merged ([ADR-0022](docs/adr/0022-spike-outputs-outside-repository.md)).

## Source of truth

| What | Where |
| --- | --- |
| Vision, scope, milestones | [docs/requirements/00-vision.md](docs/requirements/00-vision.md) |
| Functional requirements (`FR-xxx`) | [docs/requirements/01-functional.md](docs/requirements/01-functional.md) |
| Non-functional requirements (`NFR-xxx`) | [docs/requirements/02-non-functional.md](docs/requirements/02-non-functional.md) |
| Architecture overview, ports, flows | [docs/architecture/overview.md](docs/architecture/overview.md) |
| Hosting targets & cost notes | [docs/architecture/deployment-targets.md](docs/architecture/deployment-targets.md) |
| Federation design (inbox, delivery, fetch, sanitizing) | [docs/architecture/federation.md](docs/architecture/federation.md) |
| Decisions | [docs/adr/README.md](docs/adr/README.md) — **Accepted ADRs are binding; Proposed ADRs are not yet decisions** |
| Tasks | GitHub Issues / Milestones / Project of `best-friends/my` |
| Agent playbooks | [docs/agents/](docs/agents/) |

## How to pick and do work

Follow [docs/agents/workflow.md](docs/agents/workflow.md). Summary:

1. Only take issues labeled **`agent:ready`** with no assignee. Never take `status:needs-decision`.
2. Branch `<type>/<issue>-<slug>`; never push to `main`.
3. Before every commit, run the checks of the contexts you touched (`make go-check`, `make web-check`, ...) or
   `make check-changed`; the CI `check` gate runs every affected context and must pass. Run the full `make check`
   when the change touches shared configuration.
4. Open a PR using the template, with `Closes #N` and the acceptance criteria checked, and wait for its CI to pass.
   Pull requests run the `check` gate ([ADR-0047](docs/adr/0047-ci-protection-layers.md)) but not the web E2E: for a change to the web
   client, run `make web-e2e` or `gh workflow run web-e2e.yml --ref <branch>`.
   Do not merge or queue the PR yourself: the orchestrator or the decider queues it with `gh pr merge --auto`, and the merge queue
   runs `check` again on top of the latest `main` before merging. Merge `main` into the branch only when the PR conflicts or the
   queue drops it. Pushes to `main` and the nightly run the slower suites, and CodeQL code scanning (`codeql.yml`) runs on
   pushes to `main` and weekly, never on PRs; the table of what runs where is in the workflow.
5. If you need a decision that no Accepted ADR covers: small → state the assumption in the PR; large → draft a *Proposed* ADR
   (`make adr-new TITLE=... SLUG=...`), comment on the issue, add `agent:blocked`, and stop.

## Hard rules

- **Boy scout rule** ([ADR-0024](docs/adr/0024-boy-scout-rule.md)): never ignore something broken because it is outside your issue.
  Fix small things in a separate commit and list them in the PR; file an issue for anything larger and link it.
- **Never change an ADR's status to `Accepted`/`Rejected` on your own judgment.** Only the human decider decides. You may record a decision
  the decider has explicitly stated (quote it in the PR description). You may create and edit *Proposed* ADRs.
- Do not rewrite the body of an Accepted ADR; supersede it with a new ADR instead (typo fixes are fine).
- **Every file in the repo describes the present, never its history** ([ADR-0021](docs/adr/0021-documents-describe-present-state.md)).
  No changelog sections, no "previously / in the first draft / old proposal" wording, no `[x]` resolved items in Open Questions —
  fold resolved answers into the body. Explain *why it changed* in the commit message and PR description instead.
- After adding/renaming an ADR, run `make adr-index`.
- v0 allows breaking changes everywhere ([ADR-0012](docs/adr/0012-v0-breaking-change-policy.md)) — no migrations, no compat shims.
  Exception: ActivityPub wire formats and federated URIs must stay spec-compliant.
- Do not add Mastodon *client* API compatibility code ([ADR-0007](docs/adr/0007-client-api.md)). Federation with Mastodon servers is still required.
- Language: design docs (ADR, requirements) in Japanese during v0; code, identifiers, code comments, commit messages, and PR titles in English.
- Commits follow Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`; `!` for breaking).
- License is AGPL-3.0-only ([ADR-0011](docs/adr/0011-license.md)). New source files start with `SPDX-License-Identifier: AGPL-3.0-only`;
  only add dependencies with AGPL-compatible licenses.
- Links from issues and comments to repository files use `blob/main/...` or a commit permalink, never a feature branch
  (branches are deleted on merge). Before merge, link the PR instead.
- Do not commit secrets, `.env` files, or credentials. Do not run destructive commands against remote resources.

## Design rules

These come from Accepted ADRs and are binding.

- Dependency direction ([ADR-0002](docs/adr/0002-modular-monolith-hexagonal.md)) is enforced by depguard rules in `.golangci.yml` (part of `make check`):
  `internal/domain/<module>` imports only the standard library and its own module's domain;
  `internal/app/<module>` adds its own module's domain and other modules' `internal/app` packages (the only cross-module path)
  and the leaf library `internal/i18n`;
  `internal/interface/*` and `internal/adapter/*` never import each other or `internal/platform`;
  `internal/platform` and `cmd/` assemble everything, and nothing imports `cmd/`.
  Never weaken a rule to make an import pass; restructure the code instead.
- Core (domain + application) must not import cloud SDKs or drivers; those live in adapters and `cmd/` entrypoints.
- Keep the server buildable with `CGO_ENABLED=0`; a cgo dependency needs an ADR.
- Every external resource goes through a port; every adapter must pass that port's contract test suite.
- Data modeling follows [ADR-0020](docs/adr/0020-data-modeling-principles.md): classify every table as resource / event / projection / current-value;
  never encode state as nullable timestamp columns; record state changes as events and derive current state in projections owned by one module;
  no JSON for queried or constrained data. Read [docs/architecture/data-model.md](docs/architecture/data-model.md) before touching persistence.
- Schema changes: edit only the desired-state files `schema/<dialect>.sql` (all dialects together) and apply them with Atlas
  Community Edition; never hand-write migration SQL or rely on Atlas features that need `atlas login` ([ADR-0023](docs/adr/0023-declarative-schema-management.md)).
- Keep DB transactions short, retryable, and free of external I/O (HTTP, queue, blob); enqueue jobs via the outbox. Make writes safe to repeat with unique constraints, upserts, idempotency keys ([ADR-0003](docs/adr/0003-database-abstraction.md)).
- Job handlers must be idempotent; delivery is at-least-once ([ADR-0004](docs/adr/0004-job-queue-abstraction.md)).
- Outbound HTTP to destinations that remote servers or users can influence (federation fetch and delivery, WebFinger,
  media proxy, link previews, Web Push, ADR-0019 §1) goes through the SSRF-guarded `kernel.HTTPClient` port;
  `http.DefaultClient` is banned by lint. Cloud SDK clients that call the provider APIs the operator configured
  (AWS, Google Cloud) live in adapters and use the SDK's own HTTP client.
- Visibility checks go through the single domain policy function; never re-implement them in API or federation code.
  Levels are `public` / `local` / `unlisted` / `private` / `direct`; `local` is never federated beyond the author's followers ([ADR-0017](docs/adr/0017-visibility-local.md)).
- **i18n**: never hard-code user-facing strings or date/number formats. Add ICU messages to `locales/en` *and* `locales/ja`,
  run `make i18n-gen`, and format them through the generated typed accessors (`internal/i18n` in Go);
  use CSS logical properties; render user content with `dir="auto"` ([ADR-0009](docs/adr/0009-internationalization.md)).
  Namespaces other than `client*` are formatted by Go too and must avoid rich-text tags, number skeletons, and `date` / `time` arguments.

## Commands

| Command | Purpose |
| --- | --- |
| `make check` [`CHECKS=...`] [`CHECK_JOBS=N`] | Every offline check CI runs, one target per context, the contexts in parallel (each context's steps serially; output grouped per context, then a summary): `docs-check`, `go-check`, `i18n-check`, `api-check`, `ui-check`, `web-check`, `docker-check` |
| `make check-changed` [`BASE=origin/main`] | `make check` of only the contexts changed since the merge base with `BASE` (working tree and untracked files included) and of their dependents (`scripts/changed-contexts.sh`); shared configuration (`Makefile`, `.tool-versions`, workflows, lockfiles, `api/`, `locales/`, `tools/i18n/`) or an unknown path selects every context. CI skips the jobs of the other contexts the same way |
| `make docs-check` / `docker-check` | markdownlint and the ADR index check / the Dockerfile image version check (Go, Node.js, Bun) |
| `make go-check` | `go mod tidy` check, `go vet`, `golangci-lint`, `go test`, `CGO_ENABLED=0 go build`, license check |
| `make go-lint` / `go-test` / `go-build` | Run one Go step (`go-lint` uses `.golangci.yml`, including the depguard architecture rules; `go-test` runs the PostgreSQL tests on pgmem, without Docker, and the Valkey tests on a `valkey/valkey` container, skipped without Docker) |
| `make go-test-postgres` | Run the PostgreSQL tests on a real `postgres:18-alpine` container through testcontainers-go instead of pgmem; needs Docker (CI job `postgres-contract` on the pushes to main that touch the database code, queries, or schema, and nightly, [ADR-0041](docs/adr/0041-postgres-tests-on-pgmem.md)) |
| `make go-test-stream` | Run the `stream.Bus` contracts of `internal/adapter/stream` on broker containers (`valkey/valkey` through `pkg/testkit/valkeytest`, the Floci AWS emulator through `pkg/testkit/awstest`, the Pub/Sub emulator) instead of the in-process fakes (miniredis, the SNS / SQS fake of `pkg/testkit/awstest`, pstest) that `go-test` uses; needs Docker (CI job `stream-contract` on pushes to main and nightly, [ADR-0050](docs/adr/0050-stream-pubsub-across-processes.md)) |
| `make build [VERSION=...]` | Build `bin/my` with the web client embedded (`make web-build` first) and the version stamped in (`git describe` by default) |
| `make sqlc` / `sqlc-check` | Regenerate the sqlc code from `sqlc.yaml` / fail when the committed code is stale (`sqlc-check` is part of `make check`) |
| `make generate` / `generate-check` / `api-lint` | Regenerate the client API code from `api/openapi.yaml` / fail when it is stale / lint the spec with Redocly CLI (`api/redocly.yaml`); `make api-check` runs the last two and is part of `make check`. Follow [docs/architecture/client-api.md](docs/architecture/client-api.md) to add an endpoint |
| `make config-docs` | Regenerate [docs/reference/configuration.md](docs/reference/configuration.md) from the config schema (`go-test` fails when it is stale) |
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
| `make web-check` | `app/web` and the workspace-wide client checks: `web-typecheck` (incremental tsc), `web-lint`, `web-dist` (Vite build into `app/web/dist`; fails when React Compiler compiled nothing or skipped a function that does not opt out; `make web-build` also copies it to `internal/interface/web/dist` for embedding), `web-routes-check` (the committed TanStack Router route tree is up to date), `web-licenses` (npm licenses), `web-tokens` (design token contrast and color / motion usage, [app/ui/README.md](app/ui/README.md)), `web-test` (Vitest in jsdom with Testing Library and MSW), `playwright-image-check` (every Playwright image and package version equals `PLAYWRIGHT_IMAGE`) |
| `make web-lint` | Oxlint (`app/.oxlintrc.json`, enforcing the 強制 column of [web-client.md](docs/architecture/web-client.md), [ADR-0049](docs/adr/0049-client-lint-oxlint.md)) with warnings as errors, the tests of its restrictions and of a violation per 強制 row (`app/oxlint`, `node --test`), knip, and a no-`100vh` check on CSS |
| `make web-e2e` [`E2E_MATRIX=full`] | Build `app/web` and `bin/my` and run the Playwright E2E (axe, horizontal scroll, ARIA snapshots) against `vite preview`, and for the server-rendered sign-in / sign-up / consent pages against a real `my serve` on SQLite (`app/web/e2e/server.ts`): the default subset of the width × locale matrix, or all of it (CI workflow `web-e2e.yml` on pushes to main only, one build tested by 4 Playwright shards, ADR-0040 / ADR-0047 — run it locally before a PR that touches the web client, or on the PR's branch with `gh workflow run web-e2e.yml --ref <branch>`; nightly runs `full` on 8 shards). `make web-e2e-install` downloads the browser for local runs |
| `make web-install` | Install the client workspace dependencies from `app/bun.lock` |
| `make storybook` / `storybook-build` | Start / build the `@my/ui` Storybook (locale and theme toolbars) |
| `make ui-test` | Run every `@my/ui` story in Chromium: axe violations and screenshots at 3 container widths × light / dark × LTR / RTL. Needs a browser, so not in `make check`; CI job `ui-test` runs it on pull requests and pushes to main that touch `app/ui` (not in the merge queue), in the pinned Playwright image (`PLAYWRIGHT_IMAGE` in the `Makefile`). Screenshots match only there: use `make ui-test-docker` locally |
| `make ui-snapshots-update` | Regenerate the `@my/ui` screenshot baselines in the pinned Playwright image (Docker); never commit screenshots made outside it |

Toolchain versions (Go, golangci-lint, Bun, shellcheck, Node.js) are pinned in `.tool-versions` (asdf / mise).
Atlas Community Edition is pinned and checksum-verified in `scripts/install-atlas.sh` (`make atlas` installs `bin/atlas`).
Go tools (go-licenses, sqlc, oapi-codegen) are pinned with `tool` directives in `tools/go.mod` and run via
`go tool -modfile=tools/go.mod <tool>`.
The i18n generator and checks are a Bun package in `tools/i18n` with exact versions in `package.json` and a committed `bun.lock`.
The clients live in the Bun workspace `app/` (`app/ui` = `@my/ui`, shipped as TypeScript source; `app/web`) with exact versions,
one committed `app/bun.lock`, and `app/bunfig.toml` (isolated installs, no version younger than two weeks).
Bun installs packages and runs scripts; Vite runs on Node.js — never add `--bun` ([ADR-0025](docs/adr/0025-web-client-stack.md)).
