---
title: Vhost Directory 🏠
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## Why "vhost", not "service"

A systemd `.service` already lives at the heart of this collection
(`deploy@<x>.service`, `setup_deploy`, …). Reusing "service" for our
unit-of-deployment would have every paragraph carrying two meanings —
not cute. **Vhost** (virtual host: a logical host on a physical one)
keeps the systemd vocabulary clean and stays cute. One word, one
meaning.

## Goal

One canonical place on disk for each running vhost: `/srv/vhosts/<fqdn>/`,
where `<fqdn>` is the vhost's fully-qualified hostname
(e.g. `www.example.com`). The FQDN gives global uniqueness for free —
no project prefix, no path nesting, no symlink farm. The vhost
directory *is* the vhost — its files, its persistent state, its
identity, and its **git push target**. `cd /srv/vhosts/www.example.com`
is *the* answer to "where does it live"; `git push
host:/srv/vhosts/www.example.com main:deploy` is *the* answer to "how
do I deploy it".

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
├── deploy/               # deploy scripts run by deploy-vhost@<name>
│   ├── 01-pull           # tracked in git, pushed with the code
│   ├── 02-up
│   └── 03-restart
└── .gitignore            # ignores compose.override.yaml, .env, data/, …
```

Owned `<fqdn>:<pushers-group>`, mode `2775` (setgid so pushed objects
inherit the group). **Each vhost is its own Unix user** — the username
is the FQDN — with lingering enabled (`loginctl enable-linger`), a
`systemd --user` instance running continuously, and an auto-allocated
subuid/subgid range. That's what makes rootless podman per vhost
natural: quadlets in `~/.config/containers/systemd/`, container state
in `~/.local/share/containers/`, all under one tree owned by one user.

## The vhost directory is a git remote

Push-to-deploy means **pushing to the vhost literally**. The vhost
directory is the whole app as a git working tree. Two branches:

- **`deploy`** — the push target. New code arrives here.
- **`deployment`** — the checked-out branch (HEAD). What's actually
  running. Fast-forwards from `deploy` on each successful deploy.

```bash
# create the FQDN user (subuid/subgid auto-allocated)
useradd --create-home --home-dir /srv/vhosts/<fqdn> \
        --shell /usr/sbin/nologin <fqdn>
# user-systemd auto-starts at boot AND now
loginctl enable-linger <fqdn>
systemctl start user@$(id -u <fqdn>).service

sudo -u <fqdn> bash -c '
  cd /srv/vhosts/<fqdn>
  git init -b deployment .
  git commit --allow-empty -m "initial"
  git branch deploy deployment
  # local-only files: never tracked, never clobbered by push
  printf "%s\n" ".env" >> .git/info/exclude
'
# the role installs hooks/post-receive; the systemd template lives once
# at /etc/systemd/system/deploy-vhost@.service for *all* vhosts
```

From anywhere — a treehouse, a village bare repo, a CI runner — you add
the vhost as a remote and push to `deploy`:

```bash
git remote add vhost/<name> ssh://host/srv/vhosts/<name>
git push vhost/<name> main:deploy
```

What happens at the other end is **standardised** (every vhost behaves
identically — no per-vhost push logic):

1. `git-receive-pack` updates `refs/heads/deploy`. (No `denyCurrentBranch`
   override needed: `deploy` is *not* the current branch.)
2. `post-receive` fires `deploy-vhost@<name>.service`.
3. The unit (one template, `%i` carries `<name>` everywhere — see
   below) runs three steps as `User=<name>` from `/srv/vhosts/<name>`:
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
for every vhost. The single `deploy-vhost@.service` template carries it;
the vhost's `deploy/` directory carries only what's *vhost-specific*
(compose up, migrations, restarts).

## Relationship to the village bare repo

A project's village bare repo (`/srv/repos/<repo>.git`, see
[[worktree-treehouses]]) and a vhost's deploy target
(`/srv/vhosts/<name>`) are **two different git remotes that may share
code**:

- **`/srv/repos/<repo>.git`** — the bare repo where contributors spawn
  treehouses, commit, and merge to `main`. Collaboration surface.
- **`/srv/vhosts/<name>`** — the running vhost's directory, which also
  acts as a git push target. Deployment surface.

Wiring them together is just `git remote add vhost/<name> …` on the
bare repo and `git push vhost/<name> main:deploy`. The village bare
repo's `reference-transaction` hook can issue that push automatically
after a successful merge to `main`.

## How it composes with the rest

**One template unit serves every vhost.** The push protocol lives in a
single file — `/etc/systemd/system/deploy-vhost@.service` — fully
parameterized on `%i` (the vhost name):

```ini
[Unit]
Description=Vhost deploy for %i

