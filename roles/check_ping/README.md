<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# check_ping

Network connectivity monitoring using Nagios check_ping plugin.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

- `check_ping_hostname` (required) - Hostname or IP to ping
- `check_ping_cmd` - Full ping command (default computed from hostname)

## Dependencies

- `osahris.cute_devops.check`

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.check_ping
      vars:
        check_ping_hostname: google.de
```
