---
title: Push to Deploy 🚀
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## Overview 📋

We have a repo. Pushing to it triggers the deploy. That's the whole pattern.

A bare git repo lives on the host. A `post-receive` hook fires when the deploy
branch is pushed, and starts a systemd unit that does the deploy. No deploy
command, no pipeline DSL, no upload protocol — the git remote *is* the deploy.

## Goals 🎯

- One verb for deployment: `git push`.
- Deploys run in systemd — journald logs, status, restart semantics for free.
- Auditable and reversible: `git log` on the target shows what landed; a
  rollback is just another push.

## How it works 🛠️

```
git push  ──►  bare repo  ──►  post-receive hook  ──►  systemctl start deploy@<id>
                                                              │
                                                              ▼
                                                   the unit deploys (checkout, restart)
```

The hook is a doorbell, the unit is the house:

- **`post-receive`** runs as the pushing user and holds no privilege. When the
  deploy branch moves, it just starts `deploy@<id>.service`.
- **the unit** does the actual work — check the branch out, restart the
  service — as the service user. systemd handles the identity, the logging,
  and "one deploy at a time".

The bridge between them is a narrow grant: the pushers' group may *start* the
deploy unit, and nothing else.

## Automating it 🔄

Pair this with [Worktree Treehouses 🌳](../../approaches/worktree-treehouses.md):
extend the bare repo's `reference-transaction` hook to `git push` the deploy
remote after a merge to `main`. Then a merge *is* the deploy — no separate
step. For staging vs. prod, use two remotes (see [Stages 🎭](./stages.md)).

## Security 🔐

- **Keep privilege out of the hook.** The hook runs on pushed (untrusted)
  content as the pusher; it should only ring the doorbell. The privileged
  code is the systemd unit, on the system side.
- **Scope the bridge.** The grant is `start` on the deploy unit — not a shell,
  not `stop`/`restart`, not other units.

## Anti-patterns ⚠️

- ❌ **Doing the deploy inside the hook.** You lose journald logs, status, and
  concurrency handling that the unit gives for free — and put privileged work
  on the attacker-controlled side.
- ❌ **A bespoke deploy CLI or upload protocol.** You already have `git push`.
- ❌ **Deploying from a contributor's treehouse.** Deploy is a property of the
  project, not of one checkout.

## Possible Implementations 🛠️

- [`mkbrechtel.devops.repos`](../../../roles/repos/README.md) — `with_deploy`
  installs the `post-receive` hook.
- [`mkbrechtel.devops.setup_deploy`](../../../roles/setup_deploy/README.md) —
  ships the `deploy@.service` family and the polkit grant.
- [`mkbrechtel.devops.deploy`](../../../roles/deploy/README.md) — configures
  the `deploy@<id>` instance that checks out and restarts.

## Related Patterns 🔗

- [Worktree Treehouses 🌳](../../approaches/worktree-treehouses.md) — the
  lifecycle this deploy step extends.
- [Stages 🎭](./stages.md) — staging vs. prod as separate remotes.
- [Compose Service 🐋](../vhost/compose-service.md) — a common deploy target shape.
- [Vhost Directory 🏠](../vhost/vhost-directory.md) — the same-host shape: push directly to a `/srv/vhosts/<name>/` git remote.

## References 📚

- `githooks(5)` — `post-receive`, and who runs it.
- `systemd.service(5)` — templated `deploy@.service` units.
- `polkit(8)` — scoping `start` on one unit family to one group.
