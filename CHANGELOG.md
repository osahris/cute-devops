<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Merged checker collection into sysops
  - Checker framework: systemd-based monitoring with Nagios plugins
  - Check roles: check_disk, check_memory, check_ping, check_systemd
  - Notification roles: notify_email, notify_alerta
  - Meta roles: system_checks, disk_checks, network_checks
  - Checker monitor dashboard
- Merged deploy-deploy collection into sysops
  - Core `deploy` wrapper script with systemd journal integration
  - Systemd unit support (system and user level): service, timer, path, target units
  - Deployment roles: deploy_deploy, deploy_instance, ansible_play, ansible_pull
  - Git hook and webhook trigger roles: triggered_by_git_hook, webhook_server
  - Email notification support for deployment status
  - Test roles: test_ohai, test_fail

### Added
- New `bash_shell` role for Bash shell configuration
- New `elvish_shell` role for Elvish shell configuration
- New `zsh_shell` role for Zsh shell configuration

### Changed
- Renamed `fish` role to `fish_shell`
- Renamed `debian_sources` role to `debian_apt_sources`
- Removed obsolete `debian_repos` role (refactored into `debian_apt_sources`)
- Changed `updates` role to use meta dependency on `debian_apt_sources` instead of import_role
- Renamed `network` role to `resolvconf` and removed it from `common` dependencies
- Reorganized checker file structure: moved files from global `files/` directory to their appropriate roles

## [0.1.1] - 2026-02-02

### Fixed
- Added missing README.md and meta/main.yml for motd role (Galaxy import requirement)

## [0.1.0] - 2026-02-02

### Changed
- Renamed collection from `mkbrechtel.sys` to `mkbrechtel.sysops`

### Added
- New `motd` role for message of the day configuration

### Fixed
- Fixed user SSH key configuration
- Fixed home directory mode permissions

## [0.0.3] - 2025-07-26

### Fixed
- Updated meta/runtime.yml with mandatory requires_ansible field set to '>=2.14.3'
- Fixed Galaxy import error about missing requires_ansible in meta/runtime.yml

## [0.0.2] - 2025-07-26

### Fixed
- Added missing README.md files for all roles (ansible, common, users)
- Added missing meta/main.yml files for all roles with proper galaxy_info
- Fixed Galaxy import errors by ensuring all roles have required documentation

## [0.0.1] - 2025-07-26

### Added
- Podman role for installing container runtime with DNS support
  - Installs podman, dnsmasq, containernetworking-plugins, and podman-compose
  - Includes dnsname and aardvark-dns plugins for container DNS resolution
  - Daemonless operation (no systemd service management)
- Comprehensive project documentation
  - Enhanced README.md with collection overview and usage examples
  - CHANGELOG.md following Keep a Changelog format
  - CODING.md with development guidelines
  - CLAUDE.md for AI assistant context
  - RELEASE.md with release process documentation
- GitHub Actions release workflow
  - Automatic collection build on version tags
  - Ansible Galaxy publishing support
- Existing roles from initial collection structure:
  - **ansible** role for Ansible configuration and tools setup
  - **common** role for base system configuration (packages, repos, locales, timezone, etc.)
  - **updates** role for system update management
  - **users** role for user account management with home directory configuration

[0.1.1]: https://github.com/mkbrechtel/sysops/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/mkbrechtel/sysops/compare/v0.0.3...v0.1.0
[0.0.3]: https://github.com/mkbrechtel/sysops/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/mkbrechtel/sysops/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/mkbrechtel/sysops/releases/tag/v0.0.1