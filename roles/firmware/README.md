<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# firmware

Installs firmware packages (Linux firmware, SOF sound firmware, Intel WiFi firmware).

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

- `firmware_packages` - List of firmware packages to install (defined in vars)

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - osahris.cute_devops.firmware
```
