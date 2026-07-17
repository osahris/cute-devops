---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# OIDC gating

> **Pattern.** Cross-cutting convention. The deliverable is a consolidated definition that lives in `patterns/` once the ticket is closed; the [oauth2-proxy](oauth2-proxy.feature.md) role, the [reverse-proxy](reverse-proxy.pattern.md) backends, and any service-role that wants OIDC in front reference this document.

## Goal

A single, repeated convention for putting OIDC-based access control in front of a web service in this collection. The pattern recognises **two shapes** depending on whether oauth2-proxy sits in the data path or off to the side. Both shapes share the cookie / authz / identity-header contract; they differ only in how the gate is wired into the request flow.

## Scope

### Shape A — in-line OIDC proxy

oauth2-proxy is on the data path between the public-facing rpx and the app socket. Every request passes through it; oauth2-proxy enforces the gate and proxies authenticated requests to the app, adding identity headers on the way through.

```
client → network → reverse proxy → oauth2-proxy → app socket  (with X-Auth-* headers)
```

The rpx terminates TLS and proxies the whole vhost to oauth2-proxy's socket; oauth2-proxy is the gate.

**Use when:**

- A single service is gated as a whole — every request is gated, no public paths.
- The simplest mental model is the priority. One vhost, one oauth2-proxy, one app socket; the chain reads top-to-bottom.
- The service doesn't need to vary auth behaviour per path.

### Shape B — forward_auth

oauth2-proxy is *off* the data path. The rpx handles every request itself; on each gated request it issues a sub-request to oauth2-proxy via `forward_auth` (Caddy) / `auth_request` (nginx) / `forwardAuth` middleware (Traefik). On 200 + identity headers, the rpx proxies to the app socket and forwards the headers; on 401/redirect, the rpx surfaces that to the client.

```
                          ┌──forward_auth──> oauth2-proxy
                          │
client → network → reverse proxy ─────────────> app socket  (with X-Auth-* headers)
```

**Use when:**

- Multiple services / multiple vhosts share one oauth2-proxy (one instance, many gated vhosts).
- The rpx vhost block does anything beyond plain proxying — per-path public/private splits, per-user backend routing using `X-Auth-Request-User` (the [ttyd](ttyd.feature.md) and [code-server](code-server.feature.md) shape), header injection, rate limits.
- Avoiding oauth2-proxy as a hop on the data path matters (large static-asset traffic, websocket/streaming).

### What both shapes share

- **Cookie-only session storage.** No Redis, no other backend. One less moving part.
- **Wildcard cookie domain at the project's primary zone.** `cookie_domains = .<zone>` — one login covers every gated subdomain in the project.
- **Identity-only headers** flow through to the app: `X-Auth-Request-User`, `X-Auth-Request-Email`, `X-Auth-Request-Preferred-Username`. Group / role lists do **not** flow through — the cookie-size budget rules them out.
- **Server-side role gates at login time.** oauth2-proxy can require role / group / scope claims on the OIDC token *during the auth flow* and accept or reject the login based on them. The result of the check (allow / deny + identity) is what lands in the cookie; the role list does not.
- **Trust assumption: only the chain produces `X-Auth-Request-*` headers.** App sockets are unbound from the network (filesystem unix sockets per [reverse-proxy.pattern.md](reverse-proxy.pattern.md)); the only path that produces those headers is through the rpx + oauth2-proxy chain. Apps trust the headers because the socket layout makes spoofing structurally impossible.

### Authz levers a service-role gets

- **At the gate** (oauth2-proxy config): allow / deny based on email, email domain, and OIDC token claims at login time. Role / group requirements live here.
- **At the rpx** (Shape B only — Shape A doesn't have an rpx-side gate): allow / deny based on path, IP, time-of-day, anything the rpx can match. Per-path public/private splits live here.
- **At the app**: anything that requires knowing the user's full group list, custom claims, or RBAC-shaped state. The app reads identity from headers and consults its own authz (DB, config, etc.). The pattern punts this to the app — that's the right place for it.

### What this pattern does *not* specify

- The OIDC provider itself. Keycloak, Authentik, Google, GitHub, Azure AD — out of scope; the pattern works against any OIDC-conformant IdP.
- Per-rpx-backend syntax for forward_auth. Each rpx backend ([Caddy](reverse-proxy-caddy.feature.md), [nginx](reverse-proxy-nginx.feature.md), [Traefik](reverse-proxy-traefik.feature.md)) ships its own snippet.
- Logout flow. Provider-dependent; oauth2-proxy handles whatever the provider supports.

## Design notes

### Why two shapes — when does each win

Shape A is structurally simpler: oauth2-proxy is the gate, full stop, no sub-request dance. Easy to reason about, easy to debug ("if it's gated, oauth2-proxy saw it"). The trade is rigidity: oauth2-proxy is on the data hot path for every byte, including assets the gate doesn't actually need to inspect, and the rpx-side configuration above oauth2-proxy can't do anything richer than a single proxy directive.

Shape B is structurally more complex (sub-request, two units involved per request) but enables the rpx vhost block to do real work — per-path matchers, header substitution for user-routing, conditional bypass for `/healthz`. Once the rpx config has any non-trivial logic, Shape B is the only option.

Most services in the collection will land in one camp by default: a small internal service is fine in Shape A; anything that wants the user-router or per-path splits ends up in Shape B. The pattern is deliberate about both.

### Why identity-only headers, group gates at login

The cookie-size budget. Cookies cap at ~4 KB browser-side; group lists from an enterprise IdP routinely exceed that. Rather than ship a feature that subtly breaks at scale, group / role *gating* happens at the OIDC login flow (where oauth2-proxy validates token claims server-side and stores only the result), and the cookie-borne, header-forwarded identity is just identity. Apps that need group-aware authz read it from the IdP themselves or from their own state.

### Why wildcard cookie domain

One oauth2-proxy instance, one zone, one cookie. A user signs in once at any subdomain; the browser carries the session cookie to every other subdomain. One login → entire project unlocked. Per-service narrower scopes are an opt-in for the rare narrower-scope case (multiple oauth2-proxy instances, each with its own client and cookie scope).

### Why apps trust the X-Auth-Request-* headers

The rpx is the only ingress; sockets are unbound from the network (filesystem unix sockets per the [web-service socket pattern](reverse-proxy.pattern.md)). The only path that produces `X-Auth-Request-*` headers is through the rpx + oauth2-proxy chain. There is no path where an attacker could supply spoofed headers — that's the trust assumption, made structural by the socket layout rather than relying on the app to strip and re-inject.

## Open questions

- **Default shape for service-roles.** When a service-role enables OIDC gating, which shape does the role pick by default? Lean Shape B (forward_auth) when the service shares an oauth2-proxy with siblings on the host, Shape A when the service runs solo on its vhost. Articulating this default sharply would let service-roles avoid asking the question every time.
- **Logout-flow coverage matrix.** OIDC logout is provider-inconsistent; oauth2-proxy handles RP-Initiated Logout where supported, cookie-clear-only otherwise. Worth listing which providers we test against (Keycloak, Authentik likely) and what each one does.
- **Per-path public/private split snippets per backend** (Shape B only). Common case: `/api`, `/healthz` bypass the gate, everything else is gated. The shape is mechanical per backend (`@matcher` in Caddy, `location` block in nginx, middleware-per-route in Traefik) but worth a documented snippet per backend so service-roles can copy it.
