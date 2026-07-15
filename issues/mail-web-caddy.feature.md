---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Mail web UIs on Caddy — drop the Apache role

## Goal

Serve the mail stack's web interfaces through the collection's Caddy
reverse proxy ([reverse-proxy-caddy.feature.md](reverse-proxy-caddy.feature.md))
instead of the Apache role imported with rps-mail. Remove the `apache`
role; one web server for the collection, not two.

## Scope

- **Remove `roles/apache`.** Nothing in the collection should depend on
  it once the consumers below are moved.
- Move the three web consumers to Caddy following the
  [web-service socket pattern](reverse-proxy.pattern.md)
  (`/run/https/<vhost>/http.sock`):
  - **postfixadmin** — PHP, via php-fpm behind Caddy.
  - **rainloop** — PHP, via php-fpm behind Caddy.
  - **sympa web** (`wwsympa`) — Perl FastCGI, via Caddy's fastcgi
    transport.
- Drop Apache vhost templates from the `postfixadmin`, `rainloop`, and
  `sympa` roles; each role writes a Caddy `conf.d/<vhost>` drop-in
  instead.

## Design notes

- postfixadmin and rainloop are PHP apps → we need a **php-fpm** story
  (new role or shared) that Caddy proxies to. Check whether the
  collection already has one before adding.
- Sympa ships `wwsympa` as a FastCGI app; Caddy speaks FastCGI
  directly (`reverse_proxy` with the `fastcgi` transport), so no Apache
  `mod_fcgid` needed — but the socket path and env need pinning down.
- Static assets (sympa `/static-sympa`, rainloop assets) served by
  Caddy `file_server`.

## Open questions

- Central host-wide Caddy, or a per-service Caddy instance for the mail
  box? (Pattern allows either.)
- One php-fpm pool shared by postfixadmin + rainloop, or one per app?
- Does `wwsympa` need SOAP (`sympasoap`) exposed too, or only the web
  UI?
