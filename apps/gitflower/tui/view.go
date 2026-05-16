// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Rendering: View() + sidebar + main pane + commit/issue peek panes
// + inline events + style palette. All the strings the terminal
// actually paints live here.

package tui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gitflower/review"
)

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
	// styleMessageBg is the dim grey BG used for unread commit-message
	// prose lines. Read message lines drop back to no BG.
	styleMessageBg = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	styleDelUnread = lipgloss.NewStyle().Background(lipgloss.Color("88")) // mid red
	styleDelRead   = lipgloss.NewStyle().Background(lipgloss.Color("52")) // dim red
	styleDelSkip   = lipgloss.NewStyle().Background(lipgloss.Color("52")).Strikethrough(true)
	// Back-compat aliases (pick the unread variants).
	styleAdd      = styleAddUnread
	styleDel      = styleDelUnread
	styleCtx      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleHunk     = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	styleCursor   = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	cursorBg          = lipgloss.Color("54")  // dark purple — focused cursor
	cursorUnfocusedBg = lipgloss.Color("237") // dim grey — unfocused cursor
	styleLineCur      = lipgloss.NewStyle().Background(cursorBg).Bold(true)
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

// suppress unused-warning lints for back-compat aliases.
var _ = []lipgloss.Style{styleAdd, styleDel, styleSel, styleUnread, styleCursor}

func sidebarWidth(total int) int {
	// Scale the sidebar with the terminal: a third of the width on
	// wide screens, clamped to [24, 60] so it never crowds the main
	// pane on giant monitors or disappears on tiny ones.
	w := total / 3
	if w < 24 {
		w = 24
	}
	if w > 60 {
		w = 60
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
	if m.sidebarVisible() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.viewSidebar(), m.viewMain())
	} else {
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
		return "Tree[" + m.sect.Label() + "]"
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

	var lines []string
	cursorRow := 0
	// Sidebar focus colour: purple when the reviewer is in Tree
	// mode (the sidebar IS the active pane), grey when they're in
	// a line mode (the cursor stays visible but as a parked
	// breadcrumb).
	sidebarBg := cursorBg
	if m.mode != modeTree {
		sidebarBg = cursorUnfocusedBg
	}
	sidebarCur := lipgloss.NewStyle().Background(sidebarBg).Bold(true)
	// sectionFileReview is intentionally omitted: visited files are
	// highlighted in the Tree section instead of living in their own
	// list.
	for _, sec := range []section{sectionSources, sectionVerdicts, sectionIssues, sectionChanges, sectionCommits, sectionTree} {
		items := m.sectionItems(sec)
		hdr := fmt.Sprintf("%s (%d)", sec.Label(), len(items))
		if m.sect == sec {
			lines = append(lines, styleFocused.Render(hdr))
		} else {
			lines = append(lines, styleSectHdr.Render(hdr))
		}
		for i, item := range items {
			marker := "  "
			isCursor := m.sect == sec && i == m.sectIdx[sec]
			if isCursor {
				marker = "▶ "
				cursorRow = len(lines)
			}
			// Commit rows start with the SHA slug; truncate from the
			// right so the slug always stays visible. Everything else
			// keeps the trailing-tail truncation (file names etc).
			label := item
			if sec == sectionCommits {
				label = truncateRight(item, w-3)
			} else {
				label = truncate(item, w-3)
			}
			line := marker + label
			if isCursor {
				line = sidebarCur.Render(line)
			}
			lines = append(lines, line)
		}
		lines = append(lines, "")
	}

	// Window the rendered sidebar to fit the available height.
	available := max(3, m.height-4)
	if len(lines) > available {
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

// fileReadStats is kept as a shim; new code should call fileLineCounts.
func (m *model) fileReadStats(idx int) (read, total int) {
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
		if strings.HasPrefix(path, "commit:") {
			short := strings.TrimPrefix(path, "commit:")
			subject := ""
			for _, c := range m.sess.Scope.Commits {
				if c.Short == short {
					subject = c.Subject
					break
				}
			}
			if subject != "" {
				path = "commit " + short + "  " + subject
			} else {
				path = "commit " + short
			}
		}
		return path + h
	case modeFile:
		return m.filePath + "  @" + truncate(m.sess.Scope.TipSHA, 12)
	}
	return ""
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
		// Cursor on the "+ Add issue" sentinel — show the affordance
		// instead of an empty pane so users know how to act on it.
		return styleDim.Render("Press Enter (or `i`) to add a new general issue.\n`e` edits, `d` deletes the highlighted entry.")
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

// renderVerdictDetail shows the verdict at the cursor in the peek
// pane. When the cursor sits on the "+ Add verdict" sentinel (or
// there are no verdicts yet), it shows the affordance instead.
func renderVerdictDetail(m *model) string {
	idx := m.sectIdx[sectionVerdicts]
	vs := m.sess.Verdicts()
	if idx >= len(vs) {
		return styleDim.Render("Press Enter (or `v`) to record your verdict.\nOne verdict per reviewer — submitting again replaces it.")
	}
	v := vs[idx]
	var sb strings.Builder
	sb.WriteString(styleTitle.Render(string(v.State)) + "\n")
	if v.Author != "" {
		sb.WriteString(styleDim.Render(v.Author+"  "+v.Date) + "\n")
	}
	if v.Summary != "" {
		sb.WriteString("\n" + v.Summary + "\n")
	}
	if v.Author == m.sess.Reviewer {
		sb.WriteString("\n" + styleDim.Render("`e` edits, `d` deletes.") + "\n")
	}
	return sb.String()
}

// ---------------------------------------------------------------------
// rendering: inline events (comments, reactions) + snippets
// ---------------------------------------------------------------------

func renderInlineComment(c review.Comment, selected bool) string {
	icon := "💬"
	if c.Kind == review.KindQuestion {
		icon = "❓"
	}
	style := styleDim
	if selected {
		// Reverse-video the selected event so the cursor on it is
		// visible — same affordance as the diff-line cursor.
		style = styleCursor
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
		sb.WriteString(style.Render(prefix+ln) + "\n")
	}
	return sb.String()
}

// commentSelected reports whether the comment at session index `idx`
// is currently the marked target for e/d.
func (m *model) commentSelected(idx int) bool {
	return m.commentCursor == idx
}

// renderInlineEventsForLine renders all events anchored to a specific
// new-side line of `path`. Comment/Question/Like/Dislike all show inline.
func renderInlineEventsForLine(m *model, path string, newLine int) string {
	var sb strings.Builder
	for i, c := range m.sess.Comments() {
		if eventAnchoredToLine(c.Anchor, path, newLine) {
			sb.WriteString(renderInlineComment(c, m.commentSelected(i)))
		}
	}
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
	for i, c := range m.sess.Comments() {
		if c.Anchor == hunkAnchor {
			sb.WriteString(renderInlineComment(c, m.commentSelected(i)))
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
