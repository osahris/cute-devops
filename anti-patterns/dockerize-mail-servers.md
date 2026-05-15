---
title: Don't dockerize mail servers 🔻📬
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

## The Pitch 📦

> *Everything else is in containers, so the mail server should be too —
> reproducible builds, declarative config, easy upgrades.*

## Why It's Tempting 🍬

- One deployment story across the fleet.
- Pinned versions feel safer than "whatever the distro ships".
- The README of every modern mail stack starts with `docker run`.

## Why It's Not Cute 🔻

Mail servers are some of the most boring, least-changing software on
your hosts. Postfix, Dovecot and Rspamd have been doing the same job
for decades; the Debian packages get small, well-tested security
updates and otherwise sit still for years.

Wrapping them in a container buys you very little and costs:

- **A second packaging stack** to keep up with — base image rebuilds,
  CVE scans, glibc compatibility — on top of the actual MTA.
- **Networking gymnastics**: SMTP wants real DNS, real IPs, real
  reverse DNS, and intact PTR records. Bridge networks, NAT and
  rootless port maps make every troubleshooting session harder.
- **Persistent state in awkward places**: `/var/mail`, `/var/spool`,
  Sieve scripts, TLS certs, queue files, DKIM keys. Volumes are easy
  to misplace; bind-mounts to host paths reintroduce the host you
  were trying to abstract over.
- **Logging and metrics drift away from the rest of the host**:
  journald, fail2ban, log rotation and your existing monitoring all
  speak host conventions.

You're paying container-tax on a workload whose update cadence is
"once a year if that".

## The Cute Alternative 💙

Run a plain **Debian mail server** managed by Ansible. Pin the
distro, let `unattended-upgrades` ship the security patches, and
keep the configuration in version control. The MTA blends in with
the rest of the host: same logs, same firewall, same backup story.

If you want reproducibility, that's what configuration management
is for — not a container image whose only job is to package a
package.

## Related 🔗

- [Compose Service Pattern](../patterns/operation/vhost/compose-service.md) — the
  pattern this is the anti-pattern of, *for stateless services*.
- [Stages Pattern](../patterns/operation/deployment/stages.md) — for software that *does*
  benefit from containerized stages, mail servers aren't on the list.
