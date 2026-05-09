<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# bash_shell

Bash shell configuration.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.devops.bash_shell
```
