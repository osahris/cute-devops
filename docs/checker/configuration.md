<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Configuration Guide

## Overview

The setup_check monitoring system uses a layered configuration approach:

1. **Ansible Variables** - Set during deployment
2. **Environment Files** - Runtime configuration
3. **SystemD Units** - Service and timer settings

## Ansible Variables

### Global Configuration

Set in your playbook or inventory:

```yaml
# Notification settings
notify_email_to: "ops@example.com"
notify_email_from: "checker@{{ ansible_hostname }}"
notify_email_subject_prefix: "[PROD-ALERT]"

# Alerta settings
notify_alerta_api_alert_url: "https://alerta.example.com/api/alert"
notify_alerta_api_key: "your-api-key"
notify_alerta_environment: "Production"

# Default check settings
check_timeout: 60  # seconds
check_timer: "minutely"  # or "hourly"
```

### Check-Specific Variables

#### Disk Monitoring

```yaml
check_disk_warning_threshold: 80   # Percentage
check_disk_critical_threshold: 90  # Percentage
check_disk_path: "/var"           # Mount point to monitor
```

#### Memory Monitoring

```yaml
check_ram_warning_threshold: 80   # Percentage
check_ram_critical_threshold: 90  # Percentage
```

#### Ping Monitoring

```yaml
check_ping_hostname: "8.8.8.8"
check_ping_timeout: 5
check_ping_packets: 5
```

## Environment Files

### Notifier Configuration

#### Email (`/etc/checker/notify_email.env`)

```bash
# Email settings
NOTIFY_EMAIL_TO="ops@example.com"
NOTIFY_EMAIL_FROM="checker@hostname"
NOTIFY_EMAIL_SUBJECT_PREFIX="[ALERT]"

# Email is sent via sendmail only (no SMTP configuration needed)

# Notification triggers
NOTIFY_EMAIL_ON_WARNING=true
NOTIFY_EMAIL_ON_CRITICAL=true
NOTIFY_EMAIL_ON_RECOVERY=true

# Output settings
NOTIFY_EMAIL_INCLUDE_OUTPUT=true
NOTIFY_EMAIL_MAX_OUTPUT_LINES=50

# Rate limiting
NOTIFY_EMAIL_RATE_LIMIT=10      # Max emails per hour
NOTIFY_EMAIL_RATE_WINDOW=3600   # Window in seconds
```

#### Alerta (`/etc/checker/notify_alerta.env`)

```bash
# Alerta API settings
ALERTA_API_ALERT_URL="https://alerta.example.com/api/alert"
ALERTA_API_KEY="your-api-key"
ALERTA_ENVIRONMENT="Production"

# Optional settings
ALERTA_TIMEOUT=10
ALERTA_SSL_VERIFY=true
```

### Check Configuration

Each check can have its own environment file at `/etc/checker/checks/<check_id>/check.env`:

```bash
# Resource limits (used by systemd service)
CHECK_TIMEOUT=60
CHECK_MEMORY_MAX=100M
CHECK_CPU_QUOTA=20%

# Process limits
CHECK_LIMIT_NOFILE=1024
CHECK_LIMIT_NPROC=32

# Check-specific variables
DISK_MOUNT="/var/log"
WARNING_THRESHOLD=75
CRITICAL_THRESHOLD=85
```

## SystemD Configuration

### Timer Configuration

Modify check frequency by editing `/etc/systemd/system/checker-<check_id>.timer`:

```ini
[Timer]
OnCalendar=*:0/5  # Every 5 minutes
Persistent=true

# Or use shortcuts
OnCalendar=minutely
OnCalendar=hourly
OnCalendar=daily
```

### Service Configuration

Resource limits are now configured via environment variables in `/etc/checker/checks/<check_id>/check.env`:

```bash
# Resource limits (set in check.env)
CHECK_TIMEOUT=300        # Timeout in seconds
CHECK_MEMORY_MAX="100M"  # Memory limit
CHECK_CPU_QUOTA="20%"    # CPU quota
```

These variables are automatically applied to the systemd service when deployed.

## Advanced Configuration


### Conditional Notifications

Create wrapper scripts for complex logic:

```bash
#!/bin/bash
# /etc/checker/checks/conditional/check.sh

result=$(/usr/lib/nagios/plugins/check_disk -w 80 -c 90 -p /)
exit_code=$?

# Only alert during business hours
hour=$(date +%H)
if [ $exit_code -ne 0 ] && [ $hour -ge 9 ] && [ $hour -lt 17 ]; then
    echo "$result"
    exit $exit_code
else
    echo "Check failed but outside business hours"
    exit 0
fi
```

### Multi-Environment Setup

Use different variables per environment:

```yaml
# production.yaml
- hosts: production
  vars:
    notify_email_to: "prod-ops@example.com"
    notify_alerta_environment: "Production"
    check_disk_critical_threshold: 95

# staging.yaml  
- hosts: staging
  vars:
    notify_email_to: "dev-ops@example.com"
    notify_alerta_environment: "Staging"
    check_disk_critical_threshold: 85
```

### Dynamic Check Generation

```yaml
# Generate disk checks for all mounts
- name: Get mount points
  shell: df -P | awk 'NR>1 {print $6}'
  register: mount_points

- name: Create disk checks
  include_role:
    name: check_disk
  vars:
    check_id: "disk_{{ item | regex_replace('/', '_') }}"
    check_disk_path: "{{ item }}"
  loop: "{{ mount_points.stdout_lines }}"
  when: item not in ['/dev', '/proc', '/sys']
```

## Configuration Best Practices

1. **Use Ansible Vault** for sensitive data:
   ```bash
   ansible-vault encrypt_string 'your-api-key' --name 'notify_alerta_api_key'
   ```

2. **Test Configuration Changes**:
   ```bash
   # Dry run
   ansible-playbook local.yaml --check
   
   # Test specific check
   systemctl start checker-test.service
   journalctl -f -u checker-test.service
   ```

3. **Version Control Exclusions**:
   ```gitignore
   # .gitignore
   *.env
   /etc/checker/notify_*.env
   vault-password.txt
   ```

4. **Separate Secrets**:
   ```yaml
   # secrets.yaml (encrypted)
   notify_email_smtp_password: !vault |
     $ANSIBLE_VAULT;1.1;AES256
     66383439383...
   ```

5. **Environment-Specific Configs**:
   ```bash
   # Deploy with environment
   ansible-playbook -i production local.yaml
   ansible-playbook -i staging local.yaml
   ```