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
	"strings"
	"time"

	keybinding "charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

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

	// Comment cursor — index into m.sess.Comments() naming the
	// currently-selected event for e/d, or -1 when no event is
	// "marked". Set when the user lands on an event row (j/k in
	// modeDiff steps through them as well as diff lines). e/d on a
	// selected event acts on it; otherwise they fall back to the
	// line-anchor lookup.
	commentCursor int

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
	// fileLineTotals[path] = total reviewable line count for the
	// file the reviewer last opened in modeFile. Used by the Tree
	// sidebar to decide whether to mark a file as ✓ "reviewed".
	fileLineTotals map[string]int


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
	// naturally traverses commits the same way it traverses files.
	// Each commit becomes ONE virtual file `commit:<short>` whose
	// hunks are: a leading message-and-headers hunk (so the reviewer
	// reads who/why first), then every hunk from the commit's diff
	// with its original file path inlined into the hunk header.
	// Drilling into the commit lands the reviewer in this single
	// scrollable buffer that contains everything about the commit;
	// per-line read tracking applies uniformly.
	for _, c := range sess.Scope.Commits {
		patch := sess.Scope.CommitPatch(c.SHA)
		if strings.TrimSpace(patch) == "" {
			continue
		}
		combined := review.File{Path: "commit:" + c.Short}
		// Leading "commit message" hunk — context lines so it renders
		// as prose (no "+ " sign, no green background).
		if mh := commitMessageHunk(patch); len(mh.Lines) > 0 {
			combined.Hunks = append(combined.Hunks, mh)
		}
		// Then every file's hunks, with the file path prepended to
		// the hunk header so the reader can see which file each hunk
		// belongs to as they scroll.
		for _, pf := range review.ParseDiff(patch) {
			for _, h := range pf.Hunks {
				h.Header = "[" + pf.Path + "] " + h.Header
				combined.Hunks = append(combined.Hunks, h)
			}
		}
		if len(combined.Hunks) > 0 {
			files = append(files, combined)
		}
	}

	ta := textarea.New()
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	// Reactions are mostly one-liners. Start at one row and let the
	// textarea grow as the body picks up actual newlines, so the
	// editor doesn't take over the diff for a 5-word comment.
	ta.DynamicHeight = true
	ta.MinHeight = 1
	// Plain Enter saves; Alt+Enter inserts a literal newline. The
	// default keymap has Enter bound to InsertNewline — rebind so
	// only Alt+Enter inserts.
	ta.KeyMap.InsertNewline = keybinding.NewBinding(keybinding.WithKeys("alt+enter"))

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
		fileLineTotals:   map[string]int{},
		commentCursor:    -1,
	}
	// Restore per-line read/skip from the session before the first
	// frame, so a restart shows the prior progress instead of an
	// empty slate.
	m.hydrateFromSession()
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
		switch m.peekKind() {
		case "diff":
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
				m.markLineRead(lk)
				m.unmarkLineSkipped(lk)
				marked++
			}
		case "file":
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
// sidebar gets its own column next to the main pane. Below this we
// give the main pane the whole screen (the sidebar wouldn't have
// enough room to be useful).
const sidebarMinTotalWidth = 46

// sidebarVisible reports whether to render the sidebar alongside the
// main pane. The sidebar is now always shown when the window is wide
// enough — focus indication (purple vs grey cursor) tells the
// reviewer which pane is active.
func (m *model) sidebarVisible() bool {
	return m.width >= sidebarMinTotalWidth
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
	// MaxHeight caps how tall the dynamic-grow textarea may get when
	// the body really does have many newlines. Leave room for the
	// inline-editor's label row + the title row (issue case) + some
	// breathing space above the diff.
	m.textarea.MaxHeight = max(1, mainH-4)
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


// scheduleAutoSave queues an autoSaveMsg tick so dirty state gets
// flushed to disk within AutoSaveInterval. Idempotent.
func (m *model) scheduleAutoSave() {
	if m.saveScheduled {
		return
	}
	m.saveScheduled = true
	m.queuedCmds = append(m.queuedCmds, tea.Tick(AutoSaveInterval, func(time.Time) tea.Msg {
		return autoSaveMsg{}
	}))
}

// scheduleColourRefresh queues a single colourRefreshMsg to redraw
// the diff so newly-read lines flip to their dim colour. Coalesced so
// a burst of read ticks only triggers one full re-render.
func (m *model) scheduleColourRefresh() {
	if m.colourRefreshScheduled {
		return
	}
	m.colourRefreshScheduled = true
	m.queuedCmds = append(m.queuedCmds, tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return colourRefreshMsg{}
	}))
}

