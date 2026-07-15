---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Devbox host profile

## Goal

A `devbox` host profile that composes a browser-based development environment from three component roles: a terminal in the browser, an editor in the browser, and a managed Claude Code install. The `devbox` role itself is an orchestrator — it pulls in the components, sets defaults that make them fit together, and adds nothing beyond what each component already provides.

## Scope

The `devbox` role requires the following three roles by default (and applies sensible composition defaults across them):

- [ttyd](ttyd.feature.md) — browser-based terminal.
- [code-server](code-server.feature.md) — browser VS Code per user.
- [claude-code](claude-code.feature.md) — pinned-version Claude Code with central guardrail configuration.
