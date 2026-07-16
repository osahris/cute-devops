---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Mailing-list sync — loose coupling between Sympa and a list source

## Goal

Decouple the Sympa role from RPS. Sympa should **pull** its list
definitions and memberships from an external source over HTTP, without
any provider-specific logic (Keycloak, `rps-admin-tools`) on the mail
host. RPS becomes one possible source; any system that speaks the
contract works. This replaces the removed `rps_mail_sync` role.

## The contract

A source exposes, per virtual domain:

- `GET {base}/{domain}/sympa-mailinglists-family.xml` — a Sympa
  **family** manifest (was `rps-sync.xml`): one `<list>` per list, with
  its settings.
- `GET {base}/{list}/members` — newline-delimited member addresses.
- `GET {base}/{list}/editors` — newline-delimited editor addresses.

RPS's `rps-admin-tools` (Keycloak → family XML + membership) stays
external as one implementation. Nothing in this collection knows about
Keycloak.

## Scope

- **List lifecycle (pull):** a generic timer on the mail host —
  download `sympa-mailinglists-family.xml` → **validate** → `sympa.pl
  --instantiate_family <fam> --robot <domain> --close_unknown` →
  `make_alias_file` → `postmap`. Sympa's family mechanism does the
  actual create/close; we only fetch and apply.
- **Membership:** Sympa's native `include_remote_file` data sources
  already poll `/members` + `/editors` on their own schedule — no extra
  timer. Generalize the URL away from the RPS-specific
  `sympa_source_webserver_ip`/`_port` vars.
- **Ansible-configurable data sources:** declare Sympa data sources in
  inventory (name, kind, URL, auth) and template them into
  `/etc/sympa/<domain>/data_sources/*.incl` + the family config —
  instead of the hard-coded `rps-sync` / `rps-sync-editors` includes.
  Supports arbitrary `include_remote_file` sources, not just the RPS
  pair.
- Rename the `rps-sync` family and data-source names to something
  neutral (`list-sync` / `family-sync`).
- Retire `rps_mail_sync` for good once the above lands.

## Design notes

- **Validate before applying** — `--close_unknown` is destructive. A
  truncated / non-200 / empty download must never reach
  `instantiate_family`, or it closes every list. Fetch to temp → check
  HTTP 200 + well-formed XML + non-zero `<list>` count → then apply.
  Keep last-known-good on failure.
- **Change detection** — ETag / checksum the manifest, apply only on
  change, so the timer isn't re-instantiating every tick.
- **Auth** — the manifest endpoint defines who gets mail; require a
  token + HTTPS. `include_remote_file` supports credentialed URLs for
  members/editors. Credentials come from the secrets role.
- **Sophistication spectrum:** start with poll + ETag + validation.
  Optional add-on: a webhook trigger (reuse `webhook_server` /
  `triggered_by_git_hook`) that runs the same instantiate script on a
  provider push, for low-latency sync.

## Open questions

- Poll vs. webhook as the shipped default? (Lean poll + ETag.)
- One manifest per virtual domain, or one combined manifest for the
  host?
- Data-source config schema — strict/validated or loose passthrough?
- Family-instantiation as a systemd oneshot+timer (like today) or an
  Ansible-run reconcile task, or both?
