---
status: draft
---

# Backup monitor host

## Goal

A per-project monitor host that owns the off-client half of backup monitoring: freshness checks, integrity checks, and the on-line portion of the DR key escrow. Separate host so that a dead or compromised backup client cannot mask missing or corrupt backups.

Depends on: `backup-restic.feature.md`, `backup-borg.feature.md`.

## Scope

- Deploys as its own role (`backup_monitor`), typically one instance per project.
- Holds read-only credentials for every repo belonging to the project. Repo passwords are provisioned via the secrets role and stored locally; the monitor host is the on-line half of the DR-key-escrow pair (the airgapped ritual is the off-line half — see `airgapped-escrow.feature.md`).
- **Freshness checks**: calls `restic snapshots --latest 1 --json` (and the borg equivalent for borg repos) per job, alerts when the newest snapshot exceeds threshold. Defaults: `warn 25h / crit 49h`.
- **Integrity checks**: runs `restic check` per repo on its own timer battery (cadence mirrors the client side: weekly `--read-data-subset=10%`, monthly full `--read-data`). Alerts on failure or overdue.
- **On-demand integrity trigger**: when a freshness check sees a suspicious gap or anomaly, the monitor can fire an out-of-schedule `restic check` against the affected repo without waiting for the timer.
- **Optional cryptographic replication**: sites that want in-transit verification of the offsite copy run `restic copy` from the monitor host to the Hetzner Storage Box (instead of the default rsync on the backup server). The monitor already holds the credentials needed for both endpoints.

## Design notes

- The monitor is not a failover or a backup for the backup — it is strictly observation plus verification. Losing the monitor does not lose data, it loses visibility.
- One monitor per project matches the project-as-credential-boundary decision in `backup-restic.feature.md`. A monitor holds credentials only for its own project's repos.
- The monitor host also runs the backup-run freshness alerting that the client cannot do when it is down. Client-side run checks stay on the client (systemd state of the last backup unit); the monitor is the second-line check that catches the client being gone entirely.
- Timer battery on the monitor mirrors the client-side pattern (hourly / daily / weekly / monthly) so cadences are expressed consistently across roles.

## Open questions

- **Per-project or per-site?** One monitor per project keeps credential blast radius small. One per site (holding all projects' read-only credentials) is cheaper but concentrates trust. Default: per-project; site-level is an override.
- **Borg coverage**: restic and borg have different APIs for snapshot listing and integrity; is this one role with two backends, or two sibling roles (`restic_monitor`, `borg_monitor`)?
- **Self-monitoring**: who watches the monitor? A mutual check between two monitors in neighboring projects, or is a single line of defense acceptable?
- **Credential rotation**: when a client rotates its repo password, the monitor's read-only credential rotates too. Does the monitor pull the new value on the next run, or does the backup role's post-rotate hook push to it?
- **Placement**: is the monitor typically a dedicated small VM, or can it co-locate with other observability services (alertmanager, grafana) on an existing infra host?
