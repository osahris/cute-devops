// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Load reads a `.review` file from disk and parses it into a ReviewSession.
// The returned session has Path set; call Scope and merge externally to
// populate the live diff/commit patches.
func Load(path string) (*ReviewSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := Parse(string(data))
	if err != nil {
		return nil, err
	}
	s.Path = path
	return s, nil
}

// Render serialises a ReviewSession to the `.review` file format
// (issues/dot-review.feature.md). Event-line shapes:
//
//	### <Kind>[: <param>] (From: <Name> <<email>>, <RFC3339>)
//
//	body markdown
//
// Quoted patch lines: `> <text>`. See the feature spec for the full grammar.
func Render(s *ReviewSession) string {
	var sb strings.Builder

	renderReview(&sb, s)
	renderGeneralIssues(&sb, s)
	renderChanges(&sb, s)
	renderCommits(&sb, s)
	renderFileReviewSection(&sb, s)

	return sb.String()
}

// ---------------------------------------------------------------------
// # Review
// ---------------------------------------------------------------------

func renderReview(sb *strings.Builder, s *ReviewSession) {
	sb.WriteString("# Review\n\n")
	sb.WriteString("## Sources\n\n")
	fmt.Fprintf(sb, "- From: `%s` at `%s`\n", s.Scope.Base, s.Scope.BaseSHA)
	fmt.Fprintf(sb, "- To: `%s` at `%s`\n", s.Scope.Branch, s.Scope.TipSHA)
	fmt.Fprintf(sb, "- Diff: `%s`\n", s.Scope.Diff)
	fmt.Fprintf(sb, "- Commits: %d\n", len(s.Scope.Commits))
	sb.WriteString("\n")

	sb.WriteString("## Verdicts\n\n")
	verdicts := s.verdicts
	if len(verdicts) == 0 {
		// Fall back to the canonical fields (legacy path).
		verdicts = []VerdictEvent{{
			State:   s.Verdict,
			Author:  s.Reviewer,
			Date:    s.Date,
			Summary: s.Summary,
		}}
	}
	for _, v := range verdicts {
		fmt.Fprintf(sb, "### Verdict: %s (From: %s, %s)\n\n", v.State, formatAuthor(v.Author), defaultDate(v.Date))
		if v.Summary != "" {
			sb.WriteString(shiftUserHeadingsUp(v.Summary))
			sb.WriteString("\n\n")
		}
	}
}

// ---------------------------------------------------------------------
// # General Issues
// ---------------------------------------------------------------------

func renderGeneralIssues(sb *strings.Builder, s *ReviewSession) {
	sb.WriteString("# General Issues\n\n")
	for i, iss := range s.issues {
		fmt.Fprintf(sb, "## Issue %d: %s\n\n", i+1, iss.Title)
		if iss.Author != "" || iss.Date != "" {
			fmt.Fprintf(sb, "**%s** (%s)\n\n", iss.Author, iss.Date)
		}
		if iss.Body != "" {
			sb.WriteString(shiftUserHeadingsUp(iss.Body))
			sb.WriteString("\n\n")
		}
	}
}

// ---------------------------------------------------------------------
// # Changes
// ---------------------------------------------------------------------

func renderChanges(sb *strings.Builder, s *ReviewSession) {
	sb.WriteString("# Changes\n\n")

	for _, path := range s.Scope.Files {
		fmt.Fprintf(sb, "## Changes in `%s`\n\n", EncodePath(path))
		patch := s.Scope.FilePatch(path)
		if patch == "" {
			continue
		}
		renderQuotedDiff(sb, path, patch, eventsForFile(s, path))
		sb.WriteString("\n")
	}
}

// signAfterNumbers skips the leading line-number gutter (one or two
// space-separated integers) and returns the sign character that
// follows ('+', '-', or ':'). Returns 0 if no sign is found — the
// caller falls back to legacy body[0] parsing.
func signAfterNumbers(body string) byte {
	i := 0
	// First number (always present in the new format).
	if i >= len(body) || body[i] < '0' || body[i] > '9' {
		return 0
	}
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		i++
	}
	for i < len(body) && body[i] == ' ' {
		i++
	}
	if i >= len(body) {
		return 0
	}
	// Optional second number (context lines carry both new+old).
	if body[i] >= '0' && body[i] <= '9' {
		for i < len(body) && body[i] >= '0' && body[i] <= '9' {
			i++
		}
		for i < len(body) && body[i] == ' ' {
			i++
		}
		if i >= len(body) {
			return 0
		}
	}
	switch body[i] {
	case '+', '-', ':':
		return body[i]
	}
	return 0
}

