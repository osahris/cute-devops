---
status: draft
---

<!--
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Village of Villages 🏘️

> **Pattern (draft).** Multi-repo extension of [Worktree Treehouses 🌳](../patterns/approaches/worktree-treehouses.md). Lifted out of that pattern because it's a separate concern most single-repo projects don't need.

## Goal

For organisations running several related bare repos, give them a shared notice board and shared agent assets without inventing a heavyweight monorepo.

## Layout

```
/srv/orgs/acme/
├── README.md                     ← onboarding for the whole org
├── CLAUDE.md → README.md
├── .claude/                      ← shared agent skills / settings
├── manifest.yaml                 ← lists member repos + their roles
└── repos/
    ├── frontend.git/             ← each is itself a treehouse village
    │   ├── treehouses/{main, feature/x, ...}
    │   └── .claude/, hooks/, README.md
    ├── backend.git/
    │   └── treehouses/...
    └── infra.git/
        └── treehouses/...
```

Each bare repo still follows the single-repo treehouse pattern internally. The org directory adds nothing to the per-repo workflow.

## What it provides

- **Cross-repo docs.** README for "what is acme, what does each repo do, where do issues go?"
- **Shared agent assets.** Skills, slash commands, prompts that apply org-wide live in the org's `.claude/`; per-repo `.claude/` inherits them by reference.
- **A manifest.** Plain YAML naming each member repo, who maintains it, and any version-pinning rules.

## What it deliberately does not do

- **No org-level worktree.** Treehouses live inside the individual repos, not at the org root.
- **No shared `main` treehouse.** Each repo's maintainer owns their own `treehouses/main/`; an org-wide merge is several per-repo merges.

## Open questions

- How does per-repo `.claude/` actually inherit from the org's `.claude/`? Symlinks, settings.json `extends`, a sync hook?
- What goes in `manifest.yaml`, exactly — just names + maintainers, or also dependency edges / version pins?
- A "composed treehouse" (one directory holding coordinated worktrees of several repos on related branches) is tempting but needs real tooling. Defer.

## Acceptance

Promote to `patterns/approaches/village-of-villages.md` once an actual multi-repo project in this collection runs on this shape and the inheritance-by-reference story for `.claude/` is concrete.
