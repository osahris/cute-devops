---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Airgapped escrow — the 3-2-1-1-0 upgrade

## Goal

An optional additional layer on top of the standard backup cascade: an airgapped copy held offline by the admin, combined with an offline copy of the repo keys. Answers two concerns with one ritual — ransomware resistance (compromise of online infrastructure cannot wipe the backups) and DR key escrow (the admin can decrypt without touching any online system).

Depends on: `backup-restic.feature.md`, `backup-borg.feature.md`.

## Scope

- Helper scripts generated on each backup server (`airgap-pull`, `airgap-verify`, `airgap-list`) tailored to that server's repos.
- Workstation-side ritual the admin runs on a trusted machine: pull encrypted pack files (no decryption keys needed for the pull itself), verify integrity, burn to cold storage, store the key material alongside.
- The same scripts work for both restic and borg repos — underlying media (LTO, offline HDD, optical) is the admin's choice.
- Monitoring check: "time since last successful airgap ritual", overdue alerts. Completion of the ritual is the proof-of-offline-copy.

Not on by default. Opt-in, documented, scripted.

## Design notes

- Pack files in restic and segment files in borg are both content-addressed and immutable — safe to copy at the filesystem level without understanding the repo cryptography.
- The admin's trusted workstation holds the decryption keys (via the secrets role's `local-pass` store). Pulling from the near server needs only read access to the repo files, not the key.
- The ritual is the single load-bearing escrow event: if it has not happened recently, we have neither an airgapped copy nor a verified-offline key. One alert covers both failure modes.

## Open questions

- **Cadence default**: quarterly feels right for most sites; high-stakes sites may want monthly. What is the sensible default, and what gets alerted at what age?
- **Media target**: do the scripts assume a single medium (default: an offline-held HDD mounted on pull), or accept `--target` and let the admin point at whatever cold storage they use? A `--target dir` plus companion write-to-media instructions per supported medium is probably the right shape.
- **Verification depth**: `restic check` (metadata), `restic check --read-data` (full chunk verification, slow), or a sampled subset by default?
- **Scope per ritual**: pull everything every time, or incremental pulls with a full pull on a longer cadence?
- **Key material format**: printed paper (QR + human-readable), YubiKey-wrapped, offline-HSM — or all three optional?
- **Multiple admins / separation-of-duties**: does the ritual require one admin, or is it cleaner with two (one holds media, one holds keys)?
