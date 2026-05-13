---
title: Worktree Treehouses 🌳
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## Overview 📋

A bare git repository lives on one host. Inside the bare repo, alongside `HEAD` and `refs/`, sit a `README.md`, a `.claude/` directory with hooks, permissions and skills already configured for Claude usage, and a `treehouses/` directory — one git worktree per contributor. You `cd` into the bare repo and everything you need is already there.

Everyone working on the project gets their own **treehouse**: a private worktree built on the same tree everyone else is climbing. A treehouse is light, spawnable, easy to dismantle — pull yourself up, do your work, climb back down. Filesystem permissions decide who can write where, and `git worktree` itself guarantees that no two treehouses share a branch.

## Goals 🎯

- Give every contributor an isolated working tree without the cost of managing a full clone per person.
- Let teammates read each other's in-progress work without races or permission gymnastics.
- Push access control down to the kernel and git hooks — no bespoke "who can touch this branch" service.
- Keep CI / linting / agent config inside the bare repo, so a fresh treehouse works the moment it exists. 
- One primitive (`git worktree`) for everyone; no bespoke way to set up a development environment.
- Get transparency on what happens in the project by just looking at the files – Make `ls treehouses/` the honest answer to "what's everyone working on right now?"


## Pattern Structure 📑

### The bare repo *is* the project

```
/srv/repos/foo.git/                   ← the project; bare repo
├── HEAD, refs/, objects/, info/      ← standard bare-repo internals
├── worktrees/                        ← git's per-worktree metadata
├── hooks/
│   ├── pre-receive                   ← CI gate
│   └── reference-transaction         ← main-line policy + config sync
├── README.md                         ← village notice board
├── CLAUDE.md → README.md             ← symlink: same doc, two readers
├── .claude/
│   ├── settings.json                 ← wires WorktreeCreate/Remove hooks
│   └── hooks/
│       ├── worktree-create           ← creates a treehouse for Claude
│       └── worktree-remove
└── treehouses/                       ← 3775 (rwxrwsr-t)
    ├── main/                         ← the maintainer's treehouse
    ├── feature/cute-thing/           ← built by alice
    └── claude/refactor-auth/         ← built by the user driving Claude
```

