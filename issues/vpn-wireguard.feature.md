---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# VPN — WireGuard

## Goal

Role(s) to configure WireGuard between hosts managed by this collection.
Targets a mesh or hub-and-spoke topology depending on inventory.

## Scope

- `wireguard` role that configures one interface (`wg0` by default).
- Peers defined in inventory variables.
- Key management via the secrets role: private keys stored per-host,
  public keys distributed via inventory facts.
- Topologies:
  - Hub-and-spoke: one "server" peer, many clients.
  - Full mesh: every host peers with every other host.
- Routes / AllowedIPs configurable per peer.

## Design notes

- Config generated from inventory; no manual key pasting.
- Each host generates its own private key on first run (via the secrets
  role, source=random), then publishes its public key as an ansible fact
  the control node collects.
- A second pass renders each host's peer list using collected public
  keys.
- `wg-quick@wg0.service` for lifecycle.

## Open questions

- Two passes imply two playbook runs (or `meta: refresh_inventory`). Is
  that acceptable?
- Mesh vs hub — is this one role with a mode flag, or two roles?
- Do we need MTU tuning defaults for Hetzner / common ISPs?
- Firewall interaction — does this role open UDP 51820, or is that the
  `firewall` role's job (via a "profile" concept)?
- IPv6-only mesh possible?
