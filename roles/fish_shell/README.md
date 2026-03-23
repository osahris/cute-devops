<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# fish_shell

Fish shell installation and configuration. Installs fish and sets up global configuration for prompt, title, and greeting.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.sysops.fish_shell
```
