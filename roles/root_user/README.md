<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# root_user

Configures root user account (password, SSH keys, shell).

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `root_user_password` (optional) - Hashed password for root
- `root_user_with_ssh_key` (default: `false`) - Generate SSH key for root
- `root_user_ssh_authorized_keys` (optional) - List of authorized SSH keys
- `root_user_shell` (default: `"/bin/bash"`) - Root user shell

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.root_user
      vars:
        root_user_shell: /bin/bash
```
