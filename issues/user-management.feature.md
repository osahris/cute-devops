---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# User management

## Goal

Close the gaps in the existing `users` role and lock the spec. Implementation is largely in place; this ticket lists what's already done, what's missing, and the decisions behind what gets added (and, importantly, what does not).

## Status quo

`roles/users` today handles:

- User account: name, uid, primary group via `gid`, secondary groups, shell (defaults to `/usr/bin/fish`), home, home creation with the distro's default skeleton.
- SSH authorized keys from an **inline list** on the user spec; templated into `~/.ssh/authorized_keys`.
- Auto-generates a per-user ed25519 keypair.
- Password as a **pre-hashed string** on the user spec.
- Systemd `linger: true/false` toggle.
- User rename via `old_name` (groupmod + usermod, currently with `failed_when: false`).
- Separate `user_groups` dict for creating standalone groups.
- Debian setup installs `dbus` + `libpam-systemd`.

Known bugs worth fixing alongside the scope additions:

- The README shows `users` as a YAML list; the code expects a **dict keyed by username**. Fix the README, don't change the code — the dict shape is what every task uses (`users.keys() | list`, `users[item].X`).
- `move.yaml` uses `failed_when: false` on both `groupmod` and `usermod`, which silently swallows rename errors. Should surface failures or at least scope the tolerance to "already renamed" specifically.

## Scope

### Central SSH authorized keys under `/etc`

Write authorized_keys for managed users to `/etc/ssh/authorized_keys.d/<user>`, root-owned and not user-writable, so a compromised account cannot self-add a persistence key and the permitted set is auditable from one directory. This role only writes the files — wiring `sshd_config`'s `AuthorizedKeysFile` to consume them is the `ssh_server` role's job (see `ssh-server.feature.md`).

### Sudo policy per user

Managed via `/etc/sudoers.d/<name>` drop-in files written by the role, not by adding users to the `sudo` group. Two flags for now: `sudo: true` and `sudo_nopasswd: true`. Intentionally minimal; richer policies (command lists, per-group grants) can come later if real demand shows up. Enforced via `visudo -c` before install so a mistake never leaves the box sudo-broken.

### Dotfile / skeleton starting point

The role deploys a skeleton (opinionated fish prompt, `.gitconfig` stub, `.ssh/config` stub, etc.) to the user's home. Two modes, selectable per user: **on creation only** (default — user edits survive subsequent converges) and **always** (re-applied every run — for accounts whose dotfiles are centrally managed). Content comes from `/etc/skel` plus a dynamic templates directory rendered per user; see design notes.

### Full lifecycle — deprovisioning

First-class `state: absent` handling. On removal: the account is locked, a final backup of the home is taken via a dedicated restic backup class (captures the last state into the project's restic repo), and only then are account and home deleted. Because the home's last state lives in the backup repo, no separate on-host archive is needed. Accidental-removal guard: the role refuses to process a removal unless the user is explicitly listed with `state: absent` — disappearing from inventory alone is not enough.

### Password handling

The existing "raw pre-hashed string" field stays — hashes live in the inventory YAML in Git, and a password change is a merge request that syncs to every machine on the next converge.

**Helper that edits the inventory YAML.** What's missing today is a smooth path for a user to put their hash into the inventory. Ship a helper that, given a plaintext password entered locally, computes the sha512 hash and then **writes or updates that user's entry in the inventory YAML directly** — either adding a full new user entry or just updating the `password:` field of an existing one. Plaintext never leaves the developer's shell; the repo only ever sees the hash. The end user's workflow is "run the helper, commit, open MR" rather than "run a hasher, copy the output, hand-edit YAML, commit, open MR".

## Design notes

### Data shape

- Keeping the dict-keyed-by-username shape despite the README being wrong: the dict lets other roles (setup_deploy, notify, etc.) reference a specific user cleanly via `users[foo]` rather than filtering a list. Fixing the README is cheaper than reshaping every consumer.

### Sudo

- Sudoers via drop-in files, not group membership, for two reasons:
  (a) drop-ins enable more fine-grained access control per user, e.g. no-passwd vs passwd or even non-root sudo;
  (b) a per-user sudoers file takes effect immediately on creation, whereas group-based sudo requires the user to log out and back in before the new group membership is visible to `sudo`.

### Skeleton / dotfiles

- Skeleton source is `/etc/skel` **plus** a dynamic templates directory rendered per user (so values like the username, email, or git identity get interpolated). The dynamic layer is what `/etc/skel` alone cannot do — `/etc/skel` is static files copied verbatim, which is fine for bytes-identical dotfiles but breaks the moment a `.gitconfig` needs the user's name in it.
- The role ships its own `files/skel/` directory that populates the system's `/etc/skel` at converge time, so the static layer is under the collection's control rather than whatever the distro happens to ship.
- Two skeleton modes exist because the two use cases are genuinely different: personal accounts want "good starting point, then leave me alone" (on-create); shared / service / kiosk-style accounts want "these dotfiles are canonical, re-apply every run" (always). Per-user selection keeps both modes first-class instead of forcing one policy on everyone.

### Deprovisioning

- Deprovisioning backs the home up through the restic backup class (not an on-host archive) so the last state ends up in the same repo, retention, and offsite cascade as every other backup — no special "deleted-users" path to monitor separately, and a `/home/<name>` is not left lingering to block a future same-name recreation.
