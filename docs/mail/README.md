<!--
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Setting up a mailserver

This guide walks an admin through deploying a complete mailserver with the `osahris.cute_devops` collection: [postfix](../../roles/postfix/) as the MTA (SMTP on 25, SMTPS on 465, optionally submission on 587), [dovecot](../../roles/dovecot/) for IMAP and local delivery over LMTP, DKIM signing and DMARC verification via the [opendkim](../../roles/opendkim/) and [opendmarc](../../roles/opendmarc/) milters (pulled in by the postfix role by default), and optionally [sympa](../../roles/sympa/) for mailing lists. The target platform is Debian trixie.

## Prerequisites

- A Debian trixie host with a public IPv4 (and ideally IPv6) address, reachable on ports 25, 465, 993 — and outbound port 25 not blocked by the provider.
- Root access for Ansible (`ansible_user=root` or become).
- Control over the DNS zone of your mail domain, and the ability to set reverse DNS (PTR) for the host's addresses in your provider's UI.
- The collection installed: `ansible-galaxy collection install osahris.cute_devops`.

## Inventory

Put the host in a `mailservers` group:

```ini
[mailservers]
mx.example.org ansible_user=root
```

## Configuration

The one variable everything hangs off is `mailserver_domain_name` — the mailserver's own FQDN. The postfix role derives its hostname, origin and domain from it, and the opendkim role signs for it by default.

A minimal `group_vars/mailservers/mail.yaml`:

```yaml
mailserver_domain_name: mx.example.org
postfix_admin_email: postmaster@example.org

# Domains this server receives mail for. Without this, postfix tries to
# relay mail for your own domain and bounces it.
postfix_virtual_mailbox_domains:
  - example.org

# Submission (587) is off by default; 25 and 465 are on.
postfix_with_submission_service: true

# TLS: point both services at your certificate. The default is the
# Debian snakeoil cert, which remote MTAs and IMAP clients will reject.
postfix_certificate_fullchain_file: /etc/letsencrypt/live/mx.example.org/fullchain.pem
postfix_certificate_private_key_file: /etc/letsencrypt/live/mx.example.org/privkey.pem
dovecot_certificate_fullchain_file: /etc/letsencrypt/live/mx.example.org/fullchain.pem
dovecot_certificate_private_key_file: /etc/letsencrypt/live/mx.example.org/privkey.pem

# Mailboxes: file-based auth. Generate hashes with
#   doveadm pw -s SHA512-CRYPT
dovecot_auth: passwdfile
dovecot_users:
  - username: postmaster@example.org
    password_hash: "{CRYPT}$6$..."
```

**Certificates**: set `postfix_with_ssl: true` to have the postfix role provision a Let's Encrypt certificate via certbot instead of pointing at existing files. This needs the host to be publicly reachable for the ACME challenge, so it is off by default.

**Mailboxes**: with `dovecot_auth: passwdfile`, the accounts are exactly the `dovecot_users` entries above — postfix derives its virtual mailbox map from the same list, so adding a user is one entry. For a larger setup with a management UI, `dovecot_auth: sql` plugs both postfix and dovecot into a [postfixadmin](../../roles/postfixadmin/) MySQL database (`postfix_with_postfixadmin: true`).

## Playbook

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

## DNS

The zone for `example.org` needs, at minimum:

```
mx.example.org.        IN A      203.0.113.25
                       IN AAAA   2001:db8::25
                       IN TXT    "v=spf1 mx -all"
example.org.           IN MX     1 mx.example.org.
_dmarc.example.org.    IN TXT    "v=DMARC1; p=quarantine"
```

**DKIM**: the opendkim role generates the signing keys on the host (under `/etc/opendkim/keys/<domain>/<selector>.private`) and the play prints the ready-to-paste `<selector>._domainkey` TXT records during the run — copy them into the zone. The selector defaults to the host's short name; pin it explicitly with `dkim_selector` if the hostname may change.

**Reverse DNS**: set the PTR records for the host's IPv4 and IPv6 addresses to `mx.example.org` in your provider's UI. Many large providers reject mail from hosts whose forward and reverse DNS don't match.

## Trying it out first

The repository ships a container harness that deploys this exact stack into rootless podman system containers and asserts SMTP/IMAP mail flow end to end — a fast way to try a configuration before touching a real host. See [test/README.md](../../test/README.md). The same [test_mail_stack](../../roles/test_mail_stack/) role can be pointed at a deployed server to assert its units, ports and a full send/receive round trip.
