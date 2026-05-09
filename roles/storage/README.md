<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# storage

Installs storage and filesystem tools (parted, mdadm, cryptsetup, lvm2, btrfs-progs, dosfstools).

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

- `storage_packages` - List of packages to install (defined in vars)

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.devops.storage
```
