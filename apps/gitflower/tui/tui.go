// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Package tui is a bubbletea-v2 driver for review sessions. State changes
// go through *review.ReviewSession methods so a web driver can do the same.
// Every mutation auto-saves the file.
//
// Two user-facing modes:
//
//	Section mode (sidebar focused, one section selected; the section's
//	             content peeks in the right pane). Six sections, mirroring
//	             the .review file: Sources, Verdicts, General Issues,
//	             Changes, Commits, File Review.
//	Line mode    (cursor on exactly one line in the right pane; comments,
//	             questions, likes, dislikes anchor to that line). Internally
//	             modeDiff and modeFile differ only in content source.
package tui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitflower/review"
)

// DefaultReadDelay is how long a hunk must remain visible before
// the TUI emits a ReadStart/ReadEnd. Override at TUI construction.
const DefaultReadDelay = 1 * time.Second

// Run launches the TUI on sess. readDelay controls how long a hunk must stay
// visible before the read marker is emitted; pass 0 for DefaultReadDelay.
func Run(sess *review.ReviewSession, readDelay time.Duration) error {
	root, _ := gitRoot()
	if readDelay <= 0 {
		readDelay = DefaultReadDelay
	}
	m := newModel(sess, root, readDelay)
	_, err := tea.NewProgram(m).Run()
	return err
}

// ---------------------------------------------------------------------
// constants
// ---------------------------------------------------------------------

type mode int

const (
	modeTree mode = iota
	modeDiff
	modeFile
)

type section int

// Sections mirror the H1 chapters of the .review file plus the two
// sub-sections of `# Review` (Sources, Verdicts).
const (
	sectionSources section = iota
	sectionVerdicts
	sectionIssues
	sectionChanges
	sectionCommits
	sectionFileReview
)

const numSections = 6

func (s section) Label() string {
	switch s {
	case sectionSources:
		return "Sources"
	case sectionVerdicts:
		return "Verdicts"
	case sectionIssues:
		return "General Issues"
	case sectionChanges:
		return "Changes"
	case sectionCommits:
		return "Commits"
	case sectionFileReview:
		return "File Review"
	}
	return "?"
}

type editKind int

const (
	editNone editKind = iota
	editComment
	editQuestion
	editSummary
	editIssue
)

// ---------------------------------------------------------------------
// model
// ---------------------------------------------------------------------

type hunkRange struct {
	anchor         review.Anchor
	topRow, botRow int
}

type model struct {
	sess *review.ReviewSession
	root string

	// Static-ish data loaded at startup.
	files     []review.File // parsed from scope.RawDiff
	treeFiles []string      // files at tip SHA (`git ls-tree -r <tip>`)

	// Tree mode state.
	sect    section
	sectIdx [numSections]int // per-section item index

	// Diff mode state. Cursor is always on exactly one line.
	fileIdx    int
	hunkIdx    int
	lineCursor int // index into currentHunk().Lines

	// File review mode state. Cursor is always on exactly one line.
	filePath       string   // currently-open file in modeFile
	fileLines      []string // content of filePath at tip SHA
	fileLineCursor int

	// Edit overlay.
	edit       editKind
	editCmtIdx int // when editing existing comment; -1 = new
	editIssIdx int // when editing existing issue; -1 = new
	editAnchor review.Anchor
	editLabel  string
	confirmQuit bool

	mode          mode
	prevMode      mode // where to return after edit-overlay closes
	width, height int

	textarea textarea.Model
	title    textinput.Model // issue title
	viewport viewport.Model

	hunkRanges []hunkRange
	displayed  map[review.Anchor]map[int]bool

	// Delayed read marking.
	readDelay    time.Duration
	pendingReads map[review.Anchor]bool // anchor has a scheduled read tick

	// queuedCmds accumulate Cmds produced by non-Cmd-returning helpers
	// (refreshViewport, updateDisplayed); Update batches and drains them.
	queuedCmds []tea.Cmd

	status string
}

func newModel(sess *review.ReviewSession, root string, readDelay time.Duration) *model {
	files := review.ParseDiff(sess.Scope.RawDiff)

	ta := textarea.New()
	ta.CharLimit = 0
	ta.ShowLineNumbers = false

	ti := textinput.New()
	ti.Placeholder = "Issue title"
	ti.CharLimit = 200

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.SoftWrap = true       // default — long lines wrap visually
	vp.SetHorizontalStep(0)  // disable horizontal scroll while soft-wrapping

	treeFiles, _ := gitTreeFiles(sess.Scope.TipSHA)

	m := &model{
		sess:         sess,
		root:         root,
		files:        files,
		treeFiles:    treeFiles,
		mode:         modeTree,
		sect:         sectionChanges,
		editCmtIdx:   -1,
		editIssIdx:   -1,
		textarea:     ta,
		title:        ti,
		viewport:     vp,
		displayed:    map[review.Anchor]map[int]bool{},
		readDelay:    readDelay,
		pendingReads: map[review.Anchor]bool{},
	}
	return m
}

// ---------------------------------------------------------------------
// tea.Model
// ---------------------------------------------------------------------

func (m *model) Init() tea.Cmd { return nil }

// delayedReadMsg fires `readDelay` after a hunk first became fully visible.
// If the hunk is still pending (not user-unread) AND currently in the
// viewport, we mark it read.
type delayedReadMsg struct{ anchor review.Anchor }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.refreshViewport()
		return m, m.drainCmds()
	case delayedReadMsg:
		if m.pendingReads[msg.anchor] && m.isHunkCurrentlyVisible(msg.anchor) {
			m.sess.MarkRead(msg.anchor)
			_ = m.sess.Save()
		}
		delete(m.pendingReads, msg.anchor)
		return m, m.drainCmds()
	}

	var sub tea.Model = m
	var cmd tea.Cmd
	if m.confirmQuit {
		sub, cmd = m.updateConfirmQuit(msg)
	} else if m.edit != editNone {
		sub, cmd = m.updateEdit(msg)
	} else {
		switch m.mode {
		case modeTree:
			sub, cmd = m.updateTree(msg)
		case modeDiff:
			sub, cmd = m.updateDiff(msg)
		case modeFile:
			sub, cmd = m.updateFile(msg)
		}
	}
	if q := m.drainCmds(); q != nil {
		cmd = tea.Batch(q, cmd)
	}
	return sub, cmd
}

func (m *model) drainCmds() tea.Cmd {
	if len(m.queuedCmds) == 0 {
		return nil
	}
	cmds := m.queuedCmds
	m.queuedCmds = nil
	return tea.Batch(cmds...)
}

