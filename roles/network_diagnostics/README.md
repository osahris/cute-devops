<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# network_diagnostics

Installs network diagnostic tools (nmap, tcpdump).

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

- `network_diagnostics_packages` - List of packages to install (defined in vars)

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.devops.network_diagnostics
```
