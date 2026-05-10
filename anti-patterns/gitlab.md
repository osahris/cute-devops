---
title: GitLab 🔻🦊
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## The Pitch 📦

> *Code hosting + CI + container registry + issue tracker + wiki +
> packages + observability — all integrated, one login, one upgrade.*

## Why It's Tempting 🍬

- One product to procure and onboard.
- The marketing matrix shows green checkboxes for everything.
- "Consolidating from GitHub + a CI thing" feels like simplification.

## Why It's Not Cute 🔻

GitLab is a hyperscaler-shaped product sold to small teams.

- **Footprint.** GitLab Omnibus expects ~4 GB of RAM at idle, and
  much more once Sidekiq, the registry, the runner, and the package
  registry warm up. That's a tangible chunk of a small VM bill, for
  a workload most small teams hit lightly.
- **Upgrades are events.** Minor releases ship database migrations
  that require minutes of downtime. A point upgrade can break a CI
  config in a way that takes the team a week to unpick.
- **Sub-products you don't use.** Pages, packages, security
  dashboards, ML features — they're all in the binary, all consume
  resources, and all expand the attack surface.
- **Lock-in by feature breadth.** When issues, CI, packages, and the
  registry all live in one product, swapping any one piece becomes a
  project. The cost of switching grows with every feature you adopt.
- **Dev/prod divergence.** Nobody runs GitLab on their laptop. The
  reality of the system you're operating — dependencies, version
  matrix, upgrade pain — is invisible until something breaks.
- **Centralizes a security boundary you don't fully control.** Once
  the entire organization's code, CI, secrets, registry, and
  deployment triggers live behind one login on one box, that box is
  the security boundary. A compromise — or a misconfigured token,
  group permission, or runner — leaks reach across every project
  you have. When code management, CI, and deployment happen
  directly on the target hosts (git via SSH, deploys via systemd
  units, secrets via the host's own credential store) the trust
  surface stays per-host: each machine only ever sees the code and
  credentials it actually runs, and there's no central trove to
  break into. You also avoid the network gymnastics of "GitLab
  Runner has to reach prod through six firewall rules".

## The Cute Alternative 💙

Compose the few features you actually use from small, replaceable
parts:

- **Forgejo** (or Gitea) for git hosting, issues, PRs, wiki.
- A small CI of choice — Forgejo Actions, Woodpecker, or a couple of
  scheduled `git pull && deploy.sh` systemd units behind your
  reverse proxy.
- A plain container registry (Distribution) when you actually need
  one.
- Caddy out front for TLS and basic auth.

You give up the unified marketing story. You gain a system where
each layer is replaceable, fits on a small VM, and any single
component can be upgraded — or thrown out — without touching the
others.

## Related 🔗

- [Smalltown Infrastructure 🏘️](/patterns/approaches/smalltown-infrastructure) —
  the pattern this is the anti-pattern of.
- [Don't dockerize mail servers](/anti-patterns/dockerize-mail-servers) — same
  spirit: pick small, boring building blocks.
