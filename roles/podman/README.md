<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Podman Role

This role installs and configures Podman container runtime with DNS support on Debian/Ubuntu systems.

## Requirements

- Ansible >= 2.14
- Debian (bookworm, bullseye) or Ubuntu (jammy, focal)
- Root/sudo privileges for package installation

## Role Variables

Currently no variables are defined for this role.

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - osahris.cute_devops.podman
```

## What Gets Installed

- `podman` - Container runtime
- `dnsmasq` - DNS services
- `containernetworking-plugins` - Container networking support
- `podman-compose` - Docker Compose compatibility
- `golang-github-containernetworking-plugin-dnsname` - DNS name resolution plugin for containers
- `aardvark-dns` - DNS server for container name resolution

## License

EUPL-1.2