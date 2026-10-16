# AGENTS.md

Instructions for coding agents (Claude Code, Codex, Copilot, Gemini, etc.) working in this repository.
Human contributors should read [CONTRIBUTING.md](CONTRIBUTING.md); the rules are the same.

## Project in one paragraph

An ActivityPub-compatible microblog server written in Go, designed to run at near-zero cost on Google Cloud, AWS,
or a self-hosted VM (Docker / microk8s), with swappable RDB / queue / storage adapters, Mastodon-compatible client API,
a PWA web client, a Tauri 2 mobile app, and i18n from day one. Distributed as OSS.

## Current phase: M0 — Design & Harness

- **No production code yet.** The server is Go ([ADR-0001](docs/adr/0001-server-language-and-runtime.md), Accepted); the web client and Tauri UI are TypeScript with Bun.
  Cloudflare Workers is **not** a target. Do not scaffold application code or pick libraries until the relevant ADR is *Accepted*
  (spikes may try libraries under `spikes/`).
- Work in this phase is: refining requirements, drafting/refining ADRs, spikes (throwaway, clearly marked), and harness/tooling.

## Source of truth

| What | Where |
| --- | --- |
| Vision, scope, milestones | [docs/requirements/00-vision.md](docs/requirements/00-vision.md) |
| Functional requirements (`FR-xxx`) | [docs/requirements/01-functional.md](docs/requirements/01-functional.md) |
| Non-functional requirements (`NFR-xxx`) | [docs/requirements/02-non-functional.md](docs/requirements/02-non-functional.md) |
| Architecture overview, ports, flows | [docs/architecture/overview.md](docs/architecture/overview.md) |
| Hosting targets & cost notes | [docs/architecture/deployment-targets.md](docs/architecture/deployment-targets.md) |
| Decisions | [docs/adr/README.md](docs/adr/README.md) — **Accepted ADRs are binding; Proposed ADRs are not yet decisions** |
| Tasks | GitHub Issues / Milestones / Project of `best-friends/my` |
| Agent playbooks | [docs/agents/](docs/agents/) |

## How to pick and do work

Follow [docs/agents/workflow.md](docs/agents/workflow.md). Summary:

1. Only take issues labeled **`agent:ready`** with no assignee. Never take `status:needs-decision`.
2. Branch `<type>/<issue>-<slug>`; never push to `main`.
3. Run `make check` before every commit; it must pass.
4. Open a PR using the template, with `Closes #N` and the acceptance criteria checked.
5. If you need a decision that no Accepted ADR covers: small → state the assumption in the PR; large → draft a *Proposed* ADR
   (`make adr-new TITLE=... SLUG=...`), comment on the issue, add `agent:blocked`, and stop.

## Hard rules

- **Never change an ADR's status to `Accepted`/`Rejected` on your own judgment.** Only the human decider decides. You may record a decision
  the decider has explicitly stated (quote it in the PR description). You may create and edit *Proposed* ADRs.
- Do not rewrite the body of an Accepted ADR; supersede it with a new ADR instead (typo fixes are fine).
- After adding/renaming an ADR, run `make adr-index`.
- v0 allows breaking changes everywhere ([ADR-0012](docs/adr/0012-v0-breaking-change-policy.md)) — no migrations, no compat shims.
  Exception: ActivityPub / Mastodon API wire formats and federated URIs must stay spec-compliant.
- Language: design docs (ADR, requirements) in Japanese during v0; code, identifiers, code comments, commit messages, and PR titles in English.
- Commits follow Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`; `!` for breaking).
- License is AGPL-3.0-only ([ADR-0011](docs/adr/0011-license.md)). New source files start with `SPDX-License-Identifier: AGPL-3.0-only`;
  only add dependencies with AGPL-compatible licenses.
- Do not commit secrets, `.env` files, or credentials. Do not run destructive commands against remote resources.

## Design rules that will apply once implementation starts

These come from Proposed ADRs and will be confirmed when they are accepted; follow them in drafts and spikes.

- Core (domain + application) must not import cloud SDKs or drivers; those live in adapters and `cmd/` entrypoints.
- Keep the server buildable with `CGO_ENABLED=0`; a cgo dependency needs an ADR.
- Every external resource goes through a port; every adapter must pass that port's contract test suite.
- Keep DB transactions short, retryable, and free of external I/O (HTTP, queue, blob); enqueue jobs via the outbox. Make writes safe to repeat with unique constraints, upserts, idempotency keys ([ADR-0003](docs/adr/0003-database-abstraction.md)).
- Job handlers must be idempotent; delivery is at-least-once ([ADR-0004](docs/adr/0004-job-queue-abstraction.md)).
- All outbound HTTP goes through the SSRF-guarded `HttpClient` port.
- Visibility checks go through the single domain policy function; never re-implement them in API or federation code.
- **i18n**: never hard-code user-facing strings or date/number formats. Add ICU messages to `locales/en` *and* `locales/ja`;
  use CSS logical properties; render user content with `dir="auto"` ([ADR-0009](docs/adr/0009-internationalization.md)).

## Commands

| Command | Purpose |
| --- | --- |
| `make check` | Every offline check CI runs: docs lint, ADR index check, and `make go-check` |
| `make go-check` | `go mod tidy` check, `go vet`, `golangci-lint`, `go test`, `CGO_ENABLED=0 go build`, license check |
| `make go-lint` / `go-test` / `go-build` | Run one Go step (`go-lint` uses `.golangci.yml`) |
| `make go-licenses` | Fail on Go dependency licenses incompatible with AGPL-3.0 (allowlist in `Makefile`) |
| `make go-vuln` | govulncheck (needs network; run in CI, not part of `make check`) |
| `make lint-shell` | shellcheck on `scripts/*.sh` |
| `make adr-new TITLE="..." [SLUG=...]` | Create a new Proposed ADR (pass `SLUG` when the title is not ASCII) |
| `make adr-index` | Regenerate the ADR table in `docs/adr/README.md` |
| `make lint-docs` | markdownlint on all Markdown files |

Toolchain versions (Go, golangci-lint, Bun, shellcheck) are pinned in `.tool-versions` (asdf / mise).
Go tools (govulncheck, go-licenses) are pinned with `tool` directives in `tools/go.mod` and run via
`go tool -modfile=tools/go.mod <tool>`. Go steps skip with a message while the root module has no packages.
Spikes under `spikes/` are separate modules and are excluded from root Go checks.
