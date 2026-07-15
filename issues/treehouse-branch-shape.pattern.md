---
status: draft
---

<!--
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Treehouse Branch Shape — `<category>/<name>` 🪧

> **Pattern (draft).** Optional companion to [Worktree Treehouses 🌳](../patterns/approaches/worktree-treehouses.md). Originally lived inside that pattern; lifted out because branch-naming policy is a separate concern that some projects will adopt and others won't.

## Goal

Make `ls treehouses/` self-organise into legible categories without dictating which categories exist. Treat the slash-separated branch name as both a directory layout and a soft taxonomy.

## Shape

Every treehouse lives at `treehouses/<category>/<name>`. The branch name carries the same shape:

- `feature/cute-thing`
- `fix/login-redirect`
- `refactor/auth`
- `bot/auto-bump`
- `experiment/new-router`

The only exempt slot is `main/` — the maintainer's treehouse.

## Enforcement

The bare repo's `pre-receive` (or `update`) hook rejects any ref outside `refs/heads/<category>/<name>` (and `refs/heads/main`). Because treehouses share the bare repo's refs database, that hook fires the moment you commit in a treehouse — there's no "push later" step where someone could sneak a malformed name through.

Don't hard-code the allowed categories. The shape is the discipline; which prefixes a project uses is the project's call.

## Why it's optional

A small project with three branches doesn't need a taxonomy; bare top-level names like `treehouses/quickfix/` work fine. The pattern earns its keep when `ls treehouses/` would otherwise be a flat wall of names — `treehouses/feature/x` and `treehouses/feature/y` rolling up under `treehouses/feature/` make "what features are in flight?" a one-line `ls`.

## Open questions

- Should the hook offer projects a config knob (`hooks.allowedCategories = feature fix refactor`) or stay open?
- Does the `main` exemption generalise to other top-level long-lived refs (`release/*`, `stable`)?
- Is there a place for a `cottage` or `tent` second-level prefix to signal lifetime (long-lived vs throwaway)?

## Acceptance

Promote to `patterns/approaches/treehouse-branch-shape.md` when one or more existing patterns / projects in this collection adopt it and the open questions resolve.
