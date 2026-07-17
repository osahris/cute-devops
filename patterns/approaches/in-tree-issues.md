---
title: In-Tree Issues 🗂️
---

<!--
SPDX-FileCopyrightText: 2016 - 2026 Mirian Brechtel <markus.katharina.brechtel@thengo.net>
SPDX-FileCopyrightText: 2020 - 2025 Uniklinik Köln
SPDX-FileCopyrightText: 2025 - 2026 Goethe-University Frankfurt – Institute for Digital Medicine and Clinical Data Science

SPDX-License-Identifier: EUPL-1.2
-->

## Overview 📋

Issues live where the code lives — as plain markdown files inside the
repository (e.g. `issues/some-thing.feature.md`). Filing one is a
git operation: a branch + an MR creating one new file. Status moves
through the lifecycle the same way every other change does, so the
issue tracker is just *another view of the repo*.

## Goals 🎯

- Keep issue context next to the code that fulfils it; reviewing one
  always shows the other.
- Make filing an issue something a non-developer can do, without
  leaving the code-host's web UI.
- Avoid a parallel system (Jira, GitHub Projects, Linear) whose
  contents must be kept in sync with the repo by hand.
- Track status with the same primitives as code: branches, MRs,
  reviews, merges.
- Make the entire issue history searchable with `grep`,
  greppable in CI, and includable as build artefacts (release notes,
  roadmap pages on the website).

## Pattern Structure 📑

### File layout

```
issues/
├── README.md                  ← describes the convention
├── some-feature.feature.md    ← optional .<type> suffix
├── some-bug.bug.md
├── a-design.pattern.md
└── plain-issue.md             ← type optional
```

The `.feature.md` / `.bug.md` / `.pattern.md` suffix is **optional**;
when present it's a hint for filtering and tooling. Drop it for plain
issues.

### Front matter (optional)

```yaml
---
title: Optional human-readable title
status: open | in-review | done   # advisory; the MR state is the truth
owner: handle                     # optional
priority: low | normal | high     # optional
---
```

The MR / branch state is the *authoritative* status — front-matter
fields are advisory hints for dashboards. Don't make tooling depend
on them.

### Lifecycle

```
   open                        accepted                merged
 ┌──────┐  filed-as-MR  ┌─────────────────┐  reviewed  ┌────────┐
 │ idea │ ───────────► │ branch + new file │ ────────►│  main  │
 └──────┘               └─────────────────┘            └────────┘
                                  │
                                  │ rejected / withdrawn → branch closed
                                  ▼
                              (no merge)
```

Each transition is a normal forge action: open MR → review → merge.
No separate state machine.

### Acceptance semantics

Merging an issue file into `main` is more than paperwork — it's the
maintainers explicitly accepting that *this issue is part of the
current development state of the project*. The review bar on an
`issues/*.md` MR is "do we agree this is ours to think about?",
separate from "should we ship the implementation?" A rejected issue
closes the MR without merge; the conversation is preserved on the
closed MR but the main-line view stays free of wishlist clutter.

That turns `git log -- issues/` into an authoritative roadmap rather
than an unfiltered backlog: every entry is on the books because the
team said so.

### Filing interface (web UI for non-developers)

The pattern *prescribes* providing a one-click way to file an issue
without git knowledge:

- A small form on the project's website (or a forge "issue template")
  collects title + body.
- On submit, the server creates a new branch (`issue/<slug>`),
  commits a single `issues/<slug>.md` file with the user's content,
  and opens an MR — exactly the same artifact a developer would
  produce.

The reporter never has to know about branches, MRs, or git. Behind
the scenes the issue is already in the canonical place.

### Status views

Build the dashboards from the repo, not from a separate system:

