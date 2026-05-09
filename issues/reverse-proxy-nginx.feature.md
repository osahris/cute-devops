---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Reverse proxy — nginx backend

## Goal

An nginx-based reverse proxy role that serves application sockets following the [web-service socket pattern](reverse-proxy.pattern.md). Sits on the outside of `/run/https/<vhost>/http.sock`, terminates HTTPS/TLS, handles ACME via a companion. Alternate to the default Caddy backend; chosen per host when a deployer prefers nginx's tuning surface and operational familiarity.

This ticket is **scoped to feature parity with Caddy** — same socket contract, same directory conventions, same default / opt-in mode split. The deep design questions Caddy went through are not re-litigated here; this role exists for deployers who specifically want nginx.

Directionality (matching the pattern ticket): **left = outside, right = inside**. nginx is to the right of the network and to the left of the app socket.

## Scope

- Install nginx as a systemd service from the Debian package. The packaged build covers what this role needs; no custom modules required for the default mode.
- Single HTTPS entrypoint per nginx instance; HTTP → HTTPS redirect by default. **Scope is host-wide central nginx only.** Per-service nginx instances are deployed by the consuming service's own role using this role as a building block; that wiring is not in scope here.
- **Configuration owned by the deployer.** This role provides the installation, the unit, the reload mechanism, and the host-wide config skeleton (`http {}` block + `include /etc/nginx/conf.d/*.conf`). Per-vhost configuration lives as drop-in files under `/etc/nginx/conf.d/`, written by whichever role is deploying the service or by inventory.
- **ACME**: HTTP-01 by default via a companion (`certbot` with the nginx plugin, default; `lego` is an alternative). DNS-01 is an opt-in upgrade for sites that have decided they want wildcard certs; the companion handles the DNS plugin and stores certs under `/etc/letsencrypt/`. The collection's own [dns.feature.md](dns.feature.md) is one way to satisfy DNS-01 via RFC 2136; third-party DNS providers with companion plugins are equally valid.
- **Dynamic vhost routing** to an FQDN-named socket directory is supported via nginx's `$host` variable in `proxy_pass unix:/run/https/$host/http.sock`, gated by a `map` of expected hostnames. Same security stance as Caddy's wildcard block. Optional, not required.
- **Hot reload** via `nginx -t && systemctl reload nginx` on config change. No restart, no dropped connections, but `-t` validation is mandatory because a bad fragment breaks every vhost on the host.
- **OIDC gating via oauth2-proxy** is out of scope here. nginx supports both shapes oauth2-proxy needs (in-line via `proxy_pass`, and `auth_request` for sub-request checks) — see [oauth2-proxy.feature.md](oauth2-proxy.feature.md).

## Distribution

The Debian package is sufficient for everything this role does in the default mode and for DNS-01 with certbot. No upstream-binary swap, no custom build.

If a site needs an nginx module not in the Debian package, that's a per-site decision (install `nginx-extras` or build from source) and falls outside the role's central scope.

## Configuration layout

Single `nginx.conf` skeleton + a drop-in directory for per-vhost configs:

```
/etc/nginx/
├── nginx.conf            # http {} block + `include /etc/nginx/conf.d/*.conf`
└── conf.d/
    ├── element.example.com.conf       # per-host vhost: filename = the vhost FQDN + .conf
    ├── matrix.example.com.conf
    ├── …
    └── _wildcard.example.com.conf     # optional wildcard block
```

**Naming convention for files in `conf.d/`:**

- **Per-host vhost**: `/etc/nginx/conf.d/<vhost>.conf` — one file per public hostname, with the `.conf` extension nginx expects. Filename stem is the FQDN, matching the socket-side directory `/run/https/<vhost>/http.sock`.
- **Wildcard block**: `/etc/nginx/conf.d/_wildcard.<domain>.conf` — leading underscore so it sorts after per-host files. One file per zone.

Per-host files are owned by whoever deploys each service; the wildcard file is owned by the role/inventory that owns the zone. The role does not aggregate centrally.

Trivial per-host vhost:

```nginx
# /etc/nginx/conf.d/element.example.com.conf
server {
    listen 443 ssl http2;
    server_name element.example.com;
    location / {
        proxy_pass http://unix:/run/https/element.example.com/http.sock;
    }
}
```

## ACME

### HTTP-01 (default, via certbot companion)

certbot installed from `apt`, configured with the nginx plugin. Certs land under `/etc/letsencrypt/live/<vhost>/`; certbot's renewal hook reloads nginx. No additional inventory beyond the vhost names.

### DNS-01 (opt-in upgrade for wildcard certs)

certbot with the relevant DNS provider plugin (also from `apt` for major providers, or `pip` for niche ones). The collection's own DNS role with `certbot-dns-rfc2136` is one option; commercial DNS providers with their certbot plugin are equally valid. Credentials wired via the secrets role.

## Dynamic routing — `$host` over unix sockets

The dynamic-routing analogue of Caddy's `{host}` block:

```nginx
# /etc/nginx/conf.d/_wildcard.example.com.conf
map $host $valid_host {
    default 0;
    "~^[a-z0-9][a-z0-9-]{0,63}\.example\.com$" 1;
}

server {
    listen 443 ssl http2;
    server_name *.example.com;
    if ($valid_host = 0) { return 404; }
    location / {
        proxy_pass http://unix:/run/https/$host/http.sock;
    }
}
```

Same security stance as Caddy's wildcard block: explicit `map` whitelists expected subdomain shapes (64-char cap on the label), `return 404` otherwise, `proxy_pass` with `$host` only fires when validated. Same trade-offs apply — opt-in, requires wildcard cert plumbing, useful when serving many subdomains under one zone.

(`if` inside `server` is one of nginx's known sharp edges, but it's accepted practice for this kind of allow/deny gating.)

## Admin

nginx has no admin API; the role does not expose one. State and config are inspected via `nginx -T` (dump effective config) and the standard log files.

## Logging

Combined Log Format by default — what every nginx-trained admin already expects. A per-vhost `access_log` directive with a JSON `log_format` is available if a site insists on structured logs for shipping.

## Design notes

The Caddy ticket is the canonical reverse-proxy spec in this collection; this ticket exists to give deployers who specifically want nginx a path that's compatible at the contract level (same socket convention, same drop-in directory, same default vs opt-in split for ACME). The nginx-specific details (worker tuning, buffer sizes, advanced module configuration) are left to implementation and nginx's own docs; they don't change the role's contract with the rest of the collection.

## Open questions

_None — implementation tracks Caddy at the feature level; backend-specific details surface during implementation, not in this spec._
