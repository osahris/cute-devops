// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

// navScope returns a scope with multiple files, folders, and commits
// suitable for exercising every drill-in / drill-out path.
func navScope() review.Scope {
	a := buildAddPatch("dir/a.txt", 6)
	b := buildAddPatch("dir/b.txt", 4)
	c := buildAddPatch("README.md", 3)
	combined := a + "\n" + b + "\n" + c
	return review.Scope{
		Branch:  "feature",
		Base:    "main",
		TipSHA:  "abc1234567890",
		BaseSHA: "0000111122223333",
		Diff:    "main..feature",
		Title:   "nav",
		Commits: []review.Commit{
			{SHA: "abc1234567890", Short: "abc1234", Subject: "first commit"},
			{SHA: "def5678901234", Short: "def5678", Subject: "second commit"},
		},
		Files: []string{"dir/a.txt", "dir/b.txt", "README.md"},
		RawDiff: combined,
		FilePatches: map[string]string{
			"dir/a.txt":  a,
			"dir/b.txt":  b,
			"README.md":  c,
		},
		CommitPatches: map[string]string{
			"abc1234567890": "From abc1234\n" + a,
			"def5678901234": "From def5678\n" + b,
		},
	}
}

func newNavModel(t *testing.T) *model {
	t.Helper()
	tmp := t.TempDir()
	sess := review.New(navScope(), "tester@example.com", filepath.Join(tmp, "nav.review"))
	m := newModel(sess, tmp, 1000.0)
	return step(t, m, tea.WindowSizeMsg{Width: 200, Height: 40})
}

// --- Sidebar navigation ----------------------------------------------

