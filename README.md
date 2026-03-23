<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# sysops Ansible Collection

The **sysops** collection provides base system configuration and management roles for Debian systems. This collection focuses on common system administration tasks, user management, and container runtime setup.

**⚠️ Development Phase Notice** 
*This collection is currently in development (version 0.x.x). Breaking changes may occur in any release until we reach version 1.0.0. APIs, role interfaces, and variable names are subject to change.*

## Installation

```bash
ansible-galaxy collection install mkbrechtel.sysops
```

## Requirements

- Ansible >= 2.14.3
- Debian 12/bookworm or 13/trixie

## Included Roles

- **ansible**: Ansible configuration and tools setup
- **common**: Base system configuration orchestrator (includes all roles below)
- **debian_apt_sources**: Debian APT sources configuration (deb822 format)
- **tools**: Base tools and common packages
- **storage**: Storage and filesystem tools
- **firmware**: CPU and device firmware packages
- **root_user**: Root user account configuration
- **ssh_agent**: SSH and GPG agent systemd user service setup
- **hostname**: Hostname and /etc/hosts configuration
- **locales**: System locale generation and configuration
- **timezone**: System timezone configuration
- **keyboard**: Keyboard layout configuration
- **resolvconf**: Resolvconf DNS configuration
- **sysctl_tweaks**: System sysctl performance tweaks
- **microcode**: CPU microcode updates
- **bash_shell**: Bash shell configuration
- **elvish_shell**: Elvish shell configuration
- **fish_shell**: Fish shell installation and configuration
- **zsh_shell**: Zsh shell configuration
- **updates**: System updates management
- **users**: User account management with home directory configuration
- **podman**: Podman container runtime with DNS support

## Usage

### Base System Setup

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.sysops.common
    - mkbrechtel.sysops.users
    - mkbrechtel.sysops.podman
```

### User Management

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.sysops.users
      vars:
        users:
          - name: alice
            groups: ['sudo', 'docker']
            shell: /bin/bash
```

## License

AGPL-3.0-or-later
