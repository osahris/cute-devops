// SPDX-FileCopyrightText: 2026 Markus Katharina Brechtel <markus.katharina.brechtel@thengo.net>
// SPDX-License-Identifier: EUPL-1.2

package review

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Render serialises a Session to the review.md format.
func Render(s *Session) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("type: review\n")
	fmt.Fprintf(&sb, "reviewer: %s\n", s.Reviewer)
	fmt.Fprintf(&sb, "date: %s\n", s.Date)
	fmt.Fprintf(&sb, "verdict: %s\n", s.Verdict)
	sb.WriteString("scope:\n")
	fmt.Fprintf(&sb, "  diff: %s\n", s.Scope.Diff)
	fmt.Fprintf(&sb, "  tip: %s\n", s.Scope.TipSHA)
	sb.WriteString("  commits:\n")
	for _, c := range s.Scope.Commits {
		fmt.Fprintf(&sb, "    - %s  # %s\n", c.Short, c.Subject)
	}
	sb.WriteString("  files:\n")
	for _, f := range s.Scope.Files {
		fmt.Fprintf(&sb, "    - %s\n", f)
	}
	if len(s.read) > 0 {
		sb.WriteString("read:\n")
		for _, a := range s.ReadAnchors() {
			fmt.Fprintf(&sb, "  - %s\n", a)
		}
	}
	if len(s.markers) > 0 {
		sb.WriteString("markers:\n")
		for _, a := range s.MarkerAnchors() {
			fmt.Fprintf(&sb, "  %s: %s\n", a, s.markers[a])
		}
	}
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# Review: %s\n\n", s.Scope.Title)

	sb.WriteString("## Verdict\n\n")
	if s.Summary == "" {
		sb.WriteString("_(reviewer: explain the verdict here)_\n\n")
	} else {
		sb.WriteString(s.Summary)
		if !strings.HasSuffix(s.Summary, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Split comments by kind.
	var threads, questions []Comment
	for _, c := range s.comments {
		if c.Kind == KindQuestion {
			questions = append(questions, c)
		} else {
			threads = append(threads, c)
		}
	}

	writeSection := func(heading string, cs []Comment, emptyHint string) {
		fmt.Fprintf(&sb, "## %s\n\n", heading)
		if len(cs) == 0 {
			fmt.Fprintf(&sb, "_(%s)_\n\n", emptyHint)
			return
		}
		grouped := groupByAnchor(cs)
		for _, key := range sortedKeys(grouped) {
			fmt.Fprintf(&sb, "### %s\n\n", key)
			for _, c := range grouped[key] {
				if c.Snippet != "" {
					fence := pickFence(c.Snippet)
					sb.WriteString(fence + "diff\n")
					sb.WriteString(c.Snippet)
					if !strings.HasSuffix(c.Snippet, "\n") {
						sb.WriteString("\n")
					}
					sb.WriteString(fence + "\n\n")
				}
				fmt.Fprintf(&sb, "**%s** (%s):\n", c.Author, c.Date)
				sb.WriteString(c.Text)
				if !strings.HasSuffix(c.Text, "\n") {
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}
		}
	}

	writeSection("Threads", threads, "no comments yet")
	writeSection("Questions", questions, "no questions yet")

	// Free-form issues added during review.
	sb.WriteString("## Issues\n\n")
	if len(s.issues) == 0 {
		sb.WriteString("_(no issues yet)_\n\n")
	} else {
		for _, it := range s.issues {
			fmt.Fprintf(&sb, "### %s\n\n", it.Title)
			if it.Author != "" || it.Date != "" {
				fmt.Fprintf(&sb, "**%s** (%s)\n\n", it.Author, it.Date)
			}
			body := strings.TrimRight(it.Body, "\n")
			if body != "" {
				sb.WriteString(body)
				sb.WriteString("\n\n")
			}
		}
	}

	if s.Scope.RawDiff != "" {
		fence := pickFence(s.Scope.RawDiff)
		sb.WriteString("<details>\n")
		fmt.Fprintf(&sb, "<summary>Full diff (%d files)</summary>\n\n", len(s.Scope.Files))
		sb.WriteString(fence + "diff\n")
		sb.WriteString(s.Scope.RawDiff)
		if !strings.HasSuffix(s.Scope.RawDiff, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(fence + "\n\n")
		sb.WriteString("</details>\n")
	}

	return sb.String()
}

func groupByAnchor(cs []Comment) map[string][]Comment {
	m := map[string][]Comment{}
	for _, c := range cs {
		m[string(c.Anchor)] = append(m[string(c.Anchor)], c)
	}
	return m
}

func sortedKeys(m map[string][]Comment) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func pickFence(s string) string {
	maxRun := 2
	cur := 0
	for _, r := range s {
		if r == '`' {
			cur++
			if cur > maxRun {
				maxRun = cur
			}
		} else {
			cur = 0
		}
	}
	return strings.Repeat("`", maxRun+1)
}

var slugUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func slugify(s string) string {
	return strings.Trim(slugUnsafe.ReplaceAllString(s, "-"), "-")
}

// Load reads a review.md file and reconstructs a Session.
func Load(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &Session{
		Path:    path,
		Verdict: VerdictOpen,
		read:    map[Anchor]bool{},
		markers: map[Anchor]Marker{},
	}

	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, fmt.Errorf("load: %s: missing frontmatter opener", path)
	}
	if err := parseFrontmatter(sc, s); err != nil {
		return nil, err
	}
	parseBody(sc, s)
	s.dirty = false
	return s, nil
}

var (
	keyValPat   = regexp.MustCompile(`^(\w+):\s*(.*)$`)
	commitPat   = regexp.MustCompile(`^\s*-\s+([0-9a-f]+)\s*(#\s*(.*))?$`)
	listItemPat = regexp.MustCompile(`^\s*-\s+(.+)$`)
	markerPat   = regexp.MustCompile(`^\s+(\S+):\s*(\S+)$`)
)

func parseFrontmatter(sc *bufio.Scanner, s *Session) error {
	section := ""
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			return nil
		}
		// Top-level key (no indent).
		if !strings.HasPrefix(line, " ") {
			if m := keyValPat.FindStringSubmatch(line); m != nil {
				key, val := m[1], strings.TrimSpace(m[2])
				section = key
				switch key {
				case "reviewer":
					s.Reviewer = val
				case "date":
					s.Date = val
				case "verdict":
					s.Verdict = Verdict(val)
				}
				continue
			}
		}
		// Nested.
		switch section {
		case "scope":
			if m := keyValPat.FindStringSubmatch(strings.TrimLeft(line, " ")); m != nil {
				k, v := m[1], strings.TrimSpace(m[2])
				if k == "diff" {
					s.Scope.Diff = v
				} else if k == "tip" {
					s.Scope.TipSHA = v
				}
				continue
			}
			if m := commitPat.FindStringSubmatch(line); m != nil {
				c := Commit{SHA: m[1], Short: m[1]}
				if len(m) > 3 {
					c.Subject = strings.TrimSpace(m[3])
				}
				s.Scope.Commits = append(s.Scope.Commits, c)
				continue
			}
			if m := listItemPat.FindStringSubmatch(line); m != nil {
				s.Scope.Files = append(s.Scope.Files, strings.TrimSpace(m[1]))
			}
		case "read":
			if m := listItemPat.FindStringSubmatch(line); m != nil {
				s.read[Anchor(strings.TrimSpace(m[1]))] = true
			}
		case "markers":
			if m := markerPat.FindStringSubmatch(line); m != nil {
				s.markers[Anchor(m[1])] = Marker(m[2])
			}
		}
	}
	return fmt.Errorf("frontmatter: unexpected EOF")
}

