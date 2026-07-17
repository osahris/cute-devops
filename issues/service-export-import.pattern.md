---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Service export / import streams

> **Pattern.** Cross-cutting convention. The deliverable is a consolidated definition that lives in `docs/patterns/` once the ticket is closed; service roles implementing export/import endpoints and consumers (backup, restore, blue-green) reference this document.

## Goal

Define a uniform **bidirectional** stream interface for services: produce a streaming export of their state (consumed by backup) and consume a streaming import of their state (produced by restore). Both directions share one contract, symmetric by design. Enables service-level backup and restore as a first-class level, alongside host-level and machine-level.

The name deliberately signals bidirectionality — export is the backup direction, import is the restore direction, and they are the same pipe run the two ways.

## Scope

- Per-service `export` and `import` commands with a standard interface: write to stdout / read from stdin.
- Compose exports from sub-components: database dumps, filesystem archives, app-format exports (e.g. Keycloak realm JSON, Grafana dashboards, Ansible inventory).
- Streamable format — no "dump to temp file, then archive" step, no pre-known size, no random-access requirement.
- Symmetric import path: a fresh service instance consumes the same stream and reaches the exported state.

## Design notes

Challenge: flexible-size stream archive format is not available off-the-shelf.

- `tar` requires a size for each member at the header, or a way to pad/patch after the fact — awkward for streaming.
- `cpio` has the same upfront-size problem.
- `zip` is not a streaming format on the write side.
- Custom framing (length-prefixed records, like MessagePack or a simple magic + length + payload loop) is simple, but introduces yet another format to maintain.
- Per-member sub-streams concatenated and demarcated — works, but brittle at the boundary.
- Single-stream-per-component, with the whole-service export being a sequence of (component → bytes) pairs on the consumer side — punts multiplexing to the wrapper and keeps each component's format natural (SQL for dbs, tar for filesystems, JSON for app state).

Restic and borg both support `--stdin` but produce a single "file" at a well-known name, so whatever multiplexing we do has to happen on our side, before the bytes hit restic/borg.

## Open questions

- Pick an archive format (custom or existing) or avoid archives entirely and do one backup per component per service?
- Import is harder than export — how do we verify a partial import (db restored, files restored, app migration not yet run) before switching blue→green?
- Do exports include secrets (API keys embedded in config), and if so, how are they handled across the boundary — stripped on export and re-sourced on import?
- Consistency: does the export assume a quiesced service (read-only mode from backup), or does it handle running services with its own snapshotting?
- Versioning: export from version N, import into N+1 — whose responsibility is the migration, the export side, the import side, or a standalone step?
