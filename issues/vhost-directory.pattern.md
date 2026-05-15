---
status: draft
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Vhost Directory 🏠

> **Pattern (draft).** Cross-cutting convention. Once a role or two adopt
> `/srv/vhosts/<name>/`, promote to `patterns/operation/`.

## Why "vhost", not "service"

A systemd `.service` already lives at the heart of this collection
(`deploy@<x>.service`, `setup_deploy`, …). Reusing "service" for our
unit-of-deployment would have every paragraph carrying two meanings —
not cute. **Vhost** (virtual host: a logical host on a physical one)
keeps the systemd vocabulary clean and stays cute. One word, one
meaning.

## Goal

One canonical place on disk for each running vhost: `/srv/vhosts/<name>/`.
The vhost directory *is* the vhost — its files, its persistent state,
its identity, and its **git push target**. `cd /srv/vhosts/foo` is *the*
answer to "where does foo live"; `git push host:/srv/vhosts/foo main:deploy`
is *the* answer to "how do I deploy foo".

## Shape

```
/srv/vhosts/<name>/
├── .git/                 # this directory IS a git remote (see below)
├── compose.yaml          # or *.container quadlet, or a binary
├── compose.override.yaml # local overrides (proxy network, …)
├── .env                  # env file (sourced by compose / systemd)
├── VHOST.md              # what this vhost is, who runs it
├── config/               # read-only config (if any)
├── data/                 # persistent state — backup target
├── deploy/               # deploy scripts run by deploy@vhost-<name>
│   ├── 01-pull           # tracked in git, pushed with the code
│   ├── 02-up
│   └── 03-restart
└── .gitignore            # ignores compose.override.yaml, .env, data/, …
```

Owned by a per-vhost Unix user — typically `<name>` — mode `0755`.
The vhost's process runs as that user.

## The vhost directory is a git remote

Push-to-deploy means **pushing to the vhost literally**. The vhost
directory is the whole app as a git working tree. Two branches:

- **`deploy`** — the push target. New code arrives here.
- **`deployment`** — the checked-out branch (HEAD). What's actually
  running. Fast-forwards from `deploy` on each successful deploy.

```bash
sudo -u <name> bash -c '
  cd /srv/vhosts/<name>
  git init -b deployment .
  git commit --allow-empty -m "initial"
  git branch deploy deployment
  # local-only files: never tracked, never clobbered by push
  printf "%s\n" ".env" .git/info/exclude
'
# the role installs hooks/post-receive and the systemd drop-in (below)
```

From anywhere — a treehouse, a village bare repo, a CI runner — you add
the vhost as a remote and push to `deploy`:

```bash
git remote add deploy/<name> ssh://host/srv/vhosts/<name>
git push deploy/<name> main:deploy
```

What happens at the other end is **standardised** (every vhost behaves
identically — no per-vhost push logic):

1. `git-receive-pack` updates `refs/heads/deploy`. (No `denyCurrentBranch`
   override needed: `deploy` is *not* the current branch.)
2. `post-receive` fires `deploy@vhost-<name>.service`.
3. The unit's drop-in runs three steps as `User=<name>` from
   `/srv/vhosts/<name>`:
   1. `git merge --ff-only deploy` — bring the `deployment` branch
      (the working tree) up to the pushed tip. Non-fast-forward pushes
      fail loud here, before any deploy script runs.
   2. `run-parts --exit-on-error --verbose deploy/` — run the vhost's
      *tracked* deploy scripts (now updated on disk by step 1).
   3. `git tag deployed-to-<name>-at-<utc-timestamp>` — mark the
      successful deploy. `git tag | grep ^deployed-to-<name>-` is the
      audit log; rolling back is pushing an older tag's commit to
      `deploy`.

The protocol — branches, ff-merge, tag, `.env` excluded — is the same
for every vhost. The drop-in carries it; the vhost's `deploy/` directory
carries only what's *vhost-specific* (compose up, migrations, restarts).

## Relationship to the village bare repo

A project's village bare repo (`/srv/repos/<repo>.git`, see
[[worktree-treehouses]]) and a vhost's deploy target
(`/srv/vhosts/<name>`) are **two different git remotes that may share
code**:

- **`/srv/repos/<repo>.git`** — the bare repo where contributors spawn
  treehouses, commit, and merge to `main`. Collaboration surface.
- **`/srv/vhosts/<name>`** — the running vhost's directory, which also
  acts as a git push target. Deployment surface.

