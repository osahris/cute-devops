<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Performance Tuning Guide

## Overview

The setup_check monitoring system is designed to be lightweight and efficient. This guide covers optimization techniques for different deployment scenarios.

## Resource Limits

### Per-Check Resource Control

Configure resource limits via environment variables in `/etc/checker/checks/<check_id>/check.env`:

```bash
# Resource limits in check.env
CHECK_MEMORY_MAX="200M"      # Default: 100M
CHECK_CPU_QUOTA="50%"        # Default: 20%
CHECK_TIMEOUT=120            # Default: 60 seconds
```

### System-Wide Limits

Default resource limits are applied via check.env variables:

- **Memory**: 100MB per check (via CHECK_MEMORY_MAX)
- **CPU**: 20% quota per check (via CHECK_CPU_QUOTA)
- **Timeout**: 60 seconds (via CHECK_TIMEOUT)
- **File Descriptors**: 1024 (systemd default)
- **Processes**: 32 (systemd default)

## Timer Optimization

### Spreading Check Execution

Prevent all checks from running simultaneously:

```yaml
- name: Deploy checks with randomized delays
  include_role:
    name: check
  vars:
    check_id: "{{ item }}"
    check_timer_randomized_delay: "30s"  # Random 0-30s delay
  loop:
    - check1
    - check2
    - check3
```

### Power Saving

For battery-powered or energy-conscious deployments:

```yaml
check_timer_accuracy: "1m"  # Allow up to 1 minute delay for batching
```

### Staggered Schedules

Use different timer patterns to spread load:

```yaml
# Different minute offsets
- name: Deploy staggered checks
  include_role:
    name: check
  vars:
    check_id: "check_{{ item.name }}"
    check_timer: "*:{{ item.offset }}/5"  # Every 5 minutes at different offsets
  loop:
    - { name: "web", offset: "00" }
    - { name: "db", offset: "01" }
    - { name: "cache", offset: "02" }
```

## Check Optimization

### Timeout Configuration

Set appropriate timeouts for different check types:

```bash
# Fast local checks
CHECK_TIMEOUT=10

# Network checks
CHECK_TIMEOUT=30

# Complex checks
CHECK_TIMEOUT=120
```

### Parallel Execution

The notification system already runs notifiers in parallel. For checks:

```yaml
# Use systemd targets for grouping
- name: Create check group target
  copy:
    content: |
      [Unit]
      Description=All database checks
      Wants=checker-db_primary.service checker-db_replica.service
    dest: /etc/systemd/system/checker-database.target
```

### Check Script Optimization

Write efficient check scripts:

```bash
#!/bin/bash
# Bad - Multiple calls
disk_usage=$(df -h / | tail -1 | awk '{print $5}' | sed 's/%//')
disk_free=$(df -h / | tail -1 | awk '{print $4}')

# Good - Single call
read -r disk_usage disk_free <<< $(df -h / | tail -1 | awk '{print $5 " " $4}')
```

## Notification Optimization

### Rate Limiting

Configure aggressive rate limiting for non-critical environments:

```bash
# In notify_email.env
NOTIFY_EMAIL_RATE_LIMIT=5        # Only 5 emails per hour
NOTIFY_EMAIL_RATE_WINDOW=3600
```

### Conditional Notifications

Skip notifications during maintenance:

```bash
# /etc/checker/maintenance.flag
touch /etc/checker/maintenance.flag

# In notifier script
if [ -f /etc/checker/maintenance.flag ]; then
    exit 0
fi
```

### Batched Notifications

For high-volume environments, batch notifications:

```bash
# Custom batching notifier
#!/bin/bash
BATCH_FILE="/var/run/checker/notification_batch"
BATCH_INTERVAL=300  # 5 minutes

# Add to batch
echo "$(date -Iseconds) $1 $2 $3" >> "$BATCH_FILE"

# Send batch if interval passed
if [ -f "$BATCH_FILE.last" ]; then
    last_sent=$(stat -c %Y "$BATCH_FILE.last")
    now=$(date +%s)
    if [ $((now - last_sent)) -gt $BATCH_INTERVAL ]; then
        # Send batched notifications
        send_batch < "$BATCH_FILE"
        > "$BATCH_FILE"
        touch "$BATCH_FILE.last"
    fi
fi
```

## System Tuning

### Kernel Parameters

For high-frequency monitoring:

```bash
# /etc/sysctl.d/99-checker.conf
# Increase inotify limits
fs.inotify.max_user_watches = 524288
fs.inotify.max_user_instances = 512

# Network tuning for many ping checks
net.ipv4.ping_group_range = 0 2147483647
```

### SystemD Tuning

```bash
# /etc/systemd/system.conf.d/checker.conf
[Manager]
DefaultTasksMax=4096
DefaultLimitNOFILE=65535
DefaultTimeoutStartSec=90s
```

### Journal Limits

Prevent journal spam from high-frequency checks:

```bash
# /etc/systemd/journald.conf.d/checker.conf
[Journal]
RateLimitIntervalSec=30s
RateLimitBurst=1000
MaxRetentionSec=7d
```

## Monitoring Performance

### Check Execution Time

Monitor check performance:

```bash
# Show slowest checks
journalctl -u 'checker-*.service' --since today | \
  grep "Duration:" | \
  sort -k2 -n -r | \
  head -10
```

### Resource Usage

```bash
# CPU usage by check
systemd-cgtop /system.slice/checker-*.service

# Memory usage
systemctl status 'checker-*.service' | grep Memory
```

### Timer Accuracy

```bash
# Check timer delays
systemctl list-timers 'checker-*' --all
```

## Scaling Considerations

### Large Deployments (100+ checks)

1. Use hourly/daily timers for non-critical checks
2. Increase randomized delays to 60s+
3. Consider multiple checker instances
4. Use external monitoring for the monitoring system

### Resource-Constrained Systems

1. Reduce check frequency
2. Lower memory limits (50M)
3. Increase timer accuracy for batching
4. Disable non-essential checks

### High-Frequency Requirements

1. Use sub-minute timers carefully
2. Optimize check scripts for speed
3. Consider direct service triggers instead of timers
4. Monitor for timer queue buildup

## Best Practices

1. **Start Conservative**: Begin with longer intervals and tighten as needed
2. **Monitor the Monitors**: Track checker resource usage
3. **Profile Before Optimizing**: Identify actual bottlenecks
4. **Use Appropriate Check Types**: Local vs. network vs. complex
5. **Regular Cleanup**: Remove obsolete checks and notifications