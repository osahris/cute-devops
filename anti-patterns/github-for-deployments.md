---
title: Don't use GitHub for your deployments! 🔻🐙
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

## The Pitch 📦

> *Push to GitHub, run GitHub Actions, deploy to your servers from
> the runner. The whole pipeline lives next to the code.*

## Why It's Tempting 🍬

- The repo is already there; Actions is one `.yml` away.
- Marketplace actions for everything you might want to do.
- "Looks the same as everyone else's CI" feels like safety.

## Why It's Not Cute 🔻

You're putting a third-party SaaS — one you do not operate — on the
critical path of every production change.

- **No IPv6.** GitHub.com still doesn't speak IPv6 in 2026; runners,
  webhooks, and the API are IPv4-only. If your hosts live on a
  v6-only or v6-preferred network, you've now bolted a translation
  layer onto your deployment path. Quietly.
- **Outage exposure.** When GitHub is down (and it is, regularly:
  Actions in particular has bad weeks), you can't deploy. You can't
  rollback. You can't ship a hotfix, even one that's already in your
  local clone. Your release cadence is gated by a vendor's status
  page.
- **US jurisdiction & export controls.** GitHub is operated by a US
  company, which means US law follows the data. Past blocks of
  developer accounts in sanctioned regions are a reminder that
  "neutral hosting" isn't.
- **Network coupling.** Every host that gets deployed-to has to
  reach `github.com`, `api.github.com`, plus whatever runner egress
  ranges your security team is comfortable allow-listing today.
  That's a fragile, opaque ACL surface.
- **Secret stewardship.** Your deploy keys, registry tokens, signing
  keys and cloud credentials live in GitHub's secrets store. A
  compromise on GitHub's side (or a misconfigured organization
  permission, or an over-eager Marketplace action) reaches into
  every environment that trusts those secrets.
- **Lock-in by Actions YAML.** Once your pipelines speak GitHub
  Actions syntax, the contexts, the marketplace actions, the
  `${{ secrets.* }}` interpolation — moving off becomes a project,
  not a config tweak.
- **Dev/prod divergence.** Nobody runs GitHub Actions locally.
  You're pretending a YAML file you can't execute on your laptop is
  your release tooling.

## The Cute Alternative 💙

Keep deployment **on the hosts**. Tools and patterns from this
library:

- A bare git repo on the deploy host, pushed to over SSH; a
  `post-receive` hook runs the same Ansible role you'd run by hand.
  See the [triggered_by_git_hook](/roles/triggered_by_git_hook)
  role.
- For pull-based deploys, the [deploy_ansible_pull](/roles/deploy_ansible_pull)
  role with a systemd timer.
- Forgejo (or Gitea, or even plain shared hosting) for code review
  and issue tracking — neither of those needs to be in the
  deployment path.
- Mirror to GitHub if you want public visibility, but treat that
  mirror as read-only marketing, not infrastructure.

You give up "Deploy" buttons in PRs. You gain a deployment story
that survives any vendor outage and doesn't quietly require IPv4
NAT for the rest of its life.

## Related 🔗

- [Smalltown Infrastructure 🏘️](../patterns/approaches/smalltown-infrastructure.md) —
  the pattern this is the anti-pattern of.
- [Don't introduce GitLab as the central DevOps Hub of your organization! 🔻🦊](./gitlab.md) —
  same shape, with the SaaS replaced by a self-hosted megaproduct.
