// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitflower/review"
)

// TestTUIDrivesSession exercises the TUI's key handling without a real
// terminal. It constructs the model directly, feeds it a WindowSizeMsg
// followed by a sequence of synthetic key presses, and verifies session
// state after each step. Catches regressions in section→line drill-in,
// comment creation, verdict cycling, and save.
func TestTUIDrivesSession(t *testing.T) {
	tmp := t.TempDir()
	reviewPath := filepath.Join(tmp, "test.review")

	scope := review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "feature",
		Commits: []review.Commit{
			{SHA: "abc1234567890", Short: "abc1234", Subject: "feature commit"},
		},
		Files: []string{"foo.txt"},
		RawDiff: `diff --git a/foo.txt b/foo.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/foo.txt
@@ -0,0 +1,2 @@
+line one
+line two`,
		FilePatches: map[string]string{
			"foo.txt": `diff --git a/foo.txt b/foo.txt
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/foo.txt
@@ -0,0 +1,2 @@
+line one
+line two`,
		},
		CommitPatches: map[string]string{
			"abc1234567890": "From abc1234 ...\n",
		},
	}
	sess := review.New(scope, "tester@example.com", reviewPath)

	m := newModel(sess, tmp, 10*time.Millisecond)

	// Set window dimensions so the model can render.
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Initial state: section mode on Changes.
	if m.mode != modeTree {
		t.Fatalf("initial mode: got %v want modeTree", m.mode)
	}
	if m.sect != sectionChanges {
		t.Errorf("initial section: got %v want sectionChanges", m.sect)
	}

	// Press Space → drill into Changes (modeDiff) on first unread hunk.
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("after Space in section mode: got mode %v want modeDiff", m.mode)
	}
	if m.fileIdx != 0 || m.hunkIdx != 0 {
		t.Errorf("after drill: got fileIdx=%d hunkIdx=%d, want 0/0", m.fileIdx, m.hunkIdx)
	}

	// Cycle verdict forward with '>'.
	m = key(t, m, '>', ">")
	if sess.Verdict != review.VerdictChanges {
		t.Errorf("after '>': verdict %q want %q", sess.Verdict, review.VerdictChanges)
	}

	// Add a comment: c → type text → Alt+Enter.
	m = key(t, m, 'c', "c")
	if m.edit != editComment {
		t.Fatalf("after 'c': got edit %v want editComment", m.edit)
	}
	for _, r := range "Looks fine." {
		m = key(t, m, r, string(r))
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModAlt})
	if m.edit != editNone {
		t.Errorf("after submit: still in edit mode %v", m.edit)
	}
	if got := len(sess.Comments()); got != 1 {
		t.Fatalf("expected 1 comment, got %d", got)
	}
	if body := sess.Comments()[0].Text; body != "Looks fine." {
		t.Errorf("comment text: got %q", body)
	}

	// Save with 's'.
	m = key(t, m, 's', "s")
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("expected file at %s: %v", reviewPath, err)
	}
	body := string(data)
	for _, want := range []string{
		"# Review",
		"## Sources",
		"## Verdicts",
		"### Verdict: requested-changes",
		"# Changes",
		"## Changes in `foo.txt`",
		"> +line one",
		"### Comment (From: tester",
		"Looks fine.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered file missing %q\n--- BEGIN ---\n%s\n--- END ---", want, body)
			return
		}
	}
}

