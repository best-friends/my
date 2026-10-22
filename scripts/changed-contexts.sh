#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only
#
# Prints the check contexts that the changes since BASE touch, one per line, including the contexts that
# depend on them. `make check-changed` runs the matching `<context>-check` targets and CI's `changes` job
# skips the jobs of the other contexts.
#
#   scripts/changed-contexts.sh BASE          changes from the merge base of BASE and HEAD to the working tree,
#                                             plus untracked files (local use)
#   scripts/changed-contexts.sh --all         every context
#   scripts/changed-contexts.sh --files FILE  the paths listed in FILE, one per line (CI and tests)
#
# Contexts:
#   docs    Markdown lint and the ADR index                     (make docs-check)
#   go      the Go module, sqlc, and the Go tests                (make go-check)
#   i18n    the catalogs and their generated accessors           (make i18n-check)
#   api     the OpenAPI spec and the code generated from it      (make api-check)
#   ui      app/ui (@my/ui)                                      (make ui-check, the story tests)
#   web     app/web and the workspace-wide client checks         (make web-check)
#   docker  the Dockerfile and the deployments                   (make docker-check, the Docker smoke test)
#   shell   the shell scripts                                    (make lint-shell)
#   schema  the desired-state schema files                       (make db-check)
#   agentkit tools/agentkit and its context map, which covers every path (make agentkit-check)
#
# A change to shared configuration (the Makefile, .tool-versions, the workflows, the Go or Bun lockfiles,
# api/, locales/, tools/i18n/) selects every context, and so does any path no rule below knows:
# a new kind of file runs everything until a rule is added for it.
set -euo pipefail

ALL=(docs go i18n api ui web docker shell schema agentkit)

root="$(cd "$(dirname "$0")/.." && pwd)"

usage() {
  echo "usage: scripts/changed-contexts.sh BASE | --all | --files FILE" >&2
  exit 2
}

[ "$#" -eq 1 ] || [ "$#" -eq 2 ] || usage
case "$1" in
  --all)
    printf '%s\n' "${ALL[@]}"
    exit 0
    ;;
  --files)
    [ "$#" -eq 2 ] || usage
    files="$(cat "$2")"
    ;;
  -*) usage ;;
  *)
    [ "$#" -eq 1 ] || usage
    base="$(git -C "$root" merge-base "$1" HEAD)"
    files="$(
      git -C "$root" diff --name-only "$base" --
      git -C "$root" ls-files --others --exclude-standard
    )"
    ;;
esac

declare -A selected=()
select_contexts() {
  local c
  for c in "$@"; do
    selected[$c]=1
  done
}

# classify PATH selects the contexts of one changed path. Rules add up: app/ui/README.md is docs, ui, and web.
classify() {
  local path="$1" known=0
  # tools/agentkit/context-map.yaml must cover every path, so any change can break `agentkit map check`.
  select_contexts agentkit
  case "$path" in
    # Shared configuration: everything.
    Makefile | .tool-versions | .github/workflows/* | scripts/changed-contexts.sh | \
      go.mod | go.sum | tools/go.mod | tools/go.sum | \
      app/bun.lock | app/bunfig.toml | app/package.json | api/* | locales/* | tools/i18n/*)
      select_contexts "${ALL[@]}"
      return
      ;;
  esac
  case "$path" in
    # A Go module of its own: its Go files are not the go context's.
    tools/agentkit/*.md)
      select_contexts docs
      return
      ;;
    tools/agentkit/*)
      return
      ;;
    scripts/agentkit*.sh)
      select_contexts shell
      return
      ;;
  esac
  case "$path" in
    *.md | .markdownlint-cli2.jsonc | docs/* | scripts/adr-index.sh | scripts/adr-new.sh)
      select_contexts docs
      known=1
      ;;
  esac
  case "$path" in
    # go test fails when the generated configuration reference is stale (make config-docs).
    *.go | cmd/* | internal/* | pkg/* | queries/* | sqlc.yaml | .golangci.yml | scripts/sqlc.sh | scripts/go-build-tags.sh | \
      docs/reference/configuration.md)
      select_contexts go
      known=1
      ;;
  esac
  case "$path" in
    schema/* | scripts/install-atlas.sh)
      select_contexts schema go
      known=1
      ;;
  esac
  case "$path" in
    internal/i18n/*)
      select_contexts i18n
      known=1
      ;;
  esac
  case "$path" in
    internal/interface/clientapi/* | app/web/src/lib/api/* | scripts/generate.sh | scripts/api-spec-check.sh)
      select_contexts api
      known=1
      ;;
  esac
  case "$path" in
    app/ui/* | app/tsconfig.base.json | app/.oxlintrc.json | app/oxlint/* | app/knip.json | scripts/ui-test-docker.sh)
      select_contexts ui
      known=1
      ;;
  esac
  case "$path" in
    # make web-tokens fails when the token list generated from app/ui/src/theme.css is stale.
    app/* | scripts/playwright-image-check.sh | scripts/web-icons.sh | internal/domain/instance/theme_tokens.gen.go)
      select_contexts web
      known=1
      ;;
  esac
  case "$path" in
    Dockerfile | .dockerignore | deploy/* | scripts/docker-smoke.sh | scripts/federation-live.sh)
      select_contexts docker
      known=1
      ;;
  esac
  case "$path" in
    scripts/*.sh)
      select_contexts shell
      known=1
      ;;
  esac
  case "$path" in
    # Files no check reads.
    LICENSE | .editorconfig | .claude/* | .github/*)
      known=1
      ;;
  esac
  if [ "$known" -eq 0 ]; then
    select_contexts "${ALL[@]}"
  fi
}

while IFS= read -r path; do
  [ -n "$path" ] && classify "$path"
done <<< "$files"

# Dependents: @my/ui is compiled into app/web, the API code is generated into Go and app/web,
# the Go i18n accessors are Go, and the schema is embedded into Go.
[ -n "${selected[ui]:-}" ] && select_contexts web
[ -n "${selected[api]:-}" ] && select_contexts go web
[ -n "${selected[i18n]:-}" ] && select_contexts go

for c in "${ALL[@]}"; do
  if [ -n "${selected[$c]:-}" ]; then
    echo "$c"
  fi
done
