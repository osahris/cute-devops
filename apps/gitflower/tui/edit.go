// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Edit overlays: comment, question, verdict summary, issue. While any
// edit is open, all key input is routed to the textarea/title; Esc
// cancels and Alt+Enter / Ctrl+S submits. Also the unsaved-changes
// confirm-quit prompt lives here.

package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitflower/review"
)

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

// openVerdictEditor opens the inline summary editor pre-populated with
// the current canonical summary. Queues the textarea's focus cmd into
// queuedCmds so callers that don't bubble a tea.Cmd back (the void
// helpers like spaceWalk) still get the focus dispatched on the next
// drainCmds — without that, the cursor never starts blinking and the
// editor looks frozen.
func (m *model) openVerdictEditor() {
	m.openEdit(editSummary, "", -1, -1, "")
	m.textarea.SetValue(m.sess.Summary)
	if cmd := m.textarea.Focus(); cmd != nil {
		m.queuedCmds = append(m.queuedCmds, cmd)
	}
}

// openIssueEditor opens the inline issue editor (new issue, title
// focused first). Same queuedCmds pattern as openVerdictEditor — the
// caller doesn't have to plumb a tea.Cmd back.
func (m *model) openIssueEditor() {
	m.openEdit(editIssue, "", -1, -1, "")
	if cmd := m.title.Focus(); cmd != nil {
		m.queuedCmds = append(m.queuedCmds, cmd)
	}
}

// editSelectedComment loads the comment under the comment-cursor (or
// the first one anchored near the line cursor) into the editor.
func (m *model) editSelectedComment() {
	idx, found := m.resolveCommentTarget()
	if !found {
		m.status = "no comment at cursor"
		return
	}
	m.commentCursor = idx
	c := m.sess.Comments()[idx]
	kind := editComment
	if c.Kind == review.KindQuestion {
		kind = editQuestion
	}
	m.openEdit(kind, "Editing "+commentKindWord(c.Kind)+" on "+string(c.Anchor), idx, -1, "")
	if cmd := m.textarea.Focus(); cmd != nil {
		m.queuedCmds = append(m.queuedCmds, cmd)
	}
	m.textarea.SetValue(c.Text)
}

// deleteSelectedComment removes the comment under the comment-cursor
// (or the first one anchored near the line cursor). Returns false if
// nothing matches.
func (m *model) deleteSelectedComment() bool {
	idx, found := m.resolveCommentTarget()
	if !found {
		return false
	}
	c := m.sess.Comments()[idx]
	if !m.sess.DeleteComment(idx) {
		return false
	}
	// After delete the indices shift; clear the comment cursor so the
	// next e/d starts from the line-cursor lookup again.
	m.commentCursor = -1
	m.save(commentKindWord(c.Kind) + " deleted")
	return true
}

// resolveCommentTarget picks the comment that e/d should act on:
//   - if commentCursor is a valid index AND still anchored near the
//     line cursor → use it (the user "selected" with the marker).
//   - otherwise → search comments anchored to the current line OR
//     anywhere in the current hunk, returning the first match.
//
// The same lookup is used by the comment-cycle keys (`n`/`N`) so e/d
// and cycle navigation always see the same candidate set.
func (m *model) resolveCommentTarget() (int, bool) {
	if m.commentCursor >= 0 && m.commentCursor < len(m.sess.Comments()) {
		if m.commentAnchoredNearCursor(m.sess.Comments()[m.commentCursor].Anchor) {
			return m.commentCursor, true
		}
	}
	for _, idx := range m.commentsNearCursor() {
		return idx, true
	}
	return -1, false
}

// commentsNearCursor returns the indices into m.sess.Comments() of
// every comment whose anchor matches the current cursor's line OR
// the current hunk. Returned in session order so cycling is stable.
func (m *model) commentsNearCursor() []int {
	var out []int
	for i, c := range m.sess.Comments() {
		if m.commentAnchoredNearCursor(c.Anchor) {
			out = append(out, i)
		}
	}
	return out
}

// commentAnchoredNearCursor checks whether an anchor matches the
// current line cursor or the current hunk. Used to find candidate
// events for the e/d/n keys.
func (m *model) commentAnchoredNearCursor(a review.Anchor) bool {
	if m.mode != modeDiff {
		return a == m.currentAnchor()
	}
	f := m.currentFile()
	h := m.currentHunk()
	if f == nil || h == nil {
		return false
	}
	// Exact-line match: comment anchored to the same `+`/` ` line.
	line := m.cursorNewLine(h)
	if line > 0 {
		if a == review.Anchor(fmt.Sprintf("%s:%d", f.Path, line)) {
			return true
		}
	}
	// Hunk match: comment anchored to the surrounding hunk
	// (`path:start,count`), or to any other line inside the hunk.
	if a == review.HunkAnchor(f.Path, h.NewStart, h.NewLines) {
		return true
	}
	prefix := f.Path + ":"
	if !strings.HasPrefix(string(a), prefix) {
		return false
	}
	rest := string(a)[len(prefix):]
	// Lines in this hunk run [h.NewStart, h.NewStart+h.NewLines-1].
	// The anchor suffix is one of "N", "N-M", or "N,M"; we just need
	// the leading int N.
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return false
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return false
	}
	return n >= h.NewStart && n < h.NewStart+h.NewLines
}

