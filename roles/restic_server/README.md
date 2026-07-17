<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# restic_server

Sets up restic REST server for backup storage with authentication.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `restic_server_backupstorage` (default: `"/srv/backupstorage/"`) - Backup storage directory
- `restic_server_flags` (default: `"--listen :8000"`) - Server flags

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.restic_server
      vars:
        restic_server_backupstorage: /srv/backups/
```
