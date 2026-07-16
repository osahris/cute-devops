---
status: draft
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

# Shell defaults

## Goal

Opinionated defaults that apply to *whichever* shell a user chooses —
bash, zsh, or fish — so moving between shells feels consistent on a
managed host.

## Scope

- Shared defaults applied across `bash_shell`, `zsh_shell`,
  `fish_shell` roles:
  - Prompt (informative, shell-appropriate rendering).
  - Common aliases (ls/grep colors, safer `rm`, etc.).
  - History settings (size, dedup, ignore-space).
  - Completion (enable + refresh).
  - Key bindings where sensible.
- Per-shell role reads a shared defaults data model and renders it to
  the right config file (`~/.bashrc`, `~/.zshrc`, `~/.config/fish/*`).
- Users can opt out of individual pieces or override.

## Design notes

- Think of the per-shell roles as "drivers" over a shared settings
  model — same idea as the DNS roles.
- Respect existing user customization: prefer append + clearly marked
  sections over full file generation. (Or use `.d`-style drop-ins
  where the shell supports them.)
- No shell should require the others to be installed.

## Open questions

- Scope of "defaults" — where does this stop vs. dotfiles management per
  user? (There's overlap with what `user-management` might deliver.)
- Do we enforce a specific prompt (e.g. starship) or let each shell
  render its own native prompt styled similarly?
- How do we handle conflicts between central defaults and user-local
  `.bashrc` content?
