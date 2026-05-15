<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Vhosts Role

Sets up vhosts — per-service deploy targets at `/srv/vhosts/<name>/`.
Implements the [Vhost Directory 🏠](../../issues/vhost-directory.pattern.md)
*(draft)* pattern.

Each vhost is a non-bare git repo. Pushing `main:deploy` to it
ff-merges into the checked-out `deployment` branch, runs the vhost's
tracked `deploy/` scripts, and tags `deployed-to-<name>-at-<utc>`.

The role ships **one** systemd template — `deploy-vhost@.service` —
fully parameterized on `%i` (the vhost name), so no per-vhost drop-in
is needed. Adding a vhost just means: Unix user + dir + git init +
post-receive hook.

## Requirements

- Ansible ≥ 2.14
- Debian 13 (trixie)
- Self-contained: no dependency on `setup_deploy`. The role ships its
  own template unit + polkit grant.

## Role Variables

```yaml
vhosts:
  - name: foo                     # required; matches [a-z0-9.-]+
    pushers_group: devops         # optional; group allowed to push
                                  #   (defaults to vhosts_default_pushers_group)
```

Defaults (see `defaults/main.yml`):

- `vhosts_default_pushers_group: devops` — Unix group given write
  access to each vhost's `.git/`.
- `vhosts_polkit_group: ""` — set to a group name and the role installs
  a polkit rule at `/etc/polkit-1/rules.d/50-vhost.rules` letting that
  group `systemctl start deploy-vhost@<name>.service` without auth.
  This is the bridge that lets the `post-receive` hook fire the deploy
  unit. Empty disables (no rule installed).
- `vhosts_with_safe_directory: true` — register `vhosts_safe_directory`
  as a system-wide git `safe.directory` so pushers can operate on repos
  owned by the vhost user without git complaining.
- `vhosts_safe_directory: /srv/vhosts/*`.
- `vhosts_helper_dir: /usr/local/lib/vhost`.

## Example

```yaml
- hosts: appservers
  become: true
  roles:
    - role: mkbrechtel.devops.vhosts
      vars:
        vhosts_polkit_group: devops
        vhosts:
          - name: foo
```

After the role runs, push to it from anywhere:

```bash
git remote add vhost/foo ssh://appserver/srv/vhosts/foo
git push vhost/foo main:deploy
```

## What this role does NOT do

- Create the pushers Unix group (use `mkbrechtel.devops.users`).
- Push deploy scripts into the vhost — those are tracked content in
  the vhost's own git tree, arriving via the first push.

## Implements

- [Vhost Directory 🏠](../../issues/vhost-directory.pattern.md) *(draft)*

## License

EUPL-1.2
