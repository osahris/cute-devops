<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# check_disk

Disk space monitoring using Nagios check_disk plugin.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `check_disk_warning` (default: `'20%'`) - Warning threshold
- `check_disk_critical` (default: `'10%'`) - Critical threshold
- `check_disk_paths` (default: `[]`) - List of paths to monitor

## Dependencies

- `osahris.cute_devops.check`

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.check_disk
      vars:
        check_disk_paths:
          - /
          - /home
```
