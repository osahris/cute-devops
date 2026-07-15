---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Backup — restic path

## Goal

Working `restic_client`, `restic_server`, and `restic_custodian` roles as one of two backup paths (alongside borg) to satisfy 3-2-1. The three roles are the load-bearing triad: the client produces snapshots, the server holds the near repos password-free, the custodian carries read-only keys and does all the work that needs keys (freshness checks, integrity checks, offsite replication).

Per-host isolation on the server side, three backup levels on the client side (machine / host / service) delivered as three systemd job classes, monitored end-to-end, single near-target for clients with offsite cascade handled by the custodian.

## Status quo

The collection already ships working `restic_client` and `restic_server` roles inherited from an earlier project. This ticket defines the target shape. Implementation is a rework that keeps what works (systemd timer + oneshot pattern, client-generates-then-delegates htpasswd) and swaps out the parts that do not scale (single-server URL as the only config knob, inline plaintext `key:` in inventory, two flavors of job in one list, no custodian role at all, no offsite cascade, no integrity checks). The new `restic_custodian` role is net-new and specified in the Custodian section below.

## Scope

### Backup levels

- **Host** (`fs-` class) — filesystem paths on the guest. **Implemented in this ticket.**
- **Service** (`srv-` class) — streamed export from a running service (database dump, Keycloak realm export, Grafana dashboards, …) via restic's `--stdin` mode. **Shape defined here; wiring depends on `service-export-import.pattern.md`.**
- **Virtual Machine** (`vm-` class) — VM disk volumes exported from the hypervisor layer, writing into the per-project `<project>/vms` repo. **Specified here as a named class with a stub drop-in; the actual export mechanism lands in a future hypervisor-backup ticket.**

The three levels map one-to-one onto three job classes on the systemd side (see Unit model). Each class is a dash-prefix on the instance name; each gets its own class drop-in for activation, privileges, and trigger mechanism. Specifying all three classes up front — even for the not-yet-implemented vm case — reserves the namespace and pins the convention so a future ticket does not reshape it.

**Class-tag convention.** Every snapshot is tagged `vm`, `fs`, or `srv` at `restic backup` time. The class drop-in carries the tag (see Unit model), so the convention is enforced by the unit files, not by admin discipline. Snapshots in a shared repo stay filterable by level (`restic snapshots --tag fs`, `restic snapshots --tag srv`, etc.) without needing a naming convention on the snapshot side.

### Default repo model

A host belongs primarily to one project. Central shared infrastructure lives in a project conventionally named `<org>-infra`; separate backup infrastructure lives in a project conventionally named `<org>-backup`. The canonical definitions of *project*, *service*, *host* and the conventional naming patterns live in `project-service-terminology.pattern.md`; this ticket assumes them.

Near repo path on the server: `/<project>/<host>`. One repo per host — all jobs on that host write snapshots into the same repo, separated by `--tag` and snapshot paths, which is restic's native pattern. Project is an organizational prefix, not an enforced trust boundary. Enforcement is at the host level: one restic-server user per host, one access-allowed path prefix per user, enforced by `--private-repos`. Hosts within a project are isolated from each other's backups.

Offsite repo layout is **per-project, aggregated**: `/<project>` on Hetzner Storage Box. All hosts in a project feed into one offsite repo via `restic copy` from the custodian (details in the cascade section). This unlocks cross-host dedup at the offsite layer — similar VM images, shared OS packages, the same config files across a fleet — because restic's content-defined chunking dedups identical chunks across snapshots within a repo.

Blue-green deployments within a project — and the use of restore-into-green as a DR drill — are tracked separately in `blue-green-deployments.pattern.md`.

### Project base repo for efficient deduplication

Restic's content-defined chunking uses a per-repo crypto parameters picked at `restic init`. Two repos with different crypto parameters cannot dedup against each other — `restic copy` still works, but every chunk ends up re-stored on the destination, losing the point of aggregation.

Per-project chunker alignment uses a dedicated **project base repo** as the crypto parameter anchor:

1. Project bootstrap creates `/<project>/.restic-base-repo` on the backup server — an empty repo whose sole purpose is holding the project's chunker polynomial. It never receives real snapshots.
2. Every other repo in the project — near repos on the server, the `<project>/vms` repo, the aggregated offsite repo on Hetzner — is initialized with `restic init --copy-chunker-params --from-repo <backup-server-url>/<project>/.restic-base-repo`. Same crypto parameters, dedup works across the whole project.
3. Init is idempotent: on re-run, the custodian (which owns this ritual — see the Custodian section below) checks whether a repo already exists; if yes, it verifies the chunker polynomial matches the project base and warns if it does not match.