func (m *model) resize() {
	sbw := sidebarWidth(m.width)
	mainW := m.width - sbw - 2
	if mainW < 20 {
		mainW = 20
	}
	mainH := max(3, m.height-4)
	m.textarea.SetWidth(mainW)
	m.textarea.SetHeight(4)
	m.title.SetWidth(mainW)
	m.viewport.SetWidth(mainW)
	m.viewport.SetHeight(mainH)
}

// ---------------------------------------------------------------------
// global keys (quit, save, verdict)
// ---------------------------------------------------------------------

func (m *model) handleGlobal(key string) (tea.Cmd, bool) {
	switch key {
	case "ctrl+c", "ctrl+q":
		if m.sess.Dirty() {
			m.confirmQuit = true
			return nil, true
		}
		return tea.Quit, true
	case "q":
		if m.sess.Dirty() {
			m.confirmQuit = true
			return nil, true
		}
		return tea.Quit, true
	case "s":
		m.save("saved")
		return nil, true
	case ">":
		m.sess.SetVerdict(m.sess.Verdict.Next())
		m.save("verdict → " + string(m.sess.Verdict))
		return nil, true
	case "<":
		all := review.AllVerdicts
		for i, v := range all {
			if v == m.sess.Verdict {
				m.sess.SetVerdict(all[(i-1+len(all))%len(all)])
				m.save("verdict → " + string(m.sess.Verdict))
				return nil, true
			}
		}
		return nil, true
	case "V":
		m.openEdit(editSummary, "", -1, -1, "")
		m.textarea.SetValue(m.sess.Summary)
		return m.textarea.Focus(), true
	}
	return nil, false
}

func (m *model) save(s string) {
	if err := m.sess.Save(); err != nil {
		m.status = "save failed: " + err.Error()
		return
	}
	if s != "" {
		m.status = s
	}
}

// ---------------------------------------------------------------------
// tree mode
// ---------------------------------------------------------------------

// sectionItems returns the items of the given section.
// Diffs and Tree return file paths; Commits returns "shortSHA subject";
// Issues returns titles.
func (m *model) sectionItems(s section) []string {
	switch s {
	case sectionSources:
		return []string{
			"From: " + m.sess.Scope.Base,
			"To: " + m.sess.Scope.Branch,
			"Diff: " + m.sess.Scope.Diff,
			fmt.Sprintf("Commits: %d", len(m.sess.Scope.Commits)),
		}
	case sectionVerdicts:
		vs := m.sess.Verdicts()
		if len(vs) == 0 {
			return []string{string(m.sess.Verdict) + " (initial)"}
		}
		out := make([]string, len(vs))
		for i, v := range vs {
			out[i] = string(v.State) + "  " + v.Date
		}
		return out
	case sectionIssues:
		out := make([]string, len(m.sess.Issues()))
		for i, it := range m.sess.Issues() {
			out[i] = it.Title
		}
		return out
	case sectionChanges:
		out := make([]string, len(m.files))
		for i, f := range m.files {
			out[i] = f.Path
		}
		return out
	case sectionCommits:
		out := make([]string, len(m.sess.Scope.Commits))
		for i, c := range m.sess.Scope.Commits {
			out[i] = c.Short + "  " + c.Subject
		}
		return out
	case sectionFileReview:
		frs := m.sess.FileReviews()
		out := make([]string, len(frs))
		for i, fr := range frs {
			out[i] = fr.Path
		}
		return out
	}
	return nil
}

func (m *model) currentSectionItems() []string {
	return m.sectionItems(m.sect)
}

func (m *model) updateTree(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if cmd, done := m.handleGlobal(key.String()); done {
		return m, cmd
	}
	switch key.String() {
	case "j", "down":
		m.treeNext()
	case "k", "up":
		m.treePrev()
	case "tab":
		m.sect = section((int(m.sect) + 1) % 4)
	case "shift+tab":
		m.sect = section((int(m.sect) + 3) % 4)
	case "i":
		// New issue.
		m.openEdit(editIssue, "", -1, -1, "")
		return m, m.title.Focus()
	case "e":
		// Edit issue (only meaningful in Issues section).
		if m.sect != sectionIssues {
			m.status = "no editable item under cursor"
			return m, nil
		}
		idx := m.sectIdx[m.sect]
		issues := m.sess.Issues()
		if idx >= len(issues) {
			return m, nil
		}
		it := issues[idx]
		m.openEdit(editIssue, "", -1, idx, "")
		m.title.SetValue(it.Title)
		m.textarea.SetValue(it.Body)
		return m, m.title.Focus()
	case "right", "l", "enter":
		m.openSelectedItem()
	default:
		// Peek: scroll the right pane.
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.updateDisplayed()
		return m, cmd
	}
	return m, nil
}

func (m *model) treeNext() {
	items := m.currentSectionItems()
	if m.sectIdx[m.sect]+1 < len(items) {
		m.sectIdx[m.sect]++
	} else if int(m.sect)+1 < numSections {
		// advance to next non-empty section
		for s := int(m.sect) + 1; s < numSections; s++ {
			if len(m.sectionItems(section(s))) > 0 {
				m.sect = section(s)
				m.sectIdx[m.sect] = 0
				break
			}
		}
	}
	m.onTreeSelectionChanged()
}

func (m *model) treePrev() {
	if m.sectIdx[m.sect] > 0 {
		m.sectIdx[m.sect]--
	} else if int(m.sect) > 0 {
		for s := int(m.sect) - 1; s >= 0; s-- {
			items := m.sectionItems(section(s))
			if len(items) > 0 {
				m.sect = section(s)
				m.sectIdx[m.sect] = len(items) - 1
				break
			}
		}
	}
	m.onTreeSelectionChanged()
}

// onTreeSelectionChanged peeks the right pane content based on the selected item.
func (m *model) onTreeSelectionChanged() {
	switch m.sect {
	case sectionChanges:
		m.fileIdx = m.sectIdx[m.sect]
		m.hunkIdx = 0
	case sectionFileReview:
		idx := m.sectIdx[m.sect]
		frs := m.sess.FileReviews()
		if idx < len(frs) {
			m.filePath = frs[idx].Path
			m.fileLines, _ = gitFileLines(m.sess.Scope.TipSHA, m.filePath)
		}
	case sectionCommits, sectionIssues, sectionSources, sectionVerdicts:
		// no peek-side-effect; renderTreePeek handles display
	}
	m.refreshViewport()
}

