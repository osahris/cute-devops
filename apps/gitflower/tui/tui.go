// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Package tui is a bubbletea-v2 driver for review sessions. State changes
// go through *review.ReviewSession methods so a web driver can do the same.
// Every mutation auto-saves the file.
//
// Three top-level modes:
//
//	modeTree  – sidebar focused, browses 4 sections:
//	             Diffs    : changed files (selecting → opens Diff)
//	             Tree     : files at tip SHA (selecting → opens File review)
//	             Commits  : commits in scope (selecting → filters Diff to that commit)
//	             Issues   : free-form issues added during review (i adds one)
//	modeDiff  – split-diff view; hunk-level or line-level sub-state
//	modeFile  – full file at tip SHA, line cursor, comments anchored by path:line
package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitflower/review"
)

func Run(sess *review.ReviewSession) error {
	root, _ := gitRoot()
	m := newModel(sess, root)
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

	// Diff mode state.
	fileIdx    int
	hunkIdx    int
	diffLine   bool // line-level sub-mode of diff (was modeLines)
	lineCursor int
	selStart   int
	selEnd     int

	// File review mode state.
	filePath       string   // currently-open file in modeFile
	fileLines      []string // content of filePath at tip SHA
	fileLineCursor int
	fileSelStart   int
	fileSelEnd     int

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

	status string
}

func newModel(sess *review.ReviewSession, root string) *model {
	files := review.ParseDiff(sess.Scope.RawDiff)

	ta := textarea.New()
	ta.CharLimit = 0
	ta.ShowLineNumbers = false

	ti := textinput.New()
	ti.Placeholder = "Issue title"
	ti.CharLimit = 200

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	treeFiles, _ := gitTreeFiles(sess.Scope.TipSHA)

	m := &model{
		sess:       sess,
		root:       root,
		files:      files,
		treeFiles:  treeFiles,
		mode:       modeTree,
		sect:       sectionChanges,
		editCmtIdx: -1,
		editIssIdx: -1,
		textarea:   ta,
		title:      ti,
		viewport:   vp,
		displayed:  map[review.Anchor]map[int]bool{},
	}
	return m
}