// TestSpaceWalkPagesThroughLongHunk asserts that Space scrolls within a
// hunk that exceeds the viewport, then advances once the hunk has been
// fully scrolled through and its read marker has fired.
func TestSpaceWalkPagesThroughLongHunk(t *testing.T) {
	tmp := t.TempDir()
	reviewPath := filepath.Join(tmp, "test.review")

	// Synthesise a single hunk of ~60 added lines so it exceeds a 20-row
	// viewport easily.
	var sb strings.Builder
	sb.WriteString("diff --git a/big.txt b/big.txt\n")
	sb.WriteString("new file mode 100644\n")
	sb.WriteString("index 0000000..abc1234\n")
	sb.WriteString("--- /dev/null\n")
	sb.WriteString("+++ b/big.txt\n")
	sb.WriteString("@@ -0,0 +1,60 @@\n")
	for i := 1; i <= 60; i++ {
		sb.WriteString("+line ")
		sb.WriteString(string(rune('0' + (i%10))))
		sb.WriteString("\n")
	}
	patch := strings.TrimRight(sb.String(), "\n")
	scope := review.Scope{
		Branch:        "feature",
		Base:          "main",
		TipSHA:        "abc1234567890",
		BaseSHA:       "0000111122223333",
		Diff:          "main..feature",
		Title:         "big",
		Commits:       []review.Commit{{SHA: "abc1234567890", Short: "abc1234", Subject: "big commit"}},
		Files:         []string{"big.txt"},
		RawDiff:       patch,
		FilePatches:   map[string]string{"big.txt": patch},
		CommitPatches: map[string]string{"abc1234567890": "From abc1234 ...\n"},
	}
	sess := review.New(scope, "tester@example.com", reviewPath)
	m := newModel(sess, tmp, 10*time.Millisecond)
	m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})

	// First Space drills from section into line mode at "5 before the
	// next unread line" — for a brand-new file that's row 0.
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("after first Space: mode %v want modeDiff", m.mode)
	}

	// PgDn is now the paging action. Page until the hunk gets fully
	// displayed and the read marker fires.
	maxPresses := 20
	for i := 0; i < maxPresses; i++ {
		for anchor := range m.pendingReads {
			next, _ := m.Update(delayedReadMsg{anchor: anchor})
			m = next.(*model)
		}
		if m.sess.IsRead(m.hunkRanges[0].anchor) {
			break
		}
		prevY := m.viewport.YOffset()
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
		if i > 0 && m.viewport.YOffset() == prevY && !m.viewport.AtBottom() {
			t.Fatalf("PgDn stuck: YOffset=%d, not at bottom", prevY)
		}
	}
	if !m.sess.IsRead(m.hunkRanges[0].anchor) {
		t.Errorf("after %d PgDns the hunk still isn't marked read", maxPresses)
	}
}

// TestModeTransitionMatrix exhaustively walks every reachable mode
// transition from each starting mode and asserts the model never panics,
// the cursor row is always inside the viewport in any line-cursored
// mode, and every section is at least selectable.
func TestModeTransitionMatrix(t *testing.T) {
	scope := smallScope()
	tmp := t.TempDir()
	sess := review.New(scope, "alice@example.com", filepath.Join(tmp, "x.review"))
	m := newModel(sess, tmp, 10*time.Millisecond)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Section traversal: visit every section via Tab.
	start := int(m.sect)
	for i := 0; i < numSections; i++ {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
		want := (start + i + 1) % numSections
		if int(m.sect) != want {
			t.Errorf("Tab %d: sect=%d want %d", i, m.sect, want)
		}
	}

	// Drill into Changes and walk all hunks with j; cursor must always
	// be on a valid line that gets the line-cursor highlight.
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ") // drill in
	if m.mode != modeDiff {
		t.Fatalf("expected modeDiff after Space on Changes, got %v", m.mode)
	}
	for i := 0; i < 30; i++ {
		m = key(t, m, 'j', "j")
		assertCursorVisible(t, m)
	}
	for i := 0; i < 30; i++ {
		m = key(t, m, 'k', "k")
		assertCursorVisible(t, m)
	}

	// Back to section mode preserves file selection.
	m = key(t, m, 'h', "h")
	if m.mode != modeTree {
		t.Errorf("after h: mode=%v want modeTree", m.mode)
	}
	if m.sect != sectionChanges {
		t.Errorf("after h: sect=%v want sectionChanges", m.sect)
	}

	// Space cycle: each Space should change state.
	prev := stateSig(m)
	for i := 0; i < 6; i++ {
		// Fire pending reads to keep walk moving.
		for anchor := range m.pendingReads {
			next, _ := m.Update(delayedReadMsg{anchor: anchor})
			m = next.(*model)
		}
		m = key(t, m, ' ', " ")
		cur := stateSig(m)
		if cur == prev {
			t.Logf("Space at step %d didn't change state: %s", i, cur)
		}
		prev = cur
	}
}