func commentKindWord(k review.Kind) string {
	if k == review.KindQuestion {
		return "question"
	}
	return "comment"
}

// cycleCommentCursor moves the comment cursor through every event
// anchored to the current cursor line or hunk, step = +1 (next) or
// -1 (prev). Sets a status hint so the reviewer knows which item is
// now "selected".
func (m *model) cycleCommentCursor(step int) {
	cands := m.commentsNearCursor()
	if len(cands) == 0 {
		m.commentCursor = -1
		m.status = "no comments at cursor"
		return
	}
	cur := -1
	for i, idx := range cands {
		if idx == m.commentCursor {
			cur = i
			break
		}
	}
	if cur < 0 {
		// No comment was selected (or the previously-selected one
		// moved out of range): start at the first/last depending on
		// step direction.
		if step >= 0 {
			cur = 0
		} else {
			cur = len(cands) - 1
		}
	} else {
		cur = (cur + step + len(cands)) % len(cands)
	}
	m.commentCursor = cands[cur]
	c := m.sess.Comments()[m.commentCursor]
	m.status = fmt.Sprintf("selected %s %d/%d — e edits, d deletes",
		commentKindWord(c.Kind), cur+1, len(cands))
	m.refreshViewport()
}

// editEntryUnderCursor dispatches the section-mode `e` key. Returns
// true if it handled the keystroke; false if the current cursor row
// isn't editable. Caller surfaces a hint. Focus tea.Cmd is queued
// via the model's queuedCmds rather than returned, so the caller
// stays simple.
func (m *model) editEntryUnderCursor() bool {
	switch m.sect {
	case sectionIssues:
		idx := m.sectIdx[m.sect]
		issues := m.sess.Issues()
		if idx >= len(issues) {
			// "+ Add issue" sentinel: treat e same as opening a new one.
			m.openIssueEditor()
			return true
		}
		it := issues[idx]
		m.openEdit(editIssue, "", -1, idx, "")
		m.title.SetValue(it.Title)
		m.textarea.SetValue(it.Body)
		if cmd := m.title.Focus(); cmd != nil {
			m.queuedCmds = append(m.queuedCmds, cmd)
		}
		return true
	case sectionVerdicts:
		// Only your own verdict is editable; everyone else's row is
		// peek-only. The verdict editor pre-populates with the
		// existing state/summary either way.
		idx := m.sectIdx[m.sect]
		own := m.sess.VerdictIndexFor(m.sess.Reviewer)
		if own < 0 || idx == own {
			m.openVerdictEditor()
			return true
		}
		return false
	}
	return false
}

// deleteEntryUnderCursor handles the section-mode `d` key, returning
// true if it removed something. Verdicts only allow deleting your
// own.
func (m *model) deleteEntryUnderCursor() bool {
	switch m.sect {
	case sectionIssues:
		idx := m.sectIdx[m.sect]
		if idx >= len(m.sess.Issues()) {
			return false
		}
		if !m.sess.DeleteIssue(idx) {
			return false
		}
		if idx > 0 {
			m.sectIdx[m.sect] = idx - 1
		}
		m.save("issue deleted")
		return true
	case sectionVerdicts:
		own := m.sess.VerdictIndexFor(m.sess.Reviewer)
		idx := m.sectIdx[m.sect]
		if own < 0 || idx != own {
			return false
		}
		if !m.sess.DeleteVerdict(own) {
			return false
		}
		if idx > 0 {
			m.sectIdx[m.sect] = idx - 1
		}
		m.save("verdict deleted")
		return true
	}
	return false
}

func (m *model) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "esc":
			m.closeEdit()
			return m, nil
		case "enter":
			// Issue title: Enter advances to the body field, as
			// before — that's a single-line input where Enter never
			// makes sense as "submit incomplete".
			if m.edit == editIssue && m.title.Focused() {
				m.title.Blur()
				return m, m.textarea.Focus()
			}
			// Everywhere else (comment/question/verdict/issue body)
			// Enter is the save action. Newlines require Alt+Enter.
			m.submitEdit()
			return m, nil
		case "ctrl+s":
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

func (m *model) viewConfirmQuit() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		styleTitle.Render("Unsaved changes"),
		"",
		"Save before quitting? (y/n, Esc to cancel)",
	)
}

// renderInlineEditor formats the active edit overlay (textarea +
// optional title input) for splicing into the rendered viewport.
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
