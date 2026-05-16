// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// modeTree: the section sidebar / full-screen section list. Handles
// section navigation (Tab, j/k), per-section row trees (Changes &
// Tree are folder-grouped), drill-in (Enter / right / l), folder
// expand/collapse, and the sidebar-scope 's' (skip) action.

package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"gitflower/review"
)

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
		m.changesRows = m.buildChangesRows()
		out := make([]string, len(m.changesRows))
		for i, row := range m.changesRows {
			out[i] = m.renderTreeRow(row, sectionChanges)
		}
		return out
	case sectionCommits:
		out := make([]string, len(m.sess.Scope.Commits))
		for i, c := range m.sess.Scope.Commits {
			out[i] = c.Short + "  " + c.Subject
		}
		return out
	case sectionTree:
		m.fileTreeRows = m.buildFileTreeRows()
		out := make([]string, len(m.fileTreeRows))
		for i, row := range m.fileTreeRows {
			out[i] = m.renderTreeRow(row, sectionTree)
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

// buildChangesRows constructs the folder tree for the Changes sidebar.
// Files are grouped by their containing directory; commit virtual
// files are excluded. A folder is expanded by default if any of its
// files has at least one unread reviewable line.
func (m *model) buildChangesRows() []treeRow {
	type fp struct{ idx int; path string }
	var files []fp
	for fi, f := range m.files {
		if strings.HasPrefix(f.Path, "commit:") {
			continue
		}
		files = append(files, fp{fi, f.Path})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })

	hasUnread := map[string]bool{}
	for _, fpr := range files {
		if !m.fileHasUnread(fpr.idx) {
			continue
		}
		dir := pathDir(fpr.path)
		for dir != "" {
			hasUnread[dir] = true
			dir = pathDir(dir)
		}
		hasUnread[""] = true
	}
	expanded := func(dir string) bool {
		if v, ok := m.changesExpanded[dir]; ok {
			return v
		}
		return hasUnread[dir]
	}

	var rows []treeRow
	var walkDir func(prefix string, depth int)
	walkDir = func(prefix string, depth int) {
		subdirs := map[string]bool{}
		var direct []fp
		for _, fpr := range files {
			if !pathInDir(fpr.path, prefix) {
				continue
			}
			rest := fpr.path
			if prefix != "" {
				rest = strings.TrimPrefix(fpr.path, prefix+"/")
			}
			if i := strings.Index(rest, "/"); i > 0 {
				subdirs[rest[:i]] = true
			} else {
				direct = append(direct, fpr)
			}
		}
		dirNames := make([]string, 0, len(subdirs))
		for d := range subdirs {
			dirNames = append(dirNames, d)
		}
		sort.Strings(dirNames)
		for _, name := range dirNames {
			full := name
			if prefix != "" {
				full = prefix + "/" + name
			}
			rows = append(rows, treeRow{
				kind: tnDir, depth: depth,
				dirPath: full, name: name, fullPath: full, fileIdx: -1,
			})
			if expanded(full) {
				walkDir(full, depth+1)
			}
		}
		for _, fpr := range direct {
			base := fpr.path
			if i := strings.LastIndex(base, "/"); i >= 0 {
				base = base[i+1:]
			}
			rows = append(rows, treeRow{
				kind: tnFile, depth: depth,
				dirPath: prefix, name: base, fullPath: fpr.path, fileIdx: fpr.idx,
			})
		}
	}
	walkDir("", 0)
	return rows
}

// buildFileTreeRows builds the sidebar tree for sectionTree (the file
// tree at tip SHA). Folders all default to collapsed.
func (m *model) buildFileTreeRows() []treeRow {
	paths := append([]string(nil), m.treeFiles...)
	sort.Strings(paths)
	expanded := func(dir string) bool {
		return m.fileTreeExpanded[dir]
	}
	var rows []treeRow
	var walkDir func(prefix string, depth int)
	walkDir = func(prefix string, depth int) {
		subdirs := map[string]bool{}
		var direct []string
		for _, p := range paths {
			if !pathInDir(p, prefix) {
				continue
			}
			rest := p
			if prefix != "" {
				rest = strings.TrimPrefix(p, prefix+"/")
			}
			if i := strings.Index(rest, "/"); i > 0 {
				subdirs[rest[:i]] = true
			} else {
				direct = append(direct, p)
			}
		}
		dirNames := make([]string, 0, len(subdirs))
		for d := range subdirs {
			dirNames = append(dirNames, d)
		}
		sort.Strings(dirNames)
		for _, name := range dirNames {
			full := name
			if prefix != "" {
				full = prefix + "/" + name
			}
			rows = append(rows, treeRow{
				kind: tnDir, depth: depth,
				dirPath: full, name: name, fullPath: full, fileIdx: -1,
			})
			if expanded(full) {
				walkDir(full, depth+1)
			}
		}
		for _, p := range direct {
			base := p
			if i := strings.LastIndex(p, "/"); i >= 0 {
				base = p[i+1:]
			}
			rows = append(rows, treeRow{
				kind: tnFile, depth: depth,
				dirPath: prefix, name: base, fullPath: p, fileIdx: -1,
			})
		}
	}
	walkDir("", 0)
	return rows
}

