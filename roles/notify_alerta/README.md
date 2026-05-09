<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# notify_alerta

Alerta alerting integration for sending check results to the Alerta monitoring platform.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `notify_alerta_api_alert_url` - Alerta API endpoint
- `notify_alerta_api_key` (required) - API key for Alerta
- `notify_alerta_environment` (default: `'Development'`) - Environment tag for alerts

## Dependencies

- `mkbrechtel.devops.setup_check`

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.notify_alerta
      vars:
        notify_alerta_api_key: your-api-key
        notify_alerta_environment: Production
```
