// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markis.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

// TestSpaceWalkOnSelfRepoInProcess constructs the model against the real
// self-repo diff (main..experiments/stack-review) and calls spaceWalk in
// a loop. Same scenario as the PTY test, but in-process so we get cheap
// access to internal state for diagnostics.
func TestSpaceWalkOnSelfRepoInProcess(t *testing.T) {
	repo := "/tmp/gitflower-self-test-repo"
	if _, err := os.Stat(repo); err != nil {
		setup := filepath.Join("..", "test", "e2e", "setup-self.sh")
		out, err := exec.Command(setup, repo).CombinedOutput()
		if err != nil {
			t.Skipf("setup-self.sh unavailable: %v\n%s", err, out)
		}
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	scope, err := review.ScopeFor("experiments/stack-review", "main")
	if err != nil {
		t.Skipf("ScopeFor: %v", err)
	}
	t.Logf("scope: %d files, %d commits", len(scope.Files), len(scope.Commits))

	tmp := t.TempDir()
	reviewPath := filepath.Join(tmp, "test.review")
	sess := review.New(*scope, "reviewer@example.com", reviewPath)

	m := newModel(sess, repo, 1*time.Millisecond)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	var trace strings.Builder
	traced := 0
	debugSpaceWalk = func(stage string, mm *model) {
		if traced > 200 {
			return
		}
		traced++
		fmt.Fprintf(&trace, "[%s] %s\n", stage, stateSig(mm))
	}
	defer func() {
		debugSpaceWalk = nil
		if t.Failed() {
			t.Logf("spaceWalk trace (first 200):\n%s", trace.String())
		}
	}()

	t.Logf("model: %d parsed files", len(m.files))

	// Drill in.
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("expected modeDiff, got %v", m.mode)
	}

	// Spam space and watch what changes.
	const maxPresses = 500
	stuckCount := 0
	readByFile := map[string]int{}
	for i := 0; i < maxPresses; i++ {
		// Fire any pending read ticks deterministically.
		for anchor := range m.pendingReads {
			next, _ := m.Update(delayedReadMsg{anchor: anchor})
			m = next.(*model)
		}

		sigBefore := stateSig(m)

		m = key(t, m, ' ', " ")

		if stateSig(m) == sigBefore {
			stuckCount++
			if stuckCount > 3 {
				t.Logf("STUCK at press %d: %s hunkRanges=%d pending=%d",
					i, sigBefore, len(m.hunkRanges), len(m.pendingReads))
				if f := m.currentFile(); f != nil && f.Path != "" {
					t.Logf("  file: %s hunks=%d", f.Path, len(f.Hunks))
				}
				if h := m.currentHunk(); h != nil {
					a := review.HunkAnchor(m.currentFile().Path, h.NewStart, h.NewLines)
					t.Logf("  hunk anchor=%q isRead=%v", a, m.sess.IsRead(a))
				}
				t.Fatalf("walk stuck after %d press(es) with no state change", i)
			}
		} else {
			stuckCount = 0
		}

		// Did we reach the end? (mode goes back to modeTree on verdict open)
		if m.mode == modeTree && m.edit == editSummary {
			t.Logf("reached verdict editor after %d press(es)", i+1)
			break
		}
	}
	_ = readByFile

	// Tally.
	totalRead := 0
	totalHunks := 0
	for _, f := range m.files {
		for _, h := range f.Hunks {
			totalHunks++
			a := review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
			if m.sess.IsRead(a) {
				totalRead++
			}
		}
	}
	t.Logf("read %d / %d hunks after spaceWalk loop", totalRead, totalHunks)
	if totalRead < totalHunks*70/100 {
		var report strings.Builder
		for _, f := range m.files {
			read := 0
			for _, h := range f.Hunks {
				a := review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
				if m.sess.IsRead(a) {
					read++
				}
			}
			fmt.Fprintf(&report, "  %-50s %d/%d\n", f.Path, read, len(f.Hunks))
		}
		t.Errorf("only %d/%d hunks read; per-file:\n%s", totalRead, totalHunks, report.String())
	}
}

