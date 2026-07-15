---
title: Contributions
---

<!--
SPDX-FileCopyrightText: 2016-2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->


## Authors

- **Mirian Brechtel** (2016–2026) — Primary author and maintainer.

## Imported Contributions

Earlier work brought in from upstream projects, with original authorship
preserved and original licenses retained.

- **Philipp Kaluza** (2013–2022) — Contributions to the restic backup
  server and client roles (rps-backup), including TLS support, htpasswd
  generation, and systemd integration. These contributions were licensed
  under AGPL-3.0-or-later; the `restic_client` and `restic_server` roles
  therefore remain AGPL-3.0-or-later while the rest of the project is
  EUPL-1.2 (see [License](#license) below), pending coordination with
  Philipp on relicensing.

- **Janne Mareike Koschinski** (2017) — Author of
  [powerline-go](https://github.com/justjanne/powerline-go), from which
  the shell-prompt integration snippets bundled in `bash_shell`,
  `zsh_shell`, and `fish_shell` derive. Those files stay under
  `GPL-3.0-only` (powerline-go's license) and are not part of the
  EUPL-1.2 relicense.

- **Google LLC** — The 🔷 favicon / logo
  (`website/public/emoji_u1f537.svg`) is the "small blue diamond" glyph
  from the [Noto Emoji](https://github.com/googlefonts/noto-emoji)
  project, used under `Apache-2.0`.

## License

Most of the project is licensed under EUPL-1.2. The `restic_client` and
`restic_server` roles are licensed under AGPL-3.0-or-later (see
[Imported Contributions](#imported-contributions) above). Third-party
files retain their upstream licenses (e.g. powerline-go integration
snippets are GPL-3.0-only). Per-file SPDX headers are authoritative.

## Acknowledgements

A lot of the knowledge in the patterns half of this repo was gained
working at the **Institute for Digital Medicine and Clinical Data
Science (IDMKD)** and the former Working Group Cohorts in Infectious
Disease and Cancer Research of Prof. Janne Vehreschild.

## AI-Assisted Development

Parts of this project were developed with assistance from
[Claude](https://www.anthropic.com/claude) (Anthropic). AI-assisted commits
are marked with a `Co-Authored-By` trailer in the git history. Pattern
markdown content under `patterns/` originated in the now-archived
standalone Cute Patterns! library and was likewise generated with LLM
assistance (Anthropic Claude).