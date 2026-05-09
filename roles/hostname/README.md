<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# hostname

Configures system hostname and /etc/hosts entries.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

- `hostname_name` (default: `"{{ inventory_hostname }}"`) - System hostname
- `hostname_update` (default: `false`) - Whether to update or assert the hostname

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.hostname
      vars:
        hostname_name: myhost
```
