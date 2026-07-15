# SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
# SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
# SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
#
# SPDX-License-Identifier: EUPL-1.2

FROM debian:trixie-slim

# Stock Caddy from GitHub releases. Trixie's `caddy` package is 2.6.2;
# we want 2.7+ for the Sprig FuncMap in the templates module
# (regexReplaceAll, trimSuffix, hasPrefix, …) — single binary, no
# plugins. Bump the version + checksum together; sha256 published at
# https://github.com/caddyserver/caddy/releases.
ARG CADDY_VERSION=2.11.2
ARG CADDY_SHA256_AMD64=94391dfefe1f278ac8f387ab86162f0e88d87ff97df367f360e51e3cda3df56f

ADD --checksum=sha256:${CADDY_SHA256_AMD64} \
    https://github.com/caddyserver/caddy/releases/download/v${CADDY_VERSION}/caddy_${CADDY_VERSION}_linux_amd64.tar.gz \
    /tmp/caddy.tgz

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && tar -xzf /tmp/caddy.tgz -C /usr/local/bin caddy \
 && rm /tmp/caddy.tgz \
 && /usr/local/bin/caddy version

COPY Caddyfile          /etc/caddy/Caddyfile
COPY website/templates/ /srv/templates/
COPY website/static/    /srv/static/
COPY patterns/          /srv/content/patterns/
COPY patterns.md        /srv/content/patterns.md
COPY anti-patterns/     /srv/content/anti-patterns/
COPY anti-patterns.md   /srv/content/anti-patterns.md
COPY roles/             /srv/content/roles/
COPY roles.md           /srv/content/roles.md
COPY role-groups.md     /srv/content/role-groups.md
COPY improve/           /srv/content/improve/
COPY improve.md         /srv/content/improve.md
COPY README.md          /srv/content/README.md

EXPOSE 8080

CMD ["/usr/local/bin/caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
