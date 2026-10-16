.DEFAULT_GOAL := help
SHELL := bash

MARKDOWNLINT := bunx --bun markdownlint-cli2@0.23.2

# Go tools (govulncheck, go-licenses) are pinned with `tool` directives in tools/go.mod,
# kept apart from the product module so their dependencies never affect ours.
# `./...` covers the root module only; the nested tools/ module is excluded.
GO_TOOL := go tool -modfile=tools/go.mod
# Licenses compatible with AGPL-3.0-only (ADR-0011). Anything else, including unknown, fails.
GO_ALLOWED_LICENSES := 0BSD,AGPL-3.0,Apache-2.0,BSD-2-Clause,BSD-3-Clause,BSL-1.0,CC0-1.0,ISC,MIT,MPL-2.0,Unlicense,Zlib
# npm dependencies of tools/i18n and of the client workspace app/ must satisfy the same allowlist.
NPM_ALLOWED_LICENSES := $(GO_ALLOWED_LICENSES)
# Version that `make build` stamps into cmd/my.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help check lint-docs lint-shell adr-index adr-index-check adr-new build \
	go-check go-mod-check go-vet go-lint go-test go-build go-licenses go-vuln \
	i18n-install i18n-gen i18n-check

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

check: lint-docs adr-index-check go-check i18n-check dockerfile-check api-check web-check ## Run every offline check CI runs

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

build: ## Build bin/my with the version from git (override: make build VERSION=v1.2.3)
	CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/my ./cmd/my

.PHONY: config-docs
config-docs: ## Regenerate docs/reference/configuration.md from the config schema (go-test fails when it is stale)
	go test ./internal/platform/config -run '^TestReferenceUpToDate$$' -update

.PHONY: sqlc sqlc-check
sqlc: ## Regenerate Go code from sqlc.yaml with the pinned sqlc (ADR-0003)
	scripts/sqlc.sh

sqlc-check: ## Fail if the committed sqlc-generated code is stale
	scripts/sqlc.sh --check

# --- Client API (ADR-0016, ADR-0029) ----------------------------------------------------------
# api/openapi.yaml is the source of truth; oapi-codegen is pinned in tools/go.mod and Redocly CLI here.
REDOCLY := REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true bunx --bun @redocly/cli@2.57.0

.PHONY: generate generate-check api-lint api-check
generate: ## Regenerate the client API code from api/openapi.yaml (scripts/generate.sh)
	scripts/generate.sh

generate-check: ## Fail if the committed client API code is stale
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

go-test: ## Run Go tests
	go test ./...

go-build: ## Build all packages statically (CGO_ENABLED=0)
	CGO_ENABLED=0 go build ./...

go-licenses: ## Fail on dependency licenses incompatible with AGPL-3.0
	$(GO_TOOL) go-licenses check --allowed_licenses=$(GO_ALLOWED_LICENSES) ./...

go-vuln: ## Scan for known vulnerabilities with govulncheck (needs network)
	$(GO_TOOL) govulncheck ./...

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

dockerfile-check: ## Fail if the Dockerfile's Go image differs from the Go version in .tool-versions
	@want="golang:$$(awk '$$1 == "golang" { print $$2 }' .tool-versions)-"; \
		grep -q "^ARG GO_IMAGE=$$want" Dockerfile || { echo "Dockerfile: GO_IMAGE must start with $$want" >&2; exit 1; }

i18n-install: ## Install the pinned i18n tool dependencies (tools/i18n/bun.lock)
	cd tools/i18n && bun install --frozen-lockfile

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

.PHONY: web-install web-typecheck web-build web-licenses web-lint web-check

web-install: ## Install the pinned client workspace dependencies (app/bun.lock)
	cd app && bun install --frozen-lockfile

web-typecheck: web-install ## Type-check every client package with tsc
	cd app && bun run typecheck

web-build: web-install ## Build app/web into app/web/dist (fails when React Compiler compiled nothing)
	cd app && bun run build

web-licenses: web-install i18n-install ## Fail on client npm dependency licenses incompatible with AGPL-3.0
	cd tools/i18n && bun run src/licenses.ts $(NPM_ALLOWED_LICENSES) ../../app/node_modules

web-lint: web-install ## Lint the client workspace: ESLint (app/eslint.config.js), its rule tests, knip, no 100vh in CSS
	cd app && bun run lint
	cd app && bun run lint:rules
	cd app && bun run knip
	@! grep -rn --include='*.css' '100vh' app/ui/src app/web/src \
		|| { echo 'CSS must not use 100vh; use svh / dvh (docs/architecture/web-client.md D5)' >&2; exit 1; }

web-check: web-typecheck web-lint web-build web-licenses ## Run every offline web client check
