---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Firewall — nftables

## Goal

Host firewall role using nftables with a composable, role-driven rule
model: other roles declare what they need open, the firewall role
renders a coherent ruleset.

## Scope

- `firewall` role: installs nftables, manages `/etc/nftables.conf`,
  enables `nftables.service`.
- Default policy: deny inbound, allow outbound, allow established.
- Rule sources:
  - Inventory variables (explicit per-host rules).
  - Role contributions: other roles declare e.g. "open TCP 443 from
    anywhere" via a shared inventory variable or a drop-in file
    convention.
- Named sets: trusted networks, management IPs, wireguard peers.
- IPv4 + IPv6 in one ruleset (inet family).

## Design notes

- Avoid per-role ad-hoc iptables calls. Single source of truth rendered
  atomically.
- `nft -c -f` dry-run before swap-in; roll back on parse error.
- Logging for dropped packets (rate-limited) optional.

## Open questions

- How do roles contribute rules — a list variable aggregated by the
  firewall role (requires it to run last), or drop-in files in
  `/etc/nftables.d/` sourced by the main config?
- Do we want a "zones" abstraction (public / trusted / management) or
  is raw-rules-plus-sets enough?
- IPv6 RA / ICMPv6 defaults — what's the policy?
- How does this interact with podman's own nftables rules for
  containers?
