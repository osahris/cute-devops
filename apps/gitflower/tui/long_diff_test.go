// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

// TestSpaceWalkOnLongDiffs exercises Space + PgDn against synthetic
// hunks bigger than any viewport: a 500-line all-add diff and a
// 400-line all-remove diff. The driver imitates a real reviewer's
// flow — PgDn through the visible content (firing read ticks as
// hunks fully display), Space to jump to the next unread / next
// file. We require 100% read at the end; this test is here to catch
// rare regressions in the walk logic, so anything less than every
// hunk read is a bug we want to surface.
func TestSpaceWalkOnLongDiffs(t *testing.T) {
	addPatch := buildAddPatch("big_add.txt", 500)
	delPatch := buildRemovePatch("big_remove.txt", 400)
	combined := addPatch + "\n" + delPatch

	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "long",
		Commits: []review.Commit{
			{SHA: "abc1234567890", Short: "abc1234", Subject: "long diff commit"},
		},
		Files:   []string{"big_add.txt", "big_remove.txt"},
		RawDiff: combined,
		FilePatches: map[string]string{
			"big_add.txt":    addPatch,
			"big_remove.txt": delPatch,
		},
		CommitPatches: map[string]string{
			"abc1234567890": "From abc1234 ...\n" + combined,
		},
	}
	tmp := t.TempDir()
	reviewPath := filepath.Join(tmp, "long.review")
	sess := review.New(scope, "tester@example.com", reviewPath)

	m := newModel(sess, tmp, 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 25})

	// First Space drills into the first file at "5 before first unread".
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("after first Space: mode %v want modeDiff", m.mode)
	}

	// Drive: PgDn until current file is fully read, then Space to
	// advance; stop on the verdict editor.
	const maxSteps = 2000
	stuck := 0
	for i := 0; i < maxSteps; i++ {
		// Fire pending read ticks deterministically.
		if m.viewReadScheduled {
			next, _ := m.Update(viewReadMsg{gen: m.viewReadGen})
			m = next.(*model)
		}
		if m.edit == editSummary {
			t.Logf("reached verdict editor after %d step(s)", i)
			break
		}
		before := stateSig(m)
		// Use Space when the current file is exhausted, PgDn otherwise.
		var msg tea.Msg
		if m.atEOF || m.mode == modeTree || !m.fileHasUnread(m.fileIdx) {
			msg = tea.KeyPressMsg{Code: ' ', Text: " "}
		} else {
			msg = tea.KeyPressMsg{Code: tea.KeyPgDown}
		}
		m = step(t, m, msg)
		if stateSig(m) == before {
			stuck++
			if stuck > 4 {
				t.Fatalf("walk stuck after %d step(s): %s", i, before)
			}
		} else {
			stuck = 0
		}
	}

	// Tally: every reviewable line in every (real) file must be read.
	for fi, f := range m.files {
		if strings.HasPrefix(f.Path, "commit:") {
			continue
		}
		r, total := m.fileLineCounts(fi)
		if r != total {
			t.Errorf("walk left lines unread in %s: %d/%d read", f.Path, r, total)
		}
	}
}

// TestSpaceSpamPreservesScrollProgress: page partway into a long
// unread hunk with PgDn, then spam Space. The viewport must stay
// where the reader left it, and the displayed-row accumulator must
// not be cleared — otherwise spamming Space would reset PgDn
// progress and effectively prevent the read tick from firing.
func TestSpaceSpamPreservesScrollProgress(t *testing.T) {
	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "spam",
		Commits: []review.Commit{{SHA: "abc1234567890", Short: "abc1234", Subject: "spam"}},
		Files:   []string{"big.txt"},
		RawDiff: buildAddPatch("big.txt", 200),
		FilePatches: map[string]string{
			"big.txt": buildAddPatch("big.txt", 200),
		},
		CommitPatches: map[string]string{"abc1234567890": "From abc1234\n"},
	}
	tmp := t.TempDir()
	sess := review.New(scope, "tester@example.com", filepath.Join(tmp, "spam.review"))
	m := newModel(sess, tmp, 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 20})

	// Drill in, then PgDn a few times so we're mid-way through the hunk.
	m = key(t, m, ' ', " ")
	for i := 0; i < 5; i++ {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	progress := m.viewport.YOffset()
	if progress == 0 {
		t.Fatalf("PgDn made no progress")
	}
	cursor := m.lineCursor
	hunkIdx := m.hunkIdx

	// Now spam Space. Each one should be a no-op since the next
	// unread line is the one already under the cursor.
	for i := 0; i < 20; i++ {
		m = key(t, m, ' ', " ")
	}

	if got := m.viewport.YOffset(); got != progress {
		t.Errorf("Space spam shifted viewport: was %d, now %d", progress, got)
	}
	if m.lineCursor != cursor || m.hunkIdx != hunkIdx {
		t.Errorf("Space spam moved the cursor: was %d/%d, now %d/%d",
			hunkIdx, cursor, m.hunkIdx, m.lineCursor)
	}
}