// openSelectedItem performs the natural drill-in action.
func (m *model) openSelectedItem() {
	switch m.sect {
	case sectionSources, sectionVerdicts:
		// peek-only; drilling in just keeps the right pane content
	case sectionChanges:
		m.fileIdx = m.sectIdx[m.sect]
		m.hunkIdx = 0
		m.lineCursor = 0
		if h := m.currentHunk(); h != nil {
			m.lineCursor = m.firstNonDelete(h, 0, +1)
		}
		m.mode = modeDiff
		m.refreshViewport()
	case sectionFileReview:
		idx := m.sectIdx[m.sect]
		frs := m.sess.FileReviews()
		if idx < len(frs) {
			m.filePath = frs[idx].Path
			m.fileLines, _ = gitFileLines(m.sess.Scope.TipSHA, m.filePath)
			m.fileLineCursor = 0
			m.mode = modeFile
			m.refreshViewport()
		}
	case sectionCommits:
		// For v1: enter Diff mode on the changed-files list, leaving commit
		// filtering as a follow-up.
		m.mode = modeDiff
		m.refreshViewport()
	case sectionIssues:
		idx := m.sectIdx[m.sect]
		issues := m.sess.Issues()
		if idx < len(issues) {
			it := issues[idx]
			m.openEdit(editIssue, "", -1, idx, "")
			m.title.SetValue(it.Title)
			m.textarea.SetValue(it.Body)
			_ = m.title.Focus()
		}
	}
}

// ---------------------------------------------------------------------
// diff mode (hunks + line sub-mode)
// ---------------------------------------------------------------------

