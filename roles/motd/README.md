<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# motd

Configure the system Message of the Day (MOTD).

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `motd` | No | See below | The message of the day content. Defaults to a message stating the system is managed by the osahris.cute_devops collection. |

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.motd
      vars:
        motd: |
          Welcome to {{ inventory_hostname }}
          Managed by Ansible
```

## License

EUPL-1.2
