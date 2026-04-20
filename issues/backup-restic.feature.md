---
status: draft
---

# Backup — restic path

## Goal

Working `restic_client`, `restic_server`, and `restic_monitor` roles as one of two backup paths (alongside borg) to satisfy 3-2-1. The three roles are the load-bearing triad: the client produces snapshots, the server holds the near repos password-free, the monitor carries read-only keys and does all the work that needs keys (freshness checks, integrity checks, offsite replication).

Per-host isolation on the server side, two backup levels on the client side, monitored end-to-end, single near-target for clients with offsite cascade handled by the monitor.

## Status quo

The collection already ships working `restic_client` and `restic_server` roles inherited from an earlier project. This ticket defines the target shape. Implementation is a rework that keeps what works (systemd timer + oneshot pattern, client-generates-then-delegates htpasswd) and swaps out the parts that do not scale (single-server URL as the only config knob, inline plaintext `key:` in inventory, two flavors of job in one list, no monitoring role at all, no offsite cascade, no integrity checks). The new `restic_monitor` role is net-new and is specified in `backup-monitor-host.feature.md`.

## Scope

### Backup levels

- **host** — filesystem paths on the guest.
- **service** — streamed export from a running service (database dump, Keycloak realm export, Grafana dashboards, virtual-machine export, disk image) via restic's `--stdin` mode. Depends on `streamable-service-exports.feature.md`.

Machine-level backups (VM disk volumes) are not a distinct level at this role — they are produced at the hypervisor layer and appear to this role as service-level streamed exports from the hypervisor's point of view. See "Hypervisor backups" below.

### Default repo model

A host belongs primarily to one project. Central shared infrastructure lives in a project conventionally named `<org>-infra`; separate backup infrastructure lives in a project conventionally named `<org>-backup`. The canonical definitions of *project*, *service*, *host* and the conventional naming patterns live in `project-service-terminology.feature.md`; this ticket assumes them.

Near repo path on the server: `/<project>/<host>`. One repo per host — all jobs on that host write snapshots into the same repo, separated by `--tag` and snapshot paths, which is restic's native pattern. Project is an organizational prefix, not an enforced trust boundary. Enforcement is at the host level: one restic-server user per host, one access-allowed path prefix per user, enforced by `--private-repos`. Hosts within a project are isolated from each other's backups.

Offsite repo layout is **per-project, aggregated**: `/<project>` on Hetzner Storage Box. All hosts in a project feed into one offsite repo via `restic copy` from the monitor (details in the cascade section). This unlocks cross-host dedup at the offsite layer — similar VM images, shared OS packages, the same config files across a fleet — because restic's content-defined chunking dedups identical chunks across snapshots within a repo.

Blue-green deployments within a project — and the use of restore-into-green as a DR drill — are tracked separately in `blue-green-deployments.feature.md`.

### Chunker parameter coordination

Restic's content-defined chunking uses a per-repo rolling-hash polynomial picked at `restic init`. Two repos with different polynomials cannot dedup against each other — `restic copy` still works, but every chunk ends up re-stored on the destination, losing the point of aggregation.

Per-project chunker alignment uses a dedicated **project base repo** as the polynomial anchor:

1. Project bootstrap creates `/<project>/base` on the backup server — an empty repo whose sole purpose is holding the project's chunker polynomial. It never receives real snapshots.
2. Every other repo in the project — near repos on the server, `<hypervisor>-vms` repos, the aggregated offsite repo on Hetzner — is initialized with `restic init --copy-chunker-params --from-repo /<project>/base`. Same polynomial, same chunk size bounds, dedup works across the whole project.
3. Init is idempotent: on re-run, the monitor (which owns this ritual — see `backup-monitor-host.feature.md`) checks whether a repo already exists; if yes, it verifies the chunker polynomial matches the project base and warns if drift has occurred.

Using a dedicated base repo rather than "whichever repo was first" removes ambiguity about which polynomial a project uses and makes the bootstrap ritual explicit: create `/<project>/base`, then every subsequent repo is derived. The base repo is tiny (just metadata) and can be read-shared project-wide so any future init from any host works without project-admin credentials.

