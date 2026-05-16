// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// modeFile: line-by-line view of the working-tree contents of one file
// at the scope's tip SHA. Reaches here either via `F` from a Changes
// diff or via Enter on a Tree row.

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// updateFile handles key input while the reviewer is browsing a file
// in modeFile.
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
	case "esc", "left", "h":
		if (key.String() == "left" || key.String() == "h") && !m.viewport.SoftWrap && m.viewport.XOffset() > 0 {
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

// renderFileView builds the body of the file-content viewport.
func renderFileView(m *model) (body string, cursorRow int) {
	var sb strings.Builder
	editing := m.edit == editComment || m.edit == editQuestion
	digits := len(fmt.Sprintf("%d", len(m.fileLines)))
	setForFile := m.fileLineRead[m.filePath]
	for i, ln := range m.fileLines {
		row := fmt.Sprintf("%*d  %s", digits, i+1, ln)
		// Dim "already-viewed" lines like read diff lines, so the
		// reviewer can see at a glance how far they've scrolled.
		if setForFile[i+1] {
			row = styleRead.Render(row)
		}
		if m.mode == modeFile && i == m.fileLineCursor {
			row = styleLineCur.Render(row)
			cursorRow = sb.Len()
			_ = cursorRow
		}
		sb.WriteString(row + "\n")

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
	if len(m.fileLines) > 0 {
		cursorRow = m.fileLineCursor
	}
	return sb.String(), cursorRow
}
