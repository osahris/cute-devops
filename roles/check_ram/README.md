<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# check_ram

Memory usage monitoring using Nagios check_memory plugin.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

## Dependencies

- `osahris.cute_devops.check`

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - osahris.cute_devops.check_ram
```