`worktrees/` (git's internal per-worktree metadata) and `treehouses/` (the actual checkouts) sit side by side without visual collision: one is git's bookkeeping, the other is where you live while you work.

### Every treehouse is a public branch

A worktree shares its parent bare repo's objects and refs database. That means a commit in `treehouses/feature/cute-thing/` lands immediately in `foo.git/refs/heads/feature/cute-thing` — no `git push` step, because there's nowhere separate to push to. As soon as you commit, your work is part of the bare repo's history.

The flip side is delightful: **use the bare repo as a git remote and `git fetch` pulls every treehouse's live state**. From any other machine:

```bash
git remote add village ssh://host/srv/repos/foo.git
git fetch village
git log village/feature/cute-thing       # alice's in-progress work
git log village/claude/refactor-auth     # whatever Claude is doing
```

No "publish my branch" ritual, no pull-request-as-publication. Active work is *already published* the moment it's committed — the village is its own status dashboard. (`main` of course stays gated by the maintainer's merge ritual; we're talking about the in-flight branches.)

### Permissions & trust

Membership in the project's Unix group (typically `devops`) is the access primitive: being in the group lets you `git worktree add`, commit, and read everyone else's treehouse. Removal from the group revokes all of it at once. No application-level user database, no per-branch ACL — the kernel decides.

`git worktree add` writes into the bare repo: it creates `worktrees/<name>/` (git's metadata) and the treehouse directory itself. So group members *must* have write access to parts of the bare repo. But the bare repo also holds **policy** — `hooks/`, `.claude/settings.json`, `README.md` — that must not be edited in place.

Split write access per subdirectory rather than blanket group-writable:

| Path | Who needs write |
|---|---|
| `refs/`, `objects/`, `info/`, `logs/`, `packed-refs`, `worktrees/`, `treehouses/` | group members (so `git worktree add` works) |
| `hooks/`, `config`, `description` | maintainer only — git's own policy surface |
| `.claude/`, `README.md`, `CLAUDE.md` | maintainer only — our policy surface |

Concretely: set `core.sharedRepository = group`, `chgrp -R devops foo.git`, then `chmod -R g-w foo.git/{hooks,config,description,.claude,README.md,CLAUDE.md}`. The setgid + sticky bit on `treehouses/` (mode `3775`) means new treehouses inherit the group and nobody can delete a neighbour's treehouse. Group members can spawn treehouses; policy stays out of reach.

Policy files still need to be *changed* — just not in place. Everything in `hooks/`, `.claude/`, and `README.md` is **tracked content on `main`**, and a `reference-transaction` hook syncs the new tree into the bare repo's live paths after every successful merge. Edit `.claude/settings.json` the same way you edit any other file: in your own treehouse, via an MR, merged into `main`. The repo updates itself.

The same tracked files appear in every treehouse's working tree and in any normal clone of the repo. That's the source; the live config is the synced copy in the bare repo. The hook scripts (next section) resolve the project root via `git rev-parse --git-common-dir` plus a `core.bare` check so they spawn treehouses at the right level regardless of where they were invoked from — bare repo, treehouse, or normal clone.

### One branch ⇄ one treehouse

Git already refuses to check out the same branch in two worktrees. Lean on that: the branch *is* the treehouse name, and the existence of the directory is the lock. No external coordination needed.

```
$ ls treehouses/
claude/refactor-auth   feature/cute-thing   main

$ git worktree add treehouses/feature/cute-thing -b feature/cute-thing main
fatal: 'feature/cute-thing' is already checked out at
       '.../treehouses/feature/cute-thing'
```

### Spawning a treehouse

Inside the bare repo, git treats `.` as the git directory. Two commands:

```bash
cd /srv/repos/foo.git
git worktree add treehouses/feature/my-thing -b feature/my-thing main
cd treehouses/feature/my-thing
# you're home
```

That's it. The setgid bit on `treehouses/` puts the new directory in the project group; the sticky bit keeps your neighbour from deleting it (same pattern as the permissions of /tmp). There's no helper script and no setup step — `git worktree add` is the whole onboarding.

**Claude is preconfigured to use the same primitive.** From the CLI, `claude --worktree feature/my-thing` (or just `claude --worktree` for a random name) spawns a session in a fresh treehouse; from inside an existing session, asking Claude to call `EnterWorktree` does the same thing. Both routes go through `.claude/hooks/worktree-create`, which runs the same `git worktree add` under the hood.

### The maintainer's treehouse

`treehouses/main/` is **the maintainer's treehouse**, not a shared scratch area. It's where the person responsible for the project decides which branches get merged into `main` — the only treehouse from which that decision is allowed to land. Everyone else can read it; only the maintainer pushes from it.

If the bare repo's `reference-transaction` hook enforces fast-forward-only + merge-commit-only updates to `main` (as in this project), the rule is mechanical, not social: pushes from anywhere else will simply be rejected.

### Claude integration

`.claude/settings.json` wires Claude's `WorktreeCreate` and `WorktreeRemove` lifecycle hooks to two small shell scripts in `.claude/hooks/`:

```json
{
  "hooks": {
    "WorktreeCreate": [{"hooks": [{"type": "command",
      "command": "/srv/repos/foo.git/.claude/hooks/worktree-create"}]}],
    "WorktreeRemove": [{"hooks": [{"type": "command",
      "command": "/srv/repos/foo.git/.claude/hooks/worktree-remove"}]}]
  }
}
```

The create hook does the same `git worktree add` a human runs by hand, with one extra job: sanitise the branch name pulled from the JSON payload (reject anything outside `[A-Za-z0-9._/-]`, reject `..`) because that input is untrusted in a way a typed-in command isn't. The remove hook refuses to act on any path outside `treehouses/`, then calls `git worktree remove --force`.

Beyond worktree lifecycle, the `.claude/` directory ships permissions, hooks, and skills tuned for this repo — so a Claude session in a fresh treehouse starts already knowing how the project works. No per-session bootstrapping.

### Lifecycle

```
   idea ──► git worktree add … (or claude --worktree)
                       │
                       ▼
              treehouses/<branch>/  ← edit, commit (lands in bare repo)
                       │
                       ▼
              MR / merge into main      (see [[in-tree-issues]])
                       │
                       ▼
              git worktree remove        (or ExitWorktree)
```

There's no separate `push` step — see *Every treehouse is a public branch*. If the bare repo has a `reference-transaction` hook (as in this very project), `main` is fast-forward-only and merge-commit-only, protecting the village hall without extra services.

## Security Considerations 🔐

- **Input sanitisation in the `WorktreeCreate` hook is load-bearing.** The hook pulls a name out of an untrusted JSON payload. Reject anything outside `[A-Za-z0-9._/-]` and reject `..`. A typed-in `git worktree add` doesn't have this problem; the hook does.
- **The remove hook must geofence to `treehouses/`.** A typo (or a prompt-injected agent) shouldn't be able to delete arbitrary paths.
- **No network surface added.** The bare repo is reached via local filesystem permissions; there's no extra daemon to harden.
- **Ownership = authorship.** Every commit in a treehouse is made by the Unix user who owns it. `git log` and `stat` agree on who did what — useful for audit, useful for blame.
- **Isolation is per treehouse, not per process.** Anyone running as user `alice` (whether Alice herself or a Claude session she started) can read every other treehouse on the host. That's a feature for collaboration; pair it with a system-level sandbox if you need stronger separation.
- **`.claude/settings.json` is policy.** It controls what an agent is allowed to do. Treat it like CI config: review changes the same way you review code — and have the bare repo sync it from `main`, never edit it in place.

## Anti-Patterns ⚠️

- ❌ **A wrapper "project" directory around the bare repo.** The bare repo can hold operator config directly; adding a parallel directory adds a sync problem and a place to forget files.
- ❌ **Blanket group-write on the whole bare repo.** Everyone then has write access to `hooks/` and `.claude/`. Split permissions per subdir and sync policy files from `main`.
- ❌ **A helper script that wraps `git worktree add`.** Two lines of plain git do the job; a script adds a maintenance surface for no gain. Sanitisation belongs in the `WorktreeCreate` hook (which has untrusted input), not in a third place.
- ❌ **A clone per user.** Wastes disk, hides in-progress work behind `ssh`, makes "what is everyone doing?" a coordination problem instead of an `ls`.
- ❌ **One shared worktree with branch switching.** Two contributors will collide on `git checkout` within the first hour. Git designed worktrees specifically to make this unnecessary.
- ❌ **Letting an agent share its driver's treehouse.** Either you race the agent for the file lock, or you serialise edits by hand. Give the agent its own treehouse with `EnterWorktree` or `claude --worktree`.
- ❌ **Tracking who-owns-what in Slack / a wiki / a spreadsheet.** `ls -l treehouses/` is the truth; anything else drifts.
- ❌ **Replacing filesystem permissions with a CI policy gate.** The kernel already does access control. Don't outsource a primitive you already have.
- ❌ **Long-lived treehouses.** A branch that lingers past its merge turns `ls treehouses/` from a current-work view into archaeology.
- ❌ **Two `CLAUDE.md` / `README.md` files.** They will drift. Symlink one to the other and write one doc for everyone.

## Best Practices 💡

- **`treehouses/main/` is the maintainer's treehouse**, not a shared scratch area. Treat it as read-only from any other treehouse; merges land there because that's where the merge decision is made.
- **One README for everyone.** Make `CLAUDE.md` a symlink to `README.md` so the village notice board and the agent-onboarding doc can't drift apart. Write it so a teammate or a Claude session both find what they need.
- **Pair this with [[in-tree-issues]]** so the same merge ritual governs both code and the issues that describe it.
- **Sync policy from `main`, never edit it in place.** Hooks, `.claude/`, and `README.md` are tracked files on `main`; a `reference-transaction` hook updates the bare repo's live copies after each successful merge.
- **Remove treehouses on merge.** If the branch is gone from `main`, the treehouse should be too. A short cron that prunes stale, merged-and-empty treehouses keeps the canopy tidy.
- **Put CI / lint inside the bare repo's `hooks/`.** A fresh treehouse inherits them by being a worktree of the bare repo; there is nothing to install.
- **Consider [[treehouse-branch-shape]]** if `ls treehouses/` starts to feel flat — it adds a `<category>/<name>` discipline that self-organises the layout.

## Implementation Checklist ✅

### Set up the village (single repo)

- [ ] `git init --bare /srv/repos/foo.git`, owned by the project Unix group (e.g. `devops`).
- [ ] `git config core.sharedRepository group` in the bare repo.
- [ ] Create `foo.git/treehouses/` with `chmod 3775` (setgid + sticky + group write).
- [ ] Add `/treehouses/` to `.gitignore` on `main` so normal clones don't track the directory.
- [ ] Strip group-write on policy paths: `chmod -R g-w foo.git/{hooks,config,description,.claude,README.md,CLAUDE.md}` once they exist.
- [ ] Add `treehouses/main/` as the maintainer's treehouse. Make sure the maintainer (not the `devops` group) owns it.
- [ ] Write `foo.git/README.md` with: where the bare repo lives, the two-line `git worktree add` recipe, the `claude --worktree` entry point, and the who-merges-what rule for `treehouses/main/`.
- [ ] `ln -s README.md foo.git/CLAUDE.md`.

### Wire the Claude path

- [ ] Add `foo.git/.claude/settings.json` with `WorktreeCreate` and `WorktreeRemove` hooks.
- [ ] Add `foo.git/.claude/hooks/worktree-create` that reads the JSON from stdin, sanitises the name (`[A-Za-z0-9._/-]`, no `..`), resolves the project root via `git rev-parse --git-common-dir` + a `core.bare` check (so it works from a bare repo, a treehouse, or a normal clone), runs `git worktree add`, and prints the path.
- [ ] Add `foo.git/.claude/hooks/worktree-remove` that refuses any path outside `<project-root>/treehouses/` and then calls `git worktree remove --force`.
- [ ] Confirm the `CLAUDE.md → README.md` symlink covers the agent-side instructions too (use `EnterWorktree` or `claude --worktree`, never edit a treehouse you don't own).

### Protect the village hall

- [ ] In `foo.git/hooks/`, set a `reference-transaction` policy on `main` (fast-forward-only, merge-commit-only) — see [[in-tree-issues]] for the shape.
- [ ] Extend the same hook to sync `hooks/`, `.claude/`, and `README.md` from the new tree into the bare repo's live paths after every successful merge.
- [ ] Add `pre-receive` / `update` hooks for CI and lint that every push must satisfy.
- [ ] Auto-push `main` to your public mirrors from the same hook.

## Related Patterns 🔗

- [Smalltown Infrastructure 🏘️](./smalltown-infrastructure.md) — the bigger village this treehouse sits in: small, legible, operable by the team you actually have.
- [In-Tree Issues 🗂️](./in-tree-issues.md) — pair the worktree ritual with the same merge ritual for issues; both flow through `main`.
- [Treehouse Branch Shape 🪧](../../issues/treehouse-branch-shape.pattern.md) — *(draft)* an optional `<category>/<name>` discipline for treehouse names.
- [Village of Villages 🏘️](../../issues/village-of-villages.pattern.md) — *(draft)* multi-repo extension: a thin org directory wrapping several bare repos, each one its own treehouse village.
- [Cuteness Pattern 🌸](../meta/cuteness.md) — why two lines of plain git and an `ls treehouses/` are friendlier than a sprawl of clones.

## References 📚

- `git-worktree(1)` — the primitive everything sits on.
- `git-init(1)` — `--bare`, `core.sharedRepository`, and the conventional `.git` suffix.
- `chmod(1)` — the setgid + sticky combination (`3775`) is the whole permission story for `treehouses/`.
- Example implementation: the sister project `idmcd-devops-portal` (`.claude/hooks/worktree-create`, `.claude/hooks/worktree-remove`, `tests/reference-transaction`).
