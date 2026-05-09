---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Secrets management

## Goal

Secrets can live on the host where they are used. This role provides a uniform interface for sourcing and storing secrets, with pluggable source and store backends. Intended for real production use, safe defaults, no surprises.

## Scope

### Identity

Secrets are identified by a service and a name, and typed by a file-extension suffix. Default storage location is `/etc/secrets/<service>/<name>.<type>` (e.g. `/etc/secrets/restic/repo-passphrase.env`).

Roles that need a secret at a specific path (e.g. `/etc/restic/password`) may create a symlink to that secret — the role provides an option for this.

### Type / extension map

The type extension tells consumers how to parse the file. The map is defined in the role so new types can be added without patching every consumer. Chaining is supported and read outside-in: the rightmost extension is the outermost wrapping, so `foo.pass.gpg` is a `.pass` wrapped in `.gpg`.

| Extension | Type | Content |
|---|---|---|
| `.env` | env | dotenv bundle — multiple secrets in one file as `KEY=value` entries |
| `.pass` | pass | plaintext passphrase, single line |
| `.privkey.pem` | privkey | PEM-encoded private key (TLS, X.509, CA keys) |
| `.privkey.ssh` | ssh_privkey | OpenSSH private key |
| `.vault` | vault | ansible-vault encrypted wrapper (chainable) |
| `.gpg` | gpg | GPG-encrypted wrapper (chainable) |

This role handles keys only, not certificates — X.509 and SSH certificate issuance live in the separate `certificates` role, which uses this role underneath for storing CA and service keys. Public keys are not secrets: when an SSH keypair is generated via this role, the matching public key is derived on demand (`ssh-keygen -y`) and placed wherever the consumer needs it by an explicit task — it does not live in the secrets folder.

### Sources (where a secret comes from on first use)

- **random** (default): generate locally using a CSPRNG. Default recipe is 24 characters from `a-zA-Z0-9.-_%=+`, not starting with a symbol — typeable by a human in an emergency.
- **file**: read from a file on the remote node (e.g. a temp file carrying a pre-placed value).
- **local-file**: read from a file in the collection or on the control node.
- **dotenv**: read a named key from a `.env` file on the control node (`source: dotenv; file: path; key: KEY_NAME`).
- **prompt**: ask the user running the playbook via Ansible's prompt mechanism.
- **askpass**: request the secret interactively via an askpass helper (SSH_ASKPASS / systemd-ask-password).
- **pass**: read from the admin's `pass` store on the control node. `pass` is installed via `apt` on the control node.
- **var**: read from an Ansible variable. Covers ansible-vault transparently — the variable is simply vault-encrypted at rest in the repo and Ansible decrypts at play time. The value can also be an inline Jinja template, since Ansible renders variables before we see them.

A weak-password assertion rejects overrides that fall below the entropy floor.

Later: centralized password managers like HashiCorp Vault — see [hashicorp-vault-integration.feature.md](hashicorp-vault-integration.feature.md).

### Stores (where the secret lives)

- **file** (default): written to the remote machine at `/etc/secrets/<service>/<name>.<type>` (or override path).
- **local-pass**: stored only in the control node's `pass` store — the secret never touches the target filesystem. The lookup plugin fetches it at play time and injects into templates or tasks. For the case where a secret should not reside on the host.
- **ansible-vault**: vault-encrypted file shared via the ansible repo. Ansible handles decryption at play time via the standard vault password mechanism; this store writes or updates the entry.

Later: systemd credentials (LoadCredential=) — see [systemd-credentials-store.feature.md](systemd-credentials-store.feature.md).

### Defaults

- Source: `random`.
- Store: `file` on the remote machine.
- Type: `.env`. Defaulting to `.env` maximizes compatibility with systemd `EnvironmentFile=`, docker-compose, and most modern services — the value drops straight into service configuration without a translation step. Most programs we develop use the `env var carries the secret` pattern, and for the occasional non-`.env` case, explicit typing is a minor override.

### Consumer interfaces (both needed)

**Module** (`mkbrechtel.devops.secret`) runs on the target. Ensures a secret is provisioned; optionally fetches the value back to the controller for use in subsequent tasks. Arguments:

- `service` (required) — service namespace.
- `name` (required) — secret name within the service.
- `type` — extension/type (default: `env`).
- `source` — source backend (default: `random`).
- `store` — store backend (default: `file`).
- `path` — override the default storage path.
- `fetch` — if `true`, return the secret value back to the controller for template use. Default: `false`.
- Source-specific args (`length`, `charset`, `file`, `key`, `var`, …).

