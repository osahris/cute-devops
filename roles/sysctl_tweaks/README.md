<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# sysctl_tweaks

Applies system performance tweaks via sysctl.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

- `sysctl_tweaks_increase_maximum_inotify_user_watches` (default: `false`) - Increase inotify max_user_watches

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.sysctl_tweaks
      vars:
        sysctl_tweaks_increase_maximum_inotify_user_watches: true
```
