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

	inHunk := false
	var curHunkKey string
	var newLine int
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
			hunkBudget = h.NewLines
			curHunkKey = fmt.Sprintf("%d,%d", h.NewStart, h.NewLines)
			inHunk = true
			fmt.Fprintln(sb, "> "+line)
			continue
		}

		fmt.Fprintln(sb, "> "+line)

		if !inHunk || len(line) == 0 {
			continue
		}
		kind := line[0]
		if kind == '+' || kind == ' ' {
			// After emitting this new-side line, drop any line-anchored events.
			emitEvents(sb, byLineEnd[newLine])
			delete(byLineEnd, newLine)
			newLine++
			hunkBudget--
			if hunkBudget <= 0 {
				// Last new-side line of the hunk. Emit hunk-anchored events.
				emitEvents(sb, byHunk[curHunkKey])
				delete(byHunk, curHunkKey)
				inHunk = false
			}
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
	for _, c := range s.Scope.Commits {
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

	// Each fully-read hunk emits a ReadStart + ReadEnd at the same anchor;
	// the writer places ReadStart at the hunk header and ReadEnd at the
	// hunk tail (handled in renderQuotedDiff).
	for a, isRead := range s.read {
		if !isRead || !strings.HasPrefix(string(a), prefix) {
			continue
		}
		out = append(out,
			emittableEvent{Kind: "ReadStart", Author: s.Reviewer, Date: s.Date, Anchor: a},
			emittableEvent{Kind: "ReadEnd", Author: s.Reviewer, Date: s.Date, Anchor: a},
		)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Date < out[j].Date
	})
	return out
}

func emitEvents(sb *strings.Builder, evs []emittableEvent) {
	for _, e := range evs {
		sb.WriteString("\n")
		fmt.Fprintf(sb, "### %s (From: %s, %s)\n", e.Kind, formatAuthor(e.Author), defaultDate(e.Date))
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
	eventHeaderR = regexp.MustCompile(`^### (Comment|Question|ReadStart|ReadEnd|Like|Dislike|Verdict)(?:: (open|requested-changes|approved|denied))? \(From: (.+) <(.+)>, (.+)\)$`)
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
			// For v1 we mark the anchored hunk read on either start or end.
			s.read[st.evAnchor] = true
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
			st.evAuth = fmt.Sprintf("%s <%s>", m[3], m[4])
			st.evDate = m[5]
			// Anchor: derive from current parse position. The event kind
			// decides whether to use the line anchor or the hunk anchor.
			switch st.section {
			case "Changes":
				switch st.evKind {
				case "Like", "Dislike", "ReadStart", "ReadEnd":
					if st.hunkAnchor != "" {
						st.evAnchor = st.hunkAnchor
					} else if st.subPath != "" {
						st.evAnchor = Anchor(st.subPath)
					}
				default: // Comment, Question
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
				kind := body[0]
				if kind == '+' || kind == ' ' {
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