// digitsIn returns the number of base-10 digits in `n`. Used to size
// the line-number gutter in renderQuotedDiff so the largest line in
// a file picks the column width.
func digitsIn(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

// renderQuotedDiff writes a unified diff prefixed by `> `, with reviewer
// events inserted inline at their anchored positions. Anchoring rule:
// hunk-anchored events (Like/Dislike/ReadStart/ReadEnd on a hunk) are emitted
// after the hunk's last `> ` line; line-anchored events (Comment/Question on
// path:line[-end]) are emitted after the matching `> +line`.
func renderQuotedDiff(sb *strings.Builder, path, patch string, evs []emittableEvent) {
	if patch == "" {
		return
	}

	// Sort events into hunk-anchored (hunkRange "newStart,newLines") vs
	// line-anchored (single line or range "start-end").
	byHunk := map[string][]emittableEvent{}
	byLineEnd := map[int][]emittableEvent{} // event emitted after new-side line N

	for _, e := range evs {
		anc := strings.TrimPrefix(string(e.Anchor), path+":")
		if anc == string(e.Anchor) {
			continue // not for this path
		}
		// hunk anchor: "<start>,<count>"
		if i := strings.Index(anc, ","); i > 0 {
			if _, err := strconv.Atoi(anc[:i]); err == nil {
				if _, err := strconv.Atoi(anc[i+1:]); err == nil {
					byHunk[anc] = append(byHunk[anc], e)
					continue
				}
			}
		}
		// line anchor: "N" or "N-M"
		endStr := anc
		if i := strings.Index(anc, "-"); i > 0 {
			endStr = anc[i+1:]
		}
		if n, err := strconv.Atoi(endStr); err == nil {
			byLineEnd[n] = append(byLineEnd[n], e)
		}
	}

	lines := strings.Split(strings.TrimRight(patch, "\n"), "\n")

	// Pre-scan to find the widest line number we'll need to render, so
	// the gutter width is stable across the whole file's diff. Without
	// this each hunk would pick its own width and the columns wouldn't
	// line up when you cat the note.
	numW := 1
	for _, line := range lines {
		if !strings.HasPrefix(line, "@@") {
			continue
		}
		h := parseHunkHeader(line)
		if n := digitsIn(h.NewStart + h.NewLines); n > numW {
			numW = n
		}
		if n := digitsIn(h.OldStart + h.OldLines); n > numW {
			numW = n
		}
	}

	inHunk := false
	var curHunkKey string
	var newLine, oldLine int
	var hunkBudget int // remaining new-side lines in current hunk

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			if inHunk {
				// hunk ended at last line — flush pending hunk events.
				emitEvents(sb, byHunk[curHunkKey])
				delete(byHunk, curHunkKey)
			}
			h := parseHunkHeader(line)
			newLine = h.NewStart
			oldLine = h.OldStart
			hunkBudget = h.NewLines
			curHunkKey = fmt.Sprintf("%d,%d", h.NewStart, h.NewLines)
			inHunk = true
			fmt.Fprintln(sb, "> "+line)
			continue
		}

		if !inHunk || len(line) == 0 {
			// File-header lines (`> diff --git`, `> ---`, `> +++`,
			// `> index ...`) and blank patches stay as the raw text;
			// they have no line-number coordinates to inject.
			fmt.Fprintln(sb, "> "+line)
			continue
		}

		kind := line[0]
		body := line[1:]
		// Line-number gutter format:
		//   add (+):  "> <new> +<content>"
		//   del (-):  "> <old> -<content>"
		//   ctx (:):  "> <new> <old> :<content>"   (two numbers, always)
		// Context uses ':' (not the unified-diff space) so Parse can
		// tell context apart from added lines by the sign character
		// alone, regardless of how many numbers prefix it.
		switch kind {
		case '+':
			fmt.Fprintf(sb, "> %*d +%s\n", numW, newLine, body)
			emitEvents(sb, byLineEnd[newLine])
			delete(byLineEnd, newLine)
			newLine++
			hunkBudget--
		case '-':
			fmt.Fprintf(sb, "> %*d -%s\n", numW, oldLine, body)
			oldLine++
			// Deletes don't consume new-side budget.
		case ' ':
			fmt.Fprintf(sb, "> %*d %*d :%s\n", numW, newLine, numW, oldLine, body)
			emitEvents(sb, byLineEnd[newLine])
			delete(byLineEnd, newLine)
			newLine++
			oldLine++
			hunkBudget--
		default:
			// "\ No newline at end of file" and similar metadata —
			// pass through verbatim.
			fmt.Fprintln(sb, "> "+line)
			continue
		}
		if hunkBudget <= 0 {
			// Last new-side line of the hunk. Emit hunk-anchored events.
			emitEvents(sb, byHunk[curHunkKey])
			delete(byHunk, curHunkKey)
			inHunk = false
		}
	}
	if inHunk {
		emitEvents(sb, byHunk[curHunkKey])
	}
	// Stray events whose anchors didn't match anything: dump at the end so
	// they aren't lost.
	for _, leftover := range byHunk {
		emitEvents(sb, leftover)
	}
	for _, leftover := range byLineEnd {
		emitEvents(sb, leftover)
	}
}