Using a dedicated base repo rather than "whichever repo was first" removes ambiguity about which polynomial a project uses and makes the bootstrap ritual explicit: create `/<project>/.restic-base-repo`, then every subsequent repo is derived. The base repo is tiny (just metadata) and can be read-shared project-wide so any future init from any host works without project-admin credentials.

This same mechanism resolves the hypervisor-VM dedup case: the project's `<project>/vms` repo shares the project's crypto parameters via the base repo, so identical OS images across VMs dedup into a common chunk pool inside that repo and again inside the offsite aggregation.

**Access to the base repo under `--private-repos`.** restic-server's private-repos mode restricts each user to paths matching their username. For the base repo at `/<project>/.restic-base-repo`, that means the user account itself has to be named `<project>/.restic-base-repo`. The base-repo password is stored only on the custodian (provisioned via the secrets role, same machinery as per-host repo credentials).

The client still owns the init code path. When a near repo needs to be initialized, the controller fetches the base-repo password from the custodian into a transient Ansible fact (`no_log` throughout), runs `restic init --copy-chunker-params --from-repo <backup-server-url>/<project>/.restic-base-repo` on the client with both passwords in its environment, and the fact is discarded at the end of the play. The base-repo password never lands on the client's disk; the init logic lives in one place (the client role) and is not duplicated on the custodian.

Whether restic-rest accepts usernames containing `/` under `--private-repos` is tracked as an open question below — the entire per-host / per-project scoping scheme here depends on it.

### Cross-host deduplication

The aggregated per-project offsite repo on Hetzner deduplicates across all hosts in the project. That is the main storage payoff of the cascade — twenty similar hosts do not produce twenty copies of shared content offsite, they produce one.

Whether the server's near repos also share storage on disk across hosts (via a hardlink pass on `/srv/restic`) is an open question, not assumed. Tracked below.

Cross-project dedup never happens — different chunker parameters, different repo keys, by design.

### Topology — cascade

Three tiers, three roles:

1. **Client → near.** Clients write directly into their per-host repo on `restic_server` via REST, using their own credentials. The server holds no decryption keys, ever.
2. **Custodian → offsite.** The `restic_custodian` host holds read-only credentials for every near repo in its project (for freshness/integrity checks) and write credentials for the per-project offsite repo on Hetzner. Scheduled `restic copy` pulls from each near repo into the aggregated offsite repo. Cryptographically verified, deduplicated, no passwords on the server.
3. **Custodian → airgapped (optional).** Periodic ritual producing a write-only offline copy plus key escrow — driven from the custodian, since it already holds every read key for the project. Media candidates with write-once-ish properties: BD-R (archival Blu-ray, M-DISC variant) offers good tamper-resistance and decade-scale shelf life, but a full project snapshot runs into many disks; LTO with WORM cartridges handles volume cleanly but adds hardware cost. Details and media choice in `airgapped-escrow.feature.md`.

The 3-2-1 rule — 3 copies, 2 media, 1 offsite — is satisfied at the *collection* level when both restic and borg run in parallel:

- 3 copies: source filesystem, restic-server (near), Hetzner Storage Box (offsite); borg adds its own independent copy.
- 2 media: restic's disk-based cascade is one medium; borg's separate repo format on different storage is the second.
- 1 offsite: Hetzner Storage Box.

Restic alone delivers 3 copies and 1 offsite but only 1 medium — borg is what closes the rule. The two paths are deliberately asymmetric: restic favors a self-hosted near target with custodian-driven offsite; borg favors hosted-as-a-service (Borgbase or direct-to-Hetzner), tracked in `backup-borg.feature.md`.

Clients never configure an offsite repo directly. If a client needs a snapshot in a second repo (not a copy of an existing snapshot — a genuinely distinct second snapshot), that is modeled as a second job with a different `repo:`. "Same snapshot in two places" is always the custodian's job via `restic copy`.

### Client — `restic_client`

Four top-level lists (one for repos, one per class), no conditional fields, no fan-out per job. Default ships a single full-filesystem host-level job against the near repo; service and vm lists are empty unless a service or hypervisor role populates them:

