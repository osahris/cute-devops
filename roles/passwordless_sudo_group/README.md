<!--
SPDX-FileCopyrightText: 2026 Alexander Hirsch <Hirsch@med.uni-frankfurt.de>

SPDX-License-Identifier: EUPL-1.2
-->

# passwordless_sudo_group Role

Creates a group like the default "sudo" group, which unlike "sudo" allows **passwordless** access to become root.

## Requirements

- Ansible
- Debian

## Role Variables

```yaml
passwordless_sudo_group:
  passwordless_sudo_group_name: "nopasswd-sudo" # name of the passwordless sudo group
  passwordless_sudo_group_enable: true          # true: add group, false: remove group
```

Defaults (see `defaults/main.yml`):

- `passwordless_sudo_group_name: "nopasswd-sudo"`
- `passwordless_sudo_group_enable: false`

## Example

```yaml
- hosts: village
  become: true
  roles:
    - role: mkbrechtel.devops.passwordless_sudo_group
      vars:
        passwordless_sudo_group_enable: true
```

## License

EUPL-1.2
