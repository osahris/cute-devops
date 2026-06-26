<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# setup_deploy

Main role for the deploy system that provides the core deployment infrastructure - installs systemd units, wrapper scripts, and the deploy command to manage deployments.

## Requirements

- Debian-based operating system
- Systemd init system

## Role Variables

See `defaults/main.yaml` for available variables.

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  roles:
     - osahris.cute_devops.setup_deploy
```

## License

Apache-2.0

## Author Information

This role was created for the osahris.cute_devops collection.