[Service]
Type=oneshot
User=%i
WorkingDirectory=/srv/vhosts/%i
EnvironmentFile=-/srv/vhosts/%i/.env
ExecStartPre=/usr/bin/git merge --ff-only deploy
ExecStart=/usr/bin/run-parts --exit-on-error --verbose deploy/
ExecStartPost=/usr/local/lib/vhost/tag-deployed %i
```

Each phase carries one job:
- **`ExecStartPre`** brings the tree to the pushed tip (fail loud on
  non-fast-forward).
- **`ExecStart`** runs the vhost's tracked deploy scripts.
- **`ExecStartPost`** marks the deploy with a tag.

`Type=oneshot` makes systemd wait for `ExecStart` to *exit* before
running `ExecStartPost`. `systemctl list-units 'deploy-vhost@*'` lists
every vhost deploy on the host. Adding the hundredth vhost touches no
systemd state — the template already covers it. Per-vhost overrides
remain possible via `deploy-vhost@<name>.service.d/` drop-ins, but
become a rare exception, not the rule.

`tag-deployed` is a one-line helper shipped by the role:
`git tag "deployed-to-$1-at-$(date -u +%Y%m%dT%H%M%SZ)"`.

**Repos are independent of vhosts.** A repo may feed one vhost, several
vhosts, or share a vhost with other repos. The connection is "a git
remote pointing at `/srv/vhosts/<name>`", not name equality:

- **One repo, one vhost** (the common case): the village repo has
  `vhost/www.example.com` pointing at `/srv/vhosts/www.example.com`.
  Push lands, `deploy-vhost@www.example.com` runs.
- **One repo, several vhosts** (a monorepo serving web + worker):
  the village repo has `vhost/www.example.com` and
  `vhost/worker.example.com`, pointing at the two vhost directories.
  Each one's `post-receive` fires its own `deploy-vhost@<fqdn>`.
  `git remote | grep '^vhost/'` and `systemctl list-units
  'deploy-vhost@*'` each list every target.
- **Several repos, one vhost** (frontend + backend assembled into one
  running thing at `app.example.com`): both village repos have
  `vhost/app.example.com` pointing at the same vhost. Each push
  updates a different subtree (or branch); `post-receive` fires the
  same `deploy-vhost@app.example.com` either way.

Other roles latch onto the vhost-name spine:

- **[[setup_deploy]]:** ships the generic `deploy@.service` template
  for run-parts-style deploys reading `/etc/deploy/<id>/`. Vhosts are
  *parallel infrastructure*: their own `deploy-vhost@.service` template
  lives in this role, alongside its own polkit grant. The two
  coexist on a host but don't depend on each other.
- **[[compose-service]]:** the vhost directory holds `compose.yaml` /
  `compose.override.yaml`; `docker-compose up -d` is run from there by
  `deploy-vhost@<name>.service`. (The pattern's own name — *Compose
  Service* — is about compose's `services:` block, not our vhost.)
- **Backup roles** (restic / borg, when they land): `data/` is the
  source; `<vhost>` is the backup-unit name.

## Naming

**`<vhost>` is the FQDN.** It's the same charset systemd instance ids
allow (lowercase `[a-z0-9.-]+`), so `deploy-vhost@<fqdn>.service`
exists without escaping. The FQDN gives global uniqueness for free —
two projects can't accidentally collide; multi-tenant hosts work without
prefixes; the name doubles as the natural identifier in TLS,
reverse-proxy config, and DNS.

**Git remote names** that point at a vhost take the shape
`vhost/<vhost>`. Git allows `/` in ref names, so the remote lands at
`refs/remotes/vhost/<vhost>/deploy` and tooling treats it normally
(`git remote -v`, `git push vhost/<vhost> main:deploy`, `git remote |
grep ^vhost/` to list all vhost remotes). One namespace per purpose,
each target named by exactly the vhost it points at. Inside each path
component, stay in `[a-z0-9.-]+` so the whole chain — `<repo>` →
`vhost/<vhost>` → `<vhost>` → `deploy-vhost@<vhost>.service` — is
one charset, with "vhost" used everywhere our concept appears.

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
  `systemctl list-units 'deploy-vhost@*'` should answer "what vhosts
  deploy here?" — not "what repos can push?". When repo and vhost
  names diverge, the instance follows the vhost.
- ❌ **Per-vhost drop-ins for the standard push protocol.** One
  templated unit + `%i` covers every vhost; a per-vhost drop-in
  duplicates state without adding power. Keep drop-ins for genuine
  overrides only.
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

- **XDG_RUNTIME_DIR in the deploy unit.** Deploy scripts that want to
  talk to user-systemd (`systemctl --user`, quadlets) need
  `XDG_RUNTIME_DIR=/run/user/<uid>` set. The system-level
  `deploy-vhost@.service` switches user via `User=%i` but doesn't run
  pam_systemd, so this env var isn't set automatically. Either set it
  in `deploy/` scripts that need it, or have the deploy unit do a tiny
  uid lookup in a wrapper (`Environment=XDG_RUNTIME_DIR=/run/user/$(id -u %i)`
  doesn't work — no shell substitution in unit files). Probably ends up
  being a one-line `eval` in the scripts.
- **The `repos` role's `with_deploy`** puts the post-receive on
  `/srv/repos/<name>.git`, treating the bare repo as the push target.
  Under this pattern, the post-receive belongs at
  `/srv/vhosts/<fqdn>/.git/hooks/`, on the vhost. The `repos` feature
  is mis-located; the `vhosts` role now covers the right shape.
  `repos` `with_deploy` should be retired, or repurposed as a
  "village bare repo auto-pushes to vhost/<fqdn>" feature.
- **Wider terminology rename.** A few drafts still say "service" for
  our concept (`oidc-gating.pattern.md`, `project-service-terminology
  .pattern.md`, parts of `hashicorp-vault-integration.feature.md`).
  Chase the rename through when the terminology draft is settled.
- **Pure-static vhosts** (no compose, no restart): the ff-merge step
  already lands the files; the `deploy/` directory can be empty.
  Confirm `run-parts` over an empty dir really does exit 0 in
  practice.

## Possible Implementations 🛠️

- [`mkbrechtel.devops.vhosts`](../../../roles/vhosts/README.md) — ships
  `deploy-vhost@.service` + its polkit grant, and creates each vhost
  with `.git/` initialised on `deployment`, a `deploy` branch, `.env`
  in `info/exclude`, the `deploy/` subdir, and the post-receive hook
  firing `deploy-vhost@<name>.service`. Validated end-to-end in
  `test-in-containers.yaml`.

## Related Patterns 🔗

- [Worktree Treehouses 🌳](../../approaches/worktree-treehouses.md) —
  the village bare repo this pattern's vhosts often deploy from.
- [Compose Service 🐋](./compose-service.md) — a common shape for what
  lives inside a vhost directory.
- [Push to Deploy 🚀](../deployment/push-to-deploy.md) — the more
  generic statement of "deployment is a `git push`" that this
  pattern specialises for same-host vhosts.