```yaml
restic_repos:
  near:
    url: "rest:https://backup.internal:8000/"
    username: "{{ ansible_hostname }}"
    auth_secret: { service: restic, name: near-auth }       # env-typed secret

restic_host_backup_jobs:               # Host-level (fs- class)
  - name: root                          # the default job
    paths: ["/"]
    excludes:                           # virtual + volatile filesystems
      - /proc
      - /sys
      - /dev
      - /run
      - /tmp
      - /var/tmp
      - /mnt
      - /media
    flags: []                           # no -x: descend into every mount under /
    repo: near
    retention: { keep_daily: 365 }
    schedule: daily

# restic_service_backup_jobs — Service-level (srv- class), empty by default.
# Shape (once the stream contract is finalized in service-export-import.pattern.md):
# restic_service_backup_jobs:
#   - name: pg
#     stream_command: "…"                 # how to read the service-published export; TBD
#     stdin_filename: pg-dumpall.sql
#     repo: near
#     retention: { keep_daily: 365 }
#     schedule: daily

# restic_vm_backup_jobs — Virtual Machine-level (vm- class). Not implemented
# in this ticket. The list, the class prefix, and a stub drop-in are reserved
# here so the future hypervisor-backup ticket slots in without reshaping
# anything. On delivery, this list would be populated by the hypervisor role.
```

The default `root` job backs up everything under `/` — no `-x`, so additional mounts (`/home` on a separate partition, `/var/lib/data`, etc.) are captured by the same job without needing extra inventory. The excludes list covers kernel virtual filesystems (`/proc`, `/sys`, `/dev`, `/run`), scratch space (`/tmp`, `/var/tmp`), and mountpoint shells (`/mnt`, `/media`). Filesystem-specific additions extend the list via role defaults, not inventory — notably btrfs needs `.snapshots` excluded so snapper-style snapshots do not balloon the backup, and a podman host typically excludes `/var/lib/containers/storage`.

The catch-all default is convenient but not worry-free. Restic has no depth cap and no recursion-loop detection — a bind-mount loop (e.g. `/mnt/backup` bind-mounted to `/`) would cause restic to descend forever, and the only mitigation is an explicit exclude for the offending path. Admins with pathological mount setups have to add their own excludes. An alternative — auto-detecting filesystems and excluding everything that is not the rootfs (essentially a managed `-x` equivalent with discoverability on top) — comes with its own challenges (what counts as "the" rootfs on a system with multiple data partitions you *do* want backed up?) and is deliberately not attempted here.

**Uniqueness assertion**: the role refuses to template if a `name` appears twice within the same list. Names across lists do not collide because the systemd instance and on-disk path are prefixed with the class (`fs-<name>`, `srv-<name>`, `vm-<name>`) — see the unit model and filesystem layout below. Caught early, not at systemd-enable time.

### Client filesystem layout

