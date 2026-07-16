<!--
SPDX-FileCopyrightText: 2024 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2024-2025 Tom-Robin Raja <tom-robin.raja@uk-koeln.de>
SPDX-FileCopyrightText: 2024-2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
SPDX-FileCopyrightText: 2025 Vasilii Tulskii <tulskyva@mail.ru>

SPDX-License-Identifier: Apache-2.0 OR EUPL-1.2
-->

## Global variables

- `mailserver_domain_name`: name of the mailserver, e.g. mx.example.org

## manual steps to deploy with portal & steps after full mailserver deployment
- `git submodule add https://gitlab.com/idcohorts/rps/rps-mail.git ansible_collections/rps/mail` to use rps-mail in portal repository (e.g. onconnect-portal)
- create mailservers group in hosts 
```
[mailservers]
mx.example.com ansible_host=127.0.0.1
```

- create postfix.yaml & dovecot.yaml under environments/prod/group_vars/mailservers

- postfix.yaml
```
mailserver_domain_name: mx.example.com

dkim_cname_domains:
  example.com: mx.example.com

postfix_with_submission_service: true
postfix_with_smtps_service: true

postfix_virtual_mailbox_domains:
  - example.com
dovecot_auth: passwdfile

postfix_certificate_fullchain_file: /etc/letsencrypt/live/mx.example.com/fullchain.pem
postfix_certificate_private_key_file: /etc/letsencrypt/live/mx.example.com/privkey.pem

```

- dovecot.yaml
```
dovecot_certificate_fullchain_file: /etc/letsencrypt/live/mx.example.com/fullchain.pem
dovecot_certificate_private_key_file: /etc/letsencrypt/live/mx.example.com/privkey.pem

dovecot_users:
  - username: postmaster@example.com
    password_hash: $5$mZtTwRWOg1eKJAJV$XEsQj91eUCCJV4HawNVvEC5Ss4NON2LHcP7ZJKGLpP0
  - username: accounts@example.com
    password_hash: $5$pu766BzsCHNM3w.W$ojvSUAfOhX8yAQ9VYCNlPuhPMHvwgmrdrbhcmyw82Q4
  - username: no-reply@example.com
    password_hash: $5$pu766BzsCHNM3w.W$ojvSUAfOhX8yAQ9VYCNlPuhPMHvwgmrdrbhcmyw82Q4
```

- set DNS entries:
```
mx._domainkey IN TXT "v=DKIM1; h=sha256; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn8d/dV2xgQFzPYy9QuFBc2xL3Oziz+rMw/mUo83EDdt2pNMoWTOBqFhfKJuAgExO5btVWloOGX7AmucPjkRHhq1d48i2J/UF4jEq0xtdlL5fRB+HkGaoSeYfj75HzjB+0890Zkyve++FhCLCxNuFtMKLOMpfcIsBcXJZOsKh6Nt3Ed7GdrXJvubsIIUG9nWTWNtV5Q/JP4Qb8imJ5E1ESmz8Kf/KbDHvZmlqNZbRKATZk/Csa90c/zPXk6rRh859A5YC4vQ9xt9TwYDcmawY8boAeHlMlfhf1p2NuLWws1soM3vLEiqEJnQ36bqKfjKzeT8Q0g87amgcV1wzYx3gEQIDAQAB"
_dmarc.mx IN TXT "v=DMARC1; p=quarantine"
mx._domainkey.mx IN TXT "v=DKIM1; h=sha256; k=rsa; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAn8d/dV2xgQFzPYy9QuFBc2xL3Oziz+rMw/mUo83EDdt2pNMoWTOBqFhfKJuAgExO5btVWloOGX7AmucPjkRHhq1d48i2J/UF4jEq0xtdlL5fRB+HkGaoSeYfj75HzjB+0890Zkyve++FhCLCxNuFtMKLOMpfcIsBcXJZOsKh6Nt3Ed7GdrXJvubsIIUG9nWTWNtV5Q/JP4Qb8imJ5E1ESmz8Kf/KbDHvZmlqNZbRKATZk/Csa90c/zPXk6rRh859A5YC4vQ9xt9TwYDcmawY8boAeHlMlfhf1p2NuLWws1soM3vLEiqEJnQ36bqKfjKzeT8Q0g87amgcV1wzYx3gEQIDAQAB"
mx IN A 127.0.0.1
		 IN AAAA 2a01:4f8:c010:bd8c::1
		 IN TXT "v=spf1 mx -all"
		 IN MX	1	mx

*.mx IN A 127.0.0.1
		   IN AAAA 2a01:4f8:c010:bd8c::1
```

- set rDNS with Hetzner/Provider UI