// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Package review is the workflow core for code reviews. It is interface-
// agnostic — a TUI, a web handler, or a CLI scaffold-only mode all
// manipulate a *Session via the same methods, and persist by Save().
package review

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Verdict is the reviewer's overall judgement.
type Verdict string

const (
	VerdictOpen     Verdict = "open"
	VerdictChanges  Verdict = "requested-changes"
	VerdictApproved Verdict = "approved"
	VerdictDenied   Verdict = "denied"
)

// AllVerdicts lists verdicts in cycle order.
var AllVerdicts = []Verdict{VerdictOpen, VerdictChanges, VerdictApproved, VerdictDenied}

func (v Verdict) Next() Verdict {
	for i, w := range AllVerdicts {
		if w == v {
			return AllVerdicts[(i+1)%len(AllVerdicts)]
		}
	}
	return VerdictOpen
}

// Marker is a quick reaction on a hunk.
type Marker string

const (
	MarkerNone Marker = ""
	MarkerGood Marker = "good"
	MarkerBad  Marker = "bad"
)

// Anchor identifies a hunk (or smaller selection) for read tracking, markers
// and comments. Format: "path:newStart,newLines" for a hunk.
type Anchor string

// HunkAnchor builds an Anchor for the new-side range of a hunk.
func HunkAnchor(path string, newStart, newLines int) Anchor {
	return Anchor(fmt.Sprintf("%s:%d,%d", path, newStart, newLines))
}

// Kind discriminates regular comments from questions.
type Kind string

const (
	KindComment  Kind = "" // default: a review comment
	KindQuestion Kind = "question"
)

// Comment is one threaded comment on an anchor.
type Comment struct {
	Anchor  Anchor // path:range
	Author  string // reviewer email
	Date    string // RFC3339
	Text    string // markdown body
	Snippet string // optional captured diff snippet
	Kind    Kind   // comment | question
}

// Issue is a free-form review issue not tied to a specific anchor.
// (Added via the `i` key in tree mode; can later be promoted to a
// standalone issues/*.md file when the review is processed.)
type Issue struct {
	Title  string
	Body   string
	Author string
	Date   string
}

// Session is an in-progress (or saved) review.
type Session struct {
	Path     string
	Reviewer string
	Date     string
	Verdict  Verdict
	Summary  string // free-form verdict explanation (markdown)
	Scope    Scope

	read     map[Anchor]bool   // hunks the reviewer has finished reading
	markers  map[Anchor]Marker // good/bad reactions
	comments []Comment         // includes both KindComment and KindQuestion
	issues   []Issue           // free-form issues added during review

	dirty bool
}

// New creates a fresh Session.
func New(scope Scope, reviewer, path string) *Session {
	return &Session{
		Path:     path,
		Reviewer: reviewer,
		Verdict:  VerdictOpen,
		Scope:    scope,
		read:     map[Anchor]bool{},
		markers:  map[Anchor]Marker{},
	}
}

func (s *Session) Dirty() bool { return s.dirty }

// Read state.

func (s *Session) IsRead(a Anchor) bool { return s.read[a] }

func (s *Session) MarkRead(a Anchor) {
	if !s.read[a] {
		s.read[a] = true
		s.dirty = true
	}
}

func (s *Session) MarkUnread(a Anchor) {
	if s.read[a] {
		delete(s.read, a)
		s.dirty = true
	}
}

func (s *Session) ToggleRead(a Anchor) {
	if s.read[a] {
		s.MarkUnread(a)
	} else {
		s.MarkRead(a)
	}
}

// ReadAnchors returns the read-marked anchors in deterministic order.
func (s *Session) ReadAnchors() []Anchor {
	out := make([]Anchor, 0, len(s.read))
	for a := range s.read {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Markers.

func (s *Session) Marker(a Anchor) Marker { return s.markers[a] }

func (s *Session) SetMarker(a Anchor, m Marker) {
	cur := s.markers[a]
	if cur == m {
		// Toggling the same marker clears it.
		delete(s.markers, a)
		s.dirty = true
		return
	}
	if m == MarkerNone {
		delete(s.markers, a)
	} else {
		s.markers[a] = m
	}
	s.dirty = true
}

func (s *Session) MarkerAnchors() []Anchor {
	out := make([]Anchor, 0, len(s.markers))
	for a := range s.markers {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Comments and questions.

func (s *Session) AddComment(c Comment) {
	if c.Author == "" {
		c.Author = s.Reviewer
	}
	if c.Date == "" {
		c.Date = time.Now().UTC().Format(time.RFC3339)
	}
	s.comments = append(s.comments, c)
	s.dirty = true
}

// Comments returns all comments (questions included). Filter by Kind if needed.
func (s *Session) Comments() []Comment { return s.comments }

// UpdateComment replaces the comment at idx (0-based) with c, preserving
// the original Author/Date. Returns false if idx is out of range.
func (s *Session) UpdateComment(idx int, text string) bool {
	if idx < 0 || idx >= len(s.comments) {
		return false
	}
	if s.comments[idx].Text == text {
		return false
	}
	s.comments[idx].Text = text
	s.dirty = true
	return true
}

// CommentIndexAt returns the index of the first comment matching anchor
// (exact match), or -1 if none. Used by edit-comment flow.
func (s *Session) CommentIndexAt(a Anchor) int {
	for i, c := range s.comments {
		if c.Anchor == a {
			return i
		}
	}
	return -1
}

// Issues returns all free-form issues added during the review.
func (s *Session) Issues() []Issue { return s.issues }

// AddIssue appends an issue. Author and Date are filled in if empty.
func (s *Session) AddIssue(it Issue) {
	if it.Author == "" {
		it.Author = s.Reviewer
	}
	if it.Date == "" {
		it.Date = time.Now().UTC().Format(time.RFC3339)
	}
	s.issues = append(s.issues, it)
	s.dirty = true
}

// UpdateIssue replaces the issue at idx with the given title/body, preserving
// Author/Date.
func (s *Session) UpdateIssue(idx int, title, body string) bool {
	if idx < 0 || idx >= len(s.issues) {
		return false
	}
	if s.issues[idx].Title == title && s.issues[idx].Body == body {
		return false
	}
	s.issues[idx].Title = title
	s.issues[idx].Body = body
	s.dirty = true
	return true
}

func (s *Session) CommentsOn(path string) []Comment {
	var out []Comment
	for _, c := range s.comments {
		if strings.HasPrefix(string(c.Anchor), path+":") || string(c.Anchor) == path {
			out = append(out, c)
		}
	}
	return out
}

// Verdict and summary.

func (s *Session) SetVerdict(v Verdict) {
	if s.Verdict != v {
		s.Verdict = v
		s.dirty = true
	}
}

func (s *Session) SetSummary(text string) {
	if s.Summary != text {
		s.Summary = text
		s.dirty = true
	}
}

// Save serialises the session.
func (s *Session) Save() error {
	if s.Date == "" {
		s.Date = time.Now().UTC().Format(time.RFC3339)
	}
	body := Render(s)
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(s.Path, []byte(body), 0o644); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

func (s *Session) Exists() bool {
	_, err := os.Stat(s.Path)
	return err == nil
}

// DefaultPath returns the conventional review-file path.
func DefaultPath(repoRoot, tipSHA, reviewer string) string {
	short := tipSHA
	if len(short) > 12 {
		short = short[:12]
	}
	rslug := slugify(strings.SplitN(reviewer, "@", 2)[0])
	return filepath.Join(repoRoot, "issues", short+"."+rslug+".review.md")
}