Wiring them together is just `git remote add deploy/<name> …` on the
bare repo and `git push deploy/<name> main:deploy`. The village bare
repo's `reference-transaction` hook can issue that push automatically
after a successful merge to `main`.

## How it composes with the rest

**One vhost ⇄ one deploy instance, namespaced under `vhost-`.**
A vhost's deploy unit is `deploy@vhost-<name>.service`; the
`vhost-` prefix in the systemd instance id separates vhost deploys
from other kinds of `deploy@…` instances (cron-driven backups,
ad-hoc jobs) sharing the same template.
`systemctl list-units 'deploy@vhost-*'` lists every vhost deploy on
the host.

**The per-vhost drop-in carries the standard push protocol.** Every
vhost gets the same drop-in shape — only `<name>` varies — at
`/etc/systemd/system/deploy@vhost-<name>.service.d/10-vhost.conf`:

```ini
[Service]
User=<name>
WorkingDirectory=/srv/vhosts/<name>
EnvironmentFile=-/srv/vhosts/<name>/.env
ExecStart=
ExecStart=/usr/bin/git merge --ff-only deploy
ExecStart=/usr/bin/run-parts --exit-on-error --verbose deploy/
ExecStart=/usr/local/lib/vhost/tag-deployed <name>
```

(`ExecStart=` on its own clears the template's value before the new
ones replace it; multiple `ExecStart=` on `Type=oneshot` run in
sequence and any failure aborts.) `tag-deployed` is a one-line helper
shipped by the role: `git tag "deployed-to-$1-at-$(date -u +%Y%m%dT%H%M%SZ)"`.

The base `deploy@.service` template stays unchanged; generic,
non-vhost deploys keep using `/etc/deploy/<id>/` as before.

**Repos are independent of vhosts.** A repo may feed one vhost, several
vhosts, or share a vhost with other repos. The connection is "a git
remote pointing at `/srv/vhosts/<name>`", not name equality:

- **One repo, one vhost** (the common case): the village repo has
  `deploy/foo` pointing at `/srv/vhosts/foo`. Push lands,
  `deploy@vhost-foo` restarts.
- **One repo, several vhosts** (a monorepo with a webapp + a worker):
  the village repo has `deploy/myapp-web` and `deploy/myapp-worker`,
  pointing at `/srv/vhosts/myapp-web` and `/srv/vhosts/myapp-worker`.
  Each one's `post-receive` fires its own `deploy@vhost-<name>`.
  `git remote | grep '^deploy/'` and `systemctl list-units
  'deploy@vhost-*'` each list every target.
- **Several repos, one vhost** (frontend + backend assembled into one
  running thing): both village repos have `deploy/myapp` pointing at
  the same `/srv/vhosts/myapp`. Each push updates a different subtree
  (or branch); `post-receive` fires the same `deploy@vhost-myapp`
  either way.

Other roles latch onto the vhost-name spine:

- **[[setup_deploy]]:** ships the `deploy@.service` template. For
  vhost deploys, the per-vhost drop-in points it at
  `/srv/vhosts/<name>/deploy/`; non-vhost instances keep using
  `/etc/deploy/<id>/`. The `vhost-` prefix on the systemd instance
  is the namespace marker.
