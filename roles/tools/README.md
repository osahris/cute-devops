<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# tools

Installs base system tools and common packages.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

- `tools_packages` - List of packages to install (defined in vars)

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - osahris.cute_devops.tools
```
