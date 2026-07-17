<!--
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Setting up a mailserver

This guide walks an admin through deploying a mailserver for `example.org` with the `osahris.cute_devops` collection: [postfix](../../roles/postfix/) as the MTA (SMTP on 25, SMTPS on 465, optionally submission on 587), [dovecot](../../roles/dovecot/) for IMAP and local delivery over LMTP, DKIM signing and DMARC verification via the [opendkim](../../roles/opendkim/) and [opendmarc](../../roles/opendmarc/) milters (pulled in by the postfix role by default), and optionally [sympa](../../roles/sympa/) for mailing lists. The target platform is Debian trixie.

Two topologies are covered in separate chapters: a [single-server setup](#single-server-setup) with the whole stack on one host `mail.example.org`, and a [multi-server setup](#multi-server-setup) that splits the stack across `mx` / `mo` / `mb` / `ml` hosts.

## Prerequisites

- Debian trixie host(s) with public IPv4 (and ideally IPv6) addresses, reachable on the ports their role needs (25, 465, 587, 993) — and outbound port 25 not blocked by the provider.
- Root access for Ansible (`ansible_user=root` or become).
- Control over the `example.org` DNS zone, and the ability to set reverse DNS (PTR) for the hosts' addresses in your provider's UI.
- The collection installed: `ansible-galaxy collection install osahris.cute_devops`.

## Single-server setup

One host, `mail.example.org`, runs postfix and dovecot side by side: postfix accepts inbound mail and authenticated submission, hands mailbox mail to dovecot over a local LMTP socket, and dovecot serves it out over IMAP. This is the topology to start with.

### Inventory

```ini
[mailservers]
mail.example.org ansible_user=root
```

### Configuration

The one variable everything hangs off is `mailserver_domain_name` — the mailserver's own FQDN. The postfix role derives its hostname, origin and domain from it, and the opendkim role signs for it by default.

A minimal `group_vars/mailservers/mail.yaml`:

```yaml
mailserver_domain_name: mail.example.org
postfix_admin_email: postmaster@example.org

# Domains this server receives mail for. Without this, postfix tries to
# relay mail for your own domain and bounces it.
postfix_virtual_mailbox_domains:
  - example.org

# Submission (587) is off by default; 25 and 465 are on.
postfix_with_submission_service: true

# TLS: point both services at your certificate. The default is the
# Debian snakeoil cert, which remote MTAs and IMAP clients will reject.
postfix_certificate_fullchain_file: /etc/letsencrypt/live/mail.example.org/fullchain.pem
postfix_certificate_private_key_file: /etc/letsencrypt/live/mail.example.org/privkey.pem
dovecot_certificate_fullchain_file: /etc/letsencrypt/live/mail.example.org/fullchain.pem
dovecot_certificate_private_key_file: /etc/letsencrypt/live/mail.example.org/privkey.pem

# Mailboxes: file-based auth. Generate hashes with
#   doveadm pw -s SHA512-CRYPT
dovecot_auth: passwdfile
dovecot_users:
  - username: postmaster@example.org
    password_hash: "{CRYPT}$6$..."
```

**Certificates**: set `postfix_with_ssl: true` to have the postfix role provision a Let's Encrypt certificate via certbot instead of pointing at existing files. This needs the host to be publicly reachable for the ACME challenge, so it is off by default.

**Mailboxes**: with `dovecot_auth: passwdfile`, the accounts are exactly the `dovecot_users` entries above — postfix derives its virtual mailbox map from the same list, so adding a user is one entry. For a larger setup with a management UI, `dovecot_auth: sql` plugs both postfix and dovecot into a [postfixadmin](../../roles/postfixadmin/) MySQL database (`postfix_with_postfixadmin: true`).

### Playbook

```yaml
- hosts: mailservers
  become: true
  roles:
    - role: osahris.cute_devops.postfix
      tags: postfix
    - role: osahris.cute_devops.dovecot
      tags: dovecot
```

For mailing lists, add `osahris.cute_devops.sympa` and set `postfix_with_sympa: true` so postfix routes list mail to it; see the [sympa role](../../roles/sympa/) for its database and domain configuration.

### DNS

```
mail.example.org.      IN A      203.0.113.25
                       IN AAAA   2001:db8::25
                       IN TXT    "v=spf1 mx -all"
example.org.           IN MX     1 mail.example.org.
_dmarc.example.org.    IN TXT    "v=DMARC1; p=quarantine"
```

**DKIM**: the opendkim role generates the signing keys on the host (under `/etc/opendkim/keys/<domain>/<selector>.private`) and the play prints the ready-to-paste `<selector>._domainkey` TXT records during the run — copy them into the zone. The selector defaults to the host's short name (here `mail`); pin it explicitly with `dkim_selector` if the hostname may change.

**Reverse DNS**: set the PTR records for the host's IPv4 and IPv6 addresses to `mail.example.org` in your provider's UI. Many large providers reject mail from hosts whose forward and reverse DNS don't match.

## Multi-server setup

The same stack split across four hosts on a network that lets them reach each other:

- `mx.example.org` — mail exchange: inbound postfix, verifies DKIM/DMARC, hands mailbox mail to `mb` over LMTP.
- `mo.example.org` — mailout: authenticated submission (587) and outbound relay; SASL auth is delegated to `mb`'s dovecot over the network.
- `mb.example.org` — mailboxes: dovecot with IMAP for clients, plus LMTP and auth exposed on the network for `mx` and `mo`. No local postfix.
- `ml.example.org` — mailing lists: sympa with its own postfix (sympa writes transport maps and restarts it, so it needs a local MTA).

### Inventory

```ini
[mailservers]
mx.example.org ansible_user=root
mo.example.org ansible_user=root
mb.example.org ansible_user=root
ml.example.org ansible_user=root
```

### Shared configuration

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

### Per-host configuration

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

### Playbook

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

### DNS

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

## Trying it out first

The repository ships a container harness that deploys both of these exact topologies into rootless podman system containers and asserts SMTP/IMAP mail flow end to end — a fast way to try a configuration before touching a real host: `./test-in-containers-single.yaml` and `./test-in-containers-multi.yaml`, see [test/README.md](../../test/README.md). The same [test_mail_stack](../../roles/test_mail_stack/) role can be pointed at a deployed server to assert its units, ports and a full send/receive round trip.
