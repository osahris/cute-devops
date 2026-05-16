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

// DefaultReadRate is the assumed reviewer reading speed in lines per
// second. The per-hunk delay before a fully-displayed hunk gets marked
// read is computed as (reviewable lines in hunk) / DefaultReadRate, so
// a 30-line hunk at 10 l/s takes 3 seconds, a 5-line hunk takes 0.5s.
const DefaultReadRate = 10.0

// minReadDelay is the floor for the per-hunk delay. Even a 1-line
// hunk at a generous rate still goes through the tick path so the
// Update/Cmd round-trip behaves identically in tests and at runtime.
const minReadDelay = time.Millisecond

// AutoSaveInterval is the debounce window for write-on-dirty. Multiple
// rapid mutations (e.g. a Space walk through a long diff) coalesce into
// one save per AutoSaveInterval window instead of one save per mutation.
const AutoSaveInterval = 2 * time.Second

// Run launches the TUI on sess. readRate is the reviewer's assumed
// reading speed in lines per second; the per-hunk read delay scales
// with the hunk's reviewable line count and the current viewport size
// so big hunks earn more time than small ones.
func Run(sess *review.ReviewSession, readRate float64) error {
	root, _ := gitRoot()
	if readRate <= 0 {
		readRate = DefaultReadRate
	}
	m := newModel(sess, root, readRate)
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

// treeNodeKind discriminates a folder row from a file row in the
// rendered Changes / Tree sidebars.
type treeNodeKind int

const (
	tnFile treeNodeKind = iota
	tnDir
)

// treeRow is one visible row in a folder-aware sidebar. It carries
// just enough metadata to render the row and to act on it (drill in,
// skip, expand/collapse).
type treeRow struct {
	kind    treeNodeKind
	depth   int
	dirPath string // for tnDir: the folder path (no trailing slash); for tnFile: containing folder
	name    string // basename
	fullPath string // for tnFile: the file path; for tnDir: same as dirPath
	fileIdx int    // for tnFile in Changes: index into m.files; -1 otherwise
}

// lineKey is the in-memory identity of a single diff line. We use
// (fileIdx, hunkIdx, lineIdx) instead of a string anchor because the
// reading model doesn't care about persistence — saves go through the
// review.Session API and only persist whole-file aggregates.
type lineKey struct {
	fileIdx int
	hunkIdx int
	lineIdx int
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

	// Folder-tree state for the sidebar's Changes view. Folders with
	// any unread file are expanded by default; user can toggle.
	changesExpanded map[string]bool // dir path → expanded; absent = default
	// Folder-tree state for the sectionTree (file-tree at tip SHA).
	// All folders collapsed by default; user expands manually.
	fileTreeExpanded map[string]bool
	// Cached visible rows per section (built lazily by sectionItems).
	changesRows []treeRow
	fileTreeRows []treeRow

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

	// Per-line read state. Hunks are visual separators only; reading
	// happens line by line. Keys are global file-line identifiers
	// (fileIdx, hunkIdx, lineIdx); values are true once the line has
	// been visible long enough to count as read.
	lineRead    map[lineKey]bool
	lineSkipped map[lineKey]bool

	// File-mode per-line read state. Keys are filePath strings; the
	// inner map holds (1-based line number → read). This lets the
	// reviewer see which lines of a file they've already scrolled
	// through.
	fileLineRead map[string]map[int]bool


	// Per-view read tick. The user reads the lines currently in the
	// viewport over a "lines_visible / readRate" window. Each time
	// the view stabilises on a new set of unread lines we schedule
	// one tick; when it fires (and the view hasn't changed since)
	// all currently-visible unread lines flip to read.
	viewReadGen       int      // monotonic generation; bumped on any view change
	viewReadScheduled bool     // dedupe scheduling
	lastViewFile      int      // last viewState seen by updateDisplayed
	lastViewOffset    int

	// Delayed read marking.
	readRate float64 // lines/second

	// Autosave: when true an autoSaveMsg tick is in flight.
	saveScheduled bool

	// Coalesced colour-refresh: set true while a colourRefreshMsg is
	// queued so we don't pile up 200 refresh ticks during a flurry of
	// per-line read marks.
	colourRefreshScheduled bool

	// queuedCmds accumulate Cmds produced by non-Cmd-returning helpers
	// (refreshViewport, updateDisplayed); Update batches and drains them.
	queuedCmds []tea.Cmd

	status string
}

func newModel(sess *review.ReviewSession, root string, readRate float64) *model {
	if readRate <= 0 {
		readRate = DefaultReadRate
	}
	files := review.ParseDiff(sess.Scope.RawDiff)

	// Append each commit's patch as a virtual file so Space-walk
	// naturally traverses commits the same way it traverses files. The
	// virtual path is `commit:<short>:<path>` so hunks anchored to it
	// get their own read state and never collide with real files; the
	// sidebar's Changes list filters them out so they only show up via
	// the line-mode walk.
	for _, c := range sess.Scope.Commits {
		patch := sess.Scope.CommitPatch(c.SHA)
		if strings.TrimSpace(patch) == "" {
			continue
		}
		// Prepend a synthetic file holding the commit's mbox-style
		// header + message body so the reviewer reads the why before
		// the diff. We model it as a brand-new file added by the
		// commit, so its lines participate in the normal per-line
		// read-tracking just like code lines.
		if preamble := commitMessageDiff(c.Short, patch); preamble != "" {
			files = append(files, review.ParseDiff(preamble)...)
		}
		ch := review.ParseDiff(patch)
		for i := range ch {
			ch[i].Path = "commit:" + c.Short + ":" + ch[i].Path
		}
		files = append(files, ch...)
	}

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
		readRate:    readRate,
		lineRead:    map[lineKey]bool{},
		lineSkipped: map[lineKey]bool{},
		lastViewOffset:   -1,
		changesExpanded:  map[string]bool{},
		fileTreeExpanded: map[string]bool{},
		fileLineRead:     map[string]map[int]bool{},
	}
	return m
}

