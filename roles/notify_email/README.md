<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
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

- `mkbrechtel.devops.setup_check`

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.notify_email
      vars:
        notify_email_to: admin@example.com
```
