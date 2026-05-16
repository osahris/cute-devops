// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// modeDiff: line-by-line diff view. Handles key dispatch, cursor
// movement within a file (lineNext/Prev, j/k), file-cursor accessors,
// the marker reaction (g/+ /b/-), and the diff renderer.

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitflower/review"
)

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
		// If we were inside a commit virtual file, jump back to the
		// Commits section parked on that commit — not to Changes.
		if path := m.currentFile().Path; strings.HasPrefix(path, "commit:") {
			short := strings.TrimPrefix(path, "commit:")
			for i, c := range m.sess.Scope.Commits {
				if c.Short == short {
					m.sect = sectionCommits
					m.sectIdx[sectionCommits] = i
					break
				}
			}
		} else {
			m.sect = sectionChanges
			if r := m.changesRowForFile(m.fileIdx); r >= 0 {
				m.sectIdx[sectionChanges] = r
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
		// Enter file-review mode on the current Changes file.
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
		return
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

// applyMarker sets the good/bad marker on the current anchor and saves.
func (m *model) applyMarker(mk review.Marker) {
	a := m.currentAnchor()
	m.sess.SetMarker(a, mk)
	m.save(markerStatus(m.sess.Marker(a)))
}

// renderFileDiff builds the body of the diff viewport for the current
// file. Returns the rendered body, hunk row ranges, line row ranges
// (used by the cursor and read-tracking code), and the rendered-row
// index of the cursor's first row.
func renderFileDiff(m *model) (body string, ranges []hunkRange, lines []lineRange, cursorRow int) {
	var sb strings.Builder
	f := m.currentFile()
	editing := m.edit == editComment || m.edit == editQuestion

	// Gutter width: enough digits for the largest new- or old-side
	// line number in any hunk of this file.
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

	vpW := m.viewport.Width()
	wrapW := vpW - gutterW - 2 // 2 for the sign+space prefix
	if wrapW < 10 {
		wrapW = 10
	}

	row := 0
	for hi, h := range f.Hunks {
		topRow := row
		anchor := review.HunkAnchor(f.Path, h.NewStart, h.NewLines)
		isMessage := h.Header == commitMessageHeader
		// Hunks are visual separators: render the @@ title with no
		// read marker, just the good/bad reaction. The synthetic
		// "commit message" hunk skips its title row entirely — the
		// message flows straight into the buffer.
		if !isMessage {
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
		}

		editorLineIdx := -1
		if editing && hi == m.hunkIdx {
			editorLineIdx = m.lineCursor
		}

		oldLine := h.OldStart
		newLine := h.NewStart
		for li, ln := range h.Lines {
			var oldStr, newStr string
			switch {
			case isMessage:
				oldStr = blank
				newStr = blank
			case ln.Kind == review.LineAdd:
				oldStr = blank
				newStr = fmt.Sprintf("%*d", numW, newLine)
			case ln.Kind == review.LineDelete:
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
			switch {
			case isMessage:
				// Commit-message lines: plain prose with a dim grey
				// background while unread (the "needs attention"
				// hint, matching the green/red unread BG on diff
				// lines). Once read the BG drops so the eye glides
				// past. Skipped is just the dim grey too.
				sign = "  "
				switch {
				case read:
					styleLn = lipgloss.NewStyle()
				case skipped:
					styleLn = styleMessageBg
				default:
					styleLn = styleMessageBg
				}
			case ln.Kind == review.LineAdd:
				sign = "+ "
				switch {
				case read:
					styleLn = styleAddRead
				case skipped:
					styleLn = styleAddSkip
				default:
					styleLn = styleAddUnread
				}
			case ln.Kind == review.LineDelete:
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
			// Message lines wrap at the full viewport width (no gutter).
			lineWrapW := wrapW
			if isMessage {
				lineWrapW = vpW
			}
			parts := wrapDiffText(ln.Text, lineWrapW)
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
				var head, bodyPart string
				switch {
				case isMessage:
					// Commit message: no gutter at all, full-width
					// left-aligned prose.
					head = ""
					bodyPart = lineStyle.Render(part)
				case j == 0:
					head = gutterStyle.Render(oldStr + " " + newStr + "  ")
					bodyPart = lineStyle.Render(sign + part)
				default:
					head = gutterStyle.Render(strings.Repeat(" ", gutterW+2))
					bodyPart = lineStyle.Render(part)
				}
				if isCursor && j == 0 {
					cursorRow = row
				}
				line := head + bodyPart
				if isCursor {
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

		// Hunk-anchored events (Like/Dislike, ReadStart/End, etc).
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

	// Synthetic "<end of file>" marker — unambiguous landing spot.
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

// anchorBelongsToHunk reports whether the line/range anchor `a`
// belongs inside the hunk `h` of `path`.
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
