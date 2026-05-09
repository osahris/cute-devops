<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# ssh_agent

Enables SSH agent and GPG agent as systemd user services with PAM environment setup.

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
    - mkbrechtel.devops.ssh_agent
```