// ---------------------------------------------------------------------
// # Commits
// ---------------------------------------------------------------------

func renderCommits(sb *strings.Builder, s *ReviewSession) {
	sb.WriteString("# Commits\n\n")
	// Spec: emit commits oldest first. `git log` (ScopeFor's source) gives
	// newest first, so we walk in reverse.
	for i := len(s.Scope.Commits) - 1; i >= 0; i-- {
		c := s.Scope.Commits[i]
		fmt.Fprintf(sb, "## Commit %s: %s\n\n", c.Short, c.Subject)
		patch := s.Scope.CommitPatch(c.SHA)
		if patch == "" {
			continue
		}
		// Commits don't carry per-line anchored events in this v1 (the TUI
		// anchors events on file paths). Emit the patch verbatim.
		for _, line := range strings.Split(strings.TrimRight(patch, "\n"), "\n") {
			fmt.Fprintln(sb, "> "+line)
		}
		sb.WriteString("\n")
	}
}

// ---------------------------------------------------------------------
// # File Review
// ---------------------------------------------------------------------

func renderFileReviewSection(sb *strings.Builder, s *ReviewSession) {
	if len(s.fileReviews) == 0 {
		return
	}
	sb.WriteString("# File Review\n\n")
	for _, fr := range s.fileReviews {
		short := fr.TipSHA
		if len(short) > 12 {
			short = short[:12]
		}
		fmt.Fprintf(sb, "## File `%s` @ %s\n\n", EncodePath(fr.Path), short)
		for _, fl := range fr.Lines {
			if fl.Content == "" {
				fmt.Fprintf(sb, "> %d:\n", fl.Number)
			} else {
				fmt.Fprintf(sb, "> %d: %s\n", fl.Number, fl.Content)
			}
		}
		sb.WriteString("\n")
	}
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

// emittableEvent is a normalised form of any reviewer action ready to be
// written as an H3 event heading.
type emittableEvent struct {
	Kind   string // "Comment" | "Question" | "Like" | "Dislike" | "ReadStart" | "ReadEnd"
	Author string
	Date   string // RFC3339
	Body   string
	Anchor Anchor // for sorting / routing
}

// eventsForFile gathers all events that anchor to a given file, in a stable order.
func eventsForFile(s *ReviewSession, path string) []emittableEvent {
	prefix := path + ":"
	var out []emittableEvent

	for _, c := range s.comments {
		if !strings.HasPrefix(string(c.Anchor), prefix) {
			continue
		}
		kind := "Comment"
		if c.Kind == KindQuestion {
			kind = "Question"
		}
		out = append(out, emittableEvent{
			Kind: kind, Author: c.Author, Date: c.Date, Body: c.Text, Anchor: c.Anchor,
		})
	}

	for a, m := range s.markers {
		if !strings.HasPrefix(string(a), prefix) {
			continue
		}
		kind := "Like"
		if m == MarkerBad {
			kind = "Dislike"
		}
		out = append(out, emittableEvent{
			Kind: kind, Author: s.Reviewer, Date: s.Date, Anchor: a,
		})
	}

	// Read and Skip lanes used to emit a ReadStart+ReadEnd pair per
	// stored anchor. With per-line tracking that turned a 30-line
	// skip into 60 events and made the rendered note unreadable.
	// Coalesce contiguous line anchors into a single range
	// (`path:start-end`) and emit one Start + one End for the run.
	// Hunk-style anchors (`path:N,M`, legacy) are kept as-is so old
	// notes still parse round-trip.
	for _, ev := range coalesceLineRange(s.read, path, "ReadStart", "ReadEnd",
		s.Reviewer, s.Date) {
		out = append(out, ev)
	}
	for _, ev := range coalesceLineRange(s.skipped, path, "SkipStart", "SkipEnd",
		s.Reviewer, s.Date) {
		out = append(out, ev)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Date < out[j].Date
	})
	return out
}

