---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# OS install / bootstrap

## Goal

Automated OS installation so a machine can go from bare metal (or a fresh
cloud image) to "managed by this collection" without manual steps.

## Scope

- Debian preseed generation for PXE / ISO installs.
- cloud-init templates for cloud / libvirt / Proxmox targets.
- Post-install hook: fetch and run `managed` role on first boot.
- Optional PXE server role (dnsmasq + TFTP + HTTP) for on-prem use.

## Design notes

- Bootstrap minimum: SSH key authorized, ansible user, python3, network
  up. Everything else is `managed` role territory.
- Preseed / cloud-init templates take inventory-like variables
  (hostname, primary user, SSH keys) so the same data model drives
  first-boot and ongoing management.
- First-boot unit could pull an Ansible playbook from a git URL and run
  it (mirrors `deploy_ansible_pull` pattern).

## Open questions

- Is PXE actually needed, or is cloud-init + manual first-boot for bare
  metal enough?
- Where do preseed templates live — this collection, or a separate
  "installer" sibling repo?
- Do we target one Debian version at a time, or maintain templates for
  bookworm + trixie in parallel?
- Secrets at first boot — how do we inject the initial secret needed to
  fetch everything else? (SSH key in authorized_keys is probably the
  only one; the rest is bootstrapped by the secrets role post-install.)
