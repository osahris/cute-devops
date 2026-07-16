---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Container app deployment

## Goal

Deploy containerized applications on managed hosts using podman, via both
**quadlets** (systemd-native) and **docker-compose** (pragmatic
familiarity). Deploy pipelines from `deploy-from-git.feature.md` trigger
these.

## Scope

- `deploy_quadlet`: renders `.container` / `.volume` / `.network` files
  under `/etc/containers/systemd/` and reloads systemd.
- `deploy_compose`: runs `podman compose up` / `podman-compose` with a
  `compose.yml` from the source repo.
- Both integrate with:
  - Secrets role (mount secrets as files or env vars).
  - Reverse proxy (publish on a socket, register vhost).
  - Firewall (declare any required rules — though socket-only should
    mean none).
- Lifecycle: install, update (pull new image / restart unit), remove.

## Design notes

- Quadlets are preferred for long-lived managed apps: better systemd
  integration, journald logs, proper service semantics.
- Compose is for upstream projects shipped as `compose.yml` that we
  don't want to rewrite.
- Image source: pinned tags or digests. No `:latest` in production
  variable defaults.
- Data volumes under `/srv/<project>/<app>/data`, backed up by the
  backup roles via include-paths convention.

## Open questions

- podman rootless per-service user vs rootful? Rootless is cleaner
  isolation but complicates socket sharing with the RPX.
- Image pull: scheduled (timer-based check), on-deploy only, or
  both?
- `podman compose` vs `podman-compose` — pick one or support both?
- How does the deploy pipeline tell the container role "update to this
  git SHA / this image tag"? A file in the repo (`.devops/app.yaml`)
  that the deploy role reads?
- Network isolation between containers on the same host — default
  policy?