// expandRangeAnchor turns an anchor that may carry a `path:N-M`
// range into one anchor per included line (`path:N`, `path:N+1`,
// …, `path:M`). Single-line anchors and non-range shapes pass
// through unchanged so hunk-anchored / file-anchored events still
// land in the map at their original key.
func expandRangeAnchor(a Anchor) []Anchor {
	s := string(a)
	colon := strings.LastIndex(s, ":")
	if colon < 0 {
		return []Anchor{a}
	}
	path, rest := s[:colon], s[colon+1:]
	dash := strings.Index(rest, "-")
	if dash <= 0 {
		return []Anchor{a}
	}
	start, err1 := strconv.Atoi(rest[:dash])
	end, err2 := strconv.Atoi(rest[dash+1:])
	if err1 != nil || err2 != nil || end < start {
		return []Anchor{a}
	}
	out := make([]Anchor, 0, end-start+1)
	for n := start; n <= end; n++ {
		out = append(out, Anchor(fmt.Sprintf("%s:%d", path, n)))
	}
	return out
}

// coalesceLineRange turns a set of per-line anchors (`path:N`) into
// the minimum number of Start/End event pairs by grouping contiguous
// runs into a `path:start-end` range. Hunk-style anchors
// (`path:start,count`) and other shapes that aren't a plain integer
// pass through unchanged so legacy notes keep their semantics.
//
// kindStart/kindEnd are the event kinds for this lane (e.g.
// "ReadStart" / "ReadEnd"). The two events use the same anchor so
// the parser can re-derive the run on load.
func coalesceLineRange(set map[Anchor]bool, path, kindStart, kindEnd, author, date string) []emittableEvent {
	prefix := path + ":"

	var lines []int
	var passthrough []Anchor
	for a, on := range set {
		if !on || !strings.HasPrefix(string(a), prefix) {
			continue
		}
		rest := string(a)[len(prefix):]
		// Hunk anchor (`N,M`) or any non-integer suffix: keep as-is,
		// it's not a plain line we can fold into a range.
		if strings.ContainsAny(rest, ",-") {
			passthrough = append(passthrough, a)
			continue
		}
		n, err := strconv.Atoi(rest)
		if err != nil {
			passthrough = append(passthrough, a)
			continue
		}
		lines = append(lines, n)
	}

	sort.Ints(lines)
	var out []emittableEvent

	// Walk consecutive runs.
	i := 0
	for i < len(lines) {
		j := i
		for j+1 < len(lines) && lines[j+1] == lines[j]+1 {
			j++
		}
		// Always emit as a range, even for a single line — the
		// format only carries range anchors for Read/Skip so the
		// shape is one event-pair per run, never one per line.
		anchor := Anchor(fmt.Sprintf("%s:%d-%d", path, lines[i], lines[j]))
		out = append(out,
			emittableEvent{Kind: kindStart, Author: author, Date: date, Anchor: anchor},
			emittableEvent{Kind: kindEnd, Author: author, Date: date, Anchor: anchor},
		)
		i = j + 1
	}

	// Legacy hunk/other anchors keep their original (Start, End) pair.
	sort.Slice(passthrough, func(i, j int) bool { return passthrough[i] < passthrough[j] })
	for _, a := range passthrough {
		out = append(out,
			emittableEvent{Kind: kindStart, Author: author, Date: date, Anchor: a},
			emittableEvent{Kind: kindEnd, Author: author, Date: date, Anchor: a},
		)
	}
	return out
}

