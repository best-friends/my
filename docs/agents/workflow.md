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

## 6. Commit & PR

- Conventional Commits in English; reference the issue (`feat(db): add sqlite adapter (#42)`).
- Push the branch and open a PR with the template:

```sh
git push -u origin HEAD
gh pr create --fill --body-file - <<'EOF'
...template contents...
EOF
```

- Comment on the issue with the PR link. Do not merge.

## 7. When blocked

- Comment on the issue: what you tried, what decision is needed, the options you see, and your recommendation.
- `gh issue edit <N> --add-label agent:blocked`
- If the decision is architectural, draft a Proposed ADR in the same branch and link it.
