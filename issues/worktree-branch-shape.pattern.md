---
status: absorbed
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Branch Shape — `<category>/<branch>` 🪧

> **Absorbed.** This draft proposed the `<category>/<name>` branch shape as an
> optional companion. It is now a **mandatory rule** inside
> [Shared Worktrees 🌳](../patterns/approaches/shared-worktrees.md): every
> worktree name is `<category>/<branch>`, where `<category>` is simply a
> sub-folder under the work directory. The notes below survive as rationale and
> open enforcement ideas.

## The rule (now core)

Every worktree lives at `/work/<project>/<category>/<branch>` on a branch named
`work/<category>/<branch>`. The name carries the same shape everywhere:

- `feature/cute-thing`
- `fix/login-redirect`
- `refactor/auth`
- `bot/auto-bump`
- `experiment/new-router`

`<category>` is just a sub-folder under the work directory
(`/work/<project>/<category>/`), created on demand with the same permissions as
the work directory itself (mode `3775`, setgid+sticky+group-write). There's no
allowlist file to check against — the default folders are `feature`, `fix`,
`refactor`, `chore`, `docs`, `experiment`, and `bot`, but any folder name works.
`<branch>` is a URL slug: lowercase `a-z`/`0-9`, starts with a letter, contains
at least one dash, at most 40 characters.

The only exempt slot is `main` — the mainline the maintainer merges into.

## Rationale (kept)

Listing `/work/<project>/` self-organises into legible categories:
`feature/x` and `feature/y` roll up under `feature/`, so "what features are in
flight?" is a one-line `ls`. Treating the slash-separated name as both a
directory layout and a soft taxonomy is what makes the work directory readable
at a glance.

## Enforcement idea (still open)

A `pre-receive` (or `update`) hook on the bare repo could reject any ref outside
`refs/heads/work/<category>/<branch>` (and `refs/heads/main`). Because worktrees
share the bare repo's refs database, that hook fires the moment you commit —
there's no "push later" step where a malformed name could sneak through. The
Claude Code worktree hooks already validate the shape at creation time; a
server-side hook would close the gap for hand-rolled `git worktree add`.

Open threads:

- Categories are just folders (no allowlist), so the shape — not the set of
  prefixes — is the only thing worth enforcing. Is creation-time validation of
  the `<category>/<branch>` shape enough, or is a server-side hook worth it?
- Does the `main` exemption generalise to other long-lived refs
  (`release/*`, `stable`)?
- Is there room for a second-level prefix to signal lifetime (long-lived vs.
  throwaway)?