- *Open issues* → MRs that touch `issues/*` and are not yet merged.
- *Backlog* → merged issue files whose `status:` front matter isn't
  `done` (or whose paired implementation MR hasn't landed).
- *Roadmap* → pattern files in `issues/*.pattern.md`.

These all fall out of `git log`, `gh pr list`, `forgejo-cli` etc.
Render them on the website at build time when needed.

## Security Considerations 🔐

- The web filing interface is an unauthenticated branch creator
  unless gated. Rate-limit it; require an account on the forge for
  anything beyond a clearly-marked public idea bin.
- Markdown is not a secret store — operators occasionally try to
  drop credentials into "ideas". Add a CI check that scans new
  `issues/*.md` for plausible secret shapes and fails the MR.
- Branch creation costs are minimal but not free; cap concurrent
  open-from-form branches per user / IP.

## Anti-Patterns ⚠️

- ❌ Mirroring issues to GitHub Issues / Jira / Linear *and* keeping
  them in-tree. The mirror always drifts. Pick one system; this
  pattern says it's the repo.
- ❌ Encoding state in the filename (`open-foo.feature.md` →
  `done-foo.feature.md`). Renames lose history; the merge state
  carries the same information for free.
- ❌ A bot that auto-edits issue files based on external triggers.
  Issues are reviewed like any other change — through MRs.
- ❌ A long flat `issues/` dir with hundreds of half-baked entries.
  Curate with the same review bar as code; close (don't delete)
  rejected ones via the MR.

## Best Practices 💡

- Keep the filing form's rendered MR description identical to the
  file's body so reviewers don't need to read both.
- Use a `.gitkeep` or `README.md` in `issues/` describing the
  convention; new contributors will find it.
- Include the issues directory in the website build so issues are
  publicly browsable (with the same content negotiation as the rest
  of the docs).
- Treat type suffixes (`.feature.md`, `.bug.md`, `.pattern.md`) as
  conventions, not gates. Tooling reads them as hints.
- Cross-reference: an issue that proposes a pattern can ship the
  draft pattern in the same MR; merging implements the issue.

## Implementation Checklist ✅

### Setup

- [ ] Create `issues/` at the repo root with a short `README.md`
  that documents the type-suffix convention and the front-matter
  fields (if any).
- [ ] Add an MR template for `issues/*` changes that says: *"This
  MR files / amends an issue. Close it by merging — or by closing
  without merge for rejection."*
- [ ] Wire up the website: render `issues/*.md` on the public site
  (or behind a login if private).

### Web filing interface

- [ ] Provide a single-page form `/issues/new` that takes title +
  body.
- [ ] On submit, slugify the title, create branch `issue/<slug>`,
  commit `issues/<slug>.md`, and open an MR.
- [ ] Add a CI check that scans the new file for secret-shaped
  strings.
- [ ] Rate-limit per IP / per account.

### Lifecycle hygiene

- [ ] Encourage reviewers to land issue MRs that are *just* the
  issue file (no implementation). Implementation comes in follow-up
  MRs that link back.
- [ ] When closing without merging (rejected), leave a comment on
  the MR explaining why; don't lose the conversation.
- [ ] Run `git grep` on `issues/` periodically to find stale
  status: front matter; nudge owners.

## Related Patterns 🔗

- [Pattern Pattern 🔷²](../meta/pattern.md) — for documenting
  patterns once they emerge from issue conversations.
- [Smalltown Infrastructure 🏘️](./smalltown-infrastructure.md) —
  same spirit: keep the system small, composable, and operable
  without an extra SaaS.
- [Don't introduce GitLab as the central DevOps Hub of your organization! 🔻🦊](../../anti-patterns/gitlab.md) —
  the anti-pattern this approach quietly avoids.

## References 📚

- The `issues/` directory in this very repo (e.g.
  `issues/integrate-patterns-library.feature.md`,
  `issues/oidc-gating.pattern.md`,
  `issues/reuse-lint-failures.bug.md`) — *eat your own dog food*.
- Forgejo's web editor + branch-creation flow makes the
  one-click form a thin wrapper around forge primitives.
