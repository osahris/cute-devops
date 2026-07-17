<!--
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Multi-server setup

The mail stack split across four hosts on a network that lets them reach each other. See the [mailserver overview](README.md) for the stack and prerequisites, and the [single-server setup](single-server.md) for the shared knobs (certificates, mailbox auth) explained in detail.

- `mx.example.org` — mail exchange: inbound postfix, verifies DKIM/DMARC, hands mailbox mail to `mb` over LMTP.
- `mo.example.org` — mailout: authenticated submission (587) and outbound relay; SASL auth is delegated to `mb`'s dovecot over the network.
- `mb.example.org` — mailboxes: dovecot with IMAP for clients, plus LMTP and auth exposed on the network for `mx` and `mo`. No local postfix.
- `ml.example.org` — mailing lists: sympa with its own postfix (sympa writes transport maps and restarts it, so it needs a local MTA).

## Inventory

```ini
[mailservers]
mx.example.org ansible_user=root
mo.example.org ansible_user=root
mb.example.org ansible_user=root
ml.example.org ansible_user=root
```

## Shared configuration

The mail domain, the mailbox set and the certificate paths are shared across the instances in `group_vars/mailservers/mail.yaml`, so `mx`'s virtual maps and `mb`'s dovecot agree on the same users:

```yaml
mailserver_domain_name: example.org
postfix_admin_email: postmaster@example.org

# Each host presents its own certificate.
postfix_certificate_fullchain_file: /etc/letsencrypt/live/{{ inventory_hostname }}/fullchain.pem
postfix_certificate_private_key_file: /etc/letsencrypt/live/{{ inventory_hostname }}/privkey.pem
dovecot_certificate_fullchain_file: /etc/letsencrypt/live/{{ inventory_hostname }}/fullchain.pem
dovecot_certificate_private_key_file: /etc/letsencrypt/live/{{ inventory_hostname }}/privkey.pem

dovecot_auth: passwdfile
dovecot_users:
  - username: postmaster@example.org
    password_hash: "{CRYPT}$6$..."
```

## Per-host configuration

`host_vars/mx.example.org.yaml` — accept mail for the domain and deliver to `mb` over LMTP instead of a local socket:

```yaml
postfix_virtual_mailbox_domains:
  - example.org
postfix_virtual_transport: lmtp:inet:mb.example.org:24
```

`host_vars/mo.example.org.yaml` — submission host; postfix authenticates users against `mb`'s dovecot over the network instead of a local socket:

```yaml
postfix_with_submission_service: true
postfix_with_submission_service_smtpd_sasl_path: inet:mb.example.org:12345
```

`host_vars/mb.example.org.yaml` — dovecot opens its LMTP and auth listeners on the network for `mx` and `mo`, and skips the postfix-owned unix sockets since no postfix runs here:

```yaml
dovecot_lmtp_inet_listener: true
dovecot_auth_inet_listener: true
dovecot_unix_listeners_for_postfix: false
```

`host_vars/ml.example.org.yaml` — sympa plus its local postfix:

```yaml
postfix_with_sympa: true
```

The LMTP and auth listeners on `mb` are unauthenticated network services — keep ports 24 and 12345 restricted to the mail hosts (private network or firewall), never open to the internet.

## Playbook

Each host pattern gets its play, mirroring `test-in-containers-multi.yaml`:

```yaml
- hosts: mb.example.org
  become: true
  roles:
    - role: osahris.cute_devops.dovecot
      tags: dovecot

- hosts: mx.example.org:mo.example.org
  become: true
  roles:
    - role: osahris.cute_devops.postfix
      tags: postfix

- hosts: ml.example.org
  become: true
  roles:
    - role: osahris.cute_devops.postfix
      tags: postfix
    - role: osahris.cute_devops.sympa
      tags: sympa
```

## DNS

Inbound mail goes to `mx`, outbound leaves via `mo`, so both appear in the zone:

```
mx.example.org.        IN A      203.0.113.25
mo.example.org.        IN A      203.0.113.26
mb.example.org.        IN A      203.0.113.27
ml.example.org.        IN A      203.0.113.28
example.org.           IN MX     1 mx.example.org.
example.org.           IN TXT    "v=spf1 mx a:mo.example.org -all"
_dmarc.example.org.    IN TXT    "v=DMARC1; p=quarantine"
```

The milters are on by default on every postfix host: `mx` verifies DKIM/DMARC on inbound mail, and `mo` signs what it sends out — so the DKIM records to publish are the ones the run prints on `mo` (default selector `mo`). Set the PTR records for `mx`, `mo` and `ml` to their hostnames; these are the hosts that speak SMTP to the outside.

**Cross-host SASL**: the submission path (`mo`'s postfix authenticating against `mb`'s dovecot over the network) is the one seam of this split that the container harness flags as still needing validation on real hosts — test it with an IMAP/SMTP client before pointing users at it.
