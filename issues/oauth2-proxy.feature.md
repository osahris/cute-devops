---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# oauth2-proxy

## Goal

A role that deploys oauth2-proxy in **forward_auth mode** with **cookie-only session storage**, providing OIDC gating in front of services managed by this collection. The role is deliberately scoped: forward_auth (the rpx asks oauth2-proxy whether each request is allowed, app socket stays in the data path), cookie-stored sessions (no Redis), and identity-only authz (allow / deny based on email / email domain — no group claims).

## Scope

### Forward-auth mode

The reverse proxy keeps proxying directly to the app socket; on every gated request, it issues a sub-request to oauth2-proxy via `forward_auth` (Caddy) / `auth_request` (nginx) / Traefik's forward-auth middleware. oauth2-proxy returns 200 + identity headers on success, 401/redirect on failure. The reverse proxy enforces the verdict and forwards augmented headers to the app.

```
                          ┌──auth-check──> oauth2-proxy
                          │
client → network → reverse proxy ────────> app socket  (with X-Auth-* headers)
```

The user-routing pattern in [ttyd](ttyd.feature.md) and [code-server](code-server.feature.md) is exactly this shape — forward_auth supplies `X-Auth-Request-User`, the rpx vhost block uses it to route to the per-user backend.

In-line chaining (oauth2-proxy in the data path between rpx and app socket) is **not in scope here.** Forward_auth covers every case the collection currently has and avoids a hop in the data path for static-asset traffic.

### Cookie-only session storage

Sessions stored entirely in the user's cookie, signed with a cookie secret from the secrets role. **No Redis, no other session backend.** One less moving part; sessions are stateless on the server side.

The cookie size budget is the constraint that drives the rest of this spec.

### Identity claims only in the cookie — server-side role gates supported

The cookie stores only what's needed to identify the authenticated user: `sub`, `email`, `preferred_username`. Group-membership lists are explicitly **not** put into the cookie — they inflate it past browser limits and require a session backend to handle gracefully, which we've ruled out.

**Role / group requirements at login time are supported.** oauth2-proxy can validate role-, group-, or scope-claims on the OIDC token *during the auth flow* and accept or reject the login based on them. The result of that check (allow / deny + identity) is what lands in the cookie — the role list itself does not. So a deployer can require "must be in group `engineering`" or "must have role `admin`" at the gate, without the gate's success metadata bloating every subsequent request.

What does **not** flow through to the app via headers: the user's full group / role list. The rpx-supplied headers are identity-only (`X-Auth-Request-Email`, `X-Auth-Request-User`). Per-path allow lists, RBAC-shaped logic, and any decision that depends on knowing all of a user's groups stay in the application — which is the right place for them anyway.

### Single instance per zone, wildcard cookie

One oauth2-proxy instance gates a whole zone via a **wildcard cookie domain**: `cookie_domains = .<zone>` (the leading dot is what makes it cover every subdomain). A single sign-in at any subdomain produces a session cookie that the browser sends to *every* `*.<zone>` request, so the user logs in once and the gate then says yes for `terminal.example.com`, `code.example.com`, `dashboards.example.com`, and so on without re-prompting.

This is the typical project shape: one project owns one zone, one OIDC client, one oauth2-proxy. A site whose project genuinely spans multiple zones runs one instance per zone (cookies don't cross zones). A service that needs a narrower OIDC-client scope or a different session lifetime can declare its own instance — the role's systemd template (`oauth2-proxy@<name>.service`) supports multiple parallel instances. Each instance publishes on a unix socket following the [web-service socket pattern](reverse-proxy.pattern.md).

### Other concerns

- **Provider:** parameterized — Keycloak, Authentik, Google, etc. The role consumes a provider URL + client credentials and otherwise does not care.
- **OIDC client ID + secret:** from the secrets role.
- **Cookie secret:** from the secrets role.
- **Provider role:** out of scope. The IdP is its own concern; see the future `keycloak.feature.md` / `authentik.feature.md` if relevant.

