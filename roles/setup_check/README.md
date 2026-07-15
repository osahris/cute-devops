<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# setup_check

Base monitoring framework with directory structure, scripts, and systemd integration.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

None.

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - osahris.cute_devops.setup_check
```
