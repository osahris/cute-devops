---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# systemd credentials as a secret store

## Goal

Prototype `systemd-credentials` as a store backend for the secrets role. With `LoadCredential=` and `systemd-creds` encrypt-at-rest, services receive secrets at start time without an unencrypted copy ever sitting on disk — a meaningful hardening step over plain files under `/etc/secrets/`.

Open research question: does this model fit the role's source/store abstraction cleanly, and is the operational complexity worth the win?

Depends on: [secrets.feature.md](secrets.feature.md).

## Scope

- Store backend `systemd-creds` that writes encrypted credential files and wires services to load them via `LoadCredential=`.
- Key handling: use TPM-backed keys where available; fall back to host key file otherwise.
- Rotation semantics: re-encrypt the credential, restart the consuming service (or use a hook, see rotation-hooks ticket).
- Consumer integration: template fragment or drop-in that adds `LoadCredential=` to the unit, and references to the credential from within the service (env, file).

## Design notes

systemd-creds supports several ingest paths (file, env, fixed). The encrypted-at-rest case (`systemd-creds encrypt ... my.cred`) is what we care about — the cred is only readable during service start by the kernel-delivered key.

Compatibility with existing consumers is the main unknown: services that read secrets from `/etc/secrets/…` at any time don't map neatly to "only at service start". It may only fit a subset of consumers initially.

## Open questions

- TPM key availability on the target hosts we actually run — fallback behavior when no TPM.
- Revocation / rotation: how do we rotate the encryption key without breaking existing creds?
- Rendering: how do we represent a systemd-creds store in the role invocation so consumers can declare "I want this via LoadCredential="?
- Is this better expressed as a *consumer mode* (how the secret gets to the service) rather than a *store* (where it lives)?
