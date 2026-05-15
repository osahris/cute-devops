# SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#
# SPDX-License-Identifier: EUPL-1.2

# Two stages share this file. The default (last) stage is `website`; the test
# suite builds `--target systemd-env` (via test-in-containers.yaml).

# ---------------------------------------------------------------------------
# systemd-env: test base image -- Debian trixie with systemd as PID 1 plus the
# tools the deploy-stack roles need. Run with `podman run --systemd=always`.
# ---------------------------------------------------------------------------
FROM debian:trixie-slim AS systemd-env

ENV container=podman \
    DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      systemd systemd-sysv dbus \
      polkitd \
      python3 sudo git ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && systemctl mask \
      systemd-udevd.service systemd-udev-trigger.service \
      systemd-firstboot.service systemd-resolved.service \
      getty.target console-getty.service

# polkit.service ships heavy hardening (PrivateTmp, MemoryDenyWriteExecute,
# SystemCallFilter, RestrictAddressFamilies, DeviceAllow, ...) that all need
# CAP_SYS_ADMIN to apply -- which rootless podman doesn't have in its user
# namespace. A drop-in can't help: the inherited settings still require the
# cap. Full override: replace the unit with a minimal one that just runs
# polkitd. (The polkit *rule evaluation* is unchanged.)
RUN printf '%s\n' \
      '[Unit]' \
      'Description=Authorization Manager' \
      'Documentation=man:polkit(8)' \
      '' \
      '[Service]' \
      'Type=notify-reload' \
      'BusName=org.freedesktop.PolicyKit1' \
      'ExecStart=/usr/lib/polkit-1/polkitd --no-debug --log-level=notice' \
      > /etc/systemd/system/polkit.service

STOPSIGNAL SIGRTMIN+3

CMD ["/lib/systemd/systemd"]

# ---------------------------------------------------------------------------
# website: stock Caddy serving the patterns / roles markdown. Default target
# when building without --target.
# ---------------------------------------------------------------------------
FROM debian:trixie-slim AS website

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
