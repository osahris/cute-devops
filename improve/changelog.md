---
title: Changelog
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Relicensed from AGPL-3.0-or-later to EUPL-1.2; the
  `restic_client` and `restic_server` roles stay under
  AGPL-3.0-or-later pending consent from a co-author
- Renamed collection from `mkbrechtel.sysops` to `osahris.cute_devops`
  (namespace moved to the `osahris` org); merged the standalone Cute
  Patterns! library into `patterns/` and the site layout into `website/`,
  and moved the website to `cute-devops.patterns.how`
- Rename roles for consistent naming scheme:
  - Setup roles: `checker` -> `setup_check`, `deploy_deploy` -> `setup_deploy`, new `setup_notify`
  - Check instance roles: `checker_check` -> `check`, `checker_check_disk` -> `check_disk`,
    `checker_check_memory` -> `check_ram`, `checker_check_ping` -> `check_ping`,
    `checker_check_systemd` -> `check_systemd`
  - Notify instance roles: `checker_notify_alerta` -> `notify_alerta`,
    `checker_notify_email` -> `notify_email`
  - Deploy instance roles: `deploy_instance` -> `deploy`
  - Test roles: `deploy_test_fail` -> `test_deploy_fail`, `deploy_test_ohai` -> `test_deploy_ohai`
  - Trigger roles: `deploy_triggered_by_git_hook` -> `triggered_by_git_hook`
  - New `managed` orchestrator role replaces `checker_disk_checks`, `checker_network_checks`,
    `checker_system_checks` with feature flags
- All role variable prefixes updated to match new role names

## [0.2.2] - 2026-03-24

### Changed
- Require successful ansible-lint run before release workflow publishes to Galaxy

## [0.2.1] - 2026-03-24

### Fixed
- Fix license on powerline-go integration files to GPL-3.0-only
- Add missing README.md and meta files for all roles
- Fix all ansible-lint violations (473 total)

### Changed
- Run CI workflows in Debian trixie container with apt packages
- Upgrade actions/checkout from v4 to v6 for Node.js 24 support

## [0.2.0] - 2026-03-24

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
- Merged rps-backup into sysops (restic_client, restic_server roles)
- New `bash_shell` role for Bash shell configuration
- New `zsh_shell` role for Zsh shell configuration
- New `network_diagnostics` role for network diagnostic tools
- New independent roles split from `common`: `hostname`, `locales`, `timezone`, `keyboard`, `sysctl_tweaks`, `microcode`, `root_user`, `ssh_agent`
- New `tools`, `storage`, and `firmware` roles (replacing `debian_packages`)
- Powerline-go prompt integration in shell roles
- Hostname assertion and configurable hostname variable
- Per-file SPDX license headers
- Managed file headers in all templates (replacing `ansible_managed`)
- Incus-based test system

### Changed
- Relicensed from Apache-2.0 to AGPL-3.0-or-later
- Split `common` role into 12 independent roles with meta dependencies
- Renamed `fish` role to `fish_shell`
- Renamed `debian_sources` role to `debian_apt_sources`
- Renamed `network` role to `resolvconf` and removed it from `common` dependencies
- Replaced `debian_packages` role with `tools`, `storage`, and `firmware` roles
- Changed default root user shell to bash
- Removed `ssh_agent` from `common` role dependencies
- Changed `updates` role to use meta dependency on `debian_apt_sources` instead of import_role
- Deploy system switched to run-parts for modular script execution
- Reorganized checker file structure: moved files from global `files/` directory to their appropriate roles

### Removed
- Removed `elvish_shell` role (no global config option available)
- Removed obsolete `debian_repos` role (refactored into `debian_apt_sources`)

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

[0.2.0]: https://github.com/osahris/cute-devops/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/osahris/cute-devops/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/osahris/cute-devops/compare/v0.0.3...v0.1.0
[0.0.3]: https://github.com/osahris/cute-devops/compare/v0.0.2...v0.0.3
[0.0.2]: https://github.com/osahris/cute-devops/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/osahris/cute-devops/releases/tag/v0.0.1