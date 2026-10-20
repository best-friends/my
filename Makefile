.DEFAULT_GOAL := help
SHELL := bash

MARKDOWNLINT := bunx --bun markdownlint-cli2@0.23.2

# Go tools (go-licenses, sqlc, oapi-codegen) are pinned with `tool` directives in tools/go.mod,
# kept apart from the product module so their dependencies never affect ours.
# `./...` covers the root module only; the nested tools/ module is excluded.
GO_TOOL := go tool -modfile=tools/go.mod
# Licenses compatible with AGPL-3.0-only (ADR-0011). Anything else, including unknown, fails.
GO_ALLOWED_LICENSES := 0BSD,AGPL-3.0,Apache-2.0,BSD-2-Clause,BSD-3-Clause,BSL-1.0,CC0-1.0,ISC,MIT,MPL-2.0,Unlicense,Zlib
# npm dependencies of tools/i18n and of the client workspace app/ must satisfy the same allowlist, plus the
# permissive BlueOak-1.0.0 and MIT-0 (no attribution clause), which jsdom's dependencies use.
NPM_ALLOWED_LICENSES := $(GO_ALLOWED_LICENSES),BlueOak-1.0.0,MIT-0
# Version that `make build` stamps into cmd/my.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help check lint-docs lint-shell adr-index adr-index-check adr-new build \
	go-check go-mod-check go-vet go-lint go-test go-test-postgres go-test-stream go-build go-licenses \
	i18n-install i18n-gen i18n-check

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

# --- Checks by context (make check, make check-changed) -------------------------------------
# Each context has one `<context>-check` target, and CI runs each in its own job. `make check` installs the
# npm dependencies once, then runs the contexts in parallel, CHECK_JOBS at a time, each context's steps one after
# another, and prints each context's output as one block when that context ends, then a summary. Inside the run
# the installs are skipped (CHECK_INSTALLED), so no two contexts write node_modules at once, and no two contexts
# write the same file: web-check builds into app/web/dist only (web-dist), never into internal/interface/web/dist
# that the Go builds embed. scripts/changed-contexts.sh maps changed paths to contexts and their dependents.
CHECKS := docs-check go-check i18n-check api-check ui-check web-check docker-check
CHECK_JOBS ?= $(words $(CHECKS))
CHECK_STATUS := .tmp/check
# The revision `make check-changed` compares with: the changes since its merge base with HEAD, uncommitted and
# untracked files included.
BASE ?= origin/main

.PHONY: check check-changed docs-check docker-check ui-check

check: web-install i18n-install ## Run every offline check CI runs, the contexts in parallel (CHECKS="go-check web-check" for some, CHECK_JOBS=1 for one at a time)
	@rm -rf $(CHECK_STATUS) && mkdir -p $(CHECK_STATUS)
	@$(MAKE) --no-print-directory --keep-going -j$(CHECK_JOBS) --output-sync=recurse $(addprefix check-run-,$(CHECKS)); \
		status=$$?; \
		echo 'make check:'; \
		for c in $(CHECKS); do printf '  %-14s %s\n' "$$c" "$$(cat $(CHECK_STATUS)/$$c 2>/dev/null || echo 'not run')"; done; \
		exit $$status

# One context of `make check`, in a make of its own that runs the context's steps serially. Its MAKEFLAGS are
# dropped so that it neither joins the parallel jobserver nor keeps going past its first failure.
check-run-%:
	@start=$$(date +%s); \
		if env -u MAKEFLAGS -u MFLAGS $(MAKE) --no-print-directory CHECK_INSTALLED=1 $*; then \
			echo "passed in $$(( $$(date +%s) - start ))s" > $(CHECK_STATUS)/$*; \
		else \
			echo "FAILED after $$(( $$(date +%s) - start ))s; its output is above" > $(CHECK_STATUS)/$*; \
			echo "make check: $* FAILED" >&2; exit 1; \
		fi