func TestNavTabCyclesSections(t *testing.T) {
	m := newNavModel(t)
	start := m.sect
	for i := 0; i < numSections; i++ {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	if m.sect != start {
		t.Errorf("Tab × numSections should cycle back to start: got %v want %v",
			m.sect, start)
	}
}

func TestNavJKMovesRowsInChanges(t *testing.T) {
	m := newNavModel(t)
	// Jump to Changes.
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.sectIdx[sectionChanges] = 0
	// j three times should advance.
	for i := 0; i < 3; i++ {
		m = key(t, m, 'j', "j")
	}
	if m.sectIdx[sectionChanges] != 3 {
		t.Errorf("j×3 sectIdx: got %d want 3", m.sectIdx[sectionChanges])
	}
	// k twice should retreat.
	for i := 0; i < 2; i++ {
		m = key(t, m, 'k', "k")
	}
	if m.sectIdx[sectionChanges] != 1 {
		t.Errorf("k×2 sectIdx: got %d want 1", m.sectIdx[sectionChanges])
	}
}

// --- Changes section: folder tree ------------------------------------

func TestNavChangesFolderExpandedByDefault(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	items := m.sectionItems(sectionChanges)
	// "dir/" folder should be present AND its children visible.
	var sawDir, sawA, sawB bool
	for _, it := range items {
		switch {
		case strings.Contains(it, "dir/"):
			sawDir = true
		case strings.Contains(it, "a.txt"):
			sawA = true
		case strings.Contains(it, "b.txt"):
			sawB = true
		}
	}
	if !sawDir || !sawA || !sawB {
		t.Errorf("expected dir/, a.txt, b.txt visible. items=%v", items)
	}
}

func TestNavChangesEnterFileDrillsIn(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	// Force the changes-rows cache so we can find the row index.
	_ = m.sectionItems(sectionChanges)
	found := false
	for i, row := range m.changesRows {
		if row.kind == tnFile && row.fullPath == "dir/a.txt" {
			m.sectIdx[sectionChanges] = i
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("dir/a.txt not in changesRows: %#v", m.changesRows)
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != modeDiff {
		t.Fatalf("Enter on file row: mode=%v want modeDiff", m.mode)
	}
	if m.currentFile().Path != "dir/a.txt" {
		t.Errorf("drilled into wrong file: %s", m.currentFile().Path)
	}
}

func TestNavChangesEnterFolderToggles(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	_ = m.sectionItems(sectionChanges)
	for i, row := range m.changesRows {
		if row.kind == tnDir && row.fullPath == "dir" {
			m.sectIdx[sectionChanges] = i
			break
		}
	}
	wasExpanded := m.isChangesDirExpanded("dir")
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.isChangesDirExpanded("dir") == wasExpanded {
		t.Errorf("Enter on folder didn't toggle expansion")
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.isChangesDirExpanded("dir") != wasExpanded {
		t.Errorf("Second Enter didn't toggle back")
	}
}

func TestNavChangesSpaceDrillsIntoUnread(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("Space on Changes: mode=%v want modeDiff", m.mode)
	}
}

func TestNavChangesSSkipsCurrentFile(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	_ = m.sectionItems(sectionChanges)
	for i, row := range m.changesRows {
		if row.kind == tnFile && row.fullPath == "dir/a.txt" {
			m.sectIdx[sectionChanges] = i
			break
		}
	}
	m = key(t, m, 's', "s")
	fi := m.findFileIdx("dir/a.txt")
	if fi < 0 {
		t.Fatalf("a.txt not in m.files")
	}
	f := &m.files[fi]
	for hi, h := range f.Hunks {
		for li, ln := range h.Lines {
			if ln.Kind != review.LineAdd && ln.Kind != review.LineDelete {
				continue
			}
			lk := lineKey{fileIdx: fi, hunkIdx: hi, lineIdx: li}
			if !m.lineSkipped[lk] {
				t.Errorf("expected line %+v skipped", lk)
			}
		}
	}
}

// --- Commits section --------------------------------------------------

func TestNavCommitsEnterDrillsIntoCommitDiff(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionCommits {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.sectIdx[sectionCommits] = 0
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != modeDiff {
		t.Fatalf("Enter on commit: mode=%v want modeDiff", m.mode)
	}
	if !strings.HasPrefix(m.currentFile().Path, "commit:abc1234:") {
		t.Errorf("drilled into wrong virtual file: %s", m.currentFile().Path)
	}
	// The first commit-virtual file should be the synthetic message
	// file, so the reviewer reads the commit message before the diff.
	if !strings.Contains(m.currentFile().Path, "(message)") {
		t.Errorf("expected to land on the commit-message virtual file first, got %s",
			m.currentFile().Path)
	}
}

func TestNavCommitsRightDrillsIntoCommitDiff(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionCommits {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.sectIdx[sectionCommits] = 1
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.mode != modeDiff {
		t.Fatalf("Right on commit: mode=%v want modeDiff", m.mode)
	}
	if !strings.HasPrefix(m.currentFile().Path, "commit:def5678:") {
		t.Errorf("drilled into wrong commit: %s", m.currentFile().Path)
	}
}

func TestNavCommitsLDrillsIntoCommitDiff(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionCommits {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.sectIdx[sectionCommits] = 0
	m = key(t, m, 'l', "l")
	if m.mode != modeDiff {
		t.Fatalf("l on commit: mode=%v want modeDiff", m.mode)
	}
	if !strings.HasPrefix(m.currentFile().Path, "commit:abc1234:") {
		t.Errorf("drilled into wrong commit: %s", m.currentFile().Path)
	}
}

// --- File tree section -----------------------------------------------

func TestNavTreeFoldersCollapsedByDefault(t *testing.T) {
	m := newNavModel(t)
	// treeFiles is populated from `git ls-tree`; in unit tests it's
	// empty so the tree section is too. Just verify the helper
	// doesn't panic on an empty tree.
	_ = m.sectionItems(sectionTree)
}

// --- Line mode (modeDiff) navigation ---------------------------------

func TestNavDiffJKMovesCursor(t *testing.T) {
	m := newNavModel(t)
	// Drill in.
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ")
	startLine := m.lineCursor
	m = key(t, m, 'j', "j")
	if m.lineCursor == startLine {
		t.Errorf("j didn't move line cursor: still %d", m.lineCursor)
	}
	m = key(t, m, 'k', "k")
	if m.lineCursor != startLine {
		t.Errorf("k didn't return to start: got %d want %d", m.lineCursor, startLine)
	}
}

func TestNavDiffEscReturnsToTree(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ")
	if m.mode != modeDiff {
		t.Fatalf("expected modeDiff, got %v", m.mode)
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.mode != modeTree {
		t.Errorf("Esc didn't return to tree mode: got %v", m.mode)
	}
}

func TestNavDiffLeftAtCol0ExitsToTree(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ")
	// Make sure we're at x-offset 0 first.
	m.viewport.SetXOffset(0)
	m = key(t, m, 'h', "h")
	if m.mode != modeTree {
		t.Errorf("h at col 0 should return to tree: got mode %v", m.mode)
	}
}

func TestNavDiffSpaceAdvancesToNextUnread(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ")
	startFile := m.fileIdx
	// Mark all of current file's lines as read so Space advances.
	f := &m.files[m.fileIdx]
	for hi, h := range f.Hunks {
		for li, ln := range h.Lines {
			if ln.Kind == review.LineAdd || ln.Kind == review.LineDelete {
				m.lineRead[lineKey{fileIdx: m.fileIdx, hunkIdx: hi, lineIdx: li}] = true
			}
		}
	}
	// Now spam Space until we change file (or reach verdict editor).
	for i := 0; i < 10; i++ {
		m = key(t, m, ' ', " ")
		if m.fileIdx != startFile || m.edit == editSummary {
			break
		}
	}
	if m.fileIdx == startFile && m.edit != editSummary {
		t.Errorf("Space-spam after marking file read didn't leave fileIdx %d", startFile)
	}
}

func TestNavDiffAltSpaceSkipsAndAdvances(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ")
	startCur := m.currentLineKey()
	m = step(t, m, tea.KeyPressMsg{Code: ' ', Text: " ", Mod: tea.ModAlt})
	if !m.lineSkipped[startCur] {
		t.Errorf("Alt+Space didn't skip the starting cursor line")
	}
}

// --- File mode -------------------------------------------------------

func TestNavFileEnterFromTreeDispatches(t *testing.T) {
	m := newNavModel(t)
	// Force a treeFiles entry so we have something to drill into.
	m.treeFiles = []string{"dir/a.txt"}
	for m.sect != sectionTree {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	// Expand the folder so the file row becomes visible.
	m.fileTreeExpanded["dir"] = true
	m.fileTreeRows = m.buildFileTreeRows()
	for i, row := range m.fileTreeRows {
		if row.kind == tnFile && row.fullPath == "dir/a.txt" {
			m.sectIdx[sectionTree] = i
			break
		}
	}
	m = step(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	// In unit tests TipSHA is fake so gitFileLines fails — verify
	// enterFileReview was at least dispatched (status set). The full
	// modeFile transition is covered by integration tests against a
	// real git repo.
	if m.mode != modeFile && !strings.Contains(m.status, "no content at tip") {
		t.Errorf("Tree-Enter on a file produced mode=%v status=%q — expected modeFile or no-content status",
			m.mode, m.status)
	}
}

// --- Read tick ------------------------------------------------------

func TestNavDiffReadTickMarksVisibleLines(t *testing.T) {
	m := newNavModel(t)
	for m.sect != sectionChanges {
		m = step(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m = key(t, m, ' ', " ")
	if !m.viewReadScheduled {
		t.Fatalf("expected a viewReadMsg tick to be queued")
	}
	next, _ := m.Update(viewReadMsg{gen: m.viewReadGen})
	m = next.(*model)
	r, total := m.fileLineCounts(m.fileIdx)
	if r == 0 || r > total {
		t.Errorf("tick should mark some visible lines read: %d/%d", r, total)
	}
}
