---
status: reviewed
---

<!--
SPDX-FileCopyrightText: 2016-2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>

SPDX-License-Identifier: EUPL-1.2
-->

# Cute DevOps Patterns! — merge patterns repo into this collection

## Goal

The upstream [Cute Patterns!](https://patterns.how) ([mkbrechtel/patterns](https://github.com/mkbrechtel/patterns)) library and this collection are being unified into a single repo. 

The patterns content has already been pulled in via `git subtree add` at `patterns/`. This ticket plans the rest of the merge: a clean split inside the repo between **`patterns/`** (the markdown content) and **`website/`** (the Astro  site machinery), retiring the upstream patterns repo, and rebranding the unified project as **Cute DevOps Patterns!** served at **`devops.patterns.how`**.

## Scope

### Final shape — one repo, three surfaces

```
mkbrechtel/devops/
├── patterns/           ← just the markdown patterns (the *what / why*)
├── roles/              ← Ansible roles (the *how* — pattern implementations)
├── website/            ← Astro site, hosts devops.patterns.how
├── issues/             ← planning surface (.feature.md / .pattern.md / .bug.md)
├── playbooks/
├── docs/               ← contributor / collection-level docs
└── …
```

One repo, one project. **Cute DevOps Patterns!** is the human-facing name; Galaxy FQCN remains `mkbrechtel.devops`.

### Central files live at repo root

`README.md`, `CONTRIBUTIONS.md`, `LICENSE` (or `LICENSES/`), `CLAUDE.md`, `CODING.md`, `RELEASE.md`, `GLOBAL.md`, `REUSE.toml`, `galaxy.yml`, plus **`go.mod` / `go.sum`** all live at the repo root and apply to the unified project. The patterns subtree brought in its own `patterns/README.md` / `patterns/CONTRIBUTIONS.md` / `patterns/LICENSE` / `patterns/CLAUDE.md` / `patterns/go.mod` / `patterns/go.sum` — those merge upward into root-level files (credits combined in `CONTRIBUTIONS.md`, README rewritten to introduce the unified project, EUPL-1.2 already aligns, Go module unified — see below). After migration there are no duplicate central files inside `patterns/` or `website/`.

### One Go module at the root

The whole project is a single Go module with `go.mod` / `go.sum` at the repo root. Today's Go code is the patterns website server (currently `patterns/main.go` + `patterns/embed.go`); after the merge those are still the only Go files, but they live in `website/` as a package within the root module. Future Go code — any helper CLIs, Ansible modules built as raw-binary Go (per the project's [Go-implementation memory](https://example/Go-as-implementation-language)), site tooling — joins the same module rather than spawning a sibling.

Conventional Go layout under one root module:

- `go.mod`, `go.sum` at root
- `website/main.go`, `website/embed.go` — the website server binary, package within the root module
- (later) `cmd/<binary>/main.go` for additional Go binaries this project ships
- (later) `internal/...` or domain-named packages for shared Go code

One module, one set of dependencies, one `go build` to reason about.

### `patterns/` holds only the markdown content

Categories live directly under `patterns/`:

```
patterns/
├── operation/
├── development/
├── about/
├── meta/
├── index.md
└── ideas.md
```

The current `patterns/docs/` layer is flattened — the directory itself is the docs. Central files (`README.md`, `LICENSE`, etc.) move out to the repo root (see above). Cross-references from roles use `patterns/<category>/<name>.md` (one path component shorter than before).

### `website/` holds the Astro + Go site

Everything currently sitting in `patterns/` that is *not* markdown content moves into `website/`:

```
website/
├── astro.config.mjs
├── package.json
├── package-lock.json
├── tsconfig.json
├── src/                ← Astro components / layouts
├── public/             ← static site assets
├── main.go             ← Go server, package within the root Go module
├── embed.go            ← embeds the built Astro assets
├── CNAME               ← domain pointer: devops.patterns.how
└── redeploy            ← deploy hook
```

`go.mod` / `go.sum` do **not** live here — the project has one Go module at the repo root (see above). Built and deployed from this repo's CI. The site reads from `../patterns/` for pattern content and may also surface role docs by reading `../roles/*/README.md` (open question — see below).

### Domain — `devops.patterns.how`

`patterns.how` itself stays as it is for now (parent domain, can become an index of multiple pattern projects later). The merged project lives at the `devops.` subdomain, signaling its narrower devops focus while keeping the patterns family relationship visible in the URL.

### Migration steps from the current state

1. **Catch-up `git subtree push` upstream** to send any local-only changes back to mkbrechtel/patterns.
2. **Restructure inside this repo**:
   - `git mv` the Astro and Go-source files from `patterns/` → `website/` (`main.go`, `embed.go`, `astro.config.mjs`, `package.json`, `package-lock.json`, `tsconfig.json`, `src/`, `public/`, `CNAME`, `redeploy`).
   - `git mv patterns/go.mod` and `patterns/go.sum` to the **repo root** — one Go module for the whole project, not a per-directory module.
   - Flatten `patterns/docs/<category>/` → `patterns/<category>/`.
   - Merge central files upward to the repo root: combine `patterns/CONTRIBUTIONS.md` into the root `CONTRIBUTIONS.md` (preserving credits), fold relevant content from `patterns/README.md` and `patterns/CLAUDE.md` into the root README and CLAUDE.md, drop the duplicate `patterns/LICENSE` (root LICENSES/ already covers EUPL-1.2).
   - Update the Go module path and import paths to match the new module identity (e.g. `github.com/mkbrechtel/devops`).
   - Update internal links in pattern markdown files (any `./docs/...` or relative paths inside the subtree, plus any references to the moved central files).
3. **Update `website/CNAME`** to `devops.patterns.how`.
4. **CI to deploy `website/` to `devops.patterns.how`.** Reuses the existing Astro + Go pipeline; just runs from this repo now.
5. **Archive `mkbrechtel/patterns` on GitHub** with a README pointing here. No further commits upstream; the merged repo is the source of truth.
6. **Repoint old `patterns.how` deploys / DNS** if they were tied to the upstream repo. Keep `patterns.how` itself stable (parent domain placeholder); the new project lives at `devops.patterns.how`.
7. **Documentation pass** — repo-root README, CONTRIBUTIONS.md, CLAUDE.md introduce the unified project and brand.

### Cross-references — bidirectional, documentation-level

Both directions are written prose, maintained by humans, and live in the markdown:

- **Role → patterns** (in each role's README): a "Patterns" section names which patterns the role implements, links them by relative path (`../../patterns/<category>/<name>.md`). One role may reference several patterns or none.
- **Patterns → roles** (in each pattern's markdown): a "Possible implementations" / "See also" section names roles that implement the pattern, links them by relative path. One pattern may list several roles or none. The relationship is many-to-many — patterns are shared vocabulary, not a sketch the listed roles implement faithfully.

This is **documentation-level linking**, not an enforced contract. A role's "Patterns" list says "the author thinks these patterns inform the role"; a pattern's "Possible implementations" list says "the author thinks these roles work an instance of this pattern." Renames or removals require updating both sides — same as any prose cross-reference in any docs surface.

CI lint validates both directions resolve to existing files. That's it; the lint doesn't enforce that the two sides agree (it's editorially valid for a role to mention a pattern the pattern doesn't list back, or vice versa).

### Galaxy packaging

`patterns/` ships in the published collection — `ansible-galaxy collection install mkbrechtel.devops` brings the markdown along. `website/` does not — `galaxy_ignore` excludes the entire directory; site-build tooling, JS, Go server, and Astro source are all irrelevant to a Galaxy consumer.

### `patterns` role — content delivery to managed hosts

A new `patterns` role copies `patterns/` to `/usr/local/share/patterns/` on managed hosts. Default host class: **devbox**; opt-in elsewhere via `patterns_install: true`. The [claude-code](claude-code.feature.md) plugin marketplace picks `/usr/local/share/patterns/` up; user-supplied skills in `~/.claude/skills/` shadow the central ones by name.

### License

Both repos ship EUPL-1.2; merging is straightforward — the root `LICENSES/` directory already covers it, and the duplicate `patterns/LICENSE` from the subtree gets dropped. Contributor credits from `patterns/CONTRIBUTIONS.md` get folded into the root `CONTRIBUTIONS.md` verbatim.

## Design notes

### Why split `patterns/` and `website/`

The previous draft kept Astro source files inside `patterns/` because the subtree landed them there. Once we own the layout, that's wrong: `patterns/` should be just the content (the thing roles reference, the thing the Galaxy artifact ships, the thing humans browse on GitHub) and `website/` should be just the rendering machinery. Two clear directories with one job each. Adding a second pattern collection later (`mobile/`? `frontend/`?) would slot in alongside `patterns/` without confusing the rendering layer; conversely, swapping the Astro stack for something else only touches `website/`.

### Why `devops.patterns.how` (subdomain) rather than reusing `patterns.how`

`patterns.how` was the upstream's home and its scope was generic. The merged project narrows scope to devops; using a subdomain says that explicitly and leaves room at the parent for any future sibling project (or just an index page describing the family). DNS-wise it's a small, reversible decision; URL-wise it's clearer.

### Why one repo instead of two

Roles and patterns aren't a 1:1 pair, so the case isn't "co-locate the two halves of one thing." It's softer and still real: same maintainer, same voice, same steady traffic of "while writing a role I notice a pattern worth capturing" and "while reading a pattern I want to point at a role that mostly does this." In two-repo land that traffic is friction — switch repos, separate MR, separate review, separate release. In one repo it's one MR. The merge buys that, plus the smaller win of one set of CI / issues / release machinery instead of two.

### Why the website knows about both

The Astro site sits next to both content trees and can pull from either. `devops.patterns.how` can be the home for the markdown patterns *and* a browse-roles view — one site, two content sources, with the bidirectional editorial cross-references naturally rendering as links between pages on either side. The detailed shape of the role-docs view is an open question (see below); the layout makes it possible without rebuilding anything.

### Why retire the upstream patterns repo

Two repos describing the same thing produce two truths the moment one diverges. The merge means there is one repo, one history, one source of truth. The upstream becomes an archive (with a pointer to the new home) so existing links don't 404, but no further work happens there.

### Why flatten `patterns/docs/` to `patterns/`

`docs/` was an internal organization choice in the upstream — the whole subtree was effectively "docs about patterns plus a site to render them." Now that the site has its own directory (`website/`), `patterns/` is unambiguously just the content; the extra `docs/` layer is redundant and one path component longer than it needs to be in every cross-reference. Better to do the flatten now, while we already have to update internal links from the subtree split.

## Open questions

- **Should the website also render role docs?** The Astro site lives next to `roles/` so it could surface each role's README as a page on `devops.patterns.how/roles/<role>/`, alongside the patterns. That would make the site the single browsable view of the project. Trade-off: more work in `website/src/` to wire up two content sources, vs. a noticeably more useful site. Lean yes, but separate task — land the merge first, layer the role-docs view on top in a follow-up.
