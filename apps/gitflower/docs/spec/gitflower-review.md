---
#SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
#SPDX-License-Identifier: EUPL-1.2
---

# `gitflower review` — TUI and CLI for `.review` files

`gitflower review` is gitflower's front-end for the `.review` format. It reads and writes `.review` bodies on per-branch notes refs (`refs/notes/reviews/<branch>`, optionally mirroring to an on-disk file), runs a bubbletea TUI for interactive reviewing, and exposes CLI sub-commands for scripted use.

The on-disk format itself is documented separately in [`dot-review-format.md`](./dot-review-format.md). This file specifies the tool — the commands, the TUI behaviour, and the flags. Anything reading or writing `.review` bodies should follow the format spec regardless of tool.

## Commands

`gitflower review` is a convenience wrapper that runs `begin` + `diff <current-branch-spec>` + `commits <current-branch-spec>` on first open, then launches the TUI. The sub-commands below let scripts compose the same behaviour piecewise.

**`gitflower review begin`** creates an empty `.review` on HEAD: the header block plus the bare `# Review` heading and a few meta lines (`SPDX-FileCopyrightText`, `SPDX-License-Identifier`, `Review-Head-Commit`, `Reviewed-Branch`, `Created-By`). If a non-`.review` body already exists on the notes ref for this commit, it's imported as a `## Note @ git notes --ref=… show …` subsection so the prior content is preserved.

**`gitflower review diff <diffspec>`** appends a `# Diff <diffspec> @ git diff <oldcommit>..<newcommit>` section, with one per-file subsection (`## File … modified` / `## File … created` / `## File … deleted` / `## File … moved`) under it.

**`gitflower review commit <sha>…`** appends one `# Commit <sha> @ git show <sha>` section per SHA, in command-line order.

**`gitflower review commits <diffspec>`** walks the spec (typically `base..tip`) and appends a `# Commit` section per commit in oldest-first order.

**`gitflower review files <path>…`** appends `## File "<path>"` subsections (full repo-root-relative paths) under the matching `# Repo Tree` or `# Subfolder` section, creating the tree section if it doesn't exist yet. The commit defaults to HEAD; `--commit <sha>` pins a specific commit.

**`gitflower review edit`** opens the current `.review` body in `$EDITOR`. Useful when the TUI's input is too clunky or the reviewer wants to batch-edit.

**`gitflower review merge`** attaches the review to the branch history. Merges the tip of `refs/notes/reviews/<branch>` into HEAD with `-s ours` — the working tree stays clean, the per-branch notes chain hangs off the merge's second parent. The merge commit's subject is prefixed `[Review]`; the body carries a verdict-count summary, a `git show <notes-sha>` recipe, and the `Verdict-reached-by:` trailers from the `.review` verbatim. See *Attaching to history* in `dot-review-format.md` for the on-disk shape. Gated behind the `with_review_merge` build tag.

## TUI behaviour

The on-disk `.review` is the source of truth. Opening a file parses what's there; there is no implicit re-running of `git diff` or `git log`. Every keystroke that mutates state writes back through the same notes-ref path the CLI uses.

**Source-pane / peek-pane layout.** A sidebar lists the H1 sections; selecting one opens its body in the main pane. Selecting an `## Issue` / `## Remark` / `## Note` shows that subsection's body.

**`# Review` right pane.** When the cursor is on the `# Review` heading, the right pane lists the meta lines and any unknown keys verbatim so the reviewer can sanity-check what was recorded.

**Sidebar Remark numbering.** Multiple `## Remark` subsections render as "Remark 1", "Remark 2", … in the sidebar for navigation. The on-disk heading stays bare `## Remark`; the numbers are positional, not stored.

**Open-question lane.** A separate sidebar entry lists every `- Question-asked-by:` with no `- Answer-given-by:` under it, so questions don't get lost in long reviews. Drives the `ClarificationRequired` verdict state suggestion.

