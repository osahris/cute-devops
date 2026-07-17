<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Installation Guide

## Prerequisites

- Debian/Ubuntu-based system with systemd
- Ansible 2.9+ installed
- Root or sudo access
- Python 3.x

## Quick Installation

```bash
# Clone the repository
git clone <repository-url> /opt/setup_check
cd /opt/setup_check

# Install system dependencies
apt-get update
apt-get install -y \
    ansible \
    monitoring-plugins-basic \
    monitoring-plugins-standard \
    jo \
    mailutils \
    postfix

# Run the installation playbook
ansible-playbook local.yaml
```

## Step-by-Step Installation

### 1. Install Dependencies

```bash
# Update package list
apt-get update

# Install Ansible
apt-get install -y ansible

# Install monitoring plugins
apt-get install -y monitoring-plugins-basic monitoring-plugins-standard

# Install notification dependencies
apt-get install -y jo mailutils postfix

# For Alerta (optional)
apt-get install -y python3-venv python3-dev gcc postgresql nginx
```


### 3. Configure the System

Edit `local.yaml` to match your environment:

```yaml
- name: Deploy setup_check monitoring
  hosts: localhost
  vars:
    # Your email address for notifications
    notify_email_to: "admin@yourdomain.com"
    
    # Alerta configuration (if using)
    notify_alerta_api_alert_url: "http://localhost:8080/alert"
    notify_alerta_environment: "Production"
    
    # Disk monitoring thresholds
    check_disk_warning_threshold: 80
    check_disk_critical_threshold: 90
```

### 4. Deploy the Monitoring System

```bash
# Run the playbook
ansible-playbook local.yaml

# Verify deployment
systemctl list-timers 'checker-*'
```

### 5. Install Monitoring Tools (Optional)

```bash
# Install Alerta CLI
apt-get install -y pipx
pipx install alerta
pipx ensurepath

# Configure Alerta CLI
cat > ~/.alerta.conf << EOF
[DEFAULT]
endpoint = http://localhost:8080
timezone = UTC
EOF
```

## Post-Installation

### Verify Installation

```bash
# Check active timers
systemctl list-timers 'checker-*'

# Run a check manually
systemctl start checker-ping_localhost.service

# Check logs
journalctl -u checker-ping_localhost.service

# View monitoring dashboard
checker-monitor
```

### Configure Email

If you haven't configured postfix during installation:

```bash
# Reconfigure postfix
dpkg-reconfigure postfix

# Test email sending
echo "Test" | mail -s "Test Email" root@localhost
```

### Set Up Additional Checks

Add more monitoring checks as needed:

```yaml
# In your playbook
- name: Monitor external website
  include_role:
    name: check_ping
  vars:
    check_id: "ping_google"
    check_ping_hostname: "8.8.8.8"
```

## Containerized Installation

For Docker/Podman environments:

```dockerfile
FROM debian:bookworm

RUN apt-get update && apt-get install -y \
    systemd \
    systemd-sysv \
    ansible \
    monitoring-plugins-basic \
    monitoring-plugins-standard \
    jo \
    mailutils \
    postfix

COPY . /opt/setup_check
WORKDIR /opt/setup_check

RUN ansible-playbook local.yaml

CMD ["/lib/systemd/systemd"]
```

## Troubleshooting Installation

### Ansible Not Found

```bash
# Debian/Ubuntu
apt-get install -y ansible

# Or via pip
pip3 install ansible
```

### Monitoring Plugins Missing

```bash
# Search for available plugins
apt-cache search monitoring-plugins

# Install specific plugins
apt-get install -y monitoring-plugins-standard
```

### SystemD Issues in Containers

Ensure your container is running with:
- `--privileged` flag
- `/sys/fs/cgroup` mounted
- systemd as PID 1

### Permission Errors

Ensure you're running the playbook as root or with sudo:

```bash
sudo ansible-playbook local.yaml
```