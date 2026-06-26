<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Ansible Role

This role configures Ansible and installs additional Ansible-related tools.

## Requirements

- Ansible >= 2.14
- Debian (bookworm, bullseye) or Ubuntu (jammy, focal)
- Root/sudo privileges for package installation

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - osahris.cute_devops.ansible
```

## Features

- Installs and configures Ansible
- Optionally installs additional tools:
  - Mitogen for performance optimization
  - ARA (Ansible Run Analysis) for playbook recording
  - ansible-bender for building container images
  - etcd3 lookup plugin support
- Configures Ansible settings via ansible.cfg

## License

EUPL-1.2