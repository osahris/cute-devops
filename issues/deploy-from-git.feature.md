---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Local deployment from git repos

## Goal

Prove that you don't need a central platform (GitLab/Forgejo) for small-scale
deployment. Push to a bare repo on the target host, or pull from a central
forge — both flows run deploys in systemd units.

## Scope

Two complementary flows:

### Push-to-deploy

- Bare repos live in `/srv/repos/<name>.git` on the target.
- Users push over SSH (using the system's user accounts + SSH keys).
- A `post-receive` hook triggers a systemd unit that runs the deploy.
- The deploy unit is an instance of a `deploy_*` role (e.g.
  `deploy_ansible_play`, or a new `deploy_compose` / `deploy_quadlet`).

### Pull-from-remote

- Target watches a branch on a remote git host (GitHub/Gitea/whatever).
- A systemd timer (or a branch-watcher service) checks for new commits and
  triggers the same deploy unit.
- Same `deploy_*` role family is reused.

## Design notes

- Bare repo management + the push-side trigger is the `repos` role: with
  `with_deploy` it installs the `post-receive` hook that starts
  `deploy@<instance>.service`. (This supersedes the empty
  `triggered_by_git_hook` stub for the push side; that role can be dropped
  or repurposed.)
- The hook→unit privilege bridge is host-wide in `setup_deploy`:
  `setup_deploy_polkit_group` installs a polkit rule letting that group
  `start` any `deploy@` unit.
- New: `triggered_by_branch_watcher` (or similar) for the pull side —
  oneshot service + timer that fetches and, on new SHA, starts the deploy
  unit.
- Deploys are always systemd units (`deploy@.service` from `setup_deploy`)
  so we get logs, status, and restart semantics for free. The deploy
  instance (`/etc/deploy/<id>/`, via the `deploy` role) owns the actual
  checkout + restart — including running it as the service user.
- Secrets injected via the secrets role at deploy time.

## Open questions

- One bare repo per app, per host, or per project? (User said `/srv/repos` →
  sounds like per-host with free naming.)
- Who can push — every local user, or a dedicated `git` user with
  per-user SSH keys in its authorized_keys?
- Branch watcher: long-running daemon with webhooks, or pure polling via
  timer? (Polling is simpler, no inbound needed.)
- Should `deploy_*` roles be idempotent enough to run on every push even
  when nothing substantive changed?
- Rollback story — keep last N deploys in place, or rely on re-pushing an
  older commit?