// renderTreeRow formats one treeRow for sidebar display. Changes rows
// append the per-file/per-folder read/total counts.
func (m *model) renderTreeRow(row treeRow, sect section) string {
	indent := strings.Repeat("  ", row.depth)
	switch row.kind {
	case tnDir:
		marker := "▸"
		var expanded bool
		switch sect {
		case sectionChanges:
			expanded = m.isChangesDirExpanded(row.fullPath)
		case sectionTree:
			expanded = m.fileTreeExpanded[row.fullPath]
		}
		if expanded {
			marker = "▾"
		}
		if sect == sectionChanges {
			r, t := m.dirLineCounts(row.fullPath)
			return fmt.Sprintf("%s%s %s/  %d/%d", indent, marker, row.name, r, t)
		}
		return fmt.Sprintf("%s%s %s/", indent, marker, row.name)
	case tnFile:
		if sect == sectionChanges && row.fileIdx >= 0 {
			r, t := m.fileLineCounts(row.fileIdx)
			return fmt.Sprintf("%s  %s  %d/%d", indent, row.name, r, t)
		}
		// Tree section: mark files the reviewer has already opened
		// for file-review so they can see what they've visited.
		if sect == sectionTree && m.isFileReviewed(row.fullPath) {
			return fmt.Sprintf("%s  %s", indent, styleRead.Render(row.name+" ✓"))
		}
		return fmt.Sprintf("%s  %s", indent, row.name)
	}
	return ""
}

// expandOrStepIn implements the right-arrow tree navigation:
//   * folder + collapsed → expand it
//   * folder + expanded  → move cursor to the first child row
//   * file              → drill in (same as Enter/l)
func (m *model) expandOrStepIn() {
	switch m.sect {
	case sectionChanges:
		row := m.currentChangesRow()
		if row == nil {
			return
		}
		if row.kind == tnFile {
			m.openSelectedItem()
			return
		}
		if !m.isChangesDirExpanded(row.fullPath) {
			m.changesExpanded[row.fullPath] = true
			m.refreshViewport()
			return
		}
		// Already expanded — step into the first child row, which
		// is the next entry in changesRows (the rebuild keeps the
		// folder at sectIdx, with its children immediately after).
		m.changesRows = m.buildChangesRows()
		if m.sectIdx[sectionChanges]+1 < len(m.changesRows) {
			m.sectIdx[sectionChanges]++
		}
		m.onTreeSelectionChanged()
	case sectionTree:
		row := m.currentFileTreeRow()
		if row == nil {
			return
		}
		if row.kind == tnFile {
			m.openSelectedItem()
			return
		}
		if !m.fileTreeExpanded[row.fullPath] {
			m.fileTreeExpanded[row.fullPath] = true
			m.refreshViewport()
			return
		}
		m.fileTreeRows = m.buildFileTreeRows()
		if m.sectIdx[sectionTree]+1 < len(m.fileTreeRows) {
			m.sectIdx[sectionTree]++
		}
		m.onTreeSelectionChanged()
	}
}

// collapseOrStepUp implements the left-arrow tree navigation:
//   * expanded folder → collapse it
//   * file or collapsed folder → jump up to the nearest parent
//     folder row (scan backwards for a tnDir with smaller depth)
func (m *model) collapseOrStepUp() {
	switch m.sect {
	case sectionChanges:
		row := m.currentChangesRow()
		if row == nil {
			return
		}
		if row.kind == tnDir && m.isChangesDirExpanded(row.fullPath) {
			m.changesExpanded[row.fullPath] = false
			m.refreshViewport()
			return
		}
		curIdx := m.sectIdx[sectionChanges]
		curDepth := row.depth
		for i := curIdx - 1; i >= 0; i-- {
			r := m.changesRows[i]
			if r.kind == tnDir && r.depth < curDepth {
				m.sectIdx[sectionChanges] = i
				m.onTreeSelectionChanged()
				return
			}
		}
	case sectionTree:
		row := m.currentFileTreeRow()
		if row == nil {
			return
		}
		if row.kind == tnDir && m.fileTreeExpanded[row.fullPath] {
			m.fileTreeExpanded[row.fullPath] = false
			m.refreshViewport()
			return
		}
		curIdx := m.sectIdx[sectionTree]
		curDepth := row.depth
		for i := curIdx - 1; i >= 0; i-- {
			r := m.fileTreeRows[i]
			if r.kind == tnDir && r.depth < curDepth {
				m.sectIdx[sectionTree] = i
				m.onTreeSelectionChanged()
				return
			}
		}
	}
}