func emitEvents(sb *strings.Builder, evs []emittableEvent) {
	for _, e := range evs {
		sb.WriteString("\n")
		// Read/Skip events embed their anchor in the header so Parse
		// can recover the exact range without re-deriving it from
		// diff position. Without this a `path:1-5` range emitted
		// after line 5 would parse as a hunk anchor — losing the
		// range — and the TUI's per-line tracker would lose state.
		switch e.Kind {
		case "ReadStart", "ReadEnd", "SkipStart", "SkipEnd":
			fmt.Fprintf(sb, "### %s @ %s (From: %s, %s)\n",
				e.Kind, string(e.Anchor),
				formatAuthor(e.Author), defaultDate(e.Date))
		default:
			fmt.Fprintf(sb, "### %s (From: %s, %s)\n",
				e.Kind, formatAuthor(e.Author), defaultDate(e.Date))
		}
		if e.Body != "" {
			sb.WriteString("\n")
			sb.WriteString(shiftUserHeadingsUp(e.Body))
			sb.WriteString("\n")
		}
	}
}

// formatAuthor coerces an author string into the canonical "Name <email>" form
// the parser's regex expects. Inputs accepted:
//   - "alice@example.com"          → "alice <alice@example.com>"
//   - "Alice"                       → "Alice <Alice>"
//   - "Alice <alice@example.com>"  → unchanged
func formatAuthor(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown <unknown>"
	}
	if strings.Contains(raw, "<") && strings.Contains(raw, ">") {
		return raw
	}
	if at := strings.Index(raw, "@"); at > 0 {
		return raw[:at] + " <" + raw + ">"
	}
	return raw + " <" + raw + ">"
}

// AuthorEmail extracts the email portion from a formatted author string.
// "Alice <alice@example.com>" → "alice@example.com".
func AuthorEmail(formatted string) string {
	if i := strings.Index(formatted, "<"); i >= 0 {
		if j := strings.Index(formatted[i+1:], ">"); j > 0 {
			return formatted[i+1 : i+1+j]
		}
	}
	return formatted
}

func defaultDate(d string) string {
	if d == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return d
}

