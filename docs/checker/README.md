<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# setup_check

A comprehensive monitoring and alerting system that combines local checks with centralized monitoring infrastructure, deployed via Ansible.

## Overview

This Ansible collection provides a complete monitoring solution with two main components:

### Local Monitoring (Client Side)
- Uses systemd timers for reliable scheduling
- Runs standard Nagios monitoring plugins
- Sends notifications via email (sendmail) and Alerta API
- Provides resource limits and security hardening per check
- Includes a real-time monitoring dashboard

### Monitoring Infrastructure (Server Side)
- **Uptime Kuma**: External network monitoring and HTTP/HTTPS endpoint checks
- **Netdata**: Real-time resource usage tracking and performance metrics
- **Alerta**: Centralized alert aggregation, acknowledgment, and message propagation

## Features

### Local Monitoring Features
- **Modular Design**: Easy to add new checks and notification methods
- **Production Ready**: Includes error handling, timeouts, and resource limits
- **Systemd Integration**: Leverages systemd for scheduling, logging, and process management
- **Nagios Compatible**: Works with thousands of existing monitoring plugins
- **Multiple Notifiers**: Email and Alerta support with parallel execution
- **Resource Control**: Per-check CPU, memory, and timeout limits
- **Security**: Runs with minimal privileges using systemd sandboxing

### Infrastructure Features
- **External Monitoring**: Uptime Kuma provides beautiful web UI for HTTP/HTTPS/TCP/Ping monitoring
- **Performance Metrics**: Netdata offers real-time, per-second metrics with minimal overhead
- **Alert Aggregation**: Alerta consolidates alerts from all sources into a single pane of glass
- **Alert Management**: Support for acknowledgment, deduplication, and custom routing
- **Multi-Tenant**: Support for multiple environments and teams
- **API-First**: All infrastructure components provide REST APIs for automation

## Quick Start

### Local Monitoring Deployment
```bash
# Deploy local monitoring checks to a host
ansible-playbook local.yaml

# View local monitoring dashboard
checker-monitor

# Check systemd timers
systemctl list-timers 'checker-*'
```

### Infrastructure Deployment
```bash
# Deploy monitoring infrastructure components
ansible-playbook infrastructure.yaml

# Access monitoring services:
# - Uptime Kuma: http://your-server:3001
# - Netdata: http://your-server:19999
# - Alerta: http://your-server:8080
```

## Architecture

### Local Monitoring Architecture
The local monitoring system consists of:
- **Checks**: Individual monitoring scripts in `/etc/checker/checks/`
- **Timers**: Systemd timers that trigger checks on schedule
- **Notifiers**: Scripts that send alerts via various channels
- **Output Files**: Check results stored in `/run/checker/`

### Infrastructure Architecture
The centralized monitoring infrastructure provides:
- **Uptime Kuma**: Web-based external monitoring dashboard for HTTP/HTTPS/TCP/Ping checks
- **Netdata**: High-resolution performance monitoring with real-time metrics and alerting
- **Alerta**: Central alert management console that aggregates alerts from all sources (local checks, Netdata, etc.)

All components are designed to work together, with local checks sending alerts to Alerta for centralized management.

## Available Roles

### Local Monitoring Roles

#### Core
- `setup_check` - Base infrastructure and scripts
- `check` - Generic role for deploying checks

#### Checks
- `check_systemd` - Monitor failed systemd units
- `check_disk` - Disk space monitoring
- `check_ram` - Memory usage monitoring
- `check_ping` - Network connectivity checks

#### Meta Roles
- `managed` (with `managed_with_system_checks`) - Groups system-related checks
- `managed` (with `managed_with_disk_checks`) - Automatically monitors all mount points
- `managed` (with `managed_with_network_checks`) - Network monitoring checks

#### Notifiers
- `notify_email` - Email notifications via sendmail
- `notify_alerta` - Alerta API integration

### Infrastructure Roles

#### Monitoring Services
- `uptimekuma` - Deploy Uptime Kuma for external network monitoring
- `netdata` - Deploy Netdata for resource usage tracking and performance metrics
- `alerta` - Deploy Alerta for alert aggregation and management

#### Integration
- `monitoring_infrastructure` - Meta role to deploy all monitoring infrastructure components

## Configuration

Each check can be configured with:
- **Scheduling**: Via systemd timer (minutely, hourly, etc.)
- **Resource Limits**: CPU quota, memory max, timeout
- **Thresholds**: Warning and critical levels
- **Custom Variables**: Check-specific settings

Example check configuration in `check.env`:
```bash
CHECK_TIMEOUT=30
CHECK_MEMORY_MAX=50M
CHECK_CPU_QUOTA=10%
CHECK_LIMIT_NOFILE=1024
CHECK_LIMIT_NPROC=32
```

## Documentation

See the `/docs` directory for:
- `install.md` - Installation guide
- `configuration.md` - Detailed configuration reference
- `performance.md` - Performance tuning guide

## License

This collection is provided as-is for production use.