<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Repos Role

Sets up plain **bare git repositories** on the target host: `git init --bare
--shared=group`, group ownership, a `description`, and group-write stripped off
the policy paths (`hooks`, `config`, `description`). Nothing more — a bare repo
is a complete, self-contained thing.

To add the shared work directory on top (the `/work/<project>` lookup pad,
`CLAUDE.md`, categories, and Claude Code worktree hooks for stacked branching),
apply [`osahris.cute_devops.worktrees`](../worktrees/README.md) afterwards. The
two roles are independent: a bare repo works fine without worktrees, and
worktrees can point at any bare repo however it was created.

## Requirements

- Ansible >= 2.14
- Debian 13 (trixie); `git` and POSIX shell on the target.

## Role Variables

```yaml
repos:
  - name: foo                       # short label (optional; defaults to basename of path)
    path: /srv/repos/foo.git        # required; absolute path on target, ends in .git
    group: devops                   # group that shares the repo (defaults to repos_default_group)
    owner: root                     # who owns policy files (defaults to repos_default_owner)
    description: "A cute project."  # optional; written to the bare repo's `description`
```

Defaults (see `defaults/main.yml`):

- `repos_default_group: devops`
- `repos_default_owner: root`

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
    # optional: add the shared work directory + worktree hooks
    - role: osahris.cute_devops.worktrees
      vars:
        worktrees:
          - name: foo
            repo: /srv/repos/foo.git
```

## What this role does NOT do

- Set up the shared work directory or worktree hooks — that's `osahris.cute_devops.worktrees`.
- Wire up the `reference-transaction` hook on `main` (policy + sync-on-merge). That's project-specific — see the pattern.
- Create or manage Unix groups / users. Use `osahris.cute_devops.users` for that.
- Push initial content into `main`. That's the maintainer's first commit.

## Implements

- [Shared Worktrees 🌳](../../patterns/approaches/shared-worktrees.md) — the bare repo half.

## License

EUPL-1.2