This same mechanism resolves the hypervisor-VM dedup case: all `<project>/<hypervisor>-vms` repos for a given project share the project's polynomial via the base repo, so identical OS images across VMs dedup into a common chunk pool on the offsite side.

### Cross-project deduplication

Two layers of dedup work with this model without violating any security boundary:

- **In-repo dedup (free)**: restic chunk-dedups automatically within a repo. The per-project offsite repo already benefits across all hosts in the project, courtesy of the chunker-parameter alignment above.
- **Cross-repo dedup at the filesystem layer (opt-in)**: pack files are content-addressed (`data/<hash>/<hash>`) and immutable. On the server's `/srv/backupstorage`, a scheduled `hardlink(1)` (or `jdupes --link-hard`) pass replaces byte-identical pack files across different repos with hard-linked copies. No information leak: if two repos hold the same content chunk, each repo's owner could already read it; hard-linking only saves disk. Opt-in via a flag on the server role; safe default is off because it imposes a storage-layout commitment (no reflink-incompatible filesystems).

Note that hardlink dedup is only meaningful across repos that already share chunker parameters — i.e. within a project. Cross-project hardlinking is possible in theory but pointless in practice because the polynomials differ.

### Topology — cascade

Three tiers, three roles:

1. **Client → near.** Clients write directly into their per-host repo on `restic_server` via REST, using their own credentials. The server holds no decryption keys, ever.
2. **Monitor → offsite.** The `restic_monitor` host holds read-only credentials for every near repo in its project (for freshness/integrity checks) and write credentials for the per-project offsite repo on Hetzner. Scheduled `restic copy` pulls from each near repo into the aggregated offsite repo. Cryptographically verified, deduplicated, no passwords on the server.
3. **Admin → airgapped (optional).** Quarterly ritual, off-line key escrow, see `airgapped-escrow.feature.md`.

The 3-2-1 rule — 3 copies, 2 media, 1 offsite — is satisfied at the *collection* level when both restic and borg run in parallel:

- 3 copies: source filesystem, restic-server (near), Hetzner Storage Box (offsite); borg adds its own independent copy.
- 2 media: restic's disk-based cascade is one medium; borg's separate repo format on different storage is the second.
- 1 offsite: Hetzner Storage Box.

Restic alone delivers 3 copies and 1 offsite but only 1 medium — borg is what closes the rule. The two paths are deliberately asymmetric: restic favors a self-hosted near target with monitor-driven offsite; borg favors hosted-as-a-service (Borgbase or direct-to-Hetzner), tracked in `backup-borg.feature.md`.

Clients never configure an offsite repo directly. If a client needs a snapshot in a second repo (not a copy of an existing snapshot — a genuinely distinct second snapshot), that is modeled as a second job with a different `repo:`. "Same snapshot in two places" is always the monitor's job via `restic copy`.

### Client — `restic_client`

Three top-level lists, no conditional fields, no fan-out per job. Default ships a single full-filesystem job against the near repo and no stream jobs:

```yaml
restic_repos:
  near:
    url: "rest:https://backup.internal:8000/"
    username: "{{ ansible_hostname }}"
    auth_secret: { service: restic, name: near-auth }       # env-typed secret

restic_file_backup_jobs:
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
    flags: ["-x"]                       # stay on the root filesystem; other mounts need their own job
    repo: near
    retention: { keep_daily: 365 }
    schedule: daily

# restic_stream_backup_jobs is empty by default. Example for reference:
# restic_stream_backup_jobs:
#   - name: pg
#     stream_command: "restic-stream-connect pg"
#     stdin_filename: pg-dumpall.sql
#     repo: near
#     retention: { keep_daily: 365 }
#     schedule: daily
```

