---
title: Compose Service Pattern 🐋
---

<!--
SPDX-FileCopyrightText: 2023-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## Overview 📋
This pattern describes how to structure and manage services running with docker-compose on Debian stable systems.

## Goals 🎯
- Consistent service deployment structure
- Predictable locations and configurations
- Easy backup and maintenance
- Secure default settings

## Directory Structure 📂
```bash
/srv/{hostname}/          # Root directory for the service
├── compose.yaml          # Main compose file (new standard name)
├── compose.override.yaml # Proxy network integration
├── .env                 # Environment variables
├── .gitignore          # Git ignore file
└── SERVICE.md           # Service documentation
```

## Base Requirements 🛠️
- Debian Stable (current: Bookworm)
- docker.io and docker-compose from debian-stable repository with apt
- Regular system updates via unattended-upgrades

## Manual Service Setup 📝

### System Packages
```bash
sudo apt update
sudo apt install docker.io docker-compose-plugin apparmor
```

### Service Directory
```bash
mkdir /srv/{hostname}
cd /srv/hostname}
```

### Base Compose File
compose.yaml:
```yaml
version: '3.9'

services:
  app:
    image: application:version
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

### Proxy Integration
compose.override.yaml:
```yaml
version: '3.9'

networks:
  proxy:
    external: true

services:
  app:
    networks:
      - proxy
      - default
```

### Git Configuration
If your service is stored in a Git repo, you should ignore the compose.override.yaml and SERVICE.md.
.gitignore:
```
compose.override.yaml
SERVICE.md
```

## Security Practices 🔐
- Use specific version tags
- Restrict directory permissions
- Read-only mounts where possible
- Limit network exposure
- Run services as non-root user
- Only expose necessary services to proxy network

## Operations 🔄

### Start/Stop
```bash
cd /srv/{hostname}
docker-compose up -d    # Start
docker-compose down     # Stop
```

### Updates
```bash
cd /srv/{hostname}
docker-compose pull
docker-compose up -d
```

## Anti-patterns ⚠️
- Using latest tag
- Storing secrets in compose.yaml
- Direct modification of container data
- Exposing services to proxy network unnecessarily

## Tips 💡
- Document service specifics in SERVICE.md
- Use .env-File for configuration
- Enable health checks
- Keep proxy network configuration separate in override file
- On Debian Stable the docker-compose command is based on the old Python compose implementation. Because of this you need to write `docker-compose` and not `docker compose`
