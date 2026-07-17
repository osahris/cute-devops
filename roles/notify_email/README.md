<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# notify_email

Email notification integration for check results with rate limiting.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `notify_email_enabled` (default: `true`) - Enable/disable email notifications
- `notify_email_to` (default: `'root@localhost'`) - Recipient email address
- `notify_email_rate_limit` (default: `10`) - Max emails per hour per check

## Dependencies

- `osahris.cute_devops.setup_check`

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.notify_email
      vars:
        notify_email_to: admin@example.com
```
