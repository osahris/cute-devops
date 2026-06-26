---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Rename collection to osahris.cute_devops

## Goal

Rename the collection from `mkbrechtel.devops` to **`osahris.cute_devops`**.

The `mkbrechtel` namespace is too personal and `devops` is too generic.
Move to the `osahris` org namespace and name the collection `cute_devops`
to match the project's identity (***Cute DevOps!***).

## Scope

- Project name: `osahris.cute_devops`.
- `galaxy.yml`: `namespace: osahris`, `name: cute_devops`. Claim the
  `osahris` namespace on Ansible Galaxy.
- Update all FQCN references across roles, playbooks, docs, and tests
  (`mkbrechtel.devops.<role>` → `osahris.cute_devops.<role>`).
- Update README, `CLAUDE.md`, `improve/coding.md`, `improve/release.md`,
  `GLOBAL.md`, `REUSE.toml` as needed.
- Update managed-file-header convention strings
  (`mkbrechtel.devops.<role>` → `osahris.cute_devops.<role>`).
- Update GitHub Actions / Galaxy publishing workflow.
- Git repo rename as part of this ticket. GitHub redirects the old slug
  automatically; CI integrations re-point.
- Update website install instructions and any
  `ansible-galaxy collection install` snippets.