check-changed: ## Run, in parallel, the checks of the contexts changed since BASE (default origin/main) and of their dependents
	@contexts="$$(scripts/changed-contexts.sh '$(BASE)')" || exit 1; \
		checks=""; \
		for c in $$contexts; do \
			case " $(CHECKS) " in *" $$c-check "*) checks="$$checks $$c-check";; esac; \
		done; \
		echo "contexts changed since $(BASE):" $$contexts; \
		case " $$(echo $$contexts) " in *" shell "*) echo 'also run `make lint-shell` (shellcheck; CI runs it)';; esac; \
		case " $$(echo $$contexts) " in *" schema "*) echo 'also run `make db-check` for both dialects (CI runs it)';; esac; \
		if [ -z "$$checks" ]; then echo 'no check covers these changes'; exit 0; fi; \
		$(MAKE) --no-print-directory check CHECKS="$${checks# }"

docs-check: lint-docs adr-index-check ## Run the documentation checks: markdownlint and the ADR index

docker-check: dockerfile-check ## Run the offline container image checks

lint-docs: ## Lint Markdown files
	$(MARKDOWNLINT)

lint-shell: ## Lint shell scripts with shellcheck
	shellcheck scripts/*.sh

adr-index: ## Regenerate the ADR index in docs/adr/README.md
	scripts/adr-index.sh

adr-index-check: ## Fail if the ADR index is stale
	scripts/adr-index.sh --check

adr-new: ## Create a new ADR: make adr-new TITLE="..." [SLUG=...]
	@test -n "$(TITLE)" || { echo 'usage: make adr-new TITLE="..." [SLUG=kebab-slug]'; exit 2; }
	scripts/adr-new.sh "$(TITLE)" $(SLUG)

build: web-build ## Build bin/my, with the web client embedded, and the version from git (override: make build VERSION=v1.2.3)
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/my ./cmd/my

.PHONY: config-docs
config-docs: ## Regenerate docs/reference/configuration.md from the config schema (go-test fails when it is stale)
	go test ./internal/platform/config -run '^TestReferenceUpToDate$$' -update

# The Unicode emoji that emoji reactions accept and normalize (ADR-0052 §1). To move to a new
# Unicode version, change both and run make emoji-table.
EMOJI_VERSION := 18.0.0
EMOJI_TEST_SHA256 := 8f3735cda1f92a779d78af67cf86066bb1f07143dc22f2ac29394d9bc57ab21a

.PHONY: emoji-table
emoji-table: ## Regenerate internal/domain/content/emoji_table.gen.go from Unicode's emoji-test.txt (EMOJI_VERSION)
	mkdir -p .tmp
	curl -fsSL -o .tmp/emoji-test-$(EMOJI_VERSION).txt https://www.unicode.org/Public/$(EMOJI_VERSION)/emoji/emoji-test.txt
	echo "$(EMOJI_TEST_SHA256)  .tmp/emoji-test-$(EMOJI_VERSION).txt" | sha256sum -c -
	go run ./scripts/emoji-table -in .tmp/emoji-test-$(EMOJI_VERSION).txt -version $(EMOJI_VERSION) \
		-out internal/domain/content/emoji_table.gen.go

.PHONY: sqlc sqlc-check
sqlc:## Regenerate Go code from sqlc.yaml with the pinned sqlc (ADR-0003)
	scripts/sqlc.sh

sqlc-check: ## Fail if the committed sqlc-generated code is stale
	scripts/sqlc.sh --check

# --- Client API (ADR-0016, ADR-0029) ----------------------------------------------------------
# api/openapi.yaml is the source of truth; oapi-codegen is pinned in tools/go.mod and Redocly CLI here.
REDOCLY := REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true bunx --bun @redocly/cli@2.57.0

.PHONY: generate generate-check api-lint api-check
generate: web-install ## Regenerate the client API code from api/openapi.yaml (scripts/generate.sh)
	scripts/generate.sh

generate-check: web-install ## Fail if the committed client API code is stale
	scripts/generate.sh --check

api-lint: ## Lint api/openapi.yaml, including the supported OpenAPI 3.1 subset (api/redocly.yaml)
	$(REDOCLY) lint --config api/redocly.yaml api/openapi.yaml

api-check: api-lint generate-check ## Lint the client API spec and fail on stale generated code

go-check: go-mod-check go-vet go-lint go-test go-build go-licenses sqlc-check ## Run all offline Go checks

go-mod-check: ## Fail if go.mod/go.sum are not tidy
	go mod tidy -diff
	cd tools && go mod tidy -diff

go-vet: ## Run go vet
	go vet ./...

go-lint: ## Run golangci-lint (includes the depguard architecture boundaries)
	golangci-lint run ./...

go-test: ## Run Go tests (PostgreSQL tests on pgmem, ADR-0041)
	go test ./...

# The packages whose tests use PostgreSQL through pkg/testkit/pgtest.
POSTGRES_TEST_PACKAGES := ./internal/adapter/db/postgres/ ./internal/adapter/kv/db/ ./internal/adapter/queue/db/ ./internal/adapter/stream/postgres/ ./internal/platform/

go-test-postgres: ## Run the PostgreSQL tests on a postgres container instead of pgmem (needs Docker; CI on main and nightly)
	TEST_POSTGRES=container TEST_POSTGRES_REQUIRED=1 go test -count=1 $(POSTGRES_TEST_PACKAGES)

go-test-stream: ## Run the stream.Bus contracts on broker containers instead of the in-process fakes (Valkey; Floci for SNS / SQS; the Pub/Sub emulator) (needs Docker; CI on main and nightly)
	TEST_STREAM=container TEST_VALKEY_REQUIRED=1 TEST_AWS=container TEST_AWS_REQUIRED=1 TEST_PUBSUB_REQUIRED=1 go test -count=1 ./internal/adapter/stream/...

go-build: ## Build all packages statically (CGO_ENABLED=0)
	CGO_ENABLED=0 go build ./...

go-licenses: ## Fail on dependency licenses incompatible with AGPL-3.0
	$(GO_TOOL) go-licenses check --allowed_licenses=$(GO_ALLOWED_LICENSES) ./...

# --- Database schema (ADR-0023) ---------------------------------------------------------------
# Atlas Community Edition, pinned and checksum-verified by scripts/install-atlas.sh.
# Pick the dialect with DB_DIALECT and point DB_URL / DB_DEV_URL elsewhere to override the defaults.
ATLAS := bin/atlas
DB_DIALECT ?= sqlite
PG_CONTAINER ?= my-postgres
PG_IMAGE ?= postgres:18-alpine
PG_PORT ?= 5432
ifeq ($(DB_DIALECT),sqlite)
DB_URL ?= sqlite://my.db
DB_DEV_URL ?= sqlite://dev?mode=memory
else ifeq ($(DB_DIALECT),postgres)
# Schema-scoped URLs: Atlas manages only the `public` schema.
DB_URL ?= postgres://my:my@localhost:$(PG_PORT)/my?sslmode=disable&search_path=public
DB_DEV_URL ?= postgres://my:my@localhost:$(PG_PORT)/my_dev?sslmode=disable&search_path=public
else
$(error DB_DIALECT must be sqlite or postgres, got "$(DB_DIALECT)")
endif
# Runs schema/postgres.bootstrap.sql (from stdin) against the target database before each apply.
# For a database outside the db-up container: PG_BOOTSTRAP='psql -v ON_ERROR_STOP=1 -q "postgres://..."'
PG_BOOTSTRAP ?= docker exec -i -e PGOPTIONS=--client-min-messages=warning $(PG_CONTAINER) psql -v ON_ERROR_STOP=1 -q -U my -d my
DB_SCHEMA := schema/$(DB_DIALECT).sql

.PHONY: atlas db-up db-down db-bootstrap db-apply db-diff db-reset db-check

atlas: ## Install the pinned Atlas Community Edition into bin/atlas
	scripts/install-atlas.sh $(ATLAS)

db-up: ## Start the dev PostgreSQL container (databases `my` and `my_dev`, both bootstrapped)
	@docker inspect $(PG_CONTAINER) >/dev/null 2>&1 || docker run -d --name $(PG_CONTAINER) \
		-e POSTGRES_USER=my -e POSTGRES_PASSWORD=my -e POSTGRES_DB=my -p $(PG_PORT):5432 $(PG_IMAGE) >/dev/null
	@docker start $(PG_CONTAINER) >/dev/null
	@for _ in $$(seq 60); do docker exec $(PG_CONTAINER) pg_isready -q -U my -d my -h 127.0.0.1 && break; sleep 1; done
	@docker exec $(PG_CONTAINER) psql -q -U my -d my -tAc "SELECT 1 FROM pg_database WHERE datname = 'my_dev'" | grep -q 1 \
		|| docker exec $(PG_CONTAINER) createdb -U my my_dev
	@for db in my my_dev; do \
		docker exec -i -e PGOPTIONS=--client-min-messages=warning $(PG_CONTAINER) psql -v ON_ERROR_STOP=1 -q -U my -d $$db < schema/postgres.bootstrap.sql || exit 1; \
	done
	@echo "PostgreSQL is up in container $(PG_CONTAINER) on localhost:$(PG_PORT)"

db-down: ## Remove the dev PostgreSQL container and its data
	docker rm -f $(PG_CONTAINER)

db-bootstrap: ## Run the dialect's bootstrap SQL against DB_URL (PostgreSQL only; no-op for SQLite)
ifeq ($(DB_DIALECT),postgres)
	$(PG_BOOTSTRAP) < schema/postgres.bootstrap.sql
endif

db-apply: atlas db-bootstrap ## Apply schema/<DB_DIALECT>.sql to DB_URL (DB_DIALECT=sqlite|postgres)
	$(ATLAS) schema apply --url '$(DB_URL)' --to 'file://$(DB_SCHEMA)' --dev-url '$(DB_DEV_URL)' --auto-approve

db-diff: atlas ## Print the SQL that would bring DB_URL to schema/<DB_DIALECT>.sql
	$(ATLAS) schema diff --from '$(DB_URL)' --to 'file://$(DB_SCHEMA)' --dev-url '$(DB_DEV_URL)'

db-reset: atlas ## Drop everything in DB_URL, then apply the schema (destroys data)
	$(ATLAS) schema clean --url '$(DB_URL)' --auto-approve
	$(MAKE) --no-print-directory db-apply

db-check: db-reset ## Reset DB_URL, apply the schema, and fail if any diff remains
	@diff="$$($(ATLAS) schema diff --from '$(DB_URL)' --to 'file://$(DB_SCHEMA)' --dev-url '$(DB_DEV_URL)' --format '{{ sql . }}')" \
		&& if [ -n "$$diff" ]; then printf 'schema diff is not empty after apply:\n%s\n' "$$diff" >&2; exit 1; fi
	@echo "$(DB_DIALECT): schema applies cleanly and leaves no diff"

# --- Container image (ADR-0010) ---------------------------------------------------------------

.PHONY: docker-build docker-smoke dockerfile-check

docker-build: ## Build the server image my:$(VERSION) for the local platform
	docker build --build-arg VERSION=$(VERSION) -t my:$(VERSION) .

docker-smoke: ## Start deploy/docker/compose.yaml, create an admin, check WebFinger / Actor / NodeInfo, tear down
	MY_VERSION=$(VERSION) scripts/docker-smoke.sh

.PHONY: federation-live

# Not part of `make check`: it pulls Mastodon, GoToSocial, and Misskey and takes minutes (CI job federation-live, nightly).
federation-live: ## Run the live interoperability scenarios against Mastodon, GoToSocial, and Misskey (deploy/docker/federation-live)
	MY_VERSION=$(VERSION) scripts/federation-live.sh $(FEDERATION_LIVE_FLAGS)

dockerfile-check: ## Fail if the Dockerfile's Go, Node.js, or Bun image differs from the version in .tool-versions
	@want="golang:$$(awk '$$1 == "golang" { print $$2 }' .tool-versions)-"; \
		grep -q "^ARG GO_IMAGE=$$want" Dockerfile || { echo "Dockerfile: GO_IMAGE must start with $$want" >&2; exit 1; }
	@want="node:$$(awk '$$1 == "nodejs" { print $$2 }' .tool-versions)-"; \
		grep -q "^ARG NODE_IMAGE=$$want" Dockerfile || { echo "Dockerfile: NODE_IMAGE must start with $$want" >&2; exit 1; }
	@want="oven/bun:$$(awk '$$1 == "bun" { print $$2 }' .tool-versions)-"; \
		grep -q "^ARG BUN_IMAGE=$$want" Dockerfile || { echo "Dockerfile: BUN_IMAGE must start with $$want" >&2; exit 1; }

# CHECK_INSTALLED skips the installs inside `make check`, which has run them before starting the contexts.
i18n-install: ## Install the pinned i18n tool dependencies (tools/i18n/bun.lock)
ifndef CHECK_INSTALLED
	cd tools/i18n && bun install --frozen-lockfile
endif

i18n-gen: i18n-install ## Regenerate pseudo-locales, typed accessors, and parity fixtures from locales/
	cd tools/i18n && bun run src/gen.ts

i18n-check: i18n-install ## Lint catalogs, fail on stale generated i18n files, check npm licenses, test the TS runtime
	cd tools/i18n && bun run src/check.ts
	cd tools/i18n && bun run src/licenses.ts $(NPM_ALLOWED_LICENSES)
	cd tools/i18n && bun test
	cd tools/i18n && bun --bun run tsc --noEmit -p .

# --- Web client (ADR-0025, ADR-0026) ----------------------------------------------------------
# app/ is the Bun workspace of the TypeScript clients (app/ui = @my/ui, app/web). Bun installs the
# packages and runs their scripts; Vite runs on Node.js from .tool-versions, because the scripts'
# `vite` is a `#!/usr/bin/env node` bin. Do not add --bun (ADR-0025).

.PHONY: web-install web-typecheck web-dist web-build web-licenses web-lint web-lint-code web-lint-css web-tokens web-test web-e2e web-e2e-install web-routes-check web-check
.PHONY: ui-typecheck ui-unit-test

# internal/interface/web embeds the build from here with //go:embed all:dist (ADR-0028). Only its
# .gitkeep is committed, so Go builds and tests need no Bun; `make build` embeds the real build.
WEB_EMBED_DIR := internal/interface/web/dist

web-install: ## Install the pinned client workspace dependencies (app/bun.lock)
ifndef CHECK_INSTALLED
	cd app && bun install --frozen-lockfile
endif

web-typecheck: web-install ## Type-check app/web with tsc (incremental, node_modules/.cache/tsc)
	cd app/web && bun run typecheck

ui-typecheck: web-install ## Type-check app/ui with tsc (incremental, node_modules/.cache/tsc)
	cd app/ui && bun run typecheck

web-dist: web-install ## Build app/web into app/web/dist (fails when React Compiler compiled nothing or skipped a function)
	cd app && bun run build

web-build: web-dist ## Build app/web and copy the build to internal/interface/web/dist for embedding
	find $(WEB_EMBED_DIR) -mindepth 1 ! -name .gitkeep -delete
	cp -R app/web/dist/. $(WEB_EMBED_DIR)/

web-licenses: web-install i18n-install ## Fail on client npm dependency licenses incompatible with AGPL-3.0
	cd tools/i18n && bun run src/licenses.ts $(NPM_ALLOWED_LICENSES) ../../app/node_modules

web-lint: web-lint-code web-lint-css ## Lint the client workspace: Oxlint (app/.oxlintrc.json), the tests of its restrictions, knip, no 100vh in CSS

# The two halves of web-lint, for the CI jobs: the merge queue runs web-lint-code alone (ADR-0047).
web-lint-code: web-install ## Run Oxlint (app/.oxlintrc.json), the tests of its restrictions and violation fixtures (app/oxlint), and knip
	cd app && bun run lint
	cd app && bun run lint:rules
	cd app && bun run knip

web-lint-css: ## Fail when CSS uses 100vh (docs/architecture/web-client.md D5)
	@! grep -rn --include='*.css' '100vh' app/ui/src app/web/src \
		|| { echo 'CSS must not use 100vh; use svh / dvh (docs/architecture/web-client.md D5)' >&2; exit 1; }

web-tokens: web-install ## Check the @my/ui design tokens: WCAG AA contrast of every token pair, no palette / arbitrary colors, motion only under motion-safe
	cd app/ui && bun run check-tokens

web-test: web-install ## Run the app/web component tests with Vitest (jsdom + MSW)
	cd app/web && bun run test

ui-unit-test: web-install ## Run the app/ui unit tests with Vitest (jsdom); the story tests are `make ui-test`
	cd app/ui && bun run test:unit

# Playwright runs against `vite preview` of the fresh build, and the server-rendered pages against
# `my serve` on SQLite from the fresh bin/my (app/web/e2e/server.ts). E2E_MATRIX=full runs every width ×
# locale project (nightly); the default is the default subset (app/web/playwright.config.ts). It runs every
# test in one go; CI (web-e2e.yml) builds once and runs the tests as Playwright shards in the pinned Playwright
# image. Locally, install the browser once with `make web-e2e-install`. Both servers
# listen on free ports picked per run (E2E_PREVIEW_PORT / E2E_SERVER_PORT fix them), so parallel runs in
# several worktrees stay apart; E2E_BASE_URL / E2E_SERVER_URL test servers started elsewhere instead.
web-e2e: build ## Run the web E2E tests with Playwright (E2E_MATRIX=full for the whole matrix)
	cd app/web && bun run e2e

web-e2e-install: web-install ## Download the Playwright browser that web-e2e uses (local runs only)
	cd app/web && bun run playwright install chromium

web-routes-check: web-dist ## Fail when the committed app/web/src/routeTree.gen.ts differs from the one the build generates
	@git diff --exit-code --quiet -- app/web/src/routeTree.gen.ts \
		|| { echo 'app/web/src/routeTree.gen.ts was stale and the build regenerated it; commit it (docs/architecture/web-client.md §9)' >&2; exit 1; }

web-check: web-typecheck web-lint web-dist web-routes-check web-licenses web-tokens web-test playwright-image-check ## Run the offline checks of app/web and the workspace-wide client checks (Oxlint, knip, licenses, tokens)

ui-check: ui-typecheck ui-unit-test ## Run the offline checks of app/ui (@my/ui); its dependent app/web is web-check

# --- @my/ui Storybook and story tests (ADR-0027) ----------------------------------------------
# The story tests run every story in Chromium (Vitest browser mode, Playwright): axe must find no
# violation and each story must match its screenshots at 3 container widths × light / dark × LTR / RTL.
# They need a browser, so they are not part of `make check`; CI runs `make ui-test` in PLAYWRIGHT_IMAGE
# on every PR. Baselines (app/ui/src/__screenshots__/) come only from that image: `make ui-snapshots-update`.

# The official Playwright image every browser job runs in, pinned by digest. Its version must match the
# playwright packages in app/; playwright-image-check also keeps the workflows on this exact reference.
PLAYWRIGHT_IMAGE := mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27

.PHONY: storybook storybook-build ui-test ui-test-install ui-test-docker ui-snapshots-update playwright-image-check

storybook: web-install ## Start the @my/ui Storybook on http://localhost:6006
	cd app/ui && bun run storybook

storybook-build: web-install ## Build the static @my/ui Storybook into app/ui/storybook-static
	cd app/ui && bun run build-storybook

ui-test: web-install ## Run the @my/ui story tests (axe + screenshots) in Chromium; screenshots match only in PLAYWRIGHT_IMAGE
	cd app/ui && bun run test

ui-test-install: web-install ## Download the Chromium that ui-test uses (local runs only)
	cd app/ui && bun run playwright install chromium

ui-test-docker: ## Run the @my/ui story tests in PLAYWRIGHT_IMAGE, as CI does (needs docker)
	PLAYWRIGHT_IMAGE=$(PLAYWRIGHT_IMAGE) scripts/ui-test-docker.sh

ui-snapshots-update: ## Regenerate the @my/ui screenshot baselines in PLAYWRIGHT_IMAGE (needs docker); review and commit them
	PLAYWRIGHT_IMAGE=$(PLAYWRIGHT_IMAGE) scripts/ui-test-docker.sh --update

playwright-image-check: ## Fail if a workflow's Playwright image or a playwright package version differs from PLAYWRIGHT_IMAGE
	PLAYWRIGHT_IMAGE=$(PLAYWRIGHT_IMAGE) scripts/playwright-image-check.sh