- **[[compose-service]]:** the vhost directory holds `compose.yaml` /
  `compose.override.yaml`; `docker-compose up -d` is run from there by
  `deploy@vhost-<name>.service`. (The pattern's own name — *Compose
  Service* — is about compose's `services:` block, not our vhost.)
- **Backup roles** (restic / borg, when they land): `data/` is the
  source; `<vhost>` is the backup-unit name.

## Naming

`<vhost>` is lowercase `[a-z0-9.-]+` — the charset systemd instance
ids allow, so `deploy@vhost-<vhost>.service` exists without escaping.
That charset is a *requirement* here, not a convenience, because the
deploy instance carries the name.

**Git remote names** that point at a vhost take the shape
`deploy/<vhost>`. Git allows `/` in ref names, so the remote lands at
`refs/remotes/deploy/<vhost>/deploy` and tooling treats it normally
(`git remote -v`, `git push deploy/<vhost> main:deploy`, `git remote |
grep ^deploy/` to list all deploy targets). One namespace per purpose,
each target named by exactly the vhost it points at. Inside each path
component, stay in `[a-z0-9.-]+` so the whole chain — `<repo>` →
`deploy/<vhost>` → `<vhost>` → `deploy@vhost-<vhost>.service` — is
one charset.

**Tag names** for successful deploys take the shape
`deployed-to-<vhost>-at-<utc-timestamp>`. The timestamp is
`YYYYMMDDTHHMMSSZ` (`date -u +%Y%m%dT%H%M%SZ`) — no colons (illegal in
git refs), no separators that confuse `git tag | grep ^deployed-to-`.

## Anti-patterns ⚠️

- ❌ **`/srv/<hostname>/` for the vhost path.** Couples vhost identity
  to the host; breaks when you move the vhost.
- ❌ **Splitting one vhost across `/etc/<name>/` + `/var/lib/<name>/`
  + `/opt/<name>/`.** Each path is defensible alone; together they
  fragment the operator's mental model. One directory per vhost.
- ❌ **Sharing one directory between two vhosts.** The directory *is*
  the vhost.
- ❌ **Calling it a "service".** The systemd `.service` is right there
  in the same sentence; the overlap is exactly the trap this pattern
  exists to avoid.
- ❌ **Naming the deploy instance after the repo, not the vhost.**
  `systemctl list-units 'deploy@*'` should answer "what vhosts deploy
  here?" — not "what repos can push?". When repo and vhost names
  diverge, the instance follows the vhost.
- ❌ **A separate "deploy bare repo" at `/srv/repos/<name>.git` that
  exists only to receive deploy pushes** and then checks out into
  `/srv/vhosts/<name>`. That's an extra hop with no purpose — push
  directly to the vhost.
- ❌ **Pushing to `deployment` (or any non-`deploy` branch).** The
  `deploy` branch is the contract. Pushing elsewhere skips the
  ff-merge step and breaks the audit chain.
- ❌ **Per-vhost custom push hooks.** The push protocol (ff-merge,
  tag, exclude `.env`) is shared; per-vhost custom code goes in
  `deploy/`, not in `.git/hooks/`.
- ❌ **Tracking `.env` in the upstream repo.** Local secrets belong
  in `.git/info/exclude` so they aren't replaced on each push.

## Open questions

- **Compose-service today says `/srv/<hostname>/`.** Either retire that
  in favour of `/srv/vhosts/<vhost>/` (small breaking change in the
  pattern), or keep `<hostname>` as the special case for the
  one-vhost-per-host shape. Pick one.
- **The `repos` role's `with_deploy` puts the post-receive on
  `/srv/repos/<name>.git`**, treating the bare repo as the push
  target. Under this pattern, the post-receive belongs at
  `/srv/vhosts/<name>/.git/hooks/`, on the vhost. The `repos` feature
  is mis-located and probably wants to move into a future `vhost`
  role (or a `deploy_compose` / `deploy_static`) that creates the
  vhost directory + `.git/` + hook + `deploy@vhost-<name>` instance
  + its per-vhost drop-in together.
- **Wider rename.** Other docs currently say "service" for our
  unit-of-deployment (the existing `push-to-deploy.md`, parts of
  `compose-service.md`, the `deploy.user` / `deploy.work_tree`
  naming in role configs). Decide whether to chase the rename
  through, or live with the local-to-this-pattern naming.
- **Pure-static vhosts** (no compose, no restart): the ff-merge step
  already lands the files; the `deploy/` directory can be empty.
  Is it worth standing up the whole deploy instance + drop-in for a
  vhost whose deploy is "files on disk" full stop, or is a stripped-down
  variant warranted?
- **Project prefix.** Does `<vhost>` ever need a project prefix
  (`<project>-<vhost>`) to disambiguate on shared hosts, or does
  `<vhost>` stay globally unique per host? Defer to
  [[project-service-terminology]] (which itself probably wants to
  become *project-vhost-host-terminology*).

## Acceptance

Promote to `patterns/operation/` once:

1. A role creates `/srv/vhosts/<name>/` with `.git/` initialised on
   `deployment`, a `deploy` branch, `.env` in `info/exclude`, the
   `deploy/` subdir present (even if empty), the `post-receive` hook
   firing `deploy@vhost-<name>.service`, and the per-vhost drop-in at
   `deploy@vhost-<name>.service.d/10-vhost.conf` doing the ff-merge +
   run-parts + tag — end to end on one push.
2. The relationship to `repos` `with_deploy` is resolved (either move
   the hook to the vhost side, or document why both placements
   coexist).
3. The conflict with `compose-service.md`'s `/srv/<hostname>/` is
   resolved one way or the other.