// ---------------------------------------------------------------------
// tea.Model
// ---------------------------------------------------------------------

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.refreshViewport()
		return m, nil
	}

	if m.confirmQuit {
		return m.updateConfirmQuit(msg)
	}
	if m.edit != editNone {
		return m.updateEdit(msg)
	}
	switch m.mode {
	case modeTree:
		return m.updateTree(msg)
	case modeDiff:
		return m.updateDiff(msg)
	case modeFile:
		return m.updateFile(msg)
	}
	return m, nil
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
		m.mode = modeDiff
		m.refreshViewport()
	case sectionFileReview:
		idx := m.sectIdx[m.sect]
		frs := m.sess.FileReviews()
		if idx < len(frs) {
			m.filePath = frs[idx].Path
			m.fileLines, _ = gitFileLines(m.sess.Scope.TipSHA, m.filePath)
			m.fileLineCursor = 0
			m.fileSelStart = 0
			m.fileSelEnd = 0
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
	case "left", "h":
		if m.diffLine {
			m.diffLine = false
			m.refreshViewport()
		} else {
			m.mode = modeTree
			m.refreshViewport()
		}
	case "right", "l":
		if !m.diffLine {
			m.enterLineSubMode()
		}
	case "j", "down":
		if m.diffLine {
			m.lineNext()
		} else {
			m.advanceHunk()
		}
	case "k", "up":
		if m.diffLine {
			m.linePrev()
		} else {
			m.prevHunk()
		}
	case "alt+down":
		if m.diffLine {
			m.extendSel(+1)
		}
	case "alt+up":
		if m.diffLine {
			m.extendSel(-1)
		}
	case "n", "tab":
		m.nextFile()
	case "p", "shift+tab":
		m.prevFile()
	case "home":
		m.hunkIdx = 0
		m.refreshViewport()
	case "end":
		m.hunkIdx = max(0, len(m.currentFile().Hunks)-1)
		m.refreshViewport()
	case "u":
		a := m.currentAnchor()
		m.sess.MarkUnread(a)
		delete(m.displayed, a)
		m.save("marked unread")
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

func (m *model) enterLineSubMode() {
	h := m.currentHunk()
	if h == nil || len(h.Lines) == 0 {
		m.status = "no lines"
		return
	}
	m.diffLine = true
	m.lineCursor = m.firstNonDelete(h, 0, +1)
	m.selStart = m.lineCursor
	m.selEnd = m.lineCursor
	m.refreshViewport()
}

func (m *model) lineNext() {
	h := m.currentHunk()
	if h == nil {
		return
	}
	next := m.firstNonDelete(h, m.lineCursor+1, +1)
	if next != m.lineCursor {
		m.lineCursor = next
		m.selStart = next
		m.selEnd = next
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
		m.selStart = prev
		m.selEnd = prev
		m.refreshViewport()
	}
}

func (m *model) extendSel(step int) {
	h := m.currentHunk()
	if h == nil {
		return
	}
	next := m.firstNonDelete(h, m.lineCursor+step, step)
	if next == m.lineCursor {
		return
	}
	m.lineCursor = next
	m.selStart, m.selEnd = minmax(m.selStart, m.lineCursor)
	m.refreshViewport()
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
		if m.fileSelEnd > m.fileSelStart {
			return review.Anchor(fmt.Sprintf("%s@%s:%d-%d", m.filePath, m.sess.Scope.TipSHA[:12], m.fileSelStart+1, m.fileSelEnd+1))
		}
		return review.Anchor(fmt.Sprintf("%s@%s:%d", m.filePath, m.sess.Scope.TipSHA[:12], m.fileLineCursor+1))
	case modeDiff:
		f := m.currentFile()
		h := m.currentHunk()
		if h == nil {
			return review.Anchor(f.Path)
		}
		if m.diffLine {
			start, end := m.selectionNewLines(h)
			if start > 0 {
				if end > start {
					return review.Anchor(fmt.Sprintf("%s:%d-%d", f.Path, start, end))
				}
				return review.Anchor(fmt.Sprintf("%s:%d", f.Path, start))
			}
		}
		return review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
	}
	return review.Anchor("")
}

func (m *model) selectionNewLines(h *review.Hunk) (start, end int) {
	newLine := h.NewStart
	for i, ln := range h.Lines {
		if i < m.selStart {
			if ln.Kind != review.LineDelete {
				newLine++
			}
			continue
		}
		if i > m.selEnd {
			break
		}
		if ln.Kind != review.LineDelete {
			if start == 0 {
				start = newLine
			}
			end = newLine
			newLine++
		}
	}
	return
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
	case "left", "h":
		m.mode = modeTree
		m.refreshViewport()
	case "j", "down":
		if m.fileLineCursor+1 < len(m.fileLines) {
			m.fileLineCursor++
			m.fileSelStart = m.fileLineCursor
			m.fileSelEnd = m.fileLineCursor
			m.refreshViewport()
		}
	case "k", "up":
		if m.fileLineCursor > 0 {
			m.fileLineCursor--
			m.fileSelStart = m.fileLineCursor
			m.fileSelEnd = m.fileLineCursor
			m.refreshViewport()
		}
	case "alt+down":
		if m.fileLineCursor+1 < len(m.fileLines) {
			m.fileLineCursor++
			m.fileSelStart, m.fileSelEnd = minmax(m.fileSelStart, m.fileLineCursor)
			m.refreshViewport()
		}
	case "alt+up":
		if m.fileLineCursor > 0 {
			m.fileLineCursor--
			m.fileSelStart, m.fileSelEnd = minmax(m.fileLineCursor, m.fileSelEnd)
			m.refreshViewport()
		}
	case "home":
		m.fileLineCursor = 0
		m.fileSelStart, m.fileSelEnd = 0, 0
		m.refreshViewport()
	case "end":
		m.fileLineCursor = max(0, len(m.fileLines)-1)
		m.fileSelStart, m.fileSelEnd = m.fileLineCursor, m.fileLineCursor
		m.refreshViewport()
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
	dirtied := false
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
		if full {
			m.sess.MarkRead(r.anchor)
			dirtied = true
		}
	}
	if dirtied {
		_ = m.sess.Save()
	}
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
		return "Tree[" + m.sect.Label() + "]"
	case modeDiff:
		if m.diffLine {
			return "Diff[line]"
		}
		return "Diff"
	case modeFile:
		return "File"
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
		if m.diffLine {
			return "j/k line  alt+j/k extend  c/!/Enter comment  a/? question  g/b mark  e edit comment  ←/h back  q quit"
		}
		return "j next  k prev  →/l lines  ←/h tree  c comment  a question  g/b mark  u unread  e edit comment  >/< verdict  s save  q quit"
	case modeFile:
		return "j/k line  alt+j/k extend  c/!/Enter comment  a/? question  e edit comment  ←/h tree  q quit"
	}
	return ""
}