func (m *model) updateDiff(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if cmd, done := m.handleGlobal(key.String()); done {
		return m, cmd
	}
	switch key.String() {
	case "w":
		m.toggleWrap()
	case "left", "h":
		// Hard-wrap: left scrolls horizontally; only when at column 0 do
		// we go back to section mode. Soft-wrap: left always exits.
		if !m.viewport.SoftWrap && m.viewport.XOffset() > 0 {
			m.viewport.ScrollLeft(4)
			return m, nil
		}
		m.mode = modeTree
		m.refreshViewport()
	case "right", "l":
		// Hard-wrap: right scrolls horizontally; otherwise no-op (line mode
		// already in line-cursor sub-state per the simplified model).
		if !m.viewport.SoftWrap {
			m.viewport.ScrollRight(4)
			return m, nil
		}
	case "j", "down":
		m.lineNext()
	case "k", "up":
		m.linePrev()
	case "n", "tab":
		m.nextFile()
	case "p", "shift+tab":
		m.prevFile()
	case "home":
		m.lineCursor = 0
		m.refreshViewport()
	case "end":
		if h := m.currentHunk(); h != nil {
			m.lineCursor = len(h.Lines) - 1
			m.refreshViewport()
		}
	case "u":
		a := m.currentAnchor()
		m.sess.MarkUnread(a)
		delete(m.displayed, a)
		delete(m.pendingReads, a) // cancel pending delayed-read for this anchor
		m.save("marked unread")
	case " ", "space":
		m.spaceWalk()
	case "g", "+":
		m.applyMarker(review.MarkerGood)
	case "b", "-":
		m.applyMarker(review.MarkerBad)
	case "c", "!", "enter":
		m.openCommentEdit()
		return m, m.textarea.Focus()
	case "a", "?":
		m.openQuestionEdit()
		return m, m.textarea.Focus()
	case "e":
		m.editSelectedComment()
		if m.edit != editNone {
			return m, m.textarea.Focus()
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.updateDisplayed()
		return m, cmd
	}
	return m, nil
}


func (m *model) lineNext() {
	h := m.currentHunk()
	if h == nil {
		return
	}
	next := m.firstNonDelete(h, m.lineCursor+1, +1)
	if next != m.lineCursor {
		m.lineCursor = next
		m.refreshViewport()
		return
	}
	// At end of hunk: advance to first line of next hunk.
	f := m.currentFile()
	if m.hunkIdx+1 < len(f.Hunks) {
		m.hunkIdx++
		m.lineCursor = m.firstNonDelete(&f.Hunks[m.hunkIdx], 0, +1)
		m.refreshViewport()
	}
}

func (m *model) linePrev() {
	h := m.currentHunk()
	if h == nil {
		return
	}
	prev := m.firstNonDelete(h, m.lineCursor-1, -1)
	if prev != m.lineCursor {
		m.lineCursor = prev
		m.refreshViewport()
		return
	}
	// At start of hunk: jump to last line of previous hunk.
	if m.hunkIdx > 0 {
		m.hunkIdx--
		ph := &m.currentFile().Hunks[m.hunkIdx]
		m.lineCursor = m.firstNonDelete(ph, len(ph.Lines)-1, -1)
		m.refreshViewport()
	}
}

func (m *model) firstNonDelete(h *review.Hunk, start, step int) int {
	if step == 0 {
		return start
	}
	for i := start; i >= 0 && i < len(h.Lines); i += step {
		if h.Lines[i].Kind != review.LineDelete {
			return i
		}
	}
	if start < 0 {
		return 0
	}
	if start >= len(h.Lines) {
		return len(h.Lines) - 1
	}
	return start
}

func (m *model) currentFile() *review.File {
	if len(m.files) == 0 || m.fileIdx >= len(m.files) {
		return &review.File{}
	}
	return &m.files[m.fileIdx]
}

func (m *model) currentHunk() *review.Hunk {
	f := m.currentFile()
	if len(f.Hunks) == 0 || m.hunkIdx >= len(f.Hunks) {
		return nil
	}
	return &f.Hunks[m.hunkIdx]
}

func (m *model) currentAnchor() review.Anchor {
	switch m.mode {
	case modeFile:
		return review.Anchor(fmt.Sprintf("%s@%s:%d", m.filePath, m.sess.Scope.TipSHA[:12], m.fileLineCursor+1))
	case modeDiff:
		f := m.currentFile()
		h := m.currentHunk()
		if h == nil {
			return review.Anchor(f.Path)
		}
		if line := m.cursorNewLine(h); line > 0 {
			return review.Anchor(fmt.Sprintf("%s:%d", f.Path, line))
		}
		return review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
	}
	return review.Anchor("")
}

// cursorNewLine returns the new-side line number that lineCursor maps to in
// the given hunk, or 0 if cursor is on a delete-only line.
func (m *model) cursorNewLine(h *review.Hunk) int {
	newLine := h.NewStart
	for i, ln := range h.Lines {
		if i == m.lineCursor {
			if ln.Kind == review.LineDelete {
				return 0
			}
			return newLine
		}
		if ln.Kind != review.LineDelete {
			newLine++
		}
	}
	return 0
}

// spaceWalk implements the "page-walk with overlap → last line → next file"
// behaviour described in the spec. Each Space press:
//   - if viewport is not at the bottom of the current file's content,
//     scroll down by viewport_height - 5 lines (5-line overlap from the
//     previous view's bottom), put the cursor at the top of the new view
//   - else if the cursor is not on the last hunk, place it on the last hunk
//   - else advance to the next file in the Changes list (or to the next
//     FileReview in modeFile)
func (m *model) spaceWalk() {
	// Bottom of viewport content?
	if !m.viewport.AtBottom() {
		h := m.viewport.Height()
		step := h - 5
		if step < 1 {
			step = 1
		}
		m.viewport.SetYOffset(m.viewport.YOffset() + step)
		m.updateDisplayed()
		// Move cursor to whatever's at the top of the new view so it
		// stays selected when the user keeps pressing space.
		m.hunkAtTopOfView()
		return
	}
	// At bottom: ensure cursor is on the last line of the last hunk first.
	f := m.currentFile()
	last := len(f.Hunks) - 1
	if last >= 0 {
		lastH := &f.Hunks[last]
		lastLine := m.firstNonDelete(lastH, len(lastH.Lines)-1, -1)
		if m.hunkIdx < last || m.lineCursor < lastLine {
			m.hunkIdx = last
			m.lineCursor = lastLine
			m.refreshViewport()
			return
		}
	}
	// Already on the last line; advance to next file.
	if m.fileIdx+1 < len(m.files) {
		m.fileIdx++
		m.hunkIdx = 0
		m.lineCursor = 0
		if h := m.currentHunk(); h != nil {
			m.lineCursor = m.firstNonDelete(h, 0, +1)
		}
		m.refreshViewport()
		return
	}
	m.status = "end of changes"
}

// spaceWalkFile is the file-mode counterpart of spaceWalk.
func (m *model) spaceWalkFile() {
	if !m.viewport.AtBottom() {
		h := m.viewport.Height()
		step := h - 5
		if step < 1 {
			step = 1
		}
		newTop := m.viewport.YOffset() + step
		m.viewport.SetYOffset(newTop)
		// Cursor follows to roughly the top of the new view.
		if newTop < len(m.fileLines) {
			m.fileLineCursor = newTop
		}
		return
	}
	// At bottom of file: cursor → last line first.
	last := len(m.fileLines) - 1
	if last >= 0 && m.fileLineCursor < last {
		m.fileLineCursor = last
		m.refreshViewport()
		return
	}
	// On last line and at bottom: advance to next FileReview.
	frs := m.sess.FileReviews()
	for i, fr := range frs {
		if fr.Path == m.filePath && i+1 < len(frs) {
			next := frs[i+1]
			m.filePath = next.Path
			m.fileLines, _ = gitFileLines(m.sess.Scope.TipSHA, m.filePath)
			m.fileLineCursor = 0
			m.refreshViewport()
			return
		}
	}
	m.status = "end of file review"
}

// hunkAtTopOfView places the cursor on the first reviewable line of whichever
// hunk now occupies the top of the viewport, so a Space walk always leaves
// the cursor on the uppermost reviewable line of the new view.
func (m *model) hunkAtTopOfView() {
	top := m.viewport.YOffset()
	bestIdx := m.hunkIdx
	for i, r := range m.hunkRanges {
		if r.topRow <= top && r.botRow >= top {
			bestIdx = i
			break
		}
		if r.topRow > top {
			bestIdx = i
			break
		}
	}
	m.hunkIdx = bestIdx
	if h := m.currentHunk(); h != nil {
		m.lineCursor = m.firstNonDelete(h, 0, +1)
	}
}

// toggleWrap switches the diff/file viewport between soft-wrap (default;
// long lines wrap visually) and hard-wrap (lines extend off-screen; arrow
// keys scroll horizontally; left-arrow at column 0 exits to section mode).
func (m *model) toggleWrap() {
	m.viewport.SoftWrap = !m.viewport.SoftWrap
	if m.viewport.SoftWrap {
		m.viewport.SetHorizontalStep(0)
		m.viewport.SetXOffset(0)
		m.status = "wrap: soft"
	} else {
		m.viewport.SetHorizontalStep(4)
		m.status = "wrap: hard (←/→ scroll horizontally)"
	}
}

func (m *model) advanceHunk() {
	cur := m.currentHunk()
	if cur != nil {
		a := review.HunkAnchor(m.currentFile().Path, cur.NewStart, cur.NewLines)
		if !m.isHunkFullyDisplayed(a) {
			step := m.viewport.Height() / 2
			if step < 1 {
				step = 1
			}
			m.viewport.SetYOffset(m.viewport.YOffset() + step)
			m.updateDisplayed()
			return
		}
	}
	f := m.currentFile()
	if m.hunkIdx+1 < len(f.Hunks) {
		m.hunkIdx++
	} else if m.fileIdx+1 < len(m.files) {
		m.fileIdx++
		m.hunkIdx = 0
	}
	m.refreshViewport()
}

func (m *model) prevHunk() {
	if m.hunkIdx > 0 {
		m.hunkIdx--
	} else if m.fileIdx > 0 {
		m.fileIdx--
		m.hunkIdx = max(0, len(m.currentFile().Hunks)-1)
	}
	m.refreshViewport()
}

func (m *model) nextFile() {
	if m.fileIdx+1 < len(m.files) {
		m.fileIdx++
		m.hunkIdx = 0
		m.refreshViewport()
	}
}

func (m *model) prevFile() {
	if m.fileIdx > 0 {
		m.fileIdx--
		m.hunkIdx = 0
		m.refreshViewport()
	}
}

// ---------------------------------------------------------------------
// file review mode
// ---------------------------------------------------------------------

func (m *model) updateFile(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if cmd, done := m.handleGlobal(key.String()); done {
		return m, cmd
	}
	switch key.String() {
	case "w":
		m.toggleWrap()
	case "left", "h":
		if !m.viewport.SoftWrap && m.viewport.XOffset() > 0 {
			m.viewport.ScrollLeft(4)
			return m, nil
		}
		m.mode = modeTree
		m.refreshViewport()
	case "right", "l":
		if !m.viewport.SoftWrap {
			m.viewport.ScrollRight(4)
			return m, nil
		}
	case "j", "down":
		if m.fileLineCursor+1 < len(m.fileLines) {
			m.fileLineCursor++
			m.refreshViewport()
		}
	case "k", "up":
		if m.fileLineCursor > 0 {
			m.fileLineCursor--
			m.refreshViewport()
		}
	case "home":
		m.fileLineCursor = 0
		m.refreshViewport()
	case "end":
		m.fileLineCursor = max(0, len(m.fileLines)-1)
		m.refreshViewport()
	case " ", "space":
		m.spaceWalkFile()
	case "c", "!", "enter":
		m.openCommentEdit()
		return m, m.textarea.Focus()
	case "a", "?":
		m.openQuestionEdit()
		return m, m.textarea.Focus()
	case "e":
		m.editSelectedComment()
		if m.edit != editNone {
			return m, m.textarea.Focus()
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// ---------------------------------------------------------------------
// edit overlays (comment / question / summary / issue)
// ---------------------------------------------------------------------

// openEdit prepares the edit overlay with the given kind. cmtIdx/issIdx
// say which existing item (>=0) is being edited, -1 for new.
func (m *model) openEdit(kind editKind, label string, cmtIdx, issIdx int, _ string) {
	m.prevMode = m.mode
	m.edit = kind
	m.editCmtIdx = cmtIdx
	m.editIssIdx = issIdx
	m.editLabel = label
	m.editAnchor = m.currentAnchor()
	m.textarea.Reset()
	if kind != editIssue {
		m.title.SetValue("")
	}
}

func (m *model) openCommentEdit() {
	m.openEdit(editComment, "Comment on "+string(m.currentAnchor()), -1, -1, "")
}

func (m *model) openQuestionEdit() {
	m.openEdit(editQuestion, "Question on "+string(m.currentAnchor()), -1, -1, "")
}

// editSelectedComment finds the comment at the current anchor and loads it
// into the textarea for editing.
func (m *model) editSelectedComment() {
	a := m.currentAnchor()
	idx := m.sess.CommentIndexAt(a)
	if idx < 0 {
		m.status = "no comment at cursor"
		return
	}
	c := m.sess.Comments()[idx]
	kind := editComment
	if c.Kind == review.KindQuestion {
		kind = editQuestion
	}
	m.openEdit(kind, "Editing on "+string(a), idx, -1, "")
	m.textarea.SetValue(c.Text)
}

func (m *model) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.closeEdit()
			return m, nil
		case "enter":
			// Plain Enter submits for comment/question.
			if m.edit == editComment || m.edit == editQuestion {
				m.submitEdit()
				return m, nil
			}
			// For issue: if title field is focused, Enter moves to body.
			if m.edit == editIssue && m.title.Focused() {
				m.title.Blur()
				return m, m.textarea.Focus()
			}
		case "alt+enter", "ctrl+s":
			m.submitEdit()
			return m, nil
		case "tab":
			if m.edit == editIssue {
				if m.title.Focused() {
					m.title.Blur()
					return m, m.textarea.Focus()
				}
				m.textarea.Blur()
				return m, m.title.Focus()
			}
		}
	}
	var cmd tea.Cmd
	if m.edit == editIssue && m.title.Focused() {
		m.title, cmd = m.title.Update(msg)
	} else {
		m.textarea, cmd = m.textarea.Update(msg)
	}
	// Re-render so the inline view stays in sync with the textarea.
	m.refreshViewport()
	return m, cmd
}

func (m *model) closeEdit() {
	m.edit = editNone
	m.textarea.Blur()
	m.title.Blur()
	m.refreshViewport()
}

func (m *model) submitEdit() {
	text := strings.TrimRight(m.textarea.Value(), "\n")
	text = strings.TrimSpace(text)
	switch m.edit {
	case editComment, editQuestion:
		if text == "" {
			m.status = "empty — discarded"
			m.closeEdit()
			return
		}
		if m.editCmtIdx >= 0 {
			if m.sess.UpdateComment(m.editCmtIdx, text) {
				m.save("comment updated")
			} else {
				m.status = "no change"
			}
		} else {
			kind := review.KindComment
			word := "comment"
			if m.edit == editQuestion {
				kind = review.KindQuestion
				word = "question"
			}
			snippet := ""
			if h := m.currentHunk(); h != nil && m.mode == modeDiff {
				snippet = renderHunkSnippet(*h, 4)
			}
			m.sess.AddComment(review.Comment{
				Anchor:  m.editAnchor,
				Text:    text,
				Snippet: snippet,
				Kind:    kind,
			})
			m.save(word + " added")
		}
	case editSummary:
		m.sess.SetSummary(text)
		m.save("summary updated")
	case editIssue:
		title := strings.TrimSpace(m.title.Value())
		if title == "" {
			m.status = "issue title required"
			return
		}
		if m.editIssIdx >= 0 {
			if m.sess.UpdateIssue(m.editIssIdx, title, text) {
				m.save("issue updated")
			} else {
				m.status = "no change"
			}
		} else {
			m.sess.AddIssue(review.Issue{Title: title, Body: text})
			m.save("issue added")
		}
	}
	m.closeEdit()
}

// ---------------------------------------------------------------------
// confirm-quit
// ---------------------------------------------------------------------

func (m *model) updateConfirmQuit(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "y":
		if err := m.sess.Save(); err != nil {
			m.status = "save failed: " + err.Error()
			m.confirmQuit = false
			return m, nil
		}
		return m, tea.Quit
	case "n":
		return m, tea.Quit
	case "esc":
		m.confirmQuit = false
		return m, nil
	}
	return m, nil
}

