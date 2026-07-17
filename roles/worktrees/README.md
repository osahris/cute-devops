<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Worktrees Role

Adds a shared **work directory** on top of an existing bare git repository,
implementing the [Shared Worktrees 🌳](https://cute-devops.patterns.how/patterns/approaches/shared-worktrees)
layout. For each project it creates `{{ worktrees_base }}/<project>` (default
`/work/<project>`) with:

- **a `.git` lookup pad** — a `.git` file pointing at the bare repo, so anyone
  can `git log` / `git branch` / `git show` the live repo from the work
  directory. It reports as bare, so it's read-only: a place to look things up,
  not an editable checkout.
- **category folders** — a folder per default category (`feature`, `fix`,
  `hotfix`, `refactor`, `test`, `ci`, `chore`, `docs`, `update`, `experiment`,
  `release`), each with the same `3775` permissions as the work directory.
  Worktrees live inside these; new category folders are created on demand.
- **`CLAUDE.md`** — the landing doc explaining how to look things up and how to
  spawn a worktree.
- **`.claude/`** — Claude Code `WorktreeCreate` / `WorktreeRemove` hooks, a
  `Stop` hook (`require-clean`) that blocks a session from ending while its
  worktree is dirty (forcing a commit), and `bgIsolation: "worktree"`. Hooks are
  addressed by `$CLAUDE_PROJECT_DIR`. Note: for the `Stop` and stacked-spawn
  hooks to fire *inside* a worktree, the same `.claude/settings.json` must be
  tracked in the repo (checked out per worktree) — Claude Code does not inherit
  settings from the parent pad directory. This role scaffolds the pad's copy.

Worktrees then live at `/work/<project>/<category>/<branch>` on branches of the
same name, `<category>/<branch>`, one per unit of work — the branch name always
matches the worktree's path under the work directory, just like a plain
`git worktree add` there. The role records the work
directory in the bare repo's config (`cute.workdir`) so the hooks resolve it
identically from the pad or from inside any worktree.

The bare repo is independent of this role — create it however you like (e.g.
with `osahris.cute_devops.repos`). This role only needs its path.

## Naming and stacked branching

A worktree name is `<category>/<branch>`:

- `<category>` is a folder under the work directory grouping related work.
  There's no allowlist — a new category folder is created on demand with the
  same `3775` permissions as the work directory.
- `<branch>` is a URL slug: lowercase `a-z`/`0-9`, starts with a letter,
  contains at least one `-`, at most 40 characters.

The `WorktreeCreate` hook chooses the base branch by **where it runs**:

- not inside a worktree (e.g. from the pad, whose `HEAD` is `main`) → branch
  from `main`;
- inside an existing worktree → branch from that worktree's current branch.

That makes stacking work on top of in-review work the default.

## Requirements

- Ansible >= 2.14
- Debian 13 (trixie); `git` and POSIX shell on the target.
- A bare git repository already on the target (the `repo` path must exist).

## Role Variables

```yaml
worktrees:
  - name: foo                 # project slug (optional; defaults to basename of repo minus .git)
    repo: /srv/repos/foo.git  # required; path to the existing bare repo
    workdir: /work/foo        # optional; defaults to {{ worktrees_base }}/<name>
    group: devops             # who can create worktrees (defaults to worktrees_default_group)
    owner: root               # who owns policy files (defaults to worktrees_default_owner)
    description: "A cute project."   # optional; used in the scaffolded CLAUDE.md
    with_claude_hooks: true   # per-project override of worktrees_with_claude_hooks
    categories:               # optional; category folders to pre-create (overrides default)
      - feature
      - fix
```

Defaults (see `defaults/main.yml`):

- `worktrees_default_group: devops`
- `worktrees_default_owner: root`
- `worktrees_base: /work`
- `worktrees_with_claude_hooks: true`
- `worktrees_default_categories:` — `feature`, `fix`, `hotfix`, `refactor`, `test`, `ci`, `chore`, `docs`, `update`, `experiment`, `release`.

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
    - role: osahris.cute_devops.worktrees
      vars:
        worktrees:
          - name: foo
            repo: /srv/repos/foo.git
            description: "A cute project."
```

After the role runs, spawn a worktree (Claude users just call `EnterWorktree`):

```bash
cd /work/foo
git log --oneline main        # look things up (read-only pad)
git -C /srv/repos/foo.git worktree add /work/foo/feature/add-dns -b feature/add-dns main
cd /work/foo/feature/add-dns  # now edit and commit
```

## What this role does NOT do

- Create the bare repo. Use `osahris.cute_devops.repos` (or any bare repo).
- Wire up the `reference-transaction` hook on `main` (policy + sync-on-merge). That's project-specific — see the pattern.
- Enforce the `<category>/<branch>` shape on direct `git worktree add` or on push — that belongs in the bare repo's `pre-receive` hook. The `WorktreeCreate` hook enforces it for the Claude path.

## Implements

- [Shared Worktrees 🌳](../../patterns/approaches/shared-worktrees.md) — the work-directory half.

## License

EUPL-1.2
