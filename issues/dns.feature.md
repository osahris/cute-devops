---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# DNS

## Goal

Interchangeable DNS server roles. A knot implementation already exists;
this ticket is about generalizing so other implementations (bind, nsd,
powerdns) can slot in behind a common interface.

## Scope

- A common "DNS zone" data model in inventory: zones, records, SOA
  params, TSIG keys.
- Implementation roles: `dns_knot` (exists), `dns_bind` (future),
  `dns_nsd` (future), etc.
- A top-level `dns` role that dispatches to one implementation based on
  a variable.
- Zone file rendering is implementation-specific but driven from the
  same input data.
- Primary + secondary / AXFR support where backends allow.

## Design notes

- Think of the implementation roles as "drivers" — the data contract is
  shared.
- TSIG keys via the secrets role.
- DNSSEC: each impl handles signing; expose a consistent interface
  (on/off, algorithm).
- DNS-01 ACME challenges: the DNS role(s) should expose a way for the
  reverse-proxy role to publish challenge records (dynamic update or
  dropped file).

## Open questions

- Is the common data model strict (schema-validated) or loose?
- Where does the `knot` role live today — in this collection, or
  external? (User said "I already have a knot role".)
- Do we support mixing implementations (one host knot, one host bind)
  or must all hosts in a zone's serve-group use the same impl?
- Authoritative-only, or also recursive/validating resolvers? (That's a
  whole separate category.)
- Secondary DNS on Hetzner / other hosted services — in scope?
