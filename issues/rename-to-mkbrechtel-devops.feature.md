---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Rename collection to mkbrechtel.devops

## Goal

Rename the collection from `mkbrechtel.sysops` to **`mkbrechtel.devops`**. 

Framing — small-team-scale development and infrastructure automation

## Scope

- Project name: `mkbrechtel.devops`.
- `galaxy.yml`: `namespace: mkbrechtel`, `name: devops`. The existing namespace claim carries over from `mkbrechtel.sysops`.
- Update all FQCN references across roles, playbooks, docs, and tests (`mkbrechtel.sysops.<role>` → `mkbrechtel.devops.<role>`).
- Update README — new name, plus a clear "who this is for" framing: a handful of admins running a few dozen hosts, comfortable being outside platform-team territory.
- Update `CLAUDE.md`, `CODING.md`, `RELEASE.md`, `GLOBAL.md`, `REUSE.toml` as needed.
- Update managed-file-header convention strings (`mkbrechtel.sysops.<role>` → `mkbrechtel.devops.<role>`).
- Update GitHub Actions / Galaxy publishing workflow.
- Git repo rename (`mkbrechtel/sysops` → `mkbrechtel/devops`) as part of this ticket. GitHub redirects the old slug automatically; CI integrations re-point.
- Drop in-progress references to "smalltown-devops" / "smalltown_devops" from sibling tickets that mentioned it (search-and-replace).
