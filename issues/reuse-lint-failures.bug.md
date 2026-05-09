---
status: open
---

# REUSE lint fails after merge of patterns subtree

## Symptom

`reuse lint` reports 59 files (out of 351) without copyright / license
information. The full set falls into three groups:

1. **Files inherited from the upstream `mkbrechtel/patterns` subtree** —
   never had REUSE headers in their original repo. Markdown content under
   `patterns/`, plus dotfiles (`.gitignore`, `.vscode/`, `.zed/`,
   `.github/workflows/build.yml`).
2. **Files moved into `website/`** that previously sat at the patterns
   repo root — `astro.config.mjs`, `package.json`, `package-lock.json`,
   `tsconfig.json`, `CNAME`, `src/content.config.ts`,
   `src/components/SiteTitle.astro`, `public/emoji_u1f537.svg`.
3. **Repo-level files added by this merge** — root `go.mod`, the
   contents of `issues/` (planning surface — added directly without
   headers), and a few stragglers.

Lint output, summary line:
`Files with copyright information: 292 / 351`.

## Why this is a bug rather than a passing checkbox

`CODING.md` lists `reuse lint` as a required pre-merge check. The
patterns merge ([integrate-patterns-library](integrate-patterns-library.feature.md))
and the rename ([rename-to-mkbrechtel-devops](rename-to-mkbrechtel-devops.feature.md))
landed without fixing this so as not to balloon the diff; the underlying
compliance gap is real and breaks CI.

## Fix

For each missing file, add SPDX headers in the appropriate comment
syntax for the file format. Two reasonable approaches:

- **Per-file headers** for source / configuration files where a header
  comment is natural (`go.mod`, `astro.config.mjs`, JSON with `"//"`
  workaround, `.svg` via XML comment, etc.).
- **`REUSE.toml` annotations** for files where an in-file comment is
  awkward or impossible — markdown content under `patterns/` and
  `issues/`, lockfiles (`package-lock.json`), JSON without comment
  support (`tsconfig.json`, `.vscode/*.json`, `.zed/tasks.json`),
  `CNAME`. Annotate by glob (e.g. `patterns/**/*.md`,
  `issues/**/*.md`).

Pattern markdown originated from the upstream patterns repo (also
AGPL-3.0-or-later — see CONTRIBUTIONS.md acknowledgement of IDMKD); the
same SPDX-License-Identifier applies. Copyright dates on patterns
content should reflect the upstream history, roughly 2023–2026.

## Acceptance

- `reuse lint` exits 0.
- The lint job on `.github/workflows/reuse.yml` is green on `main`.