// nextVisibleSection cycles through the visible (sidebar-rendered)
// sections in `direction` (+1 / -1). sectionFileReview is hidden
// from the cycle since its content lives as highlights in Tree.
func nextVisibleSection(cur section, direction int) section {
	visible := []section{
		sectionSources, sectionVerdicts, sectionIssues,
		sectionChanges, sectionCommits, sectionTree,
	}
	idx := -1
	for i, s := range visible {
		if s == cur {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
	}
	next := (idx + direction + len(visible)) % len(visible)
	return visible[next]
}

// isFileReviewed reports whether every line of the given file has
// been seen long enough to be marked read. Only files the reviewer
// has actually opened in modeFile (so fileLineTotals[path] is
// populated) can earn the ✓.
func (m *model) isFileReviewed(path string) bool {
	total, ok := m.fileLineTotals[path]
	if !ok || total == 0 {
		return false
	}
	return len(m.fileLineRead[path]) >= total
}

// isChangesDirExpanded reports whether the given directory is expanded
// in the Changes sidebar — respects manual overrides, otherwise
// expanded if any file under it has unread.
func (m *model) isChangesDirExpanded(dir string) bool {
	if v, ok := m.changesExpanded[dir]; ok {
		return v
	}
	for fi, f := range m.files {
		if strings.HasPrefix(f.Path, "commit:") {
			continue
		}
		if !pathInDir(f.Path, dir) {
			continue
		}
		if m.fileHasUnread(fi) {
			return true
		}
	}
	return false
}

// dirLineCounts sums fileLineCounts across every real file under dir.
func (m *model) dirLineCounts(dir string) (read, total int) {
	for fi, f := range m.files {
		if strings.HasPrefix(f.Path, "commit:") {
			continue
		}
		if !pathInDir(f.Path, dir) {
			continue
		}
		r, t := m.fileLineCounts(fi)
		read += r
		total += t
	}
	return
}

// updateTree dispatches key input while the reviewer is in section mode.
func (m *model) updateTree(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	// In section mode, 's' on Changes/Tree means "skip", not "save"
	// (save is autosaved anyway). Pre-empt before the global handler.
	if key.String() == "s" && (m.sect == sectionChanges || m.sect == sectionTree) {
		m.skipFromSidebar()
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
		m.sect = nextVisibleSection(m.sect, +1)
	case "shift+tab":
		m.sect = nextVisibleSection(m.sect, -1)
	case " ", "space":
		// On Commits/Verdicts Space walks via spaceWalk; on Changes it
		// drills into the first unread file (so folder rows fall through
		// to the file walk, not a folder toggle); on Tree it drills
		// into the highlighted file.
		switch m.sect {
		case sectionCommits, sectionVerdicts:
			m.spaceWalk()
		case sectionChanges:
			fi := -1
			if row := m.currentChangesRow(); row != nil && row.kind == tnFile {
				fi = row.fileIdx
			} else {
				for i := range m.files {
					if !strings.HasPrefix(m.files[i].Path, "commit:") && m.fileHasUnread(i) {
						fi = i
						break
					}
				}
			}
			if fi < 0 {
				m.spaceWalk()
				return m, nil
			}
			m.fileIdx = fi
			m.hunkIdx = 0
			m.lineCursor = 0
			m.atEOF = false
			if h := m.currentHunk(); h != nil {
				m.lineCursor = m.firstNonDelete(h, 0, +1)
			}
			m.mode = modeDiff
			m.refreshViewport()
		default:
			m.openSelectedItem()
		}
	case "alt+space":
		m.skipWalk()
	case "i":
		// Add an entry in the current section:
		//   Verdicts → verdict editor (new audit-log entry on Alt+Enter)
		//   anywhere else → issue editor (new general issue)
		if m.sect == sectionVerdicts {
			m.openVerdictEditor()
			return m, nil
		}
		m.openEdit(editIssue, "", -1, -1, "")
		return m, m.title.Focus()
	case "e":
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
			// Submitting creates a fresh audit-log entry (verdicts are
			// append-only per the spec).
			m.openVerdictEditor()
			return m, nil
		default:
			m.status = "no editable item under cursor"
			return m, nil
		}
	case "right":
		// Right-arrow in the folder-tree sections has its own
		// behaviour: on a collapsed folder it expands; on an
		// expanded folder it steps into the first child row;
		// on a file row it drills in just like Enter/l.
		if m.sect == sectionChanges || m.sect == sectionTree {
			m.expandOrStepIn()
			return m, nil
		}
		m.openSelectedItem()
	case "left":
		// Left-arrow on a folder collapses it; otherwise (file
		// row or already-collapsed folder) the cursor jumps up
		// to the parent folder row.
		if m.sect == sectionChanges || m.sect == sectionTree {
			m.collapseOrStepUp()
			return m, nil
		}
	case "l", "enter":
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
	} else {
		// Wrap-skip to the next visible section that has items.
		s := m.sect
		for i := 0; i < 8; i++ {
			s = nextVisibleSection(s, +1)
			if s == m.sect {
				break
			}
			if len(m.sectionItems(s)) > 0 {
				m.sect = s
				m.sectIdx[s] = 0
				break
			}
		}
	}
	m.onTreeSelectionChanged()
}