// shiftUserHeadingsUp shifts user-typed H1/H2/H3 → H4/H5/H6 (clamped) so
// they can't be confused with the file's chapter structure. Blockquote
// shift `>` → `>>` is also applied so user text doesn't collide with patch
// quoting.
func shiftUserHeadingsUp(text string) string {
	var out strings.Builder
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		// Heading shift.
		if i := countHashes(line); i > 0 && i <= 3 && len(line) > i && line[i] == ' ' {
			line = strings.Repeat("#", i+3) + line[i:]
		}
		// Blockquote shift: any leading run of `>` gets one more.
		if strings.HasPrefix(line, ">") {
			j := 0
			for j < len(line) && line[j] == '>' {
				j++
			}
			line = ">" + line // prefix one more `>`
			_ = j
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

// shiftUserHeadingsDown inverts shiftUserHeadingsUp for parsed bodies.
func shiftUserHeadingsDown(text string) string {
	var out strings.Builder
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if i := countHashes(line); i >= 4 && i <= 6 && len(line) > i && line[i] == ' ' {
			line = strings.Repeat("#", i-3) + line[i:]
		}
		if strings.HasPrefix(line, ">>") {
			line = line[1:] // strip one `>`
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

func countHashes(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	return n
}

var (
	pathSafeRe   = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)
	slugUnsafeR  = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	// Format:
	//   ### <Kind>[: <state>] [@ <anchor>] (From: <Name> <<email>>, <date>)
	// `@ <anchor>` is currently emitted only for Read/Skip ranges,
	// where the diff-position parser would otherwise collapse the
	// range. Comment/Question/Like/Dislike keep their anchor implicit
	// (derived from the preceding `> +N` line) for backwards
	// compatibility with existing notes.
	eventHeaderR = regexp.MustCompile(`^### (Comment|Question|ReadStart|ReadEnd|SkipStart|SkipEnd|Like|Dislike|Verdict)(?:: (open|requested-changes|approved|denied))?(?: @ (\S+))? \(From: (.+) <(.+)>, (.+)\)$`)
	fileLineR    = regexp.MustCompile(`^> (\d+):(?: (.*))?$`)
)

// slugify makes a string safe for use in filenames.
func slugify(s string) string {
	return strings.Trim(slugUnsafeR.ReplaceAllString(s, "-"), "-")
}

// EncodePath escapes a path for inclusion in `## Changes in `<path>``
// or `## File `<path>` @ <sha>` headings. ASCII alnum + `_./-` are literal;
// other runes are \uXXXX, plus \\ and \` escapes.
func EncodePath(p string) string {
	if pathSafeRe.MatchString(p) {
		return p
	}
	var sb strings.Builder
	for _, r := range p {
		switch {
		case r == '\\':
			sb.WriteString(`\\`)
		case r == '`':
			sb.WriteString("\\`")
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '.' || r == '/' || r == '-':
			sb.WriteRune(r)
		default:
			fmt.Fprintf(&sb, `\u%04x`, r)
		}
	}
	return sb.String()
}

// DecodePath reverses EncodePath.
func DecodePath(p string) string {
	if !strings.ContainsAny(p, "\\") {
		return p
	}
	var sb strings.Builder
	for i := 0; i < len(p); {
		if p[i] == '\\' && i+1 < len(p) {
			switch p[i+1] {
			case '\\':
				sb.WriteByte('\\')
				i += 2
				continue
			case '`':
				sb.WriteByte('`')
				i += 2
				continue
			case 'u':
				if i+5 < len(p) {
					if r, err := strconv.ParseUint(p[i+2:i+6], 16, 32); err == nil {
						sb.WriteRune(rune(r))
						i += 6
						continue
					}
				}
			}
		}
		sb.WriteByte(p[i])
		i++
	}
	return sb.String()
}

// ---------------------------------------------------------------------
// Parse — recover a ReviewSession from a rendered .review file
// ---------------------------------------------------------------------

// Parse reads a `.review` file's body and reconstructs a ReviewSession.
// The scope's `RawDiff` and per-file patches are NOT re-derived from the
// file; the caller is expected to call ScopeFor afterwards if those are
// needed. This parser focuses on persisted user content: verdicts, issues,
// comments/questions, like/dislike markers, read ranges, file reviews.
func Parse(content string) (*ReviewSession, error) {
	s := &ReviewSession{
		Verdict: VerdictOpen,
		read:    map[Anchor]bool{},
		skipped: map[Anchor]bool{},
		markers: map[Anchor]Marker{},
	}

	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	type state struct {
		section    string // "Review" | "Verdicts" | "GeneralIssues" | "Changes" | "Commits" | "FileReview"
		subPath    string // when section is Changes (path) or FileReview (path) or Issue (n) or Commit (sha)
		issueTitle string
		issueBody  strings.Builder
		issueAuth  string
		issueDate  string
		fileTipSHA string
		fileLines  []FileLine
		// for events: anchor recovery from preceding `> ` patch line
		newLine     int
		lastNewLine int // the newLine value of the most recently emitted `+` or ` ` line
		hunkAnchor  Anchor
		inHunk      bool
		hunkBudget  int
		// pending event being parsed
		evKind   string
		evParam  string
		evAuth   string
		evDate   string
		evBody   strings.Builder
		evAnchor Anchor
	}
	st := &state{}

	flushIssue := func() {
		if st.issueTitle != "" {
			s.issues = append(s.issues, Issue{
				Title:  st.issueTitle,
				Author: st.issueAuth,
				Date:   st.issueDate,
				Body:   strings.TrimSpace(shiftUserHeadingsDown(st.issueBody.String())),
			})
		}
		st.issueTitle = ""
		st.issueAuth = ""
		st.issueDate = ""
		st.issueBody.Reset()
	}
	flushFile := func() {
		if st.subPath != "" && len(st.fileLines) > 0 {
			s.fileReviews = append(s.fileReviews, FileReview{
				Path:   st.subPath,
				TipSHA: st.fileTipSHA,
				Lines:  st.fileLines,
			})
		}
		st.fileLines = nil
		st.fileTipSHA = ""
	}
	flushEvent := func() {
		if st.evKind == "" {
			return
		}
		body := strings.TrimSpace(shiftUserHeadingsDown(st.evBody.String()))
		switch st.evKind {
		case "Comment", "Question":
			kind := KindComment
			if st.evKind == "Question" {
				kind = KindQuestion
			}
			s.comments = append(s.comments, Comment{
				Anchor: st.evAnchor,
				Author: st.evAuth, // already "Name <email>" form
				Date:   st.evDate,
				Text:   body,
				Kind:   kind,
			})
		case "Like":
			s.markers[st.evAnchor] = MarkerGood
		case "Dislike":
			s.markers[st.evAnchor] = MarkerBad
		case "ReadStart", "ReadEnd":
			// Renderer coalesces consecutive lines into `path:N-M`
			// ranges. Expand back into per-line entries so the
			// TUI's per-line tracker stays accurate.
			for _, a := range expandRangeAnchor(st.evAnchor) {
				s.read[a] = true
			}
		case "SkipStart", "SkipEnd":
			if s.skipped == nil {
				s.skipped = map[Anchor]bool{}
			}
			for _, a := range expandRangeAnchor(st.evAnchor) {
				s.skipped[a] = true
			}
		case "Verdict":
			s.verdicts = append(s.verdicts, VerdictEvent{
				State:   Verdict(st.evParam),
				Author:  st.evAuth,
				Date:    st.evDate,
				Summary: body,
			})
		}
		st.evKind = ""
		st.evParam = ""
		st.evAuth = ""
		st.evDate = ""
		st.evBody.Reset()
		st.evAnchor = ""
	}

	for sc.Scan() {
		line := sc.Text()

		// H1: top-level section
		if strings.HasPrefix(line, "# ") {
			flushEvent()
			flushIssue()
			flushFile()
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			switch title {
			case "Review":
				st.section = "Review"
			case "General Issues":
				st.section = "GeneralIssues"
			case "Changes":
				st.section = "Changes"
			case "Commits":
				st.section = "Commits"
			case "File Review":
				st.section = "FileReview"
			default:
				st.section = ""
			}
			st.subPath = ""
			st.inHunk = false
			continue
		}

		// H2: subsection
		if strings.HasPrefix(line, "## ") {
			flushEvent()
			flushIssue()
			flushFile()
			st.hunkAnchor = ""
			st.lastNewLine = 0
			st.inHunk = false
			title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			switch {
			case title == "Sources" || title == "Verdicts":
				// stays in section Review; nothing to record
			case strings.HasPrefix(title, "Issue "):
				// `Issue N: title`
				if i := strings.Index(title, ": "); i > 0 {
					st.issueTitle = title[i+2:]
				}
			case strings.HasPrefix(title, "Changes in `"):
				// `Changes in `<path>``
				if i := strings.Index(title, "`"); i > 0 {
					rest := title[i+1:]
					if j := strings.LastIndex(rest, "`"); j > 0 {
						st.subPath = DecodePath(rest[:j])
					}
				}
			case strings.HasPrefix(title, "Commit "):
				rest := strings.TrimPrefix(title, "Commit ")
				if i := strings.Index(rest, ": "); i > 0 {
					st.subPath = rest[:i] // sha
				}
			case strings.HasPrefix(title, "File `"):
				// `File `<path>` @ <sha>`
				rest := strings.TrimPrefix(title, "File `")
				if i := strings.LastIndex(rest, "` @ "); i > 0 {
					st.subPath = DecodePath(rest[:i])
					st.fileTipSHA = strings.TrimSpace(rest[i+4:])
				}
			}
			st.inHunk = false
			continue
		}

		// H3: event header
		if strings.HasPrefix(line, "### ") {
			flushEvent()
			m := eventHeaderR.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			st.evKind = m[1]
			st.evParam = m[2]
			st.evAuth = fmt.Sprintf("%s <%s>", m[4], m[5])
			st.evDate = m[6]
			explicitAnchor := m[3]
			st.evAnchor = ""
			// If the header carried an `@ <anchor>` (Read/Skip ranges
			// or anything else that needs an exact key) take it
			// verbatim — that's the authoritative source. Otherwise
			// fall back to deriving from parse position.
			if explicitAnchor != "" {
				st.evAnchor = Anchor(explicitAnchor)
				continue
			}
			// Anchor: derive from current parse position. The event kind
			// decides whether to use the line anchor or the hunk anchor.
			switch st.section {
			case "Changes":
				switch st.evKind {
				case "ReadStart", "ReadEnd", "SkipStart", "SkipEnd":
					// Read/Skip are hunk-scoped by design.
					if st.hunkAnchor != "" {
						st.evAnchor = st.hunkAnchor
					} else if st.subPath != "" {
						st.evAnchor = Anchor(st.subPath)
					}
				default: // Comment, Question, Like, Dislike
					// Prefer the immediately-preceding `> +N` line so a
					// marker placed on a single line round-trips to that
					// line, not the surrounding hunk. Falls back to the
					// hunk anchor when there's no line context (e.g. an
					// event emitted before any `+`/` ` line was seen).
					if st.lastNewLine > 0 && st.subPath != "" {
						st.evAnchor = Anchor(fmt.Sprintf("%s:%d", st.subPath, st.lastNewLine))
					} else if st.hunkAnchor != "" {
						st.evAnchor = st.hunkAnchor
					} else if st.subPath != "" {
						st.evAnchor = Anchor(st.subPath)
					}
				}
			case "FileReview":
				if n := len(st.fileLines); n > 0 {
					last := st.fileLines[n-1]
					short := st.fileTipSHA
					if len(short) > 12 {
						short = short[:12]
					}
					st.evAnchor = Anchor(fmt.Sprintf("%s@%s:%d", st.subPath, short, last.Number))
				} else if st.subPath != "" {
					st.evAnchor = Anchor(st.subPath)
				}
			}
			continue
		}

		// Quoted patch line
		if strings.HasPrefix(line, "> ") || line == ">" {
			flushEvent()
			body := ""
			if strings.HasPrefix(line, "> ") {
				body = line[2:]
			}
			// In file-review subsection, `> N: content` lines.
			if st.section == "FileReview" && st.subPath != "" {
				if mm := fileLineR.FindStringSubmatch(line); mm != nil {
					n, _ := strconv.Atoi(mm[1])
					st.fileLines = append(st.fileLines, FileLine{Number: n, Content: mm[2]})
				}
				continue
			}
			// In Changes / Commits, walk hunks to track newLine for anchors.
			if strings.HasPrefix(body, "@@") {
				h := parseHunkHeader(body)
				st.newLine = h.NewStart
				st.hunkBudget = h.NewLines
				st.inHunk = true
				if st.section == "Changes" {
					st.hunkAnchor = HunkAnchor(st.subPath, h.NewStart, h.NewLines)
				}
				continue
			}
			if st.inHunk && len(body) > 0 {
				// New format prefixes each in-hunk line with one or
				// two line numbers, so the sign isn't at body[0] any
				// more. Scan past digits + spaces to find it. Falls
				// back to body[0] for old notes that pre-date the
				// gutter (sign was the first char in those).
				kind := signAfterNumbers(body)
				if kind == 0 {
					kind = body[0]
				}
				// `+` (add) and `:` (context, new format) advance
				// new-side. Legacy ` ` (context, old format) too.
				if kind == '+' || kind == ' ' || kind == ':' {
					st.lastNewLine = st.newLine
					st.newLine++
					st.hunkBudget--
					if st.hunkBudget <= 0 {
						st.inHunk = false
					}
				}
			}
			continue
		}

		// Inside an event body, plain markdown lines.
		if st.evKind != "" {
			if line == "" && st.evBody.Len() == 0 {
				continue
			}
			st.evBody.WriteString(line)
			st.evBody.WriteString("\n")
			continue
		}

		// Issue body lines.
		if st.section == "GeneralIssues" && st.issueTitle != "" {
			if strings.HasPrefix(line, "**") && st.issueBody.Len() == 0 {
				rest := strings.TrimPrefix(line, "**")
				if idx := strings.Index(rest, "**"); idx >= 0 {
					st.issueAuth = rest[:idx]
					tail := strings.TrimSpace(rest[idx+2:])
					if strings.HasPrefix(tail, "(") {
						if e := strings.Index(tail, ")"); e > 0 {
							st.issueDate = tail[1:e]
						}
					}
					continue
				}
			}
			st.issueBody.WriteString(line)
			st.issueBody.WriteString("\n")
			continue
		}

		// Otherwise: ignore (Sources lines, etc.)
	}

	// Close out anything left open.
	flushEvent()
	flushIssue()
	flushFile()

	// Sync canonical Verdict/Summary/Reviewer with the audit log.
	if len(s.verdicts) > 0 {
		last := s.verdicts[len(s.verdicts)-1]
		s.Verdict = last.State
		s.Summary = last.Summary
		if s.Reviewer == "" {
			s.Reviewer = AuthorEmail(s.verdicts[0].Author)
			s.Date = s.verdicts[0].Date
		}
	}

	return s, nil
}

// nowRFC3339 returns the current UTC time in RFC3339 (placed here so
// format.go is self-contained).
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
