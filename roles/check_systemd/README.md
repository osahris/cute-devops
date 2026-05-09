<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# check_systemd

Systemd unit health monitoring using Nagios check_systemd plugin.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

## Dependencies

- `mkbrechtel.devops.check`

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.devops.check_systemd
```
