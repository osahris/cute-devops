# Rename roles for consistent naming scheme

## Problem

Role naming is inconsistent — `checker_check_*`, `deploy_deploy`, mixed
prefixes. The collection doesn't feel like one cohesive piece.

## Naming Convention

Four categories of roles:

- **Setup roles** (`setup_<system>`): Install packages, shared config, systemd
  unit templates. E.g. `setup_check`, `setup_deploy`, `setup_notify`.
- **Instance roles** (`<system>_<type>`): Configure one systemd unit instance.
  E.g. `check_disk` → `check@disk`, `deploy_ansible_play` → `deploy@ansible-play`.
- **Orchestrator role** (`managed`): Single high-level role that composes checks,
  deploys, notifications, and updates into a fully managed system. Uses `_with_`
  feature flags to control what gets enabled.
- **Test roles** (`test_<what>`): E2E test/demo environment.

## Rename Mapping

### Setup
- `checker` → `setup_check`
- `deploy_deploy` → `setup_deploy`
- (new) `setup_notify` — unified notify setup for all roles

### Check instances
- `checker_check` → `check`
- `checker_check_disk` → `check_disk`
- `checker_check_memory` → `check_ram`
- `checker_check_ping` → `check_ping`
- `checker_check_systemd` → `check_systemd`

### Notify instances
- `checker_notify_alerta` → `notify_alerta`
- `checker_notify_email` → `notify_email`

### Deploy instances
- `deploy_instance` → `deploy`
- `deploy_ansible_play` → `deploy_ansible_play` (unchanged)
- `deploy_ansible_pull` → `deploy_ansible_pull` (unchanged)

### Orchestrator
- `checker_disk_checks` → `managed` (with `managed_with_disk_checks`)
- `checker_network_checks` → `managed` (with `managed_with_network_checks`)
- `checker_system_checks` → `managed` (with `managed_with_system_checks`)

The `managed` role replaces all grouping roles. It composes setup roles,
instance roles, and notifications via feature flags:

```yaml
- role: mkbrechtel.sysops.managed
  vars:
    managed_with_disk_checks: true
    managed_with_network_checks: true
    managed_with_system_checks: true
    managed_with_updates: true
    managed_with_notify_email: true
```

### Triggers
- `deploy_triggered_by_git_hook` → `triggered_by_git_hook` (design TBD)

### Tests
- `deploy_test_fail` → `test_deploy_fail`
- `deploy_test_ohai` → `test_deploy_ohai`

## Open Questions

- `triggered_by_*`: just rename for now, full design later.

## Migration

- No backwards compatibility — old names are removed (0.x.x, unstable).
- Update all internal references, playbooks, docs, and README.
