---
# SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-License-Identifier: EUPL-1.2
#
# Sidebar grouping for the Roles section of the Cute DevOps! website.
# The YAML front matter is the data; the body is informational only.
groups:
  - name: Orchestrators
    roles:
      - managed
      - common
  - name: System
    roles:
      - hostname
      - locales
      - timezone
      - keyboard
      - sysctl_tweaks
      - microcode
      - firmware
      - storage
      - resolvconf
      - debian_apt_sources
      - tools
      - root_user
      - users
      - updates
      - motd
  - name: Shells
    roles:
      - bash_shell
      - fish_shell
      - zsh_shell
  - name: Containers
    roles:
      - podman
  - name: Mail
    roles:
      - mailname
      - postfix
      - dovecot
      - sympa
      - opendkim
      - opendmarc
      - postfixadmin
      - rainloop
      - certificate
  - name: Backup
    roles:
      - restic_client
      - restic_server
  - name: Monitoring
    roles:
      - check
      - check_disk
      - check_ping
      - check_ram
      - check_systemd
      - setup_check
      - notify_alerta
      - notify_email
      - setup_notify
      - network_diagnostics
  - name: Deployment
    roles:
      - deploy
      - deploy_ansible_play
      - deploy_ansible_pull
      - setup_deploy
      - triggered_by_git_hook
      - webhook_server
      - test_deploy_ohai
      - test_deploy_fail
  - name: Repositories
    roles:
      - repos
  - name: Tooling
    roles:
      - ansible
      - ssh_agent
      - ttyd
---

# Role groups

Edit this file to change how Ansible roles are organized in the sidebar
of the Cute DevOps! website. Only the YAML front matter is consumed;
the body of the file is ignored.

Each group needs a `name` (the sidebar header label) and a `roles` list
(role directory names under `roles/`). Roles not listed here will not
show up in the sidebar — they're still reachable at `/roles/<name>`.