**Resolved-issue display.** `## Issue` subsections that carry a `- Resolved-by:` line render collapsed/dimmed in the sidebar. Removing the line re-opens the issue in the live view.

**Duplicate-issue flag.** Two `## Issue` subsections with the same title get a warning marker in the sidebar; the writer doesn't enforce uniqueness, but the TUI surfaces dupes so reviewers can dedupe.

**Auto-import on first open.** If `gitflower review begin` has never run for the current HEAD and the notes ref already has a non-`.review` body, the TUI imports it (same shape as `review begin` does — see *Notes-ref interop* below).

### "Comment from the bottom" convenience

A reviewer who has just finished reading a section is past its last `> ` line (the EOF marker), not back at the top where a section-anchored event lives. The TUI accepts a `Commented-by:` / `Question-asked-by:` / `Reacted-by:` event submitted from past the EOF marker and inserts it at the **top** of the section. On disk the result is byte-identical to writing it at the top; the bottom-of-section input is purely a UX shortcut.

## Flags

`gitflower review --with-timestamps` turns on per-event timestamps. The format spec describes the on-disk syntax (` @<RFC3339>` between email and `; <args>`); this flag is what makes the writer emit them. Off by default for privacy reasons.

`gitflower review -o <path>` mirrors the current `.review` body to a file at `<path>` in addition to writing the notes ref. Useful for diffing two reviews on disk or for tooling that doesn't read notes refs. The notes ref stays the source of truth even with `-o` set.

`gitflower review --notes-ref <ref>` reads and writes a different notes ref than the default `refs/notes/reviews/<current-branch>`. Mostly for testing — production reviewers stick with the default so the gate hook and other tools find content where they expect it.

## Notes-ref interop

The `refs/notes/reviews/*` namespace is shared territory, not gitflower-exclusive. Any note body on a per-branch ref is a recorded review action in git. `.review`-format bodies (first line is `dot-review-File-Version:`) are what gitflower reads and writes; other bodies — freeform sign-offs, kernel-style trailers (`Reviewed-By:`, `Acked-By:`, `Signed-off-by:`), CI-bot output, anything — coexist on the ref untouched.

The `.review` parser ignores notes that don't begin with `dot-review-File-Version:`. The writer never overwrites them on save.

When `gitflower review begin` runs on a commit whose notes ref already has a non-`.review` body, that body is imported into the new `.review` as a `## Note @ git notes --ref=… show …` subsection under `# Review`. Kernel-style trailers stay grep-able in the imported body, so the planned `review-gate` hook keeps recognising sign-offs after the conversion.

### Planned `review-gate` hook

A future `gitflower review-gate` hook will block merges unless a commit has been reviewed. It scans a commit's notes-ref body for approval signals in priority order:

1. **`.review` format** — a parseable body with at least one `* Verdict-reached-by: …; Approved` counts as approved.
2. **Kernel-style trailers** — any line matching `^(Reviewed-By|Acked-By|Signed-off-by): <Name> <<email>>` counts as a sign-off.
3. **No recognised signal** — the body exists but doesn't approve.

The hook itself is a future feature; this spec only declares that the notes ref is shared territory and the gate will treat both formats as first-class.

## References

- [`dot-review-format.md`](./dot-review-format.md) — the on-disk file-format spec this tool reads and writes.
- The in-tree-review pattern (`patterns/approaches/in-tree-review.md`).

## Considerations

### "Comment from the bottom" rationale

The natural moment to leave a wrap-up comment is right after finishing reading a section, not before opening it. The "bottom inserts at top" shortcut lets the TUI accommodate that without introducing a second cursor concept (one for line-anchored events, one for section-anchored ones).

### Non-`.review` notes interop philosophy

Keeping the format opt-in: teams already using `git notes` for review records get the future `review-gate` hook for free, and teams that want the full `.review` machinery get richer state on top of the same notes ref. The hook itself is a future feature — this spec only declares that the notes ref is shared territory.
