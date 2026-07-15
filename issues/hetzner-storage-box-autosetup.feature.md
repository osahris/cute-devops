---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Hetzner Storage Box auto-setup

## Goal

A shared role that provisions sub-accounts and directories on a Hetzner Storage Box, consumed by backup roles (restic, borg, future) that target a Storage Box. Keeps Storage Box setup out of each individual backup role.

## Scope

- Create an SFTP sub-account per project on the configured Storage Box.
- Apply quota and permission settings.
- Generate the SSH keypair for the backup client via the secrets role, register the public key with the sub-account.
- Expose the resulting connection details (host, username, jail path) to consuming roles as facts; the private key is stored via the secrets role.

## Design notes

- Storage Box sub-account management uses Hetzner's web UI + a limited API. Need to check whether the API surface is sufficient to fully automate sub-account creation, or whether we document the manual first-time step and automate the rest.
- The role is consumed by backup roles but is not backup-specific — any role that wants SFTP space on a Storage Box can use it.
- Matches the pattern used by the secrets role: the consumer requests `{ service: <proj>, name: storage-box }` and the role provisions on first use, returns credentials, idempotent on re-run.

## Open questions

- Hetzner API coverage: is sub-account creation fully scriptable today, or is there a manual step gate?
- How do we handle the credential-recovery case where the admin rotates their Storage Box master password — does that break sub-accounts or require re-auth?
- One Storage Box shared across multiple projects (sub-accounts within one box), or one Storage Box per project? The former is cheaper, the latter isolates the blast radius of a box compromise.
- Directory layout inside the sub-account — `/restic/`, `/borg/`, `/<service>/`, or free-form declared by the consumer?