// ---------------------------------------------------------------------
// marker helper
// ---------------------------------------------------------------------

func (m *model) applyMarker(mk review.Marker) {
	a := m.currentAnchor()
	m.sess.SetMarker(a, mk)
	m.save(markerStatus(m.sess.Marker(a)))
}

// ---------------------------------------------------------------------
// partial-read tracking
// ---------------------------------------------------------------------

func (m *model) isHunkFullyDisplayed(a review.Anchor) bool {
	for _, r := range m.hunkRanges {
		if r.anchor != a {
			continue
		}
		set := m.displayed[a]
		for row := r.topRow; row <= r.botRow; row++ {
			if !set[row] {
				return false
			}
		}
		return true
	}
	return false
}

func (m *model) updateDisplayed() {
	top := m.viewport.YOffset()
	bot := top + m.viewport.Height() - 1
	for _, r := range m.hunkRanges {
		set := m.displayed[r.anchor]
		if set == nil {
			set = map[int]bool{}
			m.displayed[r.anchor] = set
		}
		if m.sess.IsRead(r.anchor) {
			continue
		}
		vTop, vBot := r.topRow, r.botRow
		if top > vTop {
			vTop = top
		}
		if bot < vBot {
			vBot = bot
		}
		if vTop > vBot {
			continue
		}
		for row := vTop; row <= vBot; row++ {
			set[row] = true
		}
		full := true
		for row := r.topRow; row <= r.botRow; row++ {
			if !set[row] {
				full = false
				break
			}
		}
		if full && !m.pendingReads[r.anchor] {
			// Schedule a delayed read; the actual MarkRead happens when
			// the tick fires AND the hunk is still currently visible.
			m.pendingReads[r.anchor] = true
			anchor := r.anchor
			m.queuedCmds = append(m.queuedCmds, tea.Tick(m.readDelay, func(time.Time) tea.Msg {
				return delayedReadMsg{anchor: anchor}
			}))
		}
	}
}

