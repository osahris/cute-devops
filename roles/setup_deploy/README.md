<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# setup_deploy

Main role for the deploy system that provides the core deployment infrastructure - installs systemd units, wrapper scripts, and the deploy command to manage deployments.

## Requirements

- Debian-based operating system
- Systemd init system

## Role Variables

See `defaults/main.yaml` for available variables.

`setup_deploy_polkit_group` — a Unix group allowed to
`systemctl start deploy@<id>.service` without authentication, via a polkit
rule (`/etc/polkit-1/rules.d/50-deploy.rules`). This is the bridge that lets
an unprivileged `post-receive` hook trigger a deploy (see the
[Push to Deploy 🚀](../../patterns/operation/deployment/push-to-deploy.md)
pattern and the `repos` role's `with_deploy`). `start` only — stop/restart
still need admin auth. Empty (the default) installs no rule.

## Dependencies

None.

## Example Playbook

```yaml
- hosts: servers
  roles:
     - mkbrechtel.devops.setup_deploy
```

## License

Apache-2.0

## Author Information

This role was created for the mkbrechtel.devops collection.