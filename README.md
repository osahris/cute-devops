<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Cute DevOps! 🦄

The ***Cute DevOps!*** library is a small, opinionated knowledge base
about running computers — patterns we like, the patterns we *don't
recommend*, and the Ansible roles we use to ship the ones we do.

Heads up: this library is pretty opinionated; we're working on a
comments section so you can drop in and tell us where we're wrong *IYO*.

The repository ships as the Ansible collection `mkbrechtel.devops`
and renders to the website at
[devops.devio.mkbrechtel.dev](https://devops.devio.mkbrechtel.dev).

## Patterns! 🔷

Cute, helpful patterns for getting work done — operational habits,
deployment shapes, and thinking tools. Patterns are organized by
*approach* (e.g. Smalltown Infrastructure), *operation* (deployment,
backup, monitoring), *development* (frontend), and *meta* (how to
write a pattern, why "cuteness" matters).

→ Browse the library at
[/patterns](https://devops.devio.mkbrechtel.dev/patterns).

## Anti-Patterns! 🔻

Critiques of industry patterns that aren't cute: practices that
look reasonable on the surface but produce friction, opaque
systems, or unhappy operators in the long run. Each entry names
what it is, why it's tempting, and what to do instead — usually a
pointer back to the [Patterns!](#patterns-) library.

→ See
[/anti-patterns](https://devops.devio.mkbrechtel.dev/anti-patterns).

## Deploy! 🚀

The Ansible roles that implement the patterns. Each role has a
README documenting variables, examples, and the patterns it ships.
The website's *Deploy!* sidebar groups them by purpose
(Orchestrators, System, Shells, Containers, Backup, Monitoring,
Deployment, Tooling).

```bash
ansible-galaxy collection install mkbrechtel.devops
```

Requirements: Ansible ≥ 2.14.3 on Debian 13 (trixie).

→ Browse role documentation at
[/roles](https://devops.devio.mkbrechtel.dev/roles).

## Coding! 💻

Documentation for working on the collection itself: coding
conventions, contribution flow, the release process, and the
project's planning surface (`issues/*.feature.md`,
`issues/*.pattern.md`, `issues/*.bug.md`).

→ See [/dev](https://devops.devio.mkbrechtel.dev/dev).

## License

EUPL-1.2 by default. Per-file `SPDX-License-Identifier` headers are
authoritative; the carve-outs are:

- `roles/restic_client/` and `roles/restic_server/` —
  AGPL-3.0-or-later (carve-out for a co-author's contributions; see
  `CONTRIBUTIONS.md`).
- Third-party powerline-go integration snippets
  (`roles/bash_shell/files/powerline-go.sh`,
  `roles/zsh_shell/files/powerline-go.zsh`,
  `roles/fish_shell/files/global/fish_prompt.fish`) —
  GPL-3.0-only.
- Google Noto Emoji glyphs used as section icons / favicon
  (`website/static/*.svg`) — Apache-2.0.
