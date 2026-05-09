---
title: Host-wide Traefik Container Pattern 🌐
---
Host-based Traefik pattern provides centralized HTTPS termination and routing for services on a single host without requiring Docker socket access.

## Key Components

- Shared Docker proxy network for service connectivity
- File-based configuration in /etc/traefik/conf.d/
- Per-service configuration files
- Automatic Let's Encrypt certificate management
- HTTP to HTTPS redirection

## Directory Structure

```bash
/etc/traefik/
├── docker-compose.yaml    # Traefik compose configuration
├── traefik.yaml           # Main Traefik configuration
└──  conf.d/                # Service configurations
      └── {hostname}.yaml   # Per-service routing rules
```

## Usage

1. Create the proxy network:
```bash
docker network create proxy
```

2. Add service to proxy network in compose.yml:
```yaml
networks:
  proxy:
    external: true
```

3. Configure routing in `/etc/traefik/conf.d/{hostname}.yaml`:
```yaml
http:
  routers:
    myservice:
      entrypoints: websecure
      rule: Host(`service.domain.com`)
      service: myservice
  services:
    myservice:
      loadBalancer:
        servers:
          - url: http://container_name:port
```

## Security Features

- No Docker socket access required
- File-based configuration
- Automatic HTTPS
- Network isolation via proxy network
- TLS certificate management

For detailed setup instructions see the [Docker/Podman Compose Service Pattern](./compose-service) documentation.