// isHunkCurrentlyVisible reports whether the hunk identified by anchor is
// fully within the viewport at this very moment (regardless of history).
func (m *model) isHunkCurrentlyVisible(anchor review.Anchor) bool {
	top := m.viewport.YOffset()
	bot := top + m.viewport.Height() - 1
	for _, r := range m.hunkRanges {
		if r.anchor != anchor {
			continue
		}
		return r.topRow >= top && r.botRow <= bot
	}
	return false
}

func (m *model) refreshViewport() {
	var body string
	var ranges []hunkRange
	var cursorRow int
	switch m.mode {
	case modeDiff:
		body, ranges, cursorRow = renderFileDiff(m)
	case modeFile:
		body, cursorRow = renderFileView(m)
	case modeTree:
		// Peek: render whatever the selection suggests.
		switch m.sect {
		case sectionChanges:
			body, ranges, cursorRow = renderFileDiff(m)
		case sectionFileReview:
			body, cursorRow = renderFileView(m)
		case sectionCommits:
			body = renderCommitDetail(m)
		case sectionIssues:
			body = renderIssueDetail(m)
		}
	}
	m.hunkRanges = ranges
	m.viewport.SetContent(body)
	target := cursorRow - m.viewport.Height()/3
	if target < 0 {
		target = 0
	}
	m.viewport.SetYOffset(target)
	m.updateDisplayed()
}

// ---------------------------------------------------------------------
// view
// ---------------------------------------------------------------------

var (
	styleAdd      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleDel      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleCtx      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleHunk     = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	styleCursor   = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	styleLineCur  = lipgloss.NewStyle().Background(lipgloss.Color("236")).Bold(true)
	styleSel      = lipgloss.NewStyle().Background(lipgloss.Color("235"))
	styleRead     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleUnread   = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	styleMarkGood = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleMarkBad  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleStatus   = lipgloss.NewStyle().Reverse(true).Padding(0, 1)
	styleTitle    = lipgloss.NewStyle().Bold(true)
	styleFocused  = lipgloss.NewStyle().Bold(true).Underline(true)
	styleSectHdr  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)

func sidebarWidth(total int) int {
	w := total / 4
	if w < 24 {
		w = 24
	}
	if w > 40 {
		w = 40
	}
	return w
}

func (m *model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true

	if m.width == 0 {
		v.Content = "loading…"
		return v
	}
	if m.confirmQuit {
		v.Content = m.viewConfirmQuit()
		return v
	}

	sidebar := m.viewSidebar()
	main := m.viewMain()
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)

	left := fmt.Sprintf(" %s | verdict: %s ", m.modeName(), m.sess.Verdict)
	right := " " + m.status + " "
	if m.status == "" {
		right = " " + helpFor(m) + " "
	}
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	status := styleStatus.Render(left + strings.Repeat(" ", pad) + right)

	v.Content = lipgloss.JoinVertical(lipgloss.Left, body, status)
	return v
}

func (m *model) modeName() string {
	switch m.mode {
	case modeTree:
		return "Section[" + m.sect.Label() + "]"
	case modeDiff, modeFile:
		return "Line[" + m.sect.Label() + "]"
	}
	return "?"
}

func helpFor(m *model) string {
	if m.edit != editNone {
		if m.edit == editIssue {
			return "Tab switches title/body  •  Alt+Enter submits  •  Esc cancels"
		}
		if m.edit == editSummary {
			return "Alt+Enter/Ctrl+S submits  •  Enter newline  •  Esc cancels"
		}
		return "Enter/Alt+Enter submits  •  Shift+Enter newline  •  Esc cancels"
	}
	switch m.mode {
	case modeTree:
		return "j/k item  Tab section  →/l/Enter open  i new issue  e edit issue  q quit"
	case modeDiff:
		return "j/k line  Space walk  c/!/Enter comment  a/? question  g/b mark  u unread  e edit  w wrap  >/< verdict  ←/h tree  s save  q quit"
	case modeFile:
		return "j/k line  Space walk  c/!/Enter comment  a/? question  e edit  w wrap  ←/h tree  q quit"
	}
	return ""
}

