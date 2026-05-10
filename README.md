<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Cute DevOps! — `mkbrechtel.devops`

The ***Cute DevOps!*** project provides a collection of patterns
and their implementations in form of Ansible roles. The repo ships as the
Ansible collection `mkbrechtel.devops` and renders to the website at
[devops.patterns.how](https://devops.patterns.how).

**⚠️ Development Phase Notice**
*This collection is currently in development (version 0.x.x). Breaking changes
may occur in any release until we reach version 1.0.0. APIs, role interfaces,
and variable names are subject to change.*

## Repository layout

```
mkbrechtel/devops/
├── patterns/   ← markdown patterns (the *what / why*)
├── roles/      ← Ansible roles (the *how* — pattern implementations)
├── website/    ← Astro + Go site, deployed to devops.patterns.how
├── playbooks/
├── docs/       ← contributor / collection-level docs
├── issues/     ← planning surface (.feature.md / .pattern.md / .bug.md)
└── …
```

`patterns/` and `roles/` cross-reference each other with editorial,
documentation-level links — a role README's "Patterns" section names the
patterns it implements, and a pattern's "Possible implementations" section
names roles that implement it. The relationship is many-to-many.

## Installation

```bash
ansible-galaxy collection install mkbrechtel.devops
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
- **fish_shell**: Fish shell installation and configuration
- **zsh_shell**: Zsh shell configuration
- **updates**: System updates management
- **users**: User account management with home directory configuration
- **podman**: Podman container runtime with DNS support
- **managed**: High-level orchestrator for a fully managed system
- **setup_check**: Checker monitoring framework setup
- **setup_deploy**: Deployment infrastructure setup
- **setup_notify**: Unified notification setup
- **check**: Base check instance role
- **check_disk**: Disk space check
- **check_ram**: RAM/memory check
- **check_ping**: Network connectivity check
- **check_systemd**: Systemd service health check
- **notify_alerta**: Alerta notification integration
- **notify_email**: Email notification integration
- **deploy**: Base deploy instance role
- **deploy_ansible_play**: Ansible playbook deployment
- **deploy_ansible_pull**: Ansible pull deployment
- **test_deploy_ohai**: Test deployment (success)
- **test_deploy_fail**: Test deployment (failure)
- **triggered_by_git_hook**: Git hook trigger for deployments

## Usage

### Base System Setup

```yaml
- hosts: servers
  become: yes
  roles:
    - mkbrechtel.devops.common
    - mkbrechtel.devops.users
    - mkbrechtel.devops.podman
```

### User Management

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.users
      vars:
        users:
          - name: alice
            groups: ['sudo', 'docker']
            shell: /bin/bash
```

## License

EUPL-1.2, with the following exceptions (per-file `SPDX-License-Identifier`
headers are authoritative):

- `roles/restic_client/`, `roles/restic_server/` — AGPL-3.0-or-later
  (carve-out for a co-author's contributions; see `CONTRIBUTIONS.md`).
- Third-party powerline-go integration snippets
  (`roles/bash_shell/files/powerline-go.sh`,
  `roles/zsh_shell/files/powerline-go.zsh`,
  `roles/fish_shell/files/global/fish_prompt.fish`) — GPL-3.0-only.
- Google Noto Emoji glyph used as logo / favicon
  (`website/static/unicorn.svg`) — Apache-2.0.
