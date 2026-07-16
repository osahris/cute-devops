---
status: draft
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Workshop Branch Shape — `<category>/<name>` 🪧

> **Pattern (draft).** Optional companion to [Worktree Workshops 🛠️](../patterns/approaches/worktree-workshops.md). Originally lived inside that pattern; lifted out because branch-naming policy is a separate concern that some projects will adopt and others won't.

## Goal

Make `ls workshops/` self-organise into legible categories without dictating which categories exist. Treat the slash-separated branch name as both a directory layout and a soft taxonomy.

## Shape

Every workshop lives at `workshops/<category>/<name>`. The branch name carries the same shape:

- `feature/cute-thing`
- `fix/login-redirect`
- `refactor/auth`
- `bot/auto-bump`
- `experiment/new-router`

The only exempt slot is `main/` — the maintainer's workshop.

## Enforcement

The bare repo's `pre-receive` (or `update`) hook rejects any ref outside `refs/heads/<category>/<name>` (and `refs/heads/main`). Because workshops share the bare repo's refs database, that hook fires the moment you commit in a workshop — there's no "push later" step where someone could sneak a malformed name through.

Don't hard-code the allowed categories. The shape is the discipline; which prefixes a project uses is the project's call.

## Why it's optional

A small project with three branches doesn't need a taxonomy; bare top-level names like `workshops/quickfix/` work fine. The pattern earns its keep when `ls workshops/` would otherwise be a flat wall of names — `workshops/feature/x` and `workshops/feature/y` rolling up under `workshops/feature/` make "what features are in flight?" a one-line `ls`.

## Open questions

- Should the hook offer projects a config knob (`hooks.allowedCategories = feature fix refactor`) or stay open?
- Does the `main` exemption generalise to other top-level long-lived refs (`release/*`, `stable`)?
- Is there a place for a second-level prefix to signal lifetime (long-lived bench vs. throwaway job)?

## Acceptance

Promote to `patterns/approaches/workshop-branch-shape.md` when one or more existing patterns / projects in this collection adopt it and the open questions resolve.
