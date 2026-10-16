.DEFAULT_GOAL := help
SHELL := bash

MARKDOWNLINT := bunx --bun markdownlint-cli2@0.23.2

.PHONY: help check lint-docs adr-index adr-index-check adr-new

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

check: lint-docs adr-index-check ## Run every check CI runs

lint-docs: ## Lint Markdown files
	$(MARKDOWNLINT)

adr-index: ## Regenerate the ADR index in docs/adr/README.md
	scripts/adr-index.sh

adr-index-check: ## Fail if the ADR index is stale
	scripts/adr-index.sh --check

adr-new: ## Create a new ADR: make adr-new TITLE="..." [SLUG=...]
	@test -n "$(TITLE)" || { echo 'usage: make adr-new TITLE="..." [SLUG=kebab-slug]'; exit 2; }
	scripts/adr-new.sh "$(TITLE)" $(SLUG)
