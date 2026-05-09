<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# locales

Generates system locales and configures locale settings.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `locales_gen` (default: `["de_DE.UTF-8"]`) - Locales to generate
- `locales_lang` (default: `"de_DE.UTF-8"`) - System LANG setting
- `locales_with_all` (default: `false`) - Install locales-all package

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.locales
      vars:
        locales_gen:
          - en_US.UTF-8
        locales_lang: en_US.UTF-8
```
