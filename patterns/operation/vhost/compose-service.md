---
title: Compose Service Pattern 🐋
---

<!--
SPDX-FileCopyrightText: 2023-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## Overview 📋

How to structure and manage a docker-compose-driven app inside a
[vhost directory 🏠](./vhost-directory.md). The vhost dir *is* the
service's home; the compose file lives at its root and is run from
there. The pattern's name says "service" because that's compose's own
term for the `services:` block — it's distinct from our vhost (the
running thing on disk).

## Goals 🎯

- Consistent compose layout across vhosts
- Predictable locations and configurations
- Easy backup (`data/` is the trustworthy source)
- Secure defaults: pinned images, read-only config, no socket access

## Directory Structure 📂

```text
/srv/vhosts/<fqdn>/         # the vhost directory
├── .git/                   # push target (see vhost-directory pattern)
├── compose.yaml            # main compose file
├── compose.override.yaml   # local overrides (proxy network, …)
├── .env                    # env vars (in .git/info/exclude)
├── VHOST.md                # what this vhost is, who runs it
├── config/                 # read-only mounted into the container
├── data/                   # persistent state — backup target
├── deploy/                 # post-push hooks (compose pull, up -d)
└── .gitignore              # compose.override.yaml, VHOST.md, …
```

`<fqdn>` is the vhost's fully-qualified hostname (e.g.
`www.example.com`). Owned by a per-vhost Unix user named the same.

## Base Requirements 🛠️

- Debian 13 (trixie)
- `docker.io` and `docker-compose` from the trixie repository, via apt
- Regular system updates via `unattended-upgrades`

## Setting up a compose vhost 📝

### Install Docker

```bash
sudo apt update
sudo apt install docker.io docker-compose apparmor
```

### Create the vhost

Use [`mkbrechtel.devops.vhosts`](../../../roles/vhosts/README.md) to
provision the vhost directory + Unix user + post-receive hook + the
`deploy-vhost@.service` template + polkit grant. After it runs you can
push from anywhere:

```bash
git remote add vhost/www.example.com ssh://host/srv/vhosts/www.example.com
git push vhost/www.example.com main:deploy
```

### compose.yaml (committed, in the push)

```yaml
services:
  app:
    image: application:1.2.3        # pin a tag, not :latest
    restart: unless-stopped
    environment:
      - TZ=UTC
    env_file: .env
    volumes:
      - ./config:/config:ro
      - ./data:/data

networks:
  default:
    driver: bridge
```

### compose.override.yaml (not committed; local to the vhost)

For host-Traefik integration (see
[Host-wide Traefik 🌐](../deployment/host-traefik.md)):

```yaml
networks:
  proxy:
    external: true

services:
  app:
    networks:
      - proxy
      - default
```

### deploy/ scripts (committed, in the push)

The push fast-forwards the working tree, then runs `deploy/` via
run-parts. A typical compose deploy:

```sh
# deploy/10-pull
#!/bin/sh
exec docker-compose pull
```

```sh
# deploy/20-up
#!/bin/sh
exec docker-compose up -d --remove-orphans
```

### .gitignore (committed)

Mark local files that should *not* travel in the push:

```text
compose.override.yaml
VHOST.md
```

`.env` is already kept out by the vhost's `.git/info/exclude` (set by
the role at init time) — it stays local to each vhost and never enters
git's view.

## Security Practices 🔐

- Pin image tags (`:1.2.3`, never `:latest`)
- Read-only `config/` mount, writable `data/` only where needed
- Run containers as a non-root user inside the image
- No `docker.sock` exposure unless absolutely required
- Only join the proxy network from services that need external traffic

## Operations 🔄

### Manual start / stop (operator at the vhost)

```bash
sudo -u <fqdn> bash -c '
  cd /srv/vhosts/<fqdn>
  docker-compose up -d
'
```

Normal updates happen via push — the deploy scripts run `pull` and
`up -d` automatically.

### Inspecting the deploy

```bash
systemctl status deploy-vhost@<fqdn>.service
journalctl -u deploy-vhost@<fqdn>.service
git -C /srv/vhosts/<fqdn> tag --list 'deployed-to-*'
```

## Anti-patterns ⚠️

- ❌ Using the `latest` tag
- ❌ Committing secrets to `compose.yaml` or `.env`
- ❌ Directly editing files in `data/` instead of through the app
- ❌ Joining services to the proxy network when they don't expose HTTP
- ❌ Running `docker-compose` from outside the vhost dir (loses the
  `compose.override.yaml` and `.env`)

## Tips 💡

- Document the vhost's quirks in `VHOST.md` (not tracked)
- Use `.env` for per-deployment configuration (DB passwords, secrets,
  per-environment endpoints)
- Enable container health checks; let compose restart on failure
- Keep `compose.override.yaml` for host-local concerns (proxy network,
  bind mounts, ports) so the committed `compose.yaml` stays portable

## Related Patterns 🔗

- [Vhost Directory 🏠](./vhost-directory.md) — the directory layout
  and push protocol this pattern lives inside.
- [Host-wide Traefik 🌐](../deployment/host-traefik.md) — the proxy
  network the `compose.override.yaml` plugs into.
- [Push to Deploy 🚀](../deployment/push-to-deploy.md) — the
  push-is-the-deploy idea this realises.
