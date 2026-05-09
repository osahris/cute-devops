<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# check

Generic base role for individual check implementations. Creates check directory structure and environment files.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `check_id` (required) - Identifier for the check
- `check_timeout` (default: `60`) - Check execution timeout in seconds
- `check_memory_max` (default: `'100M'`) - Memory limit for check process
- `check_cpu_quota` (default: `'20%'`) - CPU quota for check process

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.check
      vars:
        check_id: my_check
```
