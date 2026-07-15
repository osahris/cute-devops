---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# ttyd

## Goal

A `ttyd` role that publishes a browser-based terminal at a **single host-wide vhost** (`terminal.<host>`), routing each authenticated user to their own per-user ttyd instance. Same router shape as [code-server](code-server.feature.md) so the two compose cleanly on a devbox. The role does not concern itself with authentication beyond what the reverse proxy + oauth2-proxy layer enforces in front of it.

A `ttyd` role already exists in the collection (basic install + systemd unit). This ticket brings it up to the current pattern: socket-publishing instead of localhost TCP, header-routed per-user backends, integration with oauth2-proxy, version pinning.

## Scope

- Install ttyd from the **GitHub release binary** at a pinned, configurable version. The Debian package lags upstream and ttyd is a single static binary anyway — downloading directly from `github.com/tsl0922/ttyd/releases` is the cleanest path.
- **Per-user backend instances**: `ttyd@<user>.service` template unit, one instance per managed user. Runs as `<user>`, opens a shell in the user's home — the user is in their own context, not a shared one.
- Each backend listens on `/run/ttyd/<user>.sock`, owned `<user>:https-socket-access`, mode `0660`. The directory `/run/ttyd/` is `0710 root:https-socket-access` so the reverse proxy can connect but cannot enumerate.
- **Single public vhost: `terminal.<host>`** (configurable). The vhost block does oauth2-proxy forward_auth, reads the `X-Auth-Request-User` header, and reverse-proxies to `unix:/run/ttyd/<that-user>.sock`. One vhost, N users.
- **Caddy-fragment auto-deploy** (option, `ttyd_configure_caddy: true` by default). When on, the role drops the vhost block at `/etc/caddy/conf.d/terminal.<host>` ready to use. Off when the deployer uses a different RPX, writes the fragment by hand, or runs ttyd without a public-facing vhost.
- ttyd's own auth is **disabled** by default — the oauth2-proxy in front handles authentication, and the user-routing is what binds an authenticated identity to its per-user backend. A site that wants ttyd's basic auth instead can opt in (and skip the routing layer).
- Default shell + working directory are inventory-configurable; the role does not assume `/bin/bash` from `/`.
