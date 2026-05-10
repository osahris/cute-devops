---
title: Coding Guidelines
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->


## Global Variables

This collection uses global variables that can be shared across all roles, see the documnetation on [global variables](./GLOBAL.md).

## Feature Flags Pattern

This collection uses a "_with_" naming convention for optional feature flags in roles. These boolean variables enable or disable specific functionality within a role:

- Feature flags follow the pattern: `<role_name>_with_<feature>`
- Examples: `users_with_sudo`, `traefik_with_acme`
- This pattern allows roles to have a core functionality with optional extensions
- Feature flags should default to `false` to maintain backward compatibility

## Managed File Header

All templates that generate configuration files on target systems must include a managed file header comment. Do **not** use `{{ ansible_managed | comment }}` — use a fixed string instead.

The header format is:

```
# This file is managed by the mkbrechtel.devops.<role_name> Ansible role!
# MANUAL CHANGES WILL BE OVERWRITTEN WITHOUT WARNING!
```

- Replace `<role_name>` with the actual role name (e.g. `restic_client`, `deploy_ansible_play`)
- Use the appropriate comment syntax for the file format (`#` for shell/ini/systemd, `//` for JSON, `<!-- -->` for XML/HTML, etc.)
- Place the header after the SPDX license block, before the file content
- Do not use `{{ ansible_managed | comment }}` — it is deprecated and produces inconsistent output depending on the local `ansible.cfg` setting

## Role Development Guidelines

### Directory Structure

Each role should follow this structure:
```
roles/
  role_name/
    README.md         # Required for Galaxy
    meta/main.yml     # Required for Galaxy
    defaults/main.yml # Default variables
    tasks/main.yml    # Main task file
    handlers/main.yml # Handler definitions
    templates/        # Jinja2 templates
    files/           # Static files
    vars/            # Variables
```

### Task Organization

- Use descriptive task names that explain what the task does
- Group related tasks in separate files and include them from main.yml
- Use tags for optional functionality
- Always use fully qualified collection names (FQCN) for modules

### Variable Naming

- Role-specific variables should be prefixed with the role name
- Use underscores to separate words in variable names
- Document all variables in the role's README.md

### Error Handling

- Use `failed_when` and `changed_when` appropriately
- Provide meaningful error messages
- Use `block`/`rescue` for complex error handling scenarios

## Validation

Before submitting changes, run the following checks in order:

1. **Linting** — run `ansible-lint` to check for formatting and best practice issues:
   ```bash
   ansible-lint
   ```

2. **License compliance** — run `reuse lint` to verify that all files have correct SPDX licensing headers:
   ```bash
   reuse lint
   ```

3. **Integration testing** — run the VM test playbook to verify everything works end-to-end:
   ```bash
   ./test-in-vms.yaml
   ```

All three checks must pass before changes are considered ready.

### Git hooks

The `tests/` directory ships the repository's git hooks. `tests/pre-commit`
runs `reuse lint` and `ansible-lint` against a temporary checkout of the
staged tree (via `git checkout-index`), so the linters see exactly what
would be committed regardless of unstaged changes in the working tree.

`tests/post-commit` and `tests/post-receive` keep the active hooks in sync:
whenever `main` is committed-to in any worktree, or received as a push,
they install `tests/*` into `$GIT_COMMON_DIR/hooks/`. That directory is
shared across every worktree of the same repository, so a single update on
`main` rolls out the new hooks everywhere — no per-clone configuration.

`tests/reference-transaction` protects `main` from non-fast-forward
changes. It rejects force pushes received by the bare repo and local
rewrites from any worktree alike — including `git reset --hard` to an
older commit, history-rewriting rebases, and deletion. Branch creation
and ordinary fast-forward moves are allowed.

Bootstrap a fresh clone once by copying the scripts in:

```bash
install -m 755 tests/* "$(git rev-parse --git-common-dir)/hooks/"
```

After that the post-commit / post-receive hooks keep themselves up to date.

## Testing

- Test roles on all supported platforms (Debian bookworm/bullseye, Ubuntu jammy/focal)
- Verify idempotency by running roles multiple times
- Check for proper cleanup in handlers

## Documentation

- Every role must have a comprehensive README.md
- Document all variables with their types, defaults, and descriptions
- Provide usage examples in the README
- Keep CHANGELOG.md updated with all changes