---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Reverse proxy — Caddy backend (default)

## Goal

A Caddy-based reverse proxy role that serves application sockets following the [web-service socket pattern](reverse-proxy.pattern.md). Terminates HTTPS/TLS, handles ACME, connects to web-service sockets at `/run/https/<vhost>/http.sock`. Default reverse-proxy implementation in the collection.

**Default mode: HTTP-01 with the Debian-packaged Caddy.** Per-host certs, no extra infrastructure, the stock `caddy` binary from `apt`. Each vhost is a static block in its own file under `/etc/caddy/conf.d/<vhost>` (filename = the FQDN, e.g. `matrix.example.com`; see Configuration layout). This is what the role installs unless told otherwise.

**Opt-in upgrade: wildcard certs via DNS-01 ACME.** A deliberate decision the deployer makes for a site that wants wildcard coverage. Two costs come with it: the stock Debian Caddy is replaced with the official upstream binary download (the Caddy project's CI build, with the chosen DNS provider plugin selected at download time — the Debian package has none baked in), and the deployer needs reachable authoritative DNS for the zone that supports the chosen plugin's update mechanism. The collection includes a DNS zone-hosting role (see [dns.feature.md](dns.feature.md)) that the deployer can use to satisfy this on-host, but the Caddy role makes no assumptions about which DNS backend the wildcard mode runs against — any DNS provider Caddy has a plugin for is fine. What it buys: the `*.<zone>` cert, and with it the optional dynamic-routing pattern that adds services without touching Caddy.

The two modes are not equal-weight choices: HTTP-01 with the Debian package is the default. The wildcard mode is for sites that have actively decided they want it; the role supports the upgrade path but doesn't lead with it.

Directionality (matching the pattern ticket): **left = outside, right = inside**. Caddy is to the right of the network and to the left of the app socket.

## Scope

- Install Caddy as a systemd service. **Default**: the Debian-packaged `caddy` from `apt`. **Optional**: also deploy the official upstream binary (with DNS provider plugins selected at download time) at `/usr/local/bin/caddy` when the deployer has chosen the wildcard-cert upgrade — see Distribution.
- Single HTTPS entrypoint per Caddy instance; HTTP → HTTPS redirect by default. **Scope is host-wide central Caddy only.** Per-service Caddy instances (a service running its own private Caddy on the outside of its own socket) are deployed by the consuming service's role using this role as a building block; that wiring is not in scope here.
- **Caddyfile owned by the deployer.** This role provides the installation, the unit, the reload mechanism, and the host-wide config skeleton (global block + `import /etc/caddy/conf.d/*`). Per-vhost configuration lives as drop-in files under `/etc/caddy/conf.d/`, written by whichever role is deploying the service or by inventory.
- **ACME**: HTTP-01 by default (per-host certs, no extra infrastructure, works with the Debian package). DNS-01 is an opt-in upgrade for sites that have decided they want wildcard certs; it requires the custom-build replacement plus reachable DNS for the zone via whatever provider Caddy has a plugin for. The collection's own [dns.feature.md](dns.feature.md) is one way to satisfy that; third-party DNS providers are equally valid.
- **Dynamic vhost routing** via Caddy's `{host}` placeholder is supported when a wildcard cert is in play, and proven safe under that condition — see "Dynamic routing" below. It is an option, not a requirement; per-host static blocks are equally fine.
- **Hot reload** via `caddy reload --config /etc/caddy/Caddyfile --force` on Caddyfile change. No restart, no dropped connections.
- **OIDC gating via oauth2-proxy** is out of scope here. Caddy supports both shapes oauth2-proxy needs (in-line `reverse_proxy` chaining and `forward_auth` sub-request checks) — see [oauth2-proxy.feature.md](oauth2-proxy.feature.md) for the role that deploys it and worked example snippets.

## Distribution

### Default: the Debian package

Install `caddy` from `apt`. The packaged binary handles HTTP-01 ACME, all the directives a typical reverse-proxy config uses, and the stock systemd unit. For sites that don't need wildcard certs, this is the entire installation story — no custom binaries, no overrides, no extra moving parts.

### Opt-in: official upstream binary with DNS provider plugins

The Debian package ships without any DNS provider modules, which means it cannot complete a DNS-01 challenge. Sites that have chosen the wildcard mode replace the stock binary with the **official upstream Caddy binary** — the same build the Caddy project ships, but obtained from `caddyserver.com/api/download` so the relevant DNS plugin is selected at download time. The Caddy download API serves CI-built, project-signed binaries; this is not a "custom" or third-party build, it's upstream Caddy with the plugin set the deployer asked for.

**Method:** download the binary from Caddy's download API with the desired modules selected, drop it at `/usr/local/bin/caddy`, point systemd at it via a drop-in. Concretely (the example uses `rfc2136`; substitute the relevant plugin path):

```bash
curl -o /usr/local/bin/caddy \
  "https://caddyserver.com/api/download?os=linux&arch=amd64&p=github.com%2Fcaddy-dns%2F<plugin>"
```

`/etc/systemd/system/caddy.service.d/override.conf`:

```ini
[Service]
ExecStart=
ExecStart=/usr/local/bin/caddy run --environ --config /etc/caddy/Caddyfile
ExecReload=
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/Caddyfile --force
```

The role accepts a list of DNS provider modules in inventory (default empty — meaning "use the Debian package"). When the list is non-empty the role switches to the upstream-binary path and templates the download URL accordingly. The deployer picks whatever plugin matches their DNS provider — Caddy maintains plugins for most commercial DNS APIs as well as `rfc2136` for any RFC 2136-compliant nameserver. Adding a new provider is one line in inventory plus a Caddy reinstall task.

The download API is the upstream's signed CI artifact pipeline — no `xcaddy` build step on our side, no third-party packaging.

## Configuration layout

Single-file global block + a drop-in directory for per-vhost configs:

```
/etc/caddy/
├── Caddyfile                       # global block + `import /etc/caddy/conf.d/*`
└── conf.d/
    ├── element.example.com         # per-host vhost: filename = the vhost FQDN
    ├── matrix.example.com
    ├── …
    └── _wildcard.example.com       # optional wildcard block (leading underscore sorts it last)
```

**Naming convention for files in `conf.d/`:**

- **Per-host vhost**: `/etc/caddy/conf.d/<vhost>` — one file per public hostname, no extension. Filename is the FQDN (e.g. `matrix.example.com`), matching the directory name on the socket side: `/run/https/<vhost>/http.sock`.
- **Wildcard block**: `/etc/caddy/conf.d/_wildcard.<domain>` — leading underscore so it sorts after every per-host file (per-host blocks are evaluated first; the wildcard catches the rest). One file per zone.

Caddyfile global block ships from this role. Per-host files are owned by whoever deploys each service (typically the service's own role); the wildcard file is owned by the role/inventory that owns the zone. The Caddy role does not aggregate them centrally. Adding or removing a vhost is a file write + `caddy reload`.

Trivial per-host vhosts are a one-liner:

```caddy
# /etc/caddy/conf.d/element.example.com
element.example.com {
    reverse_proxy unix//run/https/element.example.com/http.sock
}
```

Vhosts with split routing (Matrix Auth Service in front of Synapse, etc.) keep their full Caddyfile shape — the role does not constrain what a `conf.d/` file may contain.

The wildcard file (when DNS-01 + a wildcard cert are in play) is shaped around explicit host-var validation — see "Dynamic routing" below for the full pattern.

### conf.d/ is general — the socket pattern is one shape among many

The drop-in directory accepts any valid Caddyfile fragment. The shapes documented above (per-host vhost reverse-proxying to a `/run/https/<vhost>/http.sock`, wildcard block with host-var validator) are the **conventional shapes** the rest of the collection expects, not the only legal ones. A deployer can also drop in:

- A vhost that proxies to a TCP backend (`reverse_proxy localhost:8080`) — for legacy services that don't fit the socket pattern, or services on a different host.
- A static-file vhost (`root *` + `file_server`) for serving documents directly from disk without an app socket.
- A redirect vhost (`redir https://elsewhere.example.com{uri}`) for retired hostnames.
- Any other Caddyfile-expressible configuration the deployer has reason to want.

These work because `import /etc/caddy/conf.d/*` simply concatenates the fragments into the global config; Caddy doesn't care what kind of site each one declares. The role neither validates the shape nor restricts what's allowed beyond "must be a valid Caddyfile fragment, must not collide with another vhost's hostname." The socket pattern is the recommended shape for new services in this collection; existing or unusual services are free to use whatever shape they need.

## ACME

### HTTP-01 (default)

Out-of-the-box behaviour with the Debian package. Each vhost gets its own per-host cert; Caddy responds to challenges on public port 80. Suitable for prototyping, single-purpose hosts, and any deployment that doesn't run its own DNS for the zone. No additional inventory beyond the vhost names — you're done.

### DNS-01 (opt-in upgrade for wildcard certs)

A deliberate choice the deployer makes for a site that wants wildcard coverage. Two pieces have to come together: the official upstream Caddy binary with a plugin matching the chosen DNS provider (see Distribution), and reachable DNS for the zone via that provider. The collection ships a DNS zone-hosting role ([dns.feature.md](dns.feature.md)) that the deployer can use; equally, a third-party provider (Cloudflare, Route53, etc.) with a Caddy plugin works.

The Caddy global block follows the chosen plugin's schema. Example with `rfc2136` (compatible with any RFC 2136 nameserver, including the in-collection DNS role):

```caddy
{
    email <admin>@<domain>
    acme_dns rfc2136 {
        key_name acme-update
        key_alg hmac-sha256
        key {env.CADDY_ACME_TSIG_KEY}
        server <dns-server-address>:53
    }
}

import /etc/caddy/conf.d/*
```

Provider credentials (the TSIG key in the example, or API tokens for other plugins) come from the secrets role as env-typed secrets, exposed to Caddy via `Environment=` in the systemd unit drop-in. They do not live in the Caddyfile.

**Wildcard certificates.** With DNS-01 enabled and a wildcard site referenced (`*.example.com { … }` somewhere in `conf.d/`), Caddy 2.10+ will reuse that wildcard for any matching subdomain vhost rather than issuing a per-host cert — useful for a host serving many subdomains under one zone. Two-level wildcards (`*.example.com` does not match `foo.bar.example.com`) need a separate `*.bar.example.com` site; Caddy handles each automatically once referenced. Sites that prefer per-host certs even with DNS-01 enabled simply don't define a wildcard site.

## Dynamic routing — `{host}` over unix sockets

A wildcard cert unlocks an optional zero-config-add pattern. **Optional, not required.** Static per-vhost blocks are equally fine and may be preferable for sites that want explicit declarations of every public name. The wildcard block lives at `/etc/caddy/conf.d/_wildcard.<domain>` (leading underscore so it sorts after every per-host file), and uses an explicit host-var validator before any proxying happens:

```caddy
# /etc/caddy/conf.d/_wildcard.example.com

*.example.com {
    @valid_host header_regexp Host ^[a-z0-9][a-z0-9-]{0,63}\.example\.com$
    handle @valid_host {
        reverse_proxy unix//run/https/{host}/http.sock
    }
    respond 404
}
```

The `@valid_host` matcher whitelists the Host header against a regex of acceptable subdomain shapes for this zone. Subdomain label is capped at 64 characters (`{0,63}` quantifier after the mandatory leading char) — well within DNS limits and short enough to keep the resulting filesystem path bounded. Anything that does not match — path traversal attempts, malformed hostnames, hosts in a different zone, hostile-length labels — falls through to the explicit `respond 404`. The reverse_proxy directive only fires when the host is known-good.

A new service is deployed by writing its socket at `/run/https/<vhost>/http.sock` and having DNS resolve `<vhost>` to the host. Zero Caddy config changes, zero reloads. Same FQDN-named directory convention as the static blocks above — `{host}` resolves to the FQDN, the path matches.

When to use which:

- **Per-host static blocks** (default): explicit, every public name visible in `conf.d/`, no DNS-01 required. The right choice for prototypes, low service counts, and anyone who values "I can grep `conf.d/` to see what is published."
- **Wildcard block with host validator** (opt-in): right choice when the host serves a high number of subdomains under one zone and the deployer wants to add services without touching Caddy. Requires the wildcard cert plumbing and the validator regex tuned to the zone.

**One block per zone.** Each `_wildcard.<domain>` file covers exactly one zone — `*.foo.example.com` does not match `*.bar.example.com`, and the same wildcard cert won't sign both. A site with services scattered across multiple zones (e.g. `*.foo.com` *and* `*.bar.org`) writes one wildcard block per zone, each with its own zone-tuned validator regex. Caddy issues the matching wildcard cert per zone automatically; from the deployer's side this is just two `_wildcard.<domain>` files instead of one.

### Security analysis: is this safe?

Yes, under wildcard cert + HTTPS, with the explicit validator pattern above. Four independent layers prevent path traversal via attacker-controlled Host headers:

1. **TLS / SNI validation.** A TLS handshake requires the SNI to match the wildcard cert's pattern. SNI `../etc` is not a valid hostname and won't match the cert; the connection is refused before HTTP is even spoken.
2. **Go's HTTP host header validation.** Go's `net/http` server (which Caddy uses) rejects Host headers containing `/` or `..` path components as malformed (400 Bad Request). This catches anything that gets past TLS.
3. **Caddy's `{host}` sanitization.** Even in a SNI/Host mismatch (valid TLS for one hostname, attacker-supplied Host header on the request), Caddy resolves `{host}` to an empty string when traversal characters are present. Without further validation, the resulting socket path would become `/run/https//http.sock` — a non-existent file, the connection fails harmlessly with 502.
4. **The `@valid_host` regex matcher.** The wildcard block does not blindly substitute `{host}` into the path; it gates the reverse_proxy directive behind an explicit regex match against expected subdomain shapes for the zone, including a 64-character cap on the subdomain label. Anything that doesn't match (traversal, malformed names, oversized labels, foreign zones) falls through to `respond 404`. This is the explicit defense; layers 1–3 are belt-and-braces.

The one passing case — clean hostname mismatch (Host: `evil.example.com` with SNI `hello.example.com`) — passes the regex if `evil.example.com` matches the zone pattern, and resolves `{host}` to `evil.example.com`. The only socket reachable that way is one the deployer actually deployed for that hostname; an attacker with DNS pointing at the host could reach it anyway.

**Conclusion:** the wildcard block with the validator regex is safe over HTTPS. The validator is the primary defense; do not omit it just because layers 1–3 happen to backstop. The other placeholders (`{uri}`, `{path}`, `{query}`) do not share the same sanitization properties as `{host}` and must not be substituted into filesystem paths without explicit validation.

## Admin API

Caddy's admin API on `localhost:2019` by default lets anything local reload config or read state. The role binds it to a unix socket at `/run/caddy/admin.sock` instead, mode `0600` owned by `caddy:caddy`. `caddy reload` works as `caddy`; root accesses it via sudo for debugging. No TCP exposure.

## Logging

JSON access log by default, written via the `log` directive into systemd-journald (where structured fields survive). Human grep is `journalctl -u caddy -o cat | jq`. A per-vhost `log_format` knob in inventory if a site insists on Combined-Log for legacy log shippers.

## Design notes

### Why Caddy as default

For single-host, small-team setups, Caddy gets the common case right with the least configuration:

- ACME built in (HTTP-01 just works; DNS-01 is a config block away when the site needs it).
- Live reload is first-class.
- The Caddyfile reads like English. A reader unfamiliar with the role can debug a vhost by squinting at `/etc/caddy/Caddyfile` and `/etc/caddy/conf.d/*`.
- Unix-socket reverse-proxy is one directive, not a workaround.
- Sane defaults for HTTP/2, HTTP/3, security headers, log format. Less yak-shaving than nginx; less moving-target than Traefik's v2/v3 churn.

### Why the Debian package is the default install mode

The stock Debian `caddy` is fine for the common case: HTTP-01, per-host certs, reverse-proxy directives, every standard module a vhost normally needs. Replacing it with the upstream binary is a real cost — a non-apt-managed binary, a systemd override, a download URL the role has to maintain. Paying that cost is justified when the deployer has decided they want wildcard certs; otherwise it's needless complexity. Default is the package; the upgrade is opt-in.

### DNS-01 is the upgrade, not the baseline

A prototype host serving two subdomains is fine with HTTP-01; demanding the upstream-binary swap plus DNS infrastructure for that case is overkill. The role does not push DNS-01. When DNS-01 *is* chosen — because the deployer wants wildcard certs over a zone — the role supports it via whichever DNS provider plugin matches the deployer's DNS hosting. The collection's own DNS role is one option; commercial DNS APIs with Caddy plugins are equally valid; the Caddy role does not pick.

### Why deployer-owned `conf.d/` and not aggregated inventory

Per-vhost files under `/etc/caddy/conf.d/` mean each role that deploys a service drops its own vhost file, the same way drop-ins are used for systemd, sudoers, and the rest of the collection. No central aggregator role needs to know every vhost. Adds and removes are file-level, atomic, and visible in `ls`.

### Reload, not restart

`caddy reload --force` is graceful: existing connections finish, new connections use the new config, no listening-socket interruption. Restart is reserved for binary upgrades.

### Zero-config-add via dynamic routing — an option, not a default

Sites that adopt the `*.<zone>` wildcard pattern can collapse "add a service" to "create a directory + socket at the conventional path" with no Caddy interaction. The security analysis above is what makes this acceptable as an option rather than a foot-gun. It is not the default mode; the default is per-host static blocks, which a prototyping developer can use without ever touching DNS-01.

### DNS-provider credential rotation is the secrets role's job

Whatever credential the chosen DNS plugin uses (TSIG key for `rfc2136`, API token for commercial providers) is a normal secret managed by the secrets role: stored centrally, rotatable, supportable across multiple hosts when a site's setup needs it. The Caddy role just consumes the env-typed secret; rotation flows through the secrets role's existing hooks (single-host today, multi-host later if it ever becomes a concrete need). Not in scope for this ticket.
