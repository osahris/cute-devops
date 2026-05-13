---
status: draft
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Push to Deploy 🚀

> **Pattern (draft).** Companion to [Worktree Treehouses 🌳](../patterns/approaches/worktree-treehouses.md). Adds a deploy step to the treehouse lifecycle: after a merge to `main`, `git push <remote>` reaches a deployment target. Same primitive as everything else.

## Goal

Make deployment a `git push`. Whatever the target — a VM with a `post-receive` hook, an ssh remote that pulls and restarts, a forge like GitLab / Forgejo running CI on push, a PaaS like Dokku — it's all just "a git remote". No bespoke deploy command, no upload protocol, no separate pipeline language.

## Shape

The deploy target lives behind a git URL. The bare repo treats it as a remote:

```bash
cd /srv/repos/foo.git
git remote add deploy ssh://deploy@example.com/srv/apps/foo.git
git push deploy main
```

What "deploy" means is the target's business: a `post-receive` hook that checks out and restarts a service, a forge pipeline, a PaaS build, a VM pull. The treehouse-side workflow doesn't care which.

## Automating from the bare repo

Extend the bare repo's `reference-transaction` hook (already syncing config on merge) to also push `main` to the deploy remote:

```bash
git push --quiet deploy main
```

A merge becomes: maintainer merges → reference-transaction fires → config syncs → `main` pushes to deploy → target deploys. One click, no separate ritual.

For multi-target setups, add multiple remotes (`staging`, `prod`) and gate which fires on what. Common pattern: `staging` on every `main` advance, `prod` on tags.

## Lifecycle (extended)

```
   idea ──► git worktree add …
                       │
                       ▼
              treehouses/<branch>/
                       │
                       ▼
              MR / merge into main
                       │
                       ▼
              git push deploy main      ← often automated
                       │
                       ▼
              git worktree remove
```

## Why it fits

- **Same primitive.** Treehouses share refs with the bare repo (no push). The bare repo pushes to deploy remotes (no special protocol). Every transfer is `git`.
- **Auditable.** `git log` on the target shows what landed and when. No "what's in prod?" mystery.
- **Reversible.** Rollback is `git push deploy <old-tip>:main --force-with-lease` — or the target keeps a ring of past tips and points an `active` symlink.

## Open questions

- Push from the `reference-transaction` hook, or from a separate `post-receive` on `main`? (Probably reference-transaction — same place already handles sync.)
- Deploys that need secrets the bare repo doesn't have: secrets live on the target; the push only transfers code. Codify this?
- Tag-based vs. branch-based deploys — recommend one, or stay agnostic?

## Acceptance

Promote to `patterns/operation/push-to-deploy.md` once a role or two in this collection ships this shape and the multi-target story is concrete.
