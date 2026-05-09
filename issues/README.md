<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Issues

We track issues as `.md` files in this directory. Three ticket kinds, distinguished by suffix:

- **`<name>.feature.md`** — a unit of implementation. The development goal is the role / script / change itself. On completion the file moves to `docs/features/` and is featured on the project homepage.
- **`<name>.pattern.md`** — a cross-cutting convention. The development goal is a *consolidated definition* that explains the convention and how implementations follow it. On completion the file moves to `docs/patterns/`. Patterns are referenced from feature tickets that adopt them; multiple features may share one pattern.
- **`<name>.bug.md`** — a defect. The development goal is to fix the broken behaviour and prevent regression (test, lint, doc note as appropriate). Closed bugs are squashed at commit time, not promoted to a docs page; the fix and any rationale live in the commit message.

All three share a loose structure: **Goal → Scope → Design notes → Open questions** (bugs may compress this to **Symptom → Root cause → Fix**).

The choice between them:

- "this role now does X" → feature.
- "every relevant role agrees to do X this way" → pattern.
- "this currently does the wrong thing" → bug.
