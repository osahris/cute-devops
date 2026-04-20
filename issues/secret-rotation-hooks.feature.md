---
status: draft
---

# Secret rotation hooks — specification

## Goal

Specify the hook surface that runs when the secrets role creates or rotates a secret, so downstream consumers (services, app configs) actually pick up the new value safely. Modeled on certbot's deploy/renew-hook pattern.

Depends on: [secrets.feature.md](secrets.feature.md).

## Scope

Two possible specification surfaces (or a combination):

- **Declarative in the role invocation**: the caller lists `post_rotate: [{ service: postgresql, action: restart }, …]` alongside the secret definition.
- **Filesystem convention**: a script at `/etc/secrets/<service>/<name>.post-rotate` is executed automatically if present.

Both is friendly to users but doubles the code paths and makes the "what will happen on rotation?" question harder to answer at a glance.

Built-in action types (both surfaces):

- `service.restart`
- `service.reload`
- `file.touch`
- `script.run` (path + args)

Execution semantics:

- `no_log: true`.
- Minimal environment (no secret values in env by default; an opt-in `pass_value_via: stdin|file|env` for hooks that must receive the value).
- Bounded timeout per hook, configurable.
- Distinct auditable logs; failure is surfaced loudly, not swallowed.

## Design notes

Hook failure after rotation is a tricky state: the secret has been regenerated, but the consumer didn't pick it up. We need a clear recovery path — roll back the store to the previous value, or leave the new value in place and flag the consumer as out-of-sync?

## Open questions

- Pick one surface or support both?
- Rollback on hook failure: default on or off?
- How do we order multiple hooks — declared order, parallel, or topologically sorted on some dependency graph?
- Do hooks get access to the old value (for migration scripts that re-encrypt or re-key)?
- Relationship to Ansible handlers — do we reuse the handler mechanism when the hook is "restart this service", or always go through our own runner for uniformity?
