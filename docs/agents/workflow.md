# Agent workflow playbook

The step-by-step procedure an agent follows for one issue. Policy background: [ADR-0014](../adr/0014-task-management-and-agent-workflow.md).

## 1. Pick

```sh
gh issue list -R best-friends/my --label agent:ready --search "no:assignee" --json number,title,milestone,labels
```

- Prefer the earliest milestone, then `priority:p0` > `p1` > `p2`.
- Read the full issue including comments: `gh issue view <N> --comments`.
- Check that every ADR the issue depends on is `Accepted` in [docs/adr/README.md](../adr/README.md). If not, comment and stop.
- Claim it: `gh issue edit <N> --add-assignee @me` and comment a one-line plan.

## 2. Understand

- Read the requirement IDs (`FR-xxx`, `NFR-xxx`) and ADRs referenced by the issue.
- If the acceptance criteria are ambiguous, ask in an issue comment, add `agent:blocked`, and stop — do not guess on scope.

## 3. Branch

```sh
git switch main && git pull --ff-only
git switch -c <type>/<N>-<short-slug>     # e.g. task/42-sqlite-adapter, spike/17-sqlc-dialects
```

## 4. Implement

- Keep the change scoped to the issue, but apply the boy scout rule ([ADR-0024](../adr/0024-boy-scout-rule.md)):
  fix broken things you find in a separate commit and list them under "ついでに直したもの" in the PR; file an issue for anything you cannot fix safely.
- Update docs that the change makes stale (ADR open questions, architecture overview, i18n catalogs).
- Spikes ([ADR-0022](../adr/0022-spike-outputs-outside-repository.md)):
  - Push throwaway code to a `spike/<N>-<slug>` branch only. Never open a PR to merge it.
  - Post the findings report as a comment on the issue, linking the code by commit permalink.
  - Put the conclusions into ADRs / docs in the present tense, from a separate `docs/<N>-<slug>` branch and PR, linking the report comment as evidence.

## 5. Verify

```sh
make check-changed        # the contexts changed since origin/main and their dependents, in parallel
make go-check             # or one context: docs-check, go-check, i18n-check, api-check, ui-check, web-check, docker-check
make check                # every context, when the change touches shared configuration
```

Must pass. CI runs every affected context again and its `check` gate decides the merge.
If a check is wrong or flaky, fix it in a separate commit and explain in the PR.

Pull requests do not run the web E2E. When the change touches the web client, run `make web-e2e` locally, or run
the workflow on the PR's branch and wait for it:

```sh
gh workflow run web-e2e.yml -R best-friends/my --ref <branch>   # add -f matrix=full for the full width × locale matrix
gh run list -R best-friends/my --workflow web-e2e.yml --branch <branch> --limit 1
```

### What CI runs where

The layout of [ADR-0047](../adr/0047-ci-protection-layers.md) (`.github/workflows/`):

| When | Workflow | What runs |
| --- | --- | --- |
| Pull request | `ci.yml` | The `ctx-<context>.yml` of every context the change touches and of its dependents (`scripts/changed-contexts.sh`), the same checks as `make check`, with the PostgreSQL tests on pgmem; the `@my/ui` story tests (`ui-test.yml`) when `app/ui` changes; the linux/arm64 image builds when the docker context changes; the Docker Compose smoke test when the docker context, `cmd/`, or `internal/platform/` changes. The required `check` collects the results |
| Merge queue | `ci.yml` | The same contexts again on the merge group (the PR on top of `main` and the PRs queued ahead of it), without the checks decided file by file (markdownlint, Go and npm licenses, shellcheck, the CSS `100vh` check, design tokens, the Playwright image version) and without the story tests, the smoke test, and the arm64 builds. The queue merges the group when `check` passes |
| Push to `main` | `main.yml` | The web E2E subset on 4 shards; the smoke test unless the push changes docs only; the story tests when `app/ui` changes; the PostgreSQL tests on a real container (`postgres-contract.yml`) and the SNS / SQS tests on the AWS emulator (`aws-contract.yml`), each when a path they depend on changes; the Go and Bun caches that pull requests and the queue restore. The `make check` contexts do not run again |
| Nightly (03:00 JST) | `nightly.yml` | The full web E2E matrix on 8 shards, the live interoperability test (`federation-live.yml`), the PostgreSQL tests on a real container, the SNS / SQS tests on the AWS emulator, the smoke test, and the arm64 builds |

`ctx-<context>.yml`, `nightly.yml`, and `federation-live.yml` also run by hand (`gh workflow run <file> --ref <branch>`).

## 6. Commit & PR

- Conventional Commits in English; reference the issue (`feat(db): add sqlite adapter (#42)`).
- Push the branch and open a PR with the template:

```sh
git push -u origin HEAD
gh pr create --fill --body-file - <<'EOF'
...template contents...
EOF
```

- Comment on the issue with the PR link. Do not merge or queue it yourself; the orchestrator or the decider queues it (§7).
- Wait for the PR's CI (`gh pr checks <PR> -R best-friends/my --watch`) and fix failures until it is green.

## 7. Merge through the merge queue

`main` takes pull requests only through GitHub's merge queue. CI runs again on each merge group (the PR on top of
`main` and of the PRs queued ahead of it) and the queue merges the group when its required `check` passes.

- Once the PR's CI is green, the orchestrator or the decider queues it: `gh pr merge <PR> -R best-friends/my --auto --merge`.
- Merge `main` into the branch only when the PR conflicts or the queue drops it, never just to stay current: the queue
  already checks the PR on top of the latest `main`.
- When the queue drops the PR (a conflict with `main`, or a failed `check` on the merge group), merge `main` into the
  branch, regenerate the generated files instead of resolving their conflicts by hand, run the checks, and push;
  the orchestrator or the decider queues it again:

```sh
git fetch origin && git merge origin/main
make generate sqlc i18n-gen web-dist   # the ones whose sources changed: client API code, sqlc code, i18n accessors
                                       # and pseudo-locales, app/web/src/routeTree.gen.ts
make check-changed
```

## 8. When blocked

- Comment on the issue: what you tried, what decision is needed, the options you see, and your recommendation.
- `gh issue edit <N> --add-label agent:blocked`
- If the decision is architectural, draft a Proposed ADR in the same branch and link it.
