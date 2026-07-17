---
title: Issues
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

This project follows the
[In-Tree Issues 🗂️](../patterns/approaches/in-tree-issues.md)
pattern: every issue is a markdown file in the repository, filed
via a merge request, accepted into `main` by the maintainers.
There is no separate tracker — `issues/` *is* the tracker.

## Where they live

```
issues/
├── README.md                                      ← convention notes
├── airgapped-escrow.feature.md
├── backup-restic.feature.md
├── integrate-patterns-library.feature.md
├── oidc-gating.pattern.md
├── reuse-lint-failures.bug.md
└── …
```

The directory is browsed at the website root via the regular
content-negotiation rules — open `issues/<slug>` in a browser to
see the rendered issue, or `issues/<slug>.md` for the source.

## Type suffix conventions

Type suffixes are *hints*; tooling can filter on them. Use the one
that fits, or drop the suffix when none applies.

| Suffix | Meaning |
|---|---|
| `.feature.md` | A capability or change we want to ship — adds, refactors, integrations. |
| `.bug.md`     | Something is broken or behaves contrary to the docs. |
| `.pattern.md` | A documentation pattern proposal — sketches an entry for the [Patterns!](../patterns.md) library. |
| *(no suffix)* | Plain issue — pick when no suffix really fits. |

## How to file an issue

### From the web (recommended for non-developers)

A small form on the project's website creates the branch + MR for
you. You only fill in title and body; the form takes care of:

- slugifying the title into `issues/<slug>.md`,
- creating an `issue/<slug>` branch,
- committing your text into a single new file,
- opening the MR for review.

You'll get a link to the resulting MR; you can keep editing it from
the forge's web UI.

### From a git checkout

```bash
git checkout -b issue/short-slug
$EDITOR issues/short-slug.feature.md
git add issues/short-slug.feature.md
git commit -m "issues: file <topic>"
git push -u origin HEAD
```

Open an MR; reviewers will discuss in-line on the file.

## What "merged" means here

Per [the pattern](../patterns/approaches/in-tree-issues.md), merging
an issue file into `main` is the maintainers' acknowledgement that
the issue is part of the current development state — not yet a
commitment to ship a fix today, but a public "yes, this is on our
plate". A rejected issue's MR is closed without merge; the
conversation is preserved on the closed MR.

So:

- **MR open**     → under discussion.
- **MR merged**   → accepted, on the project's roadmap.
- **MR closed without merge** → declined, with the rationale on the
  MR.

## Status, after merge

Status fields in the file's front matter are *advisory*. The
authoritative signal is the implementation MR(s) that follow:

- A `bug.md` is "done" when the matching code-fix MR lands.
- A `feature.md` is "done" when its implementation MRs land.
- A `pattern.md` is "done" when the corresponding entry exists
  under `patterns/`.

If you want to reflect progress on the issue file itself, do so in a
follow-up MR that touches the file — same review path as everything
else.

## Cross-references

- A `feature.md` proposing work on a role typically links the role's
  README.
- A `pattern.md` issue can ship the *draft* pattern in the same MR;
  on merge, the draft moves to `patterns/<category>/<name>.md` and
  the issue file is updated with the link.
- A `bug.md` references the affected file paths so reviewers can jump
  in directly.

## Related

- [In-Tree Issues 🗂️](../patterns/approaches/in-tree-issues.md) — the
  pattern this page implements.
- [Pattern Pattern 🔷²](../patterns/meta/pattern.md) — the structure
  used by `pattern.md` issues that graduate into the library.
- [Contributions](./contributing.md) — authorship, license carve-outs,
  contributor agreement.
