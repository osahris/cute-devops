---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Reverse proxy — web-service socket pattern

## Goal

A single convention for how web application servers expose themselves to a reverse proxy on the same host: a unix-socket path. Nothing more. Apps publish a socket at a known path; whatever sits on the outside of that socket finds it by path, not by configuration handed in from the app side.

## Model diagram

**Left = outside (client / browser), right = inside (the service).** A reverse proxy is to the right of the network and to the left of the service socket. Every box in the chain is composed of layered client/server roles that flip orientation at each boundary (the network, then the unix socket).

Beyond plain proxying, the reverse proxy may apply additional access control on the way in — auth (basic, forward-auth, OIDC via an oauth2-proxy hop), rate limiting, IP allowlists, header inspection — before any traffic reaches the service socket. The service therefore cannot assume "any byte that arrives is from an authenticated user"; that is the deployer's call and lives in the RPX configuration, not in this pattern.

The full data flow:

```
  Client {
    App (e.g. Browser)
      → HTTP Client
        → TLS Client
          → Transport Client (TCP/UDP)
  }
    → Network →
  Reverse Proxy {
    Transport Server (TCP/UDP)
      → TLS Server
        → HTTP Socket Client
  }
    → Socket in FS →
  Service {
    Socket Listener (HTTP)
      → …
  }
```

Left is the outside, right is the inside. Backend is handled right side, Frontend is executed left side (though assets providing is a backend service too).

## Terminology

This ticket only defines the reverse-proxy-specific terms. Cross-cutting terms — *project*, *service*, *host* — are defined once in [project-service-terminology.pattern.md](project-service-terminology.pattern.md) and are assumed here.

| Term | Meaning |
|---|---|
| **Outside / Inside** | Direction labels. Outside = closer to the client. Inside = closer to the service. Used consistently throughout the pattern and implementation tickets. |
| **Client** | The composite outside-end stack: app, HTTP client, TLS client, transport client. Lives outside the host. |
| **Reverse proxy / "rpx" ** | The host-resident layer that terminates public-facing transport + TLS and re-emerges as an HTTP client toward a service socket. To the right of the network, to the left of the FS socket. |
| **FS socket** | The unix socket file at `/run/https/<vhost>/http.sock`, where `<vhost>` is the fully-qualified hostname (e.g. `matrix.example.com`). The contract. The boundary between RPX and service is the filesystem, not the network. |
| **Chain** | Multiple RPX-shaped layers stacked between client and service on the host (e.g. Caddy → oauth2-proxy → service). Each link is itself an outside/inside pair. |
| **Frontend / Backend** | Where execution happens, not where code originates and not what shape the UI takes. Frontend = on the client side of the network — a browser, a native GUI, *a CLI talking to an API*, a script consuming an HTTP endpoint. Backend = on the host, serving the API and (often) the assets that frontends fetch. A bundle of frontend code served from the host is still backend-served; the same code, once running in the user's browser or shell, is the frontend. |

## Implementations

Default: [Caddy](reverse-proxy-caddy.feature.md).

Alternates:

- [Traefik](reverse-proxy-traefik.feature.md)
- [nginx](reverse-proxy-nginx.feature.md)

An app role only needs to follow this pattern to be servable by any of them.

## Scope

### The socket convention

```
/run/https/<vhost>/http.sock
```

`<vhost>` is the fully-qualified hostname the service is reachable at — `matrix.example.com`, `element.example.com`, etc. **Not** the abstract service name from [project-service-terminology.pattern.md](project-service-terminology.pattern.md); the abstract service may be `matrix`, but the directory under `/run/https/` is always the FQDN. Using the FQDN aligns the path with how reverse proxies route (by Host header / SNI) and lets the dynamic-routing pattern (where the rpx substitutes `{host}` into the path) work without a separate naming convention.

That's it. One HTTP socket per vhost, at a known path.

- **Protocol on the socket**: plain HTTP. TLS is the outer layer's job.
- **Ownership / perms**: directory `0750 <service-user>:https-socket-access`; socket `0660`. The shared `https-socket-access` system group is the access channel — any RPX (Caddy under its `caddy` user, nginx under `www-data`, Traefik under `traefik`, …) joins this group to read sockets, regardless of which user it normally runs as. A dedicated group rather than reusing `www-data` because each RPX has its own conventional user; the shared access channel is the group, not any one user.
- **Containers**: podman with `--network=none` plus a bind-mount of `/run/https/<vhost>/`. The container writes its socket into that directory. No bridge network, no exposed ports, no localhost TCP.