// TestPageScrollKeepsCursorVisible scrolls a long file all the way down
// with PgDn and then back up with PgUp, asserting that the line cursor's
// rendered row is always inside the viewport — never "stuck" at the
// bottom (or anywhere off-screen).
func TestPageScrollKeepsCursorVisible(t *testing.T) {
	repo := "/tmp/gitflower-self-test-repo"
	if _, err := os.Stat(repo); err != nil {
		setup := filepath.Join("..", "test", "e2e", "setup-self.sh")
		out, err := exec.Command(setup, repo).CombinedOutput()
		if err != nil {
			t.Skipf("setup-self.sh unavailable: %v\n%s", err, out)
		}
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	scope, err := review.ScopeFor("experiments/stack-review", "main")
	if err != nil {
		t.Skipf("ScopeFor: %v", err)
	}

	tmp := t.TempDir()
	reviewPath := filepath.Join(tmp, "test.review")
	sess := review.New(*scope, "reviewer@example.com", reviewPath)
	m := newModel(sess, repo, 1*time.Millisecond)
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})

	// Find the longest file and drill into it.
	longest := 0
	for i, f := range m.files {
		if len(f.Hunks) > 0 {
			rows := 0
			for _, h := range f.Hunks {
				rows += len(h.Lines)
			}
			if rows > 200 {
				longest = i
				break
			}
		}
	}
	m.sectIdx[sectionChanges] = longest
	m.fileIdx = longest
	m.hunkIdx = 0
	m.atEOF = false
	m.mode = modeDiff
	m.lineCursor = 0
	m.refreshViewport()

	totalRows := 0
	for _, lr := range m.lineRanges {
		if lr.botRow > totalRows {
			totalRows = lr.botRow
		}
	}
	t.Logf("file %s totalRows=%d height=%d", m.files[longest].Path, totalRows, m.viewport.Height())

	assertCursorOnTop := func(label string) {
		t.Helper()
		top := m.viewport.YOffset()
		bot := top + m.viewport.Height() - 1
		var lr *lineRange
		if m.atEOF {
			lr = m.eofRange()
		} else {
			for i := range m.lineRanges {
				r := &m.lineRanges[i]
				if !r.isEOF && r.hunkIdx == m.hunkIdx && r.lineIdx == m.lineCursor {
					lr = r
					break
				}
			}
		}
		if lr == nil {
			t.Fatalf("%s: no lineRange for cursor (hunk=%d line=%d atEOF=%v)",
				label, m.hunkIdx, m.lineCursor, m.atEOF)
		}
		if lr.botRow < top || lr.topRow > bot {
			t.Errorf("%s: cursor off-screen — cursor rows [%d..%d], viewport [%d..%d]",
				label, lr.topRow, lr.botRow, top, bot)
		}
		// Cursor should sit on the topmost row of the viewport — except
		// at EOF, where the marker naturally sits at the bottom of the
		// view because there's nothing further to scroll into.
		if !m.atEOF && lr.topRow != top {
			t.Errorf("%s: cursor not aligned to top — cursor topRow=%d viewport top=%d",
				label, lr.topRow, top)
		}
	}

	hitEOFAt := -1
	for i := 0; i < 400; i++ {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
		assertCursorOnTop(fmt.Sprintf("pgdown #%d (yOffset=%d)", i, m.viewport.YOffset()))
		if hitEOFAt < 0 && m.atEOF {
			hitEOFAt = i
		}
	}
	if hitEOFAt < 0 {
		t.Errorf("never reached EOF after 400 PgDn on %s (totalRows=%d)",
			m.files[longest].Path, totalRows)
	}
	for i := 0; i < 400; i++ {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
		assertCursorOnTop(fmt.Sprintf("pgup #%d (yOffset=%d atEOF=%v)", i, m.viewport.YOffset(), m.atEOF))
		if m.viewport.YOffset() == 0 {
			break
		}
	}

	// Now do the same in section mode (tree peek of Changes). The marker
	// should move with PgDn even though we never left section mode.
	m.mode = modeTree
	m.sect = sectionChanges
	m.atEOF = false
	m.hunkIdx = 0
	m.lineCursor = 0
	m.refreshViewport()
	for i := 0; i < 100; i++ {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
		assertCursorOnTop(fmt.Sprintf("section pgdown #%d (yOffset=%d)", i, m.viewport.YOffset()))
		if m.atEOF {
			break
		}
	}
}
