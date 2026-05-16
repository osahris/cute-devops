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
	"github.com/charmbracelet/x/ansi"

	"gitflower/review"
)

// DefaultReadDelay is how long a hunk must remain visible before
// the TUI emits a ReadStart/ReadEnd. Override at TUI construction.
const DefaultReadDelay = 1 * time.Second

// AutoSaveInterval is the debounce window for write-on-dirty. Multiple
// rapid mutations (e.g. a Space walk through a long diff) coalesce into
// one save per AutoSaveInterval window instead of one save per mutation.
const AutoSaveInterval = 2 * time.Second

// Run launches the TUI on sess. readDelay controls how long a hunk must
// stay visible before the read marker is emitted. The minimum is 1ms —
// even "immediate" still goes through the tick path so tests exercise
// the realistic Update/Cmd round-trip.
func Run(sess *review.ReviewSession, readDelay time.Duration) error {
	root, _ := gitRoot()
	if readDelay <= 0 {
		readDelay = DefaultReadDelay
	}
	if readDelay < time.Millisecond {
		readDelay = time.Millisecond
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
// sub-sections of `# Review` (Sources, Verdicts), plus a Tree view of
// every file at the tip SHA.
const (
	sectionSources section = iota
	sectionVerdicts
	sectionIssues
	sectionChanges
	sectionCommits
	sectionTree
	sectionFileReview
)

const numSections = 7

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
	case sectionTree:
		return "Tree"
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

// lineRange records the rendered-row span of a single reviewable element
// in the current viewport: either a non-delete diff line, or the synthetic
// "<end of file>" marker that follows the last hunk. We need this for
// wrap-aware cursor placement — with hanging-indent soft-wrap, a single
// logical hunk line can span multiple rendered rows, so we can't infer
// the line index from a row index by simple arithmetic.
type lineRange struct {
	hunkIdx        int  // -1 for the EOF marker
	lineIdx        int  // index into hunk.Lines; ignored when hunkIdx == -1
	topRow, botRow int
	isEOF          bool
	kind           review.LineKind // only meaningful when !isEOF
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
	lineRanges []lineRange
	atEOF      bool // cursor is on the synthetic <end of file> marker
	displayed  map[review.Anchor]map[int]bool

	// Delayed read marking.
	readDelay    time.Duration
	pendingReads map[review.Anchor]bool // anchor has a scheduled read tick

	// Autosave: when true an autoSaveMsg tick is in flight.
	saveScheduled bool

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

// autoSaveMsg fires AutoSaveInterval after dirty state was detected.
// Coalesces many rapid mutations into a single Save call.
type autoSaveMsg struct{}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.refreshViewport()
		return m, m.drainCmds()
	case delayedReadMsg:
		// The hunk's rows were all visible at some point and the delay
		// elapsed; mark read. Schedule a debounced autosave instead of
		// writing to disk on every single tick.
		if m.pendingReads[msg.anchor] {
			m.sess.MarkRead(msg.anchor)
			m.scheduleAutoSave()
		}
		delete(m.pendingReads, msg.anchor)
		return m, m.drainCmds()
	case autoSaveMsg:
		m.saveScheduled = false
		if m.sess.Dirty() {
			_ = m.sess.Save()
		}
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

// sidebarVisible decides whether to show the section sidebar. In line
// modes we drop it so the diff/file gets the whole screen — the user can
// always Esc/left-arrow back to section mode to see it again.
func (m *model) sidebarVisible() bool {
	return m.mode == modeTree
}

func (m *model) resize() {
	mainW := m.width
	if m.sidebarVisible() {
		mainW -= sidebarWidth(m.width) + 2
	}
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
	case "ctrl+c", "ctrl+q", "q":
		// Quit cleanly. Flush any pending dirty state (autosave window
		// might not have expired yet) so the user never loses read
		// markers / comments / verdict edits. If save fails, ask the
		// user instead of dropping their work.
		if m.sess.Dirty() {
			if err := m.sess.Save(); err != nil {
				m.status = "save failed: " + err.Error()
				m.confirmQuit = true
				return nil, true
			}
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
		m.openVerdictEditor()
		return nil, true
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
	case sectionTree:
		return m.treeFiles
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
		m.sect = section((int(m.sect) + 1) % numSections)
	case "shift+tab":
		m.sect = section((int(m.sect) + numSections - 1) % numSections)
	case " ", "space":
		// On Commits, Space walks commit-by-commit and eventually hands
		// off to the verdict editor (see spaceWalk). Elsewhere, Space
		// drills in.
		if m.sect == sectionCommits || m.sect == sectionVerdicts {
			m.spaceWalk()
		} else {
			m.openSelectedItem()
		}
	case "i":
		// Add an entry in the current section:
		//   Verdicts → open the verdict editor (creates a new audit-log
		//              entry on Alt+Enter).
		//   Issues   → open the issue editor (creates a new general issue).
		//   anywhere else → fall back to Issues.
		if m.sect == sectionVerdicts {
			m.openVerdictEditor()
			return m, nil
		}
		m.openEdit(editIssue, "", -1, -1, "")
		return m, m.title.Focus()
	case "e":
		// Edit the selected item in the current section.
		switch m.sect {
		case sectionIssues:
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
		case sectionVerdicts:
			// Edit means: open the verdict editor pre-populated. Submitting
			// creates a fresh audit-log entry (it doesn't mutate the
			// existing one — the spec defines verdicts as append-only).
			m.openVerdictEditor()
			return m, nil
		default:
			m.status = "no editable item under cursor"
			return m, nil
		}
	case "right", "l", "enter":
		m.openSelectedItem()
	case "pgdown", "f":
		if m.sect == sectionChanges {
			m.pageDown()
			return m, nil
		}
		return m, m.scrollViewport(msg)
	case "pgup", "b":
		if m.sect == sectionChanges {
			m.pageUp()
			return m, nil
		}
		return m, m.scrollViewport(msg)
	default:
		return m, m.scrollViewport(msg)
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
		m.atEOF = false
	case sectionFileReview:
		idx := m.sectIdx[m.sect]
		frs := m.sess.FileReviews()
		if idx < len(frs) {
			m.filePath = frs[idx].Path
			m.fileLines, _ = gitFileLines(m.sess.Scope.TipSHA, m.filePath)
		}
	case sectionTree:
		idx := m.sectIdx[m.sect]
		if idx < len(m.treeFiles) {
			m.filePath = m.treeFiles[idx]
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
		m.atEOF = false
		if h := m.currentHunk(); h != nil {
			m.lineCursor = m.firstNonDelete(h, 0, +1)
		}
		m.mode = modeDiff
		m.refreshViewport()
	case sectionFileReview:
		idx := m.sectIdx[m.sect]
		frs := m.sess.FileReviews()
		if idx < len(frs) {
			m.enterFileReview(frs[idx].Path)
		}
	case sectionTree:
		// Drilling into Tree opens File mode on the chosen path. The
		// FileReview entry gets created on the fly so the visit lands
		// in the # File Review section of the saved .review.
		idx := m.sectIdx[m.sect]
		if idx < len(m.treeFiles) {
			m.enterFileReview(m.treeFiles[idx])
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
		// Park the section cursor on the file we were just reviewing so
		// the user lands back on it in section mode.
		m.sect = sectionChanges
		m.sectIdx[sectionChanges] = m.fileIdx
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
	case "F":
		// Enter file-review mode on the current Changes file. The session
		// gains a FileReview entry; cursor moves below will populate its
		// Lines with the content the reviewer actually visits.
		m.enterFileReview(m.currentFile().Path)
	case "pgdown":
		m.pageDown()
	case "pgup":
		m.pageUp()
	default:
		return m, m.scrollViewport(msg)
	}
	return m, nil
}


func (m *model) lineNext() {
	if m.atEOF {
		return // already at the end of the file
	}
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
	// At end of hunk: advance to first line of next hunk, or step onto
	// the EOF marker after the last hunk.
	f := m.currentFile()
	if m.hunkIdx+1 < len(f.Hunks) {
		m.hunkIdx++
		m.lineCursor = m.firstNonDelete(&f.Hunks[m.hunkIdx], 0, +1)
		m.refreshViewport()
		return
	}
	m.atEOF = true
	m.refreshViewport()
}

func (m *model) linePrev() {
	if m.atEOF {
		// Step back from EOF onto the last reviewable line of the file.
		f := m.currentFile()
		if last := len(f.Hunks) - 1; last >= 0 {
			m.hunkIdx = last
			lh := &f.Hunks[last]
			m.lineCursor = m.firstNonDelete(lh, len(lh.Lines)-1, -1)
		}
		m.atEOF = false
		m.refreshViewport()
		return
	}
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

// debugSpaceWalk, when not nil, is called once per spaceWalk entry.
// Used by tests to introspect state transitions.
var debugSpaceWalk func(stage string, m *model)

// spaceWalk does the same thing everywhere: take the user to the next
// unread hunk. Concretely:
//
//   1. From section mode: enter line mode on the file under the section
//      cursor (or fileIdx if no section cursor). Treat as "before hunk 0"
//      and seek the first unread hunk in that file.
//   2. From line mode, if the current hunk is unread: no-op. The current
//      line IS the next unread.
//   3. Else, look for an unread hunk strictly after the cursor IN THE
//      CURRENT FILE. If found, jump there.
//   4. Else (no unread left in current file), if the cursor is not yet on
//      the last reviewable line of the file: move there. No file change.
//   5. Else, advance to the next file. In that file, seek the first
//      unread; if none, land on the last reviewable line.
//   6. If no further file: status "all read".
//
// The viewport is positioned so 5 rows of context precede the new cursor
// (clamped to top-of-content).
func (m *model) spaceWalk() {
	if debugSpaceWalk != nil {
		debugSpaceWalk("entry", m)
		defer debugSpaceWalk("exit", m)
	}
	// Walking commits in section mode: each Space advances the commit
	// cursor; at the last commit the verdict editor opens.
	if m.mode == modeTree && m.sect == sectionCommits {
		idx := m.sectIdx[sectionCommits]
		if idx+1 < len(m.sess.Scope.Commits) {
			m.sectIdx[sectionCommits] = idx + 1
			m.onTreeSelectionChanged()
			return
		}
		m.sect = sectionVerdicts
		m.sectIdx[sectionVerdicts] = max(0, len(m.sess.Verdicts())-1)
		m.refreshViewport()
		m.openVerdictEditor()
		m.status = "all read — record your verdict"
		return
	}
	if m.mode == modeTree {
		if m.sect == sectionChanges {
			m.fileIdx = m.sectIdx[m.sect]
		}
		m.hunkIdx = -1 // virtual "before first hunk"
		m.atEOF = false
		m.mode = modeDiff
	}

	// (2) If we're on a non-EOF unread hunk, either page within it or
	// stay put waiting for the read tick. If it's read, fall through to
	// seek the next unread one in this file.
	if !m.atEOF && m.hunkIdx >= 0 {
		h := m.currentHunk()
		if h != nil {
			a := review.HunkAnchor(m.currentFile().Path, h.NewStart, h.NewLines)
			if !m.sess.IsRead(a) {
				for _, r := range m.hunkRanges {
					if r.anchor != a {
						continue
					}
					top := m.viewport.YOffset()
					bot := top + m.viewport.Height() - 1
					if r.botRow > bot {
						// Defer to the shared pageDown logic so Space
						// walks use exactly the same "marker N lines
						// before old bottom, last-page marker on top"
						// behaviour as PgDn.
						m.pageDown()
						return
					}
					break
				}
				// Fully visible but still unread → stay; the read tick
				// will fire and the next Space will advance.
				return
			}
		}
	}

	// (3) Next unread strictly after the cursor in current file.
	f := m.currentFile()
	if !m.atEOF {
		for hi := m.hunkIdx + 1; hi < len(f.Hunks); hi++ {
			h := &f.Hunks[hi]
			a := review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
			if m.sess.IsRead(a) {
				continue
			}
			m.hunkIdx = hi
			m.lineCursor = m.firstNonDelete(h, 0, +1)
			m.refreshViewportWithContext(5)
			return
		}
	}

	// (4) No unread left in this file: land on the synthetic <end of
	// file> marker so the reader gets an unambiguous "you're done here"
	// signal. If we're already on the marker, fall through to advance.
	if !m.atEOF {
		m.atEOF = true
		m.refreshViewport()
		// Scroll so the EOF marker sits comfortably above the bottom.
		if eof := m.eofRange(); eof != nil {
			target := eof.topRow - (m.viewport.Height() - 5)
			if target < 0 {
				target = 0
			}
			m.viewport.SetYOffset(target)
			m.updateDisplayed()
		}
		return
	}

	// (5) On the EOF marker; advance to the next file.
	if m.fileIdx+1 < len(m.files) {
		m.fileIdx++
		m.hunkIdx = -1
		m.atEOF = false
		nf := m.currentFile()
		for hi := 0; hi < len(nf.Hunks); hi++ {
			h := &nf.Hunks[hi]
			a := review.HunkAnchor(nf.Path, h.NewStart, h.NewLines)
			if !m.sess.IsRead(a) {
				m.hunkIdx = hi
				m.lineCursor = m.firstNonDelete(h, 0, +1)
				m.refreshViewportWithContext(5)
				return
			}
		}
		// New file is fully read already; jump straight to its EOF.
		m.atEOF = true
		m.refreshViewport()
		return
	}

	// (6) End of changes — start walking commits next. Each follow-up
	// Space (handled at the top of spaceWalk) advances the commit
	// cursor; the last commit hands off to the verdict editor.
	if len(m.sess.Scope.Commits) > 0 {
		m.mode = modeTree
		m.sect = sectionCommits
		m.sectIdx[sectionCommits] = 0
		m.onTreeSelectionChanged()
		m.status = "all changes read — walking commits"
		return
	}
	// No commits: jump straight to verdict.
	m.sect = sectionVerdicts
	m.sectIdx[sectionVerdicts] = max(0, len(m.sess.Verdicts())-1)
	m.mode = modeTree
	m.refreshViewport()
	m.openVerdictEditor()
	m.status = "all read — record your verdict"
}

// scheduleAutoSave queues an autoSaveMsg tick so dirty state gets flushed
// to disk within AutoSaveInterval. Idempotent — a second call while a
// tick is already pending is a no-op.
func (m *model) scheduleAutoSave() {
	if m.saveScheduled {
		return
	}
	m.saveScheduled = true
	m.queuedCmds = append(m.queuedCmds, tea.Tick(AutoSaveInterval, func(time.Time) tea.Msg {
		return autoSaveMsg{}
	}))
}

// openVerdictEditor opens the inline summary editor pre-populated with the
// current canonical summary. Submitting calls AddVerdict so the audit log
// gets a fresh entry.
func (m *model) openVerdictEditor() {
	m.openEdit(editSummary, "", -1, -1, "")
	m.textarea.SetValue(m.sess.Summary)
	_ = m.textarea.Focus()
}

// refreshViewportWithContext renders the current file and scrolls the
// viewport so the cursor's hunk sits with `ctx` lines of context above it.
// Falls back to top-of-content when fewer rows are available.
func (m *model) refreshViewportWithContext(ctx int) {
	m.refreshViewport()
	if m.hunkIdx < 0 || m.hunkIdx >= len(m.hunkRanges) {
		return
	}
	target := m.hunkRanges[m.hunkIdx].topRow - ctx
	if target < 0 {
		target = 0
	}
	m.viewport.SetYOffset(target)
	m.updateDisplayed()
}

// snapCursorIntoView re-binds the line cursor to the topmost hunk that is
// (at least partially) visible in the current viewport, and sets the
// line cursor to that hunk's first reviewable line. Called after every
// viewport scroll in line mode, so PgUp / PgDn / mouse-wheel each move the
// active line along with the view.
// enterFileReview switches to modeFile on the given path. Loads the file
// content at the scope's tip SHA, parks the cursor at the top, and records
// the first visited line so the # File Review section gets a sub-section
// even if the user just opens-and-closes.
func (m *model) enterFileReview(path string) {
	if path == "" {
		return
	}
	lines, err := gitFileLines(m.sess.Scope.TipSHA, path)
	if err != nil || len(lines) == 0 {
		m.status = "no content at tip for " + path
		return
	}
	m.filePath = path
	m.fileLines = lines
	m.fileLineCursor = 0
	m.mode = modeFile
	m.recordVisitedLine()
	m.refreshViewport()
}

// recordVisitedLine records the cursor's current file-mode line into the
// session's FileReview entry for the active path.
func (m *model) recordVisitedLine() {
	if m.filePath == "" || m.fileLineCursor < 0 || m.fileLineCursor >= len(m.fileLines) {
		return
	}
	m.sess.RecordFileLine(
		m.filePath,
		m.sess.Scope.TipSHA,
		m.fileLineCursor+1, // 1-based line numbers
		m.fileLines[m.fileLineCursor],
	)
	_ = m.sess.Save()
}

// snapCursorIntoView picks a reviewable line for the cursor based on
// the current viewport position. If picked successfully, it also nudges
// the viewport so the chosen line sits at row 0 — otherwise hunk
// headers, blank inter-hunk rows, or inline comment rows would occupy
// the top of the view and the cursor would visually appear on row 1, 2,
// or 3 instead of row 0.
func (m *model) snapCursorIntoView() {
	if len(m.lineRanges) == 0 {
		return
	}
	top := m.viewport.YOffset()
	// Prefer the first reviewable line whose topRow is at or below the
	// viewport top — that way the cursor's first row IS the top row.
	// If asking the viewport to scroll there gets clamped (we're already
	// near content end), fall through and re-snap to EOF.
	for _, lr := range m.lineRanges {
		if lr.topRow < top {
			continue
		}
		if !lr.isEOF && lr.kind == review.LineDelete {
			continue
		}
		// Try to realign so the picked line sits at row 0. If the
		// viewport clamps the request (we're already past the last
		// clean boundary near content end), leave the offset alone —
		// the user can still see the line, and the "no progress on
		// PgDn" path in scrollViewport will step them to EOF when
		// they ask for one more page.
		m.viewport.SetYOffset(lr.topRow)
		m.placeCursor(lr)
		return
	}
	// Nothing starts at-or-below top; pick the line whose wrap-span
	// covers top, so the cursor highlight is at least partially visible.
	for _, lr := range m.lineRanges {
		if lr.botRow < top {
			continue
		}
		if !lr.isEOF && lr.kind == review.LineDelete {
			continue
		}
		m.placeCursor(lr)
		return
	}
	// Scrolled past everything; clamp to EOF (last range).
	m.placeCursor(m.lineRanges[len(m.lineRanges)-1])
}

// eofRange returns the lineRange of the synthetic EOF marker for the
// currently-rendered file, or nil if there isn't one in the current
// content (e.g. tree-mode peek of a non-file section).
func (m *model) eofRange() *lineRange {
	for i := range m.lineRanges {
		if m.lineRanges[i].isEOF {
			return &m.lineRanges[i]
		}
	}
	return nil
}

// placeCursor moves the diff-mode cursor to the line described by lr,
// switching to / out of the EOF state as needed. Callers are responsible
// for re-rendering and any viewport scroll they want.
func (m *model) placeCursor(lr lineRange) {
	if lr.isEOF {
		m.atEOF = true
		return
	}
	m.atEOF = false
	m.hunkIdx = lr.hunkIdx
	m.lineCursor = lr.lineIdx
}

// pageOverlap is how many reviewable lines from the bottom of the
// current view get carried over to the top of the next page. The cursor
// lands on the (overlap+1)th-from-bottom line so the reader sees that
// line stay put as the marker between pages, then the rest comes into
// view below it.
const pageOverlap = 5

// scrollViewport is the one true viewport-scroll path. For mouse-wheel
// / arrow-scroll messages we forward to the viewport directly; for
// page-sized navigation (PgUp/PgDn/Space-style paging) callers use
// pageDown / pageUp explicitly because those have richer semantics
// (marker placement, last-page exception, EOF-on-no-progress).
func (m *model) scrollViewport(msg tea.Msg) tea.Cmd {
	preY := m.viewport.YOffset()
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	m.updateDisplayed()
	if m.mode == modeDiff || (m.mode == modeTree && m.sect == sectionChanges) {
		oldHunk, oldLine, oldEOF := m.hunkIdx, m.lineCursor, m.atEOF
		m.snapCursorIntoView()
		if !m.atEOF &&
			oldHunk == m.hunkIdx && oldLine == m.lineCursor &&
			m.viewport.YOffset() == preY && m.viewport.AtBottom() {
			if eof := m.eofRange(); eof != nil {
				m.placeCursor(*eof)
			}
		}
		if m.mode == modeTree && m.sect == sectionChanges {
			m.sectIdx[sectionChanges] = m.fileIdx
		}
		if oldHunk != m.hunkIdx || oldLine != m.lineCursor || oldEOF != m.atEOF {
			off := m.viewport.YOffset()
			m.refreshViewport()
			m.viewport.SetYOffset(off)
			m.updateDisplayed()
		}
	}
	return cmd
}

// pageDown advances by one page of new content. The marker IS the
// page-break line: it sits at row 0 of the new view so the reader's
// attention starts on familiar text and the rest of the file unfurls
// downward. Exception: on the last page, where the viewport can't
// scroll far enough to put the marker on row 0, the marker drops to
// row (pageOverlap - 1) — the 5th line of the last page. One more
// pageDown after that lands on the EOF marker.
func (m *model) pageDown() {
	if len(m.lineRanges) == 0 || m.atEOF {
		return
	}
	top := m.viewport.YOffset()
	height := m.viewport.Height()
	bot := top + height - 1
	oldHunk, oldLine := m.hunkIdx, m.lineCursor

	// Marker = the last reviewable line whose topRow <= (bot - pageOverlap).
	// That line was sitting `pageOverlap` rows above the bottom of the
	// old view, which is where the reader was about to lose context, so
	// it becomes the anchor for the next page.
	target := bot - pageOverlap
	var marker *lineRange
	for i := range m.lineRanges {
		lr := &m.lineRanges[i]
		if lr.isEOF || lr.kind == review.LineDelete {
			continue
		}
		if lr.topRow > target {
			break
		}
		marker = lr
	}
	if marker == nil {
		for i := range m.lineRanges {
			lr := &m.lineRanges[i]
			if lr.isEOF || lr.kind == review.LineDelete {
				continue
			}
			if lr.botRow >= top {
				marker = lr
				break
			}
		}
	}
	if marker == nil {
		return
	}

	// Try to put the marker on row 0. If the viewport clamps (last
	// page), keep the same marker — it just appears wherever the
	// clamped view shows it. The marker IS the (bot-5)th line of the
	// page the user just left; that's their attention anchor whether
	// we're mid-file or on the last page.
	m.viewport.SetYOffset(marker.topRow)
	m.placeCursor(*marker)

	// If even the last-page-exception marker didn't advance the cursor
	// (we were already there), step onto EOF.
	if oldHunk == m.hunkIdx && oldLine == m.lineCursor && m.viewport.AtBottom() {
		if eof := m.eofRange(); eof != nil {
			m.placeCursor(*eof)
		}
	}

	off := m.viewport.YOffset()
	m.refreshViewport()
	m.viewport.SetYOffset(off)
	m.updateDisplayed()
	if m.mode == modeTree && m.sect == sectionChanges {
		m.sectIdx[sectionChanges] = m.fileIdx
	}
}

// pageUp mirrors pageDown: marker = the reviewable line that was at
// row pageOverlap of the current view (so on the way back up, the line
// the reader was holding stays visible), placed at row (height -
// pageOverlap - 1) of the new view so most of the new content is above
// the marker. At the top of the file the viewport clamps and the
// marker sits naturally on whichever row it lands.
func (m *model) pageUp() {
	if len(m.lineRanges) == 0 {
		return
	}
	if m.atEOF {
		// Step off the EOF marker onto the last reviewable line of
		// the file; the next pageUp scrolls from there.
		f := m.currentFile()
		if last := len(f.Hunks) - 1; last >= 0 {
			m.hunkIdx = last
			lh := &f.Hunks[last]
			m.lineCursor = m.firstNonDelete(lh, len(lh.Lines)-1, -1)
		}
		m.atEOF = false
		m.refreshViewport()
		return
	}
	top := m.viewport.YOffset()
	height := m.viewport.Height()

	targetRow := top + pageOverlap
	var marker *lineRange
	for i := range m.lineRanges {
		lr := &m.lineRanges[i]
		if lr.isEOF || lr.kind == review.LineDelete {
			continue
		}
		if lr.topRow >= targetRow {
			marker = lr
			break
		}
	}
	if marker == nil {
		// Fall back to bottommost reviewable in current view.
		bot := top + height - 1
		for i := range m.lineRanges {
			lr := &m.lineRanges[i]
			if lr.isEOF || lr.kind == review.LineDelete {
				continue
			}
			if lr.topRow > bot {
				break
			}
			marker = lr
		}
	}
	if marker == nil {
		return
	}

	// Place marker near the bottom of the new view.
	desiredTop := marker.topRow - (height - pageOverlap - 1)
	if desiredTop < 0 {
		desiredTop = 0
	}
	m.viewport.SetYOffset(desiredTop)
	m.placeCursor(*marker)

	off := m.viewport.YOffset()
	m.refreshViewport()
	m.viewport.SetYOffset(off)
	m.updateDisplayed()
	if m.mode == modeTree && m.sect == sectionChanges {
		m.sectIdx[sectionChanges] = m.fileIdx
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
		// Park the section cursor on the file we were just reviewing.
		m.sect = sectionFileReview
		for i, fr := range m.sess.FileReviews() {
			if fr.Path == m.filePath {
				m.sectIdx[sectionFileReview] = i
				break
			}
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
			m.recordVisitedLine()
			m.refreshViewport()
		}
	case "k", "up":
		if m.fileLineCursor > 0 {
			m.fileLineCursor--
			m.recordVisitedLine()
			m.refreshViewport()
		}
	case "home":
		m.fileLineCursor = 0
		m.recordVisitedLine()
		m.refreshViewport()
	case "end":
		m.fileLineCursor = max(0, len(m.fileLines)-1)
		m.recordVisitedLine()
		m.refreshViewport()
	case " ", "space":
		// Same semantics everywhere: next unread in Changes. From modeFile
		// this takes the user out into modeDiff at the next unread hunk.
		m.spaceWalk()
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
		// Viewport scroll fallback in file-review mode: keep the line
		// cursor in view (snap to the first visible line if scrolled away).
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if top := m.viewport.YOffset(); m.fileLineCursor < top {
			m.fileLineCursor = top
		} else if bot := top + m.viewport.Height() - 1; m.fileLineCursor > bot {
			m.fileLineCursor = bot
		}
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
		// Commit a fresh verdict audit-log entry alongside the summary so
		// each explicit V submission becomes a new entry in # Verdicts.
		// (The </> cycle only mutates the canonical state and produces no
		// audit entry until the user lands here.)
		m.sess.SetSummary(text)
		m.sess.AddVerdict(review.VerdictEvent{
			State:   m.sess.Verdict,
			Summary: text,
		})
		m.save("verdict recorded")
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
			// the tick fires.
			m.pendingReads[r.anchor] = true
			anchor := r.anchor
			m.queuedCmds = append(m.queuedCmds, tea.Tick(m.readDelay, func(time.Time) tea.Msg {
				return delayedReadMsg{anchor: anchor}
			}))
		}
	}
}


func (m *model) refreshViewport() {
	// The viewport width depends on whether the sidebar is showing,
	// which depends on mode. Re-resize on every refresh so mode flips
	// take effect on the next render.
	m.resize()
	var body string
	var ranges []hunkRange
	var lines []lineRange
	var cursorRow int
	switch m.mode {
	case modeDiff:
		body, ranges, lines, cursorRow = renderFileDiff(m)
	case modeFile:
		body, cursorRow = renderFileView(m)
	case modeTree:
		// Peek: render whatever the selection suggests.
		switch m.sect {
		case sectionChanges:
			body, ranges, lines, cursorRow = renderFileDiff(m)
		case sectionFileReview:
			body, cursorRow = renderFileView(m)
		case sectionCommits:
			body = renderCommitDetail(m)
		case sectionIssues:
			body = renderIssueDetail(m)
		}
	}
	m.hunkRanges = ranges
	m.lineRanges = lines
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
	cursorBg      = lipgloss.Color("236")
	styleLineCur  = lipgloss.NewStyle().Background(cursorBg).Bold(true)
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

	var body string
	main := m.viewMain()
	if m.sidebarVisible() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.viewSidebar(), main)
	} else {
		body = main
	}

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
	for _, sec := range []section{sectionSources, sectionVerdicts, sectionIssues, sectionChanges, sectionCommits, sectionTree, sectionFileReview} {
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

// wrapDiffText hard-wraps a single diff line's payload (no sign, no gutter)
// at `width`. ansi.Hardwrap preserves any escape codes embedded in `s`.
// Tabs are expanded to spaces first because Hardwrap counts a tab as
// width 1 while the terminal expands it to the next 8-column stop —
// without expansion, lines with tabs slip past `width`, the terminal
// re-wraps them itself, and our hanging-indent never gets applied.
func wrapDiffText(s string, width int) []string {
	if width < 1 {
		return []string{s}
	}
	s = expandTabs(s, 8)
	wrapped := ansi.Hardwrap(s, width, false)
	return strings.Split(wrapped, "\n")
}

// expandTabs replaces each tab with enough spaces to advance to the next
// `tabSize`-column tab stop. It only inspects ASCII so it stays cheap;
// any embedded ANSI escape sequences are passed through unchanged but
// counted as visible — diff payload doesn't normally contain escapes.
func expandTabs(s string, tabSize int) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	col := 0
	for _, r := range s {
		if r == '\t' {
			pad := tabSize - (col % tabSize)
			for i := 0; i < pad; i++ {
				b.WriteByte(' ')
			}
			col += pad
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

func renderFileDiff(m *model) (body string, ranges []hunkRange, lines []lineRange, cursorRow int) {
	var sb strings.Builder
	f := m.currentFile()
	editing := m.edit == editComment || m.edit == editQuestion

	// Gutter width: enough digits for the largest new- or old-side line
	// number in any hunk of this file.
	maxNum := 0
	for _, h := range f.Hunks {
		if n := h.NewStart + h.NewLines - 1; n > maxNum {
			maxNum = n
		}
		if n := h.OldStart + h.OldLines - 1; n > maxNum {
			maxNum = n
		}
	}
	numW := len(fmt.Sprintf("%d", maxNum))
	if numW < 1 {
		numW = 1
	}
	blank := strings.Repeat(" ", numW)
	gutterW := numW*2 + 3 // "<old> <new>  "

	// Wrap content (excluding the "+ "/"- "/"  " sign prefix) so that the
	// continuation rows hang past the sign, aligning with the text the
	// user is reading. We pre-wrap so the line cursor highlight stays
	// scoped to the logical line that's selected.
	vpW := m.viewport.Width()
	wrapW := vpW - gutterW - 2 // 2 for the sign+space prefix
	if wrapW < 10 {
		wrapW = 10
	}

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
		// Header is plain (no cursor highlight on the @@-line) — the
		// active cursor lives on a content line below.
		gutterPad := strings.Repeat(" ", gutterW)
		sb.WriteString(gutterPad + styleHunk.Render(readMark+mk+h.Header) + "\n")
		row++

		// Editor splices below the line the cursor is on.
		editorLineIdx := -1
		if editing && hi == m.hunkIdx {
			editorLineIdx = m.lineCursor
		}

		oldLine := h.OldStart
		newLine := h.NewStart
		for li, ln := range h.Lines {
			var oldStr, newStr string
			switch ln.Kind {
			case review.LineAdd:
				oldStr = blank
				newStr = fmt.Sprintf("%*d", numW, newLine)
			case review.LineDelete:
				oldStr = fmt.Sprintf("%*d", numW, oldLine)
				newStr = blank
			default:
				oldStr = fmt.Sprintf("%*d", numW, oldLine)
				newStr = fmt.Sprintf("%*d", numW, newLine)
			}
			var sign string
			var styleLn lipgloss.Style
			switch ln.Kind {
			case review.LineAdd:
				sign = "+ "
				styleLn = styleAdd
			case review.LineDelete:
				sign = "- "
				styleLn = styleDel
			default:
				sign = "  "
				styleLn = styleCtx
			}
			isCursor := !m.atEOF && hi == m.hunkIdx && li == m.lineCursor
			lineTop := row
			parts := wrapDiffText(ln.Text, wrapW)
			// When the cursor is on this line, paint every piece with a
			// matching background so the whole row reads as one highlight.
			// Wrapping a pre-styled string in styleLineCur won't work:
			// each nested style ends with `\x1b[0m`, which resets the
			// background too — leaving only the gutter highlighted.
			gutterStyle := styleDim
			lineStyle := styleLn
			if isCursor {
				gutterStyle = gutterStyle.Background(cursorBg)
				lineStyle = lineStyle.Background(cursorBg).Bold(true)
			}
			for j, part := range parts {
				var head, body string
				if j == 0 {
					head = gutterStyle.Render(oldStr + " " + newStr + "  ")
					body = lineStyle.Render(sign + part)
				} else {
					head = gutterStyle.Render(strings.Repeat(" ", gutterW+2))
					body = lineStyle.Render(part)
				}
				if isCursor && j == 0 {
					cursorRow = row
				}
				line := head + body
				if isCursor {
					// The viewport doesn't pad short lines; add our own
					// trailing fill so the highlight extends to the right.
					used := lipgloss.Width(line)
					if pad := vpW - used; pad > 0 {
						line += lineStyle.Render(strings.Repeat(" ", pad))
					}
				}
				sb.WriteString(line + "\n")
				row++
			}
			lines = append(lines, lineRange{
				hunkIdx: hi, lineIdx: li, topRow: lineTop, botRow: row - 1, kind: ln.Kind,
			})

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
			if ln.Kind != review.LineAdd {
				oldLine++
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
		row++
	}

	// Synthetic "<end of file>" marker — gives the user an unambiguous
	// landing spot when the walk hits the end of a file.
	eofTop := row
	eofText := "<end of file>"
	eofStyle := styleDim
	if m.atEOF {
		eofStyle = eofStyle.Background(cursorBg).Bold(true)
		cursorRow = row
	}
	eofLine := strings.Repeat(" ", gutterW) + eofStyle.Render(eofText)
	if m.atEOF {
		if pad := vpW - lipgloss.Width(eofLine); pad > 0 {
			eofLine += eofStyle.Render(strings.Repeat(" ", pad))
		}
	}
	sb.WriteString(eofLine + "\n")
	row++
	lines = append(lines, lineRange{hunkIdx: -1, topRow: eofTop, botRow: row - 1, isEOF: true})

	return sb.String(), ranges, lines, cursorRow
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

