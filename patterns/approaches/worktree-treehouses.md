---
title: Worktree Treehouses 🌳
---

<!--
SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## Overview 📋

A bare git repository lives on one host. Inside the bare repo, alongside `HEAD` and `refs/`, sit a `README.md`, a `.claude/` directory with hooks, permissions and skills already configured for claude usage, and a `treehouses/` directory — one git worktree per contributor, human or AI agent. The bare repo *is* the project: there's no wrapper directory, no parallel "checkout root", no per-clone setup. You `cd` into the bare repo and everything you need is already there.

Every contributor (human or agent) gets their own **treehouse**: a private worktree built on the same tree everyone else is climbing. A treehouse is light, spawnable, easy to dismantle — pull yourself up, do your work, climb back down. Filesystem permissions decide who can write where, and `git worktree` itself guarantees that no two treehouses share a branch.

Same trick whether you have three operators sharing a server or one operator with three Claude agents: every actor gets a treehouse, and the bare repo they all push to lives in the same directory.

## Goals 🎯

- Give every contributor (human or agent) an isolated working tree without the cost of managing a full clone per person.
- Let teammates read each other's in-progress work without races or permission gymnastics.
- Push access control down to the kernel and git hooks — no bespoke "who can touch this branch" service.
- Keep CI / linting / agent config inside the bare repo, so a fresh treehouse works the moment it exists. No `git clone && set up hooks && copy .env` ritual.
- Lean on plain `git worktree` for the human path; let agents go through the same primitive via a single Claude hook. No bespoke helper script to maintain.
- Make `ls treehouses/` the honest answer to "what's everyone working on right now?"

## Pattern Structure 📑

### The bare repo *is* the project

```
/srv/repos/foo.git/                   ← the project; bare repo
├── HEAD, refs/, objects/, info/      ← standard bare-repo internals
├── worktrees/                        ← git's per-worktree metadata
├── hooks/
│   ├── pre-receive                   ← CI + ref-shape gate
│   └── reference-transaction         ← main-line policy + config sync
├── README.md                         ← village notice board
├── CLAUDE.md → README.md             ← symlink: same doc, two readers
├── .claude/
│   ├── settings.json                 ← wires WorktreeCreate/Remove hooks
│   └── hooks/
│       ├── worktree-create           ← creates + chmods a treehouse
│       └── worktree-remove
└── treehouses/                       ← 3775 (rwxrwsr-t)
    ├── main/                         ← the maintainer's treehouse
    ├── feature/cute-thing/           ← built by alice
    └── claude/refactor-auth/         ← built by the user driving Claude
```

Git reserves only a handful of names inside a bare repo (`HEAD`, `refs/`, `objects/`, `info/`, `hooks/`, `logs/`, `packed-refs`, `config`, `description`, `worktrees/`). Everything else is yours. So the operator config, the agent config, and the treehouses all live *inside* the bare repo itself. The result:

- **`cd foo.git/` is arrival.** Nothing exists "above" the repo.
- **Backup is one directory.** `rsync /srv/repos/foo.git/` carries history, hooks, agent config, and active treehouses.
- **CI / lint / agent permissions travel with the repo.** A fresh clone of the bare repo on a new host is a fully-configured workspace, not a starting point.

