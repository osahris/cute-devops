// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"strings"

	"gitflower/review"
)

// eventCounts is the per-file (or per-dir) tally of anchored reviewer
// events: comments, questions, likes, dislikes. Drives the inline
// counter suffix in the Changes sidebar so a glance tells you which
// files have feedback waiting on them.
type eventCounts struct {
	comments  int
	questions int
	likes     int
	dislikes  int
}

func (c eventCounts) any() bool {
	return c.comments+c.questions+c.likes+c.dislikes > 0
}

// suffix formats the non-zero counters as "💬 N ❓ N 👍 N 👎 N",
// emitting only the lanes that have at least one event. Returns ""
// when there are no events.
func (c eventCounts) suffix() string {
	if !c.any() {
		return ""
	}
	var parts []string
	if c.comments > 0 {
		parts = append(parts, fmt.Sprintf("💬 %d", c.comments))
	}
	if c.questions > 0 {
		parts = append(parts, fmt.Sprintf("❓ %d", c.questions))
	}
	if c.likes > 0 {
		parts = append(parts, fmt.Sprintf("👍 %d", c.likes))
	}
	if c.dislikes > 0 {
		parts = append(parts, fmt.Sprintf("👎 %d", c.dislikes))
	}
	return strings.Join(parts, " ")
}

// fileEventCounts counts events anchored to a single file. An event's
// anchor is either `path` (rare) or `path:<rest>`; both shapes count.
func (m *model) fileEventCounts(path string) eventCounts {
	var c eventCounts
	prefix := path + ":"
	matches := func(a review.Anchor) bool {
		s := string(a)
		return s == path || strings.HasPrefix(s, prefix)
	}
	for _, cm := range m.sess.Comments() {
		if !matches(cm.Anchor) {
			continue
		}
		if cm.Kind == review.KindQuestion {
			c.questions++
		} else {
			c.comments++
		}
	}
	for _, a := range m.sess.MarkerAnchors() {
		if !matches(a) {
			continue
		}
		switch m.sess.Marker(a) {
		case review.MarkerGood:
			c.likes++
		case review.MarkerBad:
			c.dislikes++
		}
	}
	return c
}

// dirEventCounts aggregates file counts for every changed file under
// `dir` (any depth). Skips commit virtual files since they aren't
// addressable by anchor.
func (m *model) dirEventCounts(dir string) eventCounts {
	var c eventCounts
	for _, f := range m.files {
		if strings.HasPrefix(f.Path, "commit:") {
			continue
		}
		if !pathInDir(f.Path, dir) {
			continue
		}
		fc := m.fileEventCounts(f.Path)
		c.comments += fc.comments
		c.questions += fc.questions
		c.likes += fc.likes
		c.dislikes += fc.dislikes
	}
	return c
}
