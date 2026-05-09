<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# debian_apt_sources

Configures Debian APT package repositories in deb822 format.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `debian_apt_sources_distribution` (default: `"trixie"`) - Debian distribution
- `debian_apt_sources_mirror` - APT mirror URL
- Feature flags: `debian_apt_sources_with_contrib_component`, `debian_apt_sources_with_non_free_firmware_component`, `debian_apt_sources_with_backports_suite`, etc.

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.devops.debian_apt_sources
```
