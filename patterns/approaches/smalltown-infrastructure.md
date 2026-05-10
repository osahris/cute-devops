---
title: Smalltown Infrastructure 🏘️
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## Overview 📋

Most teams don't run at hyperscale. They have a handful of operators,
a manageable list of services, and quiet weekends to protect. The
*Smalltown Infrastructure* approach is about building for that
reality: simple systems, dev/prod alignment, and small composable
components glued together with managed configuration — instead of
adopting big enterprise products designed for a thousand-engineer
org you don't have.

## Goals 🎯

- Keep the operator's mental model whole — what runs in prod can
  run on a laptop.
- Avoid platforms that demand more team than you have.
- Lean on boring, stable building blocks (Debian packages, systemd,
  small daemons) and make them cute through managed config.
- Trade vendor convenience for legibility: every layer should be
  inspectable with `grep`, `journalctl`, and `systemctl status`.
- Stay close to the humans behind the tools — small, approachable
  communities of real people, not sales-and-PR machines.

## Principles 🔧

### Dev ≈ Prod

The development environment should be a small but faithful version
of production. If a deploy works on your laptop with the same
Ansible roles, you've already smoke-tested the change. No
"works on staging, fails on prod because the IaC layer is different".

### Compose, don't adopt

Prefer **small components with managed configuration** over
all-in-one platforms. A reverse proxy + a small auth layer + plain
git hooks is often easier to reason about than a self-hosted SaaS
that bundles all three with its own opinions.

### Boring building blocks

Postfix, Caddy, Restic, systemd timers, plain SQLite — these have
all been doing their jobs for years and will keep doing them without
your attention. Build on top of them.

### Managed config, not bespoke deployments

Small teams can't afford a snowflake-per-service. Configuration
lives in version control, gets applied by Ansible, and is identical
everywhere it needs to be.

### Approachable communities

A small town isn't one where everyone knows everyone — it's one
where, even when you don't, you *know someone who knows someone*,
and they can put you in touch. The building blocks of this library —
Caddy, Forgejo, Restic, Postfix, systemd, Debian itself — are made
and maintained by small, approachable communities of real people,
not by sales-and-PR machines. The bug tracker is a place where the
maintainer probably reads your issue. The chat room has the
developer dropping in. Patches sent upstream get looked at by
humans who actually wrote the code.

You give up the comfort of an enterprise support contract. You
gain the ability to reach the people whose decisions shape the
software you run.

## When *not* to use this

If your scale genuinely demands a platform — hundreds of services,
multi-region, dozens of operators — Smalltown will hurt. The
boundary isn't "what does the Internet say is best practice" but
"do we have the team to operate the recommended stack".

## Anti-Patterns ⚠️

- ❌ Adopting Kubernetes for three services and one operator.
- ❌ Reaching for [GitLab](../../anti-patterns/gitlab.md) when a Forgejo +
  Caddy combo would do.
- ❌ Container-orchestrating workloads that update once a year (see
  [Don't dockerize mail servers](../../anti-patterns/dockerize-mail-servers.md)).

## Best Practices 💡

- Default to managed configuration; only escape to per-host hand
  edits with a Pattern entry that explains why.
- Each new service: ask "could a Debian package + a 30-line role
  do this?" before reaching for an operator.
- Cross-reference: when an Anti-Pattern critiques a heavy product,
  link the smaller composable replacement here.

## Related Patterns 🔗

- [Compose Service Pattern 🐋](../operation/deployment/compose-service.md)
- [Stages Pattern 🎭](../operation/deployment/stages.md)
- [Cuteness Pattern 🌸](../meta/cuteness.md)
