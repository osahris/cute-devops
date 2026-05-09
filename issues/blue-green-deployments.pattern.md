---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Blue-green deployments

> **Pattern.** Cross-cutting convention. The deliverable is a consolidated definition that lives in `docs/patterns/` once the ticket is closed; service roles, the deploy pipeline, and the backup-driven DR drill all reference this document.

## Goal

Support blue-green deployments inside a project — two parallel environments (blue = prod, green = staged-next) with a proxy switch between them. Use this pattern both for zero-downtime deploys and as the mechanism for continuously exercising restores from backup.

## Scope

- Model blue/green as first-class in the project structure.
- Proxy switch: atomic flip between blue and green at the reverse-proxy layer (see `reverse-proxy.pattern.md`).
- Restore-into-green: nightly, the previous day's backup is restored into the green environment and exercised. If green passes checks, it becomes the new blue on the next deploy; otherwise green is discarded and blue stays.
- If there is no pending change, green is just a replica of blue — the restore drill runs anyway and serves as a DR test.

## Design notes

- Ties into backup (`backup-restic.feature.md`, `backup-borg.feature.md`), reverse proxy, and streamable service exports.
- Defines what "exercised" means: smoke-test HTTP routes, database sanity queries, service-specific health checks.
- The blue→green→blue cadence means the project needs double the capacity; acceptable for small-team infra where the green env can be ephemeral (spun up for the drill, torn down after).

## Open questions

- What is the identity boundary — green gets the same secrets as blue, or its own (test credentials, sandboxed API keys)?
- How are external-state dependencies handled (email, payments, third-party APIs) during the green exercise? Mock, route to staging, dry-run, or rate-limit?
- Proxy switch mechanism: DNS flip, label swap in the reverse proxy, or a dedicated control channel?
- Can green coexist long-term (as a canary receiving a traffic slice), or is it always short-lived (exercise → discard or promote)?
- What constitutes a "pass" for the nightly drill — a fixed smoke suite, or service-declared checks?
