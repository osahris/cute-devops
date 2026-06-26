<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# keyboard

Configures keyboard layout via /etc/default/keyboard.

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
    - osahris.cute_devops.keyboard
```