Everything the role owns lives under `/etc/restic/`. Secrets live under `/etc/secrets/` (the secrets role's territory). The auth file crosses the boundary via a symlink, the pattern the secrets role explicitly sanctions:

```
/etc/restic/
  repos/
    <repo>/
      restic-repo.env          # RESTIC_REPOSITORY, RESTIC_CACERT, base flags
      restic-repo.auth.env     # symlink → /etc/secrets/restic/<repo>-auth.env (RESTIC_PASSWORD=…)
  jobs/
    fs-<name>/               # Host-level job, systemd instance: restic-backup@fs-<name>
      restic-backup-job.env    # RESTIC_FLAGS with flags + paths
      repo                     # symlink → ../../repos/<selected-repo>/
    srv-<name>/              # Service-level job, systemd instance: restic-backup@srv-<name>
      restic-backup-job.env    # RESTIC_FLAGS with --stdin --stdin-filename …
      repo                     # symlink → ../../repos/<selected-repo>/
    # vm-<name>/             # Virtual-Machine-level job, systemd instance: restic-backup@vm-<name>
    #   (reserved; not materialized by this ticket — future hypervisor-backup ticket populates)
```

A repo's identity is the two files in its directory: `restic-repo.env` carries the public config (URL, cert path), `restic-repo.auth.env` carries the passphrase via symlink. Both are repo-level by design — the passphrase belongs to the repo, not the job, which is why it lives inside `repos/<repo>/` next to the URL.

Jobs reach their repo through one symlink — `jobs/<class>-<name>/repo` → `../../repos/<selected-repo>` — so the unit file loads both repo fragments via a stable nested path. Changing a job's repo is a single symlink flip.

The class prefix (`fs-` / `srv-` / `vm-`) on job directories matches the systemd instance name, so `%i` expansion in the unit finds the job directory at `/etc/restic/jobs/%i/`. The prefix is what lets class-wide systemd drop-ins work (see unit model below) and it rules out name collisions across the three lists.

### Backup user isolation

Backup jobs do not run as root by default. One system user — `restic-backup-client` — owns every backup unit on the host. The server-side REST daemon runs as its own separate user (`restic-backup-server`, see the Server section); a host that acts as both client and server keeps the two envelopes distinct.

**Host-level jobs** (`fs-` class) get `AmbientCapabilities=CAP_DAC_READ_SEARCH` and a matching bounding set, added via a class-wide drop-in on `restic-backup@fs-.service.d/`. That capability bypasses DAC read-and-traverse checks without granting write or any other privilege — restic gets the "read anything" power it needs for whole-filesystem backups, and nothing else. restic itself is not setuid and gets no extra capabilities.

**Service-level jobs** (`srv-` class) also run as `restic-backup-client`, without `CAP_DAC_READ_SEARCH` — they do not read paths directly; they consume a stream produced by a service-owned export endpoint. How the stream actually reaches restic (socket activation, FIFO, systemd `StandardInput=` plumbing) and how the producing service publishes it are out of scope here; that contract is defined in `service-export-import.pattern.md` when it lands. The `restic-backup@srv-.service.d/` drop-in carries the stdin wiring and nothing more.

**Virtual-Machine-level jobs** (`vm-` class) are **not implemented in this ticket** — only the namespace and a stub `restic-backup@vm-.service.d/class.conf` drop-in are shipped so the class prefix is reserved. When the future hypervisor-backup ticket lands, it fills the stub with whatever hypervisor-side access the export mechanism needs (typically `SupplementaryGroups=libvirt` or equivalent) and wires the actual VM export. Until then, no `vm-<name>.service` instances exist.

### Run-as-root escape hatch

When `CAP_DAC_READ_SEARCH` is not enough — MAC policies (SELinux, AppArmor) blocking read access, encrypted-at-rest filesystems with keys outside the `restic-backup-client` session keyring, niche kernel interfaces that ignore the capability — a per-job `run_as_root: true` override switches that job's unit to `User=root`.

Implementation: the role drops `/etc/systemd/system/restic-backup@<class>-<name>.service.d/run-as-root.conf` with `[Service]\nUser=root\nAmbientCapabilities=\nCapabilityBoundingSet=CAP_SYS_ADMIN CAP_DAC_READ_SEARCH ...` (reset to systemd's normal root defaults). Single template unit stays; the per-instance drop-in is the only place `root` appears.

This is an escape hatch, not a default. Each `run_as_root: true` job should carry a comment in inventory explaining *why* the capability path does not suffice. The docs-site page for this role calls out the pattern with a visible warning so new operators do not reach for it as a convenience.

### Unit model

One template, `restic-backup@.service`, covers every job. Instance names are `<class>-<name>` (`fs-root`, `srv-pg`, `vm-mail01`) so that systemd's dash-prefix drop-in rule reaches class-wide drop-ins automatically. ExecStart is env-driven; the per-job `.env` fills `RESTIC_FLAGS` with whatever the job needs; the class drop-in supplies `$RESTIC_CLASS_TAG`.

```ini
# /etc/systemd/system/restic-backup@.service
[Unit]
Description=restic backup job %i

[Service]
Type=oneshot
User=restic-backup-client
NoNewPrivileges=yes
EnvironmentFile=-/etc/restic/jobs/%i/repo/restic-repo.env
EnvironmentFile=-/etc/restic/jobs/%i/repo/restic-repo.auth.env
EnvironmentFile=-/etc/restic/jobs/%i/restic-backup-job.env
ExecStart=/usr/bin/restic backup --tag ${RESTIC_CLASS_TAG} $RESTIC_FLAGS
```

The base template grants *no* capabilities. Each class adds back only what it needs via a class-wide drop-in — additive security, never subtractive. Every class drop-in sets `Environment=RESTIC_CLASS_TAG=<class>`; a job without a class is a misconfiguration caught at template time.

Per-job `.env` examples:

- `fs-root` job: `RESTIC_FLAGS="-x /etc /var/lib"` (flags and paths).
- `srv-pg` job: `RESTIC_FLAGS="--stdin --stdin-filename pg-dumpall.sql"`.

**Class-wide drop-ins via dash-prefix** (systemd.unit(5), "Unit File Load Path"). For instance `restic-backup@fs-root.service`, systemd reads drop-ins from `restic-backup@fs-root.service.d/`, then `restic-backup@fs-.service.d/`, then `restic-backup@-.service.d/`. A single class-wide drop-in applies to every instance of that class without per-job templating.

Host-level jobs need read-anything access to back up a filesystem; the role ships a class-wide drop-in that grants it plus the class tag:

```ini
# /etc/systemd/system/restic-backup@fs-.service.d/class.conf
[Service]
AmbientCapabilities=CAP_DAC_READ_SEARCH
CapabilityBoundingSet=CAP_DAC_READ_SEARCH
Environment=RESTIC_CLASS_TAG=fs
```

Service-level jobs get no capabilities, just the tag and (when `service-export-import.pattern.md` lands) the stdin wiring:

```ini
# /etc/systemd/system/restic-backup@srv-.service.d/class.conf
[Service]
Environment=RESTIC_CLASS_TAG=srv
# stdin source wired here per service-export-import.pattern.md
# (socket activation / StandardInput= / FIFO)
```

Virtual-Machine-level jobs ship a **stub drop-in only** in this ticket — the namespace is reserved, no VM export mechanism is implemented. The future hypervisor-backup ticket fills in `SupplementaryGroups=libvirt` (or equivalent) plus the actual export wiring:

```ini
# /etc/systemd/system/restic-backup@vm-.service.d/class.conf
# STUB — reserved by this ticket. Filled in by the future hypervisor-backup ticket.
[Service]
Environment=RESTIC_CLASS_TAG=vm
# Future additions (not in this ticket):
#   SupplementaryGroups=libvirt
#   <VM export mechanism: snapshot + stream-to-restic>
```

**Classes as an extension axis.** The three classes shipped with this ticket (`fs-`, `srv-`, `vm-`) line up one-to-one with the three backup levels. The dash-prefix seam is not fixed, though — the same pattern absorbs any new job class that wants its own activation mechanism, timing, or privilege envelope. Examples that could land as future tickets:

- `git-<name>`: backup triggered from a bare-repo `post-receive` hook; class drop-in installs the hook and wires `ConditionPathExists=` on a flag file.
- `pg-wal-<name>`: backup triggered by a path watcher on the postgres WAL directory as new segments close; class drop-in declares a matching `.path` unit.
- `minutely-<name>`: backup with a tight retention window and a cadence timer wired directly by the class drop-in, overriding the timer battery. This would only work performantely with a small set of files.

None of the future examples land here — they are future features — but the framework is what it is: one template, one per-class drop-in, one pattern.

`EnvironmentFile=-` tolerates missing files (belt-and-braces for the auth chain on first run). Per-job customization — the `run_as_root: true` escape hatch — still lives in per-instance drop-ins under `/etc/systemd/system/restic-backup@<class>-<name>.service.d/*.conf`. The template stays canonical; overrides are additive files the role manages.

### Scheduling

Each job gets its own `restic-backup@<class>-<name>.timer` (a template timer alongside the service template) that directly activates its matching `restic-backup@<class>-<name>.service`. Timers are independent — systemd activates each on its own `OnCalendar=` schedule; there are no cadence groupings or target units in the activation path.

The role still offers a named-cadence vocabulary as a convenience — the "timer battery" is just the set of preset `OnCalendar=` values the admin can pick from:

| `schedule:` value | `OnCalendar=` expansion |
|---|---|
| `hourly` | `hourly` |
| `quarter-daily` | `*-*-* 00/6:00:00` |
| `daily` | `daily` |
| `weekly` | `weekly` |
| `monthly` | `monthly` |

Per job, `schedule: daily` becomes a drop-in on `restic-backup@<class>-<name>.timer.d/schedule.conf` with `OnCalendar=daily`. Admins who need a non-preset cadence set `schedule: "*-*-* 03:17:00"` (any valid `OnCalendar=` expression) and the role drops it in verbatim.

A convenience `restic-backup.target` exists for "run every backup now" and for `systemctl list-dependencies restic-backup.target` visibility: it `Wants=` every enabled `restic-backup@<class>-<name>.service`. It is not on any timer — admins start it by hand when they want a full pass outside the schedule.

**Run alerting.** The client's run check reads systemd unit state and exit code of the last `restic-backup@<class>-<name>.service` run and alerts on failed exit or overdue timer. Part of the checker framework, not a dedicated mechanism.

Jobs scheduled at the same `OnCalendar=` fire concurrently by default. If a site needs sequential execution (bounded bandwidth, avoid contention), that is expressed per-job with `After=` drop-ins between specific instances — not by the scheduling layer. Certain conventions like an After=restic-backup@fs-root.service might be put into the `srv-`-class backups to streamline the overall backup process.

### Retention

Retention is **off by default**. The role does not run `restic forget --prune` anywhere unless an admin explicitly opts in per repo; a backup that silently expires under an operator who did not realize a policy was active is the exact failure mode this default avoids.

When enabled, retention lives on the custodian, not on the client. The custodian is the one host that holds the repo password and already reaches every repo in the project, so `forget --prune` naturally belongs there. Per-repo opt-in means a site can, for instance, keep the near server vacating aggressively while the Hetzner offsite retains everything forever, or the other way round — each repo's retention is its own decision, set on the custodian. The on/off knob and policy shape live in the Custodian section below.

### Integrity

Integrity checks (`restic check`) for the **near repo** run on the client. The client already holds the near-repo password — moving the check elsewhere would duplicate credential-shuffling for no gain.

The role ships a per-repo timer `restic-check@<repo>.timer` that activates `restic-check@<repo>.service`, running `restic check` against the repo referenced by its instance name. Default cadence: weekly `--read-data-subset=10%` (rotating sample), monthly full `--read-data`. A matching check instance reads the last unit's state and exit code and alerts on failure or overdue — same pipeline as run alerting.

Integrity for the **aggregated per-project offsite repo** lives on the custodian — that is the only place the offsite credential exists. The custodian-side integrity schedule is described in the Custodian section below.

### Server — `restic_server`

Serves repos over REST with `--private-repos`, running as a dedicated `restic-backup-server` system user (distinct from the client-side `restic-backup-client` so hosts that run both roles keep separate privilege envelopes). Per-host user accounts authenticate via htpasswd; access scoped to `/<project>/<host>/*`. Restricted-deletion directory layout, rooted at `/srv/restic/`.

TLS via the certificates role (see `secrets.feature.md`), replacing the self-signed cert + `GODEBUG=x509ignoreCN=0` workaround. The same certificates role also provides the SSH host-key material used to pin the Hetzner Storage Box endpoint on the custodian side.

**The server does not hold client repo passwords.** It is a dumb REST endpoint that authenticates and serves pack files. Password-bearing work is split: the client does its own near-repo integrity checks (it already has the near password); the custodian does offsite replication, offsite integrity, freshness, and base-repo init (it already holds the custodian-scoped credentials). Either way, the server never gets to decrypt anything — a deliberate split, since the server is the largest and longest-lived host in the cascade.

**Narrow ongoing access from the controller.** After initial server bootstrap (a one-time privileged operation that installs restic-rest, writes the unit, creates directories), the only ongoing operation the controller needs on the server is appending or updating entries in `/srv/restic/.htpasswd` when projects add hosts or rotate credentials. The server role provisions a dedicated `restic-htpasswd-admin` user whose only privilege is writing that one file (enforced via file ownership plus a minimal sudoers rule for a narrow helper, or an SSH authorized-keys `command=` restriction — details in implementation). A compromised controller can add, remove, or modify htpasswd entries; it cannot delete or alter repo data. The "nuke every backup by SSHing to the server" path is not available.

Offsite replication is not the server's job — it is the custodian's. See the Custodian section below.

### Custodian — `restic_custodian`

Per-project host, net-new in this ticket, owning everything that needs off-client observation plus everything that needs a multi-repo credential scope. Deploys as its own role, one instance per project (site-wide custodians holding every project's keys concentrate trust unacceptably and are not supported). The custodian is observation + verification, not a failover or a backup for the backup — losing the custodian loses visibility, not data.

**Trust level.** The custodian is the highest-trust host in the cascade — it holds read access to every project repo's contents plus offsite write credentials. Placement is therefore a site-level infrastructure-admin decision (dedicated hardened VM, co-located with other top-trust observability, etc.) and explicitly out of scope for this ticket. Trust is highest in the chain, not absolute.

**Self-observation.** The custodian emits alerta heartbeats from each of its scheduled jobs (freshness, offsite replication, offsite integrity). A missing heartbeat surfaces the custodian itself being dead — no second mutual-check custodian needed.

**Scope: restic only.** The custodian pattern is specific to restic's self-hosted-near-with-server-side-offsite topology. Borg takes the opposite stance (direct-to-Borgbase, no central custodian to be a potential weak link) and is tracked in `backup-borg.feature.md`; the two paths are deliberately orthogonal.

**Responsibilities:**

- **Freshness check.** Calls `restic snapshots --latest 1 --json` per repo, alerts when the newest snapshot exceeds threshold (default `warn 25h / crit 49h`). Runs from a host that is not the client, so a dead client cannot mask a missing backup. Client-side run checks are the first line; the freshness check is the second line that catches the client being gone entirely.
- **Offsite replication.** `restic copy` from each near repo into the aggregated per-project offsite repo on Hetzner, on a schedule. The custodian is the only host that holds both near read credentials and offsite write credentials.
- **Offsite integrity.** `restic check` against the offsite repo. Default cadence matches the client side: weekly `--read-data-subset=10%`, monthly full `--read-data`. Alerts on failure or overdue. Near-repo integrity runs on the client (see Integrity section).
- **On-demand integrity trigger.** When a freshness check sees a suspicious gap or anomaly, the custodian fires an out-of-schedule `restic check` against the affected repo without waiting for the next scheduled run.
- **Repo init orchestration.** The custodian holds the `/<project>/.restic-base-repo` credential; the controller pulls it transiently to run `restic init --copy-chunker-params` on a client during bootstrap. See Project base repo and Credential provisioning sections for the detailed flow.
- **On-line half of DR key escrow.** Holds an independent read-only restic key for every project-member repo (see Credential scope below). The airgapped-escrow ritual (see `airgapped-escrow.feature.md`) is the off-line half — the custodian covers online recovery, the airgapped escrow covers the case where the custodian itself is gone.
- **Retention** (when enabled; off by default). `restic forget --prune` per repo, opt-in per repo. The custodian is the natural home because it already has every credential it needs; the client does not run prune.
- **Emergency access.** The custodian already holds read keys for every project repo, so it is the natural host for ad-hoc `restic restore` or `restic mount` when data needs to be recovered quickly — no credential scramble, no touching the original client. Any project repo is one command away on a host that already knows how to talk to it.

**Credential scope.** The custodian holds, per project: one read-only password per near repo in the project, one write password for the offsite repo, one base-repo password. Each near-repo credential is a **distinct restic key** added via `restic key add` at bootstrap — not a copy of the client's password. Decoupling the two keys means a client-side rotation does not require a matching custodian-side push: the client rotates its own key, the custodian keeps using its independent key. Custodian-side keys rotate on their own cadence as a manual administrator deployment step.

If you are really cautious you could even have two or more custodians each with it's own offsite backup maybe even managed by different administrators.

### Credential provisioning

For each `(host, repo)` pair the client talks to, the controller orchestrates a three-way handshake. Keeps the existing `perserver.yaml` pattern's shape (client-generates, controller-carries, server-ingests) but swaps the source and transport:

1. On the client, the secrets module provisions `restic/<repo>-auth.env` (env-typed secret with `RESTIC_PASSWORD=…`) with source `random`, and returns the value to the controller via `fetch: true`. No plaintext lives in inventory.
2. On the controller, the value is hashed with `password_hash('bcrypt')` (Jinja filter, no new primitive).
3. A delegated task on the matching `restic_server` ensures the `(username, bcrypt)` line exists in `/srv/restic/.htpasswd`. The delegation connects as the narrow-scope `restic-htpasswd-admin` user (see Server section), so the controller-side attack surface for this step is just "can modify .htpasswd" — not "can touch repo data". Idempotent on re-run — the line is keyed by username so rotation replaces in place.
4. Every task touching the password has `no_log: true`.

The custodian's read-only access to the near repo is provisioned as a **distinct restic key**, not a copy of the client's password. At bootstrap, the secrets role provisions an independent random password on the custodian for that repo; the controller fetches it transiently and a delegated task on the client runs `restic key add` (using the client's password) to install the custodian's key in the repo. Decoupled keys mean client rotations do not cascade into the custodian. Custodian-side keys rotate as a separate manual administrator step. Custodian-side write credentials for the offsite Hetzner repo are provisioned independently, same machinery.

Repo init runs on the client as a one-time bootstrap task. The controller fetches the project's base-repo password from the custodian (transient fact, `no_log`), then runs `restic init --copy-chunker-params --from-repo /<project>/.restic-base-repo` on the client with both the base-repo and the new repo's passwords in environment. The base-repo password is never written to disk on the client. See the project-base-repo section for the rationale and access model.

Rotation goes through the secrets-role rotation machinery. A post-rotate hook on the client re-keys the restic repo (`restic key add` new, verify, then `restic key remove` old) and triggers the controller-side htpasswd and custodian-side-credential updates via deferred plays. The bcrypt htpasswd entry is treated as a public-key derivative of the stored secret — computed on demand, not stored separately under `/etc/secrets/`.

The controller needs SSH access to client, server, and custodian during provisioning — the normal deploy assumption. The value transits via Ansible facts with `no_log`, never written to disk on the controller.

### Restore

Two paths, both admin-initiated, no systemd unit and no `.env` per job.

**1. File / raw restore via `restic-with-repo`.** A thin shell wrapper at `/usr/local/bin/restic-with-repo` that sources the repo's env fragments and execs restic with whatever args come next:

```sh
#!/bin/sh
# restic-with-repo <repo> <restic args...>
repo="$1"; shift
. /etc/restic/repos/"$repo"/restic-repo.env
. /etc/restic/repos/"$repo"/restic-repo.auth.env
exec restic "$@"
```

Typical use in an emergency:

```
restic-with-repo near snapshots
restic-with-repo near restore <snapshot-id> --target /tmp/recover
```

No surprise behaviour, no "restore to state X" declaration — just the env composition so admins do not need to hand-assemble `RESTIC_REPOSITORY` and `RESTIC_PASSWORD` at 3am.

**2. Service-level restore via the bidirectional stream contract.** For Service-level jobs (`srv-` class), restore is the inversion of export: the service role publishes an *import* socket alongside its export one, a backup-side helper streams the chosen snapshot from the repo into that socket, the service ingests. Both directions are defined by `service-export-import.pattern.md`. This is also the mechanism the blue-green DR drill uses — stream the latest snapshot from the offsite (or near) repo into a fresh green environment's import socket, smoke-test, promote or discard.

Continuous restore exercise via blue-green is **Service-level only** (path 2). A streamed service export restores cleanly into a fresh blue-green slot and can be smoke-tested there. Host-level (filesystem) restore targets a live system and does not map cleanly to blue-green; Virtual-Machine-level restore needs hypervisor cooperation. The nightly drill described in `blue-green-deployments.pattern.md` operates on Service-level jobs via path 2.


## Design notes

- Three job lists at the inventory layer (clearer schemas, no union typing on a `database_dump_command`-vs-`directories` discriminator) collapse to one systemd template at runtime — one timer battery, one alerting key, one place to edit — with per-class drop-ins carrying the class-specific behaviour.
- A job has exactly one repo. "Two repos means two snapshots" is a restic property: `restic backup` produces a new snapshot in each target repo, and those are semantically different snapshots, so they belong in different jobs. Multi-place-same-snapshot exists only via `restic copy`, which is the custodian's job.
- Cross-host dedup at the server's filesystem layer — a hardlink pass over `/srv/restic` that collapses identical files across different hosts' near repos — depends on whether restic writes byte-identical files to disk for identical inputs. Not empirically confirmed. If it works, it is a pure storage win on top of the offsite cascade; if not, the pass finds no matches and wastes cycles. An opt-in server flag only makes sense once measured.
- Sharing disk blocks across near repos (if hardlink dedup works) supposedly does not let one host's operator read another host's backups — the per-repo index stays gated by each repo's password; files on disk without that index are opaque content-addressed storage. Worth confirming during the same empirical check before relying on the disk-space saving.
- The bcrypt htpasswd entry is a public-key-shaped derivative of the stored password — computed, not stored. Matches the SSH authorized-keys pattern.
- Stream producers and consumers (import for restore) are not the backup role's concern. Service roles publish bidirectional stream endpoints; this role consumes the export direction and drives the import direction during restore. The interface lives in `service-export-import.pattern.md`.

## Open questions

- **restic-rest user scoping for path-like usernames**. The whole per-host / per-project isolation scheme assumes restic-rest under `--private-repos` accepts usernames that contain `/` and matches them against multi-segment paths — e.g. user `<project>/<host>` mapping to `/<project>/<host>/*`, user `<project>/.restic-base-repo` mapping to that exact path. If restic-rest only treats usernames as a single top-level directory component, the scheme has to flatten (usernames like `<project>-<host>`, paths like `/<project>-<host>`), or drop to project-level isolation with a shared credential per project. Verify against the current restic-rest version before building anything else around the current layout.
- **Server-side hardlink dedup across near repos**. Does restic write byte-identical files to disk when two repos in the same project back up identical inputs? Empirical test: init two repos with `--copy-chunker-params`, back up the same directory into each, diff the resulting files under `/srv/restic`. If identical files are frequent, ship an opt-in `server_hardlink_dedup: true` flag on the server role that schedules a `hardlink(1)` pass. If not, drop the idea. Same investigation should confirm the "sharing disk blocks does not share access" expectation before any flag flips on.
