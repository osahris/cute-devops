---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Monitoring

## Goal

Two complementary monitoring paths:

1. **Checker**: script-based checks run as systemd units (already partially
   in place — `check_*`, `notify_*`, `setup_check`, `setup_notify`).
2. **Prometheus / Grafana**: metrics-based observability stack, optionally
   self-hosted or pointing at a central setup.

Alerts from both paths converge in a central Alerta service.

## Scope

### Checker (extend existing)

- Audit existing `check_*` roles, document which checks exist, identify
  gaps (e.g. cert expiry, backup freshness, disk SMART).
- Standardize exit/status conventions.
- Notification routing: email + alerta (exist), consider matrix/slack
  later.

### Prometheus/Grafana

- `setup_prometheus_node_exporter` per host.
- `setup_prometheus` (server): scrape config generated from inventory.
- `setup_grafana` (server): dashboards provisioned as files.
- `setup_alertmanager` → forwards to alerta.
- A host either runs exporters (most hosts) or is the monitoring host
  (one).

### Alerta

- `setup_alerta` for the central alerta instance.
- Already have `notify_alerta` for the checker path; alertmanager also
  forwards here.

## Design notes

- Checker runs without a central server — standalone, systemd-local. Its
  notifications go outbound.
- Prometheus setup is opt-in; small deployments may run only the
  checker.
- Same alerta instance receives from both → single pane of glass.
- Dashboards versioned in-repo.

## Open questions

- Scope of checks to add: what's the minimum viable set for a managed
  host? (Disk, RAM, systemd units, ping, backup age, cert expiry,
  reboot required?)
- Prometheus storage: local TSDB, or push to a remote-write target
  (Grafana Cloud, Victoria Metrics)?
- Grafana auth — basic, OIDC, or behind the RPX with forward-auth?
- Is Loki/logs in scope, or just metrics?
- Do we standardize on one alertmanager instance per prometheus, or
  one central?
