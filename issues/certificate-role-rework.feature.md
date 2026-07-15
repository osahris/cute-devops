---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

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

## Concrete pain points (acceptance criteria)

The rework should resolve each of these, observed in the role as imported:

- **Inert change handler.** `handlers/main.yml`'s `certificate changed`
  only stats the cert; nothing reloads the consuming daemon on rotation.
  A rotated cert should reload postfix/dovecot.
- **Duplicate ACME paths racing on port 80.** The `postfix` role runs its
  own `certbot --apache` (`roles/postfix/tasks/certbot.yaml`, cron renew
  with a `systemctl reload postfix` post-hook), while this role's
  `provider-letsencrypt.yml` runs a separate `acme_certificate` HTTP-01
  flow writing to `/var/www/default/.well-known/`. One host, one mail
  name, two ACME clients, two account keys, two on-disk conventions.
  Collapse to one.
- **letsencrypt provider hardcoded** to HTTP-01 and the production ACME
  directory, with no staging and no DNS-01 — conflicting with the DNS-01
  goal and unusable behind a daemon that owns port 80.
- **Certs never rotate once present.** The `ca` and `manual` providers
  guard purely on `creates:`, so a CA-signed or manually placed cert
  silently goes stale and expires; only `selfsigned` has a `checkend`
  trigger.
- **Private-key directory unreadable by daemons.** `/etc/ssl/private` is
  `root:root 0710` with the `ssl-cert` group grant left commented out, so
  a non-root consumer cannot read the key.

(A `certificate_provider: selfsigned` default has already been added so a
bare invoke no longer fails on an undefined provider include.)

## Open questions

- Keep a bespoke `certificate` role at all, or standardize on certbot +
  a thin file-placement/renew-hook wrapper?
- Could a central ACME client (Caddy, or a dedicated one) issue and
  drop cert files for postfix/dovecot, so there's a single ACME account
  per host? Worth prototyping.
