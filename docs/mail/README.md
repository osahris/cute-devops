<!--
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Mailserver setup

The `osahris.cute_devops` collection deploys a complete mailserver stack: [postfix](../../roles/postfix/) as the MTA (SMTP on 25, SMTPS on 465, optionally submission on 587), [dovecot](../../roles/dovecot/) for IMAP and local delivery over LMTP, DKIM signing and DMARC verification via the [opendkim](../../roles/opendkim/) and [opendmarc](../../roles/opendmarc/) milters (pulled in by the postfix role by default), and optionally [sympa](../../roles/sympa/) for mailing lists. The target platform is Debian trixie.

The guides use `example.org` as the mail domain throughout. Two setups are documented:

- [Single-server setup](single-server.md) — the whole stack on one host, `mail.example.org`. The topology to start with.
- [Multi-server setup](multi-server.md) — the stack split across `mx` (inbound exchange), `mo` (submission/outbound), `mb` (mailboxes) and `ml` (mailing lists).

## Prerequisites

- Debian trixie host(s) with public IPv4 (and ideally IPv6) addresses, reachable on the ports their role needs (25, 465, 587, 993) — and outbound port 25 not blocked by the provider.
- Root access for Ansible (`ansible_user=root` or become).
- Control over the `example.org` DNS zone, and the ability to set reverse DNS (PTR) for the hosts' addresses in your provider's UI.
- The collection installed: `ansible-galaxy collection install osahris.cute_devops`.

## Trying it out first

The repository ships a container harness that deploys both of these exact topologies into rootless podman system containers and asserts SMTP/IMAP mail flow end to end — a fast way to try a configuration before touching a real host: `./test-in-containers-single.yaml` and `./test-in-containers-multi.yaml`, see [test/README.md](../../test/README.md). The same [test_mail_stack](../../roles/test_mail_stack/) role can be pointed at a deployed server to assert its units, ports and a full send/receive round trip.