// ---------------------------------------------------------------------
// tea.Model
// ---------------------------------------------------------------------

func (m *model) Init() tea.Cmd { return nil }


// viewReadMsg fires after the reading-time window for the current
// viewport elapses. The handler verifies the view hasn't changed since
// the tick was scheduled (gen match) before marking every visible
// unread reviewable line as read.
type viewReadMsg struct{ gen int }

// autoSaveMsg fires AutoSaveInterval after dirty state was detected.
// Coalesces many rapid mutations into a single Save call.
type autoSaveMsg struct{}

// colourRefreshMsg coalesces visual updates after a flurry of read-tick
// fires. Without it, marking 200 lines read in quick succession would
// trigger 200 full re-renders.
type colourRefreshMsg struct{}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.refreshViewport()
		return m, m.drainCmds()
	case viewReadMsg:
		// Reading-time window elapsed. If the viewport hasn't changed
		// since we scheduled this tick (gen match), flip every visible
		// unread reviewable line to read.
		m.viewReadScheduled = false
		if msg.gen != m.viewReadGen {
			return m, m.drainCmds()
		}
		marked := 0
		top := m.viewport.YOffset()
		bot := top + m.viewport.Height() - 1
		switch m.mode {
		case modeDiff:
			for _, lr := range m.lineRanges {
				if lr.isEOF {
					continue
				}
				if lr.kind != review.LineAdd && lr.kind != review.LineDelete {
					continue
				}
				if lr.botRow < top || lr.topRow > bot {
					continue
				}
				lk := lineKey{fileIdx: m.fileIdx, hunkIdx: lr.hunkIdx, lineIdx: lr.lineIdx}
				if m.lineRead[lk] {
					continue
				}
				// Skipped doesn't block reading; promotion clears the
				// skip flag so a line is read OR skipped OR unread.
				m.lineRead[lk] = true
				delete(m.lineSkipped, lk)
				marked++
			}
		case modeFile:
			setForFile := m.fileLineRead[m.filePath]
			if setForFile == nil {
				setForFile = map[int]bool{}
				m.fileLineRead[m.filePath] = setForFile
			}
			for row := top; row <= bot && row < len(m.fileLines); row++ {
				if row < 0 {
					continue
				}
				ln := row + 1
				if setForFile[ln] {
					continue
				}
				setForFile[ln] = true
				marked++
			}
		}
		if marked > 0 {
			m.scheduleAutoSave()
			m.scheduleColourRefresh()
		}
		// Re-evaluate display in case there's still unread content on
		// screen (e.g. tall hunks where some lines are clipped).
		m.updateDisplayed()
		return m, m.drainCmds()
	case colourRefreshMsg:
		m.colourRefreshScheduled = false
		off := m.viewport.YOffset()
		m.refreshViewport()
		m.viewport.SetYOffset(off)
		m.updateDisplayed()
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

