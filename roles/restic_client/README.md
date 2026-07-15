<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# restic_client

Configures restic backup client with per-backup configurations and systemd timers.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `restic_client_backup_url_prefix` - URL prefix for backup repository
- `restic_client_backup_url_hostport` - Host and port for backup server
- `restic_client_backup_username` - Backup authentication username
- `restic_client_backup_directives` - List of backup configurations

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.restic_client
      vars:
        restic_client_backup_directives:
          - name: home
            directories:
              - /home
```
