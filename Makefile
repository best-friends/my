.DEFAULT_GOAL := help
SHELL := bash

MARKDOWNLINT := bunx --bun markdownlint-cli2@0.23.2

# Go tools (govulncheck, go-licenses) are pinned with `tool` directives in tools/go.mod,
# kept apart from the product module so their dependencies never affect ours.
# `./...` covers the root module only; the nested tools/ module is excluded.
GO_TOOL := go tool -modfile=tools/go.mod
# Licenses compatible with AGPL-3.0-only (ADR-0011). Anything else, including unknown, fails.
GO_ALLOWED_LICENSES := 0BSD,AGPL-3.0,Apache-2.0,BSD-2-Clause,BSD-3-Clause,BSL-1.0,CC0-1.0,ISC,MIT,MPL-2.0,Unlicense,Zlib
# Version that `make build` stamps into cmd/my.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help check lint-docs lint-shell adr-index adr-index-check adr-new build \
	go-check go-mod-check go-vet go-lint go-test go-build go-licenses go-vuln

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

check: lint-docs adr-index-check go-check ## Run every offline check CI runs

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

go-check: go-mod-check go-vet go-lint go-test go-build go-licenses ## Run all offline Go checks

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
