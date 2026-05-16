// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"gitflower/review"
)

// Per-line read/skip tracking lives in m.lineRead / m.lineSkipped keyed
// by (fileIdx, hunkIdx, lineIdx) — the TUI's natural coordinate. The
// session persists the same state keyed by review.Anchor (`path:N` for
// a single line, `path:start,count` for a whole hunk). The helpers in
// this file move state across that gap so the rendered note carries
// what the TUI shows and the next session restores it.

// lineAnchor returns the line anchor for a lineKey on a real file in
// m.files. Returns ("", false) when the key points to a delete-side
// line (no new-side number), a commit virtual file (`commit:<short>`),
// or an out-of-range index.
func (m *model) lineAnchor(lk lineKey) (review.Anchor, bool) {
	if lk.fileIdx < 0 || lk.fileIdx >= len(m.files) {
		return "", false
	}
	f := m.files[lk.fileIdx]
	if strings.HasPrefix(f.Path, "commit:") {
		return "", false
	}
	if lk.hunkIdx < 0 || lk.hunkIdx >= len(f.Hunks) {
		return "", false
	}
	h := f.Hunks[lk.hunkIdx]
	newLine := h.NewStart
	for i, ln := range h.Lines {
		if i == lk.lineIdx {
			if ln.Kind == review.LineDelete {
				return "", false
			}
			return review.Anchor(fmt.Sprintf("%s:%d", f.Path, newLine)), true
		}
		if ln.Kind != review.LineDelete {
			newLine++
		}
	}
	return "", false
}

// keysForAnchor maps a stored Anchor back to the lineKeys it covers.
//   - `path:N`             → the one `+`/` ` line at new-side number N
//   - `path:start,count`   → every `+`/` ` line in the matching hunk
//
// Unknown paths (e.g. files that were renamed away) return nil so
// stale anchors don't crash the hydration step.
func (m *model) keysForAnchor(a review.Anchor) []lineKey {
	s := string(a)
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return nil
	}
	path, rest := s[:i], s[i+1:]

	fileIdx := -1
	for fi, f := range m.files {
		if f.Path == path {
			fileIdx = fi
			break
		}
	}
	if fileIdx < 0 {
		return nil
	}

	// Hunk anchor: "<start>,<count>" — collect every non-delete line
	// in the matching hunk.
	if j := strings.Index(rest, ","); j > 0 {
		start, err1 := strconv.Atoi(rest[:j])
		count, err2 := strconv.Atoi(rest[j+1:])
		if err1 != nil || err2 != nil {
			return nil
		}
		var keys []lineKey
		for hi, h := range m.files[fileIdx].Hunks {
			if h.NewStart != start || h.NewLines != count {
				continue
			}
			for li, ln := range h.Lines {
				if ln.Kind == review.LineDelete {
					continue
				}
				keys = append(keys, lineKey{fileIdx: fileIdx, hunkIdx: hi, lineIdx: li})
			}
		}
		return keys
	}

	// Line anchor: single N (possibly "N-M" range; we use N for the
	// resolved position the way Render emits it).
	endStr := rest
	if j := strings.Index(rest, "-"); j > 0 {
		endStr = rest[j+1:]
	}
	n, err := strconv.Atoi(endStr)
	if err != nil {
		return nil
	}
	for hi, h := range m.files[fileIdx].Hunks {
		if n < h.NewStart || n >= h.NewStart+h.NewLines {
			continue
		}
		newLine := h.NewStart
		for li, ln := range h.Lines {
			if ln.Kind == review.LineDelete {
				continue
			}
			if newLine == n {
				return []lineKey{{fileIdx: fileIdx, hunkIdx: hi, lineIdx: li}}
			}
			newLine++
		}
	}
	return nil
}

// hydrateFromSession populates m.lineRead and m.lineSkipped from the
// session's persisted anchors. Called once after newModel has parsed
// the diff so the TUI starts with whatever the previous run saved.
func (m *model) hydrateFromSession() {
	for _, a := range m.sess.ReadAnchors() {
		for _, lk := range m.keysForAnchor(a) {
			m.lineRead[lk] = true
		}
	}
	for _, a := range m.sess.SkippedAnchors() {
		for _, lk := range m.keysForAnchor(a) {
			m.lineSkipped[lk] = true
		}
	}
}

// markLineRead pushes a single-line read into both the TUI map and
// the session so the next Save persists it. Mirrors the existing
// "set lineRead[lk] = true" pattern; call this everywhere lineRead
// flips on.
func (m *model) markLineRead(lk lineKey) {
	if m.lineRead[lk] {
		return
	}
	m.lineRead[lk] = true
	if a, ok := m.lineAnchor(lk); ok {
		m.sess.MarkRead(a)
	}
}

// unmarkLineRead is the inverse — flip the TUI state off and clear
// the matching anchor from the session.
func (m *model) unmarkLineRead(lk lineKey) {
	if !m.lineRead[lk] {
		return
	}
	delete(m.lineRead, lk)
	if a, ok := m.lineAnchor(lk); ok {
		m.sess.MarkUnread(a)
	}
}

// markLineSkipped / unmarkLineSkipped mirror the read variants for
// the skip lane.
func (m *model) markLineSkipped(lk lineKey) {
	if m.lineSkipped[lk] {
		return
	}
	m.lineSkipped[lk] = true
	if a, ok := m.lineAnchor(lk); ok {
		m.sess.MarkSkipped(a)
	}
}

func (m *model) unmarkLineSkipped(lk lineKey) {
	if !m.lineSkipped[lk] {
		return
	}
	delete(m.lineSkipped, lk)
	if a, ok := m.lineAnchor(lk); ok {
		m.sess.UnmarkSkipped(a)
	}
}