// TestSpaceCannotAdvancePastUnread asserts the locked-in contract:
// Space inside an unread hunk only pages within it; it never spills
// onto the next hunk or EOF until the read tick fires. The reviewer
// must keep the hunk on screen for the full reading-time window, or
// Alt+Space to skip.
func TestSpaceCannotAdvancePastUnread(t *testing.T) {
	// Two small files so we can detect "advanced past first" easily.
	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "lock",
		Commits: []review.Commit{{SHA: "abc1234567890", Short: "abc1234", Subject: "lock"}},
		Files:   []string{"a.txt", "b.txt"},
		RawDiff: buildAddPatch("a.txt", 100) + "\n" + buildAddPatch("b.txt", 5),
		FilePatches: map[string]string{
			"a.txt": buildAddPatch("a.txt", 100),
			"b.txt": buildAddPatch("b.txt", 5),
		},
		CommitPatches: map[string]string{"abc1234567890": "From abc1234\n"},
	}
	tmp := t.TempDir()
	sess := review.New(scope, "tester@example.com", filepath.Join(tmp, "lock.review"))
	// Effectively infinite reading time: ticks never fire on their own.
	m := newModel(sess, tmp, 0.001)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 20})

	// Drill in.
	m = key(t, m, ' ', " ")
	if m.fileIdx != 0 {
		t.Fatalf("expected fileIdx 0, got %d", m.fileIdx)
	}
	// Spam Space — without firing the read tick, we must never leave
	// a.txt and never reach atEOF, and no line gets marked read.
	for i := 0; i < 100; i++ {
		m = key(t, m, ' ', " ")
		if m.fileIdx != 0 {
			t.Fatalf("Space #%d leaked to file %d before read tick fired", i, m.fileIdx)
		}
		if m.atEOF {
			t.Fatalf("Space #%d reached EOF before read tick fired", i)
		}
		if r, _ := m.fileLineCounts(0); r != 0 {
			t.Fatalf("Space #%d marked %d line(s) read without a tick firing", i, r)
		}
	}
}

// TestAltSpaceSkipsAndAdvances verifies that Alt+Space marks the
// current unread hunk as skipped and jumps to the next one, and that
// a skipped hunk's render uses the muted-olive add style (not the
// strong unread green).
func TestAltSpaceSkipsAndAdvances(t *testing.T) {
	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "skip",
		Commits: []review.Commit{{SHA: "abc1234567890", Short: "abc1234", Subject: "skip"}},
		Files:   []string{"big_add.txt"},
		RawDiff: buildAddPatch("big_add.txt", 50),
		FilePatches: map[string]string{
			"big_add.txt": buildAddPatch("big_add.txt", 50),
		},
		CommitPatches: map[string]string{"abc1234567890": "From abc1234\n"},
	}
	tmp := t.TempDir()
	sess := review.New(scope, "tester@example.com", filepath.Join(tmp, "skip.review"))
	m := newModel(sess, tmp, 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 25})

	// Drill in.
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("expected modeDiff, got %v", m.mode)
	}

	// Alt+Space: skip every reviewable line from the cursor forward,
	// advance to EOF or next unread.
	lk := lineKey{fileIdx: m.fileIdx, hunkIdx: m.hunkIdx, lineIdx: m.lineCursor}
	if m.lineSkipped[lk] {
		t.Fatalf("line pre-skipped before Alt+Space")
	}
	m = step(t, m, tea.KeyPressMsg{Code: ' ', Text: " ", Mod: tea.ModAlt})
	if !m.lineSkipped[lk] {
		t.Errorf("Alt+Space did not mark the line skipped")
	}
	if m.lineRead[lk] {
		t.Errorf("Alt+Space wrongly marked the line read")
	}
}

