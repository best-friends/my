.DEFAULT_GOAL := help
SHELL := bash

MARKDOWNLINT := bunx --bun markdownlint-cli2@0.23.2

# Go tools (govulncheck, go-licenses) are pinned with `tool` directives in tools/go.mod,
# kept apart from the product module so their dependencies never affect ours.
GO_TOOL := go tool -modfile=tools/go.mod
# Packages of the root module. Nested modules (spikes/*, tools/) are excluded by `./...`.
GO_PKGS = $(shell go list ./... 2>/dev/null)
# Licenses compatible with AGPL-3.0-only (ADR-0011). Anything else, including unknown, fails.
GO_ALLOWED_LICENSES := 0BSD,AGPL-3.0,Apache-2.0,BSD-2-Clause,BSD-3-Clause,BSL-1.0,CC0-1.0,ISC,MIT,MPL-2.0,Unlicense,Zlib

.PHONY: help check lint-docs lint-shell adr-index adr-index-check adr-new \
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

go-check: go-mod-check go-vet go-lint go-test go-build go-licenses ## Run all offline Go checks

go-mod-check: ## Fail if go.mod/go.sum are not tidy
	go mod tidy -diff
	cd tools && go mod tidy -diff

go-vet: ## Run go vet
	@if [ -z "$(GO_PKGS)" ]; then echo "go-vet: no Go packages, skipping"; else go vet ./...; fi

go-lint: ## Run golangci-lint
	@if [ -z "$(GO_PKGS)" ]; then echo "go-lint: no Go packages, skipping"; else golangci-lint run ./...; fi

go-test: ## Run Go tests
	@if [ -z "$(GO_PKGS)" ]; then echo "go-test: no Go packages, skipping"; else go test ./...; fi

go-build: ## Build all packages statically (CGO_ENABLED=0)
	CGO_ENABLED=0 go build ./...

go-licenses: ## Fail on dependency licenses incompatible with AGPL-3.0
	@if [ -z "$(GO_PKGS)" ]; then echo "go-licenses: no Go packages, skipping"; \
	else $(GO_TOOL) go-licenses check --allowed_licenses=$(GO_ALLOWED_LICENSES) ./...; fi

go-vuln: ## Scan for known vulnerabilities with govulncheck (needs network)
	@if [ -z "$(GO_PKGS)" ]; then echo "go-vuln: no Go packages, skipping"; else $(GO_TOOL) govulncheck ./...; fi
