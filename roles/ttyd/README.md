<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# ttyd

Installs [ttyd](https://github.com/tsl0922/ttyd), a web-based terminal, by downloading a pre-built binary from GitHub releases.

## Requirements

- Debian 12/bookworm or 13/trixie

## Role Variables

See `defaults/main.yaml` for all available variables and their default values.

Key variables:

- `ttyd_setup_method` (default: `"download"`) - Installation method
- `ttyd_version` (default: `"1.7.7"`) - Version to install (GitHub release tag without "v")
- `ttyd_github_repo_url` (default: `"https://github.com/tsl0922/ttyd"`) - GitHub repository URL
- `ttyd_arch` - Architecture identifier, auto-detected from `ansible_architecture`
- `ttyd_install_dir` (default: `"/opt/ttyd"`) - Directory for the downloaded binary
- `ttyd_with_usr_local_bin_symlink` (default: `false`) - Create a symlink at `/usr/local/bin/ttyd`

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  become: yes
  roles:
    - role: mkbrechtel.devops.ttyd
      vars:
        ttyd_with_usr_local_bin_symlink: true
```
