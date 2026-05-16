// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Space-walk / Alt+Space-skip / Page navigation. The "walk" advances
// the cursor to the next unread line (and across files/commits as
// needed); the page helpers (pageDown/pageUp/scrollViewport) handle
// the viewport mechanics with marker-on-row-0 + last-page-exception
// semantics.

package tui

import (
	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

// pageOverlap is how many reviewable lines from the bottom of the
// current view get carried over to the top of the next page. The
// cursor lands on the (overlap+1)th-from-bottom line so the reader
// sees that line stay put as the marker between pages, then the rest
// comes into view below it.
const pageOverlap = 5

// debugSpaceWalk, when not nil, is called once per spaceWalk entry.
// Used by tests to introspect state transitions.
var debugSpaceWalk func(stage string, m *model)

// skipWalk marks every reviewable line currently in the viewport
// (that isn't already read) as Skipped, then jumps to the next
// unread line / file. Bound to `s` and Alt+Enter in line mode, and
// Alt+Space everywhere — for templates / generated content the
// reviewer wants to acknowledge-and-skip a page at a time. To skip
// a whole file in one go, use `s` on the file's row in the Changes
// or Tree sidebar instead (see skipFromSidebar).
func (m *model) skipWalk() {
	if !m.atEOF && m.mode == modeDiff {
		top := m.viewport.YOffset()
		bot := top + m.viewport.Height() - 1
		skipped := 0
		for _, lr := range m.lineRanges {
			if lr.isEOF {
				continue
			}
			if lr.kind != review.LineAdd && lr.kind != review.LineDelete {
				continue
			}
			// Only count lines whose rendered span overlaps the
			// viewport — that's "the page the user is looking at".
			if lr.botRow < top || lr.topRow > bot {
				continue
			}
			lk := lineKey{fileIdx: m.fileIdx, hunkIdx: lr.hunkIdx, lineIdx: lr.lineIdx}
			if m.lineRead[lk] || m.lineSkipped[lk] {
				continue
			}
			m.markLineSkipped(lk)
			skipped++
		}
		if skipped > 0 {
			m.scheduleAutoSave()
		}
	}
	m.spaceWalk()
}

// spaceWalk advances the cursor to the next unread reviewable line.
// From section mode it first drills into the file under the section
// cursor; on EOF it advances to the next file with unread; when the
// whole scope is read it opens the verdict editor.
func (m *model) spaceWalk() {
	if debugSpaceWalk != nil {
		debugSpaceWalk("entry", m)
		defer debugSpaceWalk("exit", m)
	}

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

	if m.atEOF {
		for fi := m.fileIdx + 1; fi < len(m.files); fi++ {
			if m.fileHasUnread(fi) {
				m.fileIdx = fi
				m.hunkIdx = -1
				m.atEOF = false
				m.refreshViewport()
				m.syncSidebarsToCurrentFile()
				m.spaceWalkInFile()
				return
			}
		}
		// All files (including commit virtuals) are fully read.
		m.openVerdictEditor()
		m.status = "all read — record your verdict"
		return
	}

	m.spaceWalkInFile()
	m.syncSidebarsToCurrentFile()
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
	// cursor and viewport alone preserves PgDn progress.
	if nextHi == m.hunkIdx && nextLi == m.lineCursor {
		return
	}

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

// refreshViewportWithContext renders the current file and scrolls the
// viewport so the cursor's hunk sits with `ctx` lines of context above
// it. Falls back to top-of-content when fewer rows are available.
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

// snapCursorIntoView picks a reviewable line for the cursor based on
// the current viewport position. If picked successfully, it also
// nudges the viewport so the chosen line sits at row 0 — otherwise
// hunk headers, blank inter-hunk rows, or inline comment rows would
// occupy the top of the view and the cursor would visually appear on
// row 1, 2, or 3 instead of row 0.
func (m *model) snapCursorIntoView() {
	if len(m.lineRanges) == 0 {
		return
	}
	top := m.viewport.YOffset()
	for _, lr := range m.lineRanges {
		if lr.topRow < top {
			continue
		}
		if !lr.isEOF && lr.kind == review.LineDelete {
			continue
		}
		m.viewport.SetYOffset(lr.topRow)
		m.placeCursor(lr)
		return
	}
	// Nothing starts at-or-below top; pick the line whose wrap-span
	// covers top so the cursor highlight is at least partially visible.
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
	m.placeCursor(m.lineRanges[len(m.lineRanges)-1])
}

// eofRange returns the lineRange of the synthetic EOF marker for the
// currently-rendered file, or nil if there isn't one.
func (m *model) eofRange() *lineRange {
	for i := range m.lineRanges {
		if m.lineRanges[i].isEOF {
			return &m.lineRanges[i]
		}
	}
	return nil
}

// placeCursor moves the diff-mode cursor to the line described by lr,
// switching to / out of the EOF state as needed. Callers are
// responsible for re-rendering and any viewport scroll they want.
func (m *model) placeCursor(lr lineRange) {
	if lr.isEOF {
		m.atEOF = true
		return
	}
	m.atEOF = false
	m.hunkIdx = lr.hunkIdx
	m.lineCursor = lr.lineIdx
}

// scrollViewport is the one true viewport-scroll path. For
// mouse-wheel / arrow-scroll messages we forward to the viewport
// directly; for page-sized navigation (PgUp/PgDn) callers use
// pageDown / pageUp explicitly because those have richer semantics.
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
		m.syncSidebarsToCurrentFile()
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
// page-break line: it sits at row 0 of the new view. On the last
// page where the viewport can't scroll far enough, the marker drops
// to row (pageOverlap - 1).
func (m *model) pageDown() {
	if len(m.lineRanges) == 0 || m.atEOF {
		return
	}
	top := m.viewport.YOffset()
	height := m.viewport.Height()
	bot := top + height - 1
	oldHunk, oldLine := m.hunkIdx, m.lineCursor

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
		// No reviewable lines in view (e.g. an all-delete hunk).
		// Scroll by a page so the read tick still gets a chance.
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

	m.viewport.SetYOffset(marker.topRow)
	m.placeCursor(*marker)

	if oldHunk == m.hunkIdx && oldLine == m.lineCursor && m.viewport.AtBottom() {
		if eof := m.eofRange(); eof != nil {
			m.placeCursor(*eof)
		}
	}

	off := m.viewport.YOffset()
	m.refreshViewport()
	m.viewport.SetYOffset(off)
	m.updateDisplayed()
	m.syncSidebarsToCurrentFile()
}

// pageUp mirrors pageDown.
func (m *model) pageUp() {
	if len(m.lineRanges) == 0 {
		return
	}
	if m.atEOF {
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
	m.syncSidebarsToCurrentFile()
}

// toggleWrap switches the diff/file viewport between soft-wrap and
// hard-wrap. Hard-wrap enables horizontal scrolling via left/right.
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