The default `root` job backs up everything on the root filesystem, with `-x` preventing descent into other mounts (`/var/lib/data`, `/home` on a separate partition, etc. — those go in their own jobs). The excludes list covers kernel virtual filesystems (`/proc`, `/sys`, `/dev`, `/run`), scratch space (`/tmp`, `/var/tmp`), and mountpoint shells (`/mnt`, `/media`). Distribution-specific additions (say `/var/lib/containers/storage` for a podman host) extend the list via role defaults, not inventory.

**Uniqueness assertion**: the role refuses to template if any `name` appears in both lists, or appears twice in either list. One flat namespace for jobs. Caught early, not at systemd-enable time.

### Client filesystem layout

Everything the role owns lives under `/etc/restic/`. Secrets live under `/etc/secrets/` (the secrets role's territory). The auth file crosses the boundary via a symlink, the pattern the secrets role explicitly sanctions:

```
/etc/restic/
  repos/
    <repo>/
      restic-repo.env          # RESTIC_REPOSITORY, RESTIC_CACERT, base flags
      restic-repo.auth.env     # symlink → /etc/secrets/restic/<repo>-auth.env (RESTIC_PASSWORD=…)
  jobs/
    <job>/
      restic-backup-job.env    # job-specific: RESTIC_DIRECTORIES or STREAM_COMMAND + STDIN_FILENAME, flag overrides
      repo                     # symlink → ../../repos/<selected-repo>/
  restore/
    <job>.env                  # admin-authored restore invocations
```

A repo's identity is the two files in its directory: `restic-repo.env` carries the public config (URL, cert path), `restic-repo.auth.env` carries the passphrase via symlink. Both are repo-level by design — the passphrase belongs to the repo, not the job, which is why it lives inside `repos/<repo>/` next to the URL.

Jobs reach their repo through one symlink — `jobs/<job>/repo` → `repos/<selected-repo>/` — so the unit file loads both repo fragments via a stable nested path. Changing a job's repo is a single symlink flip.

One flat `jobs/` dir holds both file and stream jobs (uniqueness assertion makes that safe). The systemd template decides which ExecStart fires based on which list the job came from.

### Backup user isolation

Backup jobs do not run as root by default. One system user — `restic-backup` — owns every backup unit on the host.

**File-mode jobs** get `AmbientCapabilities=CAP_DAC_READ_SEARCH` and a matching bounding set. That capability bypasses DAC read-and-traverse checks without granting write or any other privilege — restic gets the "read anything" power it needs for whole-filesystem backups, and nothing else. restic itself is not setuid and gets no extra capabilities.

**Stream-mode jobs** run `$STREAM_COMMAND` as the same `restic-backup` user, piped into `restic backup --stdin`. The stream command does *not* produce the export itself — that would require per-service privilege, which `restic-backup` does not have. It reads an already-exposed export endpoint published by a service-specific role.

The role ships a helper `restic-stream-connect <service>` that canonicalizes the common case: connect to a Unix socket at a known path, stream stdin. Convention:

- **Shared directory**: `/run/restic-stream/`, mode `0710`, owned `root:restic-backup`. Group-traversable but not group-readable — the backup user can enter the dir and connect to a named socket but cannot enumerate what else lives there.
- **Per-service socket**: `/run/restic-stream/<service>.sock`, mode `0660`, owned `<service-user>:restic-backup`. The service writes, the backup user reads; no third party can connect.
- **Helper** (`/usr/local/bin/restic-stream-connect`): `exec socat - UNIX-CONNECT:/run/restic-stream/$1.sock`. That's the whole implementation. Typical `stream_command: "restic-stream-connect pg"`.

Restricting the connect permission to the `restic-backup` group is load-bearing: anyone who can connect to the socket receives the full export dump, so the socket's ACL *is* the export's access control. Socket activation on the service side plus group-gated connect on the consumer side means the export dump exists only while the backup is running, and only the backup user can trigger it.

Inventory sets `stream_command`, so write access to inventory implies read access to whatever the command fetches — but the backup user already has read-all rights, so this is not a new privilege surface. The narrower pattern (helper + known socket path) is a defense-in-depth win over running arbitrary dump commands as root.

Producer-side details — how postgres actually publishes `/run/restic-stream/pg.sock`, how keycloak emits a realm dump — belong to service-specific roles and to `streamable-service-exports.feature.md`. This role consumes; it does not produce.

### Run-as-root escape hatch

When `CAP_DAC_READ_SEARCH` is not enough — MAC policies (SELinux, AppArmor) blocking read access, encrypted-at-rest filesystems with keys outside the `restic-backup` session keyring, niche kernel interfaces that ignore the capability — a per-job `run_as_root: true` override switches that job's unit to `User=root`.

Implementation: the role drops `/etc/systemd/system/restic-backup@<job>.service.d/run-as-root.conf` with `[Service]\nUser=root\nAmbientCapabilities=\nCapabilityBoundingSet=CAP_SYS_ADMIN CAP_DAC_READ_SEARCH ...` (reset to systemd's normal root defaults). Single template unit stays; the per-job drop-in is the only place `root` appears.

This is an escape hatch, not a default. Each `run_as_root: true` job should carry a comment in inventory explaining *why* the capability path does not suffice. The docs-site page for this role calls out the pattern with a visible warning so new operators do not reach for it as a convenience.

### Unit model

Two sibling templates, no wrapper script. Both share the EnvironmentFile layering through the per-job `repo` symlink; they differ only in ExecStart because file-mode is argv-clean and stream-mode requires a shell for the pipe.

```ini
# /etc/systemd/system/restic-backup@.service — file mode
[Unit]
Description=restic file backup job %i

[Service]
Type=oneshot
User=restic-backup
AmbientCapabilities=CAP_DAC_READ_SEARCH
CapabilityBoundingSet=CAP_DAC_READ_SEARCH
NoNewPrivileges=yes
EnvironmentFile=-/etc/restic/jobs/%i/repo/restic-repo.env
EnvironmentFile=-/etc/restic/jobs/%i/repo/restic-repo.auth.env
EnvironmentFile=-/etc/restic/jobs/%i/restic-backup-job.env
ExecStart=/usr/bin/restic backup $RESTIC_FLAGS $RESTIC_DIRECTORIES
```

```ini
# /etc/systemd/system/restic-stream-backup@.service — stream mode
[Unit]
Description=restic stream backup job %i

[Service]
Type=oneshot
User=restic-backup
NoNewPrivileges=yes
EnvironmentFile=-/etc/restic/jobs/%i/repo/restic-repo.env
EnvironmentFile=-/etc/restic/jobs/%i/repo/restic-repo.auth.env
EnvironmentFile=-/etc/restic/jobs/%i/restic-backup-job.env
ExecStart=/bin/sh -c 'exec $STREAM_COMMAND | /usr/bin/restic backup $RESTIC_FLAGS --stdin --stdin-filename "$STDIN_FILENAME"'
```

The role enables the right template per job: a file-list entry wires `restic-backup@<job>.service`; a stream-list entry wires `restic-stream-backup@<job>.service`. `EnvironmentFile=-` tolerates missing files, which matters for the auth chain during the first run (the symlink is created before the timer fires, but the `-` is belt-and-braces).

Stream-mode jobs skip `CAP_DAC_READ_SEARCH` — they never read paths directly, they only read the socket/endpoint the service exposes. Narrower capability set matches the narrower job.

Per-job customization (including the `run_as_root: true` escape hatch) is delivered via systemd drop-ins at `/etc/systemd/system/restic-backup@<job>.service.d/*.conf`. The templates stay canonical; overrides are additive files the role manages.

### Scheduling — timer battery

The role ships a fixed set of named cadence timers the site can opt into — the "timer battery" pattern already used by the checker and deploy roles. Cadences cover the range most backups ever want: `hourly`, `quarter-daily`, `daily`, `weekly`, `monthly`. Each timer fires a per-cadence target that pulls in the jobs opted into that cadence via `Wants=`.

Per job the admin sets `schedule:` to one of those names. The role wires `restic-backup-<cadence>.target` ⊂ `Wants=` into `restic-backup@<job>.service` or `restic-stream-backup@<job>.service` accordingly. Jobs within a cadence run sequentially (explicit `After=` between them) so failure is attributable per job and total bandwidth stays bounded.

Retention (`restic forget --prune`) runs per repo, not per job — snapshots share the repo, and `forget` applies policy across the whole repo's tags. Default is `keep_daily: 365`, and the same policy applies to the near repo and the aggregated offsite repo. Generous cutoffs plus this symmetry mean replication timing drift cannot strand a snapshot: anything `forget` would remove has long since been copied by the monitor. Scheduled via the same timer battery.

Integrity checks do **not** run on the client. They run on the monitor — see `backup-monitor-host.feature.md`.

### Server — `restic_server`

Serves repos over REST with `--private-repos`. Per-host user accounts, access scoped to `/<project>/<host>/*`. Existing `restic-service` user and restricted-deletion `backupstorage` directory layout stays.

TLS via the certificates role (see `secrets.feature.md`), replacing the self-signed cert + `GODEBUG=x509ignoreCN=0` workaround. The same certificates role also provides the SSH host-key material used to pin the Hetzner Storage Box endpoint on the monitor side.

**The server does not hold client repo passwords.** It is a dumb REST endpoint that authenticates and serves pack files. All password-bearing work (replication, integrity check, freshness check, repo init with chunker-param coordination) happens on the monitor. This is a deliberate split — the largest and longest-lived host in the cascade is the one that never gets to decrypt anything.

Offsite replication is not the server's job — it is the monitor's. See `backup-monitor-host.feature.md`.

Optional storage optimization: a scheduled `hardlink(1)` pass deduplicates byte-identical pack files across repos on the server's `/srv/backupstorage`. Off by default; opt-in at the server role level. Safe because pack files are content-addressed and immutable; security-neutral because only clients that already had access to a chunk's content via their own repo benefit.

### Credential provisioning

For each `(host, repo)` pair the client talks to, the controller orchestrates a three-way handshake. Keeps the existing `perserver.yaml` pattern's shape (client-generates, controller-carries, server-ingests) but swaps the source and transport:

1. On the client, the secrets module provisions `restic/<repo>-auth.env` (env-typed secret with `RESTIC_PASSWORD=…`) with source `random`, and returns the value to the controller via `fetch: true`. No plaintext lives in inventory.
2. On the controller, the value is hashed with `password_hash('bcrypt')` (Jinja filter, no new primitive).
3. A delegated task on the matching `restic_server` ensures the `(username, bcrypt)` line exists in `/srv/backupstorage/.htpasswd`. Idempotent on re-run — the line is keyed by username so rotation replaces in place.
4. Every task touching the password has `no_log: true`.

The monitor receives its read-only copy of the same password through the same mechanism: the secrets module fetches the client's `restic/<repo>-auth.env` to the controller, and a delegated task installs it as the monitor's read-only credential for that repo. Monitor-side write credentials for the offsite Hetzner repo are provisioned independently.

Repo init is a monitor responsibility — see the chunker-parameter coordination section. The client does *not* init its own repo; the monitor inits the near repo with the project's polynomial (via `--copy-chunker-params --from-repo /<project>/base`), and the client only writes snapshots into an already-existing repo.

Rotation goes through the secrets-role rotation machinery. A post-rotate hook on the client re-keys the restic repo (`restic key add` new, verify, then `restic key remove` old) and triggers the controller-side htpasswd and monitor-side-credential updates via deferred plays. The bcrypt htpasswd entry is treated as a public-key derivative of the stored secret — computed on demand, not stored separately under `/etc/secrets/`.

The controller needs SSH access to client, server, and monitor during provisioning — the normal deploy assumption. The value transits via Ansible facts with `no_log`, never written to disk on the controller.

### Monitoring

Client-side:

- **Run check** — reads systemd unit state and exit code of the last `restic-backup@<job>.service` or `restic-stream-backup@<job>.service` run. Alerts on failed exit or overdue timer.

Everything else (freshness check, integrity check, on-demand integrity trigger, offsite replication, on-line key escrow) lives on the monitor host — see `backup-monitor-host.feature.md`.

### Restore

Restore is a systemd unit the admin triggers, not a CLI tool. `restic-restore@<job>.service` reads `/etc/restic/restore/<job>.env` (target snapshot ID, destination path, include/exclude filters), resolves the repo + password via the job's existing `repo/` symlink, and runs `restic restore`. Admin authors the `.env`, triggers the unit, watches the log.

A small shell helper (`restic-snapshots <job>`) lists available snapshots against the job's configured repo — convenience, not a dependency. Ships as a plain script under `/usr/local/bin/` since the surface is tiny.

No declarative "restore to state X" — too easy to footgun in an emergency.

Continuous restore exercise via blue-green is **service-level only**. A streamed service export restores cleanly into a fresh blue-green slot and can be smoke-tested. Host-level (filesystem) restore targets a live system and does not map cleanly to blue-green; machine-level (VM) restore needs hypervisor cooperation. The nightly drill described in `blue-green-deployments.feature.md` operates on stream-mode jobs.

## Design notes

- The three-role triad — client, server, monitor — splits duties along a password-access boundary. Server never decrypts, client only reads its own repo, monitor holds read-only project-wide keys plus offsite write credentials. This makes the server the simplest component despite being the longest-lived.
- The three-list client config shape (`restic_repos`, `restic_file_backup_jobs`, `restic_stream_backup_jobs`) replaces the single `restic_client_backup_directives` list with its `database_dump_command`-vs-`directories` discriminator. Host-level and service-level jobs are genuinely different units of work, wired through genuinely different systemd templates.
- A job has exactly one repo. "Two repos means two snapshots" is a restic property: `restic backup` produces a new snapshot in each target repo, and those are semantically different snapshots, so they belong in different jobs. Multi-place-same-snapshot exists only via `restic copy`, which is the monitor's job.
- Project-level chunker-parameter alignment (a dedicated `/<project>/base` repo; every other repo init'd with `--copy-chunker-params --from-repo /<project>/base`) is what makes the aggregated per-project offsite repo actually dedup. Skipping this step silently turns the offsite repo into a fan-in of unrelated chunk pools — correct but wasteful.
- Two sibling templates, no wrapper script. File-mode ExecStart is pure argv (env-driven, no shell); stream-mode ExecStart needs a shell only because a pipe is shell syntax. Making the split explicit in systemd is cleaner than dispatching in a script on disk.
- Composition via layered `EnvironmentFile=` + a single per-job `repo/` symlink directory lets each fragment have a single owner. The job's repo choice is one symlink; changing the repo is one `ansible.builtin.file` task.
- Backup does not run as root. `CAP_DAC_READ_SEARCH` covers file-mode; stream-mode needs no capability because it reads a service-published endpoint. The backup user's privilege envelope is "read any file" — not "run any command as any user".
- Stream producers are not the backup role's concern. Service roles publish export endpoints; this role connects to them. The interface is a Unix socket or equivalent on the host filesystem. Details in `streamable-service-exports.feature.md`.
- The per-project aggregated offsite repo enables cross-host dedup at the offsite layer without violating the per-host isolation at the near layer. `restic copy` respects chunking, so the wire cost is proportional to *new* content, not to the near repos' sizes.
- Server-side hardlink dedup is a storage optimization, not a security boundary. Safe because pack files are immutable and identity of content equals identity of access (if two users have a chunk, either one could already read that chunk).
- The bcrypt htpasswd entry is a public-key-shaped derivative of the stored password — computed, not stored. Matches the SSH authorized-keys pattern.
- Retention symmetry (same policy on near and offsite) plus a generous `keep_daily: 365` default sidesteps the cascade-timing-drift problem without explicit `After=`/`Before=` ordering between replication and prune.
- Inventory shape is the public API of the role. Implementation tracks the shape, not the other way around.

## Open questions

None open at the moment — flip back to draft if new ones surface during implementation.
