<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

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
    auto_push: false                # per-repo override of repos_with_auto_push
    remotes:                        # optional; git remotes to configure on the bare repo
      - name: github
        url: git@github.com:foo/foo.git
      - name: codeberg
        url: ssh://git@codeberg.org/foo/foo.git
        auto_push: false            # exclude this remote from auto-push (default true)
```

Defaults (see `defaults/main.yml`):

- `repos_default_group: devops`
- `repos_default_owner: root`
- `repos_with_treehouses: true`
- `repos_with_claude_hooks: true`
- `repos_with_auto_push: false`

### Remotes

Each entry in `remotes` needs a `name` and a `url`; the role adds the remote
(or updates its URL) in the bare repo. Existing remotes not listed are left
alone.

### Auto-push

With `auto_push: true` the role installs a `reference-transaction` git hook
in the bare repo: whenever a branch or tag changes — a push into the repo, a
commit in a treehouse worktree, a merge — the update is pushed on to every
configured remote in the background (deletions propagate too). Per remote,
`auto_push: false` opts that remote out (the hook only pushes to remotes
whose `remote.<name>.autopush` git config is true).

Notes:

- The push runs as whichever user updated the ref, so everyone in the repo
  group needs credentials for the remotes (e.g. ssh keys / agent).
- Pushes are non-forced; rejections and other failures are appended to
  `autopush.log` in the bare repo.
- The hook file is Ansible-managed while `auto_push` is true (it overwrites a
  hand-rolled `reference-transaction` hook). With `auto_push` false the role
  never touches the hook.

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

- Wire up branch policy / sync-on-merge in the `reference-transaction` hook. The optional `auto_push` hook only mirrors refs to remotes; anything beyond that is project-specific — see the pattern.
- Create or manage Unix groups / users. Use `osahris.cute_devops.users` for that.
- Push initial content into `main`. That's the maintainer's first commit.

## Implements

- [Worktree Treehouses 🌳](../../patterns/approaches/worktree-treehouses.md)

## License

EUPL-1.2