// parseBody is best-effort: it pulls out commented threads under
// `## Threads`, `## Questions`, and `## Issues` headings. Verdict summary
// text under `## Verdict` is captured as Summary.
func parseBody(sc *bufio.Scanner, s *Session) {
	state := "" // "", "verdict", "threads", "questions", "issues"
	var anchor, snippet, fence string
	var curAuthor, curDate string
	var curText strings.Builder
	var summary strings.Builder
	var issueTitle string
	var issueBody strings.Builder

	flushIssue := func() {
		if issueTitle != "" {
			s.issues = append(s.issues, Issue{
				Title:  issueTitle,
				Body:   strings.TrimSpace(issueBody.String()),
				Author: curAuthor,
				Date:   curDate,
			})
		}
		issueTitle = ""
		issueBody.Reset()
		curAuthor = ""
		curDate = ""
	}

	flush := func() {
		if state == "issues" {
			flushIssue()
			return
		}
		if anchor != "" && (curText.Len() > 0 || snippet != "") {
			kind := KindComment
			if state == "questions" {
				kind = KindQuestion
			}
			s.AddComment(Comment{
				Anchor:  Anchor(anchor),
				Author:  curAuthor,
				Date:    curDate,
				Text:    strings.TrimRight(curText.String(), "\n"),
				Snippet: snippet,
				Kind:    kind,
			})
		}
		curText.Reset()
		snippet = ""
		curAuthor = ""
		curDate = ""
	}

	inDiff := false
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "## Verdict"):
			flush()
			anchor = ""
			state = "verdict"
		case strings.HasPrefix(line, "## Threads"):
			if state == "verdict" {
				s.Summary = strings.TrimSpace(summary.String())
			}
			flush()
			anchor = ""
			state = "threads"
		case strings.HasPrefix(line, "## Questions"):
			flush()
			anchor = ""
			state = "questions"
		case strings.HasPrefix(line, "## Issues"):
			flush()
			anchor = ""
			state = "issues"
		case strings.HasPrefix(line, "## "):
			if state == "verdict" {
				s.Summary = strings.TrimSpace(summary.String())
			}
			flush()
			state = ""
		case strings.HasPrefix(line, "### ") && (state == "threads" || state == "questions"):
			flush()
			anchor = strings.TrimSpace(strings.TrimPrefix(line, "### "))
		case strings.HasPrefix(line, "### ") && state == "issues":
			flushIssue()
			issueTitle = strings.TrimSpace(strings.TrimPrefix(line, "### "))
		case state == "verdict":
			if strings.HasPrefix(line, "_(") {
				continue // placeholder hint
			}
			summary.WriteString(line)
			summary.WriteString("\n")
		case inDiff:
			if line == fence {
				inDiff = false
				continue
			}
			if snippet != "" {
				snippet += "\n"
			}
			snippet += line
		case (state == "threads" || state == "questions") && (strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")):
			fence = strings.TrimSuffix(line, "diff")
			fence = strings.TrimSpace(fence)
			if fence != "" {
				inDiff = true
			}
		case (state == "threads" || state == "questions") && strings.HasPrefix(line, "**"):
			rest := strings.TrimPrefix(line, "**")
			if idx := strings.Index(rest, "**"); idx >= 0 {
				curAuthor = rest[:idx]
				tail := strings.TrimSpace(rest[idx+2:])
				if strings.HasPrefix(tail, "(") {
					if e := strings.Index(tail, ")"); e > 0 {
						curDate = tail[1:e]
					}
				}
			}
		case state == "threads" || state == "questions":
			if strings.TrimSpace(line) == "" && curText.Len() > 0 {
				curText.WriteString("\n")
				continue
			}
			if anchor != "" {
				curText.WriteString(line)
				curText.WriteString("\n")
			}
		case state == "issues":
			if issueTitle == "" {
				continue
			}
			if strings.HasPrefix(line, "**") {
				rest := strings.TrimPrefix(line, "**")
				if idx := strings.Index(rest, "**"); idx >= 0 {
					curAuthor = rest[:idx]
					tail := strings.TrimSpace(rest[idx+2:])
					if strings.HasPrefix(tail, "(") {
						if e := strings.Index(tail, ")"); e > 0 {
							curDate = tail[1:e]
						}
					}
					continue
				}
			}
			if strings.HasPrefix(line, "_(no issues") {
				continue
			}
			issueBody.WriteString(line)
			issueBody.WriteString("\n")
		}
	}
	if state == "verdict" {
		s.Summary = strings.TrimSpace(summary.String())
	}
	flush()
}