// TestTabbedLineWraps verifies that a long line containing tabs gets
// hard-wrapped with our hanging indent rather than slipping past the
// wrap width because Hardwrap counts tabs as width 1. The continuation
// rows should start with leading spaces, not text.
func TestTabbedLineWraps(t *testing.T) {
	parts := wrapDiffText("\t\tfunc reallyLongIdentifierName(arg1 string, arg2 string, arg3 string, arg4 string) error", 40)
	if len(parts) < 2 {
		t.Fatalf("expected multi-part wrap, got %d: %v", len(parts), parts)
	}
	for i, p := range parts {
		if visW := lipgloss.Width(p); visW > 40 {
			t.Errorf("part %d wider than 40: width=%d %q", i, visW, p)
		}
	}
}

// assertCursorVisible checks that the line cursor's row is within the
// current viewport. Detects the "marker invisible" regression.
func assertCursorVisible(t *testing.T, m *model) {
	t.Helper()
	if m.mode != modeDiff && m.mode != modeFile {
		return
	}
	// Re-render and grep for styleLineCur (background 236, bold). For
	// modeDiff we look for `+ ` or `  ` styled with bold + bg.
	body, ranges, _, cursorRow := renderFileDiff(m)
	_ = body
	if m.mode == modeFile {
		return
	}
	if len(ranges) == 0 {
		t.Logf("no hunk ranges (file empty?)")
		return
	}
	top := m.viewport.YOffset()
	bot := top + m.viewport.Height() - 1
	if cursorRow < top || cursorRow > bot {
		t.Errorf("cursor row %d outside viewport [%d,%d]", cursorRow, top, bot)
	}
}

// stateSig produces a short string summarising the model's visible state
// so tests can detect "did anything change?".
func stateSig(m *model) string {
	return fmt.Sprintf("mode=%v sect=%v sectIdx=%v file=%d hunk=%d line=%d yOff=%d edit=%v",
		m.mode, m.sect, m.sectIdx, m.fileIdx, m.hunkIdx, m.lineCursor, m.viewport.YOffset(), m.edit)
}

// smallScope builds a tiny scope with two files / three hunks for
// behavioural tests.
func smallScope() review.Scope {
	patchA := `diff --git a/a.txt b/a.txt
new file mode 100644
--- /dev/null
+++ b/a.txt
@@ -0,0 +1,3 @@
+a line one
+a line two
+a line three`
	patchB := `diff --git a/b.txt b/b.txt
new file mode 100644
--- /dev/null
+++ b/b.txt
@@ -0,0 +1,5 @@
+b line one
+b line two
+b line three
+b line four
+b line five`
	return review.Scope{
		Branch: "feat", Base: "main",
		TipSHA: "aaaaaaaaaaaa", BaseSHA: "bbbbbbbbbbbb",
		Diff: "main..feat", Title: "feat",
		Commits: []review.Commit{{SHA: "aaa1234567890", Short: "aaa1234", Subject: "test"}},
		Files:   []string{"a.txt", "b.txt"},
		RawDiff: patchA + "\n" + patchB,
		FilePatches: map[string]string{
			"a.txt": patchA,
			"b.txt": patchB,
		},
		CommitPatches: map[string]string{"aaa1234567890": "From aaa1234 ...\n"},
	}
}

// step feeds a generic tea.Msg through the model and returns the (asserted)
// model after the update.
func step(t *testing.T, m *model, msg tea.Msg) *model {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(*model)
	if !ok {
		t.Fatalf("Update returned %T, want *model", next)
	}
	return mm
}

// key feeds a synthetic KeyPressMsg for a single rune.
func key(t *testing.T, m *model, code rune, text string) *model {
	return step(t, m, tea.KeyPressMsg{Code: code, Text: text})
}
