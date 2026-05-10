# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: EUPL-1.2

FROM debian:trixie-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends caddy ca-certificates \
 && rm -rf /var/lib/apt/lists/*

COPY Caddyfile          /etc/caddy/Caddyfile
COPY website/templates/ /srv/templates/
COPY website/static/    /srv/static/
COPY patterns/          /srv/content/patterns/
COPY README.md          /srv/about.md

EXPOSE 8080

CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
