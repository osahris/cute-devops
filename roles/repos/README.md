<!--
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Repos Role

Sets up bare git repositories on the target host. By default, scaffolds them with the [Worktree Treehouses 🌳](https://cute-devops.patterns.how/patterns/approaches/worktree-treehouses) layout: a `treehouses/` directory with `chmod 3775` (setgid + sticky), a starter `README.md`, a `CLAUDE.md → README.md` symlink, and `.claude/` hook scripts. Turn the layout off to get a plain bare repo with sensible group permissions.

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
    with_treehouses: true           # per-repo override of repos_with_treehouses
    with_claude_hooks: true         # per-repo override of repos_with_claude_hooks
```

Defaults (see `defaults/main.yml`):

- `repos_default_group: devops`
- `repos_default_owner: root`
- `repos_with_treehouses: true`
- `repos_with_claude_hooks: true`

## Example

```yaml
- hosts: village
  become: true
  roles:
    - role: osahris.cute_devops.repos
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

## What this role does NOT do

- Wire up the `reference-transaction` hook on `main` (policy + sync-on-merge). That's project-specific — see the pattern.
- Create or manage Unix groups / users. Use `osahris.cute_devops.users` for that.
- Push initial content into `main`. That's the maintainer's first commit.

## Implements

- [Worktree Treehouses 🌳](../../patterns/approaches/worktree-treehouses.md)

## License

EUPL-1.2