**Lookup plugin** (`mkbrechtel.devops.secret`) runs on the control node and is usable in templates. It does not SSH to the target — it serves the sources/stores that live on the control node (`var`, `ansible-vault`, `local-pass`). For target-resident secrets, use the module with `fetch: true`.

### Example usage

```yaml
- name: Ensure postgres app password
  mkbrechtel.devops.secret:
    service: postgres
    name: app-db-password
    fetch: true
  register: app_db_pass

- name: Render app config
  template:
    src: connection.env.j2
    dest: /etc/myapp/connection.env
  vars:
    password: "{{ app_db_pass.value }}"
```

### Rotation

Each stored secret carries a generation timestamp. Per-secret cycle time is configurable (e.g. `90d` for API tokens, `never` for SSH host keys). A monitoring check fires an alert when a secret is past its cycle time. Rotation can be triggered manually via a role flag or CLI helper, which forces re-source and overwrites. A test-rotate mode exercises the rotation path in a dry run so broken rotation is caught before it matters.

When SSH host keys are issued as certs by an internal SSH CA (via the `certificates` role), host keys become genuinely rotatable: clients verify the CA signature, so pinned `known_hosts` churn disappears and host-key rotation becomes a routine operation.

### Handlers / hooks

After a secret is created or rotated, declared hooks run to propagate the change to downstream consumers — modeled on certbot's deploy/renew hooks. A secret may carry a post-rotate hook (restart service, reload service, touch file, run script). Hooks run with `no_log: true`, a bounded timeout, and audit-logged outcomes; failure is surfaced loudly, not swallowed.

The detailed specification of the hook surface (declarative in-invocation, filesystem-convention, or both) lives in [secret-rotation-hooks.feature.md](secret-rotation-hooks.feature.md).

### Multi-line secrets

First-class support via the typed files above. TLS keys, SSH keys, and multi-line passphrases all work without special handling from consumers.

### Bootstrap

On a fresh host, requesting a secret "just works" with sane defaults: the consumer asks for a secret by service and name, gets a default-strength random `.env` entry written under `/etc/secrets/<service>/<name>.env/`, and proceeds. No per-call configuration is required for the common case. The first role run authenticates as root via Ansible to create `/etc/secrets/` and write the initial entries.

## Design notes

Pattern: secrets are on the host anyhow — store them there in a secure file and fetch them when a role needs them. 

Idempotency: if the secret already exists in the store, do not re-source unless rotation is requested. The `pass` source and the `local-pass` store give the admin a shared view of all secrets across hosts from their workstation without changing the operational model on the host. 

SSH keys go through this role as typed entries (see map) and reuse the same rotation and storage machinery.

**Implementation**: the module and any supporting CLIs are written in Go — project convention for everything outside Ansible and the documentation site. Ansible raw-binary modules (JSON in, JSON out) are the integration path. The lookup plugin is Python because Ansible requires that, but it may shell out to Go helpers where useful.

## Security checklist

Safe defaults are non-negotiable — this is the most sensitive role in the collection.

- [ ] **File perms**: store files `0600`, owned by `root` by default; configurable to a service user where needed. Store directory `0700`.
- [ ] **Random generation**: CSPRNG (Go `crypto/rand` or Python `secrets`, never non-crypto RNGs). The default recipe provides ≥128 bits of entropy; a weak-password assertion rejects overrides that drop below that floor.
- [ ] **`no_log: true`** on every task that touches a secret value. CI verifies we do not accidentally leak values to play output.
- [ ] **Never print** secret values to stdout, even on dry-run.
- [ ] **pass / local-pass** rely on GPG; key management requirements (agent, yubikey) for the control node are documented.
- [ ] **Rotation overdue** is an alert, not a silent condition — stale secrets stay visible.
- [ ] **Audit log**: rotation events (service, name, timestamp, trigger) are written to a local audit file. Values are never logged.
- [ ] **Transport**: the lookup plugin does not reach the target; it operates on control-node data only.
- [ ] **Memory hygiene**: secret values are held in [`memguard`](https://github.com/awnumar/memguard) enclaves — allocated off the Go heap, `mlock`'d to prevent swap, encrypted at rest with a per-enclave key, guard-paged against adjacent reads, and zeroed on destruction. Plain byte buffers are a last-resort fallback for paths memguard cannot cover.
- [ ] **Defaults err safe**: if a secret's source/store is not explicitly set, we pick random + file + default recipe. Never empty, never predictable.
- [ ] **Hook execution**: post-rotate hooks run with `no_log`, a minimal environment, a bounded timeout, and distinct auditable logs.