func (m *model) treePrev() {
	if m.sectIdx[m.sect] > 0 {
		m.sectIdx[m.sect]--
	} else {
		s := m.sect
		for i := 0; i < 8; i++ {
			s = nextVisibleSection(s, -1)
			if s == m.sect {
				break
			}
			items := m.sectionItems(s)
			if len(items) > 0 {
				m.sect = s
				m.sectIdx[s] = len(items) - 1
				break
			}
		}
	}
	m.onTreeSelectionChanged()
}

// onTreeSelectionChanged peeks the right pane based on the selected item.
func (m *model) onTreeSelectionChanged() {
	switch m.sect {
	case sectionChanges:
		if row := m.currentChangesRow(); row != nil && row.kind == tnFile && row.fileIdx >= 0 {
			m.fileIdx = row.fileIdx
			m.hunkIdx = 0
			m.atEOF = false
		}
	case sectionFileReview:
		idx := m.sectIdx[m.sect]
		frs := m.sess.FileReviews()
		if idx < len(frs) {
			m.filePath = frs[idx].Path
			m.fileLines, _ = gitFileLines(m.sess.Scope.TipSHA, m.filePath)
		}
	case sectionTree:
		if row := m.currentFileTreeRow(); row != nil && row.kind == tnFile {
			m.filePath = row.fullPath
			m.fileLines, _ = gitFileLines(m.sess.Scope.TipSHA, m.filePath)
		}
	case sectionCommits, sectionIssues, sectionSources, sectionVerdicts:
		// no peek-side-effect
	}
	m.refreshViewport()
}

// skipFromSidebar marks every reviewable line in the file (or every
// file under the folder) under the sidebar cursor as Skipped. Bound to
// 's' in section mode on Changes / Tree. Lines already marked Read
// keep their read state.
func (m *model) skipFromSidebar() {
	var paths []string
	switch m.sect {
	case sectionChanges:
		row := m.currentChangesRow()
		if row == nil {
			return
		}
		if row.kind == tnFile {
			paths = []string{row.fullPath}
		} else {
			for _, f := range m.files {
				if !strings.HasPrefix(f.Path, "commit:") && pathInDir(f.Path, row.fullPath) {
					paths = append(paths, f.Path)
				}
			}
		}
	case sectionTree:
		row := m.currentFileTreeRow()
		if row == nil {
			return
		}
		target := row.fullPath
		for _, f := range m.files {
			if !strings.HasPrefix(f.Path, "commit:") {
				if row.kind == tnFile && f.Path == target {
					paths = append(paths, f.Path)
				} else if row.kind == tnDir && pathInDir(f.Path, target) {
					paths = append(paths, f.Path)
				}
			}
		}
		if len(paths) == 0 {
			m.status = "no diff content under " + target
			return
		}
	}
	skipped := 0
	for _, p := range paths {
		fi := m.findFileIdx(p)
		if fi < 0 {
			continue
		}
		f := &m.files[fi]
		for hi, h := range f.Hunks {
			for li, ln := range h.Lines {
				if ln.Kind != review.LineAdd && ln.Kind != review.LineDelete {
					continue
				}
				lk := lineKey{fileIdx: fi, hunkIdx: hi, lineIdx: li}
				if m.lineRead[lk] || m.lineSkipped[lk] {
					continue
				}
				m.lineSkipped[lk] = true
				skipped++
			}
		}
	}
	if skipped > 0 {
		m.scheduleAutoSave()
		m.status = fmt.Sprintf("skipped %d line(s) across %d file(s)", skipped, len(paths))
	} else {
		m.status = "nothing to skip"
	}
	m.refreshViewport()
}

