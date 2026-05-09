---
status: reviewed
---

# SSH server role

## Goal

A dedicated `ssh_server` role that owns `sshd` configuration on managed hosts. Today the collection has `ssh_agent` (user-side) but no server-side counterpart — `sshd_config` is whatever the distro ships. Needed now because the `users` role moves authorized_keys into a central `/etc/ssh/authorized_keys.d/<user>` layout, which requires an `AuthorizedKeysFile` directive that something has to own.

## Scope

### Configuration via drop-in

The role manages a single drop-in under `/etc/ssh/sshd_config.d/00-managed.conf`. The distro's main `sshd_config` stays untouched; the drop-in's directives take precedence. Keeps the role's delta visible, preserves distro defaults the role doesn't care about, and plays nicely with package-integrity tooling that watches the main config.

### Opinionated defaults in the drop-in

- `PermitRootLogin no` — no root SSH at all.
- `PasswordAuthentication no` — key-only by default. A site that needs password auth sets `ssh_server_password_auth: true` in inventory; the role flips the directive only when explicitly opted in.
- `AuthorizedKeysFile /etc/ssh/authorized_keys.d/%u` — matches the `users` role's central layout.
- KEX / cipher / MAC / HostKeyAlgorithms left at OpenSSH defaults. The OpenSSH project tracks crypto deprecation more closely than we will; we don't second-guess them. Override-able per host if a site has a hardening requirement.

### Authorized-keys directory ownership

The role creates and owns `/etc/ssh/authorized_keys.d/`, mode `0755 root:root`. The `users` role drops `0600 root:root` files for each managed user. Either role can run first; ownership boundary is clean.

### Host keys

Left to the distro's first-boot generation. The role does not regenerate, replace, or rotate host keys — that's a separate concern (and the `known_hosts` churn it causes is exactly why an SSH host CA would be nice; out of scope here, see Design notes).

### Port / ListenAddress

Defaults (port 22, all addresses). The role does not expose first-class variables for either; sites that want non-default ports or specific bind addresses add their own drop-in to `/etc/ssh/sshd_config.d/`. The role's drop-in does not include `Port` or `ListenAddress` directives at all.

### Reload + validation

Handler reloads via `systemctl reload ssh` with a mandatory `sshd -t` validation gate. A failed validation aborts the play before the broken config touches a running service.

## Design notes

### Why a drop-in rather than rewriting `sshd_config`

The Debian-shipped `sshd_config` is a known-good starting point. Replacing it wholesale would force the role to track every distro change for the directives we don't even care about. A small drop-in with explicit overrides keeps our intent visible and the surface area small.

### Why `PasswordAuthentication: false` + explicit opt-in instead of a tri-state

A boolean knob (off by default, true to enable) reads cleaner than `none / required / fallback` enums and matches what every site actually wants ("on or off"). Fallback authentication patterns are a foot-gun.

### Why no first-class port / listen variables

Adding inventory knobs for things every site can express in two lines of drop-in config is over-engineering. The drop-in directory is the extension point; the role doesn't need to mirror every `sshd_config` directive as a variable.

### Out of scope, by design

- **SFTP-only / restricted-shell SSH users.** Useful in some deployments but a different shape — chroot, `Match` blocks, restricted shells. Future ticket if real demand shows up.
- **SSH host CA / certificate-signed host keys.** Would dramatically improve the `known_hosts`-churn-on-reinstall story, but requires a CA, signing infrastructure, and client-side `@cert-authority` distribution. Real work, separate ticket if anyone wants it. The role uses normal raw host keys.
- **2FA / TOTP / Yubikey on top of SSH.** Same: useful, separate concern, future ticket.

