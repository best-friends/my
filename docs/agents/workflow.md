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
git switch -c <type>/<N>-<short-slug>     # e.g. task/42-sqlite-adapter, spike/17-fedify-workers
```

## 4. Implement

- Keep the change scoped to the issue. Unrelated fixes go in a new issue.
- Update docs that the change makes stale (ADR open questions, architecture overview, i18n catalogs).
- Spikes: put throwaway code under `spikes/<N>-<slug>/` and the findings report in the issue (or `docs/spikes/<N>-<slug>.md`).
  Spike code is never imported by product code.

## 5. Verify

```sh
make check
```

Must pass. If a check is wrong or flaky, fix it in a separate commit and explain in the PR.

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
