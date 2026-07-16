---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Certificate role — rework before trusting

## Goal

Rework the `certificate` role imported with rps-mail into a
general-purpose, trustworthy certificate-provisioning role. It is far
broader than mail (any daemon that terminates its own TLS), it is
currently a pain-bringer, and it needs a review + redesign before we
rely on it.

## Scope

- The real job: **provision cert files on disk** for daemons that
  terminate their own TLS and can't sit behind Caddy — **postfix**,
  **dovecot**, and anything similar. This is distinct from Caddy, which
  does its own in-process ACME and needs no external cert files.
- Review the existing providers (`letsencrypt`, `ca`, `manual`,
  `selfsigned`) for correctness, idempotency, and renewal handling.
- Reconcile overlap: the `postfix` role already drives `certbot`
  directly. Decide on one path — the certificate role, raw certbot, or
  a central ACME that writes files out for other daemons to consume —
  and remove the duplication.

## Design notes

- Draw the line clearly: **Caddy-fronted web services → Caddy's ACME**;
  **file-consuming daemons (postfix/dovecot) → this role**. The two
  should not fight over port 80 / the same ACME account.
- Renewal must be idempotent and reload the consuming daemon
  (postfix/dovecot) on cert change via a handler, not a blind restart.
- DNS-01 vs HTTP-01: align with the DNS role
  ([dns.feature.md](dns.feature.md)) and the secrets role for provider
  credentials, same as Caddy does.

## Open questions

- Keep a bespoke `certificate` role at all, or standardize on certbot +
  a thin file-placement/renew-hook wrapper?
- Could a central ACME client (Caddy, or a dedicated one) issue and
  drop cert files for postfix/dovecot, so there's a single ACME account
  per host? Worth prototyping.
- What exactly makes the current role a pain-bringer — enumerate the
  concrete failures so the rework has acceptance criteria.
