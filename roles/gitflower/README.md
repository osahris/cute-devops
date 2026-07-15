<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# gitflower

Deploys **gitflower**, a git-based development platform — local-first and git-centric, built as a Python **click** CLI and **FastAPI** web UI on Debian `python3-*` apt packages.

gitflower is an independent product with its own repository; it is not part of this collection's codebase. Cute DevOps! ships this role because gitflower is part of the ecosystem and pairs nicely with the collection's git-centric patterns and roles (e.g. [`repos`](../repos/README.md)).

The role installs gitflower's Debian dependencies and checks the product out from git. It will grow with the product as gitflower gains packaging, a CLI entry point, and a web UI service.

## Requirements

- Ansible >= 2.14
- Debian 13 (trixie) on the target.

## Role Variables

- `gitflower_repo_url` (**required**) — git URL or local path of the gitflower repository to deploy from.
- `gitflower_version` (default: `main`) — branch, tag, or commit to deploy.
- `gitflower_install_dir` (default: `/opt/gitflower`) — where the checkout lives.
- `gitflower_update` (default: `true`) — update the checkout on subsequent runs.
- `gitflower_apt_packages` — Debian packages gitflower runs on; override to track the product's dependencies.

## Example

```yaml
- hosts: village
  become: true
  roles:
    - role: osahris.cute_devops.gitflower
      vars:
        gitflower_repo_url: /srv/repos/gitflower.git
```

## License

EUPL-1.2
