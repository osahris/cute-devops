---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Backup — borg path

## Goal

A working borg backup path as the **second, deliberately minimal** backup solution alongside restic. Where the restic path is our full-fledged self-managed cascade (client / server / custodian, per-project aggregation, offsite via Hetzner Storage Box, all parts specified down to systemd drop-ins — see `backup-restic.feature.md`), the borg path is the opposite stance: **import an upstream community Ansible role, set defaults, add run monitoring, move on.** The point is diversity — a second path with a different codebase, different format, different trust model — not a second fully-engineered system.

Together the two paths give us the "2 media" leg of 3-2-1: if a bug, supply-chain compromise, or format quirk takes out restic, borg is still there, and vice versa.

## Scope

### Strategy: thin wrapper over an upstream role

We do not implement a borg role from scratch. A small `borg_client` role in this collection:

- Pulls in [BorgBase's `ansible-role-borgbackup`](https://github.com/borgbase/ansible-role-borgbackup) as the upstream dependency.
- Sets collection-appropriate defaults (backup paths, retention policy, schedule) so a typical host gets a sensible borg backup by just including the role.
- Drops in the monitoring bits that upstream does not provide (see Monitoring below).

No custom server role. Clients push directly to:

- [Borgbase](https://www.borgbase.com/) — purpose-built borg hosting.
- [Hetzner Storage Box with borg](https://docs.hetzner.com/storage/storage-box/access/access-ssh-rsync-borg/) — already used by the restic path, cheap per TB.

No default — operators pick. The docs page for this role lists both with the links above; the trade-off (UI/feature polish at Borgbase vs. cost and one-provider simplicity at Hetzner) is for the site to decide. Either way the remote is someone else's problem to operate — restic is the "we run it" path, borg is the "someone else runs it" path. No central custodian, no offsite cascade we operate — borg's remote *is* the offsite.

### What's in scope

- Thin `borg_client` role that wraps the upstream role with our defaults. The upstream role drives borg via **borgmatic**, which means we get scheduled backups, retention (`borg prune`), and integrity checks (`borg check`) wired in for free — no separate timer battery on our side for borg.
- Host-level filesystem backup (equivalent of restic's `fs-` class). Service-level and VM-level backups stay on the restic path; borg is the dumb filesystem second line.
- Encryption passphrase sourced from `pass` and delivered via the secrets role (same machinery as restic). Keeping the canonical copy in `pass` means the admin can decrypt a borg archive straight from the remote even when every host that ever used the passphrase is gone — a realistic DR scenario, and the whole reason borg is the second path.
- **Alerta integration via borgmatic hooks.** Ship a default playbook / role-bundled snippet that configures borgmatic's `before_backup`, `after_backup`, and `on_error` hooks to `curl` the site's alerta endpoint — heartbeats on success, alerts on failure. This replaces the "write our own run check" work: borgmatic already knows when a run started, finished, or failed, and alerta already knows how to route heartbeats with expiry. Two curl calls wire the two together.
- Basic restore documentation — the upstream role's restore story plus a short how-to for our defaults.

### What's out of scope

- No self-hosted `borg_server` role. If a site wants one, they run one outside this collection.
- No custodian, no offsite replication we drive, no project-level aggregation. Borg does not need them in this design — the remote is already offsite and already operated by someone else.
- No append-only-vs-prune key split engineered by us. Whatever the upstream role and Borgbase/Hetzner support is what we use.

## Design notes

- **Why a second path at all.** Restic is extremely well-specified in this collection, but one bug or one format-level mistake could still take it out. Borg is a completely independent codebase, format, and crypto stack. Running both in parallel is cheap (borg is one more systemd timer on the client) and gives us genuine redundancy.
- **Why not symmetric with restic.** Symmetry would mean a second custodian, second offsite cascade, second base-repo scheme — doubling the ongoing maintenance burden for a backup we hope never to need. The whole point of the second path is *low effort*. A thin wrapper over a battle-tested upstream role, pointed at a hosted remote, is the minimum that still delivers on "we have a second backup."
- **Why Borgbase / Hetzner native borg.** Both are cheap, well-operated, and built specifically for borg. Running our own borg server reintroduces exactly the ops burden we are trying to avoid by having a second path. If we wanted "our own server", that is already what restic is for.
- **Why no custodian for borg.** The custodian pattern exists in the restic path because restic's topology splits password access across client/server/offsite in a way that benefits from a dedicated observer. Borg's topology is one-shot (client → remote), no split, so there is nothing for a custodian to observe that the client's own run check does not already cover.
- **Scope of the collection's contribution.** The upstream role does the heavy lifting. Our `borg_client` role contributes: opinionated defaults, integration with the secrets role, integration with the checker/alerting framework. Three thin layers, nothing load-bearing that upstream does not already provide.
- **Running as root — and an upstream patch to change that.** The upstream role runs borg (and borgmatic) as root today. The role's systemd unit already has a commented-out line that would grant `CAP_DAC_READ_SEARCH` instead — the exact capability model the restic `fs-` class uses. As part of this ticket we send a pull request upstream making that line active and configurable (opt-in via a role variable, default off to preserve current behaviour). If it merges, we flip our default to the capability path and drop root; if it stalls, we accept root as the cost of the quick-solution stance and move on. Contributing the patch upstream rather than forking keeps the thin-wrapper premise intact.