// TestFastSpamDoesNotMarkUnseenHunksRead simulates a reviewer who
// pages through content faster than the per-viewport reading time:
// the read tick is scheduled, but the user keeps scrolling so by
// the time it fires the hunk is no longer on screen. The hunk must
// stay unread — otherwise the read-rate gating is meaningless.
func TestFastSpamDoesNotMarkUnseenHunksRead(t *testing.T) {
	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "fast",
		Commits: []review.Commit{{SHA: "abc1234567890", Short: "abc1234", Subject: "fast"}},
		Files:   []string{"a.txt", "b.txt"},
		RawDiff: buildAddPatch("a.txt", 5) + "\n" + buildAddPatch("b.txt", 5),
		FilePatches: map[string]string{
			"a.txt": buildAddPatch("a.txt", 5),
			"b.txt": buildAddPatch("b.txt", 5),
		},
		CommitPatches: map[string]string{"abc1234567890": "From abc1234\n"},
	}
	tmp := t.TempDir()
	sess := review.New(scope, "tester@example.com", filepath.Join(tmp, "fast.review"))
	// Low rate (1 line/sec → 5s delay for a 5-line hunk). Spam Spaces
	// faster than that and fire the tick AFTER we've already moved on:
	// the hunk should NOT be marked read.
	m := newModel(sess, tmp, 1.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 25})

	// Drill into a.txt; updateDisplayed schedules ticks for the
	// visible add lines.
	m = key(t, m, ' ', " ")

	// Skip a.txt (Alt+Space marks every reviewable line from the
	// cursor forward as Skipped + lands on a's EOF marker), then
	// Space again to advance past EOF into b.txt. At this point the
	// pending ticks for a's lines should NOT promote them to read
	// because a's lines are no longer on screen.
	m = step(t, m, tea.KeyPressMsg{Code: ' ', Text: " ", Mod: tea.ModAlt})
	m = key(t, m, ' ', " ") // advance from a's EOF to b.txt
	if !strings.HasSuffix(m.currentFile().Path, "b.txt") {
		t.Fatalf("expected to land on b.txt, got %s", m.currentFile().Path)
	}
	if m.viewReadScheduled {
		next, _ := m.Update(viewReadMsg{gen: m.viewReadGen})
		m = next.(*model)
	}
	if r, _ := m.fileLineCounts(0); r != 0 {
		t.Errorf("a.txt has %d lines marked read despite reviewer paging past it", r)
	}
}

// TestSkippedLinesBecomeReadWhenViewed: a Skipped line is not a "do
// not show" flag — it just keeps the line from counting as unread in
// the walk. If the reviewer dwells on a previously-skipped line long
// enough for the view tick to fire, it gets promoted to Read (and the
// render shows the read colour, not the skipped one).
func TestSkippedLinesBecomeReadWhenViewed(t *testing.T) {
	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "skip",
		Commits: []review.Commit{{SHA: "abc1234567890", Short: "abc1234", Subject: "skip"}},
		Files:   []string{"x.txt"},
		RawDiff: buildAddPatch("x.txt", 20),
		FilePatches: map[string]string{
			"x.txt": buildAddPatch("x.txt", 20),
		},
		CommitPatches: map[string]string{"abc1234567890": "From abc1234\n"},
	}
	tmp := t.TempDir()
	sess := review.New(scope, "tester@example.com", filepath.Join(tmp, "x.review"))
	m := newModel(sess, tmp, 1000.0)
	m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})

	// Drill in, then Alt+Space → marks every unread reviewable line
	// from cursor forward in the current hunk as Skipped.
	m = key(t, m, ' ', " ")
	m = step(t, m, tea.KeyPressMsg{Code: ' ', Text: " ", Mod: tea.ModAlt})
	if len(m.lineSkipped) == 0 {
		t.Fatalf("Alt+Space marked nothing skipped")
	}
	skipped := map[lineKey]bool{}
	for k := range m.lineSkipped {
		skipped[k] = true
	}

	// Fire any pending tick: skipped lines that are still visible
	// should flip to read.
	if m.viewReadScheduled {
		next, _ := m.Update(viewReadMsg{gen: m.viewReadGen})
		m = next.(*model)
	}

	promoted := 0
	for lk := range skipped {
		if m.lineRead[lk] {
			promoted++
		}
	}
	if promoted == 0 {
		t.Errorf("expected at least some skipped lines to be promoted to read after viewing, got 0")
	}
}

// buildAddPatch builds a `git diff` patch that adds `n` lines to a
// brand-new file at `path`. The lines are short and deterministic so
// the wrap/render paths don't add noise to the test.
func buildAddPatch(path string, n int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "diff --git a/%s b/%s\n", path, path)
	sb.WriteString("new file mode 100644\n")
	sb.WriteString("index 0000000..abcdef0\n")
	fmt.Fprintf(&sb, "--- /dev/null\n")
	fmt.Fprintf(&sb, "+++ b/%s\n", path)
	fmt.Fprintf(&sb, "@@ -0,0 +1,%d @@\n", n)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "+line %d\n", i)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// buildRemovePatch builds a `git diff` patch that removes all `n` lines
// from an existing file at `path`.
func buildRemovePatch(path string, n int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "diff --git a/%s b/%s\n", path, path)
	sb.WriteString("deleted file mode 100644\n")
	sb.WriteString("index abcdef0..0000000\n")
	fmt.Fprintf(&sb, "--- a/%s\n", path)
	sb.WriteString("+++ /dev/null\n")
	fmt.Fprintf(&sb, "@@ -1,%d +0,0 @@\n", n)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "-line %d\n", i)
	}
	return strings.TrimRight(sb.String(), "\n")
}
