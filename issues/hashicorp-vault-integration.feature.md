---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# HashiCorp Vault integration

## Goal

Add HashiCorp Vault (and similar centralized secret managers) as a source — and possibly a store — for the secrets role. Preserves the "sane defaults without a central server" posture by keeping Vault strictly opt-in: the secrets role works without it; Vault is available when a site chooses to run one.

Depends on: [secrets.feature.md](secrets.feature.md).

## Scope

- **Source**: read a named path from a Vault server at play time and hand the value to the secrets role.
- **Store**: optionally push generated secrets back to Vault for central visibility (symmetric to `pass` / `local-pass`).
- Auth methods: Vault token, AppRole, or Kubernetes auth (for hosts that sit inside a cluster).
- Token lifecycle: short-lived, retrieved per-run, not written to disk.

## Design notes

Consumes the same source/store interface as the rest of the secrets role — adding Vault does not require changes to consumers. The Vault client dependency stays optional: pulled in only when a Vault source or store is referenced in the inventory. Mount-path convention: `secret/mkbrechtel.devops/<service>/<name>` (configurable).

## Open questions

- Which auth method is primary? AppRole is the most neutral; token is simplest for a single admin; Kubernetes auth only works inside a cluster.
- KV v2 specifically, or abstract over v1/v2?
- Relationship to `pass`: is Vault the team-scale superset and `pass` the single-admin case, or do they coexist at team scale for different workflows?
- Rotation: does Vault's own rotation (dynamic secrets, lease renewal) replace the secrets role's rotation loop, or are they complementary?
