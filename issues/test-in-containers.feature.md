---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# test-in-containers — role testing in podman system containers

## Goal

Test the collection's roles in rootless podman **system containers** — `systemd` as PID 1, full Debian userspace, real `.service` units — instead of full incus VMs. Faster to boot, cheaper to run, quick to iterate. Over time this becomes the default test path for most roles; `test-in-vms` stays only for what genuinely needs a VM.

## Scope

One base image (`test/Containerfile`) drives every instance. A templated quadlet (`cute-devops-test@<instance>`) starts named instances on a shared podman network in the invoking user's account, backed by linger. Ansible reaches them over the `containers.podman.podman` connection, mirroring how `test-in-vms` reaches VMs over the incus connection. Multi-instance topologies (the mail stack's `mo`/`mx`/`mb`/`ml` split) come from starting several instances on the one network and resolving peers by name.

The mail stack is the first and heaviest consumer — postfix, dovecot, and sympa are all real systemd services with cross-service coupling — so it proves the pattern. The lighter roles (`common`, deploy, monitoring, shells) follow, each as instances of the same base image.

A run assertion convention comes with it: the `test_mail_stack` role checks units are active and ports listen, and drives an end-to-end probe. The same shape generalises to other roles' service-up checks.

## Design notes

**System container, not dockerized.** One container, one init, one journald — the same service topology as a VM or bare metal. This is distinct from the compose-style, one-process-per-container decomposition that [`dockerize-mail-servers`](dockerize-mail-servers.feature.md) warns against.

**Prototype for `deploy_quadlet`.** The rootless-quadlet-with-linger provisioning here is the working prototype of the `deploy_quadlet` role proposed in [`container-apps.feature.md`](container-apps.feature.md); the two should converge.

**Where VMs stay.** Kernel modules, full-disk/storage, firmware/microcode, and true multi-host networking that a shared podman network can't model remain on `test-in-vms`.

## Open questions

- One base image for all roles, or per-family images once the set grows?
- CI: run the container harness in pipelines (needs cgroups v2 + rootless in the runner), and which topologies gate a merge?
- How far to push multi-instance realism (separate networks, injected latency) before it's cheaper to use VMs?