// sidebarMinTotalWidth is the minimum terminal width at which the
// section sidebar gets its own column next to the peek pane. Below
// this we still want the section view in section mode — just
// full-screen instead of as a sidebar.
const sidebarMinTotalWidth = 150

// sidebarVisible reports whether to render the section sidebar
// alongside the right-pane peek (wide layout). When false, the right
// pane gets the whole screen — and in section mode that pane swaps
// to the sidebar's content (see fullScreenSections).
func (m *model) sidebarVisible() bool {
	return m.mode == modeTree && m.width >= sidebarMinTotalWidth
}

// fullScreenSections reports whether we're in narrow-section-mode:
// section mode but the window is too narrow for a sidebar/peek
// split. The main pane then renders the section list itself.
func (m *model) fullScreenSections() bool {
	return m.mode == modeTree && m.width < sidebarMinTotalWidth
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
	case "esc", "left", "h":
		// Hard-wrap: left scrolls horizontally; only when at column 0 do
		// we go back to section mode. Soft-wrap (and Esc): always exit.
		if (key.String() == "left" || key.String() == "h") && !m.viewport.SoftWrap && m.viewport.XOffset() > 0 {
			m.viewport.ScrollLeft(4)
			return m, nil
		}
		// Park the section cursor on the file we were just reviewing so
		// the user lands back on it in section mode.
		m.sect = sectionChanges
		if r := m.changesRowForFile(m.fileIdx); r >= 0 { m.sectIdx[sectionChanges] = r }
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
		// Mark the current line unread: drop its read state and bump
		// the view generation so any pending read tick is invalidated.
		lk := m.currentLineKey()
		delete(m.lineRead, lk)
		delete(m.lineSkipped, lk)
		m.viewReadGen++
		m.viewReadScheduled = false
		off := m.viewport.YOffset()
		m.refreshViewport()
		m.viewport.SetYOffset(off)
		m.updateDisplayed()
		m.scheduleAutoSave()
		m.status = "marked unread"
	case " ", "space":
		m.spaceWalk()
	case "alt+space":
		m.skipWalk()
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
	if f == nil || len(f.Hunks) == 0 || m.hunkIdx < 0 || m.hunkIdx >= len(f.Hunks) {
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

// currentLineKey returns the lineKey under the cursor (modeDiff only).
func (m *model) currentLineKey() lineKey {
	return lineKey{fileIdx: m.fileIdx, hunkIdx: m.hunkIdx, lineIdx: m.lineCursor}
}

// skipWalk marks every unread reviewable line in the current file
// from the cursor forward (within the current hunk) as Skipped, then
// jumps to the next unread line / file. Alt+Space — for templates and
// other content the reviewer wants to acknowledge-and-skip in bulk.
func (m *model) skipWalk() {
	if !m.atEOF && m.mode == modeDiff {
		if f := m.currentFile(); f != nil && m.hunkIdx >= 0 && m.hunkIdx < len(f.Hunks) {
			h := &f.Hunks[m.hunkIdx]
			for li := m.lineCursor; li < len(h.Lines); li++ {
				if h.Lines[li].Kind != review.LineAdd && h.Lines[li].Kind != review.LineDelete {
					continue
				}
				lk := lineKey{fileIdx: m.fileIdx, hunkIdx: m.hunkIdx, lineIdx: li}
				if m.lineRead[lk] || m.lineSkipped[lk] {
					continue
				}
				m.lineSkipped[lk] = true
			}
			m.scheduleAutoSave()
		}
	}
	m.spaceWalk()
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

	// Section mode → drill in.
	if m.mode == modeTree {
		if m.sect == sectionChanges {
			if fi := m.changesFileFromRow(m.sectIdx[sectionChanges]); fi >= 0 {
				m.fileIdx = fi
			}
		}
		m.hunkIdx = -1
		m.atEOF = false
		m.mode = modeDiff
		m.refreshViewport()
	}

	// On EOF in the current file: advance to the next file (or commit
	// "file") that still has an unread hunk. If none remain, open the
	// verdict editor.
	if m.atEOF {
		for fi := m.fileIdx + 1; fi < len(m.files); fi++ {
			if m.fileHasUnread(fi) {
				m.fileIdx = fi
				m.hunkIdx = -1
				m.atEOF = false
				m.refreshViewport()
				m.spaceWalkInFile()
				return
			}
		}
		// All files (including commit virtual files) are fully read.
		m.openVerdictEditor()
		m.status = "all read — record your verdict"
		return
	}

	m.spaceWalkInFile()
}

// spaceWalkInFile jumps the cursor to "5 rendered rows before the next
// unread reviewable line in the current file". If nothing in the file
// is unread, the cursor parks on the EOF marker; the next Space then
// hops to the next file that still has unread lines.
func (m *model) spaceWalkInFile() {
	f := m.currentFile()
	if f == nil {
		return
	}

	nextHi, nextLi := m.nextUnreadLineInFile(m.fileIdx, m.hunkIdx, m.lineCursor)
	if nextHi < 0 {
		// Nothing unread left in this file — park on EOF.
		m.atEOF = true
		m.refreshViewport()
		if eof := m.eofRange(); eof != nil {
			target := eof.topRow - (m.viewport.Height() - pageOverlap)
			if target < 0 {
				target = 0
			}
			m.viewport.SetYOffset(target)
			m.updateDisplayed()
		}
		return
	}

	// Already parked on the next unread line: true no-op. Leaving
	// cursor and viewport alone means PgDn progress is preserved and
	// no extra render disturbs the read tick that's already counting.
	if nextHi == m.hunkIdx && nextLi == m.lineCursor {
		return
	}

	// Jump to the unread line, back the viewport up by `pageOverlap`
	// rows so the reader has a strip of just-read context above it.
	m.hunkIdx = nextHi
	m.lineCursor = nextLi
	m.refreshViewport()
	for _, lr := range m.lineRanges {
		if lr.isEOF || lr.hunkIdx != nextHi || lr.lineIdx != nextLi {
			continue
		}
		top := lr.topRow - pageOverlap
		if top < 0 {
			top = 0
		}
		m.viewport.SetYOffset(top)
		m.updateDisplayed()
		return
	}
}

// nextUnreadLineInFile finds the first reviewable (add or delete) line
// at or after (startHi, startLi) in file fi that's neither read nor
// skipped. Returns (-1, -1) if nothing is left.
func (m *model) nextUnreadLineInFile(fi, startHi, startLi int) (int, int) {
	if fi < 0 || fi >= len(m.files) {
		return -1, -1
	}
	f := &m.files[fi]
	if startHi < 0 {
		startHi = 0
		startLi = 0
	}
	for hi := startHi; hi < len(f.Hunks); hi++ {
		h := &f.Hunks[hi]
		first := 0
		if hi == startHi {
			first = startLi
			if first < 0 {
				first = 0
			}
		}
		for li := first; li < len(h.Lines); li++ {
			if h.Lines[li].Kind != review.LineAdd && h.Lines[li].Kind != review.LineDelete {
				continue
			}
			lk := lineKey{fileIdx: fi, hunkIdx: hi, lineIdx: li}
			if m.lineRead[lk] || m.lineSkipped[lk] {
				continue
			}
			return hi, li
		}
	}
	return -1, -1
}

// fileHasUnread reports whether any reviewable line in m.files[fi] is
// still unread AND not skipped.
func (m *model) fileHasUnread(fi int) bool {
	hi, _ := m.nextUnreadLineInFile(fi, 0, 0)
	return hi >= 0
}

// fileLineCounts returns the (read, total) reviewable-line tally for
// file fi. Used by the sidebar to show review progress per file.
func (m *model) fileLineCounts(fi int) (read, total int) {
	if fi < 0 || fi >= len(m.files) {
		return 0, 0
	}
	f := &m.files[fi]
	for hi, h := range f.Hunks {
		for li, ln := range h.Lines {
			if ln.Kind != review.LineAdd && ln.Kind != review.LineDelete {
				continue
			}
			total++
			lk := lineKey{fileIdx: fi, hunkIdx: hi, lineIdx: li}
			if m.lineRead[lk] {
				read++
			}
		}
	}
	return read, total
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

// scheduleColourRefresh queues a single colourRefreshMsg to redraw the
// diff so newly-read lines flip to their dim colour. Coalesced so a
// burst of read ticks only triggers one full re-render.
func (m *model) scheduleColourRefresh() {
	if m.colourRefreshScheduled {
		return
	}
	m.colourRefreshScheduled = true
	m.queuedCmds = append(m.queuedCmds, tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return colourRefreshMsg{}
	}))
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
			if r := m.changesRowForFile(m.fileIdx); r >= 0 { m.sectIdx[sectionChanges] = r }
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
		// No reviewable lines at all (e.g. an all-delete hunk). We
		// still need to make progress so the read tick can fire when
		// the hunk fully displays — scroll the viewport by a page and
		// let snap pick whatever lands on screen.
		step := m.viewport.Height() - pageOverlap
		if step < 1 {
			step = 1
		}
		m.viewport.SetYOffset(top + step)
		off := m.viewport.YOffset()
		m.refreshViewport()
		m.viewport.SetYOffset(off)
		m.updateDisplayed()
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
		if r := m.changesRowForFile(m.fileIdx); r >= 0 { m.sectIdx[sectionChanges] = r }
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
		if r := m.changesRowForFile(m.fileIdx); r >= 0 { m.sectIdx[sectionChanges] = r }
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
	// Hunks are visual separators; "fully displayed" is no longer a
	// meaningful read-state signal. Just advance the hunk index.
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

// updateDisplayed schedules one read tick for the current viewport.
// The tick's delay is (visible unread reviewable lines) / readRate, so
// 30 lines on screen at 10 l/s take 3 seconds before they all flip to
// read. If the view changes (scroll, file flip, etc.) before the tick
// fires, the generation counter mismatches and the old tick is a no-op.
func (m *model) updateDisplayed() {
	cur := struct{ file, off int }{m.fileIdx, m.viewport.YOffset()}
	if cur.file != m.lastViewFile || cur.off != m.lastViewOffset {
		m.viewReadGen++
		m.viewReadScheduled = false
		m.lastViewFile = cur.file
		m.lastViewOffset = cur.off
	}
	if m.viewReadScheduled {
		return
	}
	count := m.countVisibleUnreadLines()
	if count == 0 {
		return
	}
	delay := m.viewReadDelay(count)
	m.viewReadScheduled = true
	gen := m.viewReadGen
	m.queuedCmds = append(m.queuedCmds, tea.Tick(delay, func(time.Time) tea.Msg {
		return viewReadMsg{gen: gen}
	}))
}

// countVisibleUnreadLines counts reviewable diff lines (modeDiff) or
// file-content lines (modeFile) that are currently in the viewport
// and not yet read. Drives the per-view tick delay.
func (m *model) countVisibleUnreadLines() int {
	top := m.viewport.YOffset()
	bot := top + m.viewport.Height() - 1
	count := 0
	switch m.mode {
	case modeDiff:
		for _, lr := range m.lineRanges {
			if lr.isEOF {
				continue
			}
			if lr.kind != review.LineAdd && lr.kind != review.LineDelete {
				continue
			}
			if lr.botRow < top || lr.topRow > bot {
				continue
			}
			lk := lineKey{fileIdx: m.fileIdx, hunkIdx: lr.hunkIdx, lineIdx: lr.lineIdx}
			if m.lineRead[lk] {
				continue
			}
			count++
		}
	case modeFile:
		// File-mode rendered rows = file lines, 1:1 (no wrap-aware
		// lineRanges yet). The viewport's row index IS the line index.
		setForFile := m.fileLineRead[m.filePath]
		for row := top; row <= bot && row < len(m.fileLines); row++ {
			if row < 0 {
				continue
			}
			if setForFile[row+1] {
				continue
			}
			count++
		}
	}
	return count
}

// viewReadDelay = lines / readRate, clamped to the minimum tick.
func (m *model) viewReadDelay(lines int) time.Duration {
	if lines < 1 {
		lines = 1
	}
	rate := m.readRate
	if rate <= 0 {
		rate = DefaultReadRate
	}
	d := time.Duration(float64(time.Second) * float64(lines) / rate)
	if d < minReadDelay {
		d = minReadDelay
	}
	return d
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
	// Only scroll the viewport when the cursor would otherwise leave
	// the visible window. Re-centering on every refresh made arrow
	// navigation feel jumpy — the viewport would snap whenever the
	// cursor was already comfortably visible.
	top := m.viewport.YOffset()
	bot := top + m.viewport.Height() - 1
	if cursorRow < top || cursorRow > bot {
		target := cursorRow - m.viewport.Height()/3
		if target < 0 {
			target = 0
		}
		m.viewport.SetYOffset(target)
	}
	m.updateDisplayed()
}

// ---------------------------------------------------------------------
// view
// ---------------------------------------------------------------------

var (
	// Add/Delete lines render with a coloured background. Two
	// brightnesses: stronger when the hunk is still unread (so it
	// pulls attention), softer once it's been read or skipped (so
	// the eye glides past).
	styleAddUnread = lipgloss.NewStyle().Background(lipgloss.Color("28")) // mid green
	styleAddRead   = lipgloss.NewStyle().Background(lipgloss.Color("22")) // dim green
	// Skipped lines: still green/red so the reviewer sees it's an
	// add/delete, but with strikethrough so "I chose not to read this"
	// reads clearly. If a skipped line ever gets viewed long enough to
	// promote to Read, the read style wins (render checks read first).
	styleAddSkip   = lipgloss.NewStyle().Background(lipgloss.Color("22")).Strikethrough(true)
	styleDelUnread = lipgloss.NewStyle().Background(lipgloss.Color("88")) // mid red
	styleDelRead   = lipgloss.NewStyle().Background(lipgloss.Color("52")) // dim red
	styleDelSkip   = lipgloss.NewStyle().Background(lipgloss.Color("52")).Strikethrough(true)
	// Back-compat aliases (used by legacy call sites; pick the unread variants).
	styleAdd      = styleAddUnread
	styleDel      = styleDelUnread
	styleCtx      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleHunk     = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	styleCursor   = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	cursorBg      = lipgloss.Color("54") // dark purple (xterm-256)
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
	switch {
	case m.sidebarVisible():
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.viewSidebar(), m.viewMain())
	case m.fullScreenSections():
		// Narrow section mode: the section list IS the main view.
		body = m.viewSidebar()
	default:
		body = m.viewMain()
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
	// In wide layout the sidebar gets its own column; in narrow
	// (full-screen) section mode it takes the whole screen.
	var w int
	if m.fullScreenSections() {
		w = m.width
	} else {
		w = sidebarWidth(m.width)
	}

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
			// Note: Changes rows already carry "R/T" progress as part
			// of their label (see renderTreeRow / sectionItems), so no
			// extra stats column here.
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
	// Kept for any external callers; new code should use fileLineCounts.
	return m.fileLineCounts(idx)
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
		path := f.Path
		// Pretty-print commit-virtual paths: "commit:<short>:<real>"
		// → "commit <short>  <real>".
		if strings.HasPrefix(path, "commit:") {
			rest := strings.TrimPrefix(path, "commit:")
			if i := strings.Index(rest, ":"); i > 0 {
				path = "commit " + rest[:i] + "  " + rest[i+1:]
			}
		}
		return path + h
	case modeFile:
		return m.filePath + "  @" + truncate(m.sess.Scope.TipSHA, 12)
	}
	return ""
}

// ---------------------------------------------------------------------
// rendering: diff view (modeDiff and Diffs-section peek)
// ---------------------------------------------------------------------

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
		// Hunks are purely visual separators now: render the @@ title
		// with no read marker. Marker (good/bad) still applies because
		// reactions are inherently hunk-level.
		var mk string
		switch m.sess.Marker(anchor) {
		case review.MarkerGood:
			mk = styleMarkGood.Render("+ ")
		case review.MarkerBad:
			mk = styleMarkBad.Render("- ")
		default:
			mk = "  "
		}
		gutterPad := strings.Repeat(" ", gutterW)
		sb.WriteString(gutterPad + styleHunk.Render(mk+h.Header) + "\n")
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
			lk := lineKey{fileIdx: m.fileIdx, hunkIdx: hi, lineIdx: li}
			read := m.lineRead[lk]
			skipped := m.lineSkipped[lk]
			_ = anchor
			switch ln.Kind {
			case review.LineAdd:
				sign = "+ "
				switch {
				case read:
					styleLn = styleAddRead
				case skipped:
					styleLn = styleAddSkip
				default:
					styleLn = styleAddUnread
				}
			case review.LineDelete:
				sign = "- "
				switch {
				case read:
					styleLn = styleDelRead
				case skipped:
					styleLn = styleDelSkip
				default:
					styleLn = styleDelUnread
				}
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

