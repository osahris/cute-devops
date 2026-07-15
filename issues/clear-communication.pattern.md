---
status: draft
---

<!--
SPDX-FileCopyrightText: 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science
SPDX-FileCopyrightText: 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Clear Communication 🪶

> **Pattern (draft).** Voice convention for documentation files in the project: patterns, READMEs, CLAUDE.md.

## Goal

Make the project's documentation legible at a glance. Land on the smallest text that carries the idea; nothing hidden in volume.

## Principles

- **Short.** A bullet beats a paragraph; a sentence beats two.
- **Structure.** One idea per section, one section per concern.
- **Low duplication.** If the same recipe or explanation appears twice, one of them is a link.

Inverse: walls of plausible-looking text. See [Cuteness 🌸](../patterns/meta/cuteness.md) on long unreviewed AI slop.

## Aside: agents may benefit too

Brief docs may also sharpen agents reading the project. Brevity-constrained LLMs improve measurably in accuracy on some benchmarks — the "caveman mode" finding. Same compression that helps humans seems to help agents. See [caveman (Claude Code skill)](https://github.com/JuliusBrussee/caveman) and [Better Stack's writeup](https://betterstack.com/community/guides/ai/caveman-llm/) for context.

## Acceptance

Promote to `patterns/meta/clear-communication.md` once a couple of existing patterns reference it explicitly.
