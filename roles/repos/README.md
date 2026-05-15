<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Repos Role

Sets up bare git repositories on the target host. By default, scaffolds them with the [Worktree Treehouses 🌳](https://devops.patterns.how/patterns/approaches/worktree-treehouses) layout: a `treehouses/` directory with `chmod 3775` (setgid + sticky), a starter `README.md`, a `CLAUDE.md → README.md` symlink, and `.claude/` hook scripts. Turn the layout off to get a plain bare repo with sensible group permissions.

## Requirements

- Ansible >= 2.14
- Debian 13 (trixie); `git` and POSIX shell on the target.

## Role Variables

```yaml
repos:
  - name: foo                       # short label (optional; defaults to basename of path)
    path: /srv/repos/foo.git        # required; absolute path on target
    group: devops                   # who can spawn treehouses (defaults to repos_default_group)
    owner: root                     # who owns policy files (defaults to repos_default_owner)
    description: "A cute project."  # optional; written to bare repo's `description`
    default_branch: main            # per-repo override of repos_default_branch
    with_treehouses: true           # per-repo override of repos_with_treehouses
    with_claude_hooks: true         # per-repo override of repos_with_claude_hooks
    with_deploy: false              # per-repo override of repos_with_deploy
    deploy:                         # optional; all keys have defaults
      instance: foo                 #   deploy@<instance>.service to start (defaults to repo name)
      branch: main                  #   branch whose push triggers it (defaults to default_branch)
```

Defaults (see `defaults/main.yml`):

- `repos_default_group: devops`
- `repos_default_owner: root`
- `repos_default_branch: main` — passed to `git init --initial-branch` on
  creation and enforced on every run via `git symbolic-ref HEAD`, so the
  bare repo's HEAD always points at this branch.
- `repos_with_treehouses: true`
- `repos_with_claude_hooks: true`
- `repos_with_deploy: false` — when enabled, install the push-to-deploy
  hook for the repo (see *Push to deploy* below).
- `repos_with_safe_directory: true` — register `repos_safe_directory` as a
  system-wide `safe.directory` in `/etc/gitconfig`, so root and other users
  can operate on repos owned by the `devops` group without git complaining
  about "dubious ownership".
- `repos_safe_directory: /srv/repos/*` — the path (wildcard supported,
  git ≥ 2.36) recorded under `safe.directory`.

## Example

```yaml
- hosts: village
  become: true
  roles:
    - role: mkbrechtel.devops.repos
      vars:
        repos:
          - name: foo
            path: /srv/repos/foo.git
            description: "A cute project."
```

After the role runs:

```bash
cd /srv/repos/foo.git
# bootstrap an empty main:
git worktree add treehouses/main -b main
cd treehouses/main && git commit --allow-empty -m "Initial commit"
# then teammates can spawn their treehouses:
git -C /srv/repos/foo.git worktree add treehouses/feature/x -b feature/x main
```

## Push to deploy

With `with_deploy: true` (or `repos_with_deploy: true` globally) the role
installs a `post-receive` hook: on a push to the deploy branch it runs
`systemctl --no-block start deploy@<instance>.service`. The deploy then runs
in systemd — journald logs, status, notifications — instead of inside the
hook. The hook holds no privilege of its own; that's the whole `repos`-side
footprint.

Two other roles do the rest:

- [`mkbrechtel.devops.setup_deploy`](../setup_deploy/README.md) — ships the
  `deploy@.service` family, and (via `setup_deploy_polkit_group`) the polkit
  rule that lets the pushers group start `deploy@` units.
- [`mkbrechtel.devops.deploy`](../deploy/README.md) — configures the
  `deploy@<instance>` instance that does the actual checkout and restart.

The role asserts `deploy@.service` exists, so `setup_deploy` must have run on
the host first.

```yaml
repos:
  - name: foo
    path: /srv/repos/foo.git
    with_deploy: true
    # deploy: { instance: foo, branch: main }  # both defaulted
```

## What this role does NOT do

- Wire up the `reference-transaction` hook on `main` (policy + sync-on-merge). That's project-specific — see the pattern.
- Create or manage Unix groups / users. Use `mkbrechtel.devops.users` for that.
- Provide the deploy infrastructure or the deploy instance. `with_deploy` only
  installs the trigger; use `mkbrechtel.devops.setup_deploy` for the
  `deploy@.service` family and `mkbrechtel.devops.deploy` for the
  `deploy@<instance>` instance that does the checkout.
- Push initial content into `main`. That's the maintainer's first commit.

## Implements

- [Worktree Treehouses 🌳](../../patterns/approaches/worktree-treehouses.md)
- [Push to Deploy 🚀](../../patterns/operation/deployment/push-to-deploy.md) — with `with_deploy`

## License

EUPL-1.2