func (m *model) viewSidebar() string {
	w := sidebarWidth(m.width)

	// Render every section + every item without any item cap. Track the
	// row index where the cursor lands so we can window-scroll when the
	// rendered content exceeds the available height.
	var lines []string
	cursorRow := 0
	for _, sec := range []section{sectionSources, sectionVerdicts, sectionIssues, sectionChanges, sectionCommits, sectionFileReview} {
		items := m.sectionItems(sec)
		hdr := fmt.Sprintf("%s (%d)", sec.Label(), len(items))
		if m.mode == modeTree && m.sect == sec {
			lines = append(lines, styleFocused.Render(hdr))
		} else {
			lines = append(lines, styleSectHdr.Render(hdr))
		}
		for i, item := range items {
			marker := "  "
			if m.mode == modeTree && m.sect == sec && i == m.sectIdx[sec] {
				marker = "▶ "
				cursorRow = len(lines)
			}
			line := marker + truncate(item, w-3)
			if m.mode == modeTree && m.sect == sec && i == m.sectIdx[sec] {
				line = styleCursor.Render(line)
			}
			if sec == sectionChanges {
				r, t := m.fileReadStats(i)
				if t > 0 {
					stat := fmt.Sprintf(" %d/%d", r, t)
					if r == t {
						line += styleRead.Render(stat)
					} else {
						line += styleUnread.Render(stat)
					}
				}
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	// Window the rendered sidebar to fit the available height.
	available := max(3, m.height-4)
	if len(lines) > available {
		// Keep the cursor at roughly the third-of-height position.
		top := cursorRow - available/3
		if top < 0 {
			top = 0
		}
		if top+available > len(lines) {
			top = len(lines) - available
		}
		lines = lines[top : top+available]
	}

	return lipgloss.NewStyle().Width(w).Render(strings.Join(lines, "\n"))
}

func (m *model) fileReadStats(idx int) (read, total int) {
	if idx >= len(m.files) {
		return 0, 0
	}
	f := m.files[idx]
	total = len(f.Hunks)
	for _, h := range f.Hunks {
		if m.sess.IsRead(review.HunkAnchor(f.Path, h.NewStart, h.NewLines)) {
			read++
		}
	}
	return
}

func (m *model) viewMain() string {
	heading := m.mainHeading()
	hdr := styleTitle.Render(heading)
	if m.mode != modeTree {
		hdr = styleFocused.Render(heading)
	}
	return lipgloss.JoinVertical(lipgloss.Left, hdr, m.viewport.View())
}

func (m *model) mainHeading() string {
	switch m.mode {
	case modeTree:
		return m.sect.Label() + " (peek)"
	case modeDiff:
		f := m.currentFile()
		h := ""
		if hc := len(f.Hunks); hc > 0 {
			h = fmt.Sprintf("   hunk %d/%d", m.hunkIdx+1, hc)
		}
		return f.Path + h
	case modeFile:
		return m.filePath + "  @" + truncate(m.sess.Scope.TipSHA, 12)
	}
	return ""
}

func (m *model) viewConfirmQuit() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("Unsaved changes"),
		"",
		"Save before quitting? (y/n, Esc to cancel)",
	)
}

// ---------------------------------------------------------------------
// rendering: diff view (modeDiff and Diffs-section peek)
// ---------------------------------------------------------------------

func renderFileDiff(m *model) (body string, ranges []hunkRange, cursorRow int) {
	var sb strings.Builder
	f := m.currentFile()
	editing := m.edit == editComment || m.edit == editQuestion
	row := 0
	for hi, h := range f.Hunks {
		topRow := row
		anchor := review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
		readMark := styleUnread.Render("● ")
		if m.sess.IsRead(anchor) {
			readMark = styleRead.Render("✓ ")
		}
		var mk string
		switch m.sess.Marker(anchor) {
		case review.MarkerGood:
			mk = styleMarkGood.Render("+ ")
		case review.MarkerBad:
			mk = styleMarkBad.Render("- ")
		default:
			mk = "  "
		}
		header := readMark + mk + h.Header
		if hi == m.hunkIdx && (m.mode == modeDiff) {
			header = styleCursor.Render(header)
		} else {
			header = styleHunk.Render(header)
		}
		sb.WriteString(header + "\n")
		if hi == m.hunkIdx {
			cursorRow = row
		}
		row++

		// Editor splices below the line the cursor is on.
		editorLineIdx := -1
		if editing && hi == m.hunkIdx {
			editorLineIdx = m.lineCursor
		}

		newLine := h.NewStart
		for li, ln := range h.Lines {
			var styled string
			switch ln.Kind {
			case review.LineAdd:
				styled = styleAdd.Render("+ " + ln.Text)
			case review.LineDelete:
				styled = styleDel.Render("- " + ln.Text)
			default:
				styled = styleCtx.Render("  " + ln.Text)
			}
			if hi == m.hunkIdx && li == m.lineCursor {
				styled = styleLineCur.Render(styled)
				cursorRow = row
			}
			sb.WriteString(styled + "\n")
			row++

			if li == editorLineIdx {
				rs := renderInlineEditor(m)
				sb.WriteString(rs)
				row += strings.Count(rs, "\n")
			}

			// Inline events anchored to this new-side line (Comment, Question,
			// Like, Dislike). Only `+` and ` ` lines advance newLine.
			if ln.Kind != review.LineDelete {
				rs := renderInlineEventsForLine(m, f.Path, newLine)
				sb.WriteString(rs)
				row += strings.Count(rs, "\n")
				newLine++
			}
		}

		// Hunk-anchored events (anchored to "path:newStart,newLines" exactly).
		// These cover the whole hunk and render at its end.
		rs := renderInlineEventsForHunk(m, f.Path, h.NewStart, h.NewLines)
		sb.WriteString(rs)
		row += strings.Count(rs, "\n")

		sb.WriteString("\n")
		row++

		ranges = append(ranges, hunkRange{anchor: anchor, topRow: topRow, botRow: row - 1})
	}
	if len(f.Hunks) == 0 {
		sb.WriteString(styleDim.Render("(no hunks)") + "\n")
	}
	return sb.String(), ranges, cursorRow
}

func anchorBelongsToHunk(a review.Anchor, path string, h *review.Hunk) bool {
	s := string(a)
	hunkAnchor := string(review.HunkAnchor(path, h.NewStart, h.NewLines))
	if s == hunkAnchor {
		return true
	}
	prefix := path + ":"
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	// Parse "<start>" or "<start>-<end>".
	rest := s[len(prefix):]
	startStr := rest
	endStr := rest
	if i := strings.Index(rest, "-"); i > 0 {
		startStr = rest[:i]
		endStr = rest[i+1:]
	}
	start, ok1 := atoi(startStr)
	end, ok2 := atoi(endStr)
	if !ok1 || !ok2 {
		return false
	}
	return start >= h.NewStart && end < h.NewStart+h.NewLines
}

// ---------------------------------------------------------------------
// rendering: file view (modeFile and Tree-section peek)
// ---------------------------------------------------------------------

func renderFileView(m *model) (body string, cursorRow int) {
	var sb strings.Builder
	editing := m.edit == editComment || m.edit == editQuestion
	digits := len(fmt.Sprintf("%d", len(m.fileLines)))
	for i, ln := range m.fileLines {
		styled := fmt.Sprintf("%*d  %s", digits, i+1, ln)
		if m.mode == modeFile {
			if i == m.fileLineCursor {
				styled = styleLineCur.Render(styled)
				cursorRow = sb.Len()
				_ = cursorRow
			}
		}
		sb.WriteString(styled + "\n")

		if editing && m.mode == modeFile && i == m.fileLineCursor {
			sb.WriteString(renderInlineEditor(m))
		}
		// Inline comments for this line.
		anchorPrefix := m.filePath + "@" + truncate(m.sess.Scope.TipSHA, 12) + ":"
		want := fmt.Sprintf("%s%d", anchorPrefix, i+1)
		for _, c := range m.sess.Comments() {
			s := string(c.Anchor)
			if s == want || strings.HasPrefix(s, want+"-") {
				sb.WriteString(renderInlineComment(c))
			}
		}
	}
	// Compute approximate cursorRow as a fraction of file length.
	if len(m.fileLines) > 0 {
		cursorRow = m.fileLineCursor
	}
	return sb.String(), cursorRow
}

// ---------------------------------------------------------------------
// rendering: commit + issue detail (tree peek)
// ---------------------------------------------------------------------

func renderCommitDetail(m *model) string {
	idx := m.sectIdx[sectionCommits]
	if idx >= len(m.sess.Scope.Commits) {
		return styleDim.Render("(no commits)")
	}
	c := m.sess.Scope.Commits[idx]
	out, _ := exec.Command("git", "show", "--no-color", c.SHA).Output()
	return styleTitle.Render(c.Short+"  "+c.Subject) + "\n\n" + string(out)
}

func renderIssueDetail(m *model) string {
	idx := m.sectIdx[sectionIssues]
	issues := m.sess.Issues()
	if idx >= len(issues) {
		return styleDim.Render("(no issues — press `i` to add one)")
	}
	it := issues[idx]
	var sb strings.Builder
	sb.WriteString(styleTitle.Render(it.Title) + "\n")
	if it.Author != "" {
		sb.WriteString(styleDim.Render(it.Author+"  "+it.Date) + "\n")
	}
	sb.WriteString("\n" + it.Body + "\n")
	return sb.String()
}

// ---------------------------------------------------------------------
// rendering: inline editor + comments
// ---------------------------------------------------------------------

func renderInlineEditor(m *model) string {
	var label, icon string
	switch m.edit {
	case editComment:
		icon = "💬"
		label = "Comment on " + string(m.editAnchor)
	case editQuestion:
		icon = "❓"
		label = "Question on " + string(m.editAnchor)
	case editSummary:
		icon = "📝"
		label = "Verdict summary"
	case editIssue:
		icon = "🗂"
		label = "Issue"
	}
	var sb strings.Builder
	sb.WriteString("    " + styleTitle.Render(icon+" "+label) + "\n")
	if m.edit == editIssue {
		sb.WriteString("    title: " + m.title.View() + "\n")
	}
	for _, l := range strings.Split(m.textarea.View(), "\n") {
		sb.WriteString("    " + l + "\n")
	}
	return sb.String()
}

func renderInlineComment(c review.Comment) string {
	icon := "💬"
	if c.Kind == review.KindQuestion {
		icon = "❓"
	}
	lines := strings.Split(strings.TrimRight(c.Text, "\n"), "\n")
	var sb strings.Builder
	for i, ln := range lines {
		var prefix string
		if i == 0 {
			prefix = "    " + icon + " " + c.Author + ": "
		} else {
			prefix = "      "
		}
		sb.WriteString(styleDim.Render(prefix+ln) + "\n")
	}
	return sb.String()
}

// renderInlineEventsForLine renders all events anchored to a specific
// new-side line of `path`. Comment/Question/Like/Dislike all show inline.
func renderInlineEventsForLine(m *model, path string, newLine int) string {
	var sb strings.Builder
	// Comments and questions.
	for _, c := range m.sess.Comments() {
		if eventAnchoredToLine(c.Anchor, path, newLine) {
			sb.WriteString(renderInlineComment(c))
		}
	}
	// Like / Dislike via markers map.
	for _, a := range m.sess.MarkerAnchors() {
		if !eventAnchoredToLine(a, path, newLine) {
			continue
		}
		switch m.sess.Marker(a) {
		case review.MarkerGood:
			sb.WriteString(styleMarkGood.Render("    👍 "+m.sess.Reviewer) + "\n")
		case review.MarkerBad:
			sb.WriteString(styleMarkBad.Render("    👎 "+m.sess.Reviewer) + "\n")
		}
	}
	return sb.String()
}

// renderInlineEventsForHunk renders events anchored to the whole hunk
// (`path:newStart,newLines` exact match).
func renderInlineEventsForHunk(m *model, path string, newStart, newLines int) string {
	hunkAnchor := review.HunkAnchor(path, newStart, newLines)
	var sb strings.Builder
	for _, c := range m.sess.Comments() {
		if c.Anchor == hunkAnchor {
			sb.WriteString(renderInlineComment(c))
		}
	}
	return sb.String()
}

// eventAnchoredToLine returns true if `a` is "path:N" or "path:start-N"
// (a range ending at N).
func eventAnchoredToLine(a review.Anchor, path string, newLine int) bool {
	s := string(a)
	prefix := path + ":"
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	rest := s[len(prefix):]
	// Reject hunk anchors "<start>,<count>".
	if strings.Contains(rest, ",") {
		return false
	}
	endStr := rest
	if i := strings.Index(rest, "-"); i > 0 {
		endStr = rest[i+1:]
	}
	n, err := strconv.Atoi(endStr)
	if err != nil {
		return false
	}
	return n == newLine
}

func renderHunkSnippet(h review.Hunk, ctx int) string {
	var sb strings.Builder
	sb.WriteString(h.Header + "\n")
	limit := len(h.Lines)
	if ctx > 0 && limit > ctx*4 {
		limit = ctx * 4
	}
	for i := 0; i < limit; i++ {
		ln := h.Lines[i]
		switch ln.Kind {
		case review.LineAdd:
			sb.WriteString("+" + ln.Text + "\n")
		case review.LineDelete:
			sb.WriteString("-" + ln.Text + "\n")
		default:
			sb.WriteString(" " + ln.Text + "\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func markerStatus(m review.Marker) string {
	switch m {
	case review.MarkerGood:
		return "marker → good"
	case review.MarkerBad:
		return "marker → bad"
	default:
		return "marker cleared"
	}
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

func truncate(s string, w int) string {
	if w < 1 {
		return ""
	}
	if len(s) <= w {
		return s
	}
	if w < 4 {
		return s[:w]
	}
	return "…" + s[len(s)-w+1:]
}

func atoi(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func gitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func gitTreeFiles(sha string) ([]string, error) {
	out, err := exec.Command("git", "ls-tree", "-r", "--name-only", sha).Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	var files []string
	for _, l := range lines {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

func gitFileLines(sha, path string) ([]string, error) {
	out, err := exec.Command("git", "show", sha+":"+path).Output()
	if err != nil {
		return nil, err
	}
	s := strings.TrimRight(string(out), "\n")
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