`worktrees/` (git's internal per-worktree metadata) and `treehouses/` (the actual checkouts) sit side by side without visual collision: one is git's bookkeeping, the other is where you live while you work.

The `.git` suffix on the directory is convention — keep it. Every tool and every operator's muscle memory expects it.

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

`git worktree add` writes into the bare repo: it creates `worktrees/<name>/` (git's metadata) and the treehouse directory itself. So operators *must* have write access to parts of the bare repo. But the bare repo also holds **policy** — `hooks/`, `.claude/settings.json`, `README.md` — that must not be edited in place.

Split write access per subdirectory rather than blanket group-writable:

| Path | Who needs write |
|---|---|
| `refs/`, `objects/`, `info/`, `logs/`, `packed-refs`, `worktrees/`, `treehouses/` | operators (so `git push` + `git worktree add` work) |
| `hooks/`, `config`, `description` | maintainer only — git's own policy surface |
| `.claude/`, `README.md`, `CLAUDE.md` | maintainer only — our policy surface |

Concretely: set `core.sharedRepository = group`, `chgrp -R devops foo.git`, then `chmod -R g-w foo.git/{hooks,config,description,.claude,README.md,CLAUDE.md}`. Operators can push and spawn treehouses; policy stays out of reach.

Policy files still need to be *changed* — just not in place. Everything in `hooks/`, `.claude/`, and `README.md` is **tracked content on `main`**, and a `reference-transaction` hook syncs the new tree into the bare repo's live paths after every successful merge. Edit `.claude/settings.json` the same way you edit any other file: in your own treehouse, via an MR, merged into `main`. The repo updates itself.

### One branch ⇄ one treehouse

Git already refuses to check out the same branch in two worktrees. Lean on that: the branch *is* the treehouse name, and the existence of the directory is the lock. No external coordination needed.

```
$ ls treehouses/
claude/refactor-auth   feature/cute-thing   main

$ git worktree add treehouses/feature/cute-thing -b feature/cute-thing main
fatal: 'feature/cute-thing' is already checked out at
       '.../treehouses/feature/cute-thing'
```

### Branch shape: `<category>/<name>`

Every treehouse lives at `treehouses/<category>/<name>`. The branch name carries the same shape: `feature/cute-thing`, `fix/login-redirect`, `claude/refactor-auth`, `bot/auto-bump`, `experiment/new-router`. The only exempt slot is `main/` — the maintainer's treehouse.

The bare repo's `pre-receive` (or `update`) hook rejects any ref outside `refs/heads/<category>/<name>` (and `refs/heads/main`). Because treehouses share the bare repo's refs database, that hook fires the moment you commit in a treehouse — there's no "push later" step where someone could sneak a malformed name through.

Don't hard-code the allowed categories. The shape is the discipline; which prefixes a project uses is the project's call. `ls treehouses/` self-organises into directories per category, which means `ls treehouses/feature/` is a one-line answer to "what features are in flight?"

### The human path: plain `git worktree`

Inside the bare repo, git treats `.` as the git directory. No helper, no `--git-dir`:

```bash
cd /srv/repos/foo.git
git worktree add treehouses/feature/my-thing -b feature/my-thing main
chmod -R g-w treehouses/feature/my-thing
cd treehouses/feature/my-thing
# you're home
```

Three commands. The `chmod -R g-w` is the polish: by default a freshly-created treehouse inherits the `devops` group through setgid, and depending on the operator's umask its files may be group-writable too. The `chmod` makes the treehouse readable to the group but writable only by you.

That's it. There's no `bin/new-treehouse` script; the four lines of polish a script would add are easier to write into the README than to maintain as code.

### The maintainer's treehouse

`treehouses/main/` is **the maintainer's treehouse**, not a shared scratch area. It's where the person responsible for the project decides which branches get merged into `main` — the only treehouse from which that decision is allowed to land. Everyone else can read it; only the maintainer pushes from it.

If the bare repo's `reference-transaction` hook enforces fast-forward-only + merge-commit-only updates to `main` (as in this project), the rule is mechanical, not social: pushes from anywhere else will simply be rejected.

### The village notice board

`README.md` at the bare repo's root is written for **humans walking up to a shell prompt for the first time**. It says:

- where the bare repo lives,
- how the `treehouses/` layout works,
- the three-line `git worktree add` recipe,
- how Claude users spawn one with `claude --worktree`,
- who owns `treehouses/main/` and what that means for merging.

`CLAUDE.md` is a **symlink to `README.md`**. Agents and humans read the same instructions; one file means they can't drift apart.

A minimal README body — the part operators actually need — reads:

````markdown
## Spawning your own treehouse

Pick a branch name in the shape `<category>/<name>` — for example `feature/cute-thing`, `fix/login-redirect`, `claude/refactor-auth`. Then either:

```bash
# Plain git, the canonical way:
cd /srv/repos/foo.git
git worktree add treehouses/feature/cute-thing -b feature/cute-thing main
chmod -R g-w treehouses/feature/cute-thing
cd treehouses/feature/cute-thing

# Or, if you're a Claude user, ask Claude:
claude --worktree feature/cute-thing
```

Both produce the same thing: a fresh worktree at `treehouses/<branch>` on a fresh branch off `main`, owned by you, readable by the group.

## Who merges what

`treehouses/main/` is the maintainer's treehouse. To get your branch in, open an MR / PR against `main`; the maintainer reviews from their treehouse and lands the merge there. Don't push to `main` from your own treehouse — the bare repo's hooks will reject it.
````

### Agent integration

`.claude/settings.json` wires Claude's `WorktreeCreate` and `WorktreeRemove` lifecycle hooks to two small shell scripts in `.claude/hooks/`. The create hook does the same three things the human runs by hand — `git worktree add`, `chmod -R g-w`, print the path — with one extra job: sanitise the branch name pulled from the JSON payload (reject anything outside `[A-Za-z0-9._/-]`, reject `..`, enforce the `<category>/<name>` shape).

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

The remove hook is paranoid by design: it refuses to act on any path that isn't under `treehouses/`, then calls `git worktree remove --force`.

Users have two entry points into a Claude session inside a treehouse:

```bash
# Spawn a fresh session that creates and enters a new treehouse:
claude --worktree claude/my-task   # explicit branch name
claude --worktree                  # let Claude pick a random one

# Or, from inside an existing session, ask Claude to call
# EnterWorktree on a branch name.
```

Both flow through the same WorktreeCreate hook.

### Lifecycle

```
   idea ──► git worktree add … (human) | claude --worktree (agent)
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

### Multi-Repo Variant: Village of Villages 🏘️

For organisations that run several related bare repos, wrap them in a thin **org directory**:

```
/srv/orgs/acme/
├── README.md                     ← onboarding for the whole org
├── CLAUDE.md → README.md
├── .claude/                      ← shared agent skills / settings
├── manifest.yaml                 ← lists member repos + their roles
└── repos/
    ├── frontend.git/             ← each is itself a treehouse village
    │   ├── treehouses/{main, feature/x, ...}
    │   └── .claude/, hooks/, README.md
    ├── backend.git/
    │   └── treehouses/...
    └── infra.git/
        └── treehouses/...
```

Each bare repo still follows the single-repo pattern internally — the org directory adds nothing to the per-repo workflow. What it provides:

- **Cross-repo docs.** README for "what is acme, what does each repo do, where do issues go?"
- **Shared agent assets.** Skills, slash commands, and prompts that apply org-wide live in the org's `.claude/`; per-repo `.claude/` inherits them by reference.
- **A manifest.** Plain YAML naming each member repo, who maintains it, and any version-pinning rules. The manifest is the *only* thing humans need to read to understand the shape of the org.

What it deliberately does *not* do:

- **No org-level worktree.** Treehouses live inside the individual repos, not at the org root. A contributor working across repos has multiple treehouses, one per repo — same as a single-repo contributor with multiple branches.
- **No shared `main` treehouse.** Each repo's maintainer owns their own `treehouses/main/`; an org-wide merge is several per-repo merges.

A "composed treehouse" — one directory holding coordinated worktrees of several repos on related branches — is a tempting extension but needs real tooling (multi-worktree helper, branch-bundle manifest, cross-repo MR ritual). Don't build it until a real workflow asks for it; until then, contributors juggle per-repo treehouses by hand.

## Security Considerations 🔐

- **Input sanitisation in the `WorktreeCreate` hook is load-bearing.** The hook pulls a name out of an untrusted JSON payload. Reject anything outside `[A-Za-z0-9._/-]`, reject `..`, and enforce the `<category>/<name>` shape. (The human path is plain typed git, no untrusted input.)
- **The remove hook must geofence to `treehouses/`.** A typo (or a prompt-injected agent) shouldn't be able to delete arbitrary paths.
- **The `pre-receive` hook is the backstop.** Even if a treehouse is created under a malformed name, no commit on a non-conforming ref will ever land in the bare repo — the hook fires on the worktree's first commit too.
- **No network surface added.** The bare repo is reached via local filesystem permissions; there's no extra daemon to harden.
- **Ownership = authorship.** Every commit in a treehouse is made by the Unix user who owns it. `git log` and `stat` agree on who did what — useful for audit, useful for blame.
- **Agent isolation is per treehouse, not per process.** An agent running as user `alice` can still read everyone else's treehouse. That's a feature for collaboration; pair it with a system-level sandbox if you need stronger separation.
- **`.claude/settings.json` is policy.** It controls what an agent is allowed to do. Treat it like CI config: review changes the same way you review code — and have the bare repo sync it from `main`, never edit it in place.

## Anti-Patterns ⚠️

- ❌ **A wrapper "project" directory around the bare repo.** The bare repo can hold operator config directly; adding a parallel directory adds a sync problem and a place to forget files.
- ❌ **Blanket group-write on the whole bare repo.** Operators then have write access to `hooks/` and `.claude/`. Split permissions per subdir and sync policy files from `main`.
- ❌ **A helper script that wraps `git worktree add`.** Three lines of plain git do the job; a script adds a maintenance surface for no gain. Sanitisation belongs in the WorktreeCreate hook (which has untrusted input) and the pre-receive hook (which is the backstop), not in a third place.
- ❌ **A clone per user.** Wastes disk, hides in-progress work behind `ssh`, makes "what is everyone doing?" a coordination problem instead of an `ls`.
- ❌ **One shared worktree with branch switching.** Two humans (or a human and an agent) will collide on `git checkout` within the first hour. Git designed worktrees specifically to make this unnecessary.
- ❌ **Letting an agent share its driver's treehouse.** Either you race the agent for the file lock, or you serialise edits by hand. Give the agent its own treehouse with `EnterWorktree` or `claude --worktree`.
- ❌ **Bare top-level branch names.** A treehouse at `treehouses/quickfix/` with no category prefix breaks the self-organising layout. Enforce `<category>/<name>` in the pre-receive hook.
- ❌ **Tracking who-owns-what in Slack / a wiki / a spreadsheet.** `ls -l treehouses/` is the truth; anything else drifts.
- ❌ **Replacing filesystem permissions with a CI policy gate.** The kernel already does access control. Don't outsource a primitive you already have.
- ❌ **Long-lived treehouses.** A branch that lingers past its merge turns `ls treehouses/` from a current-work view into archaeology.
- ❌ **Two `CLAUDE.md` / `README.md` files.** They will drift. Symlink one to the other and write for both audiences.

## Best Practices 💡

- **Pick category prefixes that say something.** `feature/`, `fix/`, `experiment/`, `claude/`, `bot/` — whatever your project uses, make `ls treehouses/` legible at a glance.
- **`treehouses/main/` is the maintainer's treehouse**, not a shared scratch area. Treat it as read-only from any other treehouse; merges land there because that's where the merge decision is made.
- **Same README for humans and agents.** Make `CLAUDE.md` a symlink to `README.md` so the village notice board and the agent instructions can't drift apart.
- **Pair this with [[in-tree-issues]]** so the same merge ritual governs both code and the issues that describe it.
- **Sync policy from `main`, never edit it in place.** Hooks, `.claude/`, and `README.md` are tracked files on `main`; a `reference-transaction` hook updates the bare repo's live copies after each successful merge.
- **Remove treehouses on merge.** If the branch is gone from `main`, the treehouse should be too. A short cron that prunes stale, merged-and-empty treehouses keeps the canopy tidy.
- **Put CI / lint inside the bare repo's `hooks/`.** A fresh treehouse inherits them by being a worktree of the bare repo; there is nothing to install.

## Implementation Checklist ✅

### Set up the village (single repo)

- [ ] `git init --bare /srv/repos/foo.git`, owned by the project Unix group (e.g. `devops`).
- [ ] `git config core.sharedRepository group` in the bare repo.
- [ ] Create `foo.git/treehouses/` with `chmod 3775` (setgid + sticky + group write).
- [ ] Strip group-write on policy paths: `chmod -R g-w foo.git/{hooks,config,description,.claude,README.md,CLAUDE.md}` once they exist.
- [ ] Add `treehouses/main/` as the maintainer's treehouse. Make sure the maintainer (not the `devops` group) owns it.
- [ ] Write `foo.git/README.md` with: where the bare repo lives, the three-line `git worktree add` recipe, the `claude --worktree` entry point, and the who-merges-what rule for `treehouses/main/`.
- [ ] `ln -s README.md foo.git/CLAUDE.md`.

### Wire the agent path

- [ ] Add `foo.git/.claude/settings.json` with `WorktreeCreate` and `WorktreeRemove` hooks.
- [ ] Add `foo.git/.claude/hooks/worktree-create` that reads the JSON from stdin, sanitises the name (`[A-Za-z0-9._/-]`, no `..`, `<category>/<name>` shape), runs `git worktree add`, runs `chmod -R g-w` on the new treehouse, and prints the path.
- [ ] Add `foo.git/.claude/hooks/worktree-remove` that refuses any path outside `treehouses/` and then calls `git worktree remove --force`.
- [ ] Confirm the `CLAUDE.md → README.md` symlink covers the agent-side instructions too (use `EnterWorktree` or `claude --worktree`, never edit a treehouse you don't own).

### Protect the village hall

- [ ] In `foo.git/hooks/`, set a `reference-transaction` policy on `main` (fast-forward-only, merge-commit-only) — see [[in-tree-issues]] for the shape.
- [ ] Extend the same hook to sync `hooks/`, `.claude/`, and `README.md` from the new tree into the bare repo's live paths after every successful merge.
- [ ] Add `pre-receive` / `update` hooks for CI and lint that every push must satisfy, plus the `<category>/<name>` ref-name check.
- [ ] Auto-push `main` to your public mirrors from the same hook.

### Add a multi-repo wrapper (only if you need it)

- [ ] Create `/srv/orgs/<org>/` with its own `README.md`, `CLAUDE.md → README.md` symlink, and `.claude/` for shared agent assets.
- [ ] Place each member bare repo under `repos/<name>.git/`. Each still follows the single-repo checklist above.
- [ ] Add `manifest.yaml` listing the member repos, their maintainers, and any pinned relationships.
- [ ] Resist adding an org-level `treehouses/`. Treehouses live in the repos.

## Related Patterns 🔗

- [Smalltown Infrastructure 🏘️](./smalltown-infrastructure.md) — the bigger village this treehouse sits in: small, legible, operable by the team you actually have.
- [In-Tree Issues 🗂️](./in-tree-issues.md) — pair the worktree ritual with the same merge ritual for issues; both flow through `main`.
- [Cuteness Pattern 🌸](../meta/cuteness.md) — why three lines of plain git and an `ls treehouses/` are friendlier than a sprawl of clones.

## References 📚

- `git-worktree(1)` — the primitive everything sits on.
- `git-init(1)` — `--bare`, `core.sharedRepository`, and the conventional `.git` suffix.
- `chmod(1)` — the setgid + sticky combination (`3775`) is the whole permission story for `treehouses/`.
- Example implementation: the sister project `idmcd-devops-portal` (`.claude/hooks/worktree-create`, `.claude/hooks/worktree-remove`, `tests/reference-transaction`).
