# agentkit

Context and workflow tools for coding agents working in best-friends/my. Agents get the parts of the documents,
code, and issues a task needs, and run the workflow steps of [docs/agents/workflow.md](../../docs/agents/workflow.md),
at a fraction of the tokens that grep, cat, and polling cost. Decisions: [docs/adr/](docs/adr/README.md).

Run it through `scripts/agentkit.sh <command>`, which builds `bin/agentkit` when it is missing or stale.
`scripts/agentkit.sh help` lists every command and flag.

## Reading

| Instead of | Run |
| --- | --- |
| Reading the issue, then every ADR, requirement, and document section it names | `ctx <issue> [--layer server\|web]`: one pack within a token budget (default 30k), also saved to `.tmp/agentkit/ctx/<n>.md` for subagents |
| `grep -rn` over the repository | `find <words> [--kind adr,section,go,ts,op,table,query,...] [--under <path>]`: ranked locations with sizes |
| `cat` of a large document | `outline <path>`, then `show '<path>#<anchor>'`, `show 'data-model§4.4'`, `show 'ADR-0070#decision'`, `show FR-915`, `show <path>:<from>-<to>` |
| Following a request through the layers by hand | `trace op:<operationId>` or `trace <GoName>`: handler → app → adapter → query → table |
| Grepping for the users of a name | `refs <ref>`: what points at it (documents, code, queries, client calls) and what it points at |

`show` prints the outline instead of the text when a node is bigger than `--max` tokens (default 12k).
References: node ids (`ADR-0070`, `FR-915`, `op:<id>`, `table:<name>`, `query:<Name>`, `schema:<Name>`,
`sym:<dir>.<Recv>.<Name>`, `ts:<path>:<Name>`), short names (`SubmitRequest`, `custom_emojis`), sections by number or
anchor of any unique path suffix (`data-model§4.4`, `web-client#5-アクセシビリティ`), and line ranges.
A document split into one file per section (`docs/architecture/web-client/`, `docs/architecture/data-model/`) is
looked up by its directory: `web-client§14` finds the section in whichever file holds it, and `show web-client`
prints the directory's `README.md`.

## Workflow

| Step | Run |
| --- | --- |
| Take an issue (§1) | `issue claim <n> --plan "<one line>"`: refuses when the issue is not `agent:ready`, needs a decision, is assigned to someone else, depends on a non-Accepted ADR, or waits for an open issue |
| Verify (§5) | `check`: `make check-changed` with the contexts, their results, and only the failing lines |
| Open the PR (§6) | `pr open [--summary-file f] [--decisions text] [--confirm] [--footer text] [--base <branch>]` |
| Wait for CI (§6) | `pr wait <pr>` **in the background**: one line when CI passes; the failing lines of the failed jobs otherwise; exit 3 on a conflict |
| Resolve a conflict (§7) | `git merge origin/main`, `regen`, `check`, push |
| Watch many PRs (§9) | `pr watch [<pr>...]` with the Monitor tool: one line per opened, merged, closed, conflicting, ci-failed, ci-green, dequeued |
| Stop (§8) | `issue block <n> --reason-file f` |

## The context map

[context-map.yaml](context-map.yaml) maps paths to the document sections that govern them, their check contexts,
whether parallel work treats them as shared files, and short notes. `ctx` uses it to pick the rules of the areas an
issue touches. When you add a directory, module, adapter, or feature, add or extend its entry in the same change:
`make agentkit-check` (and CI, on every change) fails when a file is not covered, when a directory under a `strict`
pattern has no entry naming it, or when a document reference does not resolve ([ADR-0003](docs/adr/0003-context-map.md)).
`map show <path>` prints the entries of a path.

## Claude Code hooks

`.claude/settings.json` runs `scripts/agentkit-hook.sh` ([ADR-0004](docs/adr/0004-claude-code-hooks.md)): at session
start it builds and indexes; it denies reading a whole Markdown file of 40KB or more (code: 80KB) and watching CI with
`gh pr checks --watch` / `gh run watch`, with the agentkit command to use instead; it hints at `find` for unbounded
recursive greps; and it cuts Bash output over 20,000 characters to its head and tail, saving the whole to
`.tmp/agentkit/out/`. `AGENTKIT_BIG_FILE` and `AGENTKIT_MAX_OUTPUT` change the limits.

## Measuring

`stats [--since 2026-10-01] [--until ...]` reads the Claude Code transcripts of this repository and its worktrees
(`~/.claude/projects/`) and prints the API token use and the tool output by kind. Compare the periods before and after
a change to see what it saved.

## How it works

- `internal/extract` turns each file into nodes (Markdown sections, requirement rows, Go and TypeScript declarations,
  tables, sqlc queries, OpenAPI operations and components, messages) and edges (links, ID mentions, code spans, imports,
  calls by name, API calls by path, tables a query uses). Generated files are left out.
- `internal/store` keeps them in SQLite (FTS5 with the trigram tokenizer, so Japanese needs no morphological analysis)
  at `$(git rev-parse --git-common-dir)/agentkit/kb.db`, keyed by path and blob id: worktrees share it, and each command
  indexes only what changed (the first build takes seconds, later ones tens of milliseconds).
- `internal/gh` talks to GitHub through `gh` and caches issues and merged pull requests (with their changed files) in
  the same database.
- `internal/kb` answers `find` / `show` / `outline` / `refs` / `trace`; `internal/pack` builds `ctx`; `internal/ops`
  runs the workflow steps; `internal/hook` implements the hooks; `internal/stats` measures.

## Developing

`make agentkit-check` runs `go mod tidy -diff`, `go vet`, golangci-lint (`.golangci.yml` here), the tests, the
dependency check (only module versions the server's `go.sum` has, so `make go-licenses` covers their licenses), and
`map check`. Bump `extract.Version` when an extractor's output changes, so that indexes are rebuilt.
