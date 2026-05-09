---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# claude-code

## Goal

A `claude_code` role that installs Anthropic's Claude Code CLI system-wide on hosts where users will use it interactively (typically a devbox), with a centrally-managed configuration at `/etc/claude-code/` that disables self-upgrade, appends a host-level system prompt, and supplies skills + MCP servers — all without touching per-user homes for the central pieces.

## Scope

### Install: apt package, stable channel

Install `claude-code` from Anthropic's apt repo on the **stable channel** by default. Version control via standard apt-pinning. No npm, no native-installer wrapper, no per-user installs — just the package. Users get the version the apt repo currently ships; admins control upgrade cadence by pinning or by tracking the channel.

### Centrally-managed configuration at `/etc/claude-code/`

The load-bearing piece. The role drops files under `/etc/claude-code/` that Claude Code reads on every invocation — host-wide, no per-user duplication. Concretely:

- **System settings file** that disables self-upgrade (the managed setting that makes user-side `claude update` a no-op or refuses the operation).
- **Append-style system prompt** at `/etc/claude-code/CLAUDE.md` (or whatever path the managed-config schema uses): host-flavoured guardrails appended to whatever context Claude Code would otherwise have. Same file for every user on the host; one place to edit.
- **Skills + MCP server list** delivered as a **local plugin marketplace**: the role clones a configured list of git repos into `/etc/claude-code/plugins/<name>/` at converge time, and the managed config points Claude Code at that directory as a plugin source. Adding a skill set or MCP bundle for the host is a one-line inventory addition (`<repo-url>` in the list) plus a converge run; Claude Code picks them up via its native plugin-discovery, no per-user file rendering. User customizations merge on top of the marketplace; the central marketplace is the floor, not the ceiling.

### Per-user authentication is the user's concern

Each user runs `claude login` once; tokens land in `~/.claude/credentials.json`. The role does not wrap, intercept, or pre-provision auth. For long-lived system-agent tokens, the secrets role can hand a credential to a service account — but that's a separate concern, out of scope here.

### What this role explicitly does not do

- **Wrap or limit agentic execution.** Memory limits, sandboxing, command-allow-lists for agentic tool use — none of that lives here. If a host needs them, that's a separate role concerned with agent execution.
- **Manage system agents.** Long-lived non-interactive Claude Code processes are a different shape; out of scope.
- **Per-user template files.** No `~/.claude/CLAUDE.md` rendered from inventory variables, no per-user MCP file, no per-user dotfile interpolation. Everything host-wide goes through `/etc/claude-code/`; per-user customization lives in `~/.claude/` and the user owns it.

## Open questions

_None._
