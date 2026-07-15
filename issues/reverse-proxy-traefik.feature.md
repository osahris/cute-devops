---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Reverse proxy — Traefik backend

## Goal

A Traefik-based reverse proxy role that serves application sockets following the [web-service socket pattern](reverse-proxy.pattern.md). Sits on the outside of `/run/https/<vhost>/http.sock`, terminates HTTPS/TLS, handles ACME. Alternate to the default Caddy backend; chosen per host when a deployer prefers Traefik's provider model and dashboard.

This ticket is **scoped to feature parity with Caddy** — same socket contract, same directory conventions, same default / opt-in mode split. The deep design questions Caddy went through are not re-litigated here; this role exists for deployers who specifically want Traefik.

Directionality (matching the pattern ticket): **left = outside, right = inside**. Traefik is to the right of the network and to the left of the app socket.

## Scope

- Install Traefik as a systemd service. **Default**: the Debian-packaged `traefik` from `apt` (or upstream binary release if the package is too stale — see Distribution). HTTP-01 ACME, no plugins required.
- Single HTTPS entrypoint per Traefik instance; HTTP → HTTPS redirect by default. **Scope is host-wide central Traefik only.** Per-service Traefik instances are deployed by the consuming service's own role using this role as a building block; that wiring is not in scope here.
- **Configuration owned by the deployer.** This role provides the installation, the unit, the reload mechanism, and the host-wide static config skeleton (entrypoints, providers, ACME). Per-vhost dynamic configuration lives as drop-in files under `/etc/traefik/dynamic/`, written by whichever role is deploying the service or by inventory.
- **ACME**: HTTP-01 by default. DNS-01 is an opt-in upgrade for sites that have decided they want wildcard certs; Traefik has a wide selection of DNS provider integrations built in (no separate plugin install). The collection's own [dns.feature.md](dns.feature.md) is one way to satisfy it via RFC 2136; third-party DNS providers are equally valid.
- **Dynamic vhost routing** to an FQDN-named socket directory is supported via Traefik's file provider with a host-rule template — the same zero-config-add idea as Caddy's `{host}` block, expressed in Traefik's syntax. Optional, not required.
- **Hot reload** via Traefik's file provider watching `/etc/traefik/dynamic/`. No restart, no dropped connections.
- **OIDC gating via oauth2-proxy** is out of scope here. Traefik supports both shapes oauth2-proxy needs (in-line via a dedicated service, and `forwardAuth` middleware) — see [oauth2-proxy.feature.md](oauth2-proxy.feature.md).

## Distribution

### Default: the Debian package

Install `traefik` from `apt` where the packaged version is current enough; otherwise drop the upstream binary release at `/usr/local/bin/traefik` with a systemd unit. Both paths are well-trodden; the role picks per host.

For the wildcard-cert upgrade, Traefik does not require a custom build — DNS provider integrations are compiled into the standard binary. The role just sets the right `acme.dnsChallenge.provider` and the credentials env, and DNS-01 works.

## Configuration layout

Static config + a dynamic-config drop-in directory:

```
/etc/traefik/
├── traefik.yaml           # static config: entrypoints, providers, ACME
└── dynamic/
    ├── element.example.com.yaml      # per-host vhost: filename = the vhost FQDN
    ├── matrix.example.com.yaml
    ├── …
    └── _wildcard.example.com.yaml    # optional wildcard block
```

**Naming convention for files in `dynamic/`:**

- **Per-host vhost**: `/etc/traefik/dynamic/<vhost>.yaml` — one file per public hostname. Filename is the FQDN, matching the socket-side directory `/run/https/<vhost>/http.sock`.
- **Wildcard block**: `/etc/traefik/dynamic/_wildcard.<domain>.yaml` — leading underscore so it sorts after per-host files. One file per zone.

Per-host files are owned by whoever deploys each service; the wildcard file is owned by the role/inventory that owns the zone. The role does not aggregate centrally.

Dynamic-config files are equally free-form for non-pattern shapes (TCP backends, static-file servers, redirects) — Traefik's dynamic config language can express what the deployer needs.

## ACME

### HTTP-01 (default)

Out-of-the-box behaviour. Each vhost gets its own per-host cert via Traefik's built-in resolver on public port 80.

### DNS-01 (opt-in upgrade for wildcard certs)

The deployer picks a DNS provider Traefik supports (most are built in), wires credentials via the secrets role into Traefik's environment, configures an `acme.dnsChallenge.provider` block in the static config. The collection's own DNS role with RFC 2136 is one option; commercial providers are equally valid.

## Dynamic routing — host-rule templating to FQDN sockets

The dynamic-routing analogue of Caddy's `{host}` block: a single dynamic-config entry that proxies any vhost matching a regex to its FQDN-named socket. Lives at `/etc/traefik/dynamic/_wildcard.<domain>.yaml` and uses Traefik's host-rule + `unix://` reverse-proxy syntax. Same security stance as Caddy's wildcard block: explicit regex match against expected subdomain shapes (with a 64-char label cap), reject otherwise. Same trade-offs apply — opt-in, requires the wildcard cert plumbing, useful when serving many subdomains under one zone.

## Admin

Traefik's dashboard is bound to a unix socket at `/run/traefik/admin.sock`, mode `0600` owned by `traefik:traefik`. No public TCP exposure. Admins reach it via SSH tunnel for debugging.

## Logging

JSON access log by default, written to systemd-journald.

## Design notes

The Caddy ticket is the canonical reverse-proxy spec in this collection; this ticket exists to give deployers who specifically want Traefik a path that's compatible at the contract level (same socket convention, same drop-in directory, same default vs opt-in split for ACME). The Traefik-specific details (file provider semantics, EntryPoint configuration, middleware wiring) are left to implementation and Traefik's own docs; they don't change the role's contract with the rest of the collection.

## Open questions

_None — implementation tracks Caddy at the feature level; backend-specific details surface during implementation, not in this spec._
