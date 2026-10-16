---
name: adr
description: Draft a new Architecture Decision Record or revise a Proposed one in docs/adr, following this repo's ADR policy. Use when asked to write, update, or propose a design decision.
---

# ADR skill

1. Read `docs/agents/writing-adrs.md` and `docs/adr/README.md` (to see existing decisions and avoid overlap).
2. Read the requirements referenced by the topic in `docs/requirements/` and any related ADRs.
3. For a new ADR run `make adr-new TITLE="<Japanese title>" SLUG=<kebab-case-english>`; otherwise edit the existing file.
4. Fill every section of the template. Mark unverified third-party facts as 要検証; verify key ones with web search when possible and cite the source.
5. Never set the status to Accepted or Rejected.
6. Run `make adr-index` and `make check`.
7. If the ADR needs a decision from the owner, open or update a `type:adr` issue with `status:needs-decision` and a 5-line summary.
