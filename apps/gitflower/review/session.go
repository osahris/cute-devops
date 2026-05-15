// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

// Package review is the workflow core for code reviews. It is interface-
// agnostic — a TUI, a web handler, or a CLI scaffold-only mode all
// manipulate a *ReviewSession via the same methods, and persist by Save().
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

// VerdictEvent is one entry in the # Review > ## Verdicts audit log.
type VerdictEvent struct {
	State   Verdict // open | requested-changes | approved | denied
	Author  string  // "Name <email>"
	Date    string  // RFC3339
	Summary string  // markdown summary body
}

// FileReview captures a file inspected in file-review mode at a particular
// tip SHA. Lines holds only the visited ranges (line-number → content).
type FileReview struct {
	Path   string
	TipSHA string
	Lines  []FileLine // sorted ascending by Number
}

// FileLine is one numbered line in a FileReview.
type FileLine struct {
	Number  int    // 1-based line number in the file at TipSHA
	Content string // raw line content
}

// ReviewSession is an in-progress (or saved) review.
type ReviewSession struct {
	Path     string
	Reviewer string
	Date     string
	Verdict  Verdict
	Summary  string // free-form verdict explanation (markdown)
	Scope    Scope

	read     map[Anchor]bool   // hunks the reviewer has finished reading
	markers  map[Anchor]Marker // good/bad reactions → emitted as Like/Dislike events
	comments []Comment         // includes both KindComment and KindQuestion
	issues   []Issue           // free-form issues added during review

	// Verdicts is the audit log. The most recent entry is the current state.
	// Backwards-compat: if empty, fall back to the single Verdict/Summary
	// fields above. Render() always emits one Verdict event from the
	// current canonical state.
	verdicts []VerdictEvent

	// FileReviews holds file-mode visits. Empty until the reviewer enters
	// file-review mode on at least one file.
	fileReviews []FileReview

	dirty bool
}

// Verdicts returns the verdict audit log.
func (s *ReviewSession) Verdicts() []VerdictEvent { return s.verdicts }

// AddVerdict appends to the audit log.
func (s *ReviewSession) AddVerdict(v VerdictEvent) {
	if v.Author == "" {
		v.Author = s.Reviewer
	}
	if v.Date == "" {
		v.Date = time.Now().UTC().Format(time.RFC3339)
	}
	s.verdicts = append(s.verdicts, v)
	s.Verdict = v.State
	s.Summary = v.Summary
	s.dirty = true
}

// FileReviews returns the file-review list.
func (s *ReviewSession) FileReviews() []FileReview { return s.fileReviews }

// AddFileReview appends a FileReview entry.
func (s *ReviewSession) AddFileReview(fr FileReview) {
	s.fileReviews = append(s.fileReviews, fr)
	s.dirty = true
}

// New creates a fresh ReviewSession.
func New(scope Scope, reviewer, path string) *ReviewSession {
	return &ReviewSession{
		Path:     path,
		Reviewer: reviewer,
		Verdict:  VerdictOpen,
		Scope:    scope,
		read:     map[Anchor]bool{},
		markers:  map[Anchor]Marker{},
	}
}

func (s *ReviewSession) Dirty() bool { return s.dirty }

// Read state.

func (s *ReviewSession) IsRead(a Anchor) bool { return s.read[a] }

func (s *ReviewSession) MarkRead(a Anchor) {
	if !s.read[a] {
		s.read[a] = true
		s.dirty = true
	}
}

func (s *ReviewSession) MarkUnread(a Anchor) {
	if s.read[a] {
		delete(s.read, a)
		s.dirty = true
	}
}

func (s *ReviewSession) ToggleRead(a Anchor) {
	if s.read[a] {
		s.MarkUnread(a)
	} else {
		s.MarkRead(a)
	}
}

// ReadAnchors returns the read-marked anchors in deterministic order.
func (s *ReviewSession) ReadAnchors() []Anchor {
	out := make([]Anchor, 0, len(s.read))
	for a := range s.read {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Markers.

func (s *ReviewSession) Marker(a Anchor) Marker { return s.markers[a] }

func (s *ReviewSession) SetMarker(a Anchor, m Marker) {
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

func (s *ReviewSession) MarkerAnchors() []Anchor {
	out := make([]Anchor, 0, len(s.markers))
	for a := range s.markers {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Comments and questions.

func (s *ReviewSession) AddComment(c Comment) {
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
func (s *ReviewSession) Comments() []Comment { return s.comments }

// UpdateComment replaces the comment at idx (0-based) with c, preserving
// the original Author/Date. Returns false if idx is out of range.
func (s *ReviewSession) UpdateComment(idx int, text string) bool {
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
func (s *ReviewSession) CommentIndexAt(a Anchor) int {
	for i, c := range s.comments {
		if c.Anchor == a {
			return i
		}
	}
	return -1
}

// Issues returns all free-form issues added during the review.
func (s *ReviewSession) Issues() []Issue { return s.issues }

// AddIssue appends an issue. Author and Date are filled in if empty.
func (s *ReviewSession) AddIssue(it Issue) {
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
func (s *ReviewSession) UpdateIssue(idx int, title, body string) bool {
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

func (s *ReviewSession) CommentsOn(path string) []Comment {
	var out []Comment
	for _, c := range s.comments {
		if strings.HasPrefix(string(c.Anchor), path+":") || string(c.Anchor) == path {
			out = append(out, c)
		}
	}
	return out
}

// Verdict and summary.

func (s *ReviewSession) SetVerdict(v Verdict) {
	if s.Verdict != v {
		s.Verdict = v
		s.dirty = true
	}
}

func (s *ReviewSession) SetSummary(text string) {
	if s.Summary != text {
		s.Summary = text
		s.dirty = true
	}
}

// Save serialises the session.
func (s *ReviewSession) Save() error {
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

func (s *ReviewSession) Exists() bool {
	_, err := os.Stat(s.Path)
	return err == nil
}

// DefaultPath returns the conventional review-file path per the spec:
//
//	reviews/<to-slug>-<to-short>-from-<from-slug>-<from-short>.review
//
// where slugify replaces `/` and other unsafe chars with `-`.
func DefaultPath(repoRoot string, scope *Scope) string {
	toSlug := slugify(scope.Branch)
	fromSlug := slugify(scope.Base)
	toShort := shortSHA(scope.TipSHA)
	fromShort := shortSHA(scope.BaseSHA)
	name := toSlug + "-" + toShort + "-from-" + fromSlug + "-" + fromShort + ".review"
	return filepath.Join(repoRoot, "reviews", name)
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
