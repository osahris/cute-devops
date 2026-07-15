<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
SPDX-FileCopyrightText: 2026 Alexander Hirsch <hirsch@med.uni-frankfurt.de>

SPDX-License-Identifier: EUPL-1.2
-->

# Users Role

This role manages user accounts and their home directories on Debian/Ubuntu systems.

## Requirements

- Ansible >= 2.14
- Debian (bookworm, bullseye) or Ubuntu (jammy, focal)
- Root/sudo privileges for user management

## Role Variables

The main variable is `users`, which should be a list of user definitions:

```yaml
users:
  - name: alice
    uid: 1001  # optional
    groups: ['sudo', 'docker']  # optional
    shell: /bin/bash  # optional
    ssh_authorized_keys:  # optional
      - "ssh-rsa AAAAB3..."
    linger: true  # optional, enables systemd linger
```

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: osahris.cute_devops.users
      vars:
        users:
          alice:
            groups: ['sudo', 'docker']
            shell: /bin/bash
            ssh_authorized_keys:
              - "ssh-rsa AAAAB3..."
          bob:
            uid: 1002
            shell: /bin/zsh
            linger: true
```

## Features

- User account creation and management
- Home directory setup
- SSH authorized keys management
- User group management
- Systemd linger configuration
- Support for moving existing user home directories

## License

EUPL-1.2