// findFileIdx returns the m.files index for a path, or -1 if absent.
func (m *model) findFileIdx(path string) int {
	for i, f := range m.files {
		if f.Path == path {
			return i
		}
	}
	return -1
}

// changesRowForFile returns the index in changesRows of the file row
// matching fileIdx (or -1).
func (m *model) changesRowForFile(fileIdx int) int {
	if len(m.changesRows) == 0 {
		m.changesRows = m.buildChangesRows()
	}
	for i, row := range m.changesRows {
		if row.kind == tnFile && row.fileIdx == fileIdx {
			return i
		}
	}
	return -1
}

// changesFileFromRow returns the m.files index for the row, or -1 if
// the row isn't a file row.
func (m *model) changesFileFromRow(rowIdx int) int {
	if len(m.changesRows) == 0 {
		m.changesRows = m.buildChangesRows()
	}
	if rowIdx < 0 || rowIdx >= len(m.changesRows) {
		return -1
	}
	row := m.changesRows[rowIdx]
	if row.kind != tnFile {
		return -1
	}
	return row.fileIdx
}

// currentChangesRow returns the treeRow under the Changes sidebar
// cursor, or nil if the cache isn't built yet.
func (m *model) currentChangesRow() *treeRow {
	if len(m.changesRows) == 0 {
		m.changesRows = m.buildChangesRows()
	}
	idx := m.sectIdx[sectionChanges]
	if idx < 0 || idx >= len(m.changesRows) {
		return nil
	}
	return &m.changesRows[idx]
}

// currentFileTreeRow returns the treeRow under the Tree sidebar cursor.
func (m *model) currentFileTreeRow() *treeRow {
	if len(m.fileTreeRows) == 0 {
		m.fileTreeRows = m.buildFileTreeRows()
	}
	idx := m.sectIdx[sectionTree]
	if idx < 0 || idx >= len(m.fileTreeRows) {
		return nil
	}
	return &m.fileTreeRows[idx]
}

// openSelectedItem performs the natural drill-in action for the row
// under the section cursor.
func (m *model) openSelectedItem() {
	switch m.sect {
	case sectionSources, sectionVerdicts:
		// peek-only; drilling in just keeps the right pane content
	case sectionChanges:
		row := m.currentChangesRow()
		if row == nil {
			return
		}
		switch row.kind {
		case tnDir:
			m.changesExpanded[row.fullPath] = !m.isChangesDirExpanded(row.fullPath)
			m.refreshViewport()
		case tnFile:
			m.fileIdx = row.fileIdx
			m.hunkIdx = 0
			m.lineCursor = 0
			m.atEOF = false
			if h := m.currentHunk(); h != nil {
				m.lineCursor = m.firstNonDelete(h, 0, +1)
			}
			m.mode = modeDiff
			m.refreshViewport()
		}
	case sectionFileReview:
		idx := m.sectIdx[m.sect]
		frs := m.sess.FileReviews()
		if idx < len(frs) {
			m.enterFileReview(frs[idx].Path)
		}
	case sectionTree:
		row := m.currentFileTreeRow()
		if row == nil {
			return
		}
		switch row.kind {
		case tnDir:
			m.fileTreeExpanded[row.fullPath] = !m.fileTreeExpanded[row.fullPath]
			m.refreshViewport()
		case tnFile:
			m.enterFileReview(row.fullPath)
		}
	case sectionCommits:
		// Drill into the commit's first virtual file. Each commit's
		// patches are appended to m.files under `commit:<short>:<path>`
		// at session load — we find the first one and switch to modeDiff.
		idx := m.sectIdx[m.sect]
		if idx < 0 || idx >= len(m.sess.Scope.Commits) {
			return
		}
		short := m.sess.Scope.Commits[idx].Short
		want := "commit:" + short
		for fi, f := range m.files {
			if f.Path != want {
				continue
			}
			m.fileIdx = fi
			m.hunkIdx = 0
			m.lineCursor = 0
			m.atEOF = false
			if h := m.currentHunk(); h != nil {
				m.lineCursor = m.firstNonDelete(h, 0, +1)
			}
			m.mode = modeDiff
			m.refreshViewport()
			return
		}
		m.status = "no diff content for commit " + short
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