func (m *model) viewSidebar() string {
	w := sidebarWidth(m.width)
	var sb strings.Builder
	for _, sec := range []section{sectionSources, sectionVerdicts, sectionIssues, sectionChanges, sectionCommits, sectionFileReview} {
		items := m.sectionItems(sec)
		hdr := fmt.Sprintf("%s (%d)", sec.Label(), len(items))
		if m.mode == modeTree && m.sect == sec {
			sb.WriteString(styleFocused.Render(hdr) + "\n")
		} else {
			sb.WriteString(styleSectHdr.Render(hdr) + "\n")
		}
		// Show items, capping each section at 8 for sidebar density.
		cap := 8
		for i, item := range items {
			if i >= cap {
				sb.WriteString(styleDim.Render(fmt.Sprintf("  … +%d more", len(items)-cap)) + "\n")
				break
			}
			marker := "  "
			if m.mode == modeTree && m.sect == sec && i == m.sectIdx[sec] {
				marker = "▶ "
			}
			line := marker + truncate(item, w-3)
			if m.mode == modeTree && m.sect == sec && i == m.sectIdx[sec] {
				line = styleCursor.Render(line)
			}
			// Read-state annotation for Diffs only.
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
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}
	return lipgloss.NewStyle().Width(w).Render(sb.String())
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
		if m.diffLine {
			return f.Path + h + "   [line]"
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

		// Determine editor splice point for this hunk.
		editorLineIdx := -1
		if editing && hi == m.hunkIdx {
			if m.diffLine {
				editorLineIdx = m.selEnd
			} else {
				editorLineIdx = len(h.Lines) - 1
			}
		}

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
			if hi == m.hunkIdx && m.diffLine {
				if li == m.lineCursor {
					styled = styleLineCur.Render(styled)
					cursorRow = row
				} else if li >= m.selStart && li <= m.selEnd {
					styled = styleSel.Render(styled)
				}
			}
			sb.WriteString(styled + "\n")
			row++

			if li == editorLineIdx {
				rs := renderInlineEditor(m)
				sb.WriteString(rs)
				row += strings.Count(rs, "\n")
			}
		}

		// Existing comments anchored to this anchor or any sub-anchor of this file
		// that lies within this hunk's line range.
		for _, c := range m.sess.Comments() {
			if !anchorBelongsToHunk(c.Anchor, f.Path, &h) {
				continue
			}
			rs := renderInlineComment(c)
			sb.WriteString(rs)
			row += strings.Count(rs, "\n")
		}
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
			} else if i >= m.fileSelStart && i <= m.fileSelEnd {
				styled = styleSel.Render(styled)
			}
		}
		sb.WriteString(styled + "\n")

		if editing && m.mode == modeFile && i == m.fileSelEnd {
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
			prefix = "    " + icon + " " + string(c.Anchor) + " — " + c.Author + ": "
		} else {
			prefix = "      "
		}
		sb.WriteString(styleDim.Render(prefix+ln) + "\n")
	}
	return sb.String()
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

func minmax(a, b int) (int, int) {
	if a < b {
		return a, b
	}
	return b, a
}
