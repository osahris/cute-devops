<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
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
    - role: osahris.cute_devops.sysctl_tweaks
      vars:
        sysctl_tweaks_increase_maximum_inotify_user_watches: true
```
