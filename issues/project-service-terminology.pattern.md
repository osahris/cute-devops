---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Project / service / host terminology

> **Pattern.** Cross-cutting convention. The deliverable is a consolidated definition that lives in `docs/patterns/` once the ticket is closed; every other ticket that uses *project*, *service*, or *host* references this document.

## Goal

A single canonical definition of the collection's organizational concepts — *project*, *service*, *host*, and any siblings they acquire — living in the documentation site so every role's spec can link to one place instead of redefining them. Today the terms appear across `secrets.feature.md`, `backup-restic.feature.md`, and more, with overlapping but not identical meanings.

## Scope

- Canonical text defining each term, its relationship to the others, and the conventional naming patterns (`<org>-infra`, `<org>-backup`, per-customer projects, etc.).
- Lives under `docs/concepts/` in the repo (or the documentation site if and when one materialises), linkable from every role spec.
- Glossary entry for each term with cross-references.
- Worked example: a small-team infra with 2–3 projects, their hosts, their services, showing how backup, secrets, reverse-proxy, and monitoring all use the same vocabulary.

## Design notes

- Current usage across the specs:
  - *project* — organizational grouping of hosts and services; backup trust boundary (per-project offsite repos); secrets have a service namespace but not a project one.
  - *service* — secrets role uses it as a top-level namespace (`/etc/secrets/<service>/<name>.<type>`); backup role uses it as a backup level (app/website exposed to users). These are compatible but need the definition to make the relationship explicit.
  - *host* — a machine (physical or virtual). One-host-one-project is the default; the hypervisor is the exception.
- Naming conventions as they stand today:
  - `<org>-infra` — central shared infrastructure project.
  - `<org>-backup` — separate backup infrastructure project.
  - `<org>-<customer>` or `<org>-<purpose>` — application projects.
- The blue-green pattern adds another axis (blue/green environment within a project) — needs a terminology slot too once that ticket lands.

## Open questions

- **Where exactly in the docs tree?** A dedicated `docs/concepts/terminology.md`, or a landing page under `docs/getting-started/` that every first-time reader hits?
- **Glossary vs. long-form?** A short glossary is linkable but dry; a long-form essay explains the *why* but buries the definitions. Both, with cross-links?
- **Environment as a fourth term?** Blue-green introduces one; do we define it now so downstream specs can assume it, or wait until the blue-green ticket forces the issue?
- **Role-local overrides**: the secrets role's `service` field is narrower than the backup role's concept of service. Do we reconcile (renaming one) or document the overlap explicitly and live with it?
