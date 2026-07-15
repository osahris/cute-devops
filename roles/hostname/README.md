<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
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
    - role: osahris.cute_devops.hostname
      vars:
        hostname_name: myhost
```
