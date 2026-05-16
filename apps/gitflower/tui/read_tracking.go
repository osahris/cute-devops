// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Per-view read tracking. updateDisplayed schedules ONE viewReadMsg
// tick whose delay equals the count of visible-unread reviewable
// lines divided by the configured readRate. When the tick fires (and
// only if the view hasn't changed since), every visible unread line
// flips to read in one batch. The colour-refresh is coalesced.

package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

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

// peekKind reports what kind of trackable content is currently being
// shown — both the line modes AND the modeTree peek count as
// trackable so the reviewer's eyes on the peek pane still earn read
// marks. Returns "" when there's nothing to track.
func (m *model) peekKind() string {
	switch m.mode {
	case modeDiff:
		return "diff"
	case modeFile:
		return "file"
	case modeTree:
		switch m.sect {
		case sectionChanges:
			return "diff"
		case sectionFileReview, sectionTree:
			return "file"
		}
	}
	return ""
}

// countVisibleUnreadLines counts reviewable diff lines or file-content
// lines that are currently in the viewport and not yet read — for
// both real line modes and the modeTree peek panes. Drives the
// per-view tick delay.
func (m *model) countVisibleUnreadLines() int {
	top := m.viewport.YOffset()
	bot := top + m.viewport.Height() - 1
	count := 0
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
			count++
		}
	case "file":
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

// refreshViewport re-renders the active mode's body into the
// viewport. The viewport offset is only nudged when the cursor would
// otherwise leave the visible window — avoids the "jumpy" feel of
// re-centering on every arrow key.
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
		switch m.sect {
		case sectionChanges:
			body, ranges, lines, cursorRow = renderFileDiff(m)
		case sectionFileReview:
			body, cursorRow = renderFileView(m)
		case sectionCommits:
			body = renderCommitDetail(m)
		case sectionIssues:
			body = renderIssueDetail(m)
		case sectionVerdicts:
			body = renderVerdictDetail(m)
		}
	}
	m.hunkRanges = ranges
	m.lineRanges = lines
	m.viewport.SetContent(body)
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
