<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# managed

High-level orchestrator role that composes checks, deploys, notifications, and updates into a fully managed system. Uses feature flags to control what gets enabled.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

### Feature Flags

| Variable | Default | Description |
|---|---|---|
| `managed_with_disk_checks` | `false` | Enable disk space checks for all mounts |
| `managed_with_network_checks` | `false` | Enable network connectivity checks |
| `managed_with_system_checks` | `false` | Enable systemd and RAM checks |
| `managed_with_updates` | `false` | Enable automatic updates |
| `managed_with_notify_email` | `false` | Enable email notifications |

### Disk Checks Configuration

| Variable | Default | Description |
|---|---|---|
| `managed_disk_checks_excluded_filesystems` | (see defaults) | Filesystem types to exclude |
| `managed_disk_checks_excluded_mounts` | `[]` | Mount points to explicitly exclude |
| `managed_disk_checks_minimum_size_mb` | `128` | Minimum mount size in MB to monitor |

## Dependencies

- `setup_check` (automatic via meta dependency)

## Example Playbook

```yaml
- hosts: servers
  become: true
  roles:
    - role: osahris.cute_devops.managed
      vars:
        managed_with_disk_checks: true
        managed_with_network_checks: true
        managed_with_system_checks: true
        managed_with_updates: true
        managed_with_notify_email: true
```

## License

EUPL-1.2

This role was created for the osahris.cute_devops collection.
