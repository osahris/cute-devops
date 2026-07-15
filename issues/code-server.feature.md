---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# code-server

## Goal

A `code-server` role that publishes browser-based VS Code at a **single host-wide vhost** (`code.<host>`), routing each authenticated user to their own per-user code-server instance. Same router shape as [ttyd](ttyd.feature.md) so the two compose cleanly on a devbox. The role does not concern itself with authentication beyond what the reverse proxy + oauth2-proxy layer enforces in front of it.

## Scope

- Install code-server from the **apt package**. Version control via the standard apt-pinning mechanism (or just trusting the distro / coder.com apt repo's release cadence).
- **Per-user backend instances**: `code-server@<user>.service` template unit, one instance per managed user. Runs as `<user>`, edits in `~`.
- Each backend listens on `/run/code-server/<user>.sock`, owned `<user>:https-socket-access`, mode `0660`. The directory `/run/code-server/` is `0710 root:https-socket-access`.
- **Single public vhost: `code.<host>`** (configurable). The vhost block does oauth2-proxy forward_auth, reads the `X-Auth-Request-User` header, and reverse-proxies to `unix:/run/code-server/<that-user>.sock`. One vhost, N users.
- **Caddy-fragment auto-deploy** (option, `code_server_configure_caddy: true` by default). When on, the role drops the vhost block at `/etc/caddy/conf.d/code.<host>` ready to use. Off when the deployer uses a different RPX, writes the fragment by hand, or runs code-server without a public-facing vhost.
- code-server's built-in auth is **disabled** by default — the oauth2-proxy in front handles authentication, and the user-routing is what binds an authenticated identity to its per-user backend. Opt-in for sites that want code-server's password auth (and to skip the routing layer